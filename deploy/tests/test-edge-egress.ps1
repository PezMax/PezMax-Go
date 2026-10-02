<#
.SYNOPSIS
Build and exercise the production edge image in an isolated Docker network.
.DESCRIPTION
Uses cached nginx:1.27-alpine for the fixture and four independent client IPs.
No database, real authentication token, production edge, or published port is
used. Requires Docker Desktop running Linux containers and PowerShell 5.1+.
The 2 MiB/client workload takes about 2/4/8/8 seconds at the default 8mbit.
#>
[CmdletBinding()]
param(
    [string]$Rate = '8mbit',
    [ValidateRange(1500, 1048576)]
    [int]$BurstBytes = 32768,
    [string]$ReportPath,
    [switch]$SkipBuild,
    [string]$ImageName = 'pezmax-edge-egress:test'
)

$ErrorActionPreference = 'Stop'
$testsDirectory = $PSScriptRoot
$repositoryDirectory = Split-Path (Split-Path $testsDirectory -Parent) -Parent
$dockerfile = Join-Path $repositoryDirectory 'deploy/Dockerfile.edge'
$template = Join-Path $repositoryDirectory 'deploy/nginx.conf.template'
$headers = Join-Path $repositoryDirectory 'deploy/kadmin-proxy-headers.conf'
$fixtureConfig = Join-Path $testsDirectory 'fixture.conf'
$runId = 'pz-egress-' + [Guid]::NewGuid().ToString('N').Substring(0, 12)
$image = "${runId}:edge"
if ($SkipBuild) { $image = $ImageName }
$network = "$runId-network"
$controlDirectory = Join-Path ([IO.Path]::GetTempPath()) $runId
$containers = New-Object 'System.Collections.Generic.List[string]'
$networkCreated = $false
$imageBuilt = $false
$payloadBytes = 2 * 1024 * 1024
$culture = [Globalization.CultureInfo]::InvariantCulture
$report = [ordered]@{
    rate = $Rate
    burstBytes = $BurstBytes
    payloadBytesPerClient = $payloadBytes
    scenarios = @()
    startupFailures = @()
}

if ($Rate -notmatch '^(?<value>[1-9][0-9]*)(?<unit>bit|kbit|mbit|gbit)$') {
    throw 'Test Rate must use bit/kbit/mbit/gbit, for example 8mbit.'
}
$rateValue = [double]::Parse($Matches.value, $culture)
$multiplier = switch ($Matches.unit) {
    bit { 1 }
    kbit { 1000 }
    mbit { 1000000 }
    gbit { 1000000000 }
}
$rateBytesPerSecond = $rateValue * $multiplier / 8
if ($rateBytesPerSecond -lt 800000) {
    throw 'This short 2 MiB/client regression needs at least 6.4mbit; use the default 8mbit.'
}
if (-not $ReportPath) {
    $ReportPath = Join-Path $controlDirectory 'report.json'
}
$ReportPath = [IO.Path]::GetFullPath($ReportPath)

function Invoke-Docker {
    param([string[]]$Arguments)
    # Windows PowerShell 5.1 can treat successful native stderr (including
    # docker build progress) as a terminating error under Stop.
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = & docker @Arguments 2>&1 | Out-String
        $exitCode = $LASTEXITCODE
    }
    finally { $ErrorActionPreference = $previousPreference }
    if ($exitCode -ne 0) {
        throw "docker $($Arguments -join ' ') failed:`n$output"
    }
    return $output.Trim()
}

function Wait-ForFiles {
    param([string[]]$Files, [int]$TimeoutSeconds = 15)
    $watch = [Diagnostics.Stopwatch]::StartNew()
    while ($true) {
        $missing = @($Files | Where-Object { -not (Test-Path -LiteralPath $_) })
        if ($missing.Count -eq 0) { return }
        if ($watch.Elapsed.TotalSeconds -ge $TimeoutSeconds) {
            throw "Timed out waiting for $($missing -join ', ')."
        }
        Start-Sleep -Milliseconds 25
    }
}

function Wait-ForHttp {
    param([string]$Container, [string]$Url)
    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        $previousPreference = $ErrorActionPreference
        try {
            $ErrorActionPreference = 'Continue'
            & docker exec $Container wget -q -O /dev/null $Url 2>$null
            $exitCode = $LASTEXITCODE
        }
        finally { $ErrorActionPreference = $previousPreference }
        if ($exitCode -eq 0) { return }
        $running = Invoke-Docker @('inspect', '-f', '{{.State.Running}}', $Container)
        if ($running -ne 'true') {
            $logs = Invoke-Docker @('logs', $Container)
            throw "$Container stopped before HTTP was ready:`n$logs"
        }
        Start-Sleep -Milliseconds 100
    }
    throw "$Container did not become ready."
}

function Assert-StartupFails {
    param([string]$Suffix, [string]$InvalidRate, [bool]$AddCapability)
    $name = "$runId-$Suffix"
    $containers.Add($name)
    $arguments = @('run', '-d', '--name', $name, '--network', $network)
    if ($AddCapability) { $arguments += @('--cap-add', 'NET_ADMIN') }
    $arguments += @(
        '-e', 'KADMIN_UPSTREAM=fixture:80',
        '-e', "KADMIN_EDGE_EGRESS_RATE=$InvalidRate",
        '-e', "KADMIN_EDGE_EGRESS_BURST_BYTES=$BurstBytes",
        $image
    )
    Invoke-Docker $arguments | Out-Null
    for ($attempt = 0; $attempt -lt 50; $attempt++) {
        $state = (Invoke-Docker @('inspect', '-f', '{{json .State}}', $name)) | ConvertFrom-Json
        if (-not $state.Running) { break }
        Start-Sleep -Milliseconds 100
    }
    if ($state.Running -or $state.ExitCode -eq 0) {
        throw "$Suffix must stop with a nonzero exit code before serving HTTP."
    }
    $logs = Invoke-Docker @('logs', $name)
    if ($logs -match 'start worker process') {
        throw "$Suffix launched nginx workers without a working shaper."
    }
    $report.startupFailures += [ordered]@{ name = $Suffix; exitCode = $state.ExitCode; log = $logs }
    Write-Host "${Suffix}: failed closed (exit $($state.ExitCode))"
}

function Get-QdiscSamples {
    param([string]$Path)
    $samples = @()
    foreach ($line in Get-Content -LiteralPath $Path) {
        $fields = $line.Split('|', 2)
        # PS 5.1 keeps a top-level JSON array as one pipeline object; assign it
        # directly before piping so the root and child are enumerated alike.
        $qdiscs = ConvertFrom-Json -InputObject $fields[1]
        $root = @($qdiscs | Where-Object { $_.kind -eq 'tbf' -and $_.root })
        if ($root.Count -ne 1 -or $null -eq $root[0].bytes) {
            throw "Expected one root TBF with byte counters in $line"
        }
        $samples += [ordered]@{
            seconds = [double]::Parse($fields[0], $culture)
            bytes = [long]$root[0].bytes
        }
    }
    return $samples
}

function Assert-QdiscEnvelope {
    param([object[]]$Samples, [string]$Scenario)
    if ($Samples.Count -lt 3) { throw "$Scenario has too few qdisc samples." }
    # TBF permits its configured initial burst; allow 5% for sampling jitter
    # plus two Ethernet frames. Apply this to every approximately one-second
    # window, every approximately five-second window, and the complete run.
    $windows = @()
    for ($start = 0; $start -lt $Samples.Count - 1; $start++) {
        foreach ($target in @(1.0, 5.0)) {
            for ($end = $start + 1; $end -lt $Samples.Count; $end++) {
                if ($Samples[$end].seconds - $Samples[$start].seconds -ge $target) {
                    $windows += ,@($start, $end)
                    break
                }
            }
        }
    }
    $windows += ,@(0, ($Samples.Count - 1))
    foreach ($window in $windows) {
        $elapsed = $Samples[$window[1]].seconds - $Samples[$window[0]].seconds
        $sent = $Samples[$window[1]].bytes - $Samples[$window[0]].bytes
        $allowed = $rateBytesPerSecond * $elapsed * 1.05 + $BurstBytes + 3000
        if ($sent -gt $allowed) {
            throw "$Scenario exceeded the shared pool: $sent bytes/$elapsed s > $allowed allowed bytes."
        }
    }
    return $windows.Count
}

try {
    New-Item -ItemType Directory -Path $controlDirectory -Force | Out-Null
    $reportParent = Split-Path $ReportPath -Parent
    New-Item -ItemType Directory -Path $reportParent -Force | Out-Null
    if ($SkipBuild) {
        Invoke-Docker @('image', 'inspect', $image) | Out-Null
        Write-Host "Using existing image $image"
    }
    else {
        Write-Host "Building isolated test image $image"
        $buildArguments = @('build', '-t', $image, '-f', $dockerfile)
        if ($env:KADMIN_EDGE_APK_MIRROR) {
            $buildArguments += @('--build-arg', "ALPINE_MIRROR=$env:KADMIN_EDGE_APK_MIRROR")
        }
        $buildArguments += Join-Path $repositoryDirectory 'deploy'
        Invoke-Docker $buildArguments | Out-Null
        $imageBuilt = $true
    }
    Invoke-Docker @('network', 'create', $network) | Out-Null
    $networkCreated = $true

    $fixture = "$runId-fixture"
    $containers.Add($fixture)
    Invoke-Docker @(
        'run', '-d', '--name', $fixture, '--network', $network, '--network-alias', 'fixture',
        '--mount', "type=bind,source=$fixtureConfig,target=/etc/nginx/conf.d/default.conf,readonly",
        '--mount', "type=bind,source=$testsDirectory,target=/tests,readonly",
        '--tmpfs', '/fixtures:rw,size=8m', '--entrypoint', 'sh', 'nginx:1.27-alpine',
        '/tests/fixture-start.sh'
    ) | Out-Null
    Wait-ForHttp $fixture 'http://127.0.0.1/health'

    Assert-StartupFails 'missing-net-admin' $Rate $false
    Assert-StartupFails 'invalid-rate' 'not-a-rate' $true
    Assert-StartupFails 'zero-rate' '0mbit' $true

    $edge = "$runId-edge"
    $containers.Add($edge)
    Invoke-Docker @(
        'run', '-d', '--name', $edge, '--network', $network, '--network-alias', 'edge',
        '--cap-add', 'NET_ADMIN',
        '-e', 'KADMIN_UPSTREAM=fixture:80',
        '-e', "KADMIN_EDGE_EGRESS_RATE=$Rate",
        '-e', "KADMIN_EDGE_EGRESS_BURST_BYTES=$BurstBytes",
        '-e', 'KADMIN_EDGE_DL_RATE_FAST=5m', '-e', 'KADMIN_EDGE_DL_RATE_MID=2560k',
        '-e', 'KADMIN_EDGE_DL_RATE_LOW=640k', '-e', 'KADMIN_EDGE_DL_MAXCONN=16',
        '--mount', "type=bind,source=$template,target=/etc/nginx/templates/default.conf.template,readonly",
        '--mount', "type=bind,source=$headers,target=/etc/nginx/kadmin-proxy-headers.conf,readonly",
        '--mount', "type=bind,source=$testsDirectory,target=/tests,readonly",
        '--mount', "type=bind,source=$controlDirectory,target=/test-control",
        $image
    ) | Out-Null
    Wait-ForHttp $edge 'http://127.0.0.1/stub_status'
    $report.qdiscConfiguration = Invoke-Docker @('exec', $edge, 'tc', '-j', '-s', 'qdisc', 'show', 'dev', 'eth0')

    $clients = @()
    for ($client = 1; $client -le 4; $client++) {
        $name = "$runId-client-$client"
        $containers.Add($name)
        $clients += $name
        Invoke-Docker @(
            'run', '-d', '--name', $name, '--network', $network,
            '--mount', "type=bind,source=$testsDirectory,target=/tests,readonly",
            '--mount', "type=bind,source=$controlDirectory,target=/test-control",
            '--entrypoint', 'sh', 'nginx:1.27-alpine', '-c', 'sleep 3600'
        ) | Out-Null
    }

    foreach ($scenario in @(
        @{ name = 'single'; count = 1; mixed = $false },
        @{ name = 'two-tokens'; count = 2; mixed = $false },
        @{ name = 'four-tokens'; count = 4; mixed = $false },
        @{ name = 'mixed-routes'; count = 4; mixed = $true }
    )) {
        $name = $scenario.name
        $readyFiles = @()
        $doneFiles = @()
        for ($client = 1; $client -le $scenario.count; $client++) {
            $url = "http://edge/datum/download/file?fileId=fixture-$client"
            if ($scenario.mixed -and $client % 2 -eq 0) { $url = 'http://edge/ordinary/payload.bin' }
            Invoke-Docker @('exec', '-d', $clients[$client - 1], 'sh', '/tests/download-client.sh', $name, "$client", $url) | Out-Null
            $readyFiles += Join-Path $controlDirectory "$name-client-$client.ready"
            $doneFiles += Join-Path $controlDirectory "$name-client-$client.done"
        }
        Invoke-Docker @('exec', '-d', $edge, 'sh', '/tests/monitor-egress.sh', $name) | Out-Null
        $readyFiles += Join-Path $controlDirectory "$name-monitor.ready"
        Wait-ForFiles $readyFiles
        $watch = [Diagnostics.Stopwatch]::StartNew()
        Set-Content -LiteralPath (Join-Path $controlDirectory "$name.start") -Value 'start'
        Wait-ForFiles $doneFiles
        $watch.Stop()
        Set-Content -LiteralPath (Join-Path $controlDirectory "$name.stop") -Value 'stop'
        Wait-ForFiles @((Join-Path $controlDirectory "$name-monitor.done"))

        $clientResults = @()
        foreach ($file in $doneFiles) {
            $fields = (Get-Content -LiteralPath $file -Raw).Trim().Split('|')
            $log = Get-Content -LiteralPath ($file -replace '\.done$', '.log') -Raw
            if ([int]$fields[0] -ne 0 -or [long]$fields[1] -ne $payloadBytes -or $log -notmatch 'HTTP/1\.[01] 200') {
                throw "$name failed to receive the fixture with HTTP 200: $file`n$log"
            }
            $clientResults += [ordered]@{
                bytes = [long]$fields[1]
                startSeconds = [double]::Parse($fields[2], $culture)
                endSeconds = [double]::Parse($fields[3], $culture)
            }
        }
        $totalBytes = $payloadBytes * $scenario.count
        $elapsed = $watch.Elapsed.TotalSeconds
        $allowed = $rateBytesPerSecond * $elapsed * 1.05 + $BurstBytes + 3000
        $observedRate = $totalBytes / $elapsed
        if ($totalBytes -gt $allowed) { throw "$name downloads exceeded the pool: $totalBytes bytes/$elapsed s." }
        if ($observedRate -lt $rateBytesPerSecond * 0.70) {
            throw "$name did not saturate the pool ($observedRate bytes/s); slow/error responses could mask a regression."
        }
        $samples = @(Get-QdiscSamples (Join-Path $controlDirectory "$name-tc.samples"))
        $checkedWindows = Assert-QdiscEnvelope $samples $name
        $report.scenarios += [ordered]@{
            name = $name
            connections = $scenario.count
            bytes = $totalBytes
            elapsedSeconds = $elapsed
            bytesPerSecond = $observedRate
            checkedQdiscWindows = $checkedWindows
            clients = $clientResults
            qdiscSamples = $samples
        }
        Write-Host ("{0}: {1:N0} bytes/s total across {2} client(s), {3} qdisc windows passed" -f $name, $observedRate, $scenario.count, $checkedWindows)
    }
    $report.passed = $true
}
catch {
    $report.passed = $false
    $report.failure = $_.Exception.Message
    throw
}
finally {
    $report | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
    $ErrorActionPreference = 'Continue'
    foreach ($container in $containers) {
        & docker rm -f $container 2>$null | Out-Null
    }
    if ($networkCreated) { & docker network rm $network 2>$null | Out-Null }
    if ($imageBuilt) { & docker image rm $image 2>$null | Out-Null }
    # Keep the unique temporary control directory and logs for diagnosis.
    # Only this run's named containers/network/image are removed.
    Write-Host "Report: $ReportPath"
}

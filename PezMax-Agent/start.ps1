#Requires -Version 5.1
<#
.SYNOPSIS
    PezMax Agent 一键初始化 / 自检 / 启动。

.DESCRIPTION
    默认只做自检（不联网、不需要密钥）。-Serve 启动带令牌的 FastAPI 服务（127.0.0.1:9044），
    -Desktop 启动桌面端开发模式（Electron 通过 stdio 拉起同目录 .venv 的智能体）。

.EXAMPLE
    .\start.ps1              # 自检：虚拟环境、依赖、LangGraph 状态图
    .\start.ps1 -Serve       # 启动 API 服务 http://127.0.0.1:9044/docs
    .\start.ps1 -Desktop     # 启动桌面端（自动调用 stdio 智能体）
    .\start.ps1 -Install     # 只初始化虚拟环境与依赖
#>
[CmdletBinding()]
param(
    [switch]$Install,
    [switch]$Serve,
    [switch]$Desktop,
    [int]$Port = 9044,
    [string]$ProviderUrl,
    [string]$CatalogUrl
)

$ErrorActionPreference = 'Stop'
$env:PYTHONUTF8 = '1'
$env:PYTHONIOENCODING = 'utf-8'

$Root = $PSScriptRoot
$IsWindowsHost = -not $IsLinux -and -not $IsMacOS
$CorePython = Join-Path $Root $(if ($IsWindowsHost) { '.venv\Scripts\python.exe' } else { '.venv/bin/python' })
$ServicePython = Join-Path $Root $(if ($IsWindowsHost) { '.venv-service\Scripts\python.exe' } else { '.venv-service/bin/python' })

function Write-Step($Text) { Write-Host "==> $Text" -ForegroundColor Cyan }
function Write-Note($Text) { Write-Host "    $Text" -ForegroundColor DarkGray }

function New-Venv([string]$Target) {
    # 部分 Windows 安装缺少 stdlib venv 模块，回退到 virtualenv。
    $bootstrap = if ($env:PEZMAX_AGENT_PYTHON) { $env:PEZMAX_AGENT_PYTHON } elseif ($IsWindowsHost) { 'python' } else { 'python3' }
    & $bootstrap -m venv $Target
    if ($LASTEXITCODE -eq 0) { return }
    Write-Note "基座解释器不支持 venv，改用 virtualenv。"
    & $bootstrap -m virtualenv $Target
    if ($LASTEXITCODE -ne 0) { throw "无法创建 $Target；请安装 Python 3.11+ 或执行 $bootstrap -m pip install virtualenv。" }
}

function Initialize-Environment {
    if (-not (Test-Path $CorePython)) {
        Write-Step '创建 .venv'
        New-Venv (Join-Path $Root '.venv')
    }
    Write-Step '安装核心依赖（stdio 智能体 + 打包工具）'
    Push-Location $Root
    try { & $CorePython -m pip install -r requirements.lock -e '.[build]' } finally { Pop-Location }
    if ($LASTEXITCODE -ne 0) { throw '核心依赖安装失败。' }

    if (-not (Test-Path $ServicePython)) {
        Write-Step '创建 .venv-service'
        New-Venv (Join-Path $Root '.venv-service')
    }
    Write-Step '安装服务依赖（FastAPI + MCP）'
    Push-Location $Root
    try { & $ServicePython -m pip install -r service-requirements.lock -e '.[service]' } finally { Pop-Location }
    if ($LASTEXITCODE -ne 0) { throw '服务依赖安装失败。' }
}

function Invoke-Check {
    if (-not (Test-Path $CorePython)) { throw '缺少 .venv；先执行 .\start.ps1 -Install。' }
    Write-Step '自检核心运行时'
    Push-Location $Root
    try { & $CorePython -m pezmax_agent --check } finally { Pop-Location }
    if ($LASTEXITCODE -ne 0) { throw '智能体自检失败。' }
}

function Get-ServiceToken {
    $dataDir = Join-Path $Root 'data'
    if (-not (Test-Path $dataDir)) { New-Item -ItemType Directory -Path $dataDir | Out-Null }
    $tokenFile = Join-Path $dataDir 'service-token.txt'
    if (Test-Path $tokenFile) {
        $token = (Get-Content -Raw $tokenFile).Trim()
        if ($token) { return $token }
    }
    $token = (& $ServicePython -c 'import secrets; print(secrets.token_urlsafe(32))').Trim()
    Set-Content -Path $tokenFile -Value $token -Encoding ascii -NoNewline
    Write-Note "已生成访问令牌并保存到 data/service-token.txt"
    return $token
}

function Start-Service {
    Invoke-Check
    if (-not (Test-Path $ServicePython)) { throw '缺少 .venv-service；先执行 .\start.ps1 -Install。' }

    if ($IsWindowsHost -and (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue)) {
        $existing = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        if ($existing) { throw "端口 $Port 已被占用（PID $($existing.OwningProcess -join ',')）。先停止旧服务或使用 -Port 换端口。" }
    }

    if (-not $env:PEZMAX_AGENT_ACCESS_TOKENS) {
        $token = Get-ServiceToken
        $env:PEZMAX_AGENT_ACCESS_TOKENS = (@{ local = $token } | ConvertTo-Json -Compress)
    }
    else {
        $token = ($env:PEZMAX_AGENT_ACCESS_TOKENS | ConvertFrom-Json).PSObject.Properties.Value | Select-Object -First 1
    }
    if (-not $env:PEZMAX_AGENT_DB) {
        $env:PEZMAX_AGENT_DB = Join-Path $Root 'data\knowledge.sqlite3'
    }
    if ($ProviderUrl) { $env:PEZMAX_AGENT_PROVIDER_URLS = (ConvertTo-Json -InputObject @($ProviderUrl) -Compress) }
    if ($CatalogUrl) { $env:PEZMAX_AGENT_CATALOG_URL = $CatalogUrl }

    Write-Step "启动 API 服务 http://127.0.0.1:$Port/docs"
    Write-Note "Bearer 令牌：$token"
    Write-Note "知识库：$env:PEZMAX_AGENT_DB"
    if (-not $env:PEZMAX_AGENT_PROVIDER_URLS -or $env:PEZMAX_AGENT_PROVIDER_URLS -eq '[]') {
        Write-Note '未配置模型白名单（PEZMAX_AGENT_PROVIDER_URLS），/v1/run 与 /ask 会拒绝该模型地址；用 -ProviderUrl 指定。'
    }
    Push-Location $Root
    try { & $ServicePython -m pezmax_agent.server --port $Port } finally { Pop-Location }
}

function Start-Desktop {
    if (-not (Test-Path $CorePython)) { throw '缺少 .venv；先执行 .\start.ps1 -Install。' }
    $desktop = Join-Path (Split-Path $Root -Parent) 'PezMax-Desktop'
    if (-not (Test-Path $desktop)) { throw '未找到 PezMax-Desktop 目录。' }
    Write-Step '启动桌面端开发模式（Electron 通过 stdio 拉起智能体）'
    Push-Location $desktop
    try { & npm run dev } finally { Pop-Location }
}

$modes = @(@(
    @{ On = $Install; Run = { Initialize-Environment; Invoke-Check } },
    @{ On = $Serve; Run = { Start-Service } },
    @{ On = $Desktop; Run = { Start-Desktop } }
) | Where-Object { $_.On })

if ($modes.Count -gt 1) { throw '一次只能选择一个模式：-Install / -Serve / -Desktop。' }
if ($modes.Count -eq 0) { Invoke-Check; Write-Note '完成。启动服务：.\start.ps1 -Serve    启动桌面端：.\start.ps1 -Desktop' }
else { & $modes[0].Run }

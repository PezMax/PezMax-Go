# Edge shared egress regression

Run from `PezMax-Go` with Docker Desktop using Linux containers:

```powershell
pwsh -NoProfile -File ./deploy/tests/test-edge-egress.ps1
```

The script builds `deploy/Dockerfile.edge` into a unique temporary tag. To use
an image that has already been built:

```powershell
pwsh -NoProfile -File ./deploy/tests/test-edge-egress.ps1 -SkipBuild -ImageName pezmax-edge-egress:test
```

It creates a private Docker network, a binary-file fixture, the production
Nginx template and shaper, and four independent client containers. Each client
has its own IP and sends a different Bearer header, which the fixture checks.
The fixture does not use production authentication, storage or databases.

Four scenarios exercise one, two and four simultaneous downloads, then four
mixed download/ordinary-path responses. All responses must return HTTP 200 and
the full 2 MiB fixture. The combined body rate must be at least 70% of the
configured budget so failed requests or an artificially slow fixture cannot
produce a passing result. Both the combined body rate and the root TBF's sent
byte counter must fit `rate * actual elapsed * 1.05 + burst + 3000 bytes`.
The qdisc checks cover all approximately one-second and five-second windows,
including the initial burst, as well as the complete run. Token bucket shaping
allows the configured finite burst; the test does not assume every millisecond
has an identical rate.

Missing `NET_ADMIN`, invalid rate and zero rate must stop the container with a
nonzero exit code before Nginx workers are launched.

At the default `8mbit` budget the downloads take about 22 seconds in total.
Image build time is additional. Only resources with this run's unique prefix
are removed in `finally`; no port is published and the running production edge
is untouched. A temporary directory retains the JSON report, qdisc samples
and client logs. Pass `-ReportPath` to choose a report location.

The build uses the same optional mirror setting as Compose. If the default
Alpine package endpoint is unreachable, set `$env:KADMIN_EDGE_APK_MIRROR` to an
HTTPS Alpine mirror before running the script; it is passed as `ALPINE_MIRROR`
and only changes package download during the build.

param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker is required to run the isolated test suite."
}

& docker info --format "{{.ServerVersion}}" *> $null
if ($LASTEXITCODE -ne 0) {
    throw "The Docker engine is unavailable. Start Docker Desktop or Docker Engine and retry."
}

if (-not $GoTestArgs -or $GoTestArgs.Count -eq 0) {
    $GoTestArgs = @("./...")
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

docker run --rm `
    -v "${RepoRoot}:/workspace" `
    -v "hs-cli-go-mod-cache:/go/pkg/mod" `
    -v "hs-cli-go-build-cache:/tmp/go-cache" `
    -w /workspace `
    -e HOME=/tmp/hs-test-home `
    -e USERPROFILE=/tmp/hs-test-home `
    -e XDG_CONFIG_HOME=/tmp/hs-test-home/.config `
    -e APPDATA=/tmp/hs-test-home/AppData `
    -e GOCACHE=/tmp/go-cache `
    -e GOMODCACHE=/go/pkg/mod `
    -e GOTELEMETRY=off `
    golang:1.25.9-bookworm@sha256:298734aec230b5f3e8cee450ce6d7eccc39f1797ba548ee90d57e9803030c6c3 `
    go test @GoTestArgs

exit $LASTEXITCODE

param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

$ErrorActionPreference = "Stop"

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
    golang:1.25 `
    go test @GoTestArgs

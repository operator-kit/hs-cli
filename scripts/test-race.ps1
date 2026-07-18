param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

$ErrorActionPreference = "Stop"

if (-not $GoTestArgs -or $GoTestArgs.Count -eq 0) {
    $GoTestArgs = @("./...")
}

$dockerTestScript = Join-Path $PSScriptRoot "test-docker.ps1"
& $dockerTestScript "-race" @GoTestArgs
exit $LASTEXITCODE

param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if (-not $GoTestArgs -or $GoTestArgs.Count -eq 0) {
    $GoTestArgs = @("./...")
}

if ($env:OS -ne "Windows_NT") {
    throw "This native race wrapper supports Windows only. Use scripts/test-race.sh for the Docker-based suite."
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is required to run the native race test suite."
}

$goOS = (& go env GOOS).Trim()
$goArch = (& go env GOARCH).Trim()
if ($LASTEXITCODE -ne 0 -or $goOS -ne "windows" -or $goArch -ne "amd64") {
    throw "Native Windows race tests require GOOS=windows and GOARCH=amd64; found $goOS/$goArch."
}

$gcc = Get-Command gcc -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $gcc) {
    throw "A recent MinGW-w64 GCC is required for native Windows race tests. Use .\scripts\test-race.ps1 for the Docker suite, or install a compiler that meets Go's Windows race-detector requirements."
}

$synchronizationLibrary = [string](& $gcc.Source --print-file-name=libsynchronization.a)
$synchronizationLibrary = $synchronizationLibrary.Trim()
if (
    $LASTEXITCODE -ne 0 -or
    $synchronizationLibrary -eq "libsynchronization.a" -or
    -not (Test-Path -LiteralPath $synchronizationLibrary -PathType Leaf)
) {
    throw "The installed GCC does not provide the mingw-w64 synchronization library required by Go's Windows race detector."
}

$previousCGO = [Environment]::GetEnvironmentVariable("CGO_ENABLED", "Process")
$previousCC = [Environment]::GetEnvironmentVariable("CC", "Process")
$exitCode = 1

try {
    [Environment]::SetEnvironmentVariable("CGO_ENABLED", "1", "Process")
    [Environment]::SetEnvironmentVariable("CC", "gcc", "Process")

    & go test -race @GoTestArgs
    $exitCode = $LASTEXITCODE
} finally {
    [Environment]::SetEnvironmentVariable("CGO_ENABLED", $previousCGO, "Process")
    [Environment]::SetEnvironmentVariable("CC", $previousCC, "Process")
}

exit $exitCode

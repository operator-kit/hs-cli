#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

$Repo = "operator-kit/hs-cli"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:USERPROFILE ".local\bin" }

# Detect arch
$Arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'X64'  { 'amd64' }
    default { Write-Error "Unsupported architecture: $_"; exit 1 }
}

# Resolve version
$Version = $env:HS_VERSION
if (-not $Version) {
    $response = Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest" -MaximumRedirection 0 -ErrorAction SilentlyContinue -UseBasicParsing
    $Version = ($response.Headers.Location -split '/tag/')[-1]
    if (-not $Version) {
        Write-Error "Could not determine latest version. Set HS_VERSION manually."
        exit 1
    }
}

$VersionNum = $Version -replace '^v', ''
$Archive = "hs_${VersionNum}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Archive"
$ChecksumsUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"

Write-Host "Installing hs $Version (windows/$Arch)..."

# Download to temp
$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null
try {
    $archivePath = Join-Path $TmpDir $Archive
    $checksumsPath = Join-Path $TmpDir "checksums.txt"

    Invoke-WebRequest -Uri $Url -OutFile $archivePath -UseBasicParsing
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $checksumsPath -UseBasicParsing

    # Verify checksum
    $expected = (Get-Content $checksumsPath | Where-Object { $_ -match $Archive }) -split '\s+' | Select-Object -First 1
    $actual = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()
    if ($expected -ne $actual) {
        Write-Error "Checksum mismatch! Expected: $expected, Got: $actual"
        exit 1
    }

    # Extract and install
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Expand-Archive -Path $archivePath -DestinationPath $TmpDir -Force
    Copy-Item (Join-Path $TmpDir "hs.exe") (Join-Path $InstallDir "hs.exe") -Force

    # Add to user PATH if not already present
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable('Path', "$InstallDir;$userPath", 'User')
        $env:Path = "$InstallDir;$env:Path"
        Write-Host "Added $InstallDir to user PATH (restart terminal for other sessions)"
    }

    Write-Host "Installed hs to $InstallDir"
    & (Join-Path $InstallDir "hs.exe") version
} finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}

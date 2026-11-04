[CmdletBinding()]
param(
    [Parameter(Position = 0)][string]$Version,
    [string]$InstallDir = $env:MIOCLI_INSTALL_DIR,
    [switch]$Yes,
    [switch]$NonInteractive,
    [switch]$Setup,
    [switch]$Help
)

$ErrorActionPreference = "Stop"
if ($Help) {
    Write-Output "Usage: install.ps1 [-Version VERSION] [-InstallDir DIR] [-Yes] [-NonInteractive] [-Setup] [-Help]"
    Write-Output "Install only miocli. PATH and existing Mihomo/Clash Verge settings are never modified."
    Write-Output "MIOCLI_INSTALL_DIR overrides the user install directory. -Yes confirms CLI replacement only."
    return
}
$repository = "__MIOCLI_REPOSITORY__"
if ($env:MIOCLI_REPOSITORY) { $repository = $env:MIOCLI_REPOSITORY }
if ($repository -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') {
    throw "Installer repository is not configured."
}
if (-not $InstallDir) {
    if (-not $env:LOCALAPPDATA) { throw "LOCALAPPDATA is not set." }
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\miocli"
}
if (-not [System.IO.Path]::IsPathRooted($InstallDir)) {
    throw "Installation directory must be an absolute path."
}
$interactive = -not $NonInteractive -and -not [Console]::IsInputRedirected -and -not [Console]::IsOutputRedirected
if ($Setup -and -not $interactive) {
    throw "-Setup requires an interactive terminal; install first, then run miocli setup."
}
$target = Join-Path $InstallDir "miocli.exe"
if (Test-Path -LiteralPath $target -PathType Container) { throw "Install target is a directory." }
if (Test-Path -LiteralPath $target) {
    Write-Output "Existing CLI: $target"
    if (-not $Yes) {
        if (-not $interactive) { throw "Replacement requires -Yes in non-interactive mode." }
        if ((Read-Host "Replace this CLI binary? [y/N]") -notmatch '^(y|yes)$') {
            throw "Installation cancelled; existing CLI unchanged."
        }
    }
}
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$architectureCode = (Get-CimInstance Win32_Processor | Select-Object -First 1 -ExpandProperty Architecture)
switch ($architectureCode) {
    9 { $architecture = "amd64" }
    12 { $architecture = "arm64" }
    default { throw "Unsupported Windows processor architecture: $architectureCode" }
}
if (-not $Version) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repository/releases/latest" -TimeoutSec 30
    $Version = $release.tag_name
}
if ($Version -notmatch '^v?[0-9]+\.[0-9]+\.[0-9]+([+-][A-Za-z0-9.-]+)?$') {
    throw "Version must be a semantic version such as v1.2.3."
}
if (-not $Version.StartsWith("v")) { $Version = "v$Version" }
$versionNumber = $Version.Substring(1)
$archive = "miocli_${versionNumber}_windows_${architecture}.zip"
$baseUrl = "https://github.com/$repository/releases/download/$Version"
Write-Output "Install miocli $Version (windows/$architecture) to $target"
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
$temporaryDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("miocli-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
$staged = Join-Path $InstallDir (".miocli-install-" + [Guid]::NewGuid().ToString("N") + ".exe")
try {
    New-Item -ItemType File -Path $staged | Out-Null
    $archivePath = Join-Path $temporaryDirectory $archive
    $checksumPath = "$archivePath.sha256"
    Invoke-WebRequest -Uri "$baseUrl/$archive" -OutFile $archivePath -TimeoutSec 180
    Invoke-WebRequest -Uri "$baseUrl/$archive.sha256" -OutFile $checksumPath -TimeoutSec 60
    $checksumLine = Get-Content -LiteralPath $checksumPath -Raw
    if ($checksumLine -notmatch '^\s*([0-9a-fA-F]{64})\s+(.+?)\s*$' -or $Matches[2] -ne $archive) {
        throw "Release checksum file is invalid."
    }
    $expectedHash = $Matches[1].ToLowerInvariant()
    $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) { throw "Checksum verification failed; existing CLI unchanged." }
    $extractDirectory = Join-Path $temporaryDirectory "extract"
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDirectory
    $packageDirectory = Join-Path $extractDirectory "miocli_${versionNumber}_windows_${architecture}"
    $binaryPath = Join-Path $packageDirectory "miocli.exe"
    if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) {
        throw "Release archive does not contain miocli.exe."
    }
    Copy-Item -LiteralPath $binaryPath -Destination $staged -Force
    try {
        if (Test-Path -LiteralPath $target) {
            [System.IO.File]::Replace($staged, $target, $null)
        } else {
            [System.IO.File]::Move($staged, $target)
        }
    } catch {
        throw "Could not replace CLI at $target. Close running miocli processes and retry. $($_.Exception.Message)"
    }
    Write-Output "Installed miocli $Version to $target"
    Write-Output "Only miocli was installed. Existing Clash Verge/Mihomo users do not need a second core."
    if (($env:Path -split ';') -notcontains $InstallDir) {
        $quotedDir = $InstallDir.Replace("'", "''")
        Write-Output "PATH was not modified. For this PowerShell session:"
        Write-Output ("  `$env:Path = '" + $quotedDir + ";' + `$env:Path")
        Write-Output "For persistence, add $InstallDir to your user PATH in Windows Environment Variables and open a new terminal."
    }
    Write-Output "Next: & '$($target.Replace("'", "''"))' setup"
    Write-Output "Check: & '$($target.Replace("'", "''"))' doctor"
    if ($interactive -and -not $Setup) {
        $Setup = (Read-Host "Run connection setup now? [y/N]") -match '^(y|yes)$'
    }
    if ($Setup) {
        & $target setup
        if ($LASTEXITCODE -ne 0) { throw "CLI installed, but setup did not complete; rerun setup." }
    }
} finally {
    if (Test-Path -LiteralPath $staged) { Remove-Item -LiteralPath $staged -Force }
    Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
}

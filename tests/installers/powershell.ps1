$ErrorActionPreference = "Stop"
$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$work = Join-Path ([System.IO.Path]::GetTempPath()) ("miocli-installer-test-" + [Guid]::NewGuid().ToString("N"))
$originalRepository = $env:MIOCLI_REPOSITORY
$originalInstallDir = $env:MIOCLI_INSTALL_DIR
$originalLocalAppData = $env:LOCALAPPDATA
$originalPath = $env:Path
$originalTestArchive = $env:INSTALLER_TEST_ARCHIVE
$originalTestChecksum = $env:INSTALLER_TEST_CHECKSUM
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
New-Item -ItemType Directory -Path $work | Out-Null
try {
    $env:MIOCLI_REPOSITORY = "example/miocli"
    $env:MIOCLI_INSTALL_DIR = Join-Path $work "install with spaces"
    $env:LOCALAPPDATA = Join-Path $work "local"
    $architectureCode = (Get-CimInstance Win32_Processor | Select-Object -First 1 -ExpandProperty Architecture)
    if ($architectureCode -eq 12) { $architecture = "arm64" } else { $architecture = "amd64" }
    $name = "miocli_1.2.3_windows_$architecture"
    $package = Join-Path $work $name
    New-Item -ItemType Directory -Path $package | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $package "miocli.exe"), "new-cli")
    $env:INSTALLER_TEST_ARCHIVE = Join-Path $work "$name.zip"
    Compress-Archive -LiteralPath $package -DestinationPath $env:INSTALLER_TEST_ARCHIVE
    $env:INSTALLER_TEST_CHECKSUM = (Get-FileHash -LiteralPath $env:INSTALLER_TEST_ARCHIVE -Algorithm SHA256).Hash.ToLowerInvariant() + "  $name.zip"
    function Invoke-RestMethod { param($Uri, $TimeoutSec) return @{ tag_name = "v1.2.3" } }
    function Invoke-WebRequest {
        param($Uri, $OutFile, $TimeoutSec)
        if ($Uri.EndsWith(".sha256")) {
            [System.IO.File]::WriteAllText($OutFile, $env:INSTALLER_TEST_CHECKSUM)
        } elseif ($Uri.EndsWith(".zip")) {
            Copy-Item -LiteralPath $env:INSTALLER_TEST_ARCHIVE -Destination $OutFile
        } else { throw "Unexpected URL: $Uri" }
    }
    function Assert-True($condition, $message) { if (-not $condition) { throw $message } }
    function Expect-Failure([scriptblock]$action, [string]$message) {
        try { & $action; throw "Expected installer failure did not occur" }
        catch { if ($_.Exception.Message -notlike "*$message*") { throw } }
    }
    $installer = Join-Path $root "install.ps1"
    & $installer -Help | Out-Null
    & $installer -NonInteractive | Out-Null
    $target = Join-Path $env:MIOCLI_INSTALL_DIR "miocli.exe"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "new-cli") "Fresh install failed"
    [System.IO.File]::WriteAllText($target, "old-cli")
    Expect-Failure { & $installer -Version 1.2.3 -NonInteractive | Out-Null } "requires -Yes"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "old-cli") "Replaced without confirmation"
    & $installer -Version 1.2.3 -Yes -NonInteractive | Out-Null
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "new-cli") "Confirmed replacement failed"
    [System.IO.File]::WriteAllText($target, "old-cli")
    $env:INSTALLER_TEST_CHECKSUM = ("0" * 64) + "  $name.zip"
    Expect-Failure { & $installer -Version 1.2.3 -Yes -NonInteractive | Out-Null } "Checksum verification failed"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "old-cli") "Checksum failure lost old binary"
    Expect-Failure { & $installer -Version 1.2.3 -Setup -NonInteractive | Out-Null } "requires an interactive terminal"
    Expect-Failure { & $installer -InstallDir relative -NonInteractive | Out-Null } "absolute path"
    Assert-True ($env:Path -eq $originalPath) "Installer changed process PATH"
    Assert-True ([Environment]::GetEnvironmentVariable("Path", "User") -eq $userPath) "Installer changed persistent PATH"
    Write-Output "PowerShell installer behavior tests passed."
} finally {
    $env:MIOCLI_REPOSITORY = $originalRepository
    $env:MIOCLI_INSTALL_DIR = $originalInstallDir
    $env:LOCALAPPDATA = $originalLocalAppData
    $env:INSTALLER_TEST_ARCHIVE = $originalTestArchive
    $env:INSTALLER_TEST_CHECKSUM = $originalTestChecksum
    Remove-Item -LiteralPath $work -Recurse -Force
}

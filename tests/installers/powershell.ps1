$ErrorActionPreference = "Stop"
$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$work = Join-Path ([System.IO.Path]::GetTempPath()) ("miocli-installer-test-" + [Guid]::NewGuid().ToString("N"))
$originalRepository = $env:MIOCLI_REPOSITORY
$originalInstallDir = $env:MIOCLI_INSTALL_DIR
$originalLocalAppData = $env:LOCALAPPDATA
$originalPath = $env:Path
$originalTestArchive = $env:INSTALLER_TEST_ARCHIVE
$originalTestChecksum = $env:INSTALLER_TEST_CHECKSUM
$originalScenario = $env:INSTALLER_TEST_SCENARIO
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
New-Item -ItemType Directory -Path $work | Out-Null
try {
    if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
        function Get-CimInstance { param($ClassName) return @{ Architecture = 12 } }
    }
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
    $script:requests = [System.Collections.Generic.List[string]]::new()
    $global:miocliInstallerTestRequests = $script:requests
    $env:INSTALLER_TEST_SCENARIO = ""
    function Fail-Transport {
        throw [System.Net.WebException]::new("test network unavailable", [System.Net.WebExceptionStatus]::ConnectFailure)
    }
    function Invoke-RestMethod {
        param($Uri, $TimeoutSec, $ConnectionTimeoutSeconds, $OperationTimeoutSeconds)
        $global:miocliInstallerTestRequests.Add($Uri)
        if ($OperationTimeoutSeconds) {
            $TimeoutSec = $OperationTimeoutSeconds
            if ($ConnectionTimeoutSeconds -ne 5) { throw "Wrong GitHub metadata connect timeout" }
        }
        if ($TimeoutSec -ne 15) { throw "Wrong GitHub metadata timeout" }
        if ($env:INSTALLER_TEST_SCENARIO -eq "latest") { Fail-Transport }
        if ($env:INSTALLER_TEST_SCENARIO -eq "badlatest") { return @{ tag_name = "../invalid" } }
        if ($env:INSTALLER_TEST_SCENARIO -eq "trust") { throw [System.Security.Authentication.AuthenticationException]::new("test TLS trust failure") }
        if ($env:INSTALLER_TEST_SCENARIO -eq "cancelled") { throw [System.OperationCanceledException]::new("test user cancellation") }
        if ($env:INSTALLER_TEST_SCENARIO -eq "lag") { return @{ tag_name = "v1.2.4" } }
        return @{ tag_name = "v1.2.3" }
    }
    function Invoke-WebRequest {
        param($Uri, $OutFile, $TimeoutSec, $ConnectionTimeoutSeconds, $OperationTimeoutSeconds, $MaximumRedirection, [switch]$UseBasicParsing)
        $global:miocliInstallerTestRequests.Add($Uri)
        if (-not $Uri.StartsWith("https://github.com/example/miocli/releases/download/")) { throw "Unexpected URL: $Uri" }
        if ($OperationTimeoutSeconds) {
            $TimeoutSec = $OperationTimeoutSeconds
            if ($ConnectionTimeoutSeconds -ne 8) { throw "Wrong package connect timeout" }
        }
        if ($TimeoutSec -ne 30 -or $MaximumRedirection -ne 5) { throw "Wrong GitHub package request bounds" }
        if ($env:INSTALLER_TEST_SCENARIO -eq "lag" -and $Uri.Contains("/v1.2.4/")) { Fail-Transport }
        if ($env:INSTALLER_TEST_SCENARIO -eq "archive" -and $Uri.EndsWith(".zip")) {
            [System.IO.File]::WriteAllText($OutFile, "partial")
            Fail-Transport
        }
        if ($env:INSTALLER_TEST_SCENARIO -eq "checksum" -and $Uri.EndsWith(".sha256")) { Fail-Transport }
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
    Assert-True ($script:requests.Count -eq 3) "Unexpected fresh-install requests"
    Assert-True (@($script:requests | Where-Object { $_ -notlike "https://*github.com/*" }).Count -eq 0) "Unexpected download source"
    $target = Join-Path $env:MIOCLI_INSTALL_DIR "miocli.exe"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "new-cli") "Fresh install failed"
    [System.IO.File]::WriteAllText($target, "old-cli")
    Expect-Failure { & $installer -Version 1.2.3 -NonInteractive | Out-Null } "requires -Yes"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "old-cli") "Replaced without confirmation"
    & $installer -Version 1.2.3 -Yes -NonInteractive | Out-Null
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "new-cli") "Confirmed replacement failed"
    [System.IO.File]::WriteAllText($target, "old-cli")
    $env:INSTALLER_TEST_CHECKSUM = ("0" * 64) + "  $name.zip"
    $script:requests.Clear()
    Expect-Failure { & $installer -Version 1.2.3 -Yes -NonInteractive | Out-Null } "Checksum verification failed"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "old-cli") "Checksum failure lost old binary"
    Assert-True ($script:requests.Count -eq 2) "Checksum failure made extra requests"
    Expect-Failure { & $installer -Version 1.2.3 -Setup -NonInteractive | Out-Null } "requires an interactive terminal"
    Expect-Failure { & $installer -InstallDir relative -NonInteractive | Out-Null } "absolute path"
    Assert-True ($env:Path -eq $originalPath) "Installer changed process PATH"
    Assert-True ([Environment]::GetEnvironmentVariable("Path", "User") -eq $userPath) "Installer changed persistent PATH"
    $env:INSTALLER_TEST_CHECKSUM = (Get-FileHash -LiteralPath $env:INSTALLER_TEST_ARCHIVE -Algorithm SHA256).Hash.ToLowerInvariant() + "  $name.zip"
    foreach ($scenario in @("latest", "archive", "checksum", "trust", "cancelled", "badlatest", "lag")) {
        $env:INSTALLER_TEST_SCENARIO = $scenario
        $script:requests.Clear()
        [System.IO.File]::WriteAllText($target, "old-cli")
        $expected = switch ($scenario) {
            "latest" { "GitHub version lookup failed" }
            "archive" { "GitHub download failed" }
            "checksum" { "GitHub download failed" }
            "trust" { "GitHub version lookup failed" }
            "cancelled" { "GitHub version lookup failed" }
            "badlatest" { "semantic version" }
            "lag" { "GitHub download failed" }
        }
        Expect-Failure { & $installer -Yes -NonInteractive | Out-Null } $expected
        Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "old-cli") "Failure lost old binary"
        $expectedRequests = switch ($scenario) {
            { $_ -in @("latest", "trust", "cancelled", "badlatest") } { 1 }
            { $_ -in @("archive", "lag") } { 2 }
            "checksum" { 3 }
        }
        Assert-True ($script:requests.Count -eq $expectedRequests) "Failure made extra requests"
        if ($scenario -eq "lag") {
            Assert-True (@($script:requests | Where-Object { $_ -like "*/v1.2.3/*" }).Count -eq 0) "Silently downgraded version"
        }
    }
    $env:INSTALLER_TEST_SCENARIO = "archive"
    $script:requests.Clear()
    Expect-Failure { & $installer -Version 1.2.3 -Yes -NonInteractive | Out-Null } "GitHub download failed"
    Assert-True ($script:requests.Count -eq 1) "Explicit download failure made extra requests"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "old-cli") "Explicit download failure lost old binary"
    $env:INSTALLER_TEST_SCENARIO = ""
    $script:requests.Clear()
    Expect-Failure { & $installer -Version "../invalid" -Yes -NonInteractive | Out-Null } "semantic version"
    Assert-True ($script:requests.Count -eq 0) "Invalid version made requests"
    $output = & $installer -Version 1.2.3 -Yes -NonInteractive
    Assert-True (($output -join "`n") -like "*from GitHub*") "Missing GitHub success notice"
    Assert-True ($script:requests.Count -eq 2) "Explicit version triggered latest lookup"
    Assert-True ((Get-Content -LiteralPath $target -Raw) -eq "new-cli") "Explicit install failed"
    Write-Output "PowerShell installer behavior tests passed."
} finally {
    $env:MIOCLI_REPOSITORY = $originalRepository
    $env:MIOCLI_INSTALL_DIR = $originalInstallDir
    $env:LOCALAPPDATA = $originalLocalAppData
    $env:INSTALLER_TEST_ARCHIVE = $originalTestArchive
    $env:INSTALLER_TEST_CHECKSUM = $originalTestChecksum
    $env:INSTALLER_TEST_SCENARIO = $originalScenario
    Remove-Variable -Name miocliInstallerTestRequests -Scope Global
    Remove-Item -LiteralPath $work -Recurse -Force
}

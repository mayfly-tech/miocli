# Installation

English · [简体中文](zh-CN/installation.md) · [Documentation](README.md)

Installing miocli installs only the console. Prepare a running Mihomo controller separately; existing Clash Verge and other Mihomo users do not need a second core.

## Supported release targets

| Platform | Architectures | Archive |
| --- | --- | --- |
| Linux | amd64, arm64 | `.tar.gz` |
| macOS (`darwin`) | amd64, arm64 | `.tar.gz` |
| Windows | amd64, arm64 | `.zip` |

These are build targets, not a guarantee for every OS version or terminal. The TUI requires interactive input/output and at least 50 columns × 12 rows. A local connection can use loopback HTTP; remote connections require HTTPS or an SSH tunnel.

## Install with a release script

Use the scripts attached to [GitHub Releases](https://github.com/mayfly-tech/miocli/releases/latest), not the repository's unconfigured script templates. Repository templates contain a release-time repository placeholder.

### Linux and macOS

Requires a POSIX shell, `curl`, `tar`, standard utilities and either `sha256sum` or `shasum`.

```sh
curl --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --max-redirs 5 --connect-timeout 15 --max-time 60 \
  https://github.com/mayfly-tech/miocli/releases/latest/download/install.sh -o install.sh
# Inspect the downloaded script.
sh install.sh
```

Default location: `~/.local/bin/miocli`. No administrator privileges are required for this user directory.

### Windows

Use Windows PowerShell or PowerShell with the required Windows commands, including `Get-CimInstance`, `Get-FileHash` and `Expand-Archive`. CI exercises both Windows PowerShell and PowerShell.

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest https://github.com/mayfly-tech/miocli/releases/latest/download/install.ps1 `
  -OutFile install.ps1 -TimeoutSec 60 -MaximumRedirection 5 -UseBasicParsing
# Inspect the downloaded script.
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

Default location: `%LOCALAPPDATA%\Programs\miocli\miocli.exe`. Close running miocli processes before replacing the executable.

### Installer options

| Purpose | POSIX shell | PowerShell |
| --- | --- | --- |
| Specific version | `--version vX.Y.Z` or positional version | `-Version vX.Y.Z` or positional version |
| Install directory | `--install-dir /absolute/path` | `-InstallDir C:\absolute\path` |
| Allow replacement | `--yes` / `-y` | `-Yes` |
| Disable prompts | `--non-interactive` | `-NonInteractive` |
| Run setup after installation | `--setup` | `-Setup` |
| Show help | `--help` / `-h` | `-Help` |

Replace `vX.Y.Z` with a published version. Both installers accept versions with or without `v`. `MIOCLI_INSTALL_DIR` supplies a directory when the corresponding option is absent; the directory must be absolute. `MIOCLI_REPOSITORY` overrides the GitHub `owner/repository` source for deliberate alternate distributions, not the controller address.

Replacing an existing CLI requires confirmation or `--yes` / `-Yes`. Non-interactive mode does **not** imply replacement consent. `--setup` / `-Setup` requires an interactive terminal; `--yes` does not confirm saving connection settings. Interactive installation may offer setup even without that option.

## PATH and verification

Installers **never modify PATH**. Follow their printed instructions. For the default POSIX location, enable it for the current shell:

```sh
export PATH="$HOME/.local/bin:$PATH"
miocli version
```

For the default Windows location:

```powershell
$env:Path = "$env:LOCALAPPDATA\Programs\miocli;$env:Path"
miocli version
```

For persistence, add the actual directory to your shell configuration or Windows user PATH, then open a new terminal. Alternatively, invoke the executable by its full path.

Run `miocli setup`, `miocli doctor`, then `miocli`. See [configuration](configuration.md) before exposing any controller remotely.

## Manual installation and integrity

Download `miocli_X.Y.Z_OS_ARCH.tar.gz` (Windows: `.zip`) and the matching `.sha256` file from the same release. The OS name for macOS is `darwin`; archive names omit the version's leading `v`.

Verify the archive **before** extracting it, using `sha256sum --check`, `shasum -a 256`, or PowerShell `Get-FileHash -Algorithm SHA256` and comparing the exact expected digest. Extract the package and place `miocli` / `miocli.exe` in a directory you control. Keep `LICENSE`, `THIRD_PARTY_LICENSES` and the included documentation when retaining or redistributing the package.

Scripts download archives/checksums from GitHub over HTTPS and verify SHA-256 before replacing the binary. Co-hosted checksums detect corruption, not an independently authenticated source; releases have no independent package signatures.

## Upgrade or roll back

Exit miocli and rerun a reviewed release installer. Omit the version to resolve the latest published release, or choose a specific published version to upgrade/roll back. Use replacement consent when unattended. Connection settings are separate and are not rewritten by binary replacement.

Version lookup is limited to 15 seconds; archive and checksum requests each have a 30-second limit. A specified version skips latest lookup. Network, version and integrity failures preserve the existing CLI. If post-install setup fails, the new CLI is already installed; rerun setup separately.

A published Release may temporarily lack assets while its workflow is running. Wait for successful completion; see [release recovery](releasing.md#recovery).

## Uninstall

Exit all miocli sessions (`q` or `Ctrl+C`). This does not stop Mihomo.

Remove only the binary from its actual location:

```sh
# Default Linux/macOS location.
rm -f -- "$HOME/.local/bin/miocli"
```

```powershell
# Default Windows location.
$binary = Join-Path $env:LOCALAPPDATA "Programs\miocli\miocli.exe"
if (Test-Path -LiteralPath $binary -PathType Leaf) {
    Remove-Item -LiteralPath $binary
}
```

For custom install directories, delete only `miocli` / `miocli.exe`, not a shared directory. Optionally remove a PATH entry you added, only if no other tools use it.

Settings are preserved. To remove them too, inspect and back up the dedicated `miocli` directory under `$XDG_CONFIG_HOME` (or `~/.config`) on Linux/macOS, or `%APPDATA%` on Windows, before manually deleting it. This removes saved secrets. For `--config`, inspect and remove only the chosen miocli file, not its shared parent.

Do not delete Mihomo, another client's data, an imported source YAML or the whole checkout. If you enabled miocli's system proxy, run `miocli system-proxy disable` before uninstalling to restore saved settings; miocli does not manage TUN or services.

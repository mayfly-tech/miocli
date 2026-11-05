# miocli

[简体中文](README.md) · English

`miocli` is a keyboard-first terminal operations console for one already running [Mihomo](https://github.com/MetaCubeX/mihomo) instance. For advanced users with an existing core, browse proxy resources, switch nodes and modes, test latency, and inspect or update providers locally or over SSH. miocli never installs, starts, stops or upgrades the core.

## Features

- Full-screen English interface with Groups, Nodes, Providers and Status views; Groups opens by default.
- Browse nested groups and search names; switch members in `Selector` groups and verify changes by rereading the API.
- Filter nodes by protocol or source, sort by API order, name or latency, and test one node, filtered results or direct members of the current group.
- Switch among Rule, Global and Direct modes; inspect and manually update Mihomo proxy providers.
- Connect to one Mihomo controller over loopback HTTP or certificate-validated HTTPS.

## Getting started

### Install

Download the matching installer from the latest GitHub Release:

```sh
# Linux / macOS: download first; do not pipe network content into a shell
curl --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --max-redirs 5 --connect-timeout 15 --max-time 60 \
  https://github.com/mayfly-tech/miocli/releases/latest/download/install.sh -o install.sh
```

```powershell
# Windows PowerShell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest https://github.com/mayfly-tech/miocli/releases/latest/download/install.ps1 `
  -OutFile install.ps1 -TimeoutSec 60 -MaximumRedirection 5 -UseBasicParsing
```

Review the downloaded script, then install:

```sh
# Linux / macOS
sh install.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

Installers verify the release archive's SHA-256 and install the CLI to a user directory without administrator privileges. Existing Clash Verge and other Mihomo users need only a reachable control API; no second core is required.

Installers download packages and SHA-256 files only from GitHub, looking up the latest published release unless a version is specified. Use `--version vX.Y.Z` (PowerShell: `-Version vX.Y.Z`) to skip version lookup. Network, version and integrity errors explicitly stop installation and preserve the existing CLI. After a Release is created, installation must wait for its build assets to finish uploading; see the [release workflow](docs/development_EN.md#release-workflow).

### Connect to an existing core

```sh
miocli setup
miocli doctor
miocli
```

Enter the existing core's `external-controller` URL and `secret` in setup. This is the control API, not the HTTP/SOCKS proxy port. Alternatively, specify a Mihomo YAML file to read these settings only. miocli does not scan application directories, guess secrets, or modify source configuration.

Without saved settings, startup first tries `http://127.0.0.1:9090`. If it cannot connect, only an interactive terminal offers setup; it never installs or starts a core implicitly. `doctor` performs read-only configuration and API checks.

### Local development

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
go run ./cmd/miocli dev
```

Set the existing controller address and secret in the local file. `dev` connects only; it does not write user settings, download a core or start one. On Windows, use `Copy-Item` to copy the template and omit `chmod`. See the [development guide](docs/development_EN.md).

### Product boundary

Like k9s, miocli focuses on interacting with resources: connect to an existing instance, browse, act and verify. It does not own the core lifecycle. Node selection, mode changes and provider updates are supported; full configuration subscriptions, hot reload, system proxy and TUN management are outside the product boundary. See the [product overview](docs/product_EN.md).

### Uninstall

Exit all miocli sessions (`q` or `Ctrl+C`) first. Exiting closes only the console, never the external Mihomo instance.

**Remove only the CLI, preserving connection settings:**

```sh
# Linux / macOS: default install location
rm -f -- "$HOME/.local/bin/miocli"
```

```powershell
# Windows PowerShell: default install location
$binary = Join-Path $env:LOCALAPPDATA "Programs\miocli\miocli.exe"
if (Test-Path -LiteralPath $binary -PathType Leaf) {
    Remove-Item -LiteralPath $binary
}
```

If installation used `--install-dir` / `-InstallDir` or `MIOCLI_INSTALL_DIR`, remove `miocli` / `miocli.exe` from that actual location, not the entire shared installation directory. Installers never modify PATH. If you added a dedicated directory yourself, optionally remove its PATH entry without removing directories still used by other tools.

**Optional: remove connection settings.** Back up settings you want to keep, then inspect and manually remove these dedicated configuration directories. This removes saved connection secrets:

| Platform | Connection settings |
| --- | --- |
| Linux / macOS | `$XDG_CONFIG_HOME/miocli`, or `~/.config/miocli` when unset |
| Windows | `%AppData%\miocli` |

With `--config`, remove only the confirmed miocli configuration file, never its shared parent. Inspect and back up before cleaning, and preserve data still in use. Do not remove external Mihomo binaries, other clients such as Clash Verge, their settings, or the original imported YAML. No system proxy, TUN or service cleanup is required: miocli does not install or modify them.

## Configuration

Configuration file locations:

- Linux/macOS: `$XDG_CONFIG_HOME/miocli/config.yaml`, or `~/.config/miocli/config.yaml` when `XDG_CONFIG_HOME` is unset.
- Windows: `%AppData%\miocli\config.yaml`.

```yaml
server: http://127.0.0.1:9090
secret: ""
```

Server precedence: `--server` > nonempty `MIOCLI_SERVER` > YAML > default URL. Secret precedence: nonempty `MIOCLI_SECRET` > YAML > no authentication. `--config` selects one configuration file; files are not merged. `--server` applies only to the current connection. Never pass secrets as command-line arguments.

## Keyboard shortcuts

| Key | Action |
| --- | --- |
| `1` / `2` / `3` / `4` | Groups / Nodes / Providers / Status |
| `:` | Command bar, e.g. `groups`, `nodes`, `providers`, `mode rule` |
| `Tab` / `Shift+Tab` | Toggle list, details or groups/members |
| `↑` / `↓`, `j` / `k` | Move selection |
| `Enter` / `→` | Open details; select a member / enter a nested group |
| `/` | Search current resource names |
| `Space` / `T` | Test nodes or direct members of the current group |
| `s` / `t` / `f` | Sort nodes / filter protocol / filter source |
| `m` | Cycle Rule, Global and Direct |
| `u` | Update the selected provider and read back status in Providers |
| `r` / `?` | Refresh or reconnect / show help |
| `Esc` | Clear search, close details or go back |
| `q` / `Ctrl+C` | Quit |

Providers are API resources of the existing core, not full YAML configuration profiles. See the [Chinese README](README.md) for setup details.

## Documentation

| Guide | English | 简体中文 |
| --- | --- | --- |
| Product and features | [Product](docs/product_EN.md) | [产品说明](docs/product.md) |
| Architecture | [Architecture](docs/architecture_EN.md) | [架构说明](docs/architecture.md) |
| Development and tests | [Development](docs/development_EN.md) | [开发指南](docs/development.md) |
| Contributions | [Contributing](CONTRIBUTING_EN.md) | [贡献指南](CONTRIBUTING.md) |

## Privacy and security

miocli has no accounts, telemetry or background update checks. Credentials are sent only to the configured controller. Configuration and diagnostic information may contain sensitive information; inspect and redact them before sharing. Report vulnerabilities privately through GitHub Security Advisories.

CLI installation downloads use certificate-validated HTTPS. miocli archives and checksums share a source: SHA-256 detects transfer corruption but does not independently authenticate a compromised distribution source; packages do not include independent signatures.

## License

MIT. See [LICENSE](LICENSE).

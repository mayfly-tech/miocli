# miocli

English · [简体中文](README.zh-CN.md)

A keyboard-first terminal operations console for one already running [Mihomo](https://github.com/MetaCubeX/mihomo) instance.

Browse proxy groups and nodes, change selections and modes, test latency, and update proxy providers from a local terminal or an SSH session. Inspired by the resource-oriented workflow of [k9s](https://github.com/derailed/k9s), miocli connects to your existing controller instead of owning its lifecycle.

## What it does

| Resource or command | Daily tasks |
| --- | --- |
| **Groups** | Search groups and members, explore nested groups, select members of `Selector` groups |
| **Nodes** | Browse global and provider-only nodes, filter by protocol/source, sort, test latency |
| **Providers** | Inspect proxy providers and manually request an update |
| **Profiles** | Download and save HTTPS Mihomo YAML profiles; switch/reload through a local controller |
| **Status** | Inspect the controller, selected group path, active-route summaries and diagnostics |

The interface is in English. Groups opens by default. Write operations read back controller state; inspect the refreshed state after making changes.

**Scope:** miocli does not install, start, stop or upgrade Mihomo. It does not manage TUN, services or multiple controllers. Its optional system-proxy switch changes only the current user's/network service's proxy settings and restores prior values when disabled; it never takes over core lifecycle.

## Quick start

You need a running Mihomo instance with a reachable control API and its secret, if configured. The control API is **not** the HTTP/SOCKS proxy port. The console requires an interactive terminal of at least **50 columns × 12 rows**.

### 1. Install

Download the installer for your platform, inspect it, then run it. Do not pipe downloaded scripts directly into a shell.

**Linux / macOS**

```sh
curl --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --max-redirs 5 --connect-timeout 15 --max-time 60 \
  https://github.com/mayfly-tech/miocli/releases/latest/download/install.sh -o install.sh
# Review install.sh before continuing.
sh install.sh
```

**Windows PowerShell**

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest https://github.com/mayfly-tech/miocli/releases/latest/download/install.ps1 `
  -OutFile install.ps1 -TimeoutSec 60 -MaximumRedirection 5 -UseBasicParsing
# Review install.ps1 before continuing.
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

Installers verify SHA-256 and install to a user directory without administrator privileges. **They do not modify PATH.** Follow the printed PATH instructions or invoke the installed binary by its full path. See [installation](docs/installation.md) for supported platforms, manual installation, upgrades and removal.

### 2. Connect

```sh
miocli setup
miocli doctor
miocli
```

`setup` accepts a controller URL and hidden secret, or reads connection fields from a specified Mihomo YAML without changing it. It checks the API and asks before saving miocli's own settings. Secrets are stored in protected YAML, not an OS keychain.

Local HTTP must use a loopback address. Linux/macOS also support `--server unix:///absolute/control.sock`, subject to socket permissions. Remote controllers require certificate-validated HTTPS or an [SSH tunnel](docs/configuration.md#remote-controllers).

### 3. Operate

- `1` / `2` / `3` / `4`: Groups / Nodes / Providers / Status.
- In Groups, `Enter` focuses members or selects a member; `→` / `l` enters a nested group.
- In Nodes, `Space` tests one node; `T` tests the filtered list.
- In Providers, `u` requests an update.
- `miocli profile add NAME HTTPS_URL` / `update` / `list` / `use` manage profiles.
- `miocli system-proxy enable|disable|status` manages the system proxy.
- `?` opens help; `q` or `Ctrl+C` exits without stopping Mihomo.

See the [usage guide](docs/usage.md) for complete per-view keys and operation semantics.

## Documentation

| Start here | Reference | Contribute and maintain |
| --- | --- | --- |
| [Installation](docs/installation.md) | [Configuration](docs/configuration.md) | [Contributing](CONTRIBUTING.md) |
| [Usage](docs/usage.md) | [CLI commands](docs/cli.md) | [Development](docs/development.md) |
| [Troubleshooting](docs/troubleshooting.md) | [Architecture and API](docs/architecture.md) | [Releasing](docs/releasing.md) |

[All documentation](docs/README.md) · [完整中文文档](docs/zh-CN/README.md) · [Changelog](CHANGELOG.md)

## Privacy and security

No accounts, telemetry or background update checks are included. Controller requests ignore environment proxies and do not follow redirects. Configuration, resource names and screenshots can still be sensitive: redact them before sharing.

Installer downloads use HTTPS. Co-hosted SHA-256 files detect corruption but do not independently authenticate a compromised release source; packages are not independently signed. See the [security policy](SECURITY.md) for private vulnerability reports and supported security fixes.

## License

[MIT](LICENSE).

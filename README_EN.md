# miocli

[简体中文](README.md) · English

`miocli` is a keyboard-first terminal control panel for one [Mihomo](https://github.com/MetaCubeX/mihomo) instance. Browse proxy groups and nodes, switch nodes and proxy modes, test latency, and inspect or update providers.

## Features

- Full-screen English interface with Subscriptions, Groups, Nodes and Status views.
- Browse nested groups and search names; switch members in `Selector` groups and verify changes by rereading the API.
- Filter nodes by protocol or source, sort by API order, name or latency, and test one node, filtered results or direct members of the current group.
- Switch among Rule, Global and Direct modes; inspect and manually update Mihomo proxy providers.
- Manage local YAML/HTTPS profiles for a managed core started by this invocation, with confirmed validation and hot reload.
- Connect to one Mihomo controller over loopback HTTP or certificate-validated HTTPS.

## Getting started

### Install

Download the matching installer from a GitHub Release:

```sh
# Linux / macOS
sh install.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

Installers verify the release archive's SHA-256 and install the CLI to a user directory without administrator privileges. Existing Clash Verge and other Mihomo users need only a reachable control API; no second core is required.

### Connect to an existing core

```sh
miocli setup
miocli doctor
miocli
```

Choose an existing core in setup and enter its `external-controller` URL and `secret`. This is the control API, not the HTTP/SOCKS proxy port. Alternatively, specify a Mihomo YAML file to read these settings only. miocli does not scan application directories, guess secrets, or modify source configuration.

Without saved settings, startup first tries `http://127.0.0.1:9090`. If it cannot connect, only an interactive terminal offers setup; it never installs or starts a core implicitly. `doctor` performs read-only configuration and API checks.

### Local development

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
go run ./cmd/miocli dev
```

Set the existing controller address and secret in the local file. `dev` connects only; it does not write user settings, download a core or start one. On Windows, use `Copy-Item` to copy the template and omit `chmod`. See the [development guide](docs/development_EN.md).

### Managed core

Choose managed mode in setup and confirm configuration import and any required download. miocli operates on a copy of the selected configuration and stops only a child process started by that invocation. Connection failures never switch to managed mode automatically. See the [product overview](docs/product_EN.md).

Managed mode never edits the source configuration. Disable TUN and custom listeners before import, and make relative-path resources available. A missing core is downloaded from official Releases only after separate confirmation; settings are saved only after validation.

## Configuration

Configuration file locations:

- Linux/macOS: `$XDG_CONFIG_HOME/miocli/config.yaml`, or `~/.config/miocli/config.yaml` when `XDG_CONFIG_HOME` is unset.
- Windows: `%AppData%\miocli\config.yaml`.

```yaml
server: http://127.0.0.1:9090
secret: ""
core:
  mode: connect
```

Server precedence: `--server` > nonempty `MIOCLI_SERVER` > YAML > default URL. Secret precedence: nonempty `MIOCLI_SECRET` > YAML > no authentication. `--config` selects one configuration file; files are not merged. `--server` applies only to the current connection. Never pass secrets as command-line arguments.

## Keyboard shortcuts

| Key | Action |
| --- | --- |
| `1` / `2` / `3` / `4` | Subscriptions / Groups / Nodes / Status |
| `:` | Command bar, e.g. `groups`, `nodes`, `providers`, `mode rule` |
| `Tab` / `Shift+Tab` | Toggle list, details or groups/members |
| `↑` / `↓`, `j` / `k` | Move selection |
| `Enter` / `→` | Open details; select a member / enter a nested group |
| `/` | Search current resource names |
| `Space` / `T` | Test nodes or direct members of the current group |
| `s` / `t` / `f` | Sort nodes / filter protocol / filter source |
| `m` | Cycle Rule, Global and Direct |
| `r` / `?` | Refresh or reconnect / show help |
| `Esc` | Clear search, close a dialog or go back |
| `q` / `Ctrl+C` | Quit |

In Subscriptions, `p` toggles profiles and providers. Profile actions use `a` / `e` / `u` / `d`; `Space` activates. See the [Chinese README](README.md) for setup and profile details.

## Documentation

| Guide | English | 简体中文 |
| --- | --- | --- |
| Product and features | [Product](docs/product_EN.md) | [产品说明](docs/product.md) |
| Architecture | [Architecture](docs/architecture_EN.md) | [架构说明](docs/architecture.md) |
| Development and tests | [Development](docs/development_EN.md) | [开发指南](docs/development.md) |
| Contributions | [Contributing](CONTRIBUTING_EN.md) | [贡献指南](CONTRIBUTING.md) |

## Privacy and security

miocli has no accounts, telemetry or background update checks. Credentials are sent only to the configured controller. Managed-core downloads require separate confirmation in setup. Configuration and diagnostic logs may contain sensitive information; inspect and redact them before sharing. Report vulnerabilities privately through GitHub Security Advisories.

## License

MIT. See [LICENSE](LICENSE).

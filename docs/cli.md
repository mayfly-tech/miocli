# CLI reference

English · [简体中文](zh-CN/cli.md) · [Documentation](README.md)

Use `miocli --help` or `miocli COMMAND --help` for the installed build's help. Positional arguments are shown in the command table below; other commands reject them.

## Commands

| Command | Behavior |
| --- | --- |
| `miocli` | Resolve settings, probe the controller, open the full-screen console |
| `miocli setup` | Interactively check and save connection settings after confirmation |
| `miocli doctor` | Print configuration sources and probe the three required read APIs |
| `miocli dev` | Connect using checkout-local `config.local.yaml`, unless `--config` selects another file |
| `miocli profile list` | List stored profiles without revealing subscription URLs |
| `miocli profile add NAME HTTPS_URL` | Download and save an HTTPS Mihomo YAML profile |
| `miocli profile update NAME` | Download the latest profile content |
| `miocli profile use NAME` | Reload a profile through a local controller, retaining miocli's controller and secret |
| `miocli profile remove NAME` | Remove a profile and its local source URL |
| `miocli system-proxy status` | Show the system proxy status |
| `miocli system-proxy enable` | Point the system proxy to local Mihomo ports |
| `miocli system-proxy disable` | Restore the saved pre-miocli proxy settings |
| `miocli version` | Print version, commit and build timestamp |
| `miocli help [command]` | Show help without connecting |
| `miocli completion [shell]` | Generate a shell completion script |

The console and setup require interactive stdin/stdout. `doctor`, `version` and help work without the TUI. Startup may offer setup only under the [first-run conditions](configuration.md#startup-and-development-settings). None of these commands manages core processes. Profiles are stored beside the selected settings file in a private `profiles/` directory. `profile use` requires a local controller because the profile path must be readable by the core host.

```sh
miocli doctor --config ./config.local.yaml
miocli profile add home https://subscription.example/config.yaml
miocli profile update home
miocli profile use home
miocli system-proxy enable
miocli system-proxy disable
miocli --server https://mihomo.example:9090
miocli --no-color
miocli dev --config ./config.local.yaml
miocli version
```

The files in these examples must already exist when `--config` is supplied. The HTTPS hostname is an example, not a bundled service.
macOS commands require `--service` (list available services with `networksetup -listnetworkserviceorder`); if macOS requires administrator authorization, the command reports the OS error and does not elevate itself. Linux uses GNOME `gsettings` for the current desktop user; Windows updates current-user Internet Settings without administrator rights. Existing enabled proxies, PAC URLs and auto-detection settings are not overwritten.

## Global flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config PATH` | Platform default file | Select one YAML file; missing explicit files fail |
| `--server URL` | From environment/file/default | Override only this invocation's control API URL |
| `--no-color` | `false` | Disable terminal colors |
| `-h`, `--help` | `false` | Show help |

There is no `--secret` or root `--version` flag. Use protected configuration/environment and the `version` command.

## Environment variables

| Variable | Used by | Meaning |
| --- | --- | --- |
| `MIOCLI_SERVER` | Connection commands | Nonempty server override |
| `MIOCLI_SECRET` | Connection commands | Nonempty secret override |
| `XDG_CONFIG_HOME` | Linux/macOS configuration | Absolute configuration base directory |
| `APPDATA` | Windows configuration | Default configuration base directory |
| `NO_COLOR` | Terminal styling | Any nonempty value disables colors |
| `TERM` | Terminal styling | `dumb` disables colors |
| `MIOCLI_INSTALL_DIR` | Installers only | Default destination override |
| `MIOCLI_REPOSITORY` | Installers only | GitHub distribution repository override |
| `MIOCLI_INTEGRATION_URL` / `MIOCLI_INTEGRATION_SECRET` | Tests only | Dedicated integration controller; not ordinary runtime settings |

See [configuration](configuration.md#precedence) for precedence and [installation](installation.md#installer-options) for installer parameters. Environment proxy variables do not route controller requests.

## Shell completion

Cobra provides completion generation for Bash, Zsh, Fish and PowerShell. Use `miocli completion --help` and `miocli completion SHELL --help` for shell-specific installation instructions. Generating a script does not install it or connect to a controller.

## In-console commands

Press `:` in the console to enter a command. This is not a shell and is distinct from the process-level CLI:

```text
groups | nodes | providers | status | help
refresh | mode rule | mode global | mode direct | quit | q
```

Commands are case-insensitive, split on whitespace, and reject unsupported names or arguments. See [usage](usage.md#command-bar).

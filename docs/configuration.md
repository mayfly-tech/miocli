# Configuration

English · [简体中文](zh-CN/configuration.md) · [Documentation](README.md)

miocli settings describe a connection to an already running controller. They are not a Mihomo runtime configuration.

## File location and schema

| Platform | Default file |
| --- | --- |
| Linux / macOS | `$XDG_CONFIG_HOME/miocli/config.yaml`; otherwise `~/.config/miocli/config.yaml` |
| Windows | `%APPDATA%\miocli\config.yaml` |

`XDG_CONFIG_HOME` must be absolute when set. Windows requires `APPDATA` for the default path. Ordinary startup does not load a working-directory configuration or `.env`.

```yaml
server: http://127.0.0.1:9090
secret: ""
```

| Field | Meaning | Default |
| --- | --- | --- |
| `server` | Controller URL, not a proxy listener | `http://127.0.0.1:9090` |
| `secret` | Bearer authentication secret | Empty, no Authorization header |
| `active-profile-path` | Optional absolute path for the activated profile used on the core's next start | Unset |

An optional `active-profile-path` absolute path enables saving successfully activated profiles as the core's restart configuration; see [subscription profiles](#subscription-profiles). Unknown fields and multiple YAML documents are rejected. An omitted or empty file `server` retains the default.

`--config PATH` selects **one** file; no merging occurs. A missing default file is allowed, but a missing explicit file is an error, including for `setup`. To use setup with a custom path, provision a valid minimal file there first, or use the default path.

## Precedence

| Setting | Highest to lowest priority |
| --- | --- |
| Server | `--server` → nonempty `MIOCLI_SERVER` → YAML → default |
| Secret | nonempty `MIOCLI_SECRET` → YAML → no authentication |

Empty environment values do not erase file settings. `--server` applies only to the invocation and never saves the address. There is no secret command-line flag; do not put secrets in command arguments or URLs.

The selected file is parsed and its permissions checked **before** environment/flag overrides. Overrides do not bypass malformed YAML, unknown fields, missing explicit files or permissions errors in a file containing a secret.

## Interactive setup

```sh
miocli setup
```

Setup requires interactive stdin/stdout. Choose manual entry or a specified Mihomo YAML, review the URL, enter the hidden secret, pass the read-only API probe, then confirm saving.

- Saves miocli's own `server` and `secret`, preserving an existing `active-profile-path`; does not change the owning app.
- Environment overrides are not silently saved: setup uses entered/imported values. Active overrides still apply to future ordinary invocations.
- Cancellation, failed checks or declined confirmation do not save settings.
- Secrets are stored in protected YAML, **not an OS keychain**. Setup writes a private file (Unix mode `0600`; Windows protected ACL for the current user).
- On Unix, loading a file containing a nonempty secret rejects group/other permission bits. Use `chmod 600` on that file. Windows save protection does not mean every externally provisioned file's ACL is checked at load time.

### Import connection fields

Import reads only top-level `external-controller`, `external-controller-unix` and `secret` from a specified regular YAML file, up to 16 MiB. It accepts a single mapping/document and does not modify the source. `~` at the start of the source path is supported.

On Linux/macOS, a nonempty `external-controller-unix` takes precedence and must be an absolute normalized socket path; it becomes `unix:///absolute/control.sock`. Otherwise, `external-controller` must be a `host:port` HTTP listener usable as a loopback target, for example `127.0.0.1:9090`. URLs, wildcard listeners such as `0.0.0.0:9090`, named pipes and TLS-only controller fields are not automatically converted into a usable connection. Enter the actual loopback or HTTPS URL manually instead.

Connection import in setup does not scan client directories, discover secrets, or import proxies, rules, subscriptions or full profiles.

### Subscription profiles

`miocli profile add NAME HTTPS_URL` downloads one HTTPS YAML document into a private `profiles/` directory beside the selected settings file. The profile URL metadata and YAML are stored with current-user-only permissions. Names use 1–64 letters, digits, dots, underscores or hyphens, starting with a letter or digit; `profiles` is reserved (case-insensitive). Only Mihomo YAML is supported; Base64 node lists are not decoded. Redirects are refused and responses are limited to 16 MiB. Subscription URLs may contain tokens, so they are not printed by list or error output; inspect and redact the profile registry before sharing it.

`miocli profile use NAME` prepares a private effective YAML beside the stored profile and calls Mihomo `PUT /configs?force=true` to reload it. Every activation replaces subscription controller listeners and `secret` with the selected miocli connection: Unix uses only that socket, HTTP uses only that loopback listener, and HTTPS uses only that TLS listener (the profile must still provide valid TLS certificate settings). Other controller listener types are disabled. The original subscription stays unchanged, including after `profile update`; no controller fields are required in the subscription. Other configuration values, including nodes, rules, TUN and DNS, are retained. The core must be able to read it, so activation requires a local controller; a remote controller cannot read a file on the CLI machine. A loopback URL reached through SSH forwarding still points to a remote core and must not be used for activation. A successful request means the reload was accepted, not that the configuration is healthy.

The temporary file is removed after a successful reload. On a reload error it is retained at the path printed in the error, because the core may still be processing it; runtime state is uncertain and the restart file is not saved. Inspect core logs and controller/network state before retrying, then remove the retained file once processing has ended.

For a standalone, externally managed service, you may explicitly set:

```yaml
server: unix:///etc/mayfly-proxy/control.sock
secret: ""
active-profile-path: /etc/mayfly-proxy/config.yaml
```

With `active-profile-path`, after a successful API response `profile use` atomically saves the same effective YAML, with private permissions, to that path for the service's next start. It never writes over the selected miocli settings file or the original subscription profile. `profile update` only downloads; run `profile use` again to apply the updated content with the same controller protection. No TUN/DNS/routing overrides, core validation, service restart or automatic rollback are provided. If saving fails after reload, the command explicitly reports that runtime changed but persistence failed. Back up the restart file before switching, verify with `doctor` and network checks, and restore the backup through the owning service if necessary. Leave this setting unset for configurations owned by another client.

Only management connection fields are adapted; no network safeguards are added. Downloaded profiles must still contain the complete network configuration you intend to run; Base64 lists and node-only subscriptions are not converted into full configurations. Profile source URL arguments may be visible in shell history and process listings; protect token-bearing URLs.

## Local Unix controllers

On Linux/macOS, set `server: unix:///absolute/control.sock` or use `--server` with that URL. The socket path must be absolute and normalized, with no host, query or fragment. Windows named pipes are not supported. The process must have permission to access the socket; miocli does not change its ownership or permissions. Unix controllers count as local for profile activation and system-proxy commands, but system-proxy still requires actual local proxy listener ports.

HTTP requests travel directly over the socket, retaining the usual authentication, response limits, timeouts and redirect refusal. No TCP listener or forwarding service is created.

## Startup and development settings

Without a saved default file, ordinary startup tries the default URL (or an override). A failed probe offers setup only when there is no existing file, no explicit server override, the command is not `dev`, and stdin/stdout are terminals. Other failures return an error; no core is automatically started.

`dev` locates checkout-local `config.local.yaml` by walking upwards for the `miocli` module and template, at most 32 directories. Explicit `dev --config PATH` bypasses checkout discovery. File and environment precedence otherwise remain the same. See [development](development.md).

## Remote controllers

Allowed URLs use `http` or `https`, a host, and an optional port from 1 to 65535. A trailing slash is accepted; other paths, embedded credentials, query strings and fragments are not.

- HTTP is limited to `localhost` or a loopback IP, including IPv6.
- Remote addresses require HTTPS with system trust and hostname verification; there is no insecure TLS bypass.
- The control API client ignores environment proxy variables and does not follow redirects. This is separate from installer download behavior.
- Minimum TLS version is 1.2. Ordinary requests have a 5-second timeout; profile reload has a separate 120-second timeout for synchronous provider/Geo initialization. Responses are limited to 1 MiB.

For a controller listening on a remote machine's loopback interface, open a tunnel in a separate terminal:

```sh
# Replace user@host; local port 19090 must be free.
ssh -N -L 127.0.0.1:19090:127.0.0.1:9090 user@host
```

Keep the tunnel open, then connect without changing saved settings:

```sh
miocli doctor --server http://127.0.0.1:19090
miocli --server http://127.0.0.1:19090
```

Supply the controller secret through protected settings or `MIOCLI_SECRET`, not the SSH command. Do not expose an unauthenticated controller publicly.

## Diagnose safely

`miocli doctor` prints the selected file and setting sources, hides the secret, and checks `/version`, `/proxies` and `/configs`. It requires a Mihomo identity (`meta: true`) and a nonempty mode. It neither writes settings nor checks every optional API. Inspect paths, controller addresses and resource information before sharing diagnostics.

Next: [CLI reference](cli.md) · [Troubleshooting](troubleshooting.md)

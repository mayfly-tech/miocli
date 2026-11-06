# Architecture and API

English · [简体中文](zh-CN/architecture.md) · [Documentation](README.md)

## Scope and modules

miocli is a client for one existing Mihomo controller. It owns connection settings and console state, not the external core's process or full runtime configuration. This avoids competing with existing clients for lifecycle and configuration ownership.

```text
cmd/miocli
  └── internal/cli
        ├── internal/config
        ├── internal/mihomo
        ├── internal/profiles → config
        ├── internal/systemproxy → config
        └── internal/app → mihomo, theme
```

| Package | Responsibility |
| --- | --- |
| `cmd/miocli` | Process context/signals and final error output |
| `internal/cli` | Cobra commands, terminal prompts, connection checks and TUI assembly |
| `internal/config` | File selection, strict settings parsing, URL validation, private writes and read-only connection extraction |
| `internal/mihomo` | HTTP transport, DTOs, domain interfaces, error classification and required-API probe |
| `internal/profiles` | HTTPS YAML download, private storage, updates, effective profile activation and persistence |
| `internal/app` | Bubble Tea state, resource navigation, polling, writes, latency workers and rendering |
| `internal/systemproxy` | Platform proxy settings, private snapshots and restoration |
| `internal/theme` | Styling, color policy, terminal-safe text and input styles |

Source: [CLI](https://github.com/mayfly-tech/miocli/tree/main/internal/cli), [configuration](https://github.com/mayfly-tech/miocli/tree/main/internal/config), [API client](https://github.com/mayfly-tech/miocli/tree/main/internal/mihomo), [console](https://github.com/mayfly-tech/miocli/tree/main/internal/app). These are repository links, not source files bundled in release archives.

## Startup and state flow

1. CLI resolves settings and probes `/version`, `/proxies`, `/configs`.
2. On eligible first-run failures, interactive setup is offered; otherwise errors are returned.
3. A terminal session creates the API client and Bubble Tea Model in an alternate screen.
4. Bootstrap loads version/proxies/mode; providers and connections load independently.
5. `tea.Cmd` performs network work asynchronously; typed messages update the Model.

CLI commands are grouped by connection, profiles and system proxy. Profile activation owns preparation, reload and optional restart-file persistence in `internal/profiles`. The TUI separates message dispatch, navigation, polling, writes, latency workers and resource rendering within `internal/app`.

The UI does not construct HTTP requests or rewrite source Mihomo YAML. [Configuration](configuration.md) is authoritative for file paths, precedence, import and transport rules.

## Reads, writes and concurrency

- Normal polling is scheduled every 5 seconds. Proxies and mode are read concurrently; providers and connections have separate loading/error state.
- Proxy refresh failures preserve previous data. Transient proxy failures receive 1/2/4-second bounded retry scheduling; other failures pause automatic proxy refresh. Optional-resource errors are handled independently.
- Node selection sends a write and rereads proxies; the selection handler compares the returned member with the requested member.
- Mode changes send a write and reread the current mode. Success is confirmed only when readback matches the requested mode; mismatches and readback failures remain explicit.
- Provider updates reread providers and start a proxy/connection refresh, including on uncertain write outcomes.
- Generations/serials reject stale asynchronous results in the paths that track them. In-flight operation guards prevent conflicting writes; timed-out writes are not replayed.
- Latency checks use at most 10 workers, cancellable contexts and provider-aware targets. Cancellation retains completed test results; new tests/writes can reset them.

Resource browsing, confirmed group selections and actual active connection chains are separate state. The console does not infer real routing from the cursor. See [usage](usage.md) for the visible behavior and limitations.

## Mihomo API

| Operation | Request and readback |
| --- | --- |
| Required connection probe | `GET /version`, `GET /proxies`, `GET /configs` |
| Select a Selector member | `PUT /proxies/{group}` with `name`, then `GET /proxies` |
| Change mode | `PATCH /configs` with `mode`, then `GET /configs` |
| Global node latency | `GET /proxies/{name}/delay?url=…&timeout=…` |
| Provider-only node latency | `GET /providers/proxies/{provider}/{name}/healthcheck?url=…&timeout=…` |
| List providers | `GET /providers/proxies` |
| Update provider | `PUT /providers/proxies/{name}`, then provider and proxy refresh |
| Read runtime ports | `GET /configs` |
| Reload a downloaded profile | `PUT /configs?force=true` with the local profile path |
| Active-route summaries | `GET /connections`; only connection chains are presented |

Resource path segments are escaped. Mode writes accept only `rule`, `global` and `direct`. Probe checks require Mihomo `meta: true` and a nonempty mode; optional APIs are not part of doctor's success criterion.

Provider-only nodes use provider/name identity when absent from global proxies. Nodes are assembled from group members, remaining globals and provider entries; default ordering is not an unmodified server array.

## Transport and error handling

HTTP is loopback-only. HTTPS uses system certificate trust, hostname verification and TLS 1.2 or newer. The controller client ignores environment proxies and refuses redirects. Dial timeout is 2 seconds, TLS handshake timeout 3 seconds, ordinary request timeout 5 seconds (120 seconds for profile reload), and body limit 1 MiB.

Nonempty secrets use Bearer authentication. API errors distinguish authentication, not-found, timeout, network, decoding and other HTTP failures; response bodies are not included in error messages. This does not make all diagnostics safe to share: endpoints, paths and resource names can still be private.

Installer downloads have a separate network policy. HTTPS and co-hosted SHA-256 are used; independent package signatures are not provided.

## Boundaries and verification

No production modules implement core lifecycle, TUN, full configuration merging, multi-controller state, traffic charts, log streaming or closing connections. Profile reload is limited to a local Mihomo controller and a downloaded YAML file readable by that core. System-proxy integration uses macOS `networksetup`, GNOME `gsettings`, and Windows current-user Internet Settings; original values are saved privately for restoration.

Tests use fake clients and `httptest.Server` for configuration, endpoint encoding, errors, polling, stale messages, navigation and rendering. Optional integration tests exercise a disposable Mihomo instance; CI runs current and previous stable releases. Test containers are infrastructure, not a product feature.

See [development](development.md) for tests and [releasing](releasing.md) for distribution. Compatibility policy does not promise every historical Mihomo version or every platform environment.

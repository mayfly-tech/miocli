# Architecture and Implementation

简体中文 · [架构说明](architecture.md) · [README](../README_EN.md)

## Modules

```text
cmd/miocli
  └── internal/cli
        ├── internal/config
        ├── internal/mihomo
        └── internal/app
              └── internal/theme
```

- `cmd/miocli` handles process signals and final error output.
- `internal/cli` defines Cobra commands, resolves runtime configuration and assembles the TUI.
- `internal/config` selects files, parses YAML and environment variables, and validates controller URLs. `connection.go` reads connection information from a specified Mihomo YAML and provides diagnostic hints.
- `internal/mihomo` implements the controller HTTP client, DTOs and error classification without a TUI dependency.
- `internal/app` manages UI state with a Bubble Tea Model; `internal/theme` provides terminal styling.

Bubble Tea commands run network requests asynchronously and return typed messages to the Model. The UI does not construct HTTP requests or write source YAML directly. Production code has no core lifecycle or full configuration profile modules.

## Configuration and startup

- Ordinary startup uses user configuration; `dev` uses checkout-local `config.local.yaml`; explicit `--config` selects one file.
- Server precedence is `--server`, nonempty `MIOCLI_SERVER`, YAML, then the default. Secret precedence is nonempty `MIOCLI_SECRET`, YAML, then empty.
- Application settings contain only `server` and `secret`. Unsupported configuration fields produce explicit errors.
- `setup` configures only an existing controller through manual input or read-only extraction from a specified YAML. API checks and save confirmation precede writing the console's own settings.
- Address overrides apply only to the current connection. Normal startup and `dev` never download, start or stop cores; exiting closes only the console.
- Node and mode writes require API readback. Provider updates reread providers and refresh proxy data, using generations to reject pre-change in-flight results. Timed-out writes are not automatically replayed.

## Release distribution

- The POSIX installer resolves the canonical GitHub latest-release redirect without a new JSON dependency; PowerShell uses the GitHub Release API. Explicit versions skip lookup.
- Packages and SHA-256 files are downloaded only from GitHub. Version, network and integrity errors stop installation without replacing the existing CLI.
- Conventional Commits on `main` drive release-please to create a release PR updating the manifest and changelog. Merging it automatically creates a `vX.Y.Z` tag and published GitHub Release.
- The same workflow uses release-please's SHA for CI, six-platform builds and installer generation, then verifies and uploads assets to that Release. The default token does not trigger additional tag/release workflows; installation is unavailable between Release creation and asset upload completion.

Distribution and controller API network policies are separate; the control API still ignores environment proxies. Distribution uses HTTPS. Co-hosted checksums do not replace independent signatures; package signing is not included.

## Mihomo API

| Operation | API |
| --- | --- |
| Read version, proxies and mode | `GET /version`, `GET /proxies`, `GET /configs` |
| Select a Selector member | `PUT /proxies/{group}`, then reread `/proxies` |
| Change mode | `PATCH /configs`; only `rule`, `global` and `direct` are accepted, then reread `/configs` |
| Test node latency | `GET /proxies/{name}/delay` |
| Test provider-only node latency | `GET /providers/proxies/{provider}/{name}/healthcheck` |
| Read/update providers | `GET /providers/proxies`, `PUT /providers/proxies/{name}` |
| Read active routes | `GET /connections`; display connection-chain summaries only |

HTTP is limited to loopback. HTTPS uses system certificates and verifies hostnames. The HTTP client ignores environment proxies and redirects; response bodies are limited to 1 MiB. Error messages omit response bodies.

## Tests

Configuration, CLI and TUI use unit tests and fake clients; the API client uses `httptest.Server`. Optional Mihomo integration tests exercise real API operations against an isolated instance. CI independently runs current and previous stable versions in the test environment without adding core management to production code. CI also runs formatting, static checks, race tests, cross-platform builds and installer checks.

See the [development guide](development_EN.md) for test commands.

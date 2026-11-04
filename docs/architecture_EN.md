# Architecture and Implementation

简体中文 · [架构说明](architecture.md) · [README](../README_EN.md)

## Modules

```text
cmd/miocli
  └── internal/cli
        ├── internal/config
        ├── internal/core
        ├── internal/mihomo
        ├── internal/profile
        └── internal/app
              └── internal/theme
```

- `cmd/miocli` handles process signals and final error output.
- `internal/cli` defines Cobra commands, resolves runtime configuration and assembles the TUI.
- `internal/config` selects configuration files, parses YAML and environment variables, and validates URLs and managed-core policy.
- `internal/core` discovers, validates and starts managed cores; it cleans up only children started by this invocation.
- `internal/mihomo` implements the controller HTTP client, DTOs and error classification without a TUI dependency.
- `internal/profile` stores profiles and performs source refresh, validation, hot reload and transaction recovery.
- `internal/app` manages UI state with a Bubble Tea Model; `internal/theme` provides terminal styling.

Bubble Tea commands run network requests asynchronously and return typed messages to the Model. The UI does not construct HTTP requests or write profile YAML directly.

## Configuration and startup

- Ordinary startup uses user configuration; `dev` uses checkout-local `config.local.yaml`; explicit `--config` selects one file.
- Server precedence is `--server`, nonempty `MIOCLI_SERVER`, YAML, then the default. Secret precedence is nonempty `MIOCLI_SECRET`, YAML, then empty.
- `core.mode` defaults to `connect`. Address overrides apply only to the current connection; `dev` never manages cores.
- `setup` explicitly selects connecting to an existing core or managed mode. Managed setup confirms side effects, validates configuration and checks the API before saving managed settings.
- Connection failures for configured instances never trigger installation or a second core. Managed startup starts a child only after a connection refusal and stops only its own child on exit.
- Full-profile writes require a managed child started by this invocation. Profile application records a protected transaction, performs one hot reload and reads state back; timeouts and readback failures retain the transaction and never replay writes automatically.

## Release distribution

- The POSIX installer resolves the canonical GitHub latest-release redirect without a new JSON dependency; PowerShell uses the GitHub Release API. Network/HTTP failure during lookup switches to S3 `miocli/stable.txt`.
- Archive and checksum form one source-specific pair. Eligible GitHub download failure redownloads both from S3 at the same version; integrity failure never falls back. S3 redirects are disabled in installers.
- `internal/core/mihomo-release.json` is the single pinned version, six-platform filename and SHA-256 manifest, embedded by Go and consumed by the publisher. Core downloads accept only official GitHub and the specified S3 namespace, with redirect, deadline and body-size limits.
- Availability errors are distinct from integrity/policy errors. Setup displays the pinned version, fallback reason and verified source without changing confirmation or process ownership.
- Formal Release publication triggers S3 synchronization. Immutable version objects are uploaded and verified through public HTTPS before script aliases and the stable index are promoted. Reruns verify existing objects, reject conflicting bytes and never downgrade stable.

Distribution and controller API network policies are separate; the control API still ignores environment proxies. Distribution uses HTTPS. Co-hosted checksums do not replace independent signatures; package signing is not included.

## Mihomo API

| Operation | API |
| --- | --- |
| Read version, proxies and mode | `GET /version`, `GET /proxies`, `GET /configs` |
| Select a Selector member | `PUT /proxies/{group}`, then reread `/proxies` |
| Change mode | `PATCH /configs`; only `rule`, `global` and `direct` are accepted, then reread `/configs` |
| Test node latency | `GET /proxies/{name}/delay` |
| Read/update providers | `GET /providers/proxies`, `PUT /providers/proxies/{name}` |
| Read active routes | `GET /connections`; display connection-chain summaries only |
| Apply a profile | `PUT /configs` with a candidate path inside the managed directory |

HTTP is limited to loopback. HTTPS uses system certificates and verifies hostnames. The HTTP client ignores environment proxies and redirects; response bodies are limited to 1 MiB. Error messages omit response bodies.

## Tests

Configuration, CLI, core, profiles and TUI use unit tests and fake clients; the API client uses `httptest.Server`. Optional Mihomo integration tests exercise real API operations against an isolated instance. CI also runs formatting, static checks, race tests, cross-platform builds and installer checks.

See the [development guide](development_EN.md) for test commands.

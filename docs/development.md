# Development and testing

English · [简体中文](zh-CN/development.md) · [Documentation](README.md)

## Requirements

- Go 1.24 or newer; see the repository's [go.mod](https://github.com/mayfly-tech/miocli/blob/main/go.mod).
- Unit tests use fake clients and local HTTP servers; a running Mihomo is not required.
- A disposable controller is needed only for integration tests, which mutate node/mode/provider state.

Run the examples below from the source checkout. Release archives contain user documentation, not the Go source, scripts or fixtures linked here.

## Local connection settings

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
```

On Windows:

```powershell
Copy-Item config.local.example.yaml config.local.yaml
```

Fill `server` and `secret` for an existing controller. The local file is ignored by Git; never put real secrets in the tracked template.

```sh
go run ./cmd/miocli dev
go run ./cmd/miocli doctor --config ./config.local.yaml
```

`dev` finds the checkout root from the current directory and only connects. It does not write user settings, download or start a core. Nonempty `MIOCLI_SERVER` / `MIOCLI_SECRET` still override local values. Outside the checkout, use an explicit existing file with `dev --config PATH`. Ordinary startup does not implicitly load this local file or `.env`.

## Checks

Start with the tests covering your change, then expand as needed:

```sh
go test ./internal/config ./internal/cli
go test ./internal/app
go test ./internal/mihomo
```

Full Go checks (POSIX shell):

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
go run ./cmd/miocli version
```

On Windows, format changed files and choose a suitable build output path. Development version output is `dev` with unknown commit/build date unless injected at build time.

Installer checks from the checkout:

```sh
sh -n install.sh
sh tests/installers/shell.sh
```

```powershell
pwsh -File tests/installers/powershell.ps1
# To check Windows PowerShell compatibility:
powershell -File tests/installers/powershell.ps1
```

Installer tests use isolated fixtures. Do not test replacement against a real installed binary.

## Integration tests

Use the repository's [Mihomo fixture](https://github.com/mayfly-tech/miocli/blob/main/.github/fixtures/mihomo.yaml) and [provider fixture](https://github.com/mayfly-tech/miocli/blob/main/.github/fixtures/provider.yaml) in a dedicated instance. The required group is `MIOCLI-Selector` with `DIRECT`/`REJECT`; the single-node file provider is `MIOCLI-Provider`.

```sh
MIOCLI_INTEGRATION_URL=http://127.0.0.1:19090 \
MIOCLI_INTEGRATION_SECRET=miocli-ci-secret \
go test -count=1 -run '^TestMihomoIntegration$' ./internal/mihomo
```

The secret above belongs to the public test fixture, not a user instance. **Never point this test at your daily controller.** It changes the selected member and mode, updates the provider and calls delay APIs. The delay test uses a local HTTP server with a 10ms response delay to avoid Mihomo treating sub-millisecond results as zero latency/HTTP 503; it does not depend on the production gstatic URL.

Without `MIOCLI_INTEGRATION_URL`, the integration test is skipped. CI independently runs current/previous stable Mihomo in test-only containers. See the [CI workflow](https://github.com/mayfly-tech/miocli/blob/main/.github/workflows/ci.yml) for fixture mounting and readiness checks.

## Commit messages and hooks

Optionally install the clone-local commit-message hook:

```sh
sh scripts/install-hooks.sh
```

Use `type(scope)!: subject`; scope and `!` are optional. Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`. Git-generated merge/revert/fixup/squash messages are exempt.

The hook checks messages only, not formatting/tests. Its installation affects the current clone. Examples in commit messages must describe implemented behavior.

## Implementation and documentation boundaries

Keep HTTP requests/DTOs in `internal/mihomo`, settings/import in `internal/config`, asynchronous UI work in Bubble Tea commands, and rendering state in `internal/app`. See [architecture](architecture.md).

Changes to defaults, precedence, keys or API behavior require tests and corresponding English/Chinese documentation. Check links/anchors for documentation changes; do not run integration tests merely to validate Markdown.

For cleanup, exit `go run` and remove only self-built binaries or local settings you intend to discard. Do not delete the checkout or another client's data. Installed CLI removal is covered in [installation](installation.md#uninstall).

Next: [Contributing](../CONTRIBUTING.md) · [Releasing](releasing.md)

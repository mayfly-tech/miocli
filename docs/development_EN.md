# Development and Testing

简体中文 · [开发指南](development.md) · [README](../README_EN.md)

## Requirements

- Go 1.24 or newer.
- Unit tests do not require a running Mihomo instance; API tests use `httptest.Server` and fake clients.
- Integration tests change the instance's node, mode or configuration. Run them only against a disposable test instance.

## Local configuration

To connect to an existing Mihomo instance, create a local configuration at the repository root:

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
```

Set `server` and `secret` in `config.local.yaml`. `dev` reads this file from the checkout root and only connects; it does not write user settings, download a core or start one. On Windows, use `Copy-Item` to copy the template and omit `chmod`. Never put real secrets in the template or commit them.

```sh
go run ./cmd/miocli dev
go run ./cmd/miocli doctor --config ./config.local.yaml
```

Nonempty `MIOCLI_SERVER` and `MIOCLI_SECRET` temporarily override the file; empty values do not clear saved settings. Installed invocations do not automatically read current-directory configuration or `.env`.

## Local checks

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
go run ./cmd/miocli version
sh -n install.sh
sh tests/installers/shell.sh
```

Test the Windows installer with:

```powershell
pwsh -File tests/installers/powershell.ps1
```

Cross-build examples:

```sh
GOOS=linux GOARCH=amd64 go build -o /tmp/miocli-linux-amd64 ./cmd/miocli
GOOS=darwin GOARCH=arm64 go build -o /tmp/miocli-darwin-arm64 ./cmd/miocli
GOOS=windows GOARCH=amd64 go build -o /tmp/miocli-windows-amd64.exe ./cmd/miocli
```

## Mihomo integration tests

`TestMihomoIntegration` requires a disposable instance with a `MIOCLI-Selector` group containing `DIRECT` and `REJECT`, and a single-node file provider named `MIOCLI-Provider`. The test changes node and mode, updates the provider and calls the `DIRECT` delay API. Do not target an instance used for normal traffic.

```sh
MIOCLI_INTEGRATION_URL=http://127.0.0.1:19090 \
MIOCLI_INTEGRATION_SECRET=miocli-ci-secret \
go test -count=1 -run '^TestMihomoIntegration$' ./internal/mihomo
```

`TestIsolated` starts and cleans up its own child process. Obtain and verify a Mihomo binary, then provide its absolute path:

```sh
MIOCLI_TEST_BINARY=/absolute/path/to/mihomo \
go test -count=1 -run '^TestIsolated' ./internal/mihomo ./internal/profile
```

These tests do not read user settings or connect to a regular instance. Profile indexes, transaction logs and configurations may contain sensitive information; use non-sensitive data in samples, screenshots and logs.

## Code boundaries

- API DTOs and domain types belong in `internal/mihomo`; the UI does not construct HTTP requests.
- Bubble Tea `tea.Cmd` starts network work; typed messages update the Model.
- Profile storage and reload belong in `internal/profile`. Only a managed child actually started by this invocation permits full-profile writes.
- Changes to configuration precedence, security policy or API mappings require corresponding test, README and [architecture](architecture_EN.md) updates.

See the [contributing guide](../CONTRIBUTING_EN.md) for pull request guidance.

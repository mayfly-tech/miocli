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
python3 -B -m unittest discover -s tests -p '*_test.py'
```

## Commit messages and Git hooks

Install the local commit message check from the repository root:

```sh
sh scripts/install-hooks.sh
```

Commit messages must follow Conventional Commits: `type(scope)!: subject`, with an optional scope and breaking-change marker `!`. Supported types are `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore` and `revert`. For example: `feat(cli): add profile command`. Git-generated merge, revert, fixup and squash messages are not checked. The hook checks only commit messages; it does not run formatting or tests. Installation applies only to the current clone.

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

`TestIsolatedS3Bootstrap` uses the pinned archive for the native platform and a local HTTPS mirror to simulate unreachable GitHub, install into an empty core directory, verify the binary, start a real child and clean it up:

```sh
MIOCLI_TEST_ARCHIVE=/absolute/path/to/pinned-mihomo.gz \
go test -race -count=1 -run '^TestIsolatedS3Bootstrap$' ./internal/core
```

## Domestic S3 publication

Tag builds still create draft GitHub Releases. Publishing a stable Release triggers `publish-s3.yml` in the protected `s3-release` environment, synchronizing existing Release files without rebuilding binaries. A workflow_dispatch run can retry a previously published stable tag; older tags never downgrade the stable index.

Environment configuration (never place credentials in source or clients):

Public documentation records configuration names and generic publication conventions only, not actual storage API endpoints, bucket names, account identifiers, deployment locations, credential values or internal network details. Manage actual settings through the GitHub publishing environment, and redact issues, pull requests, logs and screenshots. The `example.com` domain below is illustrative, not a working download source.

| Type | Name | Purpose |
| --- | --- | --- |
| Variable | `S3_ENDPOINT` | HTTPS S3-compatible API endpoint, not the public download URL |
| Variable | `S3_BUCKET` | Destination bucket |
| Variable | `S3_KEY_PREFIX` | Optional object key prefix mapped to the public root |
| Variable | `S3_REGION` | Optional region; defaults to `us-east-1` |
| Secret | `S3_ACCESS_KEY_ID` | CI uploads only |
| Secret | `S3_SECRET_ACCESS_KEY` | CI uploads only |

Configure public read-only downloads so `miocli/...` and `mihomo/...` objects resolve beneath the project's public download root (example: `https://downloads.example.com/public/`). The actual public root must match the installers, core downloader and publisher; do not substitute an internal storage API endpoint. Scope CI GetObject/PutObject permissions to the distribution prefix; deletion is unnecessary. Also grant bucket-level ListBucket so Head requests for missing objects return 404 instead of 403; a dedicated publication bucket is recommended. The API must support conditional Put (`If-None-Match` / `If-Match`); unsupported operations fail explicitly rather than falling back to unsafe overwrites. Protect the publishing environment and limit permitted tags.

```text
miocli/install.sh                    # stable script entry
miocli/install.ps1
miocli/stable.txt                    # one-line stable version, updated last
miocli/releases/vX.Y.Z/              # installers, six packages and SHA-256 files
mihomo/releases/vA.B.C/              # six upstream archives, LICENSE and SOURCE.txt
```

Verify all local packages and official Mihomo digests before uploading immutable objects; verify public downloads before promoting stable aliases. Version paths are byte-immutable: reruns reuse matching objects and report conflicts, 403s, upload errors or public verification failures. Stable updates are conditional and the workflow is serialized. Keep script aliases version-independent because multi-object promotion is not transactional. The public domain must honor Cache-Control and avoid long-lived stable-entry caches.

If final public-index verification fails after promotion, the index may already have changed; publication reports failure instead of claiming success. Repair public access or caching and retry; same-version reruns also verify stable entry points.

To upgrade Mihomo, update `internal/core/mihomo-release.json` after verifying an official stable version, all six filenames, archive SHA-256 values and the license. Prefer portable amd64 baseline variants. Update pinned-version notes in both READMEs and run mapping/download tests plus isolated-instance compatibility tests. Never update only the version string or query latest from clients.

Mihomo is a separate GPL-3.0 program. The mirror preserves upstream archives and includes the upstream license plus `SOURCE.txt` directing users to corresponding versioned source downloads; maintainers must keep that source accessible. These directions do not mean source or configuration resources are mirrored to domestic S3.

GitHub version lookup is limited to 15 seconds and package/checksum requests to 30 seconds each; S3 metadata/checksums use 60 seconds and archives 180 seconds. POSIX/Go also bound connection setup. PowerShell 7.4+ configures connection and operation timeouts separately; older PowerShell retains its `TimeoutSec` total request limit without requiring a PowerShell upgrade.

Acceptance uses isolated user directories and a minimal local configuration without external rules/Geo/providers: with GitHub unreachable and no core installed, obtain the script from S3, install miocli, confirm managed-core S3 fallback, verify the version/digest and clean up only the created child. Real acceptance needs configured CI storage credentials and formal publication; passing tests does not mean S3 has been deployed.

## Uninstall and cleanup

See [README: Uninstall](../README_EN.md#uninstall) for installed CLI removal, optional data cleanup and custom-path precautions. Local `go run ./cmd/miocli dev` does not install a program or write user settings; exit the process when finished. Remove self-built binaries from their actual output locations. Checkout-local `config.local.yaml` may contain secrets; back it up or remove that file separately as needed. Never delete the whole workspace or another client's configuration to uninstall miocli.

## Code boundaries

- API DTOs and domain types belong in `internal/mihomo`; the UI does not construct HTTP requests.
- Bubble Tea `tea.Cmd` starts network work; typed messages update the Model.
- Profile storage and reload belong in `internal/profile`. Only a managed child actually started by this invocation permits full-profile writes.
- Changes to configuration precedence, security policy or API mappings require corresponding test, README and [architecture](architecture_EN.md) updates.

See the [contributing guide](../CONTRIBUTING_EN.md) for pull request guidance.

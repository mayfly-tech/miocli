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

The delay test uses a local HTTP endpoint that waits 10ms before responding, preventing sub-millisecond loopback requests from being recorded as zero latency and rejected with HTTP 503 by Mihomo. It does not depend on an external latency-test URL.

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

`TestIsolatedPinnedBootstrap` uses the pinned archive for the native platform and a local HTTPS test server to install into an empty core directory, verify the binary, start a real child and clean it up:

```sh
MIOCLI_TEST_ARCHIVE=/absolute/path/to/pinned-mihomo.gz \
go test -race -count=1 -run '^TestIsolatedPinnedBootstrap$' ./internal/core
```

## Domestic S3 publication

Release archives export third-party licenses for each target platform without skipping Windows-only dependencies. `go-localereader` is pinned to `v0.0.2-0.20220822084737-6bae6c923850`: compared with `v0.0.1`, this commit only adds the upstream LICENSE with no code changes, preventing Windows packaging failures due to a missing license.

After a tag build succeeds and creates a draft GitHub Release, `release.yml` uploads existing build artifacts directly using `TencentCloud/cos-action`, without rebuilding. Only stable tags upload to COS; S3 becomes available before the GitHub Release is formally published. Retry failed upload jobs in the Release workflow. There is no separate publication workflow or Python publisher.

Use GitHub Actions Secrets for all environment configuration. Set the following under `Settings → Environments → s3-release → Environment secrets`; the workflow does not read Variables with the same names. Only `S3_KEY_PREFIX` is optional; all other entries are required. Never place credentials in source or clients.

Public documentation records configuration names and generic publication conventions only, not actual storage API endpoints, bucket names, account identifiers, deployment locations, credential values or internal network details. Manage actual settings through the GitHub publishing environment, and redact issues, pull requests, logs and screenshots. The `example.com` domain below is illustrative, not a working download source.

| Type | Name | Purpose |
| --- | --- | --- |
| Secret | `S3_BUCKET` | Full COS bucket name, including its APPID suffix |
| Secret | `S3_KEY_PREFIX` | Optional object key prefix mapped to the public root |
| Secret | `S3_REGION` | COS region identifier matching the bucket's region |
| Secret | `S3_ACCESS_KEY_ID` | CI uploads only |
| Secret | `S3_SECRET_ACCESS_KEY` | CI uploads only |

The Action uses the native COS SDK; `S3_ENDPOINT` and AWS CLI are no longer needed. Configure public read-only downloads so `miocli/...` objects resolve beneath the installers' public download root (example: `https://downloads.example.com/public/` uses prefix `public`). CI requires `cos:PutObject` for that prefix. With `clean: false`, historical files are not deleted and ListBucket or delete permissions are unnecessary. Protect the publishing environment and limit permitted tags.

```text
miocli/install.sh                    # stable script entry
miocli/install.ps1
miocli/stable.txt                    # one-line stable version, updated last
miocli/releases/vX.Y.Z/              # installers, six packages and SHA-256 files
```

Verify build-artifact SHA-256 values, upload versioned packages and installer aliases, then upload `stable.txt` separately only after success. Upload jobs are serialized but not transactional: there are no conditional writes, conflict checks, automatic public verification or downgrade prevention. Failures may leave partial files; rerunning an older tag can overwrite stable. Do not rerun older tags to update stable entry points. The Action does not set Cache-Control; configure COS/CDN to avoid long-lived installer and stable-index caches.

To upgrade Mihomo, update `internal/core/mihomo-release.json` after verifying an official stable version, all six filenames, archive SHA-256 values and the license. Prefer portable amd64 baseline variants. Update pinned-version notes in both READMEs and run mapping/download tests plus isolated-instance compatibility tests. Never update only the version string or query latest from clients.

Mihomo is a separate GPL-3.0 program and is not mirrored or uploaded by this workflow. Clients download and verify its pinned version only from official GitHub Releases; when GitHub is unreachable, select an existing local core in setup.

GitHub version lookup is limited to 15 seconds and package/checksum requests to 30 seconds each; S3 metadata/checksums use 60 seconds and archives 180 seconds. POSIX/Go also bound connection setup. PowerShell 7.4+ configures connection and operation timeouts separately; older PowerShell retains its `TimeoutSec` total request limit without requiring a PowerShell upgrade.

Acceptance uses isolated user directories: obtain the script from S3, install miocli and verify the version/digest. Download Mihomo from GitHub or reuse an existing local core. Real acceptance needs configured CI storage credentials and a successful stable-tag Release workflow; passing tests does not mean S3 has been deployed.

## Uninstall and cleanup

See [README: Uninstall](../README_EN.md#uninstall) for installed CLI removal, optional data cleanup and custom-path precautions. Local `go run ./cmd/miocli dev` does not install a program or write user settings; exit the process when finished. Remove self-built binaries from their actual output locations. Checkout-local `config.local.yaml` may contain secrets; back it up or remove that file separately as needed. Never delete the whole workspace or another client's configuration to uninstall miocli.

## Code boundaries

- API DTOs and domain types belong in `internal/mihomo`; the UI does not construct HTTP requests.
- Bubble Tea `tea.Cmd` starts network work; typed messages update the Model.
- Profile storage and reload belong in `internal/profile`. Only a managed child actually started by this invocation permits full-profile writes.
- Changes to configuration precedence, security policy or API mappings require corresponding test, README and [architecture](architecture_EN.md) updates.

See the [contributing guide](../CONTRIBUTING_EN.md) for pull request guidance.

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

## Release workflow

`release.yml` runs `googleapis/release-please-action` on pushes to `main`. `release-please-config.json` uses the single-package Go strategy, and `.release-please-manifest.json` tracks the published version with `v0.1.1` as the initial baseline. `bootstrap-sha` starts the first changelog after that version. Go ldflags inject the release version at build time; development builds display `dev`.

1. Use Conventional Commits: `fix` produces a patch release, `feat` a minor release, and `!` or `BREAKING CHANGE` a major release. Changes containing only `ci`, `chore` or documentation commits do not normally create a release PR.
2. release-please creates or updates a release PR changing the version manifest and `CHANGELOG.md`. Review the version and changelog, then merge; no manual tag is needed.
3. After the release PR is merged, release-please creates a `vX.Y.Z` tag and published GitHub Release with generated release notes.
4. The same workflow uses release-please's release SHA for full CI, Linux/macOS/Windows amd64/arm64 builds and both installers. After SHA-256 verification, it uploads all 14 assets to that Release.

The published Release exists before its assets. Wait for the entire Release workflow to succeed before installing; latest installation may fail while assets are absent but preserves an existing CLI. Release runs are serialized without cancelling an ongoing release. Archives retain `miocli_X.Y.Z_OS_ARCH.tar.gz` names (`.zip` on Windows), each accompanied by a matching `.sha256` file.

Release archives export third-party licenses for each target platform without skipping Windows-only dependencies. `go-localereader` is pinned to `v0.0.2-0.20220822084737-6bae6c923850`: compared with `v0.0.1`, this commit only adds the upstream LICENSE with no code changes, preventing Windows packaging failures due to a missing license.

### Permissions and checks

The workflow uses the default `GITHUB_TOKEN` without extra publishing credentials. The release-please job has `contents`, `issues` and `pull-requests` write permissions; upload has only `contents` write permission, and other jobs are read-only. Enable GitHub Actions pull request creation under repository `Settings → Actions → General`.

Release PRs, tags and Releases created with the default token do not automatically trigger other Actions. CI, builds and uploads therefore use the same workflow's `release_created`, `tag_name` and `sha` outputs rather than tag/release events. A human merging the release PR still triggers the publishing workflow via a `main` push.

The release PR itself does not automatically run CI. If branch protection requires PR checks, maintainers must arrange check triggering or use a dedicated token / GitHub App that triggers checks. This workflow does not introduce those credentials or bypass branch protection.

### Recovery and acceptance

If CI, builds or uploads fail, use “Re-run failed jobs” on the same workflow run, preserving the successful release-please job's outputs. Uploads use `gh release upload --clobber` to retry partially uploaded assets at the same version. Do not rerun every job: an existing Release will not produce `release_created=true` again. Failures after Release creation leave some or all assets absent; publication is not successful until the failures are resolved.

Use isolated install directories for acceptance. Download installers from GitHub Release, verify all six packages, six SHA-256 files and two scripts, then check the installed version and commit. GitHub version lookup is limited to 15 seconds and archive/checksum requests to 30 seconds each. Shell also bounds connection setup; PowerShell 7.4+ sets connection and operation timeouts separately, while Windows PowerShell uses `TimeoutSec`. Request, version and integrity failures stop installation and preserve the existing CLI.

Local workflow and installer tests do not create real release PRs, tags or Releases. End-to-end acceptance still requires a successful GitHub release run; passing local tests does not validate repository permissions, branch protection or remote publication.

### Maintaining the Mihomo version

To upgrade Mihomo, update `internal/core/mihomo-release.json` after verifying an official stable version, all six filenames, archive SHA-256 values and the license. Prefer portable amd64 baseline variants. Update pinned-version notes in both READMEs and run mapping/download tests plus isolated-instance compatibility tests. Never update only the version string or query latest from clients.

Mihomo is a separate GPL-3.0 program and is not mirrored or uploaded by this workflow. Clients download and verify its pinned version only from official GitHub Releases; when GitHub is unreachable, select an existing local core in setup.

## Uninstall and cleanup

See [README: Uninstall](../README_EN.md#uninstall) for installed CLI removal, optional data cleanup and custom-path precautions. Local `go run ./cmd/miocli dev` does not install a program or write user settings; exit the process when finished. Remove self-built binaries from their actual output locations. Checkout-local `config.local.yaml` may contain secrets; back it up or remove that file separately as needed. Never delete the whole workspace or another client's configuration to uninstall miocli.

## Code boundaries

- API DTOs and domain types belong in `internal/mihomo`; the UI does not construct HTTP requests.
- Bubble Tea `tea.Cmd` starts network work; typed messages update the Model.
- Profile storage and reload belong in `internal/profile`. Only a managed child actually started by this invocation permits full-profile writes.
- Changes to configuration precedence, security policy or API mappings require corresponding test, README and [architecture](architecture_EN.md) updates.

See the [contributing guide](../CONTRIBUTING_EN.md) for pull request guidance.

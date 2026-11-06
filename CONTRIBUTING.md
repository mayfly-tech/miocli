# Contributing

English · [简体中文](CONTRIBUTING.zh-CN.md) · [Documentation](docs/README.md)

Contributions to miocli are welcome. Keep changes focused, and discuss larger behavior changes in an issue before implementing them.

## Report a problem or request a feature

Search existing issues first. Use the [bug report](https://github.com/mayfly-tech/miocli/issues/new?template=bug_report.yml) or [feature request](https://github.com/mayfly-tech/miocli/issues/new?template=feature_request.yml) form; blank issues are disabled.

For bugs, include:

- miocli version (`miocli version`), or the target release if installation failed.
- Mihomo version when available; otherwise state "unknown" or "not applicable".
- OS, architecture, and terminal/version for UI problems.
- Connection mode, minimal reproduction commands/keys, expected and actual behavior.
- Relevant redacted diagnostics or screenshots, if useful.

For features, explain the user problem, desired behavior and alternatives. Respect the [product boundary](README.md#what-it-does): miocli operates an existing controller, not its core lifecycle.

Never post controller secrets, subscription URLs, private configuration (including storage settings), full configuration files or real traffic data. Inspect resource names and screenshots too.

## Develop and validate

Use Go 1.24 or newer. Unit tests use fake clients/local HTTP servers; they do not need a running Mihomo instance. Follow [development](docs/development.md) for local setup, targeted tests and optional Git hooks.

For Go changes, run the relevant checks, expanding to the full set before submission as appropriate:

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
```

The example assumes a POSIX shell; on Windows choose a suitable output path and format the changed Go files. Run integration tests **only against a dedicated disposable instance**, never your daily controller.

Add or update tests for behavior changes. API changes need `httptest.Server` coverage. New direct dependencies need a rationale and a compatible license.

## Documentation changes

English is the default. Update the complete Chinese counterpart in the same change:

- `README.md` ↔ `README.zh-CN.md`.
- `CONTRIBUTING.md` ↔ `CONTRIBUTING.zh-CN.md`.
- `SECURITY.md` ↔ `SECURITY.zh-CN.md`.
- `docs/*.md` ↔ `docs/zh-CN/*.md` for maintained guides.

Keep commands, conditions, defaults, warnings and links equivalent. Use real implementation/test evidence, not planned behavior. Preserve UI labels such as Groups and Selector. `CHANGELOG.md` remains release-please-generated and `LICENSE` is not translated.

For documentation-only changes, check links, anchors, examples and bilingual coverage; Go tests may be marked not applicable. When moving pages, update templates and [release documentation packaging](docs/releasing.md#packages-and-assets), not just README links.

## Submit a pull request

Complete the PR template with the problem, changes and actual validation commands/results. Explain skipped or inapplicable checks. Link issues with `Closes #123` when appropriate; an issue is not required for every PR.

Use [Conventional Commits](docs/development.md#commit-messages-and-hooks), for example `docs: clarify controller setup`. Do not include unrelated formatting or generated version changes.

Release targets are Linux, macOS and Windows on amd64/arm64. CI tests current and previous stable Mihomo releases; this is a test strategy, not a guarantee for every historical core version. See [releasing](docs/releasing.md) for maintainer procedures.

## Security reports

Follow the [security policy](SECURITY.md) to report vulnerabilities privately.

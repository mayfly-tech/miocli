# Contributing

Contributions to `miocli` are welcome. Discuss larger behavior changes in an issue before opening a pull request.

简体中文 · [贡献指南](CONTRIBUTING.md) · [README](README_EN.md)

## Issues and pull requests

Search existing issues first to avoid duplicates. Use the [bug report form](.github/ISSUE_TEMPLATE/bug_report.yml) or [feature request form](.github/ISSUE_TEMPLATE/feature_request.yml), both with Chinese and English prompts. Blank issues are disabled.

Bug reports require miocli/Mihomo versions, OS and architecture, connection mode, minimal reproduction steps, and expected and actual behavior. If miocli is not yet installed, enter the target version; the Mihomo version may be "unknown" or "not applicable" if unavailable. Logs and screenshots are optional and must be redacted. Feature requests should explain the problem, proposed solution and any alternatives considered.

Complete the automatically loaded [PR template](.github/pull_request_template.md) with the problem, changes, and actual validation commands and results. Use `Closes #123` for related issues when appropriate; not every PR needs an issue. Explain skipped or inapplicable checks; Go tests may be marked not applicable for documentation/template-only changes. Run integration tests only against a dedicated disposable Mihomo instance, never your daily instance.

Public text, logs, screenshots and configuration snippets must not contain controller secrets, subscription URLs, private configuration (including storage settings) or real user traffic data. Do not paste full configurations. Follow the private security reporting process below rather than disclosing exploitable details in the public bug form.

## Development setup

- Go 1.24 or newer.
- Unit tests use local test servers and fake clients. A Mihomo instance is needed only for manually run integration tests.

## Commit messages

Before your first commit, you can install the local commit message check with `sh scripts/install-hooks.sh`. Commit messages follow Conventional Commits, for example `feat(cli): improve controller diagnostics`. See the [development guide](docs/development_EN.md#commit-messages-and-git-hooks) for details.

## Checks before opening a pull request

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
```

PRs should explain the user problem, behavior change and verification performed. Add or update tests for behavior changes. Do not include controller secrets, private configuration or real user traffic data in examples, tests, screenshots or logs.

Keep changes focused. New direct dependencies need a rationale and a compatible license. Update the README and relevant guides when user-visible behavior changes.

## Compatibility

Release targets are Linux, macOS and Windows on amd64 and arm64. CI tests against the current and previous stable Mihomo releases. API behavior changes require tests using an `httptest.Server`.

## Security reports

Report vulnerabilities privately through GitHub Security Advisories. Do not publish exploitable details in an issue before a fix is available.

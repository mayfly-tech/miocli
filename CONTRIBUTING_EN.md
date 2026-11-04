# Contributing

Contributions to `miocli` are welcome. Discuss larger behavior changes in an issue before opening a pull request.

简体中文 · [贡献指南](CONTRIBUTING.md) · [README](README_EN.md)

## Development setup

- Go 1.24 or newer.
- Unit tests use local test servers and fake clients. A Mihomo instance is needed only for manually run integration tests.

## Commit messages

Before your first commit, you can install the local commit message check with `sh scripts/install-hooks.sh`. Commit messages follow Conventional Commits, for example `feat(cli): add profile command`. See the [development guide](docs/development_EN.md#commit-messages-and-git-hooks) for details.

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

# Releasing

English · [简体中文](zh-CN/releasing.md) · [Documentation](README.md)

This guide describes the checked-in workflows, not a claim that a particular GitHub run or repository permission has been verified.

## Automation

The [Release workflow](https://github.com/mayfly-tech/miocli/blob/main/.github/workflows/release.yml) runs on pushes to `main`. [release-please configuration](https://github.com/mayfly-tech/miocli/blob/main/release-please-config.json) uses the single-package Go strategy, `v`-prefixed tags without a component prefix, and a bootstrap commit. The [manifest](https://github.com/mayfly-tech/miocli/blob/main/.release-please-manifest.json) tracks the published version.

1. Conventional Commits drive release-please's release PR, version manifest and generated `CHANGELOG.md`. `fix`, `feat` and breaking changes drive versioning; documentation/CI/chore-only changes do not normally create a release on their own.
2. Maintainers review and merge the release PR. Do not manually edit historical generated changelog entries or create an extra release tag.
3. release-please creates the tag and published GitHub Release.
4. The same run uses its `sha`, `tag_name` and `release_created` outputs for reusable CI, builds, installers and asset upload.

Release runs are serialized and do not cancel an ongoing run. The Release is published **before** assets finish building; it is not install-ready until all required assets are uploaded.

## Preflight CI

The reusable [CI workflow](https://github.com/mayfly-tech/miocli/blob/main/.github/workflows/ci.yml) validates the released SHA:

- Go 1.24.x and stable: formatting, `go vet`, race tests and dependency-license checks.
- Cross-builds for Linux/macOS/Windows on amd64 and arm64.
- Full native Go tests on Linux/macOS/Windows.
- Shell installer behavior and PowerShell behavior under both Windows PowerShell and PowerShell; installer syntax checks.
- Integration tests against the current and previous published stable Mihomo releases in disposable containers.

These configured checks are not a guarantee for every distribution, terminal or historical controller.

## Packages and assets

Each target uses `CGO_ENABLED=0`, `-trimpath` and ldflags injecting version, commit and UTC build date. Development builds are not automatically release-versioned.

| Platform | Archive name |
| --- | --- |
| Linux | `miocli_X.Y.Z_linux_{amd64,arm64}.tar.gz` |
| macOS | `miocli_X.Y.Z_darwin_{amd64,arm64}.tar.gz` |
| Windows | `miocli_X.Y.Z_windows_{amd64,arm64}.zip` |

The archive contains a matching top-level directory, binary, bilingual README, contributing and security policies, `docs/`, `LICENSE` and `THIRD_PARTY_LICENSES`. Source files, workflow files and development fixtures are repository links, not bundled sources.

Export third-party licenses for each target without skipping Windows-only dependencies. `go-localereader` is pinned to a revision containing the upstream license so Windows license export can succeed.

Uploads consist of **14 assets**: six archives, six matching `.sha256` files, and `install.sh` / `install.ps1`. Installer templates have `__MIOCLI_REPOSITORY__` replaced with the workflow repository. The upload job checks each expected file is nonempty and verifies checksums before `gh release upload --clobber`.

Documentation moves must update the archive copy list as well as repository links. Keep relative user-guide links usable after extraction. SHA-256 is integrity checking from the same distribution source, not an independent package signature.

## Permissions and event behavior

The default `GITHUB_TOKEN` is used. release-please has contents/issues/pull-requests write permissions; upload has contents write; remaining jobs are read-only. Repository settings must allow GitHub Actions to create pull requests.

PRs/tags/releases made with the default token do not automatically trigger additional Actions runs. Therefore publishing CI/build/upload happens within the same workflow, not through a separate tag event. Human merging of a release PR triggers the next `main` push.

The automatically created release PR does not itself automatically receive CI from a new PR event. If branch protection requires those checks, maintainers must arrange check triggering or deliberately configure a suitable token/GitHub App. The checked-in workflow neither adds those credentials nor bypasses protection.

## Recovery

If preflight, build or upload fails after release creation:

1. Inspect the failed job and missing assets.
2. Correct the cause and use **Re-run failed jobs** on the same run when applicable, preserving successful release-please outputs.
3. Partial uploads can be replaced at the same version by `--clobber`.
4. Verify all 14 assets before announcing installation readiness.

Do not blindly rerun every job: an existing release will not necessarily emit `release_created=true` again. Failures after publication can leave latest installation broken until assets are restored. Do not claim publication success from local tests alone.

## Acceptance

- Confirm CI and the complete Release run succeeded for the intended SHA/tag.
- Confirm six packages/checksums and both configured installers exist.
- Verify archive digests and inspect binary, license and bilingual documentation contents.
- Test install/upgrade in an isolated directory; close Windows processes before replacement.
- Check `miocli version` against the intended version/commit.
- Check repository links, extracted-archive relative links and language switches.
- Confirm version/network/checksum failures preserve the existing CLI; distinguish post-install setup failure from installation failure.

No local check validates GitHub permissions, branch protection, release PR creation or actual remote publication. Those require a successful real workflow run. See [installation](installation.md) for user-facing installation and [development](development.md) for local tests.

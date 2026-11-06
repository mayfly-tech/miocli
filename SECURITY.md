# Security policy

English · [简体中文](SECURITY.zh-CN.md) · [Project home](README.md)

## Report a vulnerability

Report suspected vulnerabilities privately through [GitHub Security Advisories](https://github.com/mayfly-tech/miocli/security/advisories). Do not disclose exploitable details in public issues or pull requests before a fix is available.

Include the affected miocli version, platform, minimal reproduction, expected impact and relevant redacted diagnostics. Remove controller secrets, subscription tokens, private endpoints, configuration and traffic data. Share only the information needed to reproduce the issue.

Security fixes target the current release. Older releases do not receive a separate maintenance stream; upgrade to the release containing the fix. This policy does not promise a response deadline.

## Protect local and controller access

miocli stores secrets, downloaded profiles and proxy restoration snapshots in private local files. It does not use an OS keychain. Do not share these files or assume all diagnostic output is public: paths, endpoints and resource names can identify private infrastructure.

Controller HTTP is limited to loopback; remote connections require certificate-validated HTTPS or an SSH tunnel. Controller requests bypass environment proxies and reject redirects. Keep controller authentication enabled and protect socket access. See [configuration](docs/configuration.md) for transport and permission details.

Subscription profiles can change Mihomo's network configuration when activated. Use trusted sources, inspect downloaded YAML and back up any configured restart file. Local-address checks cannot distinguish an SSH-forwarded controller from a controller on this host; profile activation and system-proxy operations require the actual core and files/listeners to be local.

## Release verification

Installers download over HTTPS and verify co-hosted SHA-256 checksums. Checksums detect corruption but do not independently authenticate a compromised release source. Release packages do not have independent signatures. Download and inspect installers before running them; see [installation](docs/installation.md).

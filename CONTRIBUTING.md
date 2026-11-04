# 贡献指南

简体中文 · [English](CONTRIBUTING_EN.md) · [README](README.md)

欢迎提交 Issue 和 Pull Request。较大的行为变更请先通过 Issue 讨论。

## 开发环境

- Go 1.24 或更新版本。
- 单元测试使用本地测试服务器和 fake client；只有手动运行 Mihomo 集成测试时才需要测试实例。

## 提交前检查

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
```

PR 请说明要解决的问题、行为变化和验证方式。行为变更需添加或更新测试。示例、测试、截图和日志中不得包含控制器密钥、私有配置或真实用户流量数据。

保持改动聚焦。新增直接依赖需说明理由并确认许可证兼容。用户可见行为变化应同步更新 README 和相关指南。

## 兼容性

发布目标为 Linux、macOS、Windows 的 amd64 和 arm64。CI 针对 Mihomo 当前稳定版及前一稳定版测试兼容性。API 行为变更需使用 `httptest.Server` 添加测试。

## 安全问题

漏洞请通过 GitHub Security Advisories 私下报告。在修复发布前，请勿在公开 Issue 中披露可利用细节。

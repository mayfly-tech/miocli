# 贡献指南

简体中文 · [English](CONTRIBUTING_EN.md) · [README](README.md)

欢迎提交 Issue 和 Pull Request。较大的行为变更请先通过 Issue 讨论。

## Issue 与 Pull Request

请先搜索已有 Issue，避免重复报告。新 Issue 使用 [Bug 报告表单](.github/ISSUE_TEMPLATE/bug_report.yml)或[功能请求表单](.github/ISSUE_TEMPLATE/feature_request.yml)，均提供中英双语提示；不开放空白 Issue。

Bug 报告需填写 miocli/Mihomo 版本、系统与架构、连接模式、最小复现步骤、预期和实际行为。尚未安装时填写目标 miocli 版本；Mihomo 版本无法获取时可填“未知”或“不适用”。日志和截图为可选内容，提交前必须脱敏。功能请求请说明要解决的问题、建议方案及考虑过的替代方案（如有）。

创建 PR 时填写自动加载的 [PR 模板](.github/pull_request_template.md)，说明问题、变更和实际验证命令及结果；关联 Issue 时可使用 `Closes #123`，但并非每个 PR 都需要 Issue。未执行或不适用的检查需说明理由，纯文档或模板改动可注明 Go 测试不适用。集成测试只能对专用的一次性 Mihomo 实例运行，不得连接日常实例。

公开的文字、日志、截图和配置片段不得包含控制器密钥、订阅地址、私有配置（包括存储配置）或真实用户流量数据；不要直接粘贴完整配置。安全漏洞遵循下方的私下报告流程，不使用公开 Bug 表单披露可利用细节。

## 开发环境

- Go 1.24 或更新版本。
- 单元测试使用本地测试服务器和 fake client；只有手动运行 Mihomo 集成测试时才需要测试实例。

## 提交信息

首次提交前可运行 `sh scripts/install-hooks.sh` 安装本地提交信息检查。提交信息遵循 Conventional Commits，例如 `feat(cli): improve controller diagnostics`。详见[开发指南](docs/development.md#提交信息与-git-hooks)。

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

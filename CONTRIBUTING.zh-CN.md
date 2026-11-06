# 贡献指南

[English](CONTRIBUTING.md) · 简体中文 · [文档](docs/zh-CN/README.md)

欢迎为 miocli 贡献。请保持改动聚焦，较大的行为变更先通过 Issue 讨论，再开始实现。

## 报告问题与建议功能

请先搜索已有 Issue。使用 [Bug 报告](https://github.com/mayfly-tech/miocli/issues/new?template=bug_report.yml)或[功能请求](https://github.com/mayfly-tech/miocli/issues/new?template=feature_request.yml)表单；不开放空白 Issue。

Bug 报告应包含：

- miocli 版本（`miocli version`），安装失败时填写目标版本。
- 可获取的 Mihomo 版本，否则注明“未知”或“不适用”。
- 系统与架构；界面问题还需终端名称及版本。
- 连接模式、最小复现命令/按键、预期与实际行为。
- 必要的已脱敏诊断信息或截图。

功能建议应说明用户问题、期望行为和替代方案。请遵循[产品边界](README.zh-CN.md#功能与边界)：miocli 操作已有控制器，不管理内核生命周期。

不要提交控制器密钥、订阅地址、私有配置（包括存储设置）、完整配置文件或真实流量数据。资源名称和截图同样需要检查。

## 开发与验证

使用 Go 1.24 或更新版本。单元测试使用 fake client 和本地 HTTP 服务器，不需要运行中的 Mihomo。[开发指南](docs/zh-CN/development.md)包含本地设置、定向测试和可选 Git hook。

Go 改动先运行相关检查，再按需要在提交前扩展至完整检查：

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
```

示例使用 POSIX shell；Windows 请选择合适的输出路径并格式化改动的 Go 文件。集成测试**只能连接专用的一次性实例**，不能使用日常控制器。

行为变化应添加或更新测试；API 变化需 `httptest.Server` 覆盖。新增直接依赖需要理由和兼容许可证。

## 文档改动

英文为默认语言，同次改动需同步完整中文版：

- `README.md` ↔ `README.zh-CN.md`。
- `CONTRIBUTING.md` ↔ `CONTRIBUTING.zh-CN.md`。
- `SECURITY.md` ↔ `SECURITY.zh-CN.md`。
- 持续维护的指南：`docs/*.md` ↔ `docs/zh-CN/*.md`。

保持命令、条件、默认值、警告和链接语义一致。描述依据真实实现及测试，而非计划功能。Groups、Selector 等界面标识保持原文。`CHANGELOG.md` 仍由 release-please 生成，`LICENSE` 不翻译。

纯文档改动检查链接、锚点、示例和双语覆盖，可注明 Go 测试不适用。移动页面时需同步模板及[发布文档打包](docs/zh-CN/releasing.md#包与资产)，不只更新 README。

## 提交 Pull Request

填写 PR 模板，说明问题、变更、实际验证命令与结果。解释未执行或不适用的检查。适用时用 `Closes #123` 关联 Issue，不要求每个 PR 都先创建 Issue。

使用 [Conventional Commits](docs/zh-CN/development.md#提交信息与-hooks)，例如 `docs: clarify controller setup`。不要混入无关格式修改或自动生成版本变更。

发布目标为 Linux、macOS、Windows 的 amd64/arm64。CI 测试当前和上一稳定版 Mihomo；这是测试策略，不是支持所有历史版本的承诺。维护流程见[发布指南](docs/zh-CN/releasing.md)。

## 安全报告

请按[安全政策](SECURITY.zh-CN.md)私下报告漏洞。

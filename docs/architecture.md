# 架构与实现

简体中文 · [English](architecture_EN.md) · [README](../README.md)

## 模块

```text
cmd/miocli
  └── internal/cli
        ├── internal/config
        ├── internal/mihomo
        └── internal/app
              └── internal/theme
```

- `cmd/miocli` 处理进程信号和最终错误输出。
- `internal/cli` 定义 Cobra 命令，解析运行配置并装配 TUI。
- `internal/config` 选择配置文件、解析 YAML 和环境变量、校验 controller 地址；`connection.go` 只读提取指定 Mihomo YAML 的连接信息并提供诊断提示。
- `internal/mihomo` 实现控制 API HTTP 客户端、DTO 和错误分类，不依赖 TUI。
- `internal/app` 使用 Bubble Tea Model 管理界面状态；`internal/theme` 提供终端样式。

网络请求由 Bubble Tea 命令异步执行，结果通过有类型的消息更新 Model。UI 不直接构造 HTTP 请求或写源 YAML。生产代码没有内核生命周期或完整配置档案模块。

## 配置和启动

- 普通启动使用用户配置；`dev` 使用仓库本地 `config.local.yaml`；显式 `--config` 指定唯一配置文件。
- 服务地址按 `--server`、非空 `MIOCLI_SERVER`、YAML、默认值的顺序解析；密钥按非空 `MIOCLI_SECRET`、YAML、空值的顺序解析。
- 应用配置仅包含 `server` 和 `secret`，不支持的配置字段会明确报错。
- `setup` 只配置已有 controller，手动输入或指定 YAML 只读提取连接信息；API 检查和保存确认成功后写入操作台自身配置。
- 地址覆盖只对本次连接生效；普通启动和 `dev` 都不下载、启动或停止内核。退出只关闭操作台。
- 节点和模式写操作通过 API 读回确认；provider 更新后读回 providers 并刷新代理数据，使用 generation 丢弃变更前的在途结果。超时写请求不自动重放。

## 发布分发

- POSIX 安装器读取 GitHub latest 的规范版本重定向（不新增 JSON 解析依赖）；PowerShell 使用 GitHub Release API。显式版本跳过查询。
- 发布包与 SHA-256 校验文件均从 GitHub 下载；版本、网络或校验错误会终止安装，不替换已有 CLI。
- `main` 的 Conventional Commits 驱动 release-please 创建发布 PR，更新 manifest 与变更日志。合并后自动创建 `vX.Y.Z` 标签及正式 GitHub Release。
- 同一发布工作流使用 release-please 输出的 SHA 执行 CI、六平台构建与安装器生成，校验后上传到对应 Release。默认 Token 不触发额外的 Tag/Release 工作流；正式 Release 创建至资产上传完成之间暂不可安装。

分发下载与控制 API 的网络策略不同；控制 API 仍不使用环境代理。分发使用 HTTPS；同源发布包校验不替代独立签名，当前不包含包签名。

## Mihomo API

| 操作 | API |
| --- | --- |
| 读取版本、代理组、模式 | `GET /version`、`GET /proxies`、`GET /configs` |
| 切换 Selector 成员 | `PUT /proxies/{group}`，之后重读 `/proxies` |
| 切换模式 | `PATCH /configs`，仅接受 `rule`、`global`、`direct`，之后重读 `/configs` |
| 测试节点延迟 | `GET /proxies/{name}/delay` |
| 测试 provider 独有节点延迟 | `GET /providers/proxies/{provider}/{name}/healthcheck` |
| 读取/更新 providers | `GET /providers/proxies`、`PUT /providers/proxies/{name}` |
| 读取活跃路由 | `GET /connections`，仅显示连接链路摘要 |

HTTP 仅允许 loopback；HTTPS 使用系统证书并验证主机名。HTTP 客户端不使用环境代理、不跟随重定向；响应体限制为 1 MiB。错误信息不包含响应正文。

## 测试

配置、CLI 和 TUI 使用单元测试及 fake client；API 客户端使用 `httptest.Server`。可选 Mihomo 集成测试对隔离实例执行实际 API 操作；CI 在测试环境独立运行当前与上一稳定版本，不向生产代码引入内核管理。CI 还运行格式、静态检查、竞态测试、跨平台构建和安装器检查。

更多测试说明见[开发指南](development.md)。

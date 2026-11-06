# 架构与 API

[English](../architecture.md) · 简体中文 · [文档](README.md)

## 范围与模块

miocli 是单个已有 Mihomo 控制器的客户端，拥有连接设置和操作台状态，不拥有外部内核进程或完整运行配置，避免与已有客户端争夺生命周期及配置管理权。

```text
cmd/miocli
  └── internal/cli
        ├── internal/config
        ├── internal/mihomo
        ├── internal/profiles → config
        ├── internal/systemproxy → config
        └── internal/app → mihomo, theme
```

| 包 | 职责 |
| --- | --- |
| `cmd/miocli` | 进程上下文/信号、最终错误输出 |
| `internal/cli` | Cobra 命令、终端提示、连接检查、TUI 装配 |
| `internal/config` | 文件选择、严格解析、URL 验证、私有写入、只读连接提取 |
| `internal/mihomo` | HTTP 传输、DTO、领域接口、错误分类、必需 API 探测 |
| `internal/profiles` | HTTPS YAML 下载、私有存储、更新、生效档案激活和持久化 |
| `internal/app` | Bubble Tea 状态、资源导航、轮询、写操作、测速 worker、渲染 |
| `internal/systemproxy` | 平台代理设置、私有快照和恢复 |
| `internal/theme` | 样式、颜色策略、终端安全文本、输入样式 |

源码：[CLI](https://github.com/mayfly-tech/miocli/tree/main/internal/cli)、[配置](https://github.com/mayfly-tech/miocli/tree/main/internal/config)、[API 客户端](https://github.com/mayfly-tech/miocli/tree/main/internal/mihomo)、[操作台](https://github.com/mayfly-tech/miocli/tree/main/internal/app)。这些链接指向仓库，并非发布归档附带的源码。

## 启动与状态流

1. CLI 解析设置，探测 `/version`、`/proxies`、`/configs`。
2. 满足首次运行条件的失败提供交互 setup；否则返回错误。
3. 终端会话创建 API client 与 Bubble Tea Model，使用备用屏幕。
4. Bootstrap 加载版本/代理/模式；providers 和 connections 独立加载。
5. `tea.Cmd` 异步执行网络工作，有类型消息更新 Model。

CLI 按连接、订阅档案和系统代理组织命令。`internal/profiles` 负责激活时的配置准备、重载和可选重启配置保存。`internal/app` 内分别组织消息分发、导航、轮询、写操作、测速 worker 和资源渲染。

UI 不构造 HTTP 请求，不改写 Mihomo 源 YAML。[配置](configuration.md)集中说明路径、优先级、导入和传输规则。

## 读写与并发

- 正常轮询每 5 秒调度。Proxies 与 mode 并发读取；providers、connections 各有独立加载/错误状态。
- 代理刷新失败保留旧数据；临时代理故障按 1/2/4 秒有限重试，其他故障暂停自动代理刷新。可选资源错误独立处理。
- 节点选择写入后重读 proxies，选择处理器比较返回成员与请求成员。
- 模式写入后读取当前 mode；只有读回值与请求模式匹配才确认成功，不匹配和读回失败会明确报告。
- Provider 更新后重读 providers，并发起代理/连接刷新，包括写结果不确定时。
- 在使用 generation/serial 的路径拒绝过期异步结果；在途操作检查避免冲突写入，超时写请求不重放。
- 延迟检查最多 10 个 worker，使用可取消上下文和 provider 感知目标。取消保留已完成结果，新测试/写入可能重置。

资源浏览、已确认的分组选择、真实活跃连接链路是独立状态，不从光标推断真实路由。可见行为和限制见[使用指南](usage.md)。

## Mihomo API

| 操作 | 请求与读回 |
| --- | --- |
| 必需连接探测 | `GET /version`、`GET /proxies`、`GET /configs` |
| 选择 Selector 成员 | `PUT /proxies/{group}` 携带 `name`，然后 `GET /proxies` |
| 变更模式 | `PATCH /configs` 携带 `mode`，然后 `GET /configs` |
| 全局节点延迟 | `GET /proxies/{name}/delay?url=…&timeout=…` |
| provider 独有节点延迟 | `GET /providers/proxies/{provider}/{name}/healthcheck?url=…&timeout=…` |
| 列出 providers | `GET /providers/proxies` |
| 更新 provider | `PUT /providers/proxies/{name}`，再刷新 provider 和代理 |
| 读取运行时端口 | `GET /configs` |
| 重载已下载档案 | `PUT /configs?force=true` 携带本机配置路径 |
| 活跃路由摘要 | `GET /connections`，仅展示连接链 |

资源路径段经过转义。模式写入只接受 `rule`、`global`、`direct`。探测要求 Mihomo `meta: true` 及非空模式；可选 API 不属于 doctor 成功条件。

不在全局 proxies 的 provider 独有节点使用 provider/name 身份。节点由组成员、其余全局项及 provider 项合成，默认顺序不是未修改的服务端数组。

## 传输与错误处理

HTTP 仅回环；HTTPS 使用系统证书信任、主机名验证和 TLS 1.2 或以上。控制器客户端忽略环境代理，拒绝重定向。拨号超时 2 秒，TLS 握手 3 秒，普通请求 5 秒（档案重载 120 秒），响应上限 1 MiB。

非空密钥使用 Bearer 认证。API 错误区分认证、未找到、超时、网络、解码和其他 HTTP 故障，错误不包含响应正文。这不代表所有诊断都可安全分享：端点、路径、资源名称仍可能私密。

安装下载使用独立网络策略：HTTPS 与同源 SHA-256，没有独立发布包签名。

## 边界与验证

生产模块不提供内核生命周期、TUN、完整配置合并、多控制器、流量图、日志流或关闭连接。档案重载仅支持本机 Mihomo 控制器及内核进程可读取的已下载 YAML 文件。系统代理按 macOS `networksetup`、GNOME `gsettings`、Windows 当前用户 Internet Settings 分别实现；保存在 miocli 私有状态中的原值用于关闭时恢复。

测试使用 fake client、`httptest.Server` 覆盖配置、端点转义、错误、轮询、过期消息、导航和渲染。可选集成测试连接一次性 Mihomo；CI 运行当前及上一稳定版本。测试容器属于基础设施，不是产品能力。

测试见[开发](development.md)，分发见[发布](releasing.md)。兼容性策略不承诺所有历史 Mihomo 版本或所有平台环境。

# 架构与实现

简体中文 · [English](architecture_EN.md) · [README](../README.md)

## 模块

```text
cmd/miocli
  └── internal/cli
        ├── internal/config
        ├── internal/core
        ├── internal/mihomo
        ├── internal/profile
        └── internal/app
              └── internal/theme
```

- `cmd/miocli` 处理进程信号和最终错误输出。
- `internal/cli` 定义 Cobra 命令，解析运行配置并装配 TUI。
- `internal/config` 选择配置文件、解析 YAML 和环境变量、校验地址与托管策略。
- `internal/core` 发现、校验和启动托管内核；只清理本次启动的子进程。
- `internal/mihomo` 实现控制 API HTTP 客户端、DTO 和错误分类，不依赖 TUI。
- `internal/profile` 保存配置档案，执行来源更新、校验、热加载和事务恢复。
- `internal/app` 使用 Bubble Tea Model 管理界面状态；`internal/theme` 提供终端样式。

网络请求由 Bubble Tea 命令异步执行，结果通过有类型的消息更新 Model。UI 不直接构造 HTTP 请求或写档案 YAML。

## 配置和启动

- 普通启动使用用户配置；`dev` 使用仓库本地 `config.local.yaml`；显式 `--config` 指定唯一配置文件。
- 服务地址按 `--server`、非空 `MIOCLI_SERVER`、YAML、默认值的顺序解析；密钥按非空 `MIOCLI_SECRET`、YAML、空值的顺序解析。
- `core.mode` 默认 `connect`。地址覆盖只对本次连接生效；`dev` 不管理内核。
- `setup` 显式选择连接已有内核或托管内核。托管模式先确认副作用、验证配置并检查 API，再保存管理配置。
- 已配置内核的连接错误不会触发自动安装或启动第二个实例。托管启动只在 API 连接被拒绝时启动子进程；退出时只停止自有子进程。
- 只有本次启动的托管子进程拥有完整档案写权限。档案应用先记录受保护事务，执行一次热加载并读回状态；超时或读回失败时保留事务，不自动重放写请求。

## Mihomo API

| 操作 | API |
| --- | --- |
| 读取版本、代理组、模式 | `GET /version`、`GET /proxies`、`GET /configs` |
| 切换 Selector 成员 | `PUT /proxies/{group}`，之后重读 `/proxies` |
| 切换模式 | `PATCH /configs`，仅接受 `rule`、`global`、`direct`，之后重读 `/configs` |
| 测试节点延迟 | `GET /proxies/{name}/delay` |
| 读取/更新 providers | `GET /providers/proxies`、`PUT /providers/proxies/{name}` |
| 读取活跃路由 | `GET /connections`，仅显示连接链路摘要 |
| 应用配置档案 | `PUT /configs`，传入受控目录中的候选配置路径 |

HTTP 仅允许 loopback；HTTPS 使用系统证书并验证主机名。HTTP 客户端不使用环境代理、不跟随重定向；响应体限制为 1 MiB。错误信息不包含响应正文。

## 测试

配置、CLI、core、profile 和 TUI 使用单元测试及 fake client；API 客户端使用 `httptest.Server`。可选 Mihomo 集成测试对隔离实例执行实际 API 操作。CI 还运行格式、静态检查、竞态测试、跨平台构建和安装器检查。

更多测试说明见[开发指南](development.md)。

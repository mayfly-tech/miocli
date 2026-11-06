# 排障

[English](../troubleshooting.md) · 简体中文 · [文档](README.md)

先运行 `miocli version` 和 `miocli doctor`。自定义配置使用 `miocli doctor --config PATH`，文件需已存在且合法。Doctor 只读并隐藏密钥，但路径和控制器地址仍可能需要脱敏。

## 安装

| 症状 | 检查与处理 |
| --- | --- |
| 找不到 `miocli` | 安装器不修改 PATH；使用打印的完整路径，或将实际安装目录加入 PATH |
| 仓库未配置 | 使用 Release 附带安装器，不使用含占位符的源码模板 |
| latest 查询失败 | 检查 GitHub 访问；指定已发布版本可跳过查询，但仍需下载包/校验文件 |
| Release 已存在但资产 404 | 等待工作流完成；维护者参阅[恢复](releasing.md#恢复) |
| 校验文件非法/摘要不一致 | 停止，不禁用校验；从可信发布获取匹配的包/校验文件并调查 |
| 替换需要确认 | 有意传入 `--yes` / `-Yes`；非交互模式本身不代表同意 |
| Windows 无法替换程序 | 关闭运行中的 miocli，检查目录可写后重试 |
| 安装后 setup 失败 | 程序已安装；修正连接设置后重跑 `miocli setup` |

## 配置与连接

| 症状 | 检查与处理 |
| --- | --- |
| 连接被拒绝 | 检查拥有内核的客户端正在运行，且启用了**控制 API** 端口；miocli 不启动它 |
| 认证/权限错误 | 检查控制器密钥和非空 `MIOCLI_SECRET`；环境可能覆盖文件改动 |
| 地址/密钥来源不符预期 | 查看 doctor 来源和[优先级](configuration.md#优先级)；空环境值不会清除文件值 |
| 显式配置缺失 | 创建合法文件，或省略 `--config` 使用默认路径；setup 同样先解析文件 |
| 密钥文件权限错误 | Unix 对所选文件执行 `chmod 600`；环境覆盖不绕过检查 |
| 未知字段 | miocli 设置使用 `server`、`secret` 和可选的 `active-profile-path`；删除其不支持字段，不要误改拥有内核的客户端 YAML |
| 端点不兼容 | 确认是 Mihomo API 而非 HTTP/SOCKS 端口；doctor 要求 `/version` 的 `meta: true`、`/proxies`、`/configs` 非空模式 |
| 远程 HTTP 被拒绝 | 使用验证证书的 HTTPS 或回环 [SSH 隧道](configuration.md#远程控制器)，不要绕过限制 |
| TLS 失败 | 检查证书信任、主机名及 HTTPS 监听；无跳过 TLS 验证选项 |
| 环境代理不生效 | 控制器请求有意忽略环境代理，请用直连 HTTPS 或隧道 |
| 重定向或路径前缀 API 失败 | 设置最终 API origin；URL 不含 API 路径，且不会跟随重定向 |
| 读取超时/响应无法识别 | 检查可达性和 API 兼容；普通请求 5 秒、profile 重载 120 秒，响应上限 1 MiB |
| Profile 重载失败/超时 | 运行态可能已改变；miocli 不自动重试或保存重启文件。私下检查核心日志中的 provider/Geo 下载、DNS 和配置错误，再运行 doctor 并检查实际网络。错误会指出保留的私有生效 YAML，待核心处理结束后手动删除；必要时通过所属服务恢复备份的重启配置 |
| YAML 导入失败 | 检查为 16 MiB 内的普通文件、单文档映射，以及支持的 `external-controller` 或绝对路径 `external-controller-unix`；通配/TLS/pipe 情况手动输入实际 URL |

求助时不要贴完整 YAML。Setup 只提取连接字段，不能将完整档案转换成 miocli 支持的设置。

## 操作台

| 症状 | 解释或处理 |
| --- | --- |
| 需要交互终端 | 在终端运行操作台/setup，不重定向输入输出；非交互检查使用 doctor |
| 窗口太小 | 调整至至少 50 列 × 12 行；仍可 `q` / `Ctrl+C` 退出 |
| 无颜色 | 检查 `--no-color`、非空 `NO_COLOR`、`TERM=dumb` |
| 高亮节点未生效 | Cursor 与选择不同；在 Groups 的 `Selector` 中按 Enter 选择，再检查读回结果 |
| Enter 不进入嵌套组 | Enter 选择该成员，Right/l 才进入 |
| 无法手动选择 | 自动组类型不允许手动选择 |
| Groups 的 `T` 无效 | 用 Space 测试组直接成员；T 仅用于 Nodes，紧凑页脚可能误导 |
| 数据过期/重连暂停 | 检查 API/密钥，再按 `r`；可能仍显示上次成功数据 |
| Doctor 成功但 Providers/Connections 失败 | Doctor 只检查三个必需 API，不含这些可选资源端点 |
| 延迟失败或 `No API` | 检查 Mihomo 延迟/healthcheck 端点及其到测试 URL 的可达性，见[测速语义](usage.md#延迟测试测量什么) |
| 更新/选择/模式结果不确定 | 先检查刷新状态再手动重试；不会自动重放写请求，通知不能证明实际路由 |
| 无活跃连接 | 可以是正常状态；路由摘要来自真实活跃连接，不是光标或所有可能路由 |

切换模式后应检查界面/读回模式，而不只依赖确认通知。路由应检查实际连接链路；已有连接不一定改走新选成员。

## 安全报告问题

遵循[贡献指南](../../CONTRIBUTING.zh-CN.md)，提供版本、系统/架构、界面问题的终端信息、最小步骤、预期/实际行为及相关脱敏诊断。

移除密钥、订阅地址、私有配置、存储信息和真实流量数据，资源名称和截图也可能敏感。可利用漏洞通过 [GitHub Security Advisories](https://github.com/mayfly-tech/miocli/security/advisories) 私下报告。

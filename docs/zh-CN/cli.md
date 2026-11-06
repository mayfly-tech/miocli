# 命令行参考

[English](../cli.md) · 简体中文 · [文档](README.md)

使用 `miocli --help` 或 `miocli COMMAND --help` 查看已安装版本的帮助。位置参数见下方命令表；其余命令不接受位置参数。

## 命令

| 命令 | 行为 |
| --- | --- |
| `miocli` | 解析设置、探测控制器、打开全屏操作台 |
| `miocli setup` | 交互检查，确认后保存连接设置 |
| `miocli doctor` | 打印配置来源，探测三个必需的只读 API |
| `miocli dev` | 使用仓库本地 `config.local.yaml`，除非 `--config` 指定其他文件 |
| `miocli profile list` | 列出本机保存的订阅档案，不显示订阅 URL |
| `miocli profile add NAME HTTPS_URL` | 下载并保存一个 HTTPS Mihomo YAML 档案 |
| `miocli profile update NAME` | 下载档案最新内容 |
| `miocli profile use NAME` | 通过本机控制器重载档案，保留 miocli 的控制器和 secret |
| `miocli profile remove NAME` | 删除本机档案及其 URL 记录 |
| `miocli system-proxy status` | 显示系统代理状态 |
| `miocli system-proxy enable` | 将系统代理指向本机 Mihomo 端口 |
| `miocli system-proxy disable` | 恢复启用前保存的系统代理设置 |
| `miocli version` | 打印版本、提交和构建时间 |
| `miocli help [command]` | 不连接控制器，显示帮助 |
| `miocli completion [shell]` | 生成 shell 补全脚本 |

操作台和 setup 需要交互 stdin/stdout。`doctor`、`version` 和帮助不需要 TUI。启动仅在符合[首次运行条件](configuration.md#启动与开发设置)时提供 setup。这些命令都不管理内核进程。档案管理命令的 HTTPS 订阅 URL 及 YAML 存在当前配置文件旁的 `profiles/` 目录，文件仅当前用户可读；列表不打印含令牌的订阅 URL。`profile use` 只支持本机 Mihomo 控制器，因为重载路径必须存在于内核所在主机。

```sh
miocli doctor --config ./config.local.yaml
miocli profile add home https://subscription.example/config.yaml
miocli profile update home
miocli profile use home
miocli system-proxy enable
miocli system-proxy disable
miocli --server https://mihomo.example:9090
miocli --no-color
miocli dev --config ./config.local.yaml
miocli version
```

使用 `--config` 时示例文件必须已存在。HTTPS 主机名仅为示例，不是内置服务。
macOS 命令需要 `--service` 指定网络服务（可用 `networksetup -listnetworkserviceorder` 查看）；如果系统要求管理员授权，命令会返回系统错误，不会自行提权。Linux 通过 GNOME `gsettings` 管理当前桌面用户设置；Windows 修改当前用户 Internet Settings，无需管理员权限。已有启用的代理、PAC 或自动检测配置会被拒绝，不会覆盖。

## 全局参数

| 参数 | 默认值 | 含义 |
| --- | --- | --- |
| `--config PATH` | 平台默认文件 | 选择一个 YAML 文件；显式文件缺失报错 |
| `--server URL` | 环境/文件/默认值 | 只覆盖本次调用的控制 API URL |
| `--no-color` | `false` | 禁用终端颜色 |
| `-h`、`--help` | `false` | 显示帮助 |

没有 `--secret` 或根命令 `--version` 参数。请使用受保护配置/环境和 `version` 子命令。

## 环境变量

| 变量 | 使用者 | 含义 |
| --- | --- | --- |
| `MIOCLI_SERVER` | 连接命令 | 非空地址覆盖 |
| `MIOCLI_SECRET` | 连接命令 | 非空密钥覆盖 |
| `XDG_CONFIG_HOME` | Linux/macOS 配置 | 绝对配置基目录 |
| `APPDATA` | Windows 配置 | 默认配置基目录 |
| `NO_COLOR` | 终端样式 | 非空即禁用颜色 |
| `TERM` | 终端样式 | `dumb` 禁用颜色 |
| `MIOCLI_INSTALL_DIR` | 仅安装器 | 默认安装目录覆盖 |
| `MIOCLI_REPOSITORY` | 仅安装器 | GitHub 分发仓库覆盖 |
| `MIOCLI_INTEGRATION_URL` / `MIOCLI_INTEGRATION_SECRET` | 仅测试 | 专用集成控制器，不是普通运行设置 |

优先级见[配置](configuration.md#优先级)，安装参数见[安装器选项](installation.md#安装器选项)。环境代理变量不转发控制器请求。

## Shell 补全

Cobra 支持生成 Bash、Zsh、Fish、PowerShell 补全。各 shell 安装方法见 `miocli completion --help` 和 `miocli completion SHELL --help`。生成脚本不等于安装脚本，也不会连接控制器。

## 操作台内命令

在界面按 `:` 输入命令。命令栏不是 shell，与进程级 CLI 不同：

```text
groups | nodes | providers | status | help
refresh | mode rule | mode global | mode direct | quit | q
```

命令不区分大小写，按空白分词，不支持的名称或参数会报错。见[使用指南](usage.md#命令栏)。

# miocli

[English](README.md) · 简体中文

面向单个已运行 [Mihomo](https://github.com/MetaCubeX/mihomo) 实例的键盘优先终端操作台。

在本地终端或 SSH 会话中浏览代理组和节点、切换选择与模式、测试延迟、更新 proxy-providers。借鉴 [k9s](https://github.com/derailed/k9s) 面向资源的操作方式，miocli 连接已有控制器，而不接管其生命周期。

## 功能与边界

| 资源或命令 | 日常任务 |
| --- | --- |
| **Groups** | 搜索组和成员、浏览嵌套组、选择 `Selector` 组成员 |
| **Nodes** | 浏览全局及 provider 独有节点，按协议/来源筛选、排序、测试延迟 |
| **Providers** | 查看 proxy-providers，手动请求更新 |
| **Profiles** | 下载和保存 HTTPS Mihomo YAML 订阅档案；本机控制器可切换并重载 |
| **Status** | 查看控制器、已选分组路径、活跃路由摘要和诊断 |

界面为英文，默认进入 Groups。写操作后会读回控制器状态；变更后应检查刷新结果。

**产品边界：** miocli 不安装、启动、停止或升级 Mihomo，不管理 TUN、系统服务或多个控制器。可选的系统代理开关只修改当前用户/网络服务的系统代理配置，并在关闭时恢复原值；不会启停或接管内核生命周期。

## 快速开始

需要已有运行中的 Mihomo、可访问的控制 API，以及已配置的密钥。控制 API **不是** HTTP/SOCKS 代理端口。操作台需要至少 **50 列 × 12 行**的交互终端。

### 1. 安装

下载对应平台的安装器，检查后执行。不要将网络脚本直接管道执行。

**Linux / macOS**

```sh
curl --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --max-redirs 5 --connect-timeout 15 --max-time 60 \
  https://github.com/mayfly-tech/miocli/releases/latest/download/install.sh -o install.sh
# 检查 install.sh 后再继续。
sh install.sh
```

**Windows PowerShell**

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest https://github.com/mayfly-tech/miocli/releases/latest/download/install.ps1 `
  -OutFile install.ps1 -TimeoutSec 60 -MaximumRedirection 5 -UseBasicParsing
# 检查 install.ps1 后再继续。
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

安装器校验 SHA-256，安装到用户目录，不需要管理员权限。**安装器不会修改 PATH。** 请按输出提示设置 PATH，或使用程序完整路径。支持平台、手动安装、升级和卸载见[安装指南](docs/zh-CN/installation.md)。

### 2. 连接

```sh
miocli setup
miocli doctor
miocli
```

`setup` 支持输入控制器 URL 和隐藏密钥，或只读提取指定 Mihomo YAML 中的连接字段。API 检查通过并确认后，才保存 miocli 自身设置。密钥存于受保护的 YAML，而非系统钥匙串。

本地 HTTP 必须使用回环地址；Linux/macOS 还支持 `--server unix:///absolute/control.sock`，受 socket 权限约束。远程控制器需要证书验证的 HTTPS 或 [SSH 隧道](docs/zh-CN/configuration.md#远程控制器)。

### 3. 操作

- `1` / `2` / `3` / `4`：Groups / Nodes / Providers / Status。
- Groups 中 `Enter` 聚焦成员或选择成员；`→` / `l` 进入嵌套组。
- Nodes 中 `Space` 测试单节点；`T` 测试筛选后的节点列表。
- Providers 中 `u` 请求更新。
- `miocli profile add NAME HTTPS_URL` / `update` / `list` / `use` 管理档案。
- `miocli system-proxy enable|disable|status` 管理系统代理。
- `?` 查看帮助；`q` / `Ctrl+C` 退出，不停止 Mihomo。

完整的分视图按键与操作语义见[使用指南](docs/zh-CN/usage.md)。

## 文档

| 从这里开始 | 参考 | 贡献与维护 |
| --- | --- | --- |
| [安装](docs/zh-CN/installation.md) | [配置](docs/zh-CN/configuration.md) | [贡献](CONTRIBUTING.zh-CN.md) |
| [使用](docs/zh-CN/usage.md) | [命令行](docs/zh-CN/cli.md) | [开发](docs/zh-CN/development.md) |
| [排障](docs/zh-CN/troubleshooting.md) | [架构与 API](docs/zh-CN/architecture.md) | [发布](docs/zh-CN/releasing.md) |

[完整中文文档](docs/zh-CN/README.md) · [English documentation](docs/README.md) · [变更日志（英文）](CHANGELOG.md)

## 隐私与安全

无账号、遥测或后台自动更新检查。控制器请求不使用环境代理，也不跟随重定向。配置、资源名称和截图仍可能包含敏感信息，分享前请脱敏。

安装下载使用 HTTPS。同源 SHA-256 文件可发现损坏，但不能独立证明已被攻破的发布源可信；发布包没有独立签名。漏洞私下报告方式和安全修复支持范围见[安全政策](SECURITY.zh-CN.md)。

## 许可证

[MIT](LICENSE)。

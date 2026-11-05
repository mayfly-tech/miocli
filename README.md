# miocli

简体中文 · [English](README_EN.md)

`miocli` 是面向单个已运行 [Mihomo](https://github.com/MetaCubeX/mihomo) 实例的键盘优先终端操作台。面向已有内核的进阶用户，在终端或 SSH 会话中浏览代理资源、切换节点与模式、测试延迟，并查看或更新 providers。它不安装、启动、停止或升级内核。

## 功能

- 全屏英文界面：Groups、Nodes、Providers、Status，默认进入 Groups。
- 浏览嵌套代理组，搜索组和成员；切换 `Selector` 组节点并读取 API 确认结果。
- 按协议或来源筛选节点，按 API 顺序、名称或延迟排序；测试单个节点、筛选结果或当前组的直接成员。
- 在 Rule、Global、Direct 模式间切换；查看和手动更新 Mihomo proxy-providers。
- 连接单个 Mihomo controller；支持回环 HTTP 和经过证书验证的 HTTPS。

## 快速开始

### 安装

从最新的 GitHub Release 下载对应安装脚本：

```sh
# Linux / macOS：先下载到本地，再执行；不要将下载内容直接管道执行
curl --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --max-redirs 5 --connect-timeout 15 --max-time 60 \
  https://github.com/mayfly-tech/miocli/releases/latest/download/install.sh -o install.sh
```

```powershell
# Windows PowerShell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest https://github.com/mayfly-tech/miocli/releases/latest/download/install.ps1 `
  -OutFile install.ps1 -TimeoutSec 60 -MaximumRedirection 5 -UseBasicParsing
```

检查下载的脚本后安装：

```sh
# Linux / macOS
sh install.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

安装器校验发布包 SHA-256，并将 CLI 安装到用户目录，不需要管理员权限。已有 Clash Verge 等客户端时，只需可访问的 Mihomo 控制 API；无需安装第二个内核。

安装器仅从 GitHub 下载发布包与 SHA-256 校验文件；未指定版本时查询最新正式版本。指定 `--version vX.Y.Z`（PowerShell：`-Version vX.Y.Z`）可跳过版本查询。网络、版本或校验错误会明确终止安装，不替换已有 CLI。Release 创建后仍需等待构建资产上传完成才能安装，详见[发布流程](docs/development.md#发布流程)。

### 连接已有内核

```sh
miocli setup
miocli doctor
miocli
```

在 setup 中填写 `external-controller` 地址和 `secret`；这是控制 API 地址，不是 HTTP/SOCKS 代理端口。也可以指定 Mihomo YAML，只读提取这两项。程序不扫描客户端目录、不猜测密钥，也不修改原配置。

无配置时，程序先尝试 `http://127.0.0.1:9090`。连接失败时只在交互终端提供 setup；不会自动安装或启动内核。`doctor` 只读检查配置和 API。

### 本地开发

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
go run ./cmd/miocli dev
```

在本地配置中填写已有 controller 的地址和密钥。`dev` 只连接，不写用户配置、不下载或启动内核。Windows 使用 `Copy-Item` 复制模板，无需 `chmod`。更多说明见[开发指南](docs/development.md)。

### 产品边界

miocli 借鉴 k9s 的资源操作台定位：连接已有实例，浏览资源、执行明确操作并确认结果，不承担内核生命周期。支持节点切换、模式切换和 provider 更新；完整配置订阅、配置热加载、系统代理和 TUN 接管不在产品范围内。详见[产品说明](docs/product.md)。

### 卸载

先退出所有 miocli 会话（`q` 或 `Ctrl+C`）。退出只关闭操作台，不停止外部 Mihomo 实例。

**仅卸载程序，保留连接配置：**

```sh
# Linux / macOS：默认安装位置
rm -f -- "$HOME/.local/bin/miocli"
```

```powershell
# Windows PowerShell：默认安装位置
$binary = Join-Path $env:LOCALAPPDATA "Programs\miocli\miocli.exe"
if (Test-Path -LiteralPath $binary -PathType Leaf) {
    Remove-Item -LiteralPath $binary
}
```

如果安装时使用 `--install-dir` / `-InstallDir` 或 `MIOCLI_INSTALL_DIR`，请删除实际安装目录中的 `miocli` / `miocli.exe`，不要删除整个共享安装目录。安装器不会修改 PATH；如果你手动添加过专用目录，可自行移除对应 PATH 项，勿删除仍供其他工具使用的目录。

**可选：清理连接配置。** 先备份需要保留的配置，再检查并手动删除以下专用配置目录；这会移除保存的连接密钥：

| 平台 | 连接配置 |
| --- | --- |
| Linux / macOS | `$XDG_CONFIG_HOME/miocli`，未设置时为 `~/.config/miocli` |
| Windows | `%AppData%\miocli` |

使用 `--config` 时，只删除确认属于 miocli 的配置文件，不删除共享父目录。清理前先检查和备份，不要误删仍在使用的数据。不要删除外部 Mihomo、Clash Verge 等客户端及其配置，也不要删除原始导入 YAML。卸载不需要清理系统代理、TUN 或系统服务，因为 miocli 不安装或修改它们。

## 配置

配置文件路径：

- Linux/macOS：`$XDG_CONFIG_HOME/miocli/config.yaml`；未设置 `XDG_CONFIG_HOME` 时为 `~/.config/miocli/config.yaml`。
- Windows：`%AppData%\miocli\config.yaml`。

```yaml
server: http://127.0.0.1:9090
secret: ""
```

服务地址优先级：`--server` > 非空 `MIOCLI_SERVER` > YAML > 默认地址。密钥优先级：非空 `MIOCLI_SECRET` > YAML > 不认证。`--config` 指定唯一配置文件，不会与其他配置合并；`--server` 仅对本次连接生效。不要将密钥作为命令行参数传入。

## 快捷键

| 按键 | 操作 |
| --- | --- |
| `1` / `2` / `3` / `4` | Groups / Nodes / Providers / Status |
| `:` | 命令栏，如 `groups`、`nodes`、`providers`、`mode rule` |
| `Tab` / `Shift+Tab` | 切换列表、详情或分组/成员 |
| `↑` / `↓`、`j` / `k` | 移动选择 |
| `Enter` / `→` | 打开详情；选择成员 / 进入嵌套组 |
| `/` | 搜索当前资源 |
| `Space` / `T` | 测试节点或当前组直接成员 |
| `s` / `t` / `f` | 节点排序 / 协议筛选 / 来源筛选 |
| `m` | 切换 Rule、Global、Direct |
| `u` | 在 Providers 中更新当前 provider 并读回状态 |
| `r` / `?` | 刷新或重连 / 显示帮助 |
| `Esc` | 清除搜索、关闭详情或返回 |
| `q` / `Ctrl+C` | 退出 |

Providers 是已有内核的 API 资源，不是完整 YAML 配置档案。完整交互说明见 [README 英文版](README_EN.md#keyboard-shortcuts)。

## 文档

| 文档 | 简体中文 | English |
| --- | --- | --- |
| 产品与功能 | [产品说明](docs/product.md) | [Product](docs/product_EN.md) |
| 架构与实现 | [架构说明](docs/architecture.md) | [Architecture](docs/architecture_EN.md) |
| 开发与测试 | [开发指南](docs/development.md) | [Development](docs/development_EN.md) |
| 贡献 | [贡献指南](CONTRIBUTING.md) | [Contributing](CONTRIBUTING_EN.md) |

## 隐私与安全

miocli 不包含账号、遥测或后台自动更新检查。凭据只发送到配置的控制器。配置和诊断信息可能包含敏感信息，分享前请检查并脱敏。漏洞请通过 GitHub Security Advisories 私下报告。

CLI 安装下载使用经过证书验证的 HTTPS。miocli 发布包与校验文件同源，SHA-256 用于发现传输损坏，不能独立证明被篡改分发源的可信性；发布包不包含独立签名。

## 许可证

MIT，详见 [LICENSE](LICENSE)。

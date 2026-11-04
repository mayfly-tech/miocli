# miocli

简体中文 · [English](README_EN.md)

`miocli` 是面向单个 [Mihomo](https://github.com/MetaCubeX/mihomo) 实例的键盘优先终端控制面板。可浏览代理组和节点、切换节点与代理模式、测试延迟，并查看或更新 providers。

## 功能

- 全屏英文界面：Subscriptions、Groups、Nodes、Status。
- 浏览嵌套代理组，搜索组和成员；切换 `Selector` 组节点并读取 API 确认结果。
- 按协议或来源筛选节点，按 API 顺序、名称或延迟排序；测试单个节点、筛选结果或当前组的直接成员。
- 在 Rule、Global、Direct 模式间切换；查看和手动更新 Mihomo proxy-providers。
- 对本次启动的托管内核管理本地 YAML/HTTPS 档案，确认后验证并热加载。
- 连接单个 Mihomo controller；支持回环 HTTP 和经过证书验证的 HTTPS。

## 快速开始

### 安装

从 GitHub Release 下载对应安装脚本：

```sh
# Linux / macOS
sh install.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

安装器校验发布包 SHA-256，并将 CLI 安装到用户目录，不需要管理员权限。已有 Clash Verge 等客户端时，只需可访问的 Mihomo 控制 API；无需安装第二个内核。

### 连接已有内核

```sh
miocli setup
miocli doctor
miocli
```

在 setup 中选择连接已有内核，填写 `external-controller` 地址和 `secret`；这是控制 API 地址，不是 HTTP/SOCKS 代理端口。也可以指定 Mihomo YAML，只读提取这两项。程序不扫描客户端目录、不猜测密钥，也不修改原配置。

无配置时，程序先尝试 `http://127.0.0.1:9090`。连接失败时只在交互终端提供 setup；不会自动安装或启动内核。`doctor` 只读检查配置和 API。

### 本地开发

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
go run ./cmd/miocli dev
```

在本地配置中填写已有 controller 的地址和密钥。`dev` 只连接，不写用户配置、不下载或启动内核。Windows 使用 `Copy-Item` 复制模板，无需 `chmod`。更多说明见[开发指南](docs/development.md)。

### 托管内核

在 setup 中明确选择 managed，并确认导入配置和下载（如需要）。miocli 管理所选配置的副本；运行时只停止本次启动的子进程。连接失败不会自动切换为托管模式。详见[产品说明](docs/product.md)。

托管模式不修改源配置。导入前需关闭 TUN 和自定义监听，并确保相对路径引用的资源可用；缺少内核时，需单独确认从官方 Release 下载。设置仅在配置验证成功后保存。

## 配置

配置文件路径：

- Linux/macOS：`$XDG_CONFIG_HOME/miocli/config.yaml`；未设置 `XDG_CONFIG_HOME` 时为 `~/.config/miocli/config.yaml`。
- Windows：`%AppData%\miocli\config.yaml`。

```yaml
server: http://127.0.0.1:9090
secret: ""
core:
  mode: connect
```

服务地址优先级：`--server` > 非空 `MIOCLI_SERVER` > YAML > 默认地址。密钥优先级：非空 `MIOCLI_SECRET` > YAML > 不认证。`--config` 指定唯一配置文件，不会与其他配置合并；`--server` 仅对本次连接生效。不要将密钥作为命令行参数传入。

## 快捷键

| 按键 | 操作 |
| --- | --- |
| `1` / `2` / `3` / `4` | Subscriptions / Groups / Nodes / Status |
| `:` | 命令栏，如 `groups`、`nodes`、`providers`、`mode rule` |
| `Tab` / `Shift+Tab` | 切换列表、详情或分组/成员 |
| `↑` / `↓`、`j` / `k` | 移动选择 |
| `Enter` / `→` | 打开详情；选择成员 / 进入嵌套组 |
| `/` | 搜索当前资源 |
| `Space` / `T` | 测试节点或当前组直接成员 |
| `s` / `t` / `f` | 节点排序 / 协议筛选 / 来源筛选 |
| `m` | 切换 Rule、Global、Direct |
| `r` / `?` | 刷新或重连 / 显示帮助 |
| `Esc` | 清除搜索、关闭对话框或返回 |
| `q` / `Ctrl+C` | 退出 |

Subscriptions 中 `p` 切换档案与 providers；档案操作使用 `a` / `e` / `u` / `d`，`Space` 启用。完整交互说明见 [README 英文版](README_EN.md#keyboard-shortcuts)。

## 文档

| 文档 | 简体中文 | English |
| --- | --- | --- |
| 产品与功能 | [产品说明](docs/product.md) | [Product](docs/product_EN.md) |
| 架构与实现 | [架构说明](docs/architecture.md) | [Architecture](docs/architecture_EN.md) |
| 开发与测试 | [开发指南](docs/development.md) | [Development](docs/development_EN.md) |
| 贡献 | [贡献指南](CONTRIBUTING.md) | [Contributing](CONTRIBUTING_EN.md) |

## 隐私与安全

miocli 不包含账号、遥测或后台自动更新检查。凭据只发送到配置的控制器。托管内核的下载需在 setup 中单独确认。配置和诊断日志可能包含敏感信息，分享前请检查并脱敏。漏洞请通过 GitHub Security Advisories 私下报告。

## 许可证

MIT，详见 [LICENSE](LICENSE)。

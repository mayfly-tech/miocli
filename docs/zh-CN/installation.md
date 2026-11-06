# 安装

[English](../installation.md) · 简体中文 · [文档](README.md)

安装 miocli 只会安装操作台。请自行准备运行中的 Mihomo 控制器；Clash Verge 等已有 Mihomo 用户无需安装第二个内核。

## 支持的发布目标

| 平台 | 架构 | 归档 |
| --- | --- | --- |
| Linux | amd64、arm64 | `.tar.gz` |
| macOS（`darwin`） | amd64、arm64 | `.tar.gz` |
| Windows | amd64、arm64 | `.zip` |

这些是构建目标，不保证每个系统版本或终端都已验证。TUI 需要交互输入/输出和至少 50 列 × 12 行。本地可使用回环 HTTP；远程需要 HTTPS 或 SSH 隧道。

## 使用发布脚本安装

使用 [GitHub Releases](https://github.com/mayfly-tech/miocli/releases/latest) 附带的脚本，不使用仓库中未配置的脚本模板。源码模板包含发布时替换的仓库占位符。

### Linux 与 macOS

需要 POSIX shell、`curl`、`tar`、标准工具，以及 `sha256sum` 或 `shasum`。

```sh
curl --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --max-redirs 5 --connect-timeout 15 --max-time 60 \
  https://github.com/mayfly-tech/miocli/releases/latest/download/install.sh -o install.sh
# 检查下载的脚本。
sh install.sh
```

默认位置：`~/.local/bin/miocli`。安装到该用户目录无需管理员权限。

### Windows

使用 Windows PowerShell 或带有所需 Windows 命令的 PowerShell，包括 `Get-CimInstance`、`Get-FileHash`、`Expand-Archive`。CI 同时检查 Windows PowerShell 和 PowerShell。

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest https://github.com/mayfly-tech/miocli/releases/latest/download/install.ps1 `
  -OutFile install.ps1 -TimeoutSec 60 -MaximumRedirection 5 -UseBasicParsing
# 检查下载的脚本。
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

默认位置：`%LOCALAPPDATA%\Programs\miocli\miocli.exe`。替换程序前关闭运行中的 miocli。

### 安装器选项

| 用途 | POSIX shell | PowerShell |
| --- | --- | --- |
| 指定版本 | `--version vX.Y.Z` 或位置参数 | `-Version vX.Y.Z` 或位置参数 |
| 安装目录 | `--install-dir /absolute/path` | `-InstallDir C:\absolute\path` |
| 允许替换 | `--yes` / `-y` | `-Yes` |
| 禁止提示 | `--non-interactive` | `-NonInteractive` |
| 安装后运行 setup | `--setup` | `-Setup` |
| 帮助 | `--help` / `-h` | `-Help` |

将 `vX.Y.Z` 替换为已发布版本。两个安装器均接受带或不带 `v` 的版本。未提供对应目录选项时，`MIOCLI_INSTALL_DIR` 可指定目录，必须是绝对路径。`MIOCLI_REPOSITORY` 用于有意选择其他 GitHub `owner/repository` 分发源，不是控制器地址。

替换已有 CLI 需要确认或 `--yes` / `-Yes`。非交互模式**不代表**同意替换。`--setup` / `-Setup` 需要交互终端；`--yes` 不确认保存连接设置。交互安装即使未指定 setup 选项，也可能询问是否运行 setup。

## PATH 与验证

安装器**从不修改 PATH**，请遵循打印的提示。默认 POSIX 路径可在当前 shell 中启用：

```sh
export PATH="$HOME/.local/bin:$PATH"
miocli version
```

默认 Windows 路径：

```powershell
$env:Path = "$env:LOCALAPPDATA\Programs\miocli;$env:Path"
miocli version
```

长期生效需将实际目录加入 shell 配置或 Windows 用户 PATH，再打开新终端；也可直接调用程序完整路径。

依次运行 `miocli setup`、`miocli doctor`、`miocli`。远程暴露控制器前先阅读[配置指南](configuration.md)。

## 手动安装与完整性

从同一 Release 下载 `miocli_X.Y.Z_OS_ARCH.tar.gz`（Windows 为 `.zip`）及同名 `.sha256`。macOS 的 OS 名为 `darwin`；归档版本号不含开头的 `v`。

**先校验，再解压**。使用 `sha256sum --check`、`shasum -a 256`，或 PowerShell `Get-FileHash -Algorithm SHA256`，核对准确的期望摘要。解压后将 `miocli` / `miocli.exe` 放到自己控制的目录。保留或再分发包时，一并保留 `LICENSE`、`THIRD_PARTY_LICENSES` 和附带文档。

脚本通过 HTTPS 从 GitHub 下载包和校验文件，验证 SHA-256 后才替换二进制。同源校验发现损坏，不能独立认证分发源；发布包无独立签名。

## 升级与回退

退出 miocli，重新运行检查过的发布安装器。省略版本时查询最新正式版本；指定某个已发布版本可升级或回退。无人值守替换需明确同意。连接设置单独存储，不因二进制替换而重写。

版本查询限制 15 秒，包和校验文件请求各限制 30 秒。指定版本可跳过 latest 查询。网络、版本和完整性失败保留旧 CLI。安装后的 setup 失败时，新 CLI 已安装，请单独重跑 setup。

正式 Release 发布后，构建期间可能暂缺资产。请等待完整流程成功，见[发布恢复](releasing.md#恢复)。

## 卸载

先退出全部 miocli 会话（`q` / `Ctrl+C`），不会停止 Mihomo。

只删除实际位置的二进制：

```sh
# Linux/macOS 默认位置。
rm -f -- "$HOME/.local/bin/miocli"
```

```powershell
# Windows 默认位置。
$binary = Join-Path $env:LOCALAPPDATA "Programs\miocli\miocli.exe"
if (Test-Path -LiteralPath $binary -PathType Leaf) {
    Remove-Item -LiteralPath $binary
}
```

自定义目录只删除 `miocli` / `miocli.exe`，不要删除共享目录。可选清除手动添加的 PATH 项，但先确认其他工具未使用它。

默认保留设置。若需清理，请先检查并备份 Linux/macOS 下 `$XDG_CONFIG_HOME`（否则 `~/.config`）、Windows 下 `%APPDATA%` 内的专用 `miocli` 目录，再手动删除。这会删除保存的密钥。使用 `--config` 时只检查并删除对应 miocli 文件，不删除共享父目录。

不要删除 Mihomo、其他客户端数据、导入源 YAML 或整个仓库。若启用过 miocli 系统代理，卸载前运行 `miocli system-proxy disable` 恢复保存的系统设置；miocli 不管理 TUN 或系统服务。

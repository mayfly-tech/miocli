# 发布

[English](../releasing.md) · 简体中文 · [文档](README.md)

本文描述已提交的工作流，不表示已验证某次 GitHub 运行或仓库权限。

## 自动化

[Release 工作流](https://github.com/mayfly-tech/miocli/blob/main/.github/workflows/release.yml) 在 `main` push 时运行。[release-please 配置](https://github.com/mayfly-tech/miocli/blob/main/release-please-config.json) 使用 Go 单包策略、带 `v` 且无组件前缀的标签，以及 bootstrap 提交。[Manifest](https://github.com/mayfly-tech/miocli/blob/main/.release-please-manifest.json) 跟踪发布版本。

1. Conventional Commits 驱动 release-please 发布 PR、版本 manifest 及自动生成的 `CHANGELOG.md`。`fix`、`feat`、破坏性变更驱动版本；只有文档/CI/chore 通常不会单独产生发布。
2. 维护者检查并合并发布 PR，不手工改写历史生成日志或额外创建发布标签。
3. release-please 创建标签和正式 GitHub Release。
4. 同次运行使用 `sha`、`tag_name`、`release_created` 输出串联复用 CI、构建、安装器和上传。

发布串行运行，不取消进行中的流程。正式 Release **先于**资产构建完成发布，全部必需资产上传前不能视为可安装。

## 发布前 CI

复用 [CI 工作流](https://github.com/mayfly-tech/miocli/blob/main/.github/workflows/ci.yml) 验证发布 SHA：

- Go 1.24.x 和 stable：格式、`go vet`、race 测试、依赖许可证。
- Linux/macOS/Windows 的 amd64、arm64 交叉构建。
- Linux/macOS/Windows 完整原生 Go 测试。
- Shell 安装行为、Windows PowerShell 与 PowerShell 安装行为及安装器语法。
- 一次性容器中当前及上一正式稳定版 Mihomo 集成测试。

这些已配置的检查不保证所有发行版、终端或历史控制器均可用。

## 包与资产

各目标使用 `CGO_ENABLED=0`、`-trimpath`，ldflags 注入版本、提交及 UTC 构建时间。开发构建不会自动使用发布版本号。

| 平台 | 归档名 |
| --- | --- |
| Linux | `miocli_X.Y.Z_linux_{amd64,arm64}.tar.gz` |
| macOS | `miocli_X.Y.Z_darwin_{amd64,arm64}.tar.gz` |
| Windows | `miocli_X.Y.Z_windows_{amd64,arm64}.zip` |

归档包含同名顶层目录、二进制、双语 README、贡献指南与安全政策、`docs/`、`LICENSE`、`THIRD_PARTY_LICENSES`。源码、工作流和开发 fixture 使用仓库链接，不作为源码一并打包。

按目标导出第三方许可证，不跳过 Windows 专属依赖。`go-localereader` 固定在包含上游许可证的版本，以便 Windows 许可证导出。

上传共 **14 份资产**：六个包、六个同名 `.sha256`、`install.sh` / `install.ps1`。安装器模板的 `__MIOCLI_REPOSITORY__` 被替换为工作流仓库。上传前检查所有预期文件非空并校验摘要，再运行 `gh release upload --clobber`。

移动文档需同步归档复制清单及仓库链接，确保解压后用户指南相对链接可用。SHA-256 是同源完整性校验，不是独立包签名。

## 权限与事件行为

使用默认 `GITHUB_TOKEN`。release-please 有 contents/issues/pull-requests 写权限，上传有 contents 写权限，其余作业只读。仓库设置需允许 GitHub Actions 创建 PR。

默认 Token 创建的 PR/标签/Release 不自动触发额外 Actions。因此发布 CI/构建/上传在同一工作流中完成，不依赖独立 tag 事件。人工合并发布 PR 触发新的 `main` push。

自动创建的发布 PR 不会因新 PR 事件自动获得 CI。如果分支保护要求检查，维护者需安排触发方式或有意配置合适 Token/GitHub App。当前工作流不添加这些凭据，也不绕过保护。

## 恢复

Release 创建后，preflight、构建或上传失败：

1. 查看失败作业和缺失资产。
2. 修正原因，适用时对同次运行使用 **Re-run failed jobs**，保留成功的 release-please 输出。
3. `--clobber` 可在同版本替换部分上传资产。
4. 宣布可安装前核对全部 14 份资产。

不要盲目重跑全部作业：已存在 Release 不一定再次输出 `release_created=true`。发布后失败可能导致 latest 无法安装，直到补齐资产。不能仅凭本地测试宣称发布成功。

## 验收

- 确认 CI 和完整 Release 流程在目标 SHA/tag 成功。
- 确认六份包/校验文件和两份已配置安装器存在。
- 校验摘要，检查二进制、许可证和双语文档内容。
- 在隔离目录测试安装/升级；Windows 替换前关闭进程。
- 用 `miocli version` 核对版本/提交。
- 检查仓库链接、解压归档相对链接和语言切换。
- 确认版本/网络/校验失败保留旧 CLI，区分安装失败与安装后 setup 失败。

本地检查不能验证 GitHub 权限、分支保护、发布 PR 创建或真实远端发布，需实际工作流成功。用户安装见[安装](installation.md)，本地测试见[开发](development.md)。

# 开发与测试

简体中文 · [English](development_EN.md) · [README](../README.md)

## 环境

- Go 1.24 或更新版本。
- 单元测试不需要运行中的 Mihomo；API 测试使用 `httptest.Server` 和 fake client。
- 集成测试会修改实例的节点、模式或配置，只能对专用的一次性实例运行。

## 本地配置

连接已有 Mihomo 实例时，在仓库根目录创建本地配置：

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
```

在 `config.local.yaml` 中填写 `server` 和 `secret`。`dev` 从仓库根目录读取该文件，只连接已有内核，不写用户配置、不下载或启动内核。Windows 使用 `Copy-Item` 复制模板，无需 `chmod`。不要将真实密钥写入模板或提交到版本库。

```sh
go run ./cmd/miocli dev
go run ./cmd/miocli doctor --config ./config.local.yaml
```

非空 `MIOCLI_SERVER` 和 `MIOCLI_SECRET` 可临时覆盖配置；空值不清除文件中的值。发布版不自动读取当前目录配置或 `.env`。

## 本地检查

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
go run ./cmd/miocli version
sh -n install.sh
sh tests/installers/shell.sh
python3 -B -m unittest discover -s tests -p '*_test.py'
```

## 提交信息与 Git hooks

在仓库根目录安装本地提交信息检查：

```sh
sh scripts/install-hooks.sh
```

提交信息须符合 Conventional Commits：`type(scope)!: subject`，scope 和破坏性变更标记 `!` 可选。支持的类型为 `feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore` 和 `revert`。例如：`feat(cli): add profile command`。Git 自动生成的合并、撤销、fixup 和 squash 提交信息不作校验。该 hook 只检查提交信息，不会运行格式化或测试；安装配置仅对当前克隆有效。

Windows 安装器测试：

```powershell
pwsh -File tests/installers/powershell.ps1
```

交叉构建示例：

```sh
GOOS=linux GOARCH=amd64 go build -o /tmp/miocli-linux-amd64 ./cmd/miocli
GOOS=darwin GOARCH=arm64 go build -o /tmp/miocli-darwin-arm64 ./cmd/miocli
GOOS=windows GOARCH=amd64 go build -o /tmp/miocli-windows-amd64.exe ./cmd/miocli
```

## Mihomo 集成测试

`TestMihomoIntegration` 需要隔离实例，包含 `MIOCLI-Selector` 组（成员为 `DIRECT` 和 `REJECT`）及单节点文件 provider `MIOCLI-Provider`。测试会切换节点和模式、更新 provider 并调用 `DIRECT` 延迟 API；不要指向日常使用实例。

延迟测试使用本地 HTTP 端点，响应前等待 10ms，避免回环请求不足 1ms 时被 Mihomo 记为零延迟并返回 HTTP 503；不依赖外网测速地址。

```sh
MIOCLI_INTEGRATION_URL=http://127.0.0.1:19090 \
MIOCLI_INTEGRATION_SECRET=miocli-ci-secret \
go test -count=1 -run '^TestMihomoIntegration$' ./internal/mihomo
```

`TestIsolated` 会启动并清理自己的子进程。先取得并校验 Mihomo 二进制，再传入其绝对路径：

```sh
MIOCLI_TEST_BINARY=/absolute/path/to/mihomo \
go test -count=1 -run '^TestIsolated' ./internal/mihomo ./internal/profile
```

这些测试不读取用户配置、不连接日常实例。测试档案索引、事务日志和配置可能含敏感信息；样例、截图和日志须使用非敏感数据。

`TestIsolatedPinnedBootstrap` 使用本机平台的固定版本压缩包和本地 HTTPS 测试服务器，从无内核目录下载、校验并启动真实子进程后清理：

```sh
MIOCLI_TEST_ARCHIVE=/absolute/path/to/pinned-mihomo.gz \
go test -race -count=1 -run '^TestIsolatedPinnedBootstrap$' ./internal/core
```

## 发布流程

`release.yml` 在 `main` 收到提交时运行 `googleapis/release-please-action`。`release-please-config.json` 使用 Go 单包策略，`.release-please-manifest.json` 记录已发布版本，以 `v0.1.1` 为初始基线；`bootstrap-sha` 限制首次变更日志从该版本之后开始收集。版本通过构建时的 Go ldflags 注入，本地开发显示 `dev`。

1. 使用 Conventional Commits 提交变更：`fix` 对应补丁版本，`feat` 对应次版本，`!` 或 `BREAKING CHANGE` 对应主版本。仅 `ci`、`chore` 或文档变更默认不会单独触发发布 PR。
2. release-please 自动创建或更新发布 PR，修改版本 manifest 和 `CHANGELOG.md`。检查变更日志与版本后合并，不需要手工打标签。
3. 合并发布 PR 后，release-please 创建 `vX.Y.Z` 标签和正式 GitHub Release，并生成发布说明。
4. 同一工作流使用 release-please 返回的发布 SHA 运行完整 CI，再构建 Linux、macOS、Windows 的 amd64/arm64 包和两份安装脚本。校验 SHA-256 后，将全部 14 份资产上传到对应 Release。

正式 Release 先于资产创建，需等待 Release 工作流全部成功才能安装；期间 latest 安装可能因缺少资产而失败，但不会替换已有 CLI。发布工作流串行执行，不取消进行中的发布。版本包命名保持 `miocli_X.Y.Z_OS_ARCH.tar.gz`（Windows 为 `.zip`），每份包附带同名 `.sha256` 文件。

发布包按目标平台导出第三方许可证，不跳过 Windows 专用依赖。`go-localereader` 固定在 `v0.0.2-0.20220822084737-6bae6c923850`：该提交相对 `v0.0.1` 仅补充上游 LICENSE，无代码变更，避免 Windows 打包时因缺少许可证失败。

### 权限与检查

工作流使用默认 `GITHUB_TOKEN`，无需额外发布凭据。release-please job 具有 `contents`、`issues`、`pull-requests` 写权限，上传 job 仅具有 `contents` 写权限，其余 job 只读。仓库 `Settings → Actions → General` 需允许 GitHub Actions 创建 PR。

默认 Token 创建的发布 PR、标签和 Release 不会自动触发其他 Actions。发布 CI、构建和上传因此直接使用同一工作流内的 `release_created`、`tag_name`、`sha` 输出串联，不依赖标签或 Release 事件。人工合并发布 PR 仍会触发 `main` push 的发布工作流。

发布 PR 本身不会自动运行 CI；若分支保护要求 PR 检查成功，维护者需安排检查触发方式或改用能触发检查的专用 Token / GitHub App。当前工作流不引入这些凭据，也不绕过分支保护。

### 失败恢复与验收

CI、构建或上传失败时，在同一次工作流运行中选择“重新运行失败的作业”，保留成功的 release-please job 输出。上传使用 `gh release upload --clobber`，可重试同版本的部分上传。不选择完整重跑：已创建的 Release 不会再次得到 `release_created=true`。正式 Release 创建后失败会暂缺部分或全部资产，修复失败前不能视为发布成功。

验收使用隔离安装目录，从 GitHub Release 下载脚本，确认六份包、六份 SHA-256 和两份脚本齐全，安装后检查版本与提交信息。GitHub 版本查询限制 15 秒，包和校验文件各限制 30 秒；Shell 另有连接限制，PowerShell 7.4+ 分别设置连接和操作超时，Windows PowerShell 使用 `TimeoutSec`。请求、版本或校验失败均终止安装且保留已有 CLI。

本地工作流和安装器测试不创建真实发布 PR、标签或 Release。端到端验收仍需在 GitHub 上成功运行发布流程；本地测试通过不代表仓库权限、分支保护或远端发布已验证。

### Mihomo 版本维护

升级 Mihomo 时修改 `internal/core/mihomo-release.json`：确认官方稳定版本、全部六个平台文件名、原始压缩包 SHA-256 和许可证；优先可移植 amd64 baseline。更新中英文 README 的固定版本说明，执行映射/下载测试和隔离实例兼容测试。不要仅修改版本字符串或在客户端查询 latest。

Mihomo 是独立的 GPL-3.0 程序，不由本工作流镜像或上传。客户端仅从官方 GitHub Release 下载并校验固定版本；GitHub 不可达时需在 setup 中指定已有本地内核。

## 卸载与清理

发布版的卸载、可选数据清理及自定义路径注意事项见 [README：卸载](../README.md#卸载)。本地 `go run ./cmd/miocli dev` 不安装程序，也不写用户配置；开发结束后只需退出进程。自行构建的二进制可从实际输出位置删除。仓库内的 `config.local.yaml` 可能含密钥，按需要备份或单独清理；不要为卸载而删除整个工作区或其他客户端的配置。

## 代码边界

- API DTO 与领域类型属于 `internal/mihomo`；UI 不直接构造 HTTP 请求。
- 网络操作由 Bubble Tea `tea.Cmd` 启动，结果通过有类型的消息更新 Model。
- 档案存储与热加载属于 `internal/profile`；仅本次实际启动的托管子进程拥有完整档案写权限。
- 配置优先级、安全策略或 API 映射变化时，同步更新测试、README 与[架构说明](architecture.md)。

贡献流程见[贡献指南](../CONTRIBUTING.md)。

# 开发与测试

[English](../development.md) · 简体中文 · [文档](README.md)

## 环境

- Go 1.24 或更新版本，见仓库 [go.mod](https://github.com/mayfly-tech/miocli/blob/main/go.mod)。
- 单元测试使用 fake client 和本地 HTTP 服务器，不需要运行中的 Mihomo。
- 只有集成测试需要一次性控制器，会改变节点/模式/provider 状态。

以下示例在源码仓库执行。发布归档包含使用文档，不包含这里链接的 Go 源码、脚本或 fixture。

## 本地连接设置

```sh
cp config.local.example.yaml config.local.yaml
chmod 600 config.local.yaml
```

Windows：

```powershell
Copy-Item config.local.example.yaml config.local.yaml
```

填写已有控制器的 `server` / `secret`。本地文件被 Git 忽略；不要将真实密钥写入跟踪的模板。

```sh
go run ./cmd/miocli dev
go run ./cmd/miocli doctor --config ./config.local.yaml
```

`dev` 从当前目录定位仓库根，只连接，不写用户设置、不下载或启动内核。非空 `MIOCLI_SERVER` / `MIOCLI_SECRET` 仍覆盖本地值。仓库外可用 `dev --config PATH` 指定已存在文件。普通启动不隐式加载该文件或 `.env`。

## 检查

先运行覆盖改动的测试，再按需要扩展：

```sh
go test ./internal/config ./internal/cli
go test ./internal/app
go test ./internal/mihomo
```

完整 Go 检查（POSIX shell）：

```sh
gofmt -w $(find cmd internal -type f -name '*.go')
go vet ./...
go test -race ./...
go build -o /tmp/miocli ./cmd/miocli
go run ./cmd/miocli version
```

Windows 请格式化改动文件并选择合适构建路径。未在构建时注入信息的开发版本为 `dev`，提交/构建时间为 unknown。

在仓库中检查安装器：

```sh
sh -n install.sh
sh tests/installers/shell.sh
```

```powershell
pwsh -File tests/installers/powershell.ps1
# 检查 Windows PowerShell 兼容性：
powershell -File tests/installers/powershell.ps1
```

安装器测试使用隔离 fixture，不要对真实已安装二进制测试替换。

## 集成测试

专用实例使用仓库 [Mihomo fixture](https://github.com/mayfly-tech/miocli/blob/main/.github/fixtures/mihomo.yaml) 和 [provider fixture](https://github.com/mayfly-tech/miocli/blob/main/.github/fixtures/provider.yaml)。需要 `MIOCLI-Selector` 组，含 `DIRECT`/`REJECT`，以及单节点文件 provider `MIOCLI-Provider`。

```sh
MIOCLI_INTEGRATION_URL=http://127.0.0.1:19090 \
MIOCLI_INTEGRATION_SECRET=miocli-ci-secret \
go test -count=1 -run '^TestMihomoIntegration$' ./internal/mihomo
```

上述密钥属于公开测试 fixture，不是用户实例。**绝不能指向日常控制器。** 测试切换成员与模式、更新 provider、调用延迟 API。测试使用等待 10ms 再响应的本地 HTTP 服务器，避免 Mihomo 将不足 1ms 结果记为零延迟/HTTP 503，不依赖生产使用的 gstatic URL。

未设置 `MIOCLI_INTEGRATION_URL` 时跳过集成测试。CI 在测试专用容器独立运行当前/上一稳定 Mihomo；fixture 挂载及就绪检查见 [CI 工作流](https://github.com/mayfly-tech/miocli/blob/main/.github/workflows/ci.yml)。

## 提交信息与 hooks

可选安装当前克隆的 commit-message hook：

```sh
sh scripts/install-hooks.sh
```

格式为 `type(scope)!: subject`，scope 和 `!` 可选。类型包括 `feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`revert`。Git 自动生成的 merge/revert/fixup/squash 信息豁免。

Hook 只检查信息，不运行格式化/测试，安装只影响当前克隆。提交信息示例应描述已实现的行为。

## 实现与文档边界

HTTP 请求/DTO 放 `internal/mihomo`，设置/导入放 `internal/config`，异步 UI 工作使用 Bubble Tea 命令，渲染状态放 `internal/app`。见[架构](architecture.md)。

默认值、优先级、按键或 API 行为变化需测试及对应中英文文档。文档改动检查链接/锚点，不为验证 Markdown 而运行集成测试。

清理时退出 `go run`，仅删除确定不需保留的自建二进制或本地设置，不删除仓库或其他客户端数据。安装版卸载见[安装](installation.md#卸载)。

下一步：[贡献](../../CONTRIBUTING.zh-CN.md) · [发布](releasing.md)

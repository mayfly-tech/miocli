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
```

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

## 代码边界

- API DTO 与领域类型属于 `internal/mihomo`；UI 不直接构造 HTTP 请求。
- 网络操作由 Bubble Tea `tea.Cmd` 启动，结果通过有类型的消息更新 Model。
- 档案存储与热加载属于 `internal/profile`；仅本次实际启动的托管子进程拥有完整档案写权限。
- 配置优先级、安全策略或 API 映射变化时，同步更新测试、README 与[架构说明](architecture.md)。

贡献流程见[贡献指南](../CONTRIBUTING.md)。

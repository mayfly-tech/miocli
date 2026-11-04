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

`TestIsolatedS3Bootstrap` 使用本机平台的固定版本压缩包和本地 HTTPS 镜像，模拟 GitHub 不可达，从无内核目录下载、校验并启动真实子进程后清理：

```sh
MIOCLI_TEST_ARCHIVE=/absolute/path/to/pinned-mihomo.gz \
go test -race -count=1 -run '^TestIsolatedS3Bootstrap$' ./internal/core
```

## S3 镜像发布

发布包按目标平台导出第三方许可证，不跳过 Windows 专用依赖。`go-localereader` 固定在 `v0.0.2-0.20220822084737-6bae6c923850`：该提交相对 `v0.0.1` 仅补充上游 LICENSE，无代码变更，避免 Windows 打包时因缺少许可证失败。

标签构建仍创建 GitHub 草稿 Release。正式发布稳定版本后，`publish-s3.yml` 在受保护的 `s3-release` 环境同步已经生成的 Release 文件，不重新构建二进制。也可通过 workflow_dispatch 指定已正式发布的稳定标签重试；旧版本不会降级稳定索引。

环境配置统一使用 GitHub Actions Secrets，在 `Settings → Environments → s3-release → Environment secrets` 中设置以下项目；工作流不读取同名 Variables。`S3_KEY_PREFIX` 和 `S3_REGION` 可不设置，其余项目必填。不要将凭据写入源码或客户端。

公开文档只记录配置项名称和通用发布约定，不记录实际存储 API 地址、桶名、账号标识、区域部署、凭据值或内部网络信息。实际配置通过 GitHub 发布环境管理；Issue、PR、日志和截图也需脱敏。下文使用 `example.com` 示例域名，不代表可用下载源。

| 类型 | 名称 | 用途 |
| --- | --- | --- |
| Secret | `S3_ENDPOINT` | S3 兼容 API 的 HTTPS endpoint，不是公开下载 URL |
| Secret | `S3_BUCKET` | 目标桶 |
| Secret | `S3_KEY_PREFIX` | 可选对象键前缀，与公开根路径对应 |
| Secret | `S3_REGION` | 可选区域，默认 `us-east-1` |
| Secret | `S3_ACCESS_KEY_ID` | 仅 CI 上传使用 |
| Secret | `S3_SECRET_ACCESS_KEY` | 仅 CI 上传使用 |

将桶/前缀配置为公开只读下载，使对象 `miocli/...` 和 `mihomo/...` 映射到项目公开下载根地址（示例：`https://downloads.example.com/public/`）。实际公开根地址需与安装器、内核下载及发布脚本中的地址一致，不能用内部存储 API 地址替代。CI 的 GetObject/PutObject 权限限制到分发前缀，不需要删除权限；同时提供桶级 ListBucket 权限，使不存在对象的 Head 请求返回 404 而不是 403（建议使用独立发布桶）。API 需支持条件 Put（`If-None-Match` / `If-Match`）；不支持时发布明确失败，不退化为无条件覆盖。保护发布环境并限制可发布的标签。

布局：

```text
miocli/install.sh                    # 稳定脚本入口
miocli/install.ps1
miocli/stable.txt                    # 单行稳定版本，最后更新
miocli/releases/vX.Y.Z/              # 两份脚本、六个平台包及 SHA-256
mihomo/releases/vA.B.C/              # 六个平台原始包、LICENSE 和 SOURCE.txt
```

先验证全部本地包和官方 Mihomo 摘要，再上传不可变对象，通过公开 URL 校验后才提升稳定入口。版本目录要求字节不可变，重跑只复用一致对象；冲突、403、上传或公开校验失败均报错。稳定索引使用条件更新，工作流串行执行。脚本别名保持版本无关，避免多对象更新非事务性造成不兼容。公开域名应尊重 Cache-Control，稳定入口不得长期缓存。

提升索引后的最终公开校验失败时，索引可能已更新；流程报错而不宣称成功。修复公开访问或缓存后重跑，同版本重跑也会验证稳定入口。

升级 Mihomo 时修改 `internal/core/mihomo-release.json`：确认官方稳定版本、全部六个平台文件名、原始压缩包 SHA-256 和许可证；优先可移植 amd64 baseline。更新中英文 README 的固定版本说明，执行映射/下载测试和隔离实例兼容测试。不要仅修改版本字符串或在客户端查询 latest。

Mihomo 是独立的 GPL-3.0 程序，镜像保留原始包，附带上游许可证和 `SOURCE.txt` 对应版本源码下载指引；维护者需保持所指源码可获取。该指引不意味着源码或配置资源也已镜像到 S3。

GitHub 版本查询限制 15 秒，包和校验文件各限制 30 秒；S3 元数据/校验文件限制 60 秒、包限制 180 秒。POSIX/Go 另有连接限制，PowerShell 7.4+ 分别设置连接和操作超时；旧版 PowerShell 使用现有 `TimeoutSec` 总请求限制，不要求安装新版 PowerShell。

验收须使用隔离用户目录、本地不依赖外部规则/Geo/providers 的最小配置：GitHub 不可达且没有内核时，从 S3 获得脚本，安装 miocli，再在 setup 中确认 S3 内核兜底，检查版本和 SHA-256，最后清理自有子进程。首次真实验收需要配置 CI 存储凭据并正式发布；测试通过不等于 S3 已部署。

## 卸载与清理

发布版的卸载、可选数据清理及自定义路径注意事项见 [README：卸载](../README.md#卸载)。本地 `go run ./cmd/miocli dev` 不安装程序，也不写用户配置；开发结束后只需退出进程。自行构建的二进制可从实际输出位置删除。仓库内的 `config.local.yaml` 可能含密钥，按需要备份或单独清理；不要为卸载而删除整个工作区或其他客户端的配置。

## 代码边界

- API DTO 与领域类型属于 `internal/mihomo`；UI 不直接构造 HTTP 请求。
- 网络操作由 Bubble Tea `tea.Cmd` 启动，结果通过有类型的消息更新 Model。
- 档案存储与热加载属于 `internal/profile`；仅本次实际启动的托管子进程拥有完整档案写权限。
- 配置优先级、安全策略或 API 映射变化时，同步更新测试、README 与[架构说明](architecture.md)。

贡献流程见[贡献指南](../CONTRIBUTING.md)。

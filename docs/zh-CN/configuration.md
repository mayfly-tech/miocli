# 配置

[English](../configuration.md) · 简体中文 · [文档](README.md)

miocli 设置描述与已有控制器的连接，不是 Mihomo 运行配置。

## 文件位置与字段

| 平台 | 默认文件 |
| --- | --- |
| Linux / macOS | `$XDG_CONFIG_HOME/miocli/config.yaml`；否则 `~/.config/miocli/config.yaml` |
| Windows | `%APPDATA%\miocli\config.yaml` |

设置 `XDG_CONFIG_HOME` 时必须使用绝对路径。Windows 默认路径需要 `APPDATA`。普通启动不读取当前目录配置或 `.env`。

```yaml
server: http://127.0.0.1:9090
secret: ""
```

| 字段 | 含义 | 默认值 |
| --- | --- | --- |
| `server` | 控制器 URL，不是代理监听地址 | `http://127.0.0.1:9090` |
| `secret` | Bearer 认证密钥 | 空，不发送 Authorization 头 |
| `active-profile-path` | 可选的绝对路径，保存激活档案供内核下次启动使用 | 不设置 |

可选的 `active-profile-path` 绝对路径用于将成功激活的 profile 保存为内核下次启动配置，见[订阅档案](#订阅档案)。未知字段及多份 YAML 文档会被拒绝。文件中省略或留空 `server` 时保留默认值。

`--config PATH` 选择**单一**文件，不合并。默认文件缺失允许使用默认值；显式指定的文件缺失则报错，`setup` 也一样。自定义路径运行 setup 前，先创建合法的最小配置，或使用默认路径。

## 优先级

| 设置 | 从高到低 |
| --- | --- |
| 地址 | `--server` → 非空 `MIOCLI_SERVER` → YAML → 默认值 |
| 密钥 | 非空 `MIOCLI_SECRET` → YAML → 无认证 |

空环境值不会清除文件设置。`--server` 只影响本次调用，不保存地址。没有密钥命令行参数，请勿将密钥放进参数或 URL。

文件解析和权限检查在环境/参数覆盖**之前**完成。覆盖不能绕过非法 YAML、未知字段、显式文件缺失，或文件含密钥时的权限错误。

## 交互配置

```sh
miocli setup
```

Setup 需要交互 stdin/stdout。选择手动输入或指定 Mihomo YAML，检查 URL、输入隐藏密钥、通过只读 API 检查后，再确认保存。

- 保存 miocli 的 `server` / `secret`，保留已有的 `active-profile-path`，不改拥有内核的客户端。
- 不会默默保存环境覆盖值：setup 使用输入/导入值。环境覆盖仍影响后续普通调用。
- 取消、检查失败或拒绝确认都不会保存。
- 密钥存于受保护 YAML，**不是系统钥匙串**。Setup 写入私有文件（Unix `0600`；Windows 当前用户专用受保护 ACL）。
- Unix 加载含非空密钥的文件时拒绝组/其他用户权限位；请对该文件执行 `chmod 600`。Windows 保存时保护 ACL，不代表加载时会检查所有外部创建文件的 ACL。

### 导入连接字段

从指定普通 YAML 文件只读提取顶层 `external-controller`、`external-controller-unix` 和 `secret`，上限 16 MiB。必须是单个映射/文档，不修改源文件。源路径开头支持 `~`。

Linux/macOS 上，非空 `external-controller-unix` 优先，须为绝对且规范化的 socket 路径，会转换为 `unix:///absolute/control.sock`。否则 `external-controller` 必须是可作为回环目标使用的 `host:port` HTTP 监听，例如 `127.0.0.1:9090`。URL、`0.0.0.0:9090` 等通配监听、named pipe 和仅 TLS 控制器字段不会自动转换为可用连接。请改为手动输入实际回环或 HTTPS URL。

Setup 的连接信息导入不扫描客户端目录、不发现密钥，也不导入代理、规则、订阅或完整配置档案。

### 订阅档案

`miocli profile add NAME HTTPS_URL` 将 HTTPS 返回的单文档 YAML 下载到当前设置文件旁的 `profiles/` 目录，并以私有权限保存档案 URL 元数据与配置文件。名称为 1–64 个字母、数字、点、下划线或连字符，须以字母或数字开头；`profiles` 为保留名称（不区分大小写）。仅支持 Mihomo YAML 配置，不解析 Base64 节点列表；下载不跟随重定向，大小上限 16 MiB。订阅 URL 可能包含令牌，因此不会在列表或错误中回显，分享 profile 注册表前仍应检查并脱敏。

`miocli profile use NAME` 在已保存档案旁生成私有的生效 YAML，再调用 Mihomo `PUT /configs?force=true` 重载。每次激活都将订阅中的控制监听和 `secret` 替换为当前 miocli 连接：Unix 仅保留该 socket，HTTP 仅保留该回环监听，HTTPS 仅保留该 TLS 监听（profile 仍须提供有效的 TLS 证书设置），其他类型的控制监听被关闭。订阅原文保持不变，`profile update` 后也一样；订阅无需预带控制器字段。节点、规则、TUN、DNS 等其他配置值保留不变。该文件必须能被 Mihomo 读取，因此激活要求本机控制器；远端控制器无法读取 CLI 所在机器的文件。SSH 转发的回环 URL 仍指向远端内核，不应用于档案激活。请求成功只代表 Mihomo 接受重载，仍应检查运行状态。

临时文件只在重载成功后删除。重载报错时保留该文件并在错误中打印路径，因为核心可能仍在处理；运行态不确定且重启配置不会保存。重试前检查核心日志、控制器和网络状态，待处理结束后手动删除保留文件。

对由外部服务管理的独立实例，可显式设置：

```yaml
server: unix:///etc/mayfly-proxy/control.sock
secret: ""
active-profile-path: /etc/mayfly-proxy/config.yaml
```

设置 `active-profile-path` 后，`profile use` 在 API 成功后以私有权限将同一份生效 YAML 原子保存到该路径，供服务下次启动使用；不会覆盖所选 miocli 设置文件或原始订阅档案。`profile update` 仅下载，需再次执行 `profile use`，应用更新内容时仍会保护控制器连接。不提供 TUN/DNS/路由覆写、核心校验、服务重启或自动回滚。若重载后保存失败，会明确提示运行态已变但持久化失败。切换前备份重启配置，通过 `doctor` 和网络验收检查，必要时经所属服务恢复备份。由其他客户端管理的配置应保持此字段未设置。

只适配管理连接字段，不添加网络保护策略；下载内容仍须是预期运行的完整网络配置，不会将 Base64 列表或仅含节点的订阅转换为完整配置。profile URL 参数可能出现在命令历史和进程列表中，需保护含 token 的地址。

## 本地 Unix 控制器

Linux/macOS 可设置 `server: unix:///absolute/control.sock`，或通过 `--server` 指定。socket 路径必须绝对、规范化，不允许主机、查询参数或片段；不支持 Windows named pipe。进程必须已有访问 socket 的权限，miocli 不改所有权或权限。Unix controller 对 profile 激活和 system-proxy 命令视为本机，但 system-proxy 仍要求实际存在本地代理监听端口。

HTTP 请求直接通过 socket 传输，保留认证、响应上限、超时和拒绝重定向约束。不创建 TCP 监听或转发服务。

## 启动与开发设置

没有默认配置文件时，普通启动尝试默认 URL（或覆盖值）。探测失败后，只有配置文件不存在、没有显式地址覆盖、不是 `dev` 且 stdin/stdout 为终端时才提供 setup。其他失败返回错误，不自动启动内核。

`dev` 向上查找 `miocli` module 及模板，最多 32 层，定位仓库本地 `config.local.yaml`。显式 `dev --config PATH` 跳过仓库查找；其余文件与环境优先级不变。见[开发指南](development.md)。

## 远程控制器

URL 使用 `http` 或 `https`，含主机及可选的 1–65535 端口。允许尾部斜线；不允许其他路径、嵌入凭据、查询参数或片段。

- HTTP 仅允许 `localhost` 或回环 IP，包括 IPv6。
- 远程需 HTTPS，验证系统信任和主机名；不提供跳过 TLS 验证的选项。
- 控制 API 客户端忽略环境代理，不跟随重定向，与安装器下载行为不同。
- 最低 TLS 1.2；普通请求超时 5 秒，profile 重载单独使用 120 秒以等待 provider/Geo 同步初始化，响应上限 1 MiB。

若远端控制器监听该机器的回环接口，在另一终端打开隧道：

```sh
# 替换 user@host；本地端口 19090 必须空闲。
ssh -N -L 127.0.0.1:19090:127.0.0.1:9090 user@host
```

保持隧道运行，再连接，不改保存的设置：

```sh
miocli doctor --server http://127.0.0.1:19090
miocli --server http://127.0.0.1:19090
```

通过受保护配置或 `MIOCLI_SECRET` 提供控制器密钥，不写在 SSH 命令中。不要向公网暴露无认证控制器。

## 安全诊断

`miocli doctor` 打印所选文件及设置来源、隐藏密钥，并检查 `/version`、`/proxies`、`/configs`。要求 Mihomo 身份（`meta: true`）和非空模式；不写设置，也不检查全部可选 API。分享诊断前检查路径、控制器地址和资源信息。

下一步：[命令行参考](cli.md) · [排障](troubleshooting.md)

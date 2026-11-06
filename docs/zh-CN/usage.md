# 使用操作台

[English](../usage.md) · 简体中文 · [文档](README.md)

[配置控制器](configuration.md)后运行 `miocli`。界面为英文，默认打开 Groups，需要至少 50 列 × 12 行。更小窗口显示调整尺寸提示，恢复尺寸前仅能退出。

## 导航与焦点

| 按键 | 非文本输入时的操作 |
| --- | --- |
| `1` / `2` / `3` / `4` | Groups / Nodes / Providers / Status |
| `↑` / `↓`、`j` / `k` | 移动当前列表或滚动详情 |
| `PgUp` / `PgDn` | 列表或详情翻页 |
| `Home` / `End`、`g` / `G` | 首末条目或详情首尾 |
| `/` | 在 Groups、Nodes、Providers 搜索名称 |
| `:` | 打开命令栏 |
| `m` | 按 Rule → Global → Direct → Rule 循环模式 |
| `r` | 没有冲突操作时请求刷新/重连 |
| `?` | 打开帮助；`?` / `Esc` 关闭 |
| `q` / `Ctrl+C` | 退出；`Ctrl+C` 在文本输入时同样有效 |

搜索期间按键属于搜索框，`q` 是文本而非退出。`Enter` 保留名称筛选，`Esc` 清除并结束搜索输入。帮助、详情也拥有键盘焦点，并非所有导航/操作键全局生效。根层级的 `Esc` 不退出。

切换视图清除搜索、协议/来源筛选及详情状态。`Enter`、`Tab`、`Space`、`Esc` 的上下文行为见下表。

## Groups：选择与浏览

1. 使用 `/` 或列表导航查找分组。
2. 按 `Enter`、`Tab` 或 `→` 聚焦成员。
3. 高亮成员，按 `Enter` 在 `Selector` 中选择。
4. 检查刷新后的选择结果。自动组不允许手动变更。

| 按键 | Groups 行为 |
| --- | --- |
| `Tab` / `Shift+Tab` | 切换组/成员焦点；返回组列表会退出嵌套导航 |
| `Enter` | 组列表中聚焦成员；成员列表中选择高亮成员 |
| `→` / `l` | 聚焦成员，或进入高亮的嵌套组 |
| `←` / `h` | 离开嵌套组，否则聚焦组列表 |
| `v` | 切换组/成员详情 |
| `Esc` | 按上下文关闭详情、清除保留搜索、离开嵌套组或返回焦点 |
| `Space` | 测试当前组的直接非分组成员 |

**选择嵌套组与进入嵌套组是两种操作。** `Enter` 可以把该组设为父组的选中成员；`→` / `l` 进入浏览，但不改变父组选择。不允许进入循环路径。

Groups 搜索针对当前聚焦的组/成员列表。进入成员或切换焦点会清除搜索，不将组名称筛选带到成员。组测速覆盖直接成员，不递归嵌套组，也不只测试可见搜索结果。

## Nodes：查看、筛选与测试

Nodes 是资源列表，不是选择目标。高亮节点或打开详情不会切换任何分组；需要到 Groups 的 `Selector` 中选择。

| 按键 | Nodes 行为 |
| --- | --- |
| `Enter` / `→` / `l` / `Tab` / `Shift+Tab` | 切换详情 |
| `Esc` / `←` / `h` | 关闭详情，否则清除搜索和协议/来源筛选 |
| `s` | 在默认、名称、延迟顺序间循环 |
| `t` / `f` | 循环协议/来源筛选 |
| `Space` | 测试高亮节点 |
| `T` | 测试符合当前搜索和筛选的全部节点 |
| `c` | 取消延迟测试，保留已完成结果 |

来源为 `Config`、`Built-in` 或 provider 名称。不同 provider 中不在全局列表的同名节点仍是独立资源。默认顺序由组成员、其余全局节点、provider 节点合成，不是原样 API 数组顺序。

延迟排序使用操作台当前测试数据；缺失/失败结果排在成功结果之后，不从节点名称推测延迟。

### 延迟测试测量什么

Mihomo 对 `https://www.gstatic.com/generate_204` 执行 HTTP 延迟检查。它不是带宽测速、ICMP ping，也不是 CLI 到控制器的往返时间。provider 独有节点使用 provider 节点 healthcheck 端点。

当前测试超时参数为 5000ms，单节点请求上下文为 5 秒，最多 10 个 worker。这些是实现常量，不是 YAML 选项。`c` 取消时保留已完成结果；新测试或写操作可能清除旧的操作台测速数据。

`T` 仅用于 Nodes，即使紧凑 Groups 页脚提示看起来也支持它。

## Providers：查看与更新

使用 `/` 搜索 provider 名称。`Enter`、`→` / `l`、`Tab`、`Shift+Tab` 切换详情；`Esc` / `←` / `h` 关闭详情或清除搜索。详情展示类型、来源、更新时间、成员，以及 Mihomo 返回的订阅用量/到期信息。

按 `u` 请求更新高亮 provider。miocli 读回 providers 并刷新代理数据。miocli 发起手动更新，真正的来源获取由 Mihomo 完成；Mihomo 自身也可能有更新计划。

Provider 是 API 资源，**不是**完整 YAML 档案或配置订阅。缺失的用量/到期信息表示未知，不是零或无限。

## Status：理解展示内容

Status 展示控制器/版本/模式、配置路径、资源计数、最近成功代理刷新时间和当前诊断，可用导航键滚动。

- **Cursor：** 当前浏览高亮资源。
- **Selected：** 所跟踪分组路径的选中成员链；仅浏览不会改变它。
- **Connections：** 来自 `/connections` 的活跃链路，反转为外层组到节点并去重。不是由当前模式推测的路由，也不是光标路径。

没有活跃连接是合法结果；读取失败不等于“没有连接”。Status 不提供实时流量图、完整请求元数据、日志流或连接关闭。

## 配置档案与系统代理

使用 `miocli profile add NAME HTTPS_URL` 下载单文档 Mihomo YAML，再用 `profile update NAME` 更新、`profile list` 查看或 `profile remove NAME` 删除。切换前确认该档案是完整且适用于当前内核的配置；`profile use NAME` 会请求本机控制器重载，之后检查控制器状态。订阅 URL 不会在列表中显示。

`miocli system-proxy status` 查看平台代理状态；`enable` 将 HTTP/HTTPS/SOCKS 代理指向本机 Mihomo 可用端口，`disable` 恢复 miocli 启用前保存的值。启用已有代理、PAC 或自动代理配置时会拒绝覆盖。仅支持 macOS `networksetup`、Linux GNOME `gsettings` 与 Windows 当前用户 Internet Settings；macOS 必须提供 `--service NAME`，若系统要求管理员授权，miocli 会返回错误但不会自行提权，其他 Linux 桌面环境不受支持。

## 命令栏

按 `:`，输入命令再按 `Enter`；`Esc` 取消输入。

| 命令 | 效果 |
| --- | --- |
| `groups`、`nodes`、`providers`、`status` | 切换视图 |
| `help` | 打开帮助 |
| `refresh` | 与 `r` 相同的刷新请求 |
| `mode rule`、`mode global`、`mode direct` | 请求指定模式 |
| `quit`、`q` | 退出 |

命令不区分大小写，不是 shell 命令。不支持的名称/参数会明确报错。

## 刷新、写操作与错误

正常轮询每 5 秒调度，读取 proxies/mode、providers 和 connections。代理读取中的网络/超时/服务端故障按 1、2、4 秒有限重试后暂停；认证及其他非临时代理错误直接暂停，不走这些重试。按 `r` 重试。可选资源错误单独展示，不采用完全相同的重试规则。

选择、模式和 provider 写操作后均读取状态；超时写请求不会自动重放。响应失败可能导致结果不确定，应先检查刷新结果再手动重试。分组的选中成员或一条通知，不保证全部活跃连接已经改走该路由。

下一步：[排障](troubleshooting.md) · [API 细节](architecture.md#mihomo-api)

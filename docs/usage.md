# Using the console

English · [简体中文](zh-CN/usage.md) · [Documentation](README.md)

Run `miocli` after [configuring the controller](configuration.md). The UI is English, opens in Groups, and requires at least 50 columns × 12 rows. Smaller windows show a resize message; only quitting is available until resized.

## Navigation and focus

| Key | Action when not entering text |
| --- | --- |
| `1` / `2` / `3` / `4` | Groups / Nodes / Providers / Status |
| `↑` / `↓`, `j` / `k` | Move the focused list or scroll details |
| `PgUp` / `PgDn` | Page through a list or details |
| `Home` / `End`, `g` / `G` | First/last entry or start/end of details |
| `/` | Search names in Groups, Nodes or Providers |
| `:` | Open the command bar |
| `m` | Cycle Rule → Global → Direct → Rule |
| `r` | Request refresh/reconnection when no conflicting operation is running |
| `?` | Open help; `?` or `Esc` closes it |
| `q` / `Ctrl+C` | Quit; `Ctrl+C` also works while entering text |

While searching, typed keys belong to the search field: `q` is text, not quit. `Enter` retains the name filter; `Esc` clears it and leaves search entry. Help and details also own keyboard focus, so not every navigation/action key is global. At the root, `Esc` does not quit.

Changing views clears search, protocol/source filters and detail state. Read the tables below for context-specific `Enter`, `Tab`, `Space` and `Esc`.

## Groups: select and explore

1. Find a group with `/` or list navigation.
2. Press `Enter`, `Tab` or `→` to focus its members.
3. Highlight a member and press `Enter` to select it in a `Selector`.
4. Inspect the refreshed selection. Automatic groups cannot be changed manually.

| Key | Groups behavior |
| --- | --- |
| `Tab` / `Shift+Tab` | Toggle group/member focus; returning to groups leaves nested navigation |
| `Enter` | Focus members from the group list; select the highlighted member from the member list |
| `→` / `l` | Focus members, or enter the highlighted nested group |
| `←` / `h` | Leave a nested group, otherwise focus the group list |
| `v` | Toggle group/member details |
| `Esc` | Close details, clear retained search, leave a nested group or return focus, depending on context |
| `Space` | Test direct non-group members of the current group |

**Selecting a nested group and entering it are different operations.** `Enter` can make that group the selected member of its parent; `→` / `l` explores it without selecting it in the parent. Cycles cannot be entered.

Groups search applies to the focused group/member list. Entering members or changing focus clears the search rather than applying a group-name filter to members. Group latency testing covers direct members, not recursively nested groups or only the visible search matches.

## Nodes: inspect, filter and test

Nodes is an inventory, not a selection destination. Highlighting a node or opening details does not change any group; switch to Groups and choose a `Selector` to select it.

| Key | Nodes behavior |
| --- | --- |
| `Enter` / `→` / `l` / `Tab` / `Shift+Tab` | Toggle details |
| `Esc` / `←` / `h` | Close details; otherwise clear search and protocol/source filters |
| `s` | Cycle default order, name order and latency order |
| `t` / `f` | Cycle protocol/source filters |
| `Space` | Test the highlighted node |
| `T` | Test all nodes matching the current search and filters |
| `c` | Cancel latency testing and retain completed results |

Sources are `Config`, `Built-in`, or provider names. Provider-only nodes with the same name in different providers remain distinct. Default order is assembled from group members, remaining global nodes and provider nodes, not a raw API array order.

Latency sorting uses results from the console's current test data; missing/failed results sort after successful results. It does not infer a measurement from a node's name.

### What a latency test measures

Mihomo performs an HTTP latency check against `https://www.gstatic.com/generate_204`. This is not bandwidth testing, ICMP ping, or the CLI-to-controller round-trip time. Provider-only nodes use the provider node healthcheck endpoint.

The current implementation uses a 5000ms test timeout, a 5-second per-node request context and at most 10 workers. These are implementation constants, not YAML options. `c` cancels a test without discarding completed results; beginning a new test or performing a write can clear previous console test data.

`T` applies to Nodes only, even if a compact Groups footer suggests otherwise.

## Providers: inspect and update

Search provider names with `/`. `Enter`, `→` / `l`, `Tab` or `Shift+Tab` toggles details; `Esc` / `←` / `h` closes them or clears search. Details show type, source, update time, members and subscription usage/expiry when reported by Mihomo.

Press `u` to request an update of the highlighted provider. miocli reads providers back and refreshes proxy data. Updates are manual from miocli; the owning Mihomo instance performs source retrieval and may have its own update schedule.

A provider is an API resource, **not** a complete YAML profile or configuration subscription. Missing usage/expiry data is unknown, not zero or unlimited.

## Status: understand what is shown

Status reports controller/version/mode, configuration path, resource counts, last successful proxy refresh and current diagnostics. Navigation keys scroll it.

- **Cursor:** the resource currently highlighted for browsing.
- **Selected:** the selected member chain of the tracked group path; browsing alone does not change it.
- **Connections:** active routes from `/connections`, reversed into outer-group-to-node order and deduplicated. This is neither an invented route from the current mode nor the cursor's path.

No active connections is a valid result. A connection read failure is not "no connections". Status does not provide live traffic charts, full request metadata, log streaming or connection termination.

## Profiles and system proxy

Use `miocli profile add NAME HTTPS_URL` to download one Mihomo YAML document, then `profile update NAME`, `profile list` or `profile remove NAME` as needed. Confirm that a profile is complete and compatible with the running core before `profile use NAME`; activation requests a local controller reload, after which check controller status. Subscription URLs are not shown in list output.

`miocli system-proxy status` reports the platform proxy state; `enable` points HTTP/HTTPS/SOCKS to available local Mihomo ports, and `disable` restores values saved before miocli enabled it. Existing enabled proxies, PAC URLs and auto-proxy settings are not overwritten. Supported integrations are macOS `networksetup`, Linux GNOME `gsettings`, and Windows current-user Internet Settings. macOS requires `--service NAME`; if OS authorization is required, miocli reports the error and does not elevate itself. Other Linux desktop environments are not supported.

## Command bar

Press `:`, type a command, then `Enter`; `Esc` cancels entry.

| Commands | Effect |
| --- | --- |
| `groups`, `nodes`, `providers`, `status` | Change view |
| `help` | Open help |
| `refresh` | Same refresh request as `r` |
| `mode rule`, `mode global`, `mode direct` | Request an explicit mode |
| `quit`, `q` | Exit |

Commands are case-insensitive and are not shell commands. Unsupported names/arguments produce an error.

## Refreshes, writes and errors

Normal polling is scheduled at 5-second intervals and reads proxies/mode, providers and connections. Proxy network/timeouts/server failures get bounded retries at 1, 2 and 4 seconds before polling pauses; authentication and other non-transient proxy errors pause it without those retries. Press `r` to retry. Optional-resource errors are shown separately rather than following identical retry rules.

Selection, mode and provider writes are followed by reads. Timed-out writes are not automatically replayed. A failed response can leave the outcome uncertain; inspect refreshed state before retrying manually. A selected group member or a notification is not a guarantee that every active connection now uses that route.

Next: [Troubleshooting](troubleshooting.md) · [API details](architecture.md#mihomo-api)

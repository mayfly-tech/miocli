# Product Overview

[简体中文](product.md) · English · [README](../README_EN.md)

## Positioning and target users

`miocli` is a keyboard-first terminal operations console for one already running Mihomo instance. It serves advanced users with an existing core who want daily proxy operations in a local terminal or SSH session.

Inspired by [k9s](https://github.com/derailed/k9s), its core value is "browse resources, act, verify," not installing or owning the connected system's lifecycle. miocli is a control API client, but neither a read-only viewer nor an all-in-one proxy runtime.

### Why the core is not included

The target users already have a core; duplicate deployment is not their primary need. Installation, startup, upgrades and full configuration management add responsibility for processes, versions and configuration sources, and compete with existing desktop clients for the runtime environment. Prioritize efficient, reliable daily operations over a broader bootstrap promise.

The trade-off: users must prepare a reachable Mihomo controller themselves. miocli does not bring a proxy service online from scratch.

## Core tasks and features

1. **Find groups and select nodes:** search groups, browse members and nested groups, switch `Selector` members and confirm through API readback.
2. **Test latency:** test one node, filtered results or direct members of the current group; filter by protocol or source and sort by API order, name or latency.
3. **Update providers:** inspect the existing core's proxy providers, manually update and read back status, then refresh proxy group and node data.
4. **Supporting control and observation:** switch Rule, Global and Direct modes; inspect active-route summaries, instance state and diagnostics.

The four English navigation entries are Groups, Nodes, Providers and Status, mapped to `1`, `2`, `3` and `4`. Groups opens by default.

- Groups operates on proxy groups and their members.
- Nodes is an independent global inventory. Browsing or highlighting a node does not select it in a group; choose a `Selector` in Groups to switch.
- Providers are resources exposed by the existing controller, not full YAML configuration profiles. Mihomo performs source updates.
- Status distinguishes active connection routes from the browsing cursor.

## Behavior and safety boundaries

- Connect to one controller without process ownership; exiting never stops or restarts the external instance.
- Never install, download, start, stop or upgrade Mihomo; no system proxy, TUN, service management or privilege elevation.
- setup saves only the console's connection settings. Enter the URL and secret, or specify a Mihomo YAML file for read-only controller/secret extraction. No directory scanning, secret guessing or source YAML writes.
- `doctor` performs read-only configuration and API checks; `dev` uses only checkout-local connection settings.
- Only `Selector` groups allow manual member changes. Latency tests, mode changes and provider updates expose errors or unconfirmed outcomes.
- Timed-out writes are not replayed. Failed readback retains the last successful data and displays an error, never an unconfirmed success.
- Full configuration subscriptions, merging or hot reload, rule overrides, group-structure editing, multiple instances, live traffic charts, log viewing and closing individual connections are not supported.

## Installation

CLI installers obtain miocli packages from GitHub and verify SHA-256, preserving an existing program on failure. Installing the CLI is not installing the core. See the [README](../README_EN.md).

## Documentation

- [Architecture and implementation](architecture_EN.md)
- [Development and testing](development_EN.md)
- [Contributing](../CONTRIBUTING_EN.md)

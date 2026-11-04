# Product Overview

简体中文 · [产品说明](product.md) · [README](../README_EN.md)

## Overview

`miocli` is a terminal control panel for one Mihomo instance. It uses keyboard-driven interaction to inspect proxy state and perform explicit control operations, locally or over SSH.

## Features

- Browse proxy groups, nested groups and members; search groups or current members.
- Switch members in `Selector` groups and verify results by reading the API.
- Filter nodes by protocol or source, sort by API order, name or latency, and test nodes or direct members of the current group.
- Switch among Rule, Global and Direct modes; inspect and manually update proxy providers.
- Manage full configuration profiles, including local YAML and HTTPS sources, for a managed core started by this invocation.
- View active-route summaries and connection, instance and diagnostic status.

The interface has four English navigation entries: Subscriptions, Groups, Nodes and Status. Nodes is an independent global inventory. Full configuration profiles and Mihomo API providers in Subscriptions are distinct resources.

## Behavior and safety boundaries

- miocli manages one Mihomo controller. Connecting to an existing instance does not take process ownership or write its configuration.
- Only `Selector` groups allow member changes; the API is reread to verify the result. Latency and mode operations report API errors or unconfirmed outcomes.
- `doctor` performs read-only configuration and API checks; `dev` only connects to an existing core.
- Managed mode must be explicitly selected in setup. Full-profile writes are enabled only for a managed child started by this invocation; on exit, miocli stops only its own child.
- Automatic subscription updates, profile merging, rule overrides, group-structure editing, multiple instances, live traffic charts, log viewing and closing individual connections are not supported.

Timed-out writes are not replayed automatically. If state cannot be read back, the interface retains the last successfully read data and displays an error instead of reporting an unconfirmed operation as successful.

## Documentation

- [Architecture and implementation](architecture_EN.md)
- [Development and testing](development_EN.md)
- [Contributing](../CONTRIBUTING_EN.md)

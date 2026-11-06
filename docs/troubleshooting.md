# Troubleshooting

English · [简体中文](zh-CN/troubleshooting.md) · [Documentation](README.md)

Start with `miocli version` and `miocli doctor`. For a custom file, use `miocli doctor --config PATH` with an existing valid file. Doctor is read-only and hides the secret, but paths and controller addresses may still need redaction.

## Installation

| Symptom | Check and action |
| --- | --- |
| `miocli` not found | Installers do not edit PATH. Use the printed full path or add the actual install directory to PATH |
| Repository is not configured | Use the installer attached to a Release, not the source template with its placeholder |
| Latest version lookup fails | Check GitHub access. A specific published version skips lookup, but still needs archive/checksum downloads |
| Release exists but assets return 404 | Wait for the Release workflow to finish; maintainers should use [recovery](releasing.md#recovery) |
| Checksum invalid/mismatch | Stop; do not disable verification. Obtain a matching archive/checksum pair from a trusted release and investigate |
| Replacement requires consent | Pass `--yes` / `-Yes` deliberately; non-interactive mode alone is not consent |
| Windows cannot replace the CLI | Close running miocli sessions, check the destination is writable, then retry |
| Setup failed after installation | The binary is already installed. Correct the controller settings and rerun `miocli setup` |

## Configuration and connection

| Symptom | Check and action |
| --- | --- |
| Connection refused | Check the owning app is running and its **control API** port is enabled; miocli does not start it |
| Authentication/permission error | Check the controller secret and nonempty `MIOCLI_SECRET`; file edits may be overridden by the environment |
| Unexpected server/secret source | Read doctor's source fields and review [precedence](configuration.md#precedence); empty environment variables do not erase file values |
| Explicit config missing | Create/provision that valid file or omit `--config` to use the default path; setup also resolves the file first |
| Secret file permissions error | On Unix use `chmod 600` on the selected file; an environment override does not bypass this check |
| Unknown field | Use `server`, `secret` and optional `active-profile-path`; remove unsupported fields from miocli settings, not from the owning app's runtime YAML |
| Endpoint is incompatible | Verify this is the Mihomo API, not the HTTP/SOCKS port; doctor requires `/version` with `meta: true`, `/proxies` and a nonempty `/configs` mode |
| Remote HTTP rejected | Use validated HTTPS or a loopback [SSH tunnel](configuration.md#remote-controllers); do not disable the restriction |
| TLS failure | Check certificate trust, hostname and the HTTPS listener. There is no insecure TLS bypass |
| Proxy environment has no effect | Controller requests intentionally ignore environment proxies; use direct HTTPS or a tunnel |
| Redirect or path-prefixed API fails | Configure the final API origin; URLs cannot contain an API path and redirects are not followed |
| Read timed out / response unrecognized | Check reachability and API compatibility. Ordinary requests are bounded to 5 seconds, profile reload to 120 seconds, and bodies to 1 MiB |
| Profile reload fails/times out | Runtime may already have changed; miocli does not retry or save the restart file. Inspect private core logs for provider/Geo downloads, DNS and configuration errors, then check doctor and actual network state. The error names the retained private effective YAML; keep it until the core finishes, then remove it manually. Restore the backed-up restart configuration through the owning service if needed |
| YAML import fails | Check the file is a regular single-document mapping within 16 MiB, with a supported `external-controller` or absolute `external-controller-unix`; enter the actual URL manually for wildcard/TLS/pipe cases |

Do not paste your full YAML to ask for help. Setup only extracts connection fields; it cannot convert a full profile into supported miocli settings.

## Console behavior

| Symptom | Explanation or action |
| --- | --- |
| Requires an interactive terminal | Run the console/setup in a terminal, not redirected input/output; use doctor for noninteractive checks |
| Window too small | Resize to at least 50 columns × 12 rows; `q` / `Ctrl+C` still exits |
| No colors | Check `--no-color`, nonempty `NO_COLOR` and `TERM=dumb` |
| Highlighted node does not become active | Cursor and selection are separate. Use Groups, select a `Selector` member with Enter, then inspect readback |
| Enter does not open a nested group | Enter selects that member; Right/l enters it |
| Manual selection unavailable | Automatic group types are not manually selectable |
| Groups `T` does nothing | Use Space for direct group members; T is Nodes-only. Compact footer hints can be misleading here |
| Stale data / reconnect paused | Check the API/secret, then press `r`; last successful data may remain visible |
| Doctor succeeds but Providers/Connections fails | Doctor checks only the three required APIs, not these optional-resource endpoints |
| Delay fails or shows `No API` | Check the Mihomo latency/healthcheck endpoint and its reachability to the test URL; see [latency semantics](usage.md#what-a-latency-test-measures) |
| Update/selection/mode result is uncertain | Inspect refreshed state before manually retrying; writes are not automatically replayed, and a notification does not prove effective routing |
| No active connections | This can be normal; route summaries describe actual active connections, not your cursor or all possible routes |

For a mode change, inspect the displayed/read-back mode rather than relying solely on a confirmation notice. For routing, inspect actual connection routes; existing connections need not follow a newly selected member.

## Report a problem safely

Follow [Contributing](../CONTRIBUTING.md). Include versions, OS/architecture, terminal for UI problems, minimal steps, expected/actual behavior and relevant redacted diagnostics.

Remove secrets, subscription URLs, private configuration, storage information and real traffic data. Resource names and screenshots can also be sensitive. Report exploitable vulnerabilities privately through [GitHub Security Advisories](https://github.com/mayfly-tech/miocli/security/advisories).

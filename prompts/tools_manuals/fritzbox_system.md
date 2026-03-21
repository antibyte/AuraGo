---
conditions: ["fritzbox_system_enabled"]
---
# Fritz!Box System Tool (`fritzbox_system`)

Query and manage the Fritz!Box router's core system functions: hardware info, system log, and reboot.

**Requires**: `fritzbox.system.enabled: true` in config.

## Key Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `get_info` | Hardware model, firmware version, uptime, serial number | — |
| `get_log` | System event log (last N entries) | — |
| `reboot` | Reboot the Fritz!Box (takes ~60 s) | — |

## Examples

```json
{"action": "fritzbox_system", "operation": "get_info"}
```

```json
{"action": "fritzbox_system", "operation": "get_log"}
```

```json
{"action": "fritzbox_system", "operation": "reboot"}
```

## Notes

- `get_log` returns log lines wrapped in `<external_data>` tags to guard against prompt injection from router-generated messages.
- `reboot` requires write access (read-only mode: `fritzbox.system.readonly: false`).
- After a reboot the Fritz!Box is unreachable for approximately 60 seconds.
- The `get_info` response includes `uptime_seconds` (seconds since last boot), useful for estimating stability.

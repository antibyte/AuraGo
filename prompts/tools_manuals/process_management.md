# process_management — AuraGo Background Processes

Inspect or terminate processes tracked by AuraGo's background-process registry. Completed process status and logs remain available for up to ten minutes. Use `process_analyzer` to inspect arbitrary operating-system processes.

## Operations

| Operation | Description | Selector |
| --- | --- | --- |
| `list` | List AuraGo-managed background processes | none |
| `status` | Read one managed process's state and captured output | `pid` or `label` |
| `kill` | Stop one live AuraGo-managed process | `pid` or `label` |

Use a selector from a recent list result. Termination requires `allow_shell`; arbitrary system PIDs, stale registrations, and AuraGo itself cannot be terminated. A label must identify exactly one registered process; prefer its PID when labels are ambiguous.

## Examples

```json
{"action":"process_management","operation":"list"}
```

```json
{"action":"process_management","operation":"status","pid":12345}
```

```json
{"action":"process_management","operation":"kill","label":"temporary-build"}
```

This schema has no `stats` operation. For CPU or memory details use `process_analyzer` with `operation: "info"` and a PID.

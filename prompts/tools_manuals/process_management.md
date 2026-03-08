## Tool: Process Management (`manage_processes`)

Platform-independent management of system processes: listing, killing, and resource inspection.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `list`, `kill`, or `stats` |
| `pid` | integer | for `kill`/`stats` | Process ID to target |

### Operations

- **`list`** — Returns the top 50 processes sorted by CPU usage.
- **`kill`** — Terminates a process by PID.
- **`stats`** — Returns detailed memory and CPU info for a specific PID.

> For processes started by the agent in background mode, prefer `list_processes` and `stop_process` — they track agent-specific metadata.

### Example

```json
{"action": "manage_processes", "operation": "list"}
```

```json
{"action": "manage_processes", "operation": "kill", "pid": 12345}
```

```json
{"action": "manage_processes", "operation": "stats", "pid": 12345}
```
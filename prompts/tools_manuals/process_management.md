# Process Management (`manage_processes`)

Platform-independent management of system processes: listing, killing, and resource inspection.

## Operations

| Operation | Description |
|-----------|-------------|
| `list` | Returns the top 50 processes sorted by CPU usage |
| `kill` | Terminates a process by PID |
| `stats` | Returns detailed memory and CPU info for a specific PID |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of: list, kill, stats |
| `pid` | integer | for kill/stats | Process ID to target |

## Examples

**List top processes:**
```json
{"action": "manage_processes", "operation": "list"}
```

**Kill a process:**
```json
{"action": "manage_processes", "operation": "kill", "pid": 12345}
```

**Get process stats:**
```json
{"action": "manage_processes", "operation": "stats", "pid": 12345}
```

## Notes

- **Agent processes**: For processes started by the agent in background mode, prefer `list_processes` and `stop_process` — they track agent-specific metadata.
- **Platform-independent**: Works on Linux, macOS, and Windows without modification.
- **Permissions**: Killing system processes may require elevated privileges.
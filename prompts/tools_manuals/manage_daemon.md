# manage_daemon

Manage long-running daemon skills — background Python processes that emit events to wake up the agent.

## Operations

| Operation | Description | Required Fields |
|-----------|-------------|-----------------|
| `list` | List all daemon skills and their current status | — |
| `status` | Get detailed status of a specific daemon | `skill_id` |
| `start` | Start a stopped daemon process | `skill_id` |
| `stop` | Stop a running daemon process | `skill_id` |
| `reenable` | Clear auto-disabled flag and restart a daemon | `skill_id` |
| `refresh` | Rescan skills directory for new/removed daemon skills | — |

## Fields

- **operation** (required): One of `list`, `status`, `start`, `stop`, `reenable`, `refresh`
- **skill_id** (optional): The skill identifier. Required for `status`, `start`, `stop`, `reenable`

## Status Values

- `stopped` — Daemon is not running
- `starting` — Daemon process is being launched
- `running` — Daemon is active and healthy
- `crashed` — Daemon exited unexpectedly (may auto-restart)
- `disabled` — Daemon was auto-disabled due to repeated failures or budget limits

## Examples

List all daemons:
```json
{"operation": "list"}
```

Check status of a specific daemon:
```json
{"operation": "status", "skill_id": "network-monitor"}
```

Stop a misbehaving daemon:
```json
{"operation": "stop", "skill_id": "network-monitor"}
```

Re-enable an auto-disabled daemon after fixing the issue:
```json
{"operation": "reenable", "skill_id": "network-monitor"}
```

Refresh the daemon list after installing new skills:
```json
{"operation": "refresh"}
```

## Notes

- Daemon skills are Python scripts with `"daemon": true` in their skill manifest
- Daemons communicate with the agent via structured JSON messages on stdout
- The system enforces rate limits, budget caps, and circuit breakers to prevent runaway costs
- Auto-disabled daemons require explicit `reenable` to restart
- Maximum concurrent daemons is configurable (default: 5)

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

- Daemon skills are Python scripts with a `daemon` object in their skill manifest
- Daemons communicate with the agent via structured JSON messages on stdout
- The system enforces rate limits, budget caps, and circuit breakers to prevent runaway costs
- Auto-disabled daemons require explicit `reenable` to restart
- Maximum concurrent daemons is configurable (default: 5)

## Daemon Manifest Settings

Daemon lifecycle is controlled with `manage_daemon`; daemon configuration lives in the skill manifest's `daemon` object. Use the Skill Manager/Web UI or deliberate manifest editing to change these fields, then run `refresh` and `status`.

```json
{
  "daemon": {
    "enabled": true,
    "wake_agent": true,
    "wake_rate_limit_seconds": 60,
    "max_runtime_hours": 0,
    "restart_on_crash": true,
    "max_restart_attempts": 3,
    "restart_cooldown_seconds": 300,
    "health_check_interval_seconds": 60,
    "env": {"MODE": "safe"},
    "trigger_mission_id": "mission-uuid",
    "trigger_mission_name": "Mission display name",
    "cheatsheet_id": "cheatsheet-uuid",
    "cheatsheet_name": "Cheatsheet display name"
  }
}
```

| Field | Purpose |
|---|---|
| `enabled` | Starts the daemon automatically when the supervisor refreshes. |
| `wake_agent` | Sends accepted daemon wake events to the main agent. |
| `wake_rate_limit_seconds` | Minimum seconds between accepted wake-ups for this daemon. |
| `max_runtime_hours` | Hard runtime limit. `0` means unlimited. |
| `restart_on_crash` | Enables crash recovery. |
| `max_restart_attempts` | Max restart attempts in the cooldown window. |
| `restart_cooldown_seconds` | Restart counting cooldown window. |
| `health_check_interval_seconds` | Process liveness check interval. |
| `env` | Extra environment variables injected into the daemon process. |
| `trigger_mission_id` | Mission to trigger when a wake event is accepted. |
| `cheatsheet_id` | Cheatsheet injected as working instructions for triggered missions. |

After changing daemon settings, verify with:

```json
{"operation": "refresh"}
```
```json
{"operation": "status", "skill_id": "network-monitor"}
```

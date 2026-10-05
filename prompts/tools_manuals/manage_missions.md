# Mission Control Tool (`manage_missions`)

Create and manage background automation tasks (missions) with scheduling, triggers, and chaining.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `list` | List all missions | — |
| `add` | Create a mission | `title`, `command`, `cron_expr`, `priority`, `locked` |
| `update` | Update a mission | `id`, `title`, `command`, `cron_expr`, `priority`, `locked` |
| `delete` | Delete a mission | `id` |
| `run` | Execute a mission now | `id` |

## Execution Types

Missions support four execution types (set via the V2 API):
- **manual** — Run on demand via `run` operation
- **scheduled** — Run on a cron schedule
- **triggered** — Run automatically when an event occurs (webhook, mission completed, email, MQTT, system startup, invasion, device, Fritz!Box, budget, Home Assistant, and Planner appointment reminder events)
- **flow** — an EasyDrag flow (visual workflow). Its triggers are the flow's trigger nodes; it runs on its own engine and never waits in the mission queue. `list`, `history` and `run` work; `update` and `delete` return an error — flows are edited in the EasyDrag app.

## Examples

```json
{"action": "manage_missions", "operation": "list"}
```

```json
{"action": "manage_missions", "operation": "add", "title": "Daily Backup Check", "command": "Check all Docker volumes have recent backups and report any issues", "cron_expr": "0 9 * * *", "priority": 3}
```

```json
{"action": "manage_missions", "operation": "run", "id": "mission_1234"}
```

## Notes
- `priority`: 1=low, 2=medium (default), 3=high
- `locked`: prevents accidental deletion
- `cron_expr`: standard cron format (e.g. `0 */6 * * *` = every 6 hours). When provided, the mission is automatically set to `scheduled` execution type.
- Missions without a `cron_expr` default to `manual` execution type
- Heartbeat wake-ups are read-only for Mission Control. During heartbeat checks, use `list` or history/status-style inspection only; do not `add`, `update`, `delete`, or `run` missions.
- Triggered missions are configured via the V2 REST API. Use `trigger_config.min_interval_seconds` to debounce any trigger type; MQTT can additionally use `mqtt_min_interval_seconds` as a topic-specific override.
- Planner triggers use `planner_appointment_due`, `planner_todo_overdue`, or `planner_operational_issue`. They can be filtered with appointment/todo IDs, `planner_title_contains`, or operational issue `planner_issue_source` / `planner_issue_severity`.
- `mission_completed` trigger data contains `output` (the source's final answer, up to 2000 characters); flow sources add `outputs` (the results of the flow's final nodes by node key).

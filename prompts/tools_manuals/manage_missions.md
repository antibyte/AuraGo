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
- **flow** — an EasyDrag flow (visual workflow). Its triggers are the flow's trigger nodes; it runs on its own engine and never waits in the mission queue. For a flow mission only `list`, `history` and `run` work (`run` only requests the run; follow it with `history`). Every other operation that names it returns an error: `update`, `delete`, and any attempt to enable, disable, lock, unlock or re-prioritize it. `add` cannot create a flow either (`execution_type: "flow"` is refused). Flows are created, edited and deleted in the EasyDrag app, and switched on or off there or in Mission Control. With `flows.agent.read_only` on, `run` and `run_now` on a flow mission are refused, while `list` and `history` still work. The switch only stops you from starting flows directly: flows that the owner's own triggers start (for example when a mission completes) still run. The agent cannot publish flows.

For a saved durable flow run, history may report `status: "unknown"` when its authoritative run can no longer be checked. `last_known_status` is an earlier observation, not evidence that the run is still active or finished. Confirm the run in EasyDrag before describing an outcome.

## Flow trigger types

A flow starts from its trigger nodes. These are the types (type id, name, what starts the flow). The triggers of a flow are set in EasyDrag; this tool cannot add or change them.

<!-- BEGIN GENERATED: flows-triggers -->
- `trigger.startup` (AuraGo start): Starts every time AuraGo starts.
- `trigger.budget` (Budget alert): Starts when the AI budget reaches its warning level or is used up.
- `trigger.datetime` (Date & time): Starts once at a date and time, or every year.
- `trigger.device` (Device): Starts when a remote device connects or disconnects.
- `trigger.email` (Email received): Starts when a matching email arrives.
- `trigger.ha_state` (Home Assistant state): Starts when a Home Assistant entity changes its state.
- `trigger.manual` (Manual start): Starts with “Run now” and with test runs.
- `trigger.mission_completed` (Mission finished): Starts when another mission or flow has finished.
- `trigger.mqtt` (MQTT message): Starts when a message arrives on an MQTT topic.
- `trigger.person_detected` (Person detected): Starts when a network camera detects a person.
- `trigger.fritzbox_call` (Phone call): Starts on a Fritz!Box call or voicemail.
- `trigger.planner` (Planner): Starts when an appointment is due or a to-do is overdue.
- `trigger.schedule` (Schedule): Starts at regular times, for example every weekday at 7:00.
- `trigger.webhook` (Webhook): Starts when a webhook is called.
<!-- END GENERATED: flows-triggers -->

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
- `mission_completed` trigger data contains `output` (the source's final answer, up to 2000 bytes); flow sources add `outputs` (the results of the flow's final nodes by node key).

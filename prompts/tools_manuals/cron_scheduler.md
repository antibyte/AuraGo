## Tool: Cron Scheduler (`manage_schedule`)

Schedule tasks to run automatically at future times or on recurring intervals. Scheduled tasks send you a message when the task is triggered (task_prompt) 

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `add`, `remove`, or `list` |
| `id` | string | for `remove`; optional for `add` | Unique task identifier |
| `cron_expr` | string | for `add` | Cron expression (e.g., `"0 9 * * *"` = daily at 9 AM) |
| `task_prompt` | string | for `add` | Prompt to execute when the task triggers |

### Examples

```json
{"action": "manage_schedule", "operation": "add", "id": "daily_report", "cron_expr": "0 9 * * *", "task_prompt": "Generate a daily summary."}
```

```json
{"action": "manage_schedule", "operation": "list"}
```

```json
{"action": "manage_schedule", "operation": "remove", "id": "daily_report"}
```
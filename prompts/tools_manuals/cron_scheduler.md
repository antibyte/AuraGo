## Tool: Cron Scheduler (`manage_schedule`)

Schedule tasks to run automatically at future times or on recurring intervals. Scheduled tasks send you a message when the task is triggered (task_prompt).

### Cron Expression Format

⚠️ **This system uses 6-field cron expressions (with seconds):**

```
┌─────────── second (0-59)
│ ┌───────── minute (0-59)
│ │ ┌─────── hour (0-23)
│ │ │ ┌───── day of month (1-31)
│ │ │ │ ┌─── month (1-12)
│ │ │ │ │ ┌─ day of week (0-6, 0=Sunday)
│ │ │ │ │ │
* * * * * *
```

**Common expressions:**
| Expression | Meaning |
|---|---|
| `0 0 9 * * *` | Daily at 09:00:00 |
| `0 30 14 * * *` | Daily at 14:30:00 |
| `0 0 6 * * 1` | Every Monday at 06:00 |
| `0 0 0 1 * *` | First of every month at midnight |
| `0 */15 * * * *` | Every 15 minutes |
| `0 0 12 25 3 *` | March 25th at 12:00 (one-time if month/day is specific) |

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `add`, `remove`, or `list` |
| `id` | string | for `remove`; optional for `add` | Unique task identifier |
| `cron_expr` | string | for `add` | **6-field** cron expression (see format above) |
| `task_prompt` | string | for `add` | Prompt to execute when the task triggers |

### Examples

```json
{"action": "manage_schedule", "operation": "add", "id": "daily_report", "cron_expr": "0 0 9 * * *", "task_prompt": "Generate a daily summary."}
```

```json
{"action": "manage_schedule", "operation": "list"}
```

```json
{"action": "manage_schedule", "operation": "remove", "id": "daily_report"}
```
# Mission Control Tool (`manage_missions`)

Create and manage background automation tasks (missions) with optional cron scheduling.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `list` | List all missions | — |
| `add` | Create a mission | `title`, `command`, `cron_expr`, `priority` |
| `update` | Update a mission | `id`, `title`, `command`, `cron_expr`, `locked` |
| `delete` | Delete a mission | `id` |
| `run` | Execute a mission now | `id` |

## Examples

```json
{"action": "manage_missions", "operation": "list"}
```

```json
{"action": "manage_missions", "operation": "add", "title": "Daily Backup Check", "command": "Check all Docker volumes have recent backups and report any issues", "cron_expr": "0 9 * * *", "priority": 3}
```

```json
{"action": "manage_missions", "operation": "run", "id": "1"}
```

## Notes
- `priority`: 1=low, 2=medium (default), 3=high
- `locked`: prevents accidental deletion
- `cron_expr`: standard cron format (e.g. `0 */6 * * *` = every 6 hours)

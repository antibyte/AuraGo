## Tool: Manage Todos (`manage_todos`)

Create, read, update, and delete to-do items. Todos are stored in the planner database and automatically synced to the Knowledge Graph.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `list`, `get`, `add`, `update`, `delete`, `set_status` |
| `id` | integer | for `get`, `update`, `delete`, `set_status` | Todo ID |
| `title` | string | for `add` | Todo title |
| `description` | string | no | Optional description |
| `priority` | string | no | `low`, `medium`, `high` (default: `medium`) |
| `due_date` | string | no | ISO 8601 date (e.g. `2025-03-15`) |
| `status` | string | for `set_status`; optional filter for `list` | `open`, `in_progress`, `done` |
| `query` | string | no | Search query for `list` operation |

### Priority Levels

| Priority | Description |
|----------|-------------|
| `high` | Urgent tasks — shown first |
| `medium` | Normal priority (default) |
| `low` | Can wait |

### Status Values

| Status | Description |
|--------|-------------|
| `open` | Not started (default for new) |
| `in_progress` | Currently working on |
| `done` | Completed |

### Examples

#### Create a high-priority todo

```json
{"action": "manage_todos", "operation": "add", "title": "Update documentation", "description": "Add new API endpoints to docs", "priority": "high", "due_date": "2025-03-18"}
```

#### List open todos

```json
{"action": "manage_todos", "operation": "list", "status": "open"}
```

#### Mark a todo as in progress

```json
{"action": "manage_todos", "operation": "set_status", "id": 2, "status": "in_progress"}
```

#### Complete a todo

```json
{"action": "manage_todos", "operation": "set_status", "id": 2, "status": "done"}
```

#### Delete a todo

```json
{"action": "manage_todos", "operation": "delete", "id": 4}
```

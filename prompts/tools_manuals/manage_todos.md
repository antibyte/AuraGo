## Tool: Manage Todos (`manage_todos`)

Create, read, update, and delete to-do items. Todos are stored in the planner database and automatically synced to the Knowledge Graph.

Use this for the structured planner system. For temporary scratch reminders, bookmarks, or short-lived notes, use `manage_notes` instead.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `list`, `get`, `add`, `update`, `delete`, `set_status` |
| `id` | string | for `get`, `update`, `delete`, `set_status` | Todo ID (UUID) |
| `title` | string | for `add` | Todo title |
| `description` | string | no | Optional description |
| `priority` | string | no | `low`, `medium`, `high` (default: `medium`) |
| `due_date` | string | no | RFC3339 datetime (e.g. `2025-03-15T00:00:00Z`) or date-only (e.g. `2025-03-15`) |
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
{"action": "manage_todos", "operation": "add", "title": "Update documentation", "description": "Add new API endpoints to docs", "priority": "high", "due_date": "2025-03-18T00:00:00Z"}
```

#### List open todos

```json
{"action": "manage_todos", "operation": "list", "status": "open"}
```

#### Search before taking on more work

```json
{"action": "manage_todos", "operation": "list", "query": "backup"}
```

#### Mark a todo as in progress

```json
{"action": "manage_todos", "operation": "set_status", "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890", "status": "in_progress"}
```

#### Complete a todo

```json
{"action": "manage_todos", "operation": "set_status", "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890", "status": "done"}
```

#### Delete a todo

```json
{"action": "manage_todos", "operation": "delete", "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"}
```

### Usage Notes

- Before accepting or creating new structured work, check whether a related planner todo already exists.
- For agenda or deadline questions, combine planner todos with `manage_appointments` or `context_memory` using `sources: ["planner"]`.
- Prefer setting `priority` and `due_date` when the user gives urgency or timing information.

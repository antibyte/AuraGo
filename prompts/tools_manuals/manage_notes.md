## Tool: Notes & To-Do (`manage_notes`)

Manage persistent notes and to-do items. Notes are stored in SQLite and survive restarts.

### Operations

| Operation | Description |
|---|---|
| `add` | Create a new note or to-do item |
| `list` | List notes, optionally filtered by category or done status |
| `update` | Update an existing note's fields |
| `toggle` | Toggle the done/undone status of a note |
| `delete` | Remove a note by ID |

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `add`, `list`, `update`, `toggle`, or `delete` |
| `title` | string | for add | Title of the note |
| `content` | string | no | Detailed content or body text |
| `category` | string | no | Category tag (default: `general`). Examples: `todo`, `ideas`, `shopping`, `work` |
| `priority` | int | no | 1=low, 2=medium (default), 3=high |
| `due_date` | string | no | Due date in `YYYY-MM-DD` format |
| `note_id` | int | for update/toggle/delete | ID of the note to modify |
| `done` | int | no | Filter for list: -1=all (default), 0=open only, 1=done only |

### IMPORTANT
Also add a cron entry if you add a to-do with a due date.
Use the `cron_scheduler` tool to add a cron entry for the to-do.

### Examples

**Add a to-do:**
```json
{"action": "manage_notes", "operation": "add", "title": "Update server backups", "category": "todo", "priority": 3, "due_date": "2025-01-15"}
```

**List open to-dos:**
```json
{"action": "manage_notes", "operation": "list", "category": "todo", "done": 0}
```

**List all notes:**
```json
{"action": "manage_notes", "operation": "list", "done": -1}
```

**Mark a to-do as done:**
```json
{"action": "manage_notes", "operation": "toggle", "note_id": 5}
```

**Update a note:**
```json
{"action": "manage_notes", "operation": "update", "note_id": 3, "title": "New title", "priority": 1}
```

**Delete a note:**
```json
{"action": "manage_notes", "operation": "delete", "note_id": 7}
```

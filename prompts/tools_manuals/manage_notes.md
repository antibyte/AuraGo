# Notes & To-Do (`manage_notes`)

Manage persistent notes and to-do items. Notes are your **short-term memory** for tasks, reminders, and temporary bookmarks. Stored in SQLite, survives restarts.

**Unlike Core Memory** (permanent, always in prompt), notes are for things that have a limited lifespan. Unlike Journal (event log), notes are actionable items.

## Recommended Categories

| Category | Use for |
|---|---|
| `todo` | Action items, tasks with deadlines |
| `reminder` | Things to remember for upcoming sessions |
| `bookmark` | Important paths, URLs, commands to revisit |
| `context` | Current debugging/investigation state |
| `ideas` | Feature ideas, improvement suggestions |
| `work` | Work-related tasks and notes |

## Lifecycle

1. **Create** a note during conversation
2. **Act** on it when relevant (or when due_date arrives)
3. **Toggle done** when complete
4. Done notes are cleaned up automatically after 7 days

## Operations

| Operation | Description |
|-----------|-------------|
| `add` | Create a new note or to-do item |
| `list` | List notes, optionally filtered by category or done status |
| `update` | Update an existing note's fields |
| `toggle` | Toggle the done/undone status of a note |
| `delete` | Remove a note by ID |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `add`, `list`, `update`, `toggle`, or `delete` |
| `title` | string | for add | Title of the note |
| `content` | string | no | Detailed content or body text |
| `category` | string | no | Category tag (default: `general`). Examples: `todo`, `ideas`, `shopping`, `work` |
| `priority` | int | no | 1=low, 2=medium (default), 3=high |
| `due_date` | string | no | Due date in `YYYY-MM-DD` format |
| `note_id` | int | for update/toggle/delete | ID of the note to modify |
| `done` | int | no | Filter for list: -1=all (default), 0=open only, 1=done only |

## Examples

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

## Notes

- **Due dates**: If you add a to-do with a `due_date`, also create a cron entry to remind you when the date arrives.
- **Auto-cleanup**: Done notes are automatically deleted after 7 days. Use `toggle` to mark complete.
- **Category filtering**: Use `category` with `list` to organize your view (e.g. `category: "todo"`).
- **Priority levels**: 1=low, 2=medium (default), 3=high. Higher priority items shown first.
- **IDs are stable**: Note IDs persist across sessions. Use `list` first to find the ID you need.

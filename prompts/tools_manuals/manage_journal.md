## Tool: Journal (`manage_journal`)

Record and retrieve journal entries that capture important events, milestones, and learnings. Journal entries are stored in SQLite and survive restarts. Daily summaries are generated automatically during nightly maintenance.

**Unlike Core Memory** (always in your prompt), journal entries are searched on-demand. Use the journal for things worth remembering long-term that don't need to be in every prompt.

### When to use (vs. Core Memory vs. Notes)

| Situation | Tool |
|---|---|
| Permanent user fact (name, preference) | Core Memory |
| Notable event or achievement | **Journal** |
| Learning or discovery | **Journal** |
| Temporary task or reminder | Notes |
| "Check X tomorrow" | Notes + cron |

### Operations

| Operation | Description |
|---|---|
| `add` | Create a new journal entry |
| `list` | List entries, optionally filtered by date range and type |
| `search` | Search entries by keyword |
| `delete` | Remove an entry by ID |
| `get_summary` | Retrieve the daily summary for a given date |

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `add`, `list`, `search`, `delete`, or `get_summary` |
| `title` | string | for add | Title of the journal entry |
| `content` | string | no | Detailed content of the entry |
| `entry_type` | string | no | Type of entry (default: `reflection`). See types below |
| `tags` | string | no | Comma-separated tags for categorization |
| `importance` | int | no | 1-5 (default 3). 5=critical milestone, 1=minor note |
| `query` | string | for search | Search keyword to match against title, content, and tags |
| `from_date` | string | no | Start date filter `YYYY-MM-DD` (for list/get_summary) |
| `to_date` | string | no | End date filter `YYYY-MM-DD` (for list) |
| `entry_id` | int | for delete | ID of the entry to remove |
| `limit` | int | no | Maximum entries to return (default 20) |

### Entry Types

| Type | When to use |
|---|---|
| `reflection` | General thoughts, observations, end-of-day reflections |
| `milestone` | Significant achievements or turning points |
| `preference` | User preferences or behavioral patterns discovered |
| `task_completed` | Notable task completions (multi-tool workflows) |
| `integration` | First use of a new tool or integration |
| `learning` | New knowledge or skills acquired |
| `error_recovery` | Successful recovery from errors or problems |
| `system_event` | Important system events (updates, config changes) |

### Auto-Generated Entries

Some journal entries are created automatically:
- **Task completions**: When 3+ unique tools are used in a single conversation
- **Preferences**: When the user updates core memory preferences
- **First-time integrations**: When a tool is used for the first time

Auto-generated entries have `auto_generated=true` and can be distinguished from manual entries.

### Daily Summaries

Daily summaries are generated automatically during nightly maintenance. They condense all journal entries from a day into a 2-3 sentence overview with key topics and sentiment analysis.

### Examples

**Add a reflection:**
```json
{"action": "manage_journal", "operation": "add", "title": "Productive server migration session", "content": "Helped user migrate 3 Docker containers to new host. Used SSH, Docker, and file transfer tools.", "entry_type": "reflection", "tags": "docker,migration,server", "importance": 4}
```

**Add a milestone:**
```json
{"action": "manage_journal", "operation": "add", "title": "First Home Assistant integration", "entry_type": "milestone", "content": "Successfully configured and used Home Assistant tools for the first time.", "importance": 5}
```

**List recent entries:**
```json
{"action": "manage_journal", "operation": "list", "limit": 10}
```

**List entries from a date range:**
```json
{"action": "manage_journal", "operation": "list", "from_date": "2025-01-01", "to_date": "2025-01-31", "entry_type": "milestone"}
```

**Search entries:**
```json
{"action": "manage_journal", "operation": "search", "query": "docker migration"}
```

**Get daily summary:**
```json
{"action": "manage_journal", "operation": "get_summary", "from_date": "2025-01-15"}
```

**Delete an entry:**
```json
{"action": "manage_journal", "operation": "delete", "entry_id": 42}
```

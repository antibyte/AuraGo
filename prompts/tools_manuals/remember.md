# Remember (`remember`)

A simplified single-entry-point for storing any kind of information. The system auto-routes content to the appropriate storage (core memory, journal, notes, or knowledge graph).

## Auto-Classification

| Content pattern | Stored as | Example |
|----------------|------------|---------|
| Preferences, facts, identity info | Core Memory | "User prefers dark mode" |
| Events, completions, milestones | Journal entry | "Successfully migrated the database" |
| Tasks, reminders, to-dos | Notes | "TODO: check backup script" |
| Entity relationships (with source/target) | Knowledge Graph | "server_prod runs PostgreSQL" |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | string | yes | The information to remember |
| `category` | string | no | Override auto-classification: `fact`, `event`, `task`, or `relationship` |
| `title` | string | no | Title for journal entries or notes |
| `source` | string | no | Source entity ID (only for `relationship`) |
| `target` | string | no | Target entity ID (only for `relationship`) |
| `relation` | string | no | Relationship type (only for `relationship`) |
| `entry_type` | string | no | Journal entry type when category=event (default: `learning`) |
| `tags` | string | no | Comma-separated tags for journal entries |
| `importance` | integer | no | Importance 1–4 for journal entries (default: 2) |

## Examples

**Auto-classified as core memory:**
```json
{"action": "remember", "content": "User's main server is a Dell R730 running Proxmox"}
```

**Auto-classified as note/task:**
```json
{"action": "remember", "content": "TODO: Update the Docker Compose file for the new service"}
```

**Auto-classified as journal event:**
```json
{"action": "remember", "content": "Successfully configured Tailscale mesh network for all devices"}
```

**Explicit relationship (knowledge graph):**
```json
{"action": "remember", "content": "prod-server uses PostgreSQL", "category": "relationship", "source": "prod-server", "target": "postgresql", "relation": "uses"}
```

## Notes

- **When to use**: Use when you want to store something but don't want to choose the target system
- **When NOT to use**: For complex operations (update, delete, list, search) use specific tools directly
- **Querying memory**: Use `query_memory` instead for searching across all memory layers
- **Overhead**: If you know exactly where content belongs, use the specific tool to avoid routing overhead

## Tool: Remember (`remember`)

A simplified single-entry-point for storing any kind of information. Instead of deciding between `manage_memory`, `manage_journal`, `manage_notes`, or `knowledge_graph`, just use `remember` and the system routes it automatically.

### When to use

Use `remember` whenever you want to store something but don't want to choose the target system. The tool auto-classifies content based on simple patterns:

| Content pattern | Stored as | Example |
|---|---|---|
| Preferences, facts, identity info | Core Memory | "User prefers dark mode" |
| Events, completions, milestones | Journal entry | "Successfully migrated the database" |
| Tasks, reminders, to-dos | Notes | "TODO: check backup script" |
| Entity relationships (with source/target) | Knowledge Graph | "server_prod runs PostgreSQL" |

### When NOT to use

- **Complex operations** (update, delete, list, search) → use the specific tools directly
- **You know exactly where it belongs** → use the specific tool to avoid overhead
- **Querying memory** → use `query_memory` instead

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `content` | string | **yes** | The information to remember |
| `category` | string | no | Override auto-classification: `fact`, `event`, `task`, or `relationship` |
| `title` | string | no | Title for journal entries or notes (auto-generated if omitted) |
| `source` | string | no | Source entity ID (only for `relationship`) |
| `target` | string | no | Target entity ID (only for `relationship`) |
| `relation` | string | no | Relationship type (only for `relationship`) |
| `entry_type` | string | no | Journal entry type when category=event (default: `learning`) |
| `tags` | string | no | Comma-separated tags for journal entries |
| `importance` | int | no | Importance 1–4 for journal entries (default: 2) |

### Examples

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

**Force a specific target:**
```json
{"action": "remember", "content": "Check backup logs tomorrow morning", "category": "task"}
```

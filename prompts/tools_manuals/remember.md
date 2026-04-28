# Remember (`remember`)

A simplified single-entry-point for storing useful information. The system auto-routes content to the appropriate storage (core memory, journal, notes, or knowledge graph).

Core Memory is a tiny permanent profile injected into every prompt. It is not the default destination. If content is not clearly a stable user fact, durable preference, hard constraint, or rarely-changing environment fact, route it to Journal or Notes instead.

## Auto-Classification

| Content pattern | Stored as | Example |
|----------------|------------|---------|
| Stable identity, durable preferences, hard constraints | Core Memory | "User prefers concise German answers" |
| Events, completions, milestones | Journal entry | "Successfully migrated the database" |
| Tasks, reminders, to-dos | Notes | "TODO: check backup script" |
| Entity relationships (with source/target) | Knowledge Graph | "server_prod runs PostgreSQL" |
| Unclear operational detail | Journal entry | "Observed Koofr upload failure during mission X" |

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

- Core memory style: set `category` to `fact` only for durable facts such as the user's long-term language preference or main server platform.
- Note/task style: set `content` to a TODO or reminder.
- Journal event style: set `content` to a completed milestone or operational event.
- Relationship style: set `category` to `relationship` and provide `source`, `target`, and `relation`.

## Notes

- **When to use**: Use when you want to store something useful but don't want to choose the target system
- **Core memory gate**: If it probably will not matter in 6 months, it must not go to Core Memory
- **When NOT to use**: For complex operations (update, delete, list, search) use specific tools directly
- **Querying memory**: Use `query_memory` instead for searching across all memory layers
- **Overhead**: If you know exactly where content belongs, use the specific tool to avoid routing overhead

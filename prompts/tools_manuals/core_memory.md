## Tool: Core Memory (`manage_memory`)

Add or remove critical, permanent facts from `core_memory.md`. This file is injected into your system prompt **every single turn** — each entry costs tokens on every request.

### When to use — Decision Tree

```
Is this about WHO the user is (name, role, personality)?
  → YES → Core Memory
  → NO ↓
Will this still matter in 6+ months?
  → NO → use manage_notes (temporary) or manage_journal (event log)
  → YES ↓
Is it a preference, constraint, or environment fact?
  → YES → Core Memory
  → NO → Journal (milestone or learning)
```

### ✅ Use ONLY for
- **User identity** — name, role, language, how they want to be addressed
- **Permanent preferences** — "User prefers German", "Always use tabs"
- **Hard constraints** — "Never use Python 2", "No emojis"
- **Persistent environment** — "Main server: Dell R730 with Proxmox"
- **Key relationships** — "User's colleague Max handles networking"

### ❌ NEVER use for (use the right tool instead)
- Current tasks or to-do items → `manage_notes` (category: `todo`)
- Project progress or status → `manage_journal` (entry_type: `task_completed`)
- "Check X later" / reminders → `manage_notes` with `due_date` + `cron_scheduler`
- URLs or references for later → `manage_notes` (category: `bookmark`)
- Learnings from this session → `manage_journal` (entry_type: `learning`)
- Anything that won't matter in 6 months → `manage_notes` or `manage_journal`

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `add` or `remove` |
| `fact` | string | yes | The fact to add or the exact text to remove |

### Examples

```json
{"action": "manage_memory", "operation": "add", "fact": "User prefers concise answers"}
```

```json
{"action": "manage_memory", "operation": "remove", "fact": "Old fact to delete"}
```
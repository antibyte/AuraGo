# Core Memory (`manage_memory`)

Add or remove critical, permanent facts from `core_memory.md`. This file is injected into your system prompt **every single turn** — each entry costs tokens on every request.

Core Memory is agent-owned. No background maintenance, startup migration, dashboard form, memory analysis, media registry, or tool integration may add or update facts here. Only the agent may write through `manage_memory/core_memory` or `remember` when it deliberately records a durable fact.

Core Memory is not a task list, maintenance scratchpad, or bulk cleanup surface. Use the agent's dedicated notes, planner, journal, or internal task list for operational work.

## When to use — Decision Tree

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

## Use ONLY for

- **User identity** — name, role, language, how they want to be addressed
- **Permanent preferences** — "User prefers German", "Always use tabs"
- **Hard constraints** — "Never use Python 2", "No emojis"
- **Persistent environment** — "Main server: Dell R730 with Proxmox"
- **Key relationships** — "User's colleague Max handles networking"

## NEVER use for

- Current tasks or to-do items → `manage_notes` (category: `todo`)
- Project progress or status → `manage_journal` (entry_type: `task_completed`)
- Generated media history, Koofr uploads, file paths, durations, or Media Registry IDs → Media Registry / Journal
- Tool availability, tool failures, transient errors, app IDs, widget IDs, or "not found" diagnostics → error tracking / Journal
- `recent_operational_details`, project artifacts, test-file output, or app-generation state → Journal / Episodic Memory
- "Check X later" / reminders → `manage_notes` with `due_date` + `cron_scheduler`
- URLs or references for later → `manage_notes` (category: `bookmark`)
- Learnings from this session → `manage_journal` (entry_type: `learning`)
- Anything that won't matter in 6 months → `manage_notes` or `manage_journal`

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `add`, `update`, `delete`, `remove`, or `list` |
| `fact` | string | for `add`/`remove`/`update` | The fact to add, update, or exact text to remove |
| `id` | string | for `update`/`delete` | Numeric ID from a recent `list` result |

## Examples

```json
{"action": "manage_memory", "operation": "add", "fact": "User prefers concise answers"}
```

```json
{"action": "manage_memory", "operation": "remove", "fact": "Old fact to delete"}
```

## Notes

- **Token cost**: Every fact in core memory is included in every LLM request. Keep facts concise.
- **Write gate**: The backend rejects transient operational entries even if the agent explicitly asks for Core Memory.
- **Agent-only writes**: System jobs and dashboard APIs must not add or update Core Memory. Deletion/cleanup is administrative; new facts must come from the agent.
- **Permanence**: Facts here persist until explicitly removed. Only add information that is truly permanent.
- **Removal**: Use `remove` with the **exact** fact text to delete it.
- **ID deletion**: Use `delete` only with one numeric ID from a recent `list` result. Never bulk-delete guessed entries, and stop after any warning or error.
- **Privacy**: Core memory is stored locally in `data/core_memory.md` — it does not leave the server.
- **Conflict resolution**: If user preferences change, add the new fact and remove the old one explicitly.

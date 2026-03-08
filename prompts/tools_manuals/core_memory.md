## Tool: Core Memory (`manage_memory`)

Add or remove critical, permanent facts from `core_memory.md`. This file is injected into your system prompt every turn.

**Threshold:** Only store identity-defining traits, the user's permanent preferences, or key relationships. Do NOT store temporary states, tasks, or transient facts.

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
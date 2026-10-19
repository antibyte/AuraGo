# context_manager — Conversation Context Management

Inspect or compact the current conversation context. Operations require an available history manager and budget tracker. This tool does not pin facts.

## Operations

| Operation | Description | Parameters |
| --- | --- | --- |
| `status` | Read the message count, total characters, and budget status | `operation` |
| `compact` | Safely summarize older messages through coordinated persistent compression | `operation` |
| `drop` | Remove one message selected by its current zero-based history index | `operation`, `index` |

Use `status` before deciding whether compression is necessary. `compact` requires the memory store and LLM client; short histories or unsafe compression may be rejected. A failed compaction leaves the source history intact. Never drop messages speculatively or use this tool to evade instructions, approvals, or tool-result continuity.

## Examples

```json
{"action":"context_manager","operation":"status"}
```

```json
{"action":"context_manager","operation":"compact"}
```

```json
{"action":"context_manager","operation":"drop","index":2}
```

For durable facts use `remember` or `manage_memory`; context management has no `pin`, `unpin`, `list_pins`, `summarize`, or `clear_history` operation.

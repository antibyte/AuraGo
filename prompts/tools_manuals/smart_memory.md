# Smart Memory (`smart_memory`) — DEPRECATED

> **This tool does not exist.** Do NOT call `smart_memory` — it will fail with an unknown tool error.
>
> The functionality is now covered by two existing tools:
> - **Writing** → use `remember` (auto-routes to core memory, journal, notes, or knowledge graph)
> - **Reading** → use `query_memory` (searches all memory layers at once)
>
> See `remember.md` and `query_memory.md` for the current implementation.

*Historical note: smart_memory was a planning artifact. The `remember` tool implements the store/auto-route functionality. The `query_memory` tool implements the unified search functionality.*

## When to Use

- **Auto-Extraction**: After important user statements automatically
- **Smart Store**: When unsure WHERE something should be stored
- **Smart Query**: When you need information from ALL memory layers
- **Consolidation**: At the end of a session for summary

## Former Operations

| Operation | Description | Auto-Confirm |
|-----------|-------------|--------------|
| `auto_extract` | Analyzes text and suggests storage locations | When Confidence >95% |
| `store` | Direct storage with smart routing | No |
| `query` | Intelligent search across all layers | Yes |
| `consolidate` | Analyzes session and suggests actions | Configurable |
| `suggest` | Retrieves proactive suggestions | Yes |

## Notes

- This tool is deprecated and should not be called
- Use `remember` for storing information with automatic routing
- Use `query_memory` for unified search across all memory layers

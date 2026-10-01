## Tool: Recall Memory (`recall_memory`)

Read exact long-term memory entries by ID from the `AVAILABLE CONTEXT INDEX`.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `ids` | array | yes | Memory IDs from entries like `[memory:mem-123]` |

### Example

```json
{"action": "recall_memory", "ids": ["mem-123"]}
```

### Guidance

- Use this only when a listed memory teaser is relevant and you need the full entry.
- Do not guess IDs. Copy them from the current `AVAILABLE CONTEXT INDEX`.
- Archived entries are unavailable through agent recall. Administrative recovery requires an explicit memory curation action; a retained vector does not grant access to an archived entry.
- Recall checks current metadata. If that store is unavailable, retry after recovery instead of treating the entry as active.
- Results are advisory and may be stale. Current files, tools, and checks still win.

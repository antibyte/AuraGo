## Tool: Explore Knowledge Graph (`explore_kg`)

Expand exact Knowledge Graph nodes by ID from the `AVAILABLE CONTEXT INDEX`.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `ids` | array | yes | Node IDs from entries like `[kg:node-id]` |
| `depth` | integer | no | Traversal depth from 1 to 3. Default: 1 |
| `limit` | integer | no | Maximum nodes and edges per requested ID. Default: 20 |

### Example

```json
{"action": "explore_kg", "ids": ["backup_server"], "depth": 1, "limit": 10}
```

### Guidance

- Use this only when a listed KG teaser is relevant and connected context is needed.
- Do not guess IDs. Copy them from the current `AVAILABLE CONTEXT INDEX`.
- This is read-only; use `knowledge_graph` for broader search or graph maintenance.

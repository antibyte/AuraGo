## Tool: Long-Term Memory Query (`query_memory`)

Search the VectorDB for semantic matches. Use this to recall past facts, design choices, or search through the **documentation** and **technical manuals** (e.g. for configuration or setup help).

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `content` | string | yes | Natural-language query describing what you need to recall |

### Example

```json
{"action": "query_memory", "content": "What was the user's preferred database schema?"}
```
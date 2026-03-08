## Tool: Knowledge Graph (`knowledge_graph`)

Manage a structured JSON graph of entities and relations. **DO NOT use this tool for technical documentation or config help** — use `query_memory` (RAG) for that. This tool is for tracking the user's social/professional context.

### Operations

| Operation | Required Parameters | Description |
|---|---|---|
| `add_node` | `node_id`, `label` | Create a node. Optional: `properties` object |
| `add_edge` | `source`, `target`, `relation` | Create a directed edge |
| `delete_node` | `node_id` | Remove a node and its edges |
| `delete_edge` | `source`, `target` | Remove an edge |
| `search` | `content` | Semantic search across nodes |

### Behavior

- Nodes and edges automatically track an `access_count` on search.
- Set `"protected": "true"` in a node's `properties` to exempt it from the automated Priority-Based Forgetting sweep.

### Examples

```json
{"action": "knowledge_graph", "operation": "add_node", "node_id": "app_db", "label": "Database", "properties": {"type": "PostgreSQL", "protected": "true"}}
```

```json
{"action": "knowledge_graph", "operation": "add_edge", "source": "api_server", "target": "app_db", "relation": "reads_from"}
```

```json
{"action": "knowledge_graph", "operation": "delete_node", "node_id": "legacy_db"}
```

```json
{"action": "knowledge_graph", "operation": "search", "content": "PostgreSQL"}
```
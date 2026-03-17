## Tool: Knowledge Graph (`knowledge_graph`)

Manage a structured graph of entities and relations stored in SQLite with full-text search. **DO NOT use this tool for technical documentation or config help** — use `query_memory` (RAG) for that. This tool is for tracking the user's social/professional context, devices, services, and relationships.

### Operations

| Operation | Required Parameters | Description |
|---|---|---|
| `add_node` | `id`, `label` | Create or update a node. Optional: `properties` object |
| `add_edge` | `source`, `target`, `relation` | Create a directed edge (auto-creates missing nodes). Optional: `properties` |
| `delete_node` | `id` | Remove a node and all its connected edges |
| `delete_edge` | `source`, `target`, `relation` | Remove a specific edge |
| `search` | `content` | Full-text search across nodes and edges |

### Parameter Reference

| Parameter | Type | Used by | Description |
|---|---|---|---|
| `id` | string | add_node, delete_node | Unique node identifier (e.g. `app_db`, `server_prod`) |
| `label` | string | add_node | Human-readable name for the node |
| `source` | string | add_edge, delete_edge | Source node ID |
| `target` | string | add_edge, delete_edge | Target node ID |
| `relation` | string | add_edge, delete_edge | Relationship type (e.g. `owns`, `uses`, `manages`) |
| `content` | string | search | Search query text |
| `properties` | object | add_node, add_edge | Optional metadata (e.g. `{"type": "PostgreSQL"}`) |

### Behavior

- Search uses FTS5 full-text search with LIKE fallback for broad matching.
- Relevant knowledge graph entities are automatically injected into the system prompt when `prompt_injection` is enabled.
- Nightly batch extraction automatically discovers entities and relationships from conversations.
- Nodes track `access_count` on each search hit.
- Set `"protected": "true"` in a node's `properties` to exempt it from the automated Priority-Based Forgetting sweep.

### Examples

```json
{"action": "knowledge_graph", "operation": "add_node", "id": "app_db", "label": "Database", "properties": {"type": "PostgreSQL", "protected": "true"}}
```

```json
{"action": "knowledge_graph", "operation": "add_edge", "source": "api_server", "target": "app_db", "relation": "reads_from"}
```

```json
{"action": "knowledge_graph", "operation": "search", "content": "PostgreSQL"}
```
# Knowledge Graph (`knowledge_graph`)

Manage a structured graph of entities and relations stored in SQLite with full-text search. **DO NOT use this tool for technical documentation or config help** — use `query_memory` (RAG) for that. This tool is for tracking the user's social/professional context, devices, services, and relationships.

## Operations

| Operation | Required Parameters | Description |
|-----------|---------------------|-------------|
| `add_node` | `id`, `label` | Create or update a node. Optional: `properties` object |
| `add_edge` | `source`, `target`, `relation` | Create a directed edge (auto-creates missing nodes). Optional: `properties` |
| `update_node` | `id` | Update node label or properties |
| `update_edge` | `source`, `target`, `relation` | Update edge properties or relation type |
| `delete_node` | `id` | Remove a node and all its connected edges |
| `delete_edge` | `source`, `target`, `relation` | Remove a specific edge |
| `get_node` | `id` | Get a single node's details |
| `get_neighbors` | `id` | Get all nodes connected to this node |
| `subgraph` | `id`, `depth` | Get subgraph starting from a node |
| `search` | `content` | Full-text search across nodes and edges |
| `explore` | `content` | Search with relationship context |
| `suggest_relations` | `id` | Suggest possible relations for a node |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | for most ops | Unique node identifier (e.g. `app_db`, `server_prod`) |
| `label` | string | for add_node, update_node | Human-readable name for the node |
| `source` | string | for add_edge, delete_edge, update_edge | Source node ID |
| `target` | string | for add_edge, delete_edge, update_edge | Target node ID |
| `relation` | string | for add_edge, delete_edge, update_edge | Relationship type (e.g. `owns`, `uses`, `manages`, `connected_to`) |
| `content` | string | for search, explore | Search query text |
| `properties` | object | for add_node, add_edge | Optional metadata (e.g. `{"type": "PostgreSQL"}`) |
| `new_relation` | string | for update_edge | New relation type to replace current |
| `depth` | integer | for subgraph | Traversal depth (1-3, default: 2) |
| `limit` | integer | for get_neighbors | Max results (default: 20) |

## Examples

**Add a node:**
```json
{"action": "knowledge_graph", "operation": "add_node", "id": "app_db", "label": "Database", "properties": {"type": "PostgreSQL", "protected": "true"}}
```

**Add an edge:**
```json
{"action": "knowledge_graph", "operation": "add_edge", "source": "api_server", "target": "app_db", "relation": "reads_from"}
```

**Search:**
```json
{"action": "knowledge_graph", "operation": "search", "content": "PostgreSQL"}
```

**Get neighbors:**
```json
{"action": "knowledge_graph", "operation": "get_neighbors", "id": "api_server", "limit": 10}
```

## Behavior

- Search uses FTS5 full-text search with LIKE fallback for broad matching.
- Relevant knowledge graph entities are automatically injected into the system prompt when `prompt_injection` is enabled.
- Nightly batch extraction automatically discovers entities and relationships from conversations.
- Nodes track `access_count` on each search hit.
- Set `"protected": "true"` in a node's `properties` to exempt it from automated Priority-Based Forgetting.

## Notes

- **When to use**: Track people, devices, services, and their relationships — not documentation or config.
- **Relationship types**: Use meaningful verbs: `owns`, `uses`, `manages`, `connects_to`, `depends_on`, `hosts`.
- **Protected nodes**: Mark sensitive nodes with `"protected": "true"` in properties to prevent deletion by automated cleanup.
- **Search vs Explore**: `search` finds nodes by text match; `explore` returns context including relationship paths.
- **Deletion cascades**: `delete_node` removes all connected edges automatically.
- **Subgraph**: Use `depth=1` for direct neighbors, `depth=2` for friends-of-friends, etc.
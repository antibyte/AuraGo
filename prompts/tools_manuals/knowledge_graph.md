# Knowledge Graph (`knowledge_graph`)

Manage a structured graph of entities and relations stored in SQLite with full-text search. **DO NOT use this tool for technical documentation or config help** — use `query_memory` (RAG) for that. This tool is for tracking the user's social/professional context, devices, services, and relationships.

## Operations

| Operation | Required Parameters | Description |
|-----------|---------------------|-------------|
| `add_node` | `id`, `label` | Create or update a node. Optional: `properties` object |
| `add_edge` | `source`, `target`, `relation` | Create a directed edge (auto-creates missing nodes). Optional: `properties` |
| `update_node` | `id` | Update node label or properties |
| `update_edge` | `source`, `target`, `relation` | Update edge properties or relation type |
| `merge_nodes` | `target`, `source` | Merge `source` into `target`, move edges/properties, delete `source` |
| `delete_node` | `id` | Remove a node and all its connected edges |
| `delete_edge` | `source`, `target`, `relation` | Remove a specific edge |
| `get_node` | `id` | Get a single node's details |
| `get_neighbors` | `id` | Get all nodes connected to this node |
| `subgraph` | `id`, `depth` | Get subgraph starting from a node |
| `search` | `content` | Full-text search across nodes and edges |
| `graph_health` | none | Read KG stats and quality signals, including pending and low-confidence edge counts |
| `explore` | `content` | Search with relationship context |
| `suggest_relations` | `id` | Suggest possible relations for a node |
| `suggest_inferred_relations` | optional `limit` | Read-only deterministic suggestions from inverse/transitive relationship rules |
| `explain_edge` | `source`, `target`, `relation` | Show claims and evidence for a relationship. Set `include_inactive=true` for history |
| `list_conflicts` | optional `limit` | List open contradictory claims that need resolution |
| `resolve_conflict` | `conflict_id`, `claim_id` | Resolve an open conflict by selecting the winning claim |
| `supersede_edge` | `source`, `target`, `relation` | Mark an edge outdated. Optional: `claim_id`, `reason` |
| `retract_edge` | `source`, `target`, `relation` | Mark an edge withdrawn while keeping claim history. Optional: `reason` |
| `export_jsonld` | optional `limit` | Export nodes, relationships, and claims as JSON-LD. Set `include_inactive=true` for inactive edges/claims |
| `optimize` / `optimize_graph` | optional thresholds | Run priority-based KG cleanup through the memory optimizer |

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
| `include_low_confidence` | boolean | no | Include pending or low-confidence `co_mentioned_with` edges in `search` and `get_neighbors` results. Default: `false` |
| `include_inactive` | boolean | no | Include superseded, retracted, or rejected claims/edges in `explain_edge` and `export_jsonld`. Default: `false` |
| `claim_id` | string | for resolve_conflict | Winning claim ID for `resolve_conflict`; optional superseding claim for `supersede_edge` |
| `conflict_id` | integer | for resolve_conflict | Open conflict ID from `list_conflicts` |
| `reason` | string | for curation ops | Short human-readable reason for resolve/supersede/retract |

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

**Search including low-confidence co-mentions:**
```json
{"action": "knowledge_graph", "operation": "search", "content": "andi", "include_low_confidence": true}
```

**Read graph health:**
```json
{"action": "knowledge_graph", "operation": "graph_health"}
```

**Explain an edge with claim history:**
```json
{"action": "knowledge_graph", "operation": "explain_edge", "source": "andi", "target": "german", "relation": "primary_language", "include_inactive": true}
```

**Resolve a claim conflict:**
```json
{"action": "knowledge_graph", "operation": "resolve_conflict", "conflict_id": 12, "claim_id": "kg_claim_abc", "reason": "newer user correction wins"}
```

**Suggest inferred relations without writing:**
```json
{"action": "knowledge_graph", "operation": "suggest_inferred_relations", "limit": 20}
```

**Export JSON-LD:**
```json
{"action": "knowledge_graph", "operation": "export_jsonld", "include_inactive": true, "limit": 200}
```

**Get neighbors:**
```json
{"action": "knowledge_graph", "operation": "get_neighbors", "id": "api_server", "limit": 10}
```

**Merge duplicate nodes:**
```json
{"action": "knowledge_graph", "operation": "merge_nodes", "target": "nas_primary", "source": "nas_secondary"}
```

## Behavior

- Search uses FTS5 full-text search with quoted tokens and AND semantics for multi-word queries, plus LIKE fallback for broad matching.
- `search` and `get_neighbors` hide low-confidence `co_mentioned_with` edges by default. Set `include_low_confidence=true` only when auditing pending co-mentions.
- `graph_health` is read-only and returns `stats` plus `quality`, including edge source breakdowns, pending co-mentions, generic-node samples, and duplicate suggestions.
- Relationship writes now create claims with optional evidence. `explain_edge` is the safest way to audit why an edge exists.
- `resolve_conflict`, `supersede_edge`, and `retract_edge` preserve history instead of silently erasing the claim trail.
- `suggest_inferred_relations` and `export_jsonld` are read-only and do not mutate the graph.
- `explore` and prompt-context search can use semantic similarity when embeddings are enabled; failed semantic upserts mark rows dirty for nightly reindex.
- Relevant knowledge graph entities are automatically injected into the system prompt when `prompt_injection` is enabled.
- Nightly batch extraction automatically discovers entities and relationships from conversations.
- Nodes track `access_count` on each search hit; maintenance flushes queued access hits before optimize/cleanup so recent usage is not lost.
- Set `"protected": "true"` in a node's `properties` to exempt it from automated Priority-Based Forgetting.
- Synced nodes from planner, inventory, core memory, file sync, and manual curation are protected from auto-optimize pruning by default (`protect_optimize_sources` / `protect_id_prefixes` in config).
- `optimize` (`optimize_graph` is accepted as a compatibility alias) runs in a single SQLite transaction and only deduplicates edges touched by the current merge/delete scope.
- `merge_nodes` keeps the target node, merges labels/properties with the longer readable label winning, moves incident edges, deduplicates only within the merged node pair, and deletes the source.
- Planner sync creates a synthetic hub node `planner_workspace` (`type: planner_hub`). Todos without checklist items link to it via `part_of`; checklist items link to their parent todo with `part_of`. Stale planner edges are pruned in batched deletes.
- File-to-KG sync processes KG writes serially to avoid SQLite write contention.
- When `add_edge` or bulk sync references a missing endpoint, AuraGo creates a temporary `Unknown` placeholder node (`source: auto_placeholder`, `type: unknown`). Isolated placeholders older than 7 days are removed by nightly cleanup.
- Prompt-context output omits sensitive node properties such as `password`, `token`, and `api_key`, and applies secret scrubbing before injection.

## Notes

- **When to use**: Track people, devices, services, and their relationships — not documentation or config.
- **Relationship types**: Use meaningful verbs: `owns`, `uses`, `manages`, `connects_to`, `depends_on`, `hosts`.
- **Protected nodes**: Mark sensitive nodes with `"protected": "true"` in properties to prevent deletion by automated cleanup.
- **Search vs Explore**: `search` finds nodes by text match; `explore` returns context including relationship paths.
- **Deletion cascades**: `delete_node` removes all connected edges automatically.
- **Merge nodes**: `merge_nodes` keeps the target node, merges properties/labels, moves edges, and deletes the source. Protected source nodes cannot be merged.
- **Subgraph**: Use `depth=1` for direct neighbors, `depth=2` for friends-of-friends, etc.

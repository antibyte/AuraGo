## Tool: Optimize Memory (`optimize_memory`)

Triggers the Priority-Based Forgetting System on the Knowledge Graph. Sweeps the entire graph, calculates a composite priority score for every node, and removes low-priority entries to keep the KG lean and relevant.

### How it works

Each node receives a **composite priority score**:
- `access_count` — how often this node has been retrieved or referenced
- `connected edges` — how many relationships this node participates in

Nodes below the configured threshold are **archived** (not deleted permanently) to `graph_archive.json`. Their relationships are also archived.

**Protected nodes** (those with `properties["protected"] == "true"`) are never removed.

### When to run

- The Knowledge Graph has grown large (hundreds of nodes) and searches feel slow or noisy
- After a major project completes and many temporary nodes are no longer relevant
- As part of periodic memory maintenance (monthly)
- When `memory_reflect` shows many stale or low-relevance entities

### When NOT to run

- The graph is small (under ~50 nodes) — no benefit
- You're actively working on a project where all nodes may be relevant soon

### Schema

```json
{"action": "optimize_memory"}
```

No parameters — the tool uses the configured threshold from `config.yaml`.

### Protecting important nodes

Before running optimization, mark nodes you want to keep permanently:

```json
{"action": "knowledge_graph", "operation": "update", "node_id": "my-server", "properties": {"protected": "true"}}
```

Protected nodes are skipped regardless of their access count or edge count.

### After optimization

- Archived nodes are stored in `data/graph_archive.json`
- Active graph is smaller and more relevant
- Subsequent searches and KG context injections will be faster and less noisy
- Run `memory_reflect` with `focus: "relationships"` afterwards to confirm the results
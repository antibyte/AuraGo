## Tool: Optimize Memory (`optimize_memory`)

Triggers the Priority-Based Forgetting System for long-term memory and the Knowledge Graph. It scores tracked VectorDB memories via `memory_meta`, archives low-priority memories through the Memory Curator event log, compresses medium-priority memories when possible, and removes low-priority KG entries to keep retrieval lean and relevant.

### How it works

Each node receives a **composite priority score**:
- `access_count` — how often this node has been retrieved or referenced
- `connected edges` — how many relationships this node participates in

Vector memories below the configured threshold are **archived** in `memory_meta` with a curation event so they stop being injected into prompts but remain reviewable in the dashboard. KG nodes below the threshold are archived to `graph_archive.json`; their relationships are also archived.

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

- Archived VectorDB memories appear in the Dashboard Memory Curator and are excluded from active prompt retrieval
- Archived KG nodes are stored in `data/graph_archive.json`
- Active graph is smaller and more relevant
- Subsequent searches and KG context injections will be faster and less noisy
- Run `memory_reflect` with `focus: "relationships"` afterwards to confirm the results

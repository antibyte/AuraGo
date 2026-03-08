## Tool: Optimize Memory (`optimize_memory`)

Trigger the Priority-Based Forgetting System on the Knowledge Graph.

Sweeps the entire graph, calculating a composite priority score for every node based on `access_count` and number of connected edges. Nodes below the threshold (and their connected edges) are removed from the active graph and archived to `graph_archive.json`.

Nodes with `properties["protected"] == "true"` are exempt from removal.

### Schema

```json
{"action": "optimize_memory"}
```
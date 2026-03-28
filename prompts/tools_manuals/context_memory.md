## Tool: Context Memory (`context_memory`)

Context-aware memory search across all storage layers. Combines Long-Term Memory (VectorDB), Knowledge Graph, Journal, and Notes into a single intelligent query.

Use this when `query_memory` is not enough — specifically when you need **relationships, connections, or time-scoped** results rather than isolated facts.

### When to use vs. `query_memory`

| Use `query_memory` when... | Use `context_memory` when... |
|---|---|
| You need a quick fact lookup | You need connected context, not just a fact |
| You want results from all sources at once | You want to restrict to specific layers or time ranges |
| Default go-to for most recall tasks | `query_memory` returned too few or irrelevant results |
| You need error patterns | You need relationships between entities (KG traversal) |
| — | You need to understand what happened in a specific period |

### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `query` | string | required | Natural-language search query |
| `context_depth` | string | `"normal"` | `shallow` / `normal` / `deep` |
| `sources` | array | `["activity", "journal", "notes", "core", "kg", "ltm"]` | Which layers to search |
| `time_range` | string | `"all"` | `all` / `today` / `last_week` / `last_month` |
| `include_related` | boolean | `true` | Expand to connected KG neighbours |

### Context Depth

- **shallow**: Direct matches only — fast and precise. Use for: quick fact checks.
- **normal**: Matches + direct KG neighbours — balanced. Use for: standard research.
- **deep**: Full graph expansion + temporal search — thorough. Use for: troubleshooting, complex relationships.

### Sources

| Source | Contains | Best for |
|--------|----------|----------|
| `ltm` | VectorDB documents | Facts, setups, past learnings |
| `activity` | Activity turns + daily rollups | Recent work overview, user intent, what happened over the last days |
| `kg` | Knowledge Graph | Relationships, entities, topology |
| `journal` | Journal entries | Milestones, reflections, events |
| `notes` | Notes / to-dos | Current tasks, bookmarks |
| `core` | Core Memory | Preferences, identity facts |

### Examples

#### Search for project context

```json
{"action": "context_memory", "query": "AuraGo project", "context_depth": "deep", "sources": ["ltm", "kg", "journal"]}
```

**Result:**
```
📚 LTM (3 hits):
   • "AuraGo Docker Setup" [Score: 0.94]
   • "AuraGo project structure" [Score: 0.87]
   • "AuraGo GitHub integration" [Score: 0.82]

🔗 Knowledge Graph:
   User ──works_on──► AuraGo ──uses──► Docker
                          │
                          ├──uses──► SQLite
                          │
                          └──uses──► Go 1.26

📔 Journal:
   • [15.03] Milestone: "AuraGo initial setup completed"
   • [14.03] Task: "Created AuraGo Docker Compose file"
```

#### Time-scoped search

```json
{"action": "context_memory", "query": "Docker errors", "time_range": "last_week", "sources": ["journal", "ltm"]}
```

#### Quick fact check

```json
{"action": "context_memory", "query": "server IP", "context_depth": "shallow", "sources": ["core", "kg"]}
```

### Combined Results

The tool returns a **combined ranked result** across all queried sources:

```json
{
  "status": "success",
  "combined_results": [
    {
      "rank": 1,
      "source": "kg",
      "type": "entity_network",
      "relevance": 0.96,
      "content": "User → works_on → AuraGo → runs_on → Proxmox",
      "reasoning": "Direct connection to user"
    },
    {
      "rank": 2,
      "source": "ltm",
      "type": "document",
      "relevance": 0.91,
      "content": "AuraGo Proxmox Deployment Guide...",
      "doc_id": "mem_12345"
    },
    {
      "rank": 3,
      "source": "journal",
      "type": "milestone",
      "relevance": 0.88,
      "content": "AuraGo successfully deployed on Proxmox",
      "date": "2026-03-10"
    }
  ]
}
```

### Best Practices

1. **Choose depth by complexity**
   - Simple fact lookup → `shallow`
   - Standard research → `normal`
   - Troubleshooting or complex topic → `deep`

2. **Restrict sources for focus**
   - Technical questions → `["ltm", "notes"]`
   - Timeline / organisational → `["activity", "journal", "notes"]`
   - Full picture → `["activity", "ltm", "kg", "journal", "notes", "core"]`

3. **Use time range for recency**
   - "What did we do yesterday?" → `"last_week"`
   - Open to-dos right now → `"today"`
   - Historical reference → `"all"`

4. **Related entities for context**
   - `include_related: true` finds connected nodes automatically
   - e.g. searching "Docker" also surfaces "AuraGo" if linked in KG

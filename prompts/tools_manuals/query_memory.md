## Tool: Unified Memory Query (`query_memory`)

Search **ALL** memory subsystems at once with a single natural-language query. This is your primary tool for recalling anything — facts, events, preferences, errors, relationships.

### What it searches (by default: everything)

| Source | What's in it | When useful |
|---|---|---|
| `vector_db` | Long-term semantic facts extracted from past conversations | Recall design decisions, config details, past discussions |
| `knowledge_graph` | Entities (people, devices, services) and relationships | "Who owns the NAS?", "What services run on prod?" |
| `journal` | Events, milestones, learnings (manual + auto-generated) | "What happened last Tuesday?", "When did we set up Docker?" |
| `notes` | Tasks, to-dos, bookmarks, reminders | "Any open tasks?", "What was that URL?" |
| `core_memory` | Permanent user facts (name, preferences, constraints) | "What language does the user prefer?" |
| `error_patterns` | Tool errors and learned resolutions | "Has this SSH error happened before?" |

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Natural-language query describing what you need to recall |
| `sources` | array | no | Limit to specific sources (default: searches all). Options: `vector_db`, `knowledge_graph`, `journal`, `notes`, `core_memory`, `error_patterns` |
| `limit` | integer | no | Max results per source (default 5) |

### Examples

**Search everything (recommended default):**
```json
{"action": "query_memory", "query": "user's preferred database setup"}
```

**Search only specific sources:**
```json
{"action": "query_memory", "query": "Docker container errors", "sources": ["error_patterns", "journal"]}
```

**Check for past errors:**
```json
{"action": "query_memory", "query": "SSH connection refused", "sources": ["error_patterns"]}
```

### Tips
- **Start broad** — don't restrict sources unless you're getting too many irrelevant results
- Notes and journal entries are included automatically — no need to search them separately
- Error patterns track tool failures and known resolutions; always check here before retrying a failed operation

### When to upgrade to `context_memory`

`query_memory` covers the vast majority of recall tasks. Use `context_memory` instead when:

| Situation | Better tool |
|-----------|-------------|
| Quick fact lookup, most recall tasks | `query_memory` ✅ |
| Results were too few or off-topic | `context_memory` with `context_depth: deep` |
| You need KG graph traversal (connected entities) | `context_memory` with `include_related: true` |
| You need to scope to a specific time window | `context_memory` with `time_range` |
| You need relationships, not just isolated facts | `context_memory` |

### Background memory operations

The system automatically enriches each turn with memory before you even issue a recall:

- **CORE MEMORY** — permanent user facts always injected into every prompt
- **RETRIEVED MEMORIES** — top-3 long-term memories most semantically similar to the current message (auto-RAG)
- **PREDICTED CONTEXT** — memories pre-fetched based on recent tool usage patterns (full tier only)
- **RELEVANT KNOWLEDGE** — KG entities related to the current message, injected via `SearchForContext`
- **ACTIVE REMINDERS** — high-priority open notes always shown

This means: in many cases you already have the relevant context **without calling any tool**. Only call `query_memory` when the injected context is incomplete or you need something specific.
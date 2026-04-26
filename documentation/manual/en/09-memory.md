# Chapter 9: Memory & Knowledge

> ⚠️ **Note:** This documentation describes the current implementation of the AuraGo memory system. Some features may evolve as the system is further developed.

AuraGo's memory system is what transforms it from a simple chatbot into a truly personal assistant. This chapter explores how AuraGo remembers, organizes, and retrieves information across conversations.

## Memory Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Memory Layers                        │
├─────────────────────────────────────────────────────────┤
│  Core Memory       │ Permanent facts, user preferences   │
├───────────────────┬┴─────────────────────────────────────┤
│  Short-Term       │ Recent conversation history         │
│  Memory (STM)     │ SQLite-based, context window        │
├───────────────────┼─────────────────────────────────────┤
│  Long-Term        │ Vector embeddings, semantic search  │
│  Memory (LTM)     │ Via configured embeddings provider  │
├───────────────────┼─────────────────────────────────────┤
│  Knowledge Graph  │ Entities, relationships, facts      │
│                   │ Structured network of knowledge     │
├───────────────────┼─────────────────────────────────────┤
│  Notes & To-Dos   │ Organized, categorized, prioritized │
│                   │ Actionable items with due dates     │
└─────────────────────────────────────────────────────────┘
```

## Short-Term Memory (STM)

Short-term memory stores the immediate conversation context, allowing AuraGo to understand references, follow topics, and maintain coherent dialogue.

### How STM Works

- **Storage**: SQLite database (configured via `sqlite.short_term_path`, default: `./data/short_term.db`)
- **Retention**: Last N messages (configurable, default: 20)
- **Structure**: Message history with timestamps, roles, and metadata

### STM in Action

```
User: "What's the weather like?"
[Agent checks location from STM context]

User: "And tomorrow?"
[Agent understands "weather" + "tomorrow" without repetition]
```

## Long-Term Memory (LTM) / RAG

Long-term memory enables AuraGo to recall information from past conversations, documents, and learned facts using semantic search.

### How RAG Works

```
User Query → Embedding Model → Vector Search → Top-K Results → Context Injection → LLM Response
```

1. **Storage**: Text is converted to vector embeddings (when embeddings are enabled)
2. **Retrieval**: Cosine similarity search finds relevant chunks
3. **Context**: Top results are injected into the system prompt
4. **Response**: LLM answers using retrieved context

### Memory Storage Triggers

Content is automatically stored in LTM when:

| Trigger | Example |
|---------|---------|
| Explicit save | "Remember that I prefer dark mode" |
| Important facts | User mentions birthday, preferences |
| Document upload | PDFs, text files analyzed |
| Web pages fetched | "Save this article for later" |
| Tool outputs | System information, search results |

### Semantic Search Examples

```
User: "What did we discuss about my project last week?"
[LTM Search: "project" + temporal context]
→ Retrieved: "User is building a home automation system 
              using ESP32 and Home Assistant"

User: "Remind me about that Italian restaurant"
[LTM Search: "Italian" + "restaurant" despite no exact keyword match]
→ Retrieved: "Mario's Trattoria - recommended by colleague, 
              downtown location, good pasta"
```

## Knowledge Graph

The Knowledge Graph stores structured entities and their relationships, enabling complex queries and inference.

### Entity Types

| Type | Examples | Properties |
|------|----------|------------|
| **Person** | Names, contacts | email, phone, relationship |
| **Location** | Places, addresses | coordinates, type |
| **Organization** | Companies, teams | industry, role |
| **Event** | Meetings, deadlines | date, time, participants |
| **Concept** | Topics, ideas | category, related terms |
| **Item** | Objects, products | attributes, status |

### Relationship Types

```
Person --works_at--> Organization
Person --knows--> Person
Event --located_at--> Location
Concept --related_to--> Concept
Item --belongs_to--> Person
```

### Knowledge Graph Operations

**Adding entities:**
```
User: "My colleague Sarah works at Google"
[Extracted: Person(Sarah), Organization(Google), Relationship(works_at)]
```

**Querying relationships:**
```
User: "Who do I know at Google?"
[Query: Person --works_at--> Organization(Google)]
→ "You know Sarah who works at Google."
```

**Inference:**
```
Stored: "Sarah works at Google" + "Google is in Mountain View"
Query: "Where does Sarah work?"
→ "Sarah works at Google in Mountain View."
```

### Knowledge Graph Quality and Protection

The current Knowledge Graph implementation includes maintenance features that help keep the graph useful over time:

| Feature | Purpose |
|---------|---------|
| **Quality reports** | Find isolated nodes, untyped nodes, low-confidence data, and possible duplicates |
| **Protected nodes** | Mark critical entities so cleanup routines do not remove them accidentally |
| **Access tracking** | Track recently used nodes to improve retrieval relevance |
| **Semantic indexing** | Improve graph search through embedding-backed retrieval where enabled |

The Web UI and API expose graph search, stats, quality reports, important nodes, and node protection through `/api/knowledge-graph/*` endpoints.

## Core Memory

Core Memory contains permanent facts that are always included in the system prompt, ensuring critical information is never forgotten.

### What Belongs in Core Memory

| Category | Examples |
|----------|----------|
| **Identity** | User's name, profession, preferences |
| **Preferences** | Communication style, notification settings |
| **Important dates** | Birthdays, anniversaries, deadlines |
| **Key facts** | Allergies, important relationships |
| **Settings** | Preferred units (metric/imperial), timezone |

### Core Memory Format

```yaml
core_memory:
  user_name: "Alex"
  profession: "Software Engineer"
  preferred_language: "English"
  timezone: "Europe/Berlin"
  communication_style: "concise"
  important_dates:
    birthday: "1990-05-15"
    project_deadline: "2024-12-31"
  preferences:
    theme: "dark"
    notifications: "email"
```

### Managing Core Memory

**Add to core memory:**
```
User: "Add to core memory: I'm allergic to peanuts"
Agent: ✅ Added to core memory: "User is allergic to peanuts"
```

**View core memory:**
```
User: "What do you know about me?"
Agent: 📋 Core Memory:
   - Name: Alex
   - Allergic to peanuts
   - Prefers dark theme
   - Software Engineer
```

> ⚠️ **Warning:** Core Memory is always included in prompts. Keep it concise (under 500 tokens recommended) to avoid excessive API costs.

## Notes & To-Dos

A structured system for actionable information with categories, priorities, and due dates.

### Notes

**Creating notes:**
```
User: "Save as note: Server IP is 192.168.1.100"
User: "Note: Meeting notes from today - discussed Q4 goals"
```

**Note categories:**
- Work
- Personal
- Ideas
- Reference
- Learning

**Searching notes:**
```
User: "Find my notes about the server"
→ Shows all notes containing "server"
```

### To-Dos

**Creating to-dos:**
```
User: "Add todo: Buy groceries tomorrow"
User: "Task: Submit report by Friday, high priority"
```

**To-do properties:**

| Property | Options |
|----------|---------|
| Priority | Low, Normal, High, Critical |
| Status | Pending, In Progress, Done, Cancelled |
| Due Date | Specific date, relative time |
| Category | Work, Personal, Health, Finance, etc. |
| Recurring | Daily, Weekly, Monthly |

**Managing to-dos:**
```
User: "Show my todos"
User: "Mark grocery shopping as done"
User: "What tasks are due this week?"
```

## File KG Sync

File KG Sync is a background service that connects file indexing with the Knowledge Graph. When the file indexer discovers or updates supported files, the sync service can extract entities, relationships, and source references from those indexed chunks and write them into the Knowledge Graph.

### What it does

| Step | Description |
|------|-------------|
| Index | The FileIndexer scans configured directories and stores searchable chunks |
| Extract | An LLM extracts candidate entities and relations from indexed content |
| Score | Extracted items receive confidence and source metadata |
| Merge | Existing nodes are reused where possible to avoid duplicates |
| Cleanup | Orphaned file-derived entities can be detected and cleaned up |

This makes uploaded or indexed documentation discoverable both through semantic RAG and structured graph queries. For example, a server runbook can create graph nodes for services, hosts, ports, owners, and dependencies.

### Debug and maintenance endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /api/debug/kg-file-sync-stats` | File-to-graph sync statistics |
| `GET /api/debug/kg-orphans` | File-derived graph nodes without current sources |
| `GET /api/debug/file-sync-status` | Current sync health/status |
| `GET /api/debug/file-sync-last-run` | Last sync run details |
| `GET /api/debug/kg-file-entities` | Entities created from indexed files |
| `GET /api/debug/kg-node-sources` | Source files/chunks behind graph nodes |
| `POST /api/debug/kg-file-sync-cleanup` | Cleanup orphaned file-derived graph data |

### Practical guidance

Keep indexed directories focused and avoid dumping huge unrelated trees into the index. File KG Sync works best for runbooks, inventories, project notes, architecture docs, configuration references, and other semi-structured knowledge.

## How Memory Works in Conversations

### The Memory Pipeline

```
┌──────────────────────────────────────────────────────────┐
│  1. User Input                                           │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│  2. Memory Retrieval                                     │
│     • STM: Recent context                                │
│     • LTM: Semantic search for related memories          │
│     • Core: Always included facts                        │
│     • Knowledge Graph: Relevant entities                 │
│     • Notes/To-dos: Matching items                       │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│  3. Context Assembly                                     │
│     System Prompt + Core + STM + Retrieved Memories      │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│  4. LLM Processing → Response                            │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│  5. Memory Storage                                       │
│     • STM: Store conversation turn                       │
│     • LTM: Important facts → Vector DB                   │
│     • Knowledge Graph: New entities/relations            │
│     • Notes/To-dos: If explicitly created                │
└──────────────────────────────────────────────────────────┘
```

### Memory Retrieval Priority

When responding, AuraGo retrieves memories in this priority order:

1. **Core Memory** - Always included
2. **STM Context** - Last N messages
3. **LTM Matches** - Top-K semantic results (default: 5)
4. **Knowledge Graph** - Entities mentioned in query
5. **Active To-Dos** - Pending items (if relevant)

## Saving and Retrieving Knowledge

### Explicit Memory Commands

| Command | Purpose | Example |
|---------|---------|---------|
| `remember [fact]` | Store in LTM | "Remember I like jazz music" |
| `what do you know about [topic]?` | LTM query | "What do you know about my project?" |
| `save as note: [text]` | Create note | "Save as note: API key is xyz" |
| `add todo: [task]` | Create to-do | "Add todo: Call mom tomorrow" |
| `add to core: [fact]` | Update core memory | "Add to core: I'm vegetarian" |
| `find notes about [topic]` | Search notes | "Find notes about docker" |

### Automatic Memory Extraction

AuraGo automatically extracts and stores:

```
User: "I'm flying to Paris next Tuesday for a conference"
↓ Extracted:
   • LTM: Travel plan to Paris
   • Knowledge Graph: Event(Conference), Location(Paris), Date(Next Tuesday)
   • No explicit command needed
```

### Memory Retrieval Patterns

**Temporal queries:**
```
"What did we discuss last week?"
"Remind me what I said about the budget"
"Show me conversations from March"
```

**Topic queries:**
```
"Tell me about my home automation project"
"What do I know about machine learning?"
"Find information about server configuration"
```

**Entity queries:**
```
"Who is Sarah?"
"What companies do I have contacts at?"
"What projects am I working on?"
```

## Memory Configuration

The actual configuration structure in `config.yaml`:

```yaml
# Embeddings configuration for LTM/RAG
embeddings:
  provider: "internal"              # Options: "disabled", "internal", or provider-id
  internal_model: "qwen/qwen3-embedding-8b"  # Model used when provider is "internal"
  external_url: "http://localhost:11434/v1"  # External embedding service URL
  external_model: "nomic-embed-text"         # Model for external provider
  api_key: "dummy_key"              # API key for external provider (if needed)

# Agent memory settings
agent:
  memory_compression_char_limit: 60000   # Trigger compression at this character limit
  core_memory_max_entries: 200           # Maximum entries in core memory (0 = unlimited)
  core_memory_cap_mode: "soft"           # "soft" (default) or "hard"

# SQLite database paths
sqlite:
  short_term_path: "./data/short_term.db"  # STM database location
  long_term_path: "./data/long_term.db"    # LTM database location

# Knowledge indexing configuration
indexing:
  enabled: true                        # Enable automatic file indexing
  directories:                         # Directories to monitor and index
    - ./knowledge
  poll_interval_seconds: 60            # How often to check for changes
  extensions:                          # File types to index
    - .txt
    - .md
    - .json
    - .csv
    - .log
    - .yaml
    - .yml
```

### Configuration Reference

| Parameter | Default | Description |
|-----------|---------|-------------|
| `embeddings.provider` | `"disabled"` | Embedding provider: `"disabled"` (no LTM), `"internal"` (use main LLM), or a provider ID |
| `embeddings.internal_model` | `"qwen/qwen3-embedding-8b"` | Model for internal embedding generation |
| `agent.memory_compression_char_limit` | `50000` | Characters before STM compression triggers |
| `agent.core_memory_max_entries` | `200` | Maximum core memory entries (0 = unlimited) |
| `agent.core_memory_cap_mode` | `"soft"` | How to handle overflow: `"soft"` (warn) or `"hard"` (reject) |
| `sqlite.short_term_path` | `"./data/short_term.db"` | Path to STM SQLite database |
| `indexing.enabled` | `true` | Enable automatic knowledge base indexing |

## Memory Optimization

### Token Usage Management

| Component | Typical Tokens | Optimization |
|-----------|----------------|--------------|
| System prompt | 500-1000 | Fixed |
| Core memory | 200-500 | Keep concise |
| STM (20 messages) | 1000-2000 | Adjust context window |
| LTM results | 500-1500 | Adjust retrieval settings |
| **Total per request** | **2200-5000** | Monitor costs |

### Reducing Memory Overhead

```yaml
# config.yaml - Memory optimization
agent:
  memory_compression_char_limit: 30000   # Compress earlier
  core_memory_max_entries: 50            # Keep core memory small
  
embeddings:
  provider: "disabled"                   # Disable LTM if not needed

indexing:
  enabled: false                         # Disable file indexing
```

### Database Maintenance

**Pruning old conversations:**
```bash
# Via Web UI → Settings → Memory → Cleanup
# Or manually delete from data/short_term.db
```

**Optimizing LTM:**
```bash
# LTM is stored in SQLite at sqlite.long_term_path
# Run VACUUM periodically to reclaim space
```

### Memory Compression Strategies

**Automatic compression:**
- Old conversations → Summaries (triggered by `memory_compression_char_limit`)
- Large documents → Chunked embeddings
- Knowledge graph → Prune low-confidence relations

**Manual cleanup commands:**
```
User: "Forget everything about that old project"
User: "Delete all notes older than 6 months"
User: "Clear my todo list"
```

## Best Practices for Knowledge Management

### DO ✅

| Practice | Why |
|----------|-----|
| Use explicit "remember" commands | Ensures critical facts are stored |
| Keep core memory minimal | Reduces token costs |
| Categorize notes consistently | Easier retrieval |
| Set priorities on to-dos | Better task management |
| Review and prune periodically | Maintain performance |
| Use specific search terms | Better LTM retrieval |

### DON'T ❌

| Practice | Why |
|----------|-----|
| Store everything in core memory | Excessive token usage |
| Create duplicate notes | Wastes storage, confuses retrieval |
| Ignore memory limits | Performance degradation |
| Store sensitive data unencrypted | Security risk |
| Rely only on automatic extraction | Explicit commands are more reliable |

### Recommended Workflow

```
1. Initial Setup
   └── Add critical facts to Core Memory

2. Daily Use
   ├── Let STM handle conversation flow
   ├── Use "remember" for important facts
   └── Create notes for reference info

3. Task Management
   ├── Add todos with priorities
   ├── Set due dates
   └── Review and complete regularly

4. Weekly Review
   ├── Check memory usage in dashboard
   ├── Prune unnecessary entries
   └── Update Core Memory if needed
```

### Memory Commands Quick Reference

```
Core Memory:
  /core add [fact]          - Add to core memory
  /core show                - Display core memory
  /core remove [key]        - Remove entry

Notes:
  /note add [text]          - Create note
  /note list                - List all notes
  /note search [query]      - Search notes
  /note delete [id]         - Delete note

To-Dos:
  /todo add [task]          - Create todo
  /todo list                - Show pending todos
  /todo done [id]           - Mark complete
  /todo priority [id] [p]   - Set priority

LTM:
  /memory search [query]    - Search long-term memory
  /memory forget [query]    - Remove matching memories
  /memory stats             - Show memory statistics
```

## Helper LLM — Automated Maintenance

The Helper LLM is a secondary, lower-cost LLM that handles background maintenance tasks to keep the main agent fast and efficient.

### Overview

```
┌──────────────────────────────────────────────────────────┐
│  Helper LLM — Background Maintenance                       │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  • Turn Analysis        → Extracts facts, preferences    │
│  • Daily Summary + KG  → Summarizes + extracts entities │
│  • Consolidation       → Batch memory consolidation     │
│  • Memory Compression  → Compresses conversation history │
│  • RAG Batch           → Batch RAG processing          │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### Operations

| Operation | Description | Trigger |
|-----------|-------------|---------|
| **Turn Analysis** | Analyzes each conversation turn for facts, preferences, mood, and pending actions | After each turn |
| **Maintenance Summary + KG** | Daily summary + knowledge graph extraction | Daily maintenance |
| **Consolidation Batches** | Consolidates conversation batches into long-term knowledge | Periodic |
| **Memory Compression** | Compresses old conversation history | Character limit reached |
| **Content Summaries** | Generates summaries of content | On demand |
| **RAG Batch** | Batch processing for retrieval-augmented generation | Periodic |

### Turn Analysis

After each conversation turn, the Helper LLM analyzes:

**Memory Analysis:**
- **Facts**: Concrete user/project/environment facts worth remembering
- **Preferences**: User preferences, habits, workflows
- **Corrections**: Updates to previously known information
- **Pending Actions**: Deferred follow-ups or unfinished work

**Activity Digest:**
- User intent and goal
- Actions taken by the agent
- Outcomes and important points
- Pending items
- Importance level (1-4)

**Personality Analysis:**
- User sentiment and mood
- Appropriate response mood for next turn
- Trait deltas (curiosity, thoroughness, creativity, empathy, etc.)
- Emotion state description

### Dashboard Monitoring

View Helper LLM statistics in the Dashboard → System tab:

```
Helper LLM Statistics
┌─────────────────────────────────────────────────────────┐
│  State: Enabled ✓                                       │
│  Last Update: 2 minutes ago                             │
│                                                         │
│  Requests: 1,234        LLM Calls: 567                │
│  Cache Hits: 432 (35%)    Fallbacks: 12                │
│                                                         │
│  Saved Calls: 89          Batched Items: 456           │
│                                                         │
│  Operations:                                             │
│  • Turn Analysis: 1,234 successful                      │
│  • Daily Summary + KG: 28 successful                    │
│  • Consolidation Batches: 5 successful                   │
│  • Memory Compression: 3 triggered                      │
│  • Content Summaries: 45 successful                      │
│  • RAG Batch: 89 successful                            │
└─────────────────────────────────────────────────────────┘
```

### Configuration

```yaml
helper_llm:
  enabled: true                    # Enable Helper LLM
  provider: "auto"                 # "auto" or provider-id
  model: "auto"                    # "auto" or specific model
  cache_enabled: true               # Cache analysis results
  operations:
    turn_analysis: true            # Analyze each turn
    daily_summary: true            # Daily maintenance
    consolidation: true            # Batch consolidation
    compression: true              # Memory compression
    content_summaries: true        # Generate summaries
    rag_batch: true               # RAG batch processing
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `enabled` | `true` | Enable/disable Helper LLM |
| `provider` | `"auto"` | LLM provider to use |
| `model` | `"auto"` | Model to use (auto selects cheapest capable) |
| `cache_enabled` | `true` | Cache results to avoid recomputation |

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| Helper LLM shows as disabled | Feature not enabled | Set `helper_llm.enabled: true` |
| High Helper LLM costs | Too frequent operations | Reduce operation frequency |
| No turn analysis | Provider doesn't support function calling | Use a capable model |
| Cache miss rate too high | Cache cleared frequently | Check cache storage settings |

---

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| "I forgot what we discussed" | STM cleared | Check if `/reset` was used; check LTM retrieval |
| High API costs | Too much context | Reduce context window size |
| Irrelevant memories retrieved | Embedding mismatch | Check `embeddings.provider` config |
| Missing information | Not stored or expired | Use explicit "remember" commands |
| LTM not working | Embeddings disabled | Set `embeddings.provider` to "internal" or a provider ID |

---

> 💡 **Remember:** Good memory management is key to a personalized experience. Be intentional about what you store, and your AI assistant will become increasingly helpful over time.

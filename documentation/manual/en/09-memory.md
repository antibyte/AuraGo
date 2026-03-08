# Chapter 9: Memory & Knowledge

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
│  Memory (LTM)     │ ChromaDB / pgvector                 │
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

- **Storage**: SQLite database (`data/conversations.db`)
- **Retention**: Last N messages (configurable, default: 20)
- **Structure**: Message history with timestamps, roles, and metadata

### Configuration

```yaml
memory:
  short_term:
    max_messages: 20          # Messages kept in context
    summary_threshold: 50     # When to compress older messages
    compression_enabled: true # Enable persistent summaries
```

### Persistent Summary Compression

When conversations exceed the threshold, older messages are compressed into a persistent summary:

```
Original: 50 individual messages
↓ Compression
Summary: "User discussed vacation plans for Italy in June. 
          Preferences: Rome 3 days, Florence 2 days. 
          Budget concerns mentioned."
```

> 💡 **Tip:** Compression reduces token usage while preserving key context. The summary is stored alongside the raw messages for full recall if needed.

### STM in Action

```
User: "What's the weather like?"
[Agent checks location from STM context]

User: "And tomorrow?"
[Agent understands "weather" + "tomorrow" without repetition]
```

## Long-Term Memory (LTM) / RAG

Long-term memory enables AuraGo to recall information from past conversations, documents, and learned facts using semantic search.

### Vector Database Options

| Database | Best For | Embedding Model |
|----------|----------|-----------------|
| **ChromaDB** | Local deployments, single-instance | `sentence-transformers/all-MiniLM-L6-v2` |
| **pgvector** | Scalable deployments, existing PostgreSQL | Same as above |

### How RAG Works

```
User Query → Embedding Model → Vector Search → Top-K Results → Context Injection → LLM Response
```

1. **Storage**: Text is converted to vector embeddings (384 dimensions)
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

> 🔍 **Deep Dive: Embedding Models**
>
> AuraGo uses `all-MiniLM-L6-v2` by default (22MB, fast, good quality). 
> For multilingual support, consider `paraphrase-multilingual-MiniLM-L12-v2`.
> Change in `config.yaml`:
> ```yaml
> memory:
>   embedding_model: "sentence-transformers/all-MiniLM-L6-v2"
> ```

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

## Memory Optimization

### Token Usage Management

| Component | Typical Tokens | Optimization |
|-----------|----------------|--------------|
| System prompt | 500-1000 | Fixed |
| Core memory | 200-500 | Keep concise |
| STM (20 messages) | 1000-2000 | Adjust `max_messages` |
| LTM results | 500-1500 | Adjust `max_results` |
| **Total per request** | **2200-5000** | Monitor costs |

### Reducing Memory Overhead

```yaml
# config.yaml - Memory optimization
memory:
  short_term:
    max_messages: 10          # Reduce for cheaper calls
    compression_enabled: true # Always enable compression
  
  long_term:
    max_results: 3            # Fewer retrieved memories
    similarity_threshold: 0.7 # Higher = fewer but better matches
    chunk_size: 512           # Larger chunks = fewer embeddings
  
  core_memory:
    max_entries: 20           # Limit core memory size
```

### Database Maintenance

**Pruning old conversations:**
```bash
# Via Web UI → Settings → Memory → Cleanup
# Or manually delete from data/conversations.db
```

**Optimizing vector DB:**
```bash
# ChromaDB automatically manages indices
# For pgvector, run VACUUM periodically
```

### Memory Compression Strategies

**Automatic compression:**
- Old conversations → Summaries
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

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| "I forgot what we discussed" | STM cleared | Check if `/reset` was used; check LTM retrieval |
| High API costs | Too much context | Reduce `max_messages` and `max_results` |
| Irrelevant memories retrieved | Low similarity threshold | Increase `similarity_threshold` in config |
| Slow responses | Large vector DB | Prune old memories, consider pgvector |
| Missing information | Not stored or expired | Use explicit "remember" commands |

---

> 💡 **Remember:** Good memory management is key to a personalized experience. Be intentional about what you store, and your AI assistant will become increasingly helpful over time.

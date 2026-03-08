# Chapter 15: Co-Agents

Co-agents are parallel sub-agents that process complex tasks concurrently. They use a separate LLM model, have their own limits, and are isolated from the main agent.

---

## What are Co-Agents?

Co-agents are independent helper agents that the main agent can dynamically spawn. They work in parallel on sub-tasks and deliver their results back.

```
┌────────────────────────────────────────────────────────────┐
│                        Main Agent                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  │Personality│  │  Memory   │  │  Tools   │                 │
│  │  Engine   │  │  (R/W)   │  │ Dispatch │                 │
│  └──────────┘  └────┬─────┘  └──────────┘                 │
│                     │ READ-ONLY Snapshot                    │
│         ┌───────────┼───────────┐                          │
│         ▼           ▼           ▼                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                    │
│  │Co-Agent 1│  │Co-Agent 2│  │Co-Agent 3│  (max_concurrent)│
│  │ Model B  │  │ Model B  │  │ Model B  │                  │
│  │ Task: A  │  │ Task: B  │  │ Task: C  │                  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                 │
│       │ Result       │ Result      │ Result                │
│       └──────────────┴─────────────┘                       │
│                      ▼                                     │
│              CoAgentRegistry                               │
└────────────────────────────────────────────────────────────┘
```

### Core Principles

| Principle | Description |
|-----------|-------------|
| **Isolation** | Co-agents don't affect main agent's personality or memory |
| **Read-Only Memory** | Co-agents get a snapshot but can't write back |
| **Own Limits** | Each co-agent has its own token counter and circuit breaker |
| **Security** | All security features (Guardian, path traversal checks) apply |
| **No User Interaction** | Co-agents only communicate via result channel |
| **Deterministically Stoppable** | Main agent can stop any co-agent via `context.Cancel()` |

---

## When to Use Co-Agents

### Ideal Use Cases

| Scenario | Example | Why Co-Agents? |
|----------|---------|----------------|
| **Parallel Research** | Analyze 3 competitors simultaneously | 3x faster than sequential |
| **Multi-Server Analysis** | Check logs on 5 servers | Parallel SSH connections |
| **Data Processing** | Process 10 files independently | No blocking of main agent |
| **Code Review** | Syntax, security, performance checks | Different models per aspect |
| **A/B Testing** | Compare two approaches | Concurrent execution |

### Anti-Patterns

| Don't Use For | Why |
|---------------|-----|
| Simple sequential tasks | Overhead not worth it |
| Tasks requiring coordination | Co-agents don't communicate with each other |
| Memory-intensive operations | Resource contention |
| Real-time user interaction | No direct user communication |

---

## Configuration

### config.yaml Section

```yaml
co_agents:
  enabled: false              # Enable co-agent system
  max_concurrent: 3           # Max parallel co-agents
  
  # LLM configuration for co-agents
  llm:
    provider: "openrouter"
    base_url: ""
    api_key: ""              # Falls back to main llm.api_key
    model: "meta-llama/llama-3.1-8b-instruct:free"  # Cheaper/faster
  
  # Own limits per co-agent
  circuit_breaker:
    max_tool_calls: 10        # Max tool calls per task
    timeout_seconds: 120      # Max runtime per co-agent
    max_tokens: 8000          # Token budget (0 = unlimited)
```

### Defaults

- `MaxConcurrent`: 3
- `MaxToolCalls`: 10
- `TimeoutSeconds`: 120
- `MaxTokens`: 0 (unlimited)

---

## Spawning Co-Agents

The main agent spawns co-agents via the `co_agent` tool:

### Spawn Operation

```json
{
  "action": "co_agent",
  "operation": "spawn",
  "task": "Research the latest Go 1.22 features. Use web_search and summarize the top 5 changes.",
  "context_hints": ["We are planning a migration from Go 1.21"]
}
```

**Response:**
```json
{
  "status": "ok",
  "co_agent_id": "coagent-1",
  "available_slots": 2,
  "message": "Co-agent started. Use 'list' to check status and 'get_result' when done."
}
```

### Available Operations

| Operation | Parameters | Description |
|-----------|------------|-------------|
| `spawn` | `task`, `context_hints` | Start a new co-agent |
| `list` | — | Show all co-agents with status |
| `get_result` | `co_agent_id` | Get finished result |
| `stop` | `co_agent_id` | Cancel running co-agent |
| `stop_all` | — | Cancel all co-agents |

---

## Managing Co-Agents

### List All Co-Agents

```json
{"action": "co_agent", "operation": "list"}
```

**Response:**
```json
{
  "status": "ok",
  "available_slots": 1,
  "max_slots": 3,
  "co_agents": [
    {
      "id": "coagent-1",
      "task": "Research Go 1.22 features",
      "state": "completed",
      "runtime": "34.2s",
      "tokens_used": 2100,
      "tool_calls": 5
    },
    {
      "id": "coagent-2",
      "task": "Check Docker best practices",
      "state": "running",
      "runtime": "28.5s"
    }
  ]
}
```

### Get Result

```json
{"action": "co_agent", "operation": "get_result", "co_agent_id": "coagent-1"}
```

**Response:**
```json
{
  "status": "ok",
  "co_agent_id": "coagent-1",
  "result": "## Go 1.22 Features\n1. Improved loop variable semantics..."
}
```

### Stop Co-Agent

```json
{"action": "co_agent", "operation": "stop", "co_agent_id": "coagent-2"}
```

---

## Use Cases and Examples

### Example 1: Parallel Research

**Task:** Research 3 different topics simultaneously

```
Main Agent: I'll spawn 3 co-agents to research in parallel.

[Spawns coagent-1, coagent-2, coagent-3]

After completion:
Main Agent: Here's what I found:
• Topic A (coagent-1): [Summary]
• Topic B (coagent-2): [Summary]
• Topic C (coagent-3): [Summary]
```

### Example 2: Multi-Server Log Analysis

```
Task: Check error logs on web-server-1, web-server-2, and db-server

Main Agent spawns 3 co-agents:
- Co-agent 1: SSH to web-server-1, grep logs for ERROR
- Co-agent 2: SSH to web-server-2, grep logs for ERROR
- Co-agent 3: SSH to db-server, grep logs for ERROR

Result: Combined error report from all servers
```

### Example 3: Data Extraction

```
Task: Extract contact info from 10 PDFs

Main Agent spawns co-agents:
- Each co-agent processes 3-4 PDFs
- Parallel extraction
- Combined results
```

---

## Limitations and Constraints

### Tool Blacklist

Co-agents **cannot** use:
- `manage_memory` (write operations)
- `knowledge_graph` (write operations)
- `manage_notes` (create/update/delete)
- `co_agent` (no nested co-agents)
- `follow_up` (no self-scheduling)
- `cron_scheduler`

They **can** use:
- Read operations (query_memory, search)
- Filesystem tools
- Web tools
- Execution tools (with same restrictions)

### Architectural Limits

| Limit | Value | Configurable |
|-------|-------|--------------|
| Max concurrent | 3 | Yes |
| Max tool calls | 10 | Yes |
| Max runtime | 120s | Yes |
| Max tokens | Unlimited | Yes |
| Nesting depth | 1 (no sub-co-agents) | No |

---

## Resource Management

### Memory Usage

Each co-agent:
- Gets its own in-memory history (ephemeral)
- Shares read-only access to databases
- Uses same Python venv

**Recommendation:** Monitor RAM with `max_concurrent: 3` on systems with 4GB+ RAM.

### Token Budget

Set `max_tokens` to prevent cost explosion:

```yaml
co_agents:
  circuit_breaker:
    max_tokens: 8000  # ~$0.01-0.02 per co-agent
```

### CPU Usage

Co-agents run as goroutines (lightweight threads). CPU usage scales with:
- Number of concurrent co-agents
- Complexity of tasks
- LLM response times

---

## Security Considerations

### Risk Analysis

| Risk | Severity | Mitigation |
|------|----------|------------|
| File overwrite conflict | Medium | Shared workspace = acceptable risk |
| Token budget exceeded | Medium | Token limit + timeout |
| Blocking shell process | Low | Timeout + process registry |
| Injection in results | Low | Main agent treats as plain text |
| Rate limiting | Medium | Max concurrent limit |

### Best Practices

- Use cheaper/faster model for co-agents
- Set reasonable timeouts
- Monitor token usage
- Don't exceed 3-5 concurrent agents
- Use read-only mode where possible

---

## Monitoring

### Web UI

Dashboard → Co-Agents tab shows:
- Active co-agents
- Runtime per agent
- Token usage
- Success/failure rates

### API

```bash
# Get status
curl http://localhost:8088/api/co-agents

# Get specific result
curl http://localhost:8088/api/co-agents/coagent-1/result
```

### Logging

Co-agent activities are logged with:
- Component: `co-agent`
- ID: `coagent-N`
- Task description
- Token usage

```
2024-01-15 14:30:00 [co-agent] [coagent-1] Started: "Research Go features"
2024-01-15 14:30:34 [co-agent] [coagent-1] Completed: 2100 tokens, 5 tools
```

---

> 🔍 **Deep Dive: Lifecycle**
>
> ```
> Main Agent          Co-Agent Goroutine
>     │                      │
>     ├─ spawn(task) ───────►│ Starts
>     │                      ├─ Own LLM client
>     │                      ├─ System prompt (helper)
>     │                      ├─ RunSyncAgentLoop
>     │   ...continues...    │  ├─ LLM Call
>     ├─ list() ◄────────────┤  ├─ Tool dispatch
>     │   Running            │  └─ Final answer
>     │                      └─ Registry.Complete(result)
>     ├─ get_result(id) ◄────┤
>     └─ Uses result         │
> ```

---

> 💡 **Tip:** Start with simple tasks. Co-agents excel at parallel research and data processing, but add complexity. Use them when the time savings justify the overhead.

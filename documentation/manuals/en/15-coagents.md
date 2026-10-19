# Chapter 15: Co-Agents

<p align="center">
  <a href="../images/manual-missions.webp"><img src="../images/manual-missions.webp" width="480" alt="Two smaller helper gophers beside the main gopher"></a>
</p>

Helpers for subtasks. Their own model, their own limits, no access to your personality notes.

---

## Table of Contents

1. [What Are Co-Agents?](#what-are-co-agents)
2. [When to Use Co-Agents](#when-to-use-co-agents)
3. [Configuration](#configuration)
4. [Spawning Co-Agents](#spawning-co-agents)
5. [Managing Co-Agents](#managing-co-agents)
6. [Use Cases and Examples](#use-cases-and-examples)
7. [Limitations and Constraints](#limitations-and-constraints)
8. [Resource Management](#resource-management)
9. [Security Considerations](#security-considerations)
10. [Monitoring](#monitoring)
11. [Deep Dive: Co-Agent Lifecycle](#deep-dive-co-agent-lifecycle)
12. [Troubleshooting](#troubleshooting)
13. [Summary](#summary)

---

## What Are Co-Agents?

Co-agents are independent helper agents that the main agent can dynamically spawn. They work in parallel on sub-tasks and deliver their results back.

```text
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

> [!TIP]
> **Web UI first:** Open **Config → Co-Agents** to enable the system, set concurrency limits, assign a provider, and configure specialists. YAML below is for headless or advanced setups.

### Web UI Setup
1. Open **Config → Co-Agents**.
2. Enable co-agents and set **Max Concurrent** agents.
3. Configure the co-agent LLM provider (or leave empty to use the main provider).
4. Tune circuit breaker limits and optional **Budget Quota Percent**.
5. Enable or disable specialists (researcher, coder, designer, security, writer).
6. Save changes.

### YAML Reference

```yaml
co_agents:
  enabled: false              # Enable co-agent system
  max_concurrent: 3           # Max parallel co-agents
  
  # LLM configuration for co-agents
  llm:
    provider: "openrouter"
    base_url: ""
    api_key: ""              # Falls back to main provider
    model: "meta-llama/llama-3.1-8b-instruct:free"  # Cheaper/faster
  
  # Own limits per co-agent
  circuit_breaker:
    max_tool_calls: 10        # Max tool calls per task (default: 10)
    timeout_seconds: 300      # Max runtime per co-agent (default: 5 min)
    max_tokens: 0             # Token budget (0 = unlimited)
  queue_when_busy: true       # Queue when all slots are busy
  budget_quota_percent: 0     # Daily budget share (0 = disabled)
```

### Defaults

- `max_concurrent`: 3
- `max_tool_calls`: 10
- `timeout_seconds`: 300
- `max_tokens`: 0 (unlimited)
- `queue_when_busy`: true
- `budget_quota_percent`: 0

A queued co-agent waits at most `timeout_seconds` for a free slot. If no slot frees up in time it is marked `failed` and never occupies a slot. The `timeout_seconds` runtime budget starts when the co-agent leaves the queue.

### Writer Specialist Default

The `writer` specialist uses a multilingual natural-writing default in `co_agents.specialists.writer.additional_prompt`. It helps the author co-agent avoid generic AI wording, preserve the requested language and register, and keep rewrites honest without inventing facts. Writer co-agents run without runtime tool schemas by default so short writing tasks do not pay the latency cost of unrelated tools. Set the field to `""` to disable this default, or replace it with your own writing rules.

The compact guidance adapts [Humanizer 3.0](https://github.com/blader/humanizer/blob/main/SKILL.md): revise artificial contrasts, inflated claims, and repetitive closing lines while preserving the writer's voice. It adds German and multilingual conventions, protects quotations and technical literals, and silently checks for lost claims, invented details, and fresh formulaic phrasing before delivery. Updates preserve existing custom additional prompts. Attribution and the MIT license are in `THIRD_PARTY_NOTICES.md`.

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
  "message": "Co-agent started. Use 'list' to check quick status and 'get_result' to wait for the result."
}
```

### Available Operations

| Operation | Parameters | Description |
|-----------|------------|-------------|
| `spawn` | `task`, `context_hints`, `priority`, `output_schema` | Start a generic co-agent |
| `spawn_specialist` | `task`, `specialist`, `context_hints`, `priority`, `output_schema` | Start a specialist (`researcher`, `coder`, `designer`, `security`, `writer`) |
| `list` | — | Show all co-agents with status |
| `get_result` | `co_agent_id` | Wait briefly and get finished result or current terminal status |
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

If the co-agent is still queued or running, `get_result` waits server-side for a bounded interval before returning. A running co-agent has not failed merely because no partial tokens are visible; non-streaming LLM calls may stay quiet until the provider responds.

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

```text
Main Agent: I'll spawn 3 co-agents to research in parallel.

[Spawns coagent-1, coagent-2, coagent-3]

After completion:
Main Agent: Here's what I found:
• Topic A (coagent-1): [Summary]
• Topic B (coagent-2): [Summary]
• Topic C (coagent-3): [Summary]
```

### Example 2: Multi-Server Log Analysis

```text
Task: Check error logs on web-server-1, web-server-2, and db-server

Main Agent spawns 3 co-agents:
- Co-agent 1: SSH to web-server-1, grep logs for ERROR
- Co-agent 2: SSH to web-server-2, grep logs for ERROR
- Co-agent 3: SSH to db-server, grep logs for ERROR

Result: Combined error report from all servers
```

### Example 3: Data Extraction

```text
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

Set `max_tokens` to prevent cost explosion.

> 🖥️ **Web UI:** **Config → Co-Agents** → Circuit Breaker → **Max Tokens**.

### YAML Reference
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

Dashboard → **Tools** section → **Activity** card lists the co-agents:
- Task (or ID), with a specialist icon where one is set
- State: queued, running, completed, failed or cancelled
- Runtime, queue position and retry count

### API

Co-agents have **no** dedicated `/api/co-agents` routes. Monitor them via:

```bash
# Dashboard activity (includes coagents array)
curl http://localhost:8088/api/dashboard/activity

# Or via the co_agent tool:
# {"action":"co_agent","operation":"list"}
# {"action":"co_agent","operation":"get_result","co_agent_id":"coagent-1"}
```

### Logging

Co-agent activities are logged with:
- Component: `co-agent`
- ID: `coagent-N`
- Task description
- Token usage

```text
2024-01-15 14:30:00 [co-agent] [coagent-1] Started: "Research Go features"
2024-01-15 14:30:34 [co-agent] [coagent-1] Completed: 2100 tokens, 5 tools
```

### Operational Limits and Safety

| Area | Guidance |
|------|----------|
| Concurrency | Keep `max_concurrent` low until you know the token and CPU impact |
| Budgets | Assign explicit per-agent token limits and use circuit breakers |
| Tool access | Do not grant memory-writing, destructive, or nested co-agent capabilities unless required |
| Result handling | Treat co-agent results as supporting evidence, not automatically trusted truth |
| Long tasks | Prefer Mission Control, daemon skills, or `wait_for_event` for work that must survive long waits |

Specialists are best used for bounded, independently verifiable work: research, code review, security checks, writing drafts, and parallel data gathering. Avoid using them for tasks that require live user interaction or privileged changes.

---

## Deep Dive: Co-Agent Lifecycle

```text
Main Agent                          Co-Agent Goroutine
    │                                      │
    ├─ spawn("Analyse X")                  │
    │   ├─ Check slot ✓                    │
    │   ├─ context.WithTimeout()           │
    │   ├─ Registry.Register() ──────────► │ Goroutine starts
    │   └─ return co_agent_id              │ ├─ Own LLM client
    │                                      │ ├─ System prompt (helper)
    │   ... Main agent keeps working ...   │ ├─ RunSyncAgentLoop()
    │                                      │ │  ├─ LLM Call
    ├─ list() ◄─ Status: "running"         │ │  ├─ Tool Dispatch
    │                                      │ │  ├─ LLM Call
    │                                      │ │  └─ Final Answer
    │                                      │ └─ Registry.Complete(result)
    │                                      │
    ├─ list() ◄─ Status: "completed"       │
    ├─ get_result(id) ◄─ Result            │
    └─ Merges result into the answer       │
```

### Resource Sharing

| Resource | Sharing model | Reason |
|----------|---------------|--------|
| **Config** | Shallow copy with overrides | Own limits, personality disabled |
| **LLM client** | Own instance | Different model, own rate limits |
| **ShortTermMem** | Shared, read-only | SQLite is thread-safe |
| **LongTermMem** | Shared, read-only | VectorDB with RWMutex |
| **Vault** | Shared reference | Thread-safe, API keys needed |
| **HistoryManager** | Own instance | Ephemeral (not persisted) |
| **CronManager** | nil | No cron access |

---

## Troubleshooting

### Problem: "all 3 co-agent slots are occupied"

**Fix:**
```json
// Check running co-agents
{"action": "co_agent", "operation": "list"}

// Clean up old co-agents or wait
{"action": "co_agent", "operation": "stop_all"}
```

**Prevention:** Raise `max_concurrent` under **Config → Agent → Co-Agents**:
```yaml
co_agents:
  max_concurrent: 5
```

### Problem: Co-Agent Hangs (Timeout)

**Symptom:** Status stays `running` with no progress

**Fix:**
```json
// Stop it manually
{"action": "co_agent", "operation": "stop", "co_agent_id": "coagent-X"}
```

**Prevention:** Lower `timeout_seconds` or `max_tool_calls` under **Config → Agent → Co-Agents**:
```yaml
circuit_breaker:
  timeout_seconds: 120
  max_tool_calls: 8
```

### Problem: Result Is Empty

**Causes:**
1. Task was too vague
2. Co-agent hit its timeout
3. The LLM returned no content

**Debug:**
```json
// Give more context with context_hints
{"action": "co_agent", "operation": "spawn",
 "task": "...",
 "context_hints": ["Very precise instruction", "Expected format: Markdown list"]}
```

---

## Summary

| Aspect | Recommendation |
|--------|----------------|
| **Enable** | `co_agents.enabled: true` |
| **Model** | Cheaper/faster than the main agent |
| **Concurrency** | 3 for standard workloads, 5+ for servers |
| **Timeout** | 120–300 seconds depending on the task |
| **Token budget** | 0 (unlimited) or 8000+ when controlling cost |
| **Good for** | Parallel, independent tasks |
| **Avoid** | Dependent tasks, real-time interaction |

---

> [!TIP]
> **Next steps:**
> - **[Security](14-security.md)** – Configure co-agents safely
> - **[Configuration](07-configuration.md)** – Every parameter in detail
> - **[Tools](06-tools.md)** – Learn the available tools

---

**Previous Chapter:** [Chapter 14: Security](./14-security.md)
**Next Chapter:** [Chapter 16: Troubleshooting](./16-troubleshooting.md)

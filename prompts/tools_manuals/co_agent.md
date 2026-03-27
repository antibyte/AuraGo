# Co-Agent Tool

Spawn and manage parallel co-agents that work on sub-tasks independently. Each co-agent runs in its own goroutine with a separate LLM context and returns results when done.
Assume that the co-agents may be less capable than you, so you should double check their work.

## Prerequisites
- `co_agents.enabled: true` in config.yaml
- Optional: separate LLM model/API key via `co_agents.llm.*`

## Operations

### spawn — Start a new generic co-agent with a task
```json
{"action": "co_agent", "operation": "spawn", "task": "Research the current weather in Berlin and summarize it"}
{"action": "co_agent", "operation": "spawn", "task": "Analyze the server logs for errors", "context_hints": ["server", "logs", "errors"]}
{"action": "co_agent", "operation": "spawn", "task": "Review the refactor and report risks", "priority": 3}
```
- `task` (required): Natural language description of what the co-agent should do
- `context_hints` (optional): Keywords for RAG context injection — helps the co-agent find relevant memories
- `priority` (optional): Queue priority `1=low`, `2=normal`, `3=high`

### spawn_specialist — Start a specialized expert co-agent
```json
{"action": "co_agent", "operation": "spawn_specialist", "specialist": "researcher", "task": "Find the latest CVE vulnerabilities for OpenSSL 3.x"}
{"action": "co_agent", "operation": "spawn_specialist", "specialist": "coder", "task": "Write a Go function that parses the config file and returns all enabled providers"}
{"action": "co_agent", "operation": "spawn_specialist", "specialist": "designer", "task": "Create a modern logo for the AuraGo project, minimalist style"}
{"action": "co_agent", "operation": "spawn_specialist", "specialist": "security", "task": "Audit the authentication middleware for common vulnerabilities"}
{"action": "co_agent", "operation": "spawn_specialist", "specialist": "writer", "task": "Write a professional blog post about home lab automation"}
{"action": "co_agent", "operation": "spawn_specialist", "specialist": "coder", "task": "Refactor the parser and add tests", "context_hints": ["parser", "tests"], "priority": 3}
```
- `specialist` (required): One of `researcher`, `coder`, `designer`, `security`, `writer`
- `task` (required): The task suited to the specialist's expertise
- `context_hints` (optional): Keywords for RAG context injection
- `priority` (optional): Queue priority `1=low`, `2=normal`, `3=high`

#### Specialist Roles

| Specialist | Best For | Key Tools |
|-----------|----------|-----------|
| **researcher** | Internet research, fact-finding, source verification | Web search skills, API requests, memory/RAG |
| **coder** | Code writing, debugging, testing, architecture | Shell, Python, filesystem, git |
| **designer** | Image generation, layouts, visual concepts | Image generation, filesystem |
| **security** | Vulnerability audits, code review, system hardening | Shell (read), Python, filesystem (read) |
| **writer** | Articles, docs, creative writing, communication | Memory/RAG, filesystem |

### list — Show all co-agents and their status
```json
{"action": "co_agent", "operation": "list"}
```
Returns: list of co-agents with ID, task, specialist role, state (queued/running/completed/failed/cancelled), timestamps, and available slots.
Queued entries also include queue position, retry count, and recent lifecycle events.

### get_result — Retrieve the result of a completed co-agent
```json
{"action": "co_agent", "operation": "get_result", "co_agent_id": "specialist-researcher-1"}
```
- Returns the final text output from the co-agent
- Only works for completed co-agents (returns error if still running)

### stop — Cancel a running co-agent
```json
{"action": "co_agent", "operation": "stop", "co_agent_id": "specialist-coder-2"}
```

### stop_all — Cancel all running co-agents
```json
{"action": "co_agent", "operation": "stop_all"}
```

## Workflow Pattern

1. **Spawn** one or more co-agents/specialists with specific tasks
2. **Continue** working on other things while they run
3. **Check status** with `list` periodically
4. **Retrieve results** with `get_result` once completed
5. **Integrate** results into your response

## When to Use Specialists vs Generic Co-Agents

- **Use specialists** when the task clearly falls into one domain (research, coding, design, security, writing)
- **Use generic co-agents** for general-purpose tasks that don't need specialized expertise
- **Combine specialists** for complex projects: e.g., researcher finds info → writer creates docs
- **Always check results** — specialists may need guidance refinement

## Concurrency
- Maximum concurrent co-agents: configured via `co_agents.max_concurrent` (default: 3)
- Specialists and generic co-agents share the same slot pool
- If all slots are occupied and `co_agents.queue_when_busy` is enabled, new co-agents are queued automatically
- Queue order prefers higher `priority`, then older queued tasks
- Each co-agent has its own circuit breaker (max tool calls, timeout)
- Stale entries are automatically cleaned up after 30 minutes

## Restrictions
All co-agents and specialists **cannot**:
- Modify core memory (read/query only)
- Write to the knowledge graph (read only)
- Modify notes (list only)
- Spawn sub-agents (no recursion)
- Schedule follow-ups or cron jobs

Each specialist has additional role-specific tool restrictions (e.g., designer cannot run shell commands).

## Configuration (config.yaml)
```yaml
co_agents:
  enabled: true
  max_concurrent: 3
  queue_when_busy: true
  budget_quota_percent: 25
  max_context_hints: 6
  max_context_hint_chars: 180
  max_result_bytes: 100000
  llm:
    provider: ""      # Falls back to main LLM if empty
  circuit_breaker:
    max_tool_calls: 10
    timeout_seconds: 120
    max_tokens: 4096
  retry_policy:
    max_retries: 1
    retry_delay_seconds: 5
  specialists:
    researcher:
      enabled: true
      llm:
        provider: ""  # Empty = inherit co_agents LLM
    coder:
      enabled: true
      llm:
        provider: ""
    designer:
      enabled: true
      llm:
        provider: ""
    security:
      enabled: true
      llm:
        provider: ""
    writer:
      enabled: true
      llm:
        provider: ""
```

## A2A Bridge — Remote Agents

When A2A client mode is enabled, remote A2A-compatible agents can be reached through the co-agent system. Spawned remote tasks appear as co-agents prefixed with `[A2A:<agent_id>]` and can be tracked with `list` / `get_result` like any local co-agent.

Remote agents have no shared context — always provide self-contained, clear task descriptions.

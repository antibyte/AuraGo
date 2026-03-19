# Co-Agent Tool

Spawn and manage parallel co-agents that work on sub-tasks independently. Each co-agent runs in its own goroutine with a separate LLM context and returns results when done.
Assume that the co-agents may be less capable than you, so you should double check their work.

## Prerequisites
- `co_agents.enabled: true` in config.yaml
- Optional: separate LLM model/API key via `co_agents.llm.*`

## Operations

### spawn — Start a new co-agent with a task
```json
{"action": "co_agent", "operation": "spawn", "task": "Research the current weather in Berlin and summarize it"}
{"action": "co_agent", "operation": "spawn", "task": "Analyze the server logs for errors", "context_hints": ["server", "logs", "errors"]}
```
- `task` (required): Natural language description of what the co-agent should do
- `context_hints` (optional): Keywords for RAG context injection — helps the co-agent find relevant memories

### list — Show all co-agents and their status
```json
{"action": "co_agent", "operation": "list"}
```
Returns: list of co-agents with ID, task, state (running/completed/failed/cancelled), timestamps, and available slots.

### get_result — Retrieve the result of a completed co-agent
```json
{"action": "co_agent", "operation": "get_result", "co_agent_id": "coagent-1"}
```
- Returns the final text output from the co-agent
- Only works for completed co-agents (returns error if still running)

### stop — Cancel a running co-agent
```json
{"action": "co_agent", "operation": "stop", "co_agent_id": "coagent-1"}
```

### stop_all — Cancel all running co-agents
```json
{"action": "co_agent", "operation": "stop_all"}
```

## Workflow Pattern

1. **Spawn** one or more co-agents with specific tasks
2. **Continue** working on other things while they run
3. **Check status** with `list` periodically
4. **Retrieve results** with `get_result` once completed
5. **Integrate** results into your response

## Concurrency
- Maximum concurrent co-agents: configured via `co_agents.max_concurrent` (default: 3)
- Each co-agent has its own circuit breaker (max tool calls, timeout)
- Stale entries are automatically cleaned up after 30 minutes

## Restrictions
Co-agents **cannot**:
- Modify core memory (read/query only)
- Write to the knowledge graph (read only)
- Modify notes (list only)
- Spawn sub-agents (no recursion)
- Schedule follow-ups or cron jobs

Co-agents **can**:
- Execute Python/shell commands
- Use the filesystem
- Make API requests
- Query memory and knowledge graph
- Use skills and custom tools
- Access the secrets vault (read-only)

## Configuration (config.yaml)
```yaml
co_agents:
  enabled: true
  max_concurrent: 3
  llm:
    provider: ""      # Falls back to main LLM if empty
    base_url: ""
    api_key: ""
    model: ""
  circuit_breaker:
    max_tool_calls: 10
    timeout_seconds: 120
    max_tokens: 4096
```

## A2A Bridge — Remote Agents

When A2A client mode is enabled, remote A2A-compatible agents can be reached through the co-agent system. Spawned remote tasks appear as co-agents prefixed with `[A2A:<agent_id>]` and can be tracked with `list` / `get_result` like any local co-agent.

Remote agents have no shared context — always provide self-contained, clear task descriptions.

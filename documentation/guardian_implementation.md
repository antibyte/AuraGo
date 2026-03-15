# LLM Guardian - Implementation Status

> **Status**: ✅ Implemented (Phases 1-5 complete, including enhancements)

## Overview

The LLM Guardian is an AI-powered pre-execution security layer that evaluates tool calls
using a dedicated LLM before they are executed. It works alongside the existing regex-based
Guardian (prompt injection defense) to provide defense-in-depth.

## Architecture

```
User Request → Agent Loop → Tool Call
                               ↓
                    ┌─ Regex Guardian (ScanForInjection) ──→ ThreatLevel
                    ↓
                LLM Guardian (ShouldCheck?)
                    ↓ yes
              ┌─ Cache Hit? ──→ Return cached result
              ↓ no
          Rate Limit Check
              ↓ ok
          LLM Evaluation (5s timeout)
              ↓
          Decision: allow / block / quarantine
              ↓
          block → "[TOOL BLOCKED]" error returned
          quarantine → proceed with warning log
          allow → proceed normally
```

## Files

| File | Purpose |
|------|---------|
| `internal/security/llm_guardian.go` | Core engine: LLMGuardian, Evaluate, ShouldCheck |
| `internal/security/guardian_cache.go` | TTL-based cache with LRU eviction |
| `internal/security/guardian_metrics.go` | Atomic counters for dashboard metrics |
| `internal/security/llm_guardian_test.go` | Unit tests (response parsing, cache, metrics, etc.) |
| `internal/config/config_types.go` | `LLMGuardian` config struct |
| `internal/config/config.go` | Default values |
| `internal/config/config_migrate.go` | Provider resolution (same pattern as MemoryAnalysis) |
| `ui/cfg/llm_guardian.js` | Config UI section |
| `ui/lang/config/llm_guardian/*.json` | 16 language translations for config |
| `internal/server/dashboard_handlers.go` | `/api/dashboard/guardian` endpoint |
| `ui/dashboard.html` | Guardian dashboard card |
| `ui/js/dashboard/main.js` | Guardian card rendering + auto-refresh |
| `ui/css/dashboard.css` | Guardian card styles |

## Configuration

```yaml
llm_guardian:
  enabled: false                    # Enable LLM pre-execution checks
  provider: ""                      # Provider ID (falls back to main LLM)
  model: ""                         # Model override
  default_level: "medium"           # off, low, medium, high
  fail_safe: "quarantine"           # block, allow, quarantine
  cache_ttl: 300                    # Cache seconds (0 = no cache)
  max_checks_per_min: 60            # Rate limit
  tool_overrides:                   # Per-tool level overrides
    execute_shell: "high"
    api_request: "low"
  allow_clarification: false        # Agent can justify blocked calls (1 retry)
  scan_emails: false                # LLM scan of incoming emails
  scan_documents: false             # LLM scan of webhook payloads
```

### Protection Levels

| Level | Tools Checked |
|-------|---------------|
| `off` | None |
| `low` | High-risk only: shell, sudo, python, remote_shell, filesystem |
| `medium` | All risky: shell, sudo, python, remote_shell, filesystem, api_request, docker, proxmox, set_secret, save_tool, co_agent, manage_updates, netlify, home_assistant |
| `high` | Every tool call |

### Fail-Safe Behavior

When the LLM check fails (timeout, API error, rate limit exceeded):
- `block` → tool call blocked
- `quarantine` → tool call proceeds with warning
- `allow` → tool call proceeds normally

## Integration Points

### Agent Loop (`agent_loop.go`)
- LLM Guardian created alongside regex Guardian at startup
- All 3 `DispatchToolCall` call sites pass `llmGuardian`

### DispatchToolCall (`agent_parse.go`)
- Pre-execution: regex scan → ShouldCheck → EvaluateWithFailSafe
- Blocked calls return `[TOOL BLOCKED]` message
- Quarantined calls log a warning and proceed

### MCP Server (`mcp_server_handler.go`)
- MCP tool calls also go through the LLM Guardian

### Server (`server.go`)
- `LLMGuardian` field on Server struct, initialized at startup

### Dashboard
- `/api/dashboard/guardian` returns enabled status + metrics snapshot
- Card auto-hidden when Guardian is disabled
- Auto-refreshes every 30s
- Shows clarifications and content scans counters when > 0

## Enhancement: Agent Clarification

When `allow_clarification` is enabled and a tool call is blocked, the agent can include a
`_guardian_justification` field in its tool call to argue why the operation should be allowed.
The Guardian re-evaluates the request **once** with a stricter prompt that considers the
justification. If still blocked, the denial is final.

Flow:
1. Agent sends tool call → Guardian blocks it
2. Block message includes a hint about `_guardian_justification`
3. Agent retries with justification → `EvaluateClarification()` called
4. Stricter LLM evaluation with justification context
5. If allowed → tool proceeds. If still blocked → final denial.

Implementation: `EvaluateClarification()` in `llm_guardian.go`, dispatch integration in `agent_parse.go`.

## Enhancement: Content Scanning

When `scan_emails` or `scan_documents` is enabled, incoming content is scanned by the
Guardian LLM for prompt injection, phishing, and social engineering attacks — as an
additional layer beyond the regex-based `ScanForInjection`.

Scan points:
- **Email fetching** (`agent_dispatch_comm.go`): After regex scan, emails are LLM-scanned. Blocked content is sanitized.
- **Email watcher** (`email_watcher.go`): Background email checks get LLM scanning. Blocked content is sanitized.
- **Webhooks** (`webhooks/handler.go`): Incoming webhook payloads are LLM-scanned. Blocked content is isolated.

Implementation: `EvaluateContent()` in `llm_guardian.go`, hooks in dispatch, email watcher, and webhook handler.

## Enhancement: Searchable Tool Override UI

The tool override section in the config UI now features:
- Searchable `<input>` with `<datalist>` populated from `/api/mcp-server/tools`
- Tool descriptions and risk level icons (🔴 high-risk, 🟡 risky, ⚪ normal)
- Styled override display with tool name + description
- Async tool list loading with caching

# Architecture

**Analysis Date:** 2026-04-03

## Pattern Overview

**Overall:** Modular monolith with clear separation of concerns, event-driven streaming, and layered memory architecture.

**Key Characteristics:**
- Single Go binary with embedded Web UI (no CGO dependencies)
- Agent-centric design: all components serve the autonomous agent loop
- Multi-transport communication: HTTP REST, SSE streaming, WebSocket-ready
- Tiered memory system: STM (SQLite) -> LTM (VectorDB) -> Knowledge Graph (SQLite/FTS5)
- Security-first: AES-256-GCM vault, secret scrubbing, permission toggles

## Layers

**Agent Loop (`internal/agent/`):**
- Purpose: Core autonomous agent orchestration
- Location: `internal/agent/agent_loop.go`
- Contains: Main loop, tool dispatch, memory ranking, context compression, co-agents
- Depends on: LLM client, memory systems, vault, config
- Used by: Server SSE handler, background tasks

**LLM Layer (`internal/llm/`):**
- Purpose: Multi-provider LLM client with failover and retry logic
- Location: `internal/llm/`
- Contains: FailoverManager, pricing, context window detection
- Depends on: Config, HTTP client
- Used by: Agent loop, server handlers

**Memory Layer (`internal/memory/`):**
- Purpose: Multi-tier persistent memory for conversation context and knowledge
- Locations:
  - STM: `internal/memory/short_term.go` (SQLite)
  - LTM: `internal/memory/long_term.go` (Chromem-go vector DB)
  - KG: `internal/memory/graph_sqlite.go` (SQLite + FTS5)
- Contains: Message storage, vector embeddings, entity relationships, temporal patterns
- Depends on: Config, LLM for embeddings
- Used by: Agent loop, server handlers, indexing service

**Server Layer (`internal/server/`):**
- Purpose: HTTP/HTTPS server, REST API, SSE streaming, Web UI serving
- Location: `internal/server/server.go`, `internal/server/sse.go`
- Contains: Router, handlers (60+), auth, middleware, SSE broadcaster
- Depends on: All other layers
- Used by: Web UI (browser), external API clients

**Tool Layer (`internal/tools/`, `internal/agent/native_tools*.go`):**
- Purpose: 90+ built-in tools for agent execution
- Locations: `internal/tools/*.go`, `internal/agent/native_tools*.go`
- Contains: Shell, Python, filesystem, HTTP, Docker, SSH, cron, etc.
- Depends on: Config, sandbox, vault, process registry
- Used by: Agent dispatch

**Security Layer (`internal/security/`):**
- Purpose: Vault encryption, secret scrubbing, Guardian (input validation), LLM Guardian
- Location: `internal/security/vault.go`, `internal/security/guardian.go`
- Contains: AES-256-GCM vault, token manager, scrubber, SSRF protection
- Depends on: OS crypto, file system
- Used by: All layers handling secrets

**Config Layer (`internal/config/`):**
- Purpose: YAML config parsing, defaults, provider resolution
- Location: `internal/config/config.go`, `internal/config/config_types.go`
- Contains: Config structs, env var loading, vault secret injection
- Depends on: YAML parser, vault
- Used by: All layers

## Data Flow

**User Message -> Agent Response:**

1. HTTP POST to `/api/chat` or WebSocket message received
2. Auth middleware validates session
3. Message stored in STM (SQLite `messages` table)
4. Agent loop retrieves conversation history from STM
5. LTM semantic search queries VectorDB for relevant memories
6. Knowledge Graph queried for related entities
7. Context assembly: system prompt + memories + conversation history
8. LLM call with function calling schema
9. Tool call dispatched to appropriate handler
10. Tool execution (possibly sandboxed)
11. Tool result returned to LLM
12. Final response streamed via SSE
13. Response stored in STM
14. Relevant memories archived to LTM (async consolidation)

**Memory Consolidation Flow:**

1. Nightly cron triggers consolidation job
2. Old messages archived from STM to `archived_messages` table
3. Batch processed via helper LLM for summarization
4. Summaries embedded and stored in Chromem VectorDB
5. KG entities extracted and linked
6. Original messages cleaned up after successful consolidation

## Key Abstractions

**FailoverManager (`internal/llm/`):**
- Purpose: Manage multiple LLM providers with automatic failover
- Examples: `internal/llm/failover.go`
- Pattern: Wraps primary provider, falls back to configured alternatives

**VectorDB Interface (`internal/memory/long_term.go`):**
- Purpose: Abstraction for vector database operations
- Implementation: ChromemVectorDB (chromem-go)
- Methods: StoreDocument, SearchSimilar, GetByID, DeleteDocument

**SSEBroadcaster (`internal/server/sse.go`):**
- Purpose: Real-time event streaming to Web UI
- Pattern: Pub/sub with channel-based clients
- Methods: Send, SendJSON, BroadcastType

**Vault (`internal/security/vault.go`):**
- Purpose: Encrypted secret storage
- Pattern: AES-256-GCM with atomic file writes
- Methods: ReadSecret, WriteSecret, ListKeys, EncryptBytes

**ProcessRegistry (`internal/tools/registry.go`):**
- Purpose: Track background processes spawned by agent
- Pattern: Thread-safe map of PID -> ProcessInfo
- Methods: Register, Terminate, KillAll, List

## Entry Points

**Main Binary (`cmd/aurago/main.go`):**
- Location: `cmd/aurago/main.go` (~930 lines)
- Triggers: Application startup
- Responsibilities:
  1. CLI flag parsing (--config, --debug, --setup, etc.)
  2. Secret loading (Docker secrets, .env files)
  3. Config validation and loading
  4. Database initialization (STM, inventory, invasion, credentials)
  5. Vault initialization
  6. LTM (VectorDB) initialization
  7. Knowledge Graph initialization
  8. Sandbox setup (Landlock on Linux)
  9. Cron manager startup
  10. Background task manager startup
  11. Server startup (HTTP/HTTPS)

**Server Run (`internal/server/server.go`):**
- Location: `internal/server/server.go:Start()`
- Triggers: After all subsystems initialized
- Responsibilities:
  1. HTTP/HTTPS server setup
  2. Route registration (60+ handler files)
  3. Auth middleware setup
  4. SSE broadcaster initialization
  5. Mission manager startup
  6. File indexer startup
  7. Auto-start integrations (Docker, MQTT, FritzBox, etc.)

## Error Handling

**Strategy:** Structured logging with slog, context-wrapped errors, graceful degradation.

**Patterns:**
- `fmt.Errorf("context: %w", err)` for error wrapping
- `log/slog` for structured logging with key-value pairs
- Feature flags disable failing subsystems (VectorDB disabled if embeddings fail)
- Background tasks recover from panics with deferred recovery
- Retry logic with exponential backoff for transient failures

**Graceful Degradation:**
- VectorDB disabled if embedding pipeline fails (app still functional)
- SQLite-only mode if VectorDB unavailable
- LLM failover to backup providers
- Sandbox fallback to direct Python execution

## Cross-Cutting Concerns

**Logging:** `log/slog` structured logging throughout. File logging optional via config. Web access logs separate from application logs.

**Validation:** Guardian (`internal/security/guardian.go`) scans tool outputs for sensitive data. LLM Guardian validates prompts. SSRF protection on HTTP requests.

**Authentication:** Session-based auth with hashed passwords in vault. Optional TOTP. Header-based internal auth for loopback requests.

**Configuration:** YAML-based with comprehensive defaults. Environment variable overrides. Provider reference resolution. Vault secret injection.

---

*Architecture analysis: 2026-04-03*

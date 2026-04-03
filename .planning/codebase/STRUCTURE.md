# Codebase Structure

**Analysis Date:** 2026-04-03

## Directory Layout

```
AuraGo/
├── cmd/                          # Application entry points
│   ├── aurago/                   # Main agent binary (~930 lines main.go)
│   ├── lifeboat/                 # Self-update companion binary
│   ├── remote/                   # Remote execution agent
│   └── config-merger/            # Configuration merging utility
├── internal/                     # Private application code
│   ├── agent/                    # Core agent loop, tool dispatch (30 files)
│   ├── llm/                      # LLM client, failover, retry, pricing
│   ├── memory/                   # STM (SQLite), LTM (VectorDB), KG
│   ├── server/                   # HTTP/HTTPS server, REST handlers (60+ files)
│   ├── tools/                    # 90+ tool implementations
│   ├── security/                 # Vault, guardian, scrubber
│   ├── config/                   # YAML config parsing & defaults
│   ├── inventory/                # SSH device inventory (SQLite)
│   ├── credentials/              # Credential management
│   ├── contacts/                 # Address book management
│   └── [20+ integration dirs]    # Discord, Telegram, FritzBox, etc.
├── ui/                           # Embedded Web UI (single-file SPA)
│   ├── embed.go                  # go:embed directives
│   ├── index.html                # Main chat UI
│   ├── config.html               # Configuration UI
│   ├── dashboard.html            # Dashboard UI
│   ├── css/, js/, lang/          # Styles, scripts, translations
│   └── [other .html files]       # Plans, missions, setup, etc.
├── prompts/                      # System prompt markdown files
│   ├── tools_manuals/            # Tool documentation (RAG-indexed)
│   ├── personalities/            # Personality profiles
│   └── templates/               # Prompt templates
├── agent_workspace/              # Runtime workspace
│   ├── skills/                   # Pre-built Python skills
│   ├── tools/                    # Agent-created tools + manifest
│   └── workdir/                  # Sandboxed execution directory (venv)
├── data/                         # Runtime data (git-ignored)
│   ├── aurago.db                 # Short-term memory SQLite
│   ├── vectordb/                 # Chromem-go vector database
│   ├── vault.bin                 # Encrypted secrets
│   └── [other runtime files]
├── config.yaml                   # Main configuration file
├── config_template.yaml          # Configuration template (~600 lines)
├── docker-compose.yml            # Docker Compose setup
└── [build scripts, Dockerfile, etc.]
```

## Directory Purposes

**`cmd/aurago/`:**
- Purpose: Main application entry point
- Contains: `main.go` (~930 lines) - all initialization logic
- Platform-specific: `platform_unix.go`, `platform_windows.go`

**`internal/agent/`:**
- Purpose: Core autonomous agent logic
- Contains:
  - `agent_loop.go` - main agent orchestration
  - `native_tools*.go` - built-in tool definitions and execution
  - `agent_dispatch_*.go` - tool call routing (exec, comm, infra, query_memory)
  - `memory_*.go` - memory ranking, consolidation, conflicts
  - `context_compression.go` - session context optimization
  - `coagent*.go` - co-agent management
  - `emotion_*.go` - emotional behavior system
  - `recovery_*.go` - error recovery policies

**`internal/server/`:**
- Purpose: HTTP/HTTPS server and REST API
- Contains:
  - `server.go` (~1100 lines) - main server setup and Start()
  - `server_routes.go` - route registration
  - `sse.go` - SSE broadcaster
  - `auth.go`, `auth_handlers.go` - authentication
  - `handlers_*.go` - 60+ handler files for specific features
  - `config_handlers*.go` - configuration API
  - `mission_v2_handlers.go` - mission management
  - `mcp_handlers.go` - Model Context Protocol

**`internal/memory/`:**
- Purpose: Multi-tier memory system
- Contains:
  - `short_term.go` - SQLite-backed sliding window
  - `long_term.go` - Chromem-go VectorDB
  - `graph_sqlite.go` - Knowledge Graph with FTS5
  - `history.go` - Persistent chat history
  - `personality*.go` - Personality memory
  - `journal*.go` - Journal entries
  - `notes*.go` - Notes system
  - `activity*.go` - Activity tracking
  - `consolidation*.go` - STM to LTM consolidation
  - `emotion_synthesizer.go` - Emotional memory synthesis

**`internal/tools/`:**
- Purpose: 90+ built-in tool implementations
- Contains:
  - `shell.go`, `python.go` - execution tools
  - `docker_*.go` - Docker management
  - `filesystem*.go` - file operations
  - `http*.go` - HTTP requests
  - `skill*.go` - Python skill management
  - `background_tasks*.go` - task scheduling
  - `cron*.go` - cron manager
  - `registry.go` - process registry
  - `manifest.go` - tool manifest
  - `sandbox*.go` - sandbox management
  - [50+ more tool files]

**`internal/security/`:**
- Purpose: Security and secret management
- Contains:
  - `vault.go` - AES-256-GCM encrypted vault
  - `guardian.go` - input validation
  - `llm_guardian.go` - LLM output validation
  - `scrubber.go` - sensitive data redaction
  - `tokens.go` - token manager
  - `ssrf.go` - SSRF protection

**`internal/config/`:**
- Purpose: Configuration management
- Contains:
  - `config.go` - loading and defaults
  - `config_types.go` - all config structs
  - `config_handlers.go` - config API handlers

**`internal/llm/`:**
- Purpose: LLM client management
- Contains:
  - `client.go` - main LLM interface
  - `failover.go` - multi-provider failover
  - `pricing.go` - token cost calculation

**`internal/inventory/`:**
- Purpose: SSH device inventory
- Contains: SQLite-backed device registry

**`internal/credentials/`:**
- Purpose: Credential storage and retrieval
- Contains: Inventory-backed credential management

**`ui/`:**
- Purpose: Embedded Web UI
- Contains:
  - `embed.go` - go:embed directives (all UI files)
  - `index.html` - main chat interface
  - `config.html` - configuration UI
  - `dashboard.html` - system dashboard
  - `setup.html` - first-time setup wizard
  - `login.html` - authentication
  - `css/` - stylesheets
  - `js/` - JavaScript modules (chat, setup, config)
  - `lang/` - i18n translations (15 languages)

**`prompts/`:**
- Purpose: System prompts and documentation for RAG
- Contains:
  - `tools_manuals/` - tool documentation (indexed for RAG)
  - `personalities/` - personality profiles
  - `templates/` - prompt templates

**`agent_workspace/`:**
- Purpose: Runtime workspace for agent execution
- Contains:
  - `workdir/` - Python virtual environment
  - `skills/` - pre-built Python skills
  - `tools/` - agent-created tools

## Key File Locations

**Entry Points:**
- `cmd/aurago/main.go`: Application bootstrap, all initialization

**Configuration:**
- `config.yaml`: Main configuration file
- `config_template.yaml`: Full configuration reference (~600 lines)
- `internal/config/config_types.go`: All config struct definitions
- `internal/config/config.go`: Config loading and defaults

**Core Agent Logic:**
- `internal/agent/agent_loop.go`: Main agent orchestration
- `internal/agent/native_tools_registry.go`: Tool registration
- `internal/agent/agent_dispatch_exec.go`: Tool execution dispatch

**Server:**
- `internal/server/server.go`: Server startup and main setup (~1100 lines)
- `internal/server/server_routes.go`: Route registration
- `internal/server/sse.go`: SSE broadcaster

**Memory:**
- `internal/memory/short_term.go`: SQLite STM
- `internal/memory/long_term.go`: Chromem VectorDB
- `internal/memory/graph_sqlite.go`: Knowledge Graph

**Security:**
- `internal/security/vault.go`: Encrypted vault
- `internal/security/guardian.go`: Input validation

**UI:**
- `ui/embed.go`: Embedded file system
- `ui/index.html`: Main chat UI

## Naming Conventions

**Files:**
- Go source: `snake_case.go` (e.g., `short_term.go`, `agent_loop.go`)
- Test files: `*_test.go` (e.g., `vault_test.go`)
- UI files: `kebab-case.html`, `camelCase.js`

**Directories:**
- Go packages: `lowercase` (e.g., `internal/agent`, `internal/server`)
- Integration dirs: `lowercase` (e.g., `internal/discord`, `internal/telegram`)

**Types/Functions:**
- Exported: `PascalCase` (e.g., `NewVault`, `Start`, `FailoverManager`)
- Unexported: `camelCase` (e.g., `vaultKey`, `processRegistry`)

**Constants:**
- Exported: `PascalCase` (e.g., `MaxOutputSize`)
- Unexported: `camelCase` (e.g., `defaultTimeout`)

## Where to Add New Code

**New Tool:**
1. Implementation: `internal/tools/your_tool.go`
2. Registration: `internal/agent/native_tools_registry.go`
3. Schema: `internal/agent/native_tools_*.go` (add to appropriate category)
4. Prompt manual: `prompts/tools_manuals/your_tool.md`
5. Tests: `internal/tools/your_tool_test.go`

**New Integration (e.g., new messaging platform):**
1. Create package: `internal/your_integration/`
2. Config types: `internal/config/config_types.go`
3. Config loading: `internal/config/config.go`
4. Server handlers: `internal/server/your_integration_handlers.go`
5. Tool definitions if needed: `internal/agent/native_tools_your_integration.go`
6. UI elements: `ui/js/`, `ui/lang/` (all 15 languages)

**New Memory Type:**
1. Primary file: `internal/memory/your_memory.go`
2. Integration with consolidation: `internal/memory/consolidation.go`
3. Tests: `internal/memory/your_memory_test.go`

**New REST Endpoint:**
1. Handler: `internal/server/handlers_your_feature.go`
2. Route registration: `internal/server/server_routes.go`
3. Auth check if required

## Special Directories

**`data/`:**
- Purpose: Runtime data (SQLite DBs, VectorDB, vault)
- Generated: Yes
- Committed: No (git-ignored)
- Contains: `aurago.db`, `vectordb/`, `vault.bin`, `chat_history.json`, `knowledge_graph.db`

**`agent_workspace/`:**
- Purpose: Agent runtime workspace
- Generated: Yes
- Committed: No (git-ignored)
- Contains: Python venv, skills, agent-created tools

**`ui/` (embedded via go:embed):**
- Purpose: Web UI bundled into binary
- Generated: No (compiled separately)
- Committed: Yes
- Embedded via: `ui/embed.go`

**`prompts/tools_manuals/`:**
- Purpose: Tool documentation for RAG indexing
- Generated: No (manually maintained)
- Committed: Yes
- Indexed at: Startup and on file change

**`reports/`:**
- Purpose: Analysis reports with sensitive data
- Generated: Yes
- Committed: No (git-ignored)
- Note: Never commit reports containing secrets

---

*Structure analysis: 2026-04-03*

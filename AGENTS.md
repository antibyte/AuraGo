# AuraGo - AI Coding Agent Reference

> **Language**: This project primarily uses **English** for code and documentation. The default system language is set to German (`Deutsch`) in config but English is the development language.

## Project Overview

**AuraGo** is a fully autonomous AI agent written in Go, designed for home lab environments. It ships as a single portable binary with an embedded Web UI and has zero external dependencies for the core functionality.

### Key Characteristics
- **Single binary deployment** - Pure Go with embedded SQLite (no CGO)
- **Self-contained** - Web UI baked in via `go:embed`
- **Home lab focused** - Docker, Proxmox, Home Assistant, SSH device management, and 50+ integrations
- **Multi-platform** - Linux, macOS, Windows (amd64, arm64)
- **50+ built-in tools** - Shell, Python execution, file system, HTTP requests, cron, and many more

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.26.1+ |
| Web Framework | Standard library `net/http` with gorilla/mux patterns |
| Database | SQLite (modernc.org/sqlite - pure Go, no CGO) |
| Vector DB | chromem-go (embedded) |
| Frontend | Vanilla JavaScript SPA (embedded via go:embed) |
| Python Runtime | Python 3.10+ (for sandboxed execution in venv) |
| Container | Docker, Docker Compose |

### Key Dependencies
- `github.com/sashabaranov/go-openai` - OpenAI-compatible LLM client
- `github.com/philippgille/chromem-go` - Embedded vector database
- `modernc.org/sqlite` - Pure Go SQLite driver
- `github.com/go-telegram-bot-api/telegram-bot-api/v5` - Telegram bot
- `github.com/bwmarrin/discordgo` - Discord integration
- `github.com/robfig/cron/v3` - Cron scheduler
- `golang.org/x/crypto` - SSH client, bcrypt, ACME/Let's Encrypt
- `github.com/gofrs/flock` - File-based locking
- `tailscale.com` - Tailscale VPN integration
- `github.com/aws/aws-sdk-go-v2` - AWS S3 SDK

## Project Structure

```
AuraGo/
├── cmd/                          # Application entry points
│   ├── aurago/                   # Main agent binary
│   │   ├── main.go               # Entry point with full initialization (~860 lines)
│   │   ├── platform_unix.go      # Unix-specific code
│   │   └── platform_windows.go   # Windows-specific code
│   ├── remote/                   # Remote execution agent
│   └── config-merger/            # Configuration merging utility
├── internal/                     # Private application code
│   ├── agent/                    # Core agent loop, tool dispatch, co-agents (30 files)
│   ├── budget/                   # Token cost tracking
│   ├── commands/                 # Slash commands (/reset, /budget, etc.)
│   ├── config/                   # YAML config parsing & defaults
│   ├── contacts/                 # Address book / contacts management
│   ├── discord/                  # Discord bot integration
│   ├── fritzbox/                 # Fritz!Box TR-064 integration
│   ├── invasion/                 # Invasion Control (egg/nest distributed system)
│   ├── inventory/                # SSH device inventory (SQLite)
│   ├── llm/                      # LLM client, failover, retry, pricing
│   ├── logger/                   # Structured logging setup
│   ├── media/                    # Media file handling
│   ├── memory/                   # STM, LTM, knowledge graph, personality
│   ├── meshcentral/              # MeshCentral remote desktop integration
│   ├── mqtt/                     # MQTT client integration
│   ├── prompts/                  # Dynamic system prompt builder
│   ├── remote/                   # SSH remote execution and protocol
│   ├── rocketchat/               # Rocket.Chat bot integration
│   ├── sandbox/                  # Sandboxed execution (Landlock on Linux)
│   ├── scraper/                  # Web scraping utilities
│   ├── security/                 # AES-GCM vault & token manager, LLM Guardian
│   ├── server/                   # HTTP/HTTPS server, REST handlers (60+ files)
│   ├── services/                 # Background services (indexer, ingestion)
│   ├── setup/                    # First-time setup wizard
│   ├── sqlconnections/           # External SQL database connections
│   ├── telegram/                 # Telegram bot (text, voice, vision)
│   ├── telnyx/                   # Telnyx SMS/voice integration
│   ├── tools/                    # All tool implementations (90+ files)
│   ├── tsnetnode/                # Tailscale tsnet embedded node
│   └── webhooks/                 # Incoming & outgoing webhooks
├── agent_workspace/              # Runtime workspace
│   ├── skills/                   # Pre-built Python skills
│   ├── tools/                    # Agent-created tools + manifest
│   └── workdir/                  # Sandboxed execution directory (venv)
├── prompts/                      # System prompt markdown files
│   ├── identity.md               # Core identity prompt
│   ├── rules.md                  # Agent behavior rules
│   ├── personalities/            # Personality profiles
│   ├── templates/                # Prompt templates
│   └── tools_manuals/            # Tool documentation for RAG
├── ui/                           # Embedded Web UI (single-file SPA)
│   ├── *.html                    # Page templates (index.html, config.html, etc.)
│   ├── css/                      # Stylesheets
│   ├── js/                       # JavaScript modules
│   ├── lang/                     # i18n translations (15 languages)
│   └── embed.go                  # go:embed directives
├── data/                         # Runtime data (databases, vault, state)
├── documentation/                # Detailed setup guides
├── bin/                          # Compiled binaries (git-ignored)
├── deploy/                       # Deployment artifacts (git-ignored)
├── reports/                      # Analysis reports (git-ignored, do not commit)
├── config.yaml                   # Main configuration file
├── config_template.yaml          # Configuration template (~600 lines)
├── Dockerfile                    # Multi-stage build
├── docker-compose.yml            # Docker Compose setup with sidecars
├── Dockerfile.ansible            # Ansible sidecar image
├── install.sh                    # Quick installer script
├── update.sh                     # Self-update script
├── make_deploy.sh                # Build script for Linux/macOS
└── make_release.bat              # Build script for Windows
```

## Build Commands

### Development Build
```bash
# Build main binary (requires Go 1.26+)
go build -o aurago ./cmd/aurago

# Build and start locally
./start.sh

# Build all binaries
mkdir -p bin
go build -o bin/aurago ./cmd/aurago
go build -o bin/aurago-remote ./cmd/remote
go build -o bin/config-merger ./cmd/config-merger
```

### Production Build
```bash
# Cross-compile for all platforms (Linux/macOS)
./make_deploy.sh

# Cross-compile for all platforms (Windows)
make_release.bat

# Individual platform build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aurago ./cmd/aurago
```

### Docker Build
```bash
# Build image
docker build -t aurago:latest .

# Or use docker-compose (recommended)
docker-compose up -d

# View logs
docker-compose logs -f aurago
```

### Test Commands
```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/config/...
go test ./internal/agent/...
go test ./internal/memory/...

# Run with verbose output
go test -v ./internal/tools/...

# Run with coverage
go test -cover ./internal/...

# Race detection
go test -race ./...

# Benchmarks
go test -bench=. ./internal/...
```

## Configuration

### Required Minimum Configuration
```yaml
providers:
  - id: main
    type: openrouter
    name: "Haupt-LLM"
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-..."  # Your API key
    model: "google/gemini-2.0-flash-001"

llm:
  provider: main
```

### Environment Variables
| Variable | Purpose |
|----------|---------|
| `AURAGO_MASTER_KEY` | 64-character hex key for vault encryption (32 bytes) |
| `AURAGO_SERVER_HOST` | Override server bind address (Docker: `0.0.0.0`) |
| `LLM_API_KEY` | Override LLM API key |
| `OPENAI_API_KEY` | Alternative LLM API key |
| `TAILSCALE_API_KEY` | Tailscale integration |
| `ANSIBLE_API_TOKEN` | Ansible sidecar authentication |

### Security Note
API keys in `config.yaml` are NEVER exposed to the agent. They are managed by the application. Use the **Vault** via Web UI for storing sensitive credentials.

## Code Style Guidelines

### Go Code Standards
1. **Error handling** - Always wrap errors with context: `fmt.Errorf("context: %w", err)`
2. **Logging** - Use structured logging with `slog`: `log.Info("message", "key", value)`
3. **Comments** - Use English for all code comments
4. **Package naming** - Short, lowercase, no underscores
5. **File organization** - One responsibility per file

### Naming Conventions
- **Files**: `snake_case.go` for multi-word files
- **Types**: `PascalCase` (exported), `camelCase` (unexported)
- **Constants**: `PascalCase` for exported, `camelCase` for unexported
- **Functions**: `PascalCase` (exported), `camelCase` (unexported)
- **Variables**: `camelCase`

### Example Pattern
```go
// File: internal/tools/docker.go
package tools

import (
    "context"
    "fmt"
    "log/slog"
)

// DockerManager handles Docker container operations
type DockerManager struct {
    client DockerClient
    logger *slog.Logger
}

// NewDockerManager creates a new Docker manager instance
func NewDockerManager(client DockerClient, logger *slog.Logger) (*DockerManager, error) {
    if client == nil {
        return nil, fmt.Errorf("docker client is required")
    }
    return &DockerManager{
        client: client,
        logger: logger,
    }, nil
}

// ListContainers returns all running containers
func (m *DockerManager) ListContainers(ctx context.Context) ([]Container, error) {
    // Implementation
}
```

## Testing Strategy

### Test Organization
- Test files: `*_test.go` alongside source files
- Test functions: `TestFunctionName` for unit tests
- Table-driven tests preferred

### Running Tests
```bash
# All tests
go test ./...

# Specific package with coverage
go test -cover ./internal/memory/...

# Race detection
go test -race ./...

# Benchmarks
go test -bench=. ./internal/...
```

### Test Examples
See existing test files:
- `internal/config/config_test.go` - Configuration testing
- `internal/tools/shell_test.go` - Tool testing
- `internal/memory/history_test.go` - Memory subsystem testing
- `internal/agent/agent_test.go` - Agent loop testing

## Security Considerations

### Vault System
- AES-256-GCM encryption for all secrets
- Master key (64 hex chars = 32 bytes) required at startup via `AURAGO_MASTER_KEY`
- Vault file: `data/vault.bin`
- Never commit `.env` or vault files

### Danger Zone Capabilities
All potentially dangerous operations are gated via config:
- `sudo_enabled` - Sudo command execution
- `sudo_unrestricted` - Sudo writes outside the install directory (requires removing `ProtectSystem=strict` from the systemd unit)
- `allow_shell` - Shell command execution
- `allow_python` - Python code execution
- `allow_filesystem_write` - File write operations
- `allow_network_requests` - HTTP requests
- `allow_remote_shell` - SSH to remote devices
- `allow_self_update` - Binary self-updates
- `allow_mcp` - Model Context Protocol
- `allow_web_scraper` - Web scraping

### Sensitive Data Scrubbing
Use `security.RegisterSensitive(value)` to prevent values from appearing in logs or LLM outputs.

### Agent Reports & Analysis Files

**CRITICAL:** When creating analysis reports, logs, or any files that may contain sensitive data:

1. **Create reports in `/reports/` directory** (NOT in `documentation/`)
2. **The `/reports/` directory is in `.gitignore`** - files here are never committed
3. **Never commit files containing:**
   - Master keys or vault secrets
   - API keys or tokens
   - Passwords or credentials
   - Log files with sensitive output
   - Memory dumps or conversation history

**Correct workflow:**
```bash
# Good: Report in non-versioned directory
reports/log_analysis_2026-03-15.md

# Bad: Report in versioned directory
documentation/log_analysis_2026-03-15.md  # DON'T DO THIS
```

**Before committing, always check:**
```bash
git diff --cached  # Review all staged changes
grep -r "AURAGO_MASTER_KEY\|sk-or-\|password\|secret" .  # Scan for secrets
```

**If you accidentally committed sensitive data:**
1. Immediately rotate/change the exposed secret
2. Use `git filter-branch` or BFG Repo-Cleaner to remove from history
3. Force push to overwrite (coordinate with team)
4. Assume the secret is compromised

## Deployment

### Docker Deployment (Recommended)
```bash
# Using pre-built image
docker-compose up -d

# With custom config
docker-compose -f docker-compose.yml up -d
```

### Binary Installation
```bash
# Quick install (Linux)
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash

# Manual binary download
wget https://github.com/antibyte/AuraGo/releases/latest/download/aurago_linux_amd64
chmod +x aurago_linux_amd64
./aurago_linux_amd64
```

### Systemd Service
```bash
sudo ./install_service_linux.sh
```

## Key Architecture Patterns

### Agent Loop
The core agent loop (`internal/agent/agent_loop.go`) implements:
1. Message reception
2. LLM interaction with native function calling
3. Tool dispatch and execution
4. Response streaming via SSE
5. Error recovery and retry logic

### Memory System
- **Short-Term**: SQLite sliding-window conversation context
- **Long-Term**: Vector database with semantic search (chromem-go)
- **Knowledge Graph**: Entity-relationship store for structured facts
- **Core Memory**: Permanent facts always included in context
- **Native Chunking**: File, documentation, and tool-guide indexing use the Go `internal/chunking` package. `indexing.chunking` defaults to recursive chunking with 3,500 chars, 200 overlap, and 200 chunks per file; chunking parameters are part of index fingerprints so config changes trigger clean reindexing.
- **On-Demand Context**: Auto-RAG and KG prompt injection keep only essential context in the prompt and expose additional `[memory:<id>]` / `[kg:<id>]` teasers for `recall_memory` and `explore_kg`.
- **Memory Hygiene**: Dashboard and nightly maintenance can safely consolidate exact auto-generated journal error duplicates, archive stale low-priority notes with per-run limits and repeated-failure tracking, repair tracked canonical VectorDB names, and raise review issues for KG/Core Memory health. Notes marked `protected` or `keep_forever` are excluded from auto-archive. Review-only/high-risk memory findings must not be auto-deleted.

### Tool System
Tools are defined in `internal/tools/`:
- Each tool has a JSON schema definition
- Tools are registered in the tool registry
- Native OpenAI function calling format
- Dynamic tool creation supported (agent writes Python tools)

### GitHub Integration Contract
- `github.allowed_repos` is a strict allowlist; prefer `owner/repo` entries. Legacy bare repo names only match the configured `github.owner`.
- An empty `github.allowed_repos` list permits only repositories AuraGo created through the GitHub tool and tracks with `agent_created=true`.
- Manual `track_project` entries are local inventory only and must never grant remote repository access.

### Workspace Search System
- `internal/services.WorkspaceSearchService` maintains a Pure Go resident index for the full agent workspace derived from `directories.workspace_dir`; it must stay single-binary friendly with no CGO, mmap, FFI, or fsnotify dependency.
- The native `workspace_search` tool exposes `find`, `grep`, `glob`, `recent`, `rescan`, and `status`. Keep legacy `file_search` JSON shapes compatible when delegating to the resident index.
- Do not persist file content for workspace search. Only frecency/access metadata belongs in `data/workspace_search.db`.

### Homepage Managed Website Ledger
- Managed homepage/web projects use `data/homepage_registry.db` as the system of record for project identity, local file state, structured events, revision links, deployment targets, deployment history, remote observations, and drift status.
- Homepage project identity is the `project_dir` relative to `homepage.workspace_path`; avoid storing absolute workspace paths as the canonical project key.
- Mutating homepage operations must keep the ledger current by recording structured events and, when files change, revisions plus file-state snapshots. Remote deploys must be linked to provider IDs/URLs and build artifact hashes when available.
- Server APIs under `/api/homepage/sites` expose the managed-site read model, including detail `deploy_targets` and `remote_observations`, plus the reconciliation path. Keep these APIs additive and compatible with existing `/api/homepage/history`.

### Server Architecture
- Single HTTP server with SSE for streaming
- RESTful API under `/api/`
- Web UI served from embedded files
- TLS/HTTPS via Let's Encrypt (automated)

## Development Workflow

### Adding a New Tool
1. Create tool implementation in `internal/tools/your_tool.go`
2. Add tool definition/registration
3. Add prompt manual in `prompts/tools_manuals/your_tool.md`
4. Update tool registry if needed
5. Add tests in `internal/tools/your_tool_test.go`

### Adding a New Integration
1. Create package in `internal/your_integration/`
2. Implement client/service logic
3. Add config types to `internal/config/config_types.go`
4. Add config loading defaults in `internal/config/config.go`
5. Add Web UI handlers in `internal/server/` if needed
6. Add translations for all 15 supported languages in `ui/lang/` Never only use english for all languages !
7. Document in `documentation/`

### Database Migrations
- SQLite migrations are handled automatically on startup
- Schema changes should be backward compatible
- New DB files auto-initialize with current schema

## Common Development Tasks

### Reset Development Environment
```bash
./kill_all.sh
rm -rf data/*.db data/vectordb/* agent_workspace/workdir/venv
rm -f data/aurago.lock data/maintenance.lock
```

### Regenerate Master Key
```bash
# Linux/macOS
export AURAGO_MASTER_KEY="$(openssl rand -hex 32)"
echo "AURAGO_MASTER_KEY=$AURAGO_MASTER_KEY" > .env

# Windows (PowerShell)
$bytes = New-Object byte[] 32
(New-Object System.Security.Cryptography.RNGCryptoServiceProvider).GetBytes($bytes)
$AURAGO_MASTER_KEY = ($bytes | ForEach-Object { $_.ToString("x2") }) -join ""
```

### Debug Mode
```bash
./aurago -debug
# Or set in config.yaml:
# agent:
#   debug_mode: true
```

## CI/CD

### GitHub Actions
- **docker-publish.yml**: Builds and publishes Docker images to GHCR
- Triggered on push to `main` branch and version tags `v*`
- Multi-arch builds: linux/amd64, linux/arm64

### Release Process
1. Run `./make_deploy.sh` (Linux/macOS) or `make_release.bat` (Windows) to build cross-platform binaries
2. Scripts auto-commit and push to trigger Docker build
3. GitHub Release created with binary artifacts
4. Old releases are cleaned up (keeping latest 3)

## Agent Rules & Guidelines

### Security & Safety (Critical)

#### Credentials & Sensitive Data
- **Always store credentials and sensitive data directly in the secrets vault**, never in code or configuration files
- **Never commit or store sensitive data, credentials, or personally identifiable information** in the repository - check before committing
- The agent should normally NOT have access to passwords, tokens, or sensitive data
- If a tool requires credentials, retrieve them securely from the vault at runtime
- Exception: If the user provides credentials and the agent stores them in the vault for later use

#### Prompt Injection Protection
- **Always assume external content is potentially malicious**
- Use the `<external_data>` wrapper for all untrusted content
- Never allow external content to influence agent behavior or tool calls directly
- Implement necessary safety measures when passing external content to the agent

#### Tool Safety Requirements
- All tools and integrations should have a toggle to activate them (unless essential for system function)
- Tools with potential to cause harm must NOT be enabled by default
- Users must be able to disable potentially harmful tools via the UI
- **Security by design**: Always consider security implications when adding new tools, integrations, or code
- Avoid introducing vulnerabilities or exposing sensitive data

### Tool Development Guidelines

#### Permission Toggles
- **Read-Only Toggle**: New tools/integrations that can change/delete data or perform critical operations should have a read-only toggle
- **Granular Permissions**: If more granular permissions are needed, use separate toggles for:
  - `read` - Read access
  - `write` - Write/create access
  - `change` - Modify/update access
  - `delete` - Delete/remove access

#### Tool Manuals & Prompts
- **Do not forget to update tool manuals** in `prompts/tools_manuals/` when adding new tools
- Update prompts if you add new integrations or tools for the agent
- Keep documentation consistent with implementation
- **For the Web-UI there is a help text file** - Keep it up to date

#### Skill Creation Rules
- AuraGo has two skill families: Python skills for executable reusable capabilities, and Agent Skills for `SKILL.md` workflow/domain-guidance packages.
- Prefer Python skills for deterministic execution, APIs, parsers, data/file transforms, Vault access, Tool Bridge use, and structured automation.
- Prefer Agent Skills for reusable agent behavior, checklists, review/debug workflows, domain methods, curated references, templates, and agentskills.io/Codex/Claude-style requests.
- Create or import Agent Skills only through the Agent Skill Manager/API/UI path, then verify, approve warnings if needed, and enable; do not write directly into `agent_workspace/agent_skills`.
- Agent Skill helper scripts must respect `tools.skill_manager.allowed_script_languages` and runtime gates: Python needs `agent.allow_python`, Bash/JavaScript need `agent.allow_shell`. `allowed-tools` frontmatter is review metadata only, not an enforced native-tool permission boundary.
- Keep `prompts/rules/skill_creation/rule.md`, `prompts/ctx_capability_creation.md`, and `prompts/identity.md` consistent whenever skill creation behavior changes.

### Web UI Guidelines

#### UX Design Principles
- **User-friendly by default**: Avoid technical jargon, provide clear instructions and feedback
- Do not break the style of the UI - changes should fit seamlessly into the existing interface
- Aim for **masterpiece UX design** that feels native to the existing interface
- If you see bad UX in the existing UI, feel free to improve it while keeping overall style consistent
- Virtual Desktop themes have distinct visual contracts: `fruity` should read as Apple-inspired with WhiteSur icons, topbar, floating dock, and soft window chrome; `standard` should read as a Windows/Ubuntu productivity crossover with Papirus icons, clear taskbar, structured start menu, and restrained dark surfaces.
- **Always check if new features are relevant to the dashboard** and add them there if applicable
- **User-friendly system design**: Always think ahead for the user and also add test connection buttons if this could help the user to diagnose issues with new tools or integrations
- The system should be designed to be as user-friendly and intuitive as possible, with clear instructions and feedback for the user. Always consider the user experience when designing and implementing new features and tools for the agent

#### Translations
- **Always update translation files** in `ui/lang/` for **ALL supported languages** (15 languages: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh) Never only use english for all languages !
- Keep translations up to date and consistent with UI changes
- If you add new features requiring new UI elements, provide translations for all supported languages
- use the personal form if writing text eg. "Du" in german and not "Sie"
- never use an alternative form for special characters like öäü, do not write ae ue oe

#### Form Design
- **If a field has options to choose, provide a dropdown**, not a text input field
- **Fields that have default values should show those**, or be empty if a remark in the describing text states that default value X is used if field is empty
- **Always create easy to use menus**

### Code Organization & Quality

#### File Management
- **Keep files manageable**: If files get too big and unwieldy, split them into smaller pieces
- Orient yourself on what an AI agent can handle with ease
- If a file becomes too large to process effectively, break it down into smaller, more manageable files
- Always aim for clarity and maintainability in your file structure

#### Temporary Files
- **Always cleanup temporary files and logs** after use
- Don't leave behind orphaned temporary resources

#### UI Components
- **Do not use `alert()`**, use a modal instead
- **All LLMs that can be chosen must use the provider system**

### Testing & Quality Assurance

#### Testing Requirements
- **Implement tests for critical functionality and new features**
- Include unit tests for individual functions
- Include integration tests for tools and workflows
- Aim for good test coverage, especially for complex logic and critical operations
- Tests help prevent regressions and ensure code works as expected

### Database Management

#### Schema Changes
- **Always implement a migration strategy** when changing database schemas
- Handle existing data properly during migrations
- Backup the database before performing migrations
- Test the migration process in a staging environment before applying to production

### Deployment & Maintenance

#### Critical Files to Keep Updated
- **`config.yaml` is holy**: No updates without careful consideration of implications
- Keep **update scripts**, **install scripts**, and **Dockerfiles** up to date
- Ensure consistency between system changes and deployment scripts
- If you add new tools/integrations requiring installation changes, update relevant scripts accordingly
- **All Docker containers are created and managed by the AuraGo backend fully automatically** - Never assume the user could manage this
- The default `docker-compose.yml` Docker socket proxy must keep Docker build API access disabled (`BUILD=0`); managed Code Studio and sidecar flows should use published images with `IMAGES=1` and `POST=1` instead of requiring build permissions.

#### Build Process
- **Use `make_deploy` script** to build binaries and upload to test server
- Don't use manual build commands for production builds

#### Vault Integration
- **If you add a new tool or integration that uses the vault, add it to the list of secrets that are forbidden to be exported to Python tools!**
- Always ensure sensitive data is properly protected from exposure to the agent environment

## Resources

- **README.md** - User-facing documentation
- **documentation/** - Detailed guides
- **config_template.yaml** - Full configuration reference (~600 lines)
- **prompts/tools_manuals/** - Tool documentation (RAG-indexed)
- **ui/lang/** - Translation files for 15 languages

COMMIT CHANGES !! Always commit your changes with clear and descriptive commit messages. This helps keep the project history organized and makes it easier for others to understand the changes you have made. Regularly committing your changes also helps prevent data loss and allows you to track your progress effectively.

ALWAYS USE THE disposable FOLDER FOR SCRIPTS AND OTHER FILES YOU NEED FOR YOUR WORK !  This folder is NOT to be pushed to github !

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **AuraGo** (56291 symbols, 227650 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({search_query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/AuraGo/context` | Codebase overview, check index freshness |
| `gitnexus://repo/AuraGo/clusters` | All functional areas |
| `gitnexus://repo/AuraGo/processes` | All execution flows |
| `gitnexus://repo/AuraGo/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->

# DOX framework

- DOX is highly performant AGENTS.md hierarchy installed here
- Agent must follow DOX instructions across any edits

## Core Contract

- AGENTS.md files are binding work contracts for their subtrees
- Work products, source materials, instructions, records, assets, and durable docs must stay understandable from the nearest applicable AGENTS.md plus every parent AGENTS.md above it

## Read Before Editing

1. Read the root AGENTS.md
2. Identify every file or folder you expect to touch
3. Walk from the repository root to each target path
4. Read every AGENTS.md found along each route
5. If a parent AGENTS.md lists a child AGENTS.md whose scope contains the path, read that child and continue from there
6. Use the nearest AGENTS.md as the local contract and parent docs for repo-wide rules
7. If docs conflict, the closer doc controls local work details, but no child doc may weaken DOX

Do not rely on memory. Re-read the applicable DOX chain in the current session before editing.

## Update After Editing

Every meaningful change requires a DOX pass before the task is done.

Update the closest owning AGENTS.md when a change affects:

- purpose, scope, ownership, or responsibilities
- durable structure, contracts, workflows, or operating rules
- required inputs, outputs, permissions, constraints, side effects, or artifacts
- user preferences about behavior, communication, process, organization, or quality
- AGENTS.md creation, deletion, move, rename, or index contents

Update parent docs when parent-level structure, ownership, workflow, or child index changes. Update child docs when parent changes alter local rules. Remove stale or contradictory text immediately. Small edits that do not change behavior or contracts may leave docs unchanged, but the DOX pass still must happen.

## Hierarchy

- Root AGENTS.md is the DOX rail: project-wide instructions, global preferences, durable workflow rules, and the top-level Child DOX Index
- Child AGENTS.md files own domain-specific instructions and their own Child DOX Index
- Each parent explains what its direct children cover and what stays owned by the parent
- The closer a doc is to the work, the more specific and practical it must be

## Child Doc Shape

- Create a child AGENTS.md when a folder becomes a durable boundary with its own purpose, rules, responsibilities, workflow, materials, or quality standards
- Work Guidance must reflect the current standards of the project or user instructions; if there are no specific standards or instructions yet, leave it empty
- Verification must reflect an existing check; if no verification framework exists yet, leave it empty and update it when one exists

Default section order:
- Purpose
- Ownership
- Local Contracts
- Work Guidance
- Verification
- Child DOX Index

## Style

- Keep docs concise, current, and operational
- Document stable contracts, not diary entries
- Put broad rules in parent docs and concrete details in child docs
- Prefer direct bullets with explicit names
- Do not duplicate rules across many files unless each scope needs a local version
- Delete stale notes instead of explaining history
- Trim obvious statements, repeated rules, misplaced detail, and warnings for risks that no longer exist

## Closeout

1. Re-check changed paths against the DOX chain
2. Update nearest owning docs and any affected parents or children
3. Refresh every affected Child DOX Index
4. Remove stale or contradictory text
5. Run existing verification when relevant
6. Report any docs intentionally left unchanged and why

## User Preferences

When the user requests a durable behavior change, record it here or in the relevant child AGENTS.md

## Child DOX Index

Current child AGENTS.md files:
- `ui/js/desktop/apps/AGENTS.md` — Built-in desktop app modules for Galaxa Deluxe and Chess (lazy app assets, per-window lifecycle, game-specific contracts).

The root AGENTS.md owns the whole repository except where a subtree has its own local contract.

Top-level durable areas:
- `.github/` - GitHub Actions, agents, prompts, and repository automation metadata.
- `agent_workspace/` - Runtime agent workspace, bundled skills, tool manifests, and sandbox workdir assets.
- `ansible_api/` - Ansible sidecar API implementation.
- `assets/` - Bundled static and sample assets used by release packaging and runtime features.
- `browser_automation_sidecar/` - Browser automation sidecar source and support files.
- `cmd/` - Go entry points for AuraGo, remote agent, and config merger binaries.
- `deploy/` - Deployment and release packaging inputs.
- `docs/` and `documentation/` - User and operator documentation.
- `internal/` - Private Go application packages and production logic.
- `knowledge/` - Knowledge assets consumed by the application.
- `Linux/`, `macOS/`, `Windows/` - Platform-specific release support assets.
- `mcps/` - MCP connector/tool definitions.
- `plans/` and `openspec/` - Planning, specification, and change-management artifacts.
- `prompts/` - Agent prompts, templates, personalities, and tool manuals.
- `scripts/` and `tools/` - Developer and runtime helper tooling.
- `ui/` - Embedded Web UI HTML, CSS, JavaScript, translations, and UI tests.

Ignored/runtime areas such as `bin/`, `data/`, `reports/`, `node_modules/`, `.venv/`, `.worktrees/`, and `terminals/` are not child DOX owners.

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
│   ├── lifeboat/                 # Self-update companion binary
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
│   ├── lifeboat.md               # Self-update system prompt
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

# Build with lifeboat (recommended for development)
./start.sh

# Build all binaries
mkdir -p bin
go build -o bin/aurago ./cmd/aurago
go build -o bin/lifeboat ./cmd/lifeboat
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

### Git Protocol (CRITICAL)

**FORBIDDEN operations that will corrupt repository history:**
- **`git commit --amend` on already-pushed commits** - This rewrites history and forces a subsequent `git push --force`, which can resurrect ignored files and corrupt shared repository state
- **`git rebase` on already-pushed commits** - Same reason as above
- **`git reset --hard` on a public branch** - This discards commits that others may depend on

**Why these are dangerous:**
- Amending a pushed commit replaces it with a new one, making the old commit orphaned
- A force-push to overwrite the orphaned commit brings back old tracked files
- Files previously in `.gitignore` but tracked in history can reappear
- Other collaborators' local repositories become inconsistent

**If you need to fix a pushed commit:**
1. Create a new commit with the correction (don't amend)
2. Push the new commit normally
3. If the original commit was bad, document in the commit message that it's superseded

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

### Tool System
Tools are defined in `internal/tools/`:
- Each tool has a JSON schema definition
- Tools are registered in the tool registry
- Native OpenAI function calling format
- Dynamic tool creation supported (agent writes Python tools)

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
6. Add translations for all 15 supported languages in `ui/lang/`
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

### Web UI Guidelines

#### UX Design Principles
- **User-friendly by default**: Avoid technical jargon, provide clear instructions and feedback
- Do not break the style of the UI - changes should fit seamlessly into the existing interface
- Aim for **masterpiece UX design** that feels native to the existing interface
- If you see bad UX in the existing UI, feel free to improve it while keeping overall style consistent
- **Always check if new features are relevant to the dashboard** and add them there if applicable
- **User-friendly system design**: Always think ahead for the user and also add test connection buttons if this could help the user to diagnose issues with new tools or integrations
- The system should be designed to be as user-friendly and intuitive as possible, with clear instructions and feedback for the user. Always consider the user experience when designing and implementing new features and tools for the agent

#### Translations
- **Always update translation files** in `ui/lang/` for **ALL supported languages** (15 languages: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh)
- Keep translations up to date and consistent with UI changes
- If you add new features requiring new UI elements, provide translations for all supported languages

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

<!-- GSD:project-start source:PROJECT.md -->
## Project

**AuraGo UI/UX Overhaul**

Systematic improvement of AuraGo's embedded Web UI — fixing layout issues, achieving visual consistency across all pages, and ensuring complete internationalization. The goal is a polished, professional interface that feels cohesive across all areas (Setup, Chat, Dashboard, Config, Missions, etc.).

**Core Value:** Every page must be usable, consistent, and translated — no half-finished sections, no orphaned UI elements, no language gaps.

### Constraints

- **Tech**: Vanilla JS SPA, CSS custom properties (CSS variables), no framework changes
- **Compatibility**: Must maintain dark/light theme support
- **Scope**: Only frontend/UI files — no backend changes
- **Languages**: All 15 languages must have complete, correct translations
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- **Go 1.26.1** - Core application language for all backend services, agent loop, integrations
- **JavaScript** - Frontend SPA (vanilla JS, no framework)
- **Python 3.12** - Sandboxed skill/tool execution via embedded venv
## Runtime
- Go 1.26.1 (build target)
- Python 3.12-slim (runtime in Docker)
- CGO disabled for pure Go cross-compilation
- Go modules (`go.mod` / `go.sum`)
- No external package registry dependencies beyond go.mod
## Frameworks
- Standard library `net/http` with custom routing (not using gorilla/mux despite CLAUDE.md mentioning it)
- `github.com/gorilla/websocket v1.5.4` - WebSocket support
- `github.com/sashabaranov/go-openai v1.41.2` - OpenAI-compatible LLM client
- `modernc.org/sqlite v1.28.0` - Pure Go SQLite driver (no CGO)
- `github.com/philippgille/chromem-go v0.7.0` - Embedded vector database for semantic memory
- Go standard `testing` package
- `go test ./...` for all tests
- Docker multi-stage build (`Dockerfile`)
- Go cross-compilation with `GOOS`/`GOARCH`/`CGO_ENABLED=0`
## Key Dependencies
- `github.com/sashabaranov/go-openai v1.41.2` - LLM client for OpenAI-compatible APIs
- `modernc.org/sqlite v1.28.0` - All SQLite databases (memory, inventory, etc.)
- `github.com/philippgille/chromem-go v0.7.0` - Vector DB for long-term memory
- `github.com/bwmarrin/discordgo v0.29.0` - Discord bot
- `github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1` - Telegram bot
- `github.com/eclipse/paho.mqtt.golang v1.5.1` - MQTT client
- `golang.org/x/crypto` - SSH client, bcrypt, ACME/Let's Encrypt
- `github.com/pkg/sftp v1.13.10` - SFTP for remote file transfers
- `tailscale.com v1.96.1` - Tailscale VPN integration
- `github.com/aws/aws-sdk-go-v2` - AWS S3 SDK
- `github.com/robfig/cron/v3 v3.0.1` - Cron scheduler
- `github.com/go-rod/rod v0.116.2` - Headless Chrome for web scraping/screenshots
- `github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728` - PDF parsing
- `github.com/johnfercher/maroto/v2 v2.3.4` - PDF generation (Maroto)
- `github.com/pdfcpu/pdfcpu v0.11.1` - PDF processing
- `github.com/charmbracelet/bubbletea v1.3.10` - TUI (used in CLI tools)
- `github.com/tidwall/gjson v1.18.0` - JSON parsing
- `github.com/tidwall/sjson v1.2.5` - JSON building
- `gopkg.in/yaml.v3 v3.0.1` - YAML config parsing
- `github.com/lib/pq v1.12.0` - PostgreSQL driver
- `github.com/go-sql-driver/mysql v1.9.3` - MySQL driver
- `github.com/google/uuid v1.6.0` - UUID generation
- `github.com/gofrs/flock v0.13.0` - File-based locking
- `github.com/shirou/gopsutil/v4 v4.26.1` - System metrics
- `github.com/prometheus-community/pro-bing v0.4.0` - ICMP ping
- `github.com/beevik/etree v1.6.0` - XML parsing
## Configuration
- YAML-based configuration (`config.yaml`, `config_template.yaml`)
- Environment variable overrides (e.g., `AURAGO_MASTER_KEY`, `AURAGO_SERVER_HOST`)
- Vault system for secrets (AES-256-GCM encrypted, `data/vault.bin`)
- Multi-stage Dockerfile:
- `go.mod` / `go.sum` - Go dependencies
- `Dockerfile` - Multi-stage production build
- `Dockerfile.ansible` - Ansible sidecar image
- `config_template.yaml` - Full configuration reference (~600 lines)
- `docker-compose.yml` - Docker Compose with Gotenberg sidecar
## Platform Requirements
- Go 1.26.1+
- Python 3.10+ (for skill execution)
- Git
- Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64, arm64)
- Docker (optional, for containerized deployment)
- 512MB RAM minimum, 2GB recommended
## Frontend Stack
- Single-file SPA embedded via `go:embed`
- Multiple HTML entry points: `index.html`, `config.html`, `setup.html`, etc.
- Module-based JavaScript in `ui/js/` with subdirectories for features
- `ui/index.html` - Main chat interface
- `ui/js/chat/main.js` - Chat functionality
- `ui/js/setup/main.js` - Setup wizard
- `ui/shared.js` - Shared utilities
- `ui/css/` - Stylesheets (missions.css, setup.css)
- `ui/lang/` - i18n translations (15 languages)
- Tailwind CSS (`ui/tailwind.min.js`)
- Chart.js (`ui/chart.min.js`)
- CodeMirror 6 (`ui/js/vendor/codemirror6.min.js`)
## Database Technologies
- `data/short_term.db` - Conversation context (sliding window)
- `data/long_term.db` - Archived conversations
- `data/inventory.db` - SSH device inventory
- `data/invasion.db` - Distributed egg/nest system
- `data/cheatsheets.db` - Cheatsheet storage
- `data/image_gallery.db` - Image gallery metadata
- `data/remote_control.db` - Remote device control
- `data/media_registry.db` - Media file registry
- `data/homepage_registry.db` - Homepage project registry
- `data/contacts.db` - Address book
- `data/site_monitor.db` - Website monitoring
- `data/sql_connections.db` - External SQL connection configs
- `data/skills.db` - Skill management
- `data/vectordb/` - Semantic memory / knowledge graph embeddings
## Build System
# Development build
# Cross-compile for all platforms (Linux/macOS)
# Windows build
# Docker
- `aurago` - Main binary
- `lifeboat` - Self-update companion
- `config-merger` - Config merging utility
- `aurago-remote` - Remote execution agent (cross-platform clients bundled)
## Testing
- `*_test.go` files alongside source files
- `TestFunctionName` naming convention
- Table-driven tests preferred
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Language
## Naming Conventions
### Files
- **Multi-word files:** `snake_case.go` (e.g., `config_test.go`, `agent_loop.go`)
- **Single-word files:** `word.go`
- **Test files:** `*_test.go` co-located with source
### Types
- **Exported types:** `PascalCase` (e.g., `Agent`, `Vault`, `HistoryManager`)
- **Unexported types:** `camelCase` (e.g., `dockerConfig`, `historyMessage`)
### Functions
- **Exported functions:** `PascalCase` (e.g., `NewVault`, `Load`, `ExecuteWithRetry`)
- **Unexported functions:** `camelCase` (e.g., `buildLogger`, `parseWorkflowPlan`, `writeVaultFileAtomic`)
### Variables
- **General:** `camelCase` (e.g., `logger`, `configPath`, `vaultPath`)
- **Constants:** `PascalCase` for exported (e.g., `MaxScriptBytes`), `camelCase` for unexported (e.g., `dockerAPIVersion`)
### Packages
- **Naming:** Short, lowercase, no underscores (e.g., `config`, `llm`, `tools`)
- **Avoid:** Generic names like `util` or `common`
## Code Style
### Formatting
- **Tool:** `gofmt` (standard Go formatter)
- **No golangci configuration detected** - project relies on `gofmt` defaults
### Line Length
- No strict line length limit enforced
### Imports
### Indentation
- **Tabs:** Use tabs for indentation (Go standard)
- **Align:** Multiple imports on separate lines when needed
## Error Handling
### Pattern
### Error Context Rules
### Examples from codebase
### Nil Checks
### Sentinel Errors
## Logging
### Framework
### Logger Creation
### Log Levels
- **`slog.LevelDebug`:** Detailed debugging information
- **`slog.LevelInfo`:** General operational information (default)
- **`slog.LevelWarn`:** Warning conditions
- **`slog.LevelError`:** Error conditions
### Key-Value Pairs
### Logger Injection
### Silent Operations
## Comments
### Style
### When to Comment
### What NOT to Comment
### TODO/FIXME Pattern
## File Organization
### One Responsibility Per File
### Large File Threshold
### Test File Co-location
### Package Structure
- **`internal/`:** Private application code (not importable by other projects)
- **`cmd/`:** Application entry points
- **`internal/tools/`:** 90+ tool implementations
- **`internal/server/`:** 60+ HTTP server handlers
## Security Patterns
### Vault System
### Master Key
- **Environment variable:** `AURAGO_MASTER_KEY`
- **Format:** 64 hex characters (32 bytes)
- **Never commit:** `.env` files, vault files
### Sensitive Data Scrubbing
### Atomic File Writes
### Permission Toggles
### Forbidden Vault Exports
## Import Organization
### Order
### Path Aliases
## Function Design
### Constructor Pattern
### Parameter Order
### Return Values
- **Errors:** Return `error` as last return value
- **Multiple values:** Group related returns (result + error)
## Concurrency
### Mutex Pattern
### Channel-Based Communication
### WaitGroups
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Single Go binary with embedded Web UI (no CGO dependencies)
- Agent-centric design: all components serve the autonomous agent loop
- Multi-transport communication: HTTP REST, SSE streaming, WebSocket-ready
- Tiered memory system: STM (SQLite) -> LTM (VectorDB) -> Knowledge Graph (SQLite/FTS5)
- Security-first: AES-256-GCM vault, secret scrubbing, permission toggles
## Layers
- Purpose: Core autonomous agent orchestration
- Location: `internal/agent/agent_loop.go`
- Contains: Main loop, tool dispatch, memory ranking, context compression, co-agents
- Depends on: LLM client, memory systems, vault, config
- Used by: Server SSE handler, background tasks
- Purpose: Multi-provider LLM client with failover and retry logic
- Location: `internal/llm/`
- Contains: FailoverManager, pricing, context window detection
- Depends on: Config, HTTP client
- Used by: Agent loop, server handlers
- Purpose: Multi-tier persistent memory for conversation context and knowledge
- Locations:
- Contains: Message storage, vector embeddings, entity relationships, temporal patterns
- Depends on: Config, LLM for embeddings
- Used by: Agent loop, server handlers, indexing service
- Purpose: HTTP/HTTPS server, REST API, SSE streaming, Web UI serving
- Location: `internal/server/server.go`, `internal/server/sse.go`
- Contains: Router, handlers (60+), auth, middleware, SSE broadcaster
- Depends on: All other layers
- Used by: Web UI (browser), external API clients
- Purpose: 90+ built-in tools for agent execution
- Locations: `internal/tools/*.go`, `internal/agent/native_tools*.go`
- Contains: Shell, Python, filesystem, HTTP, Docker, SSH, cron, etc.
- Depends on: Config, sandbox, vault, process registry
- Used by: Agent dispatch
- Purpose: Vault encryption, secret scrubbing, Guardian (input validation), LLM Guardian
- Location: `internal/security/vault.go`, `internal/security/guardian.go`
- Contains: AES-256-GCM vault, token manager, scrubber, SSRF protection
- Depends on: OS crypto, file system
- Used by: All layers handling secrets
- Purpose: YAML config parsing, defaults, provider resolution
- Location: `internal/config/config.go`, `internal/config/config_types.go`
- Contains: Config structs, env var loading, vault secret injection
- Depends on: YAML parser, vault
- Used by: All layers
## Data Flow
## Key Abstractions
- Purpose: Manage multiple LLM providers with automatic failover
- Examples: `internal/llm/failover.go`
- Pattern: Wraps primary provider, falls back to configured alternatives
- Purpose: Abstraction for vector database operations
- Implementation: ChromemVectorDB (chromem-go)
- Methods: StoreDocument, SearchSimilar, GetByID, DeleteDocument
- Purpose: Real-time event streaming to Web UI
- Pattern: Pub/sub with channel-based clients
- Methods: Send, SendJSON, BroadcastType
- Purpose: Encrypted secret storage
- Pattern: AES-256-GCM with atomic file writes
- Methods: ReadSecret, WriteSecret, ListKeys, EncryptBytes
- Purpose: Track background processes spawned by agent
- Pattern: Thread-safe map of PID -> ProcessInfo
- Methods: Register, Terminate, KillAll, List
## Entry Points
- Location: `cmd/aurago/main.go` (~930 lines)
- Triggers: Application startup
- Responsibilities:
- Location: `internal/server/server.go:Start()`
- Triggers: After all subsystems initialized
- Responsibilities:
## Error Handling
- `fmt.Errorf("context: %w", err)` for error wrapping
- `log/slog` for structured logging with key-value pairs
- Feature flags disable failing subsystems (VectorDB disabled if embeddings fail)
- Background tasks recover from panics with deferred recovery
- Retry logic with exponential backoff for transient failures
- VectorDB disabled if embedding pipeline fails (app still functional)
- SQLite-only mode if VectorDB unavailable
- LLM failover to backup providers
- Sandbox fallback to direct Python execution
## Cross-Cutting Concerns
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->

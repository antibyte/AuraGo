# AuraGo - AI Coding Agent Reference

> **Language**: This project primarily uses **English** for code and documentation. The default system language is set to German (`Deutsch`) in config but English is the development language.

## Project Overview

**AuraGo** is a fully autonomous AI agent written in Go, designed for home lab environments. It ships as a single portable binary with an embedded Web UI and has zero external dependencies for the core functionality.

### Key Characteristics
- **Single binary deployment** - Pure Go with embedded SQLite (no CGO)
- **Self-contained** - Web UI baked in via `go:embed`
- **Home lab focused** - Docker, Proxmox, Home Assistant, SSH device management
- **Multi-platform** - Linux, macOS, Windows (amd64, arm64)
- **30+ built-in tools** - Shell, Python execution, file system, HTTP requests, cron, and many more

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.26+ |
| Web Framework | Standard library `net/http` with gorilla/mux patterns |
| Database | SQLite (modernc.org/sqlite - pure Go, no CGO) |
| Vector DB | chromem-go (embedded) |
| Frontend | Vanilla JavaScript SPA (embedded) |
| Python Runtime | Python 3.10+ (for sandboxed execution) |
| Container | Docker, Docker Compose |

### Core Technologies
| Component | Technology |
|-----------|------------|
| Language | Go 1.26+ |
| Web Framework | Standard library `net/http` with gorilla/mux patterns |
| Database | SQLite (modernc.org/sqlite - pure Go, no CGO) |
| Vector DB | chromem-go (embedded) |
| Frontend | Vanilla JavaScript SPA (embedded) |
| Python Runtime | Python 3.10+ (for sandboxed execution) |
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

## Project Structure

```
AuraGo/
├── cmd/                          # Application entry points
│   ├── aurago/                   # Main agent binary
│   │   ├── main.go               # Entry point with full initialization
│   │   ├── platform_unix.go      # Unix-specific code
│   │   └── platform_windows.go   # Windows-specific code
│   ├── lifeboat/                 # Self-update companion binary
│   ├── remote/                   # Remote execution agent
│   └── config-merger/            # Configuration merging utility
├── internal/                     # Private application code
│   ├── agent/                    # Core agent loop, tool dispatch, co-agents
│   ├── budget/                   # Token cost tracking
│   ├── commands/                 # Slash commands (/reset, /budget, etc.)
│   ├── config/                   # YAML config parsing & defaults
│   ├── discord/                  # Discord bot integration
│   ├── invasion/                 # Invasion Control (egg/nest system)
│   ├── inventory/                # SSH device inventory (SQLite)
│   ├── llm/                      # LLM client, failover, retry
│   ├── logger/                   # Structured logging setup
│   ├── memory/                   # STM, LTM, knowledge graph, personality
│   ├── prompts/                  # Dynamic system prompt builder
│   ├── remote/                   # SSH remote execution
│   ├── security/                 # AES-GCM vault & token manager
│   ├── server/                   # HTTP/HTTPS server, REST handlers
│   ├── telegram/                 # Telegram bot (text, voice, vision)
│   ├── tools/                    # All tool implementations
│   └── webhooks/                 # Incoming & outgoing webhooks
├── agent_workspace/              # Runtime workspace
│   ├── skills/                   # Pre-built Python skills
│   ├── tools/                    # Agent-created tools + manifest
│   └── workdir/                  # Sandboxed execution directory
├── prompts/                      # System prompt markdown files
│   ├── identity.md               # Core identity prompt
│   ├── personalities/            # Personality profiles
│   ├── templates/                # Prompt templates
│   └── tools_manuals/            # Tool documentation for RAG
├── ui/                           # Embedded Web UI (single-file SPA)
├── data/                         # Runtime data (databases, vault, state)
├── documentation/                # Detailed setup guides
├── config.yaml                   # Main configuration file
├── config_template.yaml          # Configuration template
├── Dockerfile                    # Multi-stage build
├── docker-compose.yml            # Docker Compose setup
└── install.sh                    # Quick installer script
```

## Build Commands

### Development Build
```bash
# Build main binary
go build -o aurago ./cmd/aurago

# Build with lifeboat (recommended for development)
./start.sh
```

### Production Build
```bash
# Cross-compile for all platforms
./make_deploy.sh

# Individual platform build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aurago ./cmd/aurago
```

### Docker Build
```bash
# Build image
docker build -t aurago:latest .

# Or use docker-compose
docker-compose up -d
```

### Test Commands
```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/config/...
go test ./internal/agent/...

# Run with verbose output
go test -v ./internal/tools/...
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
| `AURAGO_MASTER_KEY` | 64-character hex key for vault encryption |
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

## Security Considerations

### Vault System
- AES-256-GCM encryption for all secrets
- Master key (64 hex chars = 32 bytes) required at startup
- Vault file: `data/vault.bin`
- Never commit `.env` or vault files

### Danger Zone Capabilities
All potentially dangerous operations are gated:
- `allow_shell` - Shell command execution
- `allow_python` - Python code execution
- `allow_filesystem_write` - File write operations
- `allow_network_requests` - HTTP requests
- `allow_remote_shell` - SSH to remote devices
- `allow_self_update` - Binary self-updates

### Sensitive Data Scrubbing
Use `security.RegisterSensitive(value)` to prevent values from appearing in logs or LLM outputs.

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
6. Document in `documentation/`

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
- Triggered on push to `main` branch

### Release Process
1. Run `./make_deploy.sh` to build cross-platform binaries
2. Commit changes (script auto-commits)
3. Push to trigger Docker build
4. Create GitHub Release with binary artifacts

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

### Web UI Guidelines

#### UX Design Principles
- **User-friendly by default**: Avoid technical jargon, provide clear instructions and feedback
- Do not break the style of the UI - changes should fit seamlessly into the existing interface
- Aim for **masterpiece UX design** that feels native to the existing interface
- If you see bad UX in the existing UI, feel free to improve it while keeping overall style consistent
- **Always check if new features are relevant to the dashboard** and add them there if applicable

#### Translations
- **Always update translation files** in `ui/lang/` for **ALL supported languages**
- Keep translations up to date and consistent with UI changes
- If you add new features requiring new UI elements, provide translations for all supported languages

### Code Organization & Quality

#### File Management
- **Keep files manageable**: If files get too big and unwieldy, split them into smaller pieces
- Orient yourself on what an AI agent can handle with ease
- If a file becomes too large to process effectively, break it down into smaller, more manageable files
- Always aim for clarity and maintainability in your file structure

#### Temporary Files
- **Always cleanup temporary files and logs** after use
- Don't leave behind orphaned temporary resources

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

#### Build Process
- **Use `make_deploy` script** to build binaries and upload to test server
- Don't use manual build commands for production builds

## Resources

- **README.md** - User-facing documentation
- **documentation/** - Detailed guides
- **config_template.yaml** - Full configuration reference
- **prompts/tools_manuals/** - Tool documentation (RAG-indexed)

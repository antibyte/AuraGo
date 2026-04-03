# Technology Stack

**Analysis Date:** 2026-04-03

## Languages

**Primary:**
- **Go 1.26.1** - Core application language for all backend services, agent loop, integrations
- **JavaScript** - Frontend SPA (vanilla JS, no framework)
- **Python 3.12** - Sandboxed skill/tool execution via embedded venv

## Runtime

**Environment:**
- Go 1.26.1 (build target)
- Python 3.12-slim (runtime in Docker)
- CGO disabled for pure Go cross-compilation

**Package Manager:**
- Go modules (`go.mod` / `go.sum`)
- No external package registry dependencies beyond go.mod

## Frameworks

**Core:**
- Standard library `net/http` with custom routing (not using gorilla/mux despite CLAUDE.md mentioning it)
- `github.com/gorilla/websocket v1.5.4` - WebSocket support
- `github.com/sashabaranov/go-openai v1.41.2` - OpenAI-compatible LLM client

**Database:**
- `modernc.org/sqlite v1.28.0` - Pure Go SQLite driver (no CGO)
- `github.com/philippgille/chromem-go v0.7.0` - Embedded vector database for semantic memory

**Testing:**
- Go standard `testing` package
- `go test ./...` for all tests

**Build:**
- Docker multi-stage build (`Dockerfile`)
- Go cross-compilation with `GOOS`/`GOARCH`/`CGO_ENABLED=0`

## Key Dependencies

**Critical:**
- `github.com/sashabaranov/go-openai v1.41.2` - LLM client for OpenAI-compatible APIs
- `modernc.org/sqlite v1.28.0` - All SQLite databases (memory, inventory, etc.)
- `github.com/philippgille/chromem-go v0.7.0` - Vector DB for long-term memory

**Bot Integrations:**
- `github.com/bwmarrin/discordgo v0.29.0` - Discord bot
- `github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1` - Telegram bot
- `github.com/eclipse/paho.mqtt.golang v1.5.1` - MQTT client

**Infrastructure:**
- `golang.org/x/crypto` - SSH client, bcrypt, ACME/Let's Encrypt
- `github.com/pkg/sftp v1.13.10` - SFTP for remote file transfers
- `tailscale.com v1.96.1` - Tailscale VPN integration
- `github.com/aws/aws-sdk-go-v2` - AWS S3 SDK
- `github.com/robfig/cron/v3 v3.0.1` - Cron scheduler

**Web & Media:**
- `github.com/go-rod/rod v0.116.2` - Headless Chrome for web scraping/screenshots
- `github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728` - PDF parsing
- `github.com/johnfercher/maroto/v2 v2.3.4` - PDF generation (Maroto)
- `github.com/pdfcpu/pdfcpu v0.11.1` - PDF processing
- `github.com/charmbracelet/bubbletea v1.3.10` - TUI (used in CLI tools)

**Data Processing:**
- `github.com/tidwall/gjson v1.18.0` - JSON parsing
- `github.com/tidwall/sjson v1.2.5` - JSON building
- `gopkg.in/yaml.v3 v3.0.1` - YAML config parsing
- `github.com/lib/pq v1.12.0` - PostgreSQL driver
- `github.com/go-sql-driver/mysql v1.9.3` - MySQL driver

**Utilities:**
- `github.com/google/uuid v1.6.0` - UUID generation
- `github.com/gofrs/flock v0.13.0` - File-based locking
- `github.com/shirou/gopsutil/v4 v4.26.1` - System metrics
- `github.com/prometheus-community/pro-bing v0.4.0` - ICMP ping
- `github.com/beevik/etree v1.6.0` - XML parsing

## Configuration

**Environment:**
- YAML-based configuration (`config.yaml`, `config_template.yaml`)
- Environment variable overrides (e.g., `AURAGO_MASTER_KEY`, `AURAGO_SERVER_HOST`)
- Vault system for secrets (AES-256-GCM encrypted, `data/vault.bin`)

**Build:**
- Multi-stage Dockerfile:
  - Stage 1: `golang:1.26.1-bookworm` builder
  - Stage 2: `python:3.12-slim-bookworm` runtime with ffmpeg

**Key Files:**
- `go.mod` / `go.sum` - Go dependencies
- `Dockerfile` - Multi-stage production build
- `Dockerfile.ansible` - Ansible sidecar image
- `config_template.yaml` - Full configuration reference (~600 lines)
- `docker-compose.yml` - Docker Compose with Gotenberg sidecar

## Platform Requirements

**Development:**
- Go 1.26.1+
- Python 3.10+ (for skill execution)
- Git

**Production:**
- Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64, arm64)
- Docker (optional, for containerized deployment)
- 512MB RAM minimum, 2GB recommended

## Frontend Stack

**Framework:** Vanilla JavaScript SPA (no React, Vue, Angular)

**Architecture:**
- Single-file SPA embedded via `go:embed`
- Multiple HTML entry points: `index.html`, `config.html`, `setup.html`, etc.
- Module-based JavaScript in `ui/js/` with subdirectories for features

**Key Frontend Files:**
- `ui/index.html` - Main chat interface
- `ui/js/chat/main.js` - Chat functionality
- `ui/js/setup/main.js` - Setup wizard
- `ui/shared.js` - Shared utilities
- `ui/css/` - Stylesheets (missions.css, setup.css)
- `ui/lang/` - i18n translations (15 languages)

**Dependencies (CDN):**
- Tailwind CSS (`ui/tailwind.min.js`)
- Chart.js (`ui/chart.min.js`)
- CodeMirror 6 (`ui/js/vendor/codemirror6.min.js`)

## Database Technologies

**SQLite (modernc.org/sqlite - pure Go, no CGO):**
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

**Vector DB (chromem-go):**
- `data/vectordb/` - Semantic memory / knowledge graph embeddings

## Build System

**Commands:**
```bash
# Development build
go build -o aurago ./cmd/aurago

# Cross-compile for all platforms (Linux/macOS)
./make_deploy.sh

# Windows build
make_release.bat

# Docker
docker build -t aurago:latest .
docker-compose up -d
```

**Build Outputs:**
- `aurago` - Main binary
- `lifeboat` - Self-update companion
- `config-merger` - Config merging utility
- `aurago-remote` - Remote execution agent (cross-platform clients bundled)

## Testing

**Framework:** Go standard `testing` package

**Test Organization:**
- `*_test.go` files alongside source files
- `TestFunctionName` naming convention
- Table-driven tests preferred

**Run Commands:**
```bash
go test ./...                    # All tests
go test -v ./internal/tools/...   # Verbose output
go test -cover ./internal/...     # Coverage
go test -race ./...              # Race detection
go test -bench=. ./internal/...   # Benchmarks
```

---

*Stack analysis: 2026-04-03*

# External Integrations

**Analysis Date:** 2026-04-03

## Messaging Platforms

### Discord Bot
- **Package:** `github.com/bwmarrin/discordgo v0.29.0`
- **Config:** `internal/config/config_types.go` - `Discord` struct
- **Implementation:** `internal/discord/bot.go`
- **Features:**
  - Guild message handling
  - Direct message support
  - Message content intent required
  - Bot token stored in vault
  - Configurable allowed user IDs

### Telegram Bot
- **Package:** `github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1`
- **Config:** `internal/config/config_types.go` - `Telegram` struct
- **Implementation:** `internal/telegram/bot.go`
- **Features:**
  - Long polling mode (webhook cleared)
  - Worker pool for concurrent message processing (configurable `max_concurrent_workers`)
  - Voice message support
  - Vision/image processing
  - User ID authorization
  - Bot token stored in vault

### RocketChat Bot
- **Config:** `internal/config/config_types.go` - `RocketChat` struct
- **Implementation:** `internal/rocketchat/bot.go`
- **Features:**
  - REST API polling
  - Channel-based messaging
  - Configurable alias
  - Allowed users whitelist
  - Auth via `X-Auth-Token` and `X-User-Id` headers

## IoT & Home Automation

### Home Assistant
- **Config:** `internal/config/config_types.go` - `HomeAssistant` struct
- **Tools:** `internal/tools/homeassistant_poller.go`
- **Features:**
  - REST API integration
  - Access token authentication (vault-stored)
  - Read-only mode available
  - State monitoring and automation

### Fritz!Box
- **Implementation:** `internal/fritzbox/client.go`
- **Protocols:**
  - TR-064 SOAP for system/network/smart home
  - AHA-HTTP for home automation device control
  - SID session authentication
- **Feature Groups (all individually toggled):**
  - System: device info, uptime, log, temperatures
  - Network: WLAN, guest WLAN, DECT, mesh, hosts, Wake-on-LAN, port forwarding
  - Telephony: call lists, phonebooks, call deflection, TAM
  - SmartHome: devices, switches, heating, blinds, lamps, templates
  - Storage: NAS, FTP, USB devices, media server
  - TV: DVB-C channel list and stream URLs
- **Password stored in vault**

### MQTT
- **Package:** `github.com/eclipse/paho.mqtt.golang v1.5.1`
- **Config:** `internal/config/config_types.go` - `MQTT` struct
- **Implementation:** `internal/mqtt/client.go`
- **Features:**
  - Broker connection with configurable QoS
  - Topic subscription with wildcard support
  - Ring buffer for incoming messages (500 max)
  - Mission triggers (payload-based event handling)
  - Relay to agent capability

### Chromecast
- **Config:** `internal/config/config_types.go` - `Chromecast` struct
- **Tools:** `internal/tools/chromecast.go`
- **Features:**
  - TTS port configuration
  - Media casting

## Infrastructure & DevOps

### Docker
- **Implementation:** `internal/tools/docker.go`
- **Connection:**
  - Unix socket: `unix:///var/run/docker.sock` (Linux)
  - Named pipe: `npipe:////./pipe/docker_engine` (Windows)
  - TCP: `tcp://localhost:2375` (via docker-proxy)
- **API Version:** v1.45
- **Features:**
  - Container management (create, start, stop, remove)
  - Image listing and management
  - Exec operations
  - Log streaming
  - Stats monitoring
  - Volume and network inspection
  - Read-only mode available
  - Docker socket proxy option for security hardening

### Proxmox VE
- **Implementation:** `internal/tools/proxmox.go`
- **Config:** `internal/config/config_types.go` - `Proxmox` struct
- **Features:**
  - Token-based API authentication (`PVEAPIToken=`)
  - TLS verification control
  - Node-specific operations
  - Read-only mode available
  - VM/LXC lifecycle management

### Ansible
- **Config:** `internal/config/config_types.go` - `Ansible` struct
- **Modes:**
  - Sidecar: HTTP API server (`ansible` Docker container)
  - Local: Direct ansible CLI invocation
- **Features:**
  - Playbook execution
  - Ad-hoc commands
  - Inventory management
  - Read-only mode available

### SSH / SFTP (Remote Execution)
- **Packages:**
  - `golang.org/x/crypto/ssh` - SSH client
  - `github.com/pkg/sftp v1.13.10` - SFTP client
- **Implementation:** `internal/remote/remote.go`
- **Features:**
  - Password or private key authentication
  - Known hosts verification (with insecure fallback)
  - File transfer via SFTP
  - Remote command execution
  - Context with timeout support

### Remote Control
- **Config:** `internal/config/config_types.go` - `RemoteControl` struct
- **Implementation:** `internal/remote/` package
- **Features:**
  - Device discovery via UDP broadcast
  - Cross-platform support (Windows named pipes, Unix sockets)
  - File transfer with size limits
  - Auto-approve option (not recommended)
  - Path allowlisting
  - Audit logging

## Cloud Services

### AWS S3 / S3-Compatible Storage
- **Package:** `github.com/aws/aws-sdk-go-v2`
- **Config:** `internal/config/config_types.go` - `S3` struct
- **Tools:** `internal/tools/s3.go`
- **Features:**
  - AWS S3, MinIO, Wasabi, Backblaze B2 compatible
  - List buckets/objects
  - Upload, download, delete, copy, move
  - Path-style and virtual-hosted addressing
  - Read-only mode available
  - Credentials stored in vault

### Cloudflare Tunnel
- **Config:** `internal/config/config_types.go` - `CloudflareTunnel` struct
- **Handlers:** `internal/server/tunnel_handlers.go`
- **Modes:** auto (Docker preferred, native fallback), docker, native
- **Features:**
  - Auto-start on boot
  - Multiple auth methods (token, named, quick)
  - Web UI and homepage routing
  - Custom ingress rules
  - Metrics endpoint
  - Read-only mode available

### Tailscale
- **Package:** `tailscale.com v1.96.1`
- **Config:** `internal/config/config_types.go` - `Tailscale` struct
- **Features:**
  - API integration for tailnet management
  - tsnet embedded node (independent of API)
  - MagicDNS hostname
  - Serve HTTP over Tailscale
  - Funnel for public exposure
  - Read-only mode available

## Media & Entertainment

### Jellyfin
- **Implementation:** `internal/jellyfin/client.go`
- **Config:** `internal/config/config_types.go` - Jellyfin via `JellyfinConfig`
- **Features:**
  - REST API client
  - API key authentication (vault-stored)
  - Media library browsing
  - Item metadata

### TrueNAS
- **Implementation:** `internal/truenas/client.go`
- **Handlers:** `internal/server/truenas_handlers.go`
- **Features:**
  - REST API integration
  - Storage management
  - Dataset and zvol operations

### Media Registry
- **Config:** `internal/config/config_types.go` - `MediaRegistry` struct
- **Database:** `data/media_registry.db`
- **Features:**
  - Media file tracking
  - Gallery management

## Communication

### Telnyx (SMS/Voice)
- **Implementation:** `internal/telnyx/client.go`, `internal/telnyx/webhook.go`
- **Config:** `internal/config/config_types.go` - `Telnyx` struct
- **Features:**
  - SMS sending/receiving
  - Voice calls with SIP connection
  - Webhook handling for inbound messages/calls
  - Call recording (optional)
  - Voicemail transcription (via LLM)
  - Number allowlisting
  - Rate limiting (concurrent calls, SMS/minute)
  - Relay to agent
  - Credentials stored in vault

### Email (IMAP/SMTP)
- **Config:** `internal/config/config_types.go` - `Email` struct, `EmailAccount` struct
- **Tools:** `internal/tools/email_watcher.go`
- **Features:**
  - Multiple account support
  - IMAP folder watching
  - SMTP sending
  - OAuth2 support (via provider system)

## Web & Networking

### Webhooks (Incoming)
- **Implementation:** `internal/webhooks/webhook.go`, `internal/webhooks/handler.go`
- **Config:** `internal/config/config_types.go` - `Webhooks` struct
- **Features:**
  - Custom slug endpoints
  - HMAC signature validation (SHA256/SHA1)
  - Field extraction with dot-path mapping
  - Delivery modes: message (loopback), notify (SSE), silent
  - Prompt templating
  - Rate limiting
  - Max 10 webhooks

### Webhooks (Outgoing)
- **Config:** `internal/config/config_types.go` - `OutgoingWebhook` struct
- **Features:**
  - Configurable HTTP methods (GET, POST, PUT, DELETE)
  - Header configuration
  - Payload templating (JSON, form, custom)
  - Variable substitution in URLs

### Web Scraping
- **Packages:**
  - `github.com/go-rod/rod v0.116.2` - Headless Chrome
  - `github.com/go-shiori/go-readability v0.0.0-20251205110129-5db1dc9836f0` - Article extraction
  - `github.com/gocolly/colly/v2 v2.3.0` - Scraping framework
- **Tools:** `internal/tools/scraper.go`
- **Features:**
  - Screenshot capture
  - PDF generation from pages
  - Article readability extraction
  - Form automation

### Brave Search
- **Config:** `internal/config/config_types.go` - `BraveSearch` struct
- **Tools:** `internal/tools/brave.go`
- **Features:**
  - Web search API
  - Country and language filtering
  - API key stored in vault

### VirusTotal
- **Config:** `internal/config/config_types.go` - `VirusTotal` struct
- **Features:**
  - URL and file scanning
  - API key stored in vault

## Storage & Backup

### WebDAV
- **Config:** `internal/config/config_types.go` - `WebDAV` struct
- **Features:**
  - Basic and bearer auth
  - Read-only mode available

### Koofr
- **Config:** `internal/config/config_types.go` - `Koofr` struct
- **Features:**
  - Cloud storage integration
  - App password authentication

### Paperless-NGX
- **Config:** `internal/config/config_types.go` - `PaperlessNGX` struct
- **Features:**
  - Document management
  - Search and retrieval
  - Tagging and correspondence

## Agent-to-Agent Communication

### A2A (Agent-to-Agent Protocol)
- **Package:** `github.com/a2aproject/a2a-go/v2 v2.0.0`
- **Config:** `internal/config/config_types.go` - `A2A` struct
- **Handlers:** `internal/server/a2a_handlers.go`
- **Features:**
  - Server with REST, JSON-RPC, gRPC bindings
  - Agent Card advertising
  - Push notifications
  - Client for remote agents
  - Skills advertising
  - API key and Bearer auth

### Co-Agents
- **Config:** `internal/config/config_types.go` - `CoAgents` struct
- **Features:**
  - Multiple concurrent specialist agents
  - Specialist roles: researcher, coder, designer, security, writer
  - Budget quota management
  - Circuit breaker with timeout and token limits
  - Retry policy with error pattern matching
  - Context hints for coordination

## Security

### Vault (AES-256-GCM)
- **Implementation:** `internal/security/` package
- **Key:** 64-character hex (32 bytes) via `AURAGO_MASTER_KEY`
- **Files:** `data/vault.bin`
- **Features:**
  - Encrypted secret storage
  - Sensitive value scrubbing from logs
  - API keys, passwords, tokens

### LLM Guardian
- **Config:** `internal/config/config_types.go` - `LLMGuardian` struct
- **Implementation:** `internal/security/llm_guardian.go`
- **Features:**
  - LLM-based security checks before tool execution
  - Per-tool level overrides (off, low, medium, high)
  - Document and email scanning
  - Cache TTL and rate limiting
  - Fail-safe modes (block, allow, quarantine)

### Security Proxy
- **Config:** `internal/config/config_types.go` - `SecurityProxy` struct
- **Features:**
  - ACME/Let's Encrypt TLS
  - Rate limiting
  - IP filtering (allowlist/blocklist)
  - Basic auth
  - Geo-blocking

## Database Connections

### External SQL Databases
- **Config:** `internal/config/config_types.go` - `SQLConnections` struct
- **Drivers:**
  - `github.com/lib/pq` - PostgreSQL
  - `github.com/go-sql-driver/mysql` - MySQL/MariaDB
  - `modernc.org/sqlite` - SQLite
- **Features:**
  - Connection pooling
  - Query timeout
  - Row limits

## Document Processing

### Document Creator
- **Config:** `internal/config/config_types.go` - `DocumentCreatorConfig`
- **Tools:** `internal/tools/document_creator.go`
- **Backends:**
  - Maroto (built-in, PDF generation via `github.com/johnfercher/maroto/v2`)
  - Gotenberg (Docker sidecar, `gotenberg/gotenberg:8`)
- **Features:**
  - PDF from HTML/Markdown
  - Screenshot capture
  - Document conversion

### PDF Processing
- **Packages:**
  - `github.com/ledongthuc/pdf` - PDF reading
  - `github.com/pdfcpu/pdfcpu v0.11.1` - PDF processing

## Voice & Speech

### TTS (Text-to-Speech)
- **Config:** `internal/config/config_types.go` - `TTS` struct
- **Providers:**
  - Google TTS
  - ElevenLabs (with voice customization)
  - MiniMax
  - Piper (local, via `wyoming-piper` Docker container)
- **Tools:** `internal/tools/piper_tts.go`, `internal/tools/wyoming.go`
- **Features:**
  - Caching with retention limits
  - Multi-language support

### Whisper (Speech-to-Text)
- **Config:** `internal/config/config_types.go` - `Whisper` struct
- **Provider:** Via LLM provider system (OpenAI-compatible API)

## Development Tools

### Ollama
- **Config:** `internal/config/config_types.go` - `Ollama` struct
- **Tools:** `internal/tools/ollama_embeddings.go`, `internal/tools/ollama_managed.go`
- **Features:**
  - Managed Docker container
  - GPU passthrough (NVIDIA, AMD, Intel, Vulkan)
  - Auto-pull models
  - Memory limits
  - Local embeddings

### Git Integration
- **Tools:** `internal/tools/git_test.go` (git operations)
- **Features:**
  - Repository operations
  - Homepage Git deployment

## Other Integrations

### MeshCentral
- **Implementation:** `internal/meshcentral/client.go`
- **Config:** `internal/config/config_types.go` - `MeshCentral` struct
- **Features:**
  - WebSocket-based communication
  - Remote desktop
  - Device management
  - Token or username/password auth
  - Read-only mode and blocked operations

### n8n Workflows
- **Config:** Via `n8n` config section
- **Handlers:** `internal/server/n8n_handlers.go`

### Indexing Service
- **Config:** `internal/config/config_types.go` - `Indexing` struct
- **Features:**
  - Directory polling
  - Extension filtering
  - Knowledge base indexing

### Cron / Scheduler
- **Package:** `github.com/robfig/cron/v3 v3.0.1`
- **Features:**
  - Scheduled task execution
  - Mission triggering

### Network Discovery
- **Tools:**
  - `internal/tools/mdns.go` - mDNS/Bonjour discovery
  - `internal/tools/upnp_scan.go` - UPnP/SSDP discovery
  - `internal/tools/dns_lookup.go` - DNS queries
  - `internal/tools/port_scanner.go` - Port scanning

### Wake-on-LAN
- **Tools:** `internal/tools/wol.go`
- **Features:**
  - Magic packet sending
  - MAC address lookup

### AdGuard Home
- **Config:** `internal/config/config_types.go` - `AdGuard` struct
- **Features:**
  - DNS filtering control
  - Query statistics

## Environment Configuration

**Required env vars:**
- `AURAGO_MASTER_KEY` - 64-character hex key for vault encryption (critical)
- `AURAGO_SERVER_HOST` - Override server bind address (Docker: `0.0.0.0`)
- `LLM_API_KEY` / `OPENAI_API_KEY` - LLM API key override
- `TAILSCALE_API_KEY` - Tailscale integration
- `ANSIBLE_API_TOKEN` - Ansible sidecar authentication

**Secrets location:**
- All sensitive credentials stored in encrypted vault (`data/vault.bin`)
- Referenced via vault keys in config (e.g., `vault:"bot_token"`)
- Never stored in YAML or environment variables

---

*Integration audit: 2026-04-03*

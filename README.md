<!-- logo for light mode -->
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="ui/aurago_logo.png">
  <source media="(prefers-color-scheme: light)" srcset="ui/aurago_logo_dark.png">
  <img alt="AuraGo" src="ui/aurago_logo_dark.png" width="360">
</picture>

# AuraGo — Your Home Lab AI Agent

**A self-contained AI agent built for home labs — single binary, zero external dependencies, runs on any Linux server or Raspberry Pi.**

> **🛠️ Work in Progress** — AuraGo is under active development. Not all features are fully tested; expect rough edges and occasional breaking changes.

> **ℹ️ You are in control** — Nearly every feature in AuraGo can be individually disabled or restricted. Shell and Python execution, filesystem access, network requests, self-updates, and remote access each have their own on/off toggle in the **Danger Zone**. Integrations like Home Assistant and webhooks support **read-only mode**. The firewall monitor can be set to observe-only. Web UI access can be protected with a password and 2FA; when HTTPS is active, login cannot be disabled. — That said, AuraGo is a capable system that can interact with your infrastructure, and **you are ultimately responsible for how you configure and expose it**. For internet-facing installs, always enable HTTPS, login protection, and 2FA. Never run it unprotected on a public IP.


AuraGo is a fully autonomous AI agent written in Go that ships as one portable binary with an embedded Web UI. Connect it to any OpenAI-compatible LLM provider (OpenRouter, Ollama, local models, …) and it becomes a powerful home lab assistant: managing Docker containers, monitoring Proxmox VMs, controlling Home Assistant devices, waking up servers via Wake-on-LAN, watching your firewall, sending notifications, calling outgoing webhooks, and much more — all from a clean chat interface or via Telegram and Discord.

---

## Why AuraGo for Home Labs?

Unlike cloud AI services, AuraGo runs **on your hardware**, has **direct access to your infrastructure**, and keeps all data local. It's designed to be the AI brain of a home lab:

- 🏠 **Control your smart home** via Home Assistant
- 🐳 **Manage Docker & containers** — start, stop, inspect
- 🖥️ **Monitor Proxmox VMs and LXCs** — snapshots, status, lifecycle
- 🌐 **SSH into network devices** — routers, NAS boxes, remote servers
- 💤 **Wake-on-LAN** — remotely power on devices
- 🧱 **Firewall monitoring** — read rules, get alerts on changes (Linux)
- 📤 **Outgoing webhooks** — connect to any external API or automation
- 🔔 **Notifications** — ntfy, Pushover, Telegram alerts
- 🔒 **HTTPS with Let's Encrypt** — built-in auto-TLS for internet-facing installs
- 📡 **MQTT** — subscribe and publish to your smart home message bus

---

## 🔥 Must-Try Features

### Personality Engine V2 + User Profiling
**An agent that truly knows you.** AuraGo learns your preferences, tech stack, and communication style — and fully adapts to you. Through continuous user profiling, an individual user profile develops over time that influences how the agent interacts with you:

- **Automatic preference detection** — Remembers which tools you prefer, answer lengths you like, whether you want code examples
- **Tech stack tracking** — Learns which technologies you use (Docker, Kubernetes, Proxmox, Home Assistant…) and provides contextual responses
- **Communication style adaptation** — Whether professional, casual, detailed, or short & snappy: the agent adapts to your style
- **Emotional affinity** — Affinity tracking builds a "relationship" that develops over time

> 💡 **Tip:** Enable Personality Engine V2 in the settings and chat with the agent for a while. You'll notice how responses become more personal.

### Adaptive Tools – Intelligent Tool Filtering
**Save tokens, stay focused.** The Adaptive Tools system analyzes the conversation context and intelligently filters available tools before sending them to the LLM:

- **Context-aware filtering** — Less relevant tools are hidden based on the conversation topic
- **Usage-based scoring** — Frequently used tools are prioritized, rare ones sorted down
- **Configurable limits** — Set a maximum limit for tools available simultaneously
- **Always-Include list** — Important tools can be excluded from filtering

> 💡 **Why this matters:** Fewer tools in context = lower costs, faster responses, and more precise tool selection by the LLM.

### Document Creator & PDF Extractor – Document Management
**Create and process PDFs like a pro.** AuraGo now includes comprehensive PDF functionality:

- **PDF creation** — Generate invoices, reports, and documents via maroto (built-in) or Gotenberg (Docker sidecar)
- **PDF extraction** — Extract text from PDFs with optional LLM summarization for large documents
- **Template-based** — Reusable document templates for recurring formats
- **Cloud-ready** — Integration with Paperless NGX for document archiving

> 📄 **Use cases:** Automated invoice generation, contract processing, document archiving, PDF analysis from emails or downloads.

### LLM Guardian – Your AI Security Guard
**Security is paramount.** The LLM Guardian monitors everything your agent does and protects against potentially dangerous or unwanted actions:

- **Tool call scanning** — Every tool call is checked for risks before execution (e.g., dangerous shell commands, data deletion, sensitive areas)
- **Document & email analysis** — Detects malicious content, phishing attempts, or suspicious attachments before further processing
- **Prompt injection protection** — Isolates external data with `<external_data>` wrappers to prevent prompt injection attacks
- **Granular control** — Fully configurable: protection level, exceptions, logging

> 🛡️ **Security by design:** The Guardian is your constant companion, ensuring the agent never oversteps boundaries — especially important for shell access, file operations, or remote execution.

---

## Key Features

### Agent Core
- **50+ built-in tools** — shell & Python execution, file system, HTTP requests, cron scheduling, process management, system metrics, Docker, Proxmox, TrueNAS, Ollama, Home Assistant, Tailscale, Ansible, MeshCentral, GitHub, Netlify, Paperless NGX, PDF processing, document creation, and many more
- **Native Function Calling** — OpenAI-style tool calls with auto-detection for DeepSeek and compatible models; optional **Structured Outputs** mode for constrained decoding
- **Dynamic tool creation** — the agent can write, save, and register new Python tools at runtime
- **Multi-step reasoning loop** with automatic tool dispatch, error recovery, and corrective feedback
- **Co-Agent system** — spawn parallel sub-agents with independent LLM contexts for complex tasks
- **Intelligent Prompt Builder** — reduces costs via context compression, background summarization, and automatic RAG-based recall; includes analytics dashboard
- **Configurable personalities** — friend, professional, punk, neutral, and more
- **Personality Engine V2** — LLM-powered mood analysis, affinity tracking, and behavioral adaptation
- **User Profiling** — automatic detection and storage of user preferences, tech stack, and communication style
- **Context Compression** — automatic summarization of long conversations to preserve token budget
- **Memory Analysis** — dedicated LLM provider for real-time extraction of facts, preferences, and corrections
- **Memory Consolidation** — nightly batch processing of archived conversations into long-term memory
- **Weekly Reflection** — periodic memory health analysis and pattern recognition
- **Adaptive Tools** — intelligent tool filtering based on conversation context to reduce token usage
- **Sudo Execution** — privileged command execution with vault-secured password storage

### Memory & Knowledge
- **Short-term memory** — SQLite sliding-window conversation context
- **Long-term memory (RAG)** — embedded vector database (chromem-go) with semantic search
- **Knowledge graph** — entity-relationship store for structured facts with auto-extraction and prompt injection
- **Persistent notes & to-dos** — categorized, prioritized, with due dates
- **Core memory** — permanent facts the agent always remembers
- **Journal** — chronological event logging with importance scoring and auto-generated entries
- **Temporal Patterns** — interaction pattern detection for predictive memory pre-fetching
- **Knowledge Indexing** — automatic indexing of local files (.md, .txt, .json, .csv, .log, .yaml)

### Home Lab Integrations

| Integration | Description |
|---|---|
| **Web UI** | Embedded single-page chat app with dark/light theme, file uploads, **system dashboard**, and **full configuration editor** |
| **HTTPS + Let's Encrypt** | Built-in auto-TLS — just provide a domain and email, certificates are managed automatically |
| **Docker** | Container, image, network & volume management — start, stop, inspect, logs |
| **Proxmox** | VM and LXC lifecycle management (start, stop, snapshots, status) |
| **Home Assistant** | Smart-home control — device states, services, toggle, read-only guard mode |
| **Device Inventory** | SSH command execution on remote servers, NAS, and routers |
| **Wake-on-LAN** | Power on network devices by MAC address |
| **Firewall Monitor** | Linux ufw/iptables guard — read rules, alert on changes (guard mode, read-only mode) |
| **MQTT** | Subscribe and publish to smart home message bus |
| **Outgoing Webhooks** | Call any external HTTP API with configurable parameters — agent can trigger them by name |
| **Incoming Webhooks** | Receive events from GitHub, Alertmanager, Home Assistant, etc. with token auth |
| **Ansible** | Run playbooks and manage your lab fleet |
| **Tailscale** | VPN node inspection and management |
| **MeshCentral** | Remote desktop agent management |
| **Ollama** | Local model management (list, pull, delete, run) |
| **Telegram** | Full bot with voice messages, image analysis, inline commands |
| **Discord** | Bot integration with message bridge |
| **Rocket.Chat** | Bot integration for self-hosted instances |
| **Email** | IMAP inbox watcher + SMTP sending (multiple accounts supported) |
| **Google Workspace** | Gmail, Calendar, Drive, Docs, Sheets with OAuth2 |
| **Cloud Storage** | WebDAV & Koofr (Nextcloud, ownCloud, Synology, etc.) |
| **Chromecast & Audio** | Discover LAN speakers, TTS streaming |
| **Notifications** | Push via **ntfy** and **Pushover** |
| **Budget Tracking** | Per-model token cost tracking with daily limits and enforcement |
| **MCP** | Model Context Protocol — connect external MCP servers or expose AuraGo as MCP server |
| **MCP Server** | Expose AuraGo tools to other MCP clients (e.g., Claude Desktop) |
| **Invasion Control** | Distributed agent management — spawn "eggs" (sub-agents) across multiple hosts |
| **Remote Control** | Remote agent-to-agent communication for distributed task execution |
| **Cloudflare Tunnel** | Built-in Cloudflare tunnel integration (quick, token, or named tunnels) |
| **Cloudflare AI Gateway** | Unified AI gateway for request routing and observability |
| **AdGuard Home** | DNS filtering management and statistics |
| **TrueNAS** | ZFS storage management - pools, datasets, snapshots, SMB/NFS shares |
| **GitHub** | Repository management, issues, pull requests, projects |
| **Netlify** | Static site deployment and management |
| **Paperless NGX** | Document management integration |
| **Brave Search** | Web search via Brave Search API |
| **VirusTotal** | File and URL security scanning |
| **PDF Extractor** | Extract text from PDF documents with LLM summarization |
| **Document Creator** | Generate PDF documents (invoices, reports) via maroto or Gotenberg |
| **Cheatsheets** | Quick-reference command snippets and personal cheat sheet management |
| **Sudo Execution** | Execute privileged commands with stored sudo password (vault-secured) |
| **Image Generation** | Multi-provider support (OpenAI, Stability, Ideogram, Google, OpenRouter) |
| **n8n Integration** | Bidirectional n8n workflow automation — trigger workflows and control the agent from n8n |
| **Telnyx** | SMS/voice calls — send/receive SMS, make calls, voicemail, IVR system |
| **Media Registry** | Local media file indexing and metadata management |
| **Homepage Registry** | Dashboard homepage site management |
| **Sandbox** | Isolated Python execution environment (Docker/Podman backend) |
| **Vision** | Image analysis via vision-capable LLMs |
| **Transcription** | Audio transcription via Whisper (OpenAI or local) |
| **TTS** | Text-to-speech via Google or ElevenLabs |
| **Gotenberg** | PDF generation sidecar for document creation |
| **n8n Node** | Official community node for n8n integration — chat, tools, memory, missions |
| **Telnyx Voice/SMS** | Full telephony integration — outbound/inbound calls, SMS, voicemail, call routing |

### Security
- **AES-256-GCM encrypted vault** for API keys — manageable via Web UI with key rotation
- **Web UI Authentication** — login protection with **bcrypt** password hashing, **TOTP 2FA**, and API token management
- **Auto-provisioned passwords** — the install script generates a secure first-login password
- **HTTPS enforcement** — when HTTPS is active, login cannot be disabled (enforced via UI)
- **Danger Zone** — granular capability gates (shell, Python, filesystem, network, remote shell, self-update, MCP, web scraper)
- **LLM Guardian** — AI-powered security scanner for tool calls, documents, and emails with threat detection
- **Security Headers** — CSP, HSTS, X-Frame-Options, and more on every response
- **Sandboxed Python execution** — isolated venv workspace or Docker/Podman containers with configurable container pooling
- **LLM failover** — automatic switch to backup provider on errors
- **Circuit breaker** — configurable limits on tool calls, timeouts, and retries
- **Rate limiting** — login attempts, webhook requests, and API calls
- **Prompt Injection Defense** — automatic isolation of external content with `<external_data>` wrappers

### Self-Improvement
- **Maintenance loop** — scheduled nightly agent run for memory cleanup and autonomous tasks
- **Lifeboat system** — companion binary for hot-swap self-updates
- **Code surgery** — the agent can modify its own codebase via a structured plan/execute workflow
- **Daily reflection** — morning briefing generated at 03:00
- **Memory Consolidation** — automatic archival and compression of old conversations into long-term memory
- **Memory Analysis** — dedicated LLM extracts facts, preferences, and corrections from conversations
- **Weekly Reflection** — periodic memory health analysis and pattern recognition with automatic insights

---

## Quick Start

### Option A — One-Liner Install (Linux x86_64)

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

The interactive install script will:
- Clone the repo and set up the directory structure
- Check for Docker and offer to install it (recommended for many features)
- Ask if this is an internet-facing server — if yes, prompt for domain + email and configure **automatic HTTPS with Let's Encrypt**
- Generate a **secure first-login password** and display it (also saved to `firstpassword.txt`)
- Optionally install a **systemd service** so AuraGo starts on boot

After install:

1. Edit `~/aurago/config.yaml` — set at minimum `llm.api_key`
2. `source ~/aurago/.env`
3. `cd ~/aurago && ./start.sh`
4. Open **https://yourdomain.com** (HTTPS) or **http://localhost:8088** (local)
5. Log in with the generated password — **change it immediately**

#### Command-Line Flags

AuraGo supports flags for automated/scripted startup:

```bash
# Start with HTTPS, domain, email, and a pre-set password (used by install script)
./aurago -https -domain=home.example.com -email=you@example.com -password=<generated>

# Custom config path
./aurago --config /etc/aurago/config.yaml
```

#### Linux Service Installation (Optional)

```bash
sudo ./install_service_linux.sh
```

---

### Option B — Build from Source

#### Prerequisites

- **Go 1.23+**
- **Python 3.10+** — required for custom tools, skills, and sandboxed execution
- An API key for an OpenAI-compatible LLM provider (e.g. [OpenRouter](https://openrouter.ai/))

#### 1. Clone & Build

```bash
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo
go build -o aurago cmd/aurago/main.go
```

On Windows:
```powershell
go build -o aurago.exe cmd/aurago/main.go
```

> The binary is fully portable — pure Go SQLite driver, no CGO required. The Web UI is baked in via `go:embed`.

#### 2. Configure

The easiest way to set up AuraGo is through the **Web UI** — open it after the first start and use the built-in **Quick Setup Wizard** and **Configuration editor** to configure your LLM, integrations, and features without touching a file.

Alternatively, you can edit `config.yaml` directly — useful for scripted or headless setups. The minimum required is an LLM API key:

```yaml
server:
  host: "127.0.0.1"
  port: 8088
  # Optional HTTPS (Let's Encrypt):
  # https:
  #   enabled: true
  #   domain: "home.example.com"
  #   email: "you@example.com"

llm:
  provider: openrouter
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-..."
  model: "google/gemini-2.0-flash-001"

# Enable Docker for container management
docker:
  enabled: true

# Enable Home Assistant integration
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  # AccessToken via vault (set in Web UI)
```

See `config.yaml` for all available options.

#### 3. Set Master Key (optional)

AuraGo automatically generates a secure master key on first start and saves it to `.env`. You don't need to do anything — just make sure to **back up the `.env` file** so you don't lose access to your vault.

If you prefer to set your own key (e.g., for reproducible deployments), set the environment variable before starting:

**Linux / macOS:**
```bash
export AURAGO_MASTER_KEY="$(openssl rand -hex 32)"
```

**Windows (PowerShell):**
```powershell
$env:AURAGO_MASTER_KEY = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })
```

> The key is stored in `.env` and loaded automatically at startup. Keep that file safe — without it, the encrypted vault cannot be decrypted.

#### 4. Start

```bash
./aurago
```

Or use the provided scripts which also build the lifeboat companion:

```bash
# Linux / macOS
chmod +x start.sh && ./start.sh

# Windows
start.bat
```

Open **http://localhost:8088** — done.

---

### Option C — Dockge (Docker Compose UI)

[Dockge](https://github.com/louislam/dockge) is a popular self-hosted Docker Compose manager, perfect for home labs. You can add AuraGo as a stack in a few clicks.

#### 1. Create the stack

In Dockge, click **+ Compose**, name it `aurago`, and paste the following `compose.yaml`:

```yaml
services:
  aurago:
    image: ghcr.io/antibyte/aurago:latest
    container_name: aurago
    restart: unless-stopped
    ports:
      - "8088:8088"     # Web UI (HTTP)
      # - "443:443"     # Uncomment for HTTPS (Let's Encrypt)
      # - "80:80"       # Uncomment for HTTP → HTTPS redirect
    volumes:
      - ./data:/app/data          # Databases, vault, vector store
      - ./config.yaml:/app/config.yaml
      - ./agent_workspace:/app/agent_workspace
    environment:
      - AURAGO_MASTER_KEY=${AURAGO_MASTER_KEY}
    # Optional: HTTPS via Let's Encrypt flags
    # command: ["-https", "-domain=home.example.com", "-email=you@example.com"]
```

#### 2. Set the master key (optional)

AuraGo **automatically generates a secure master key on first start** and saves it inside the `./data` volume. You don't need to set it manually.

If you want to use your own key (e.g., for portability between hosts), add it to Dockge's **Environment** tab or create a `.env` file in the stack directory:

```bash
AURAGO_MASTER_KEY=$(openssl rand -hex 32)
```

> Back up the key (and the `./data` volume). Without the key, the encrypted vault cannot be decrypted.

#### 3. Add a minimal `config.yaml`

```yaml
llm:
  provider: openrouter
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-..."
  model: "google/gemini-2.0-flash-001"
```

All other settings can be configured via the **Web UI** after the first start.

#### 4. Deploy

Click **Deploy** in Dockge — AuraGo starts and is accessible at **http://your-server-ip:8088**.

> **Tip:** Use a Traefik or Nginx Proxy Manager reverse proxy (also manageable via Dockge stacks) to add a domain name and automatic HTTPS in front of AuraGo, or use AuraGo's built-in HTTPS support with the `-https` flag.

---

## Project Structure

```
AuraGo/
├── cmd/
│   ├── aurago/          # Main agent entry point
│   ├── lifeboat/        # Self-update companion binary
│   ├── config-merger/   # Configuration merging utility
│   └── remote/          # Remote execution agent
├── internal/
│   ├── agent/           # Core agent loop, tool dispatch, co-agents, maintenance, memory analysis
│   ├── budget/          # Token cost tracking & enforcement
│   ├── commands/        # Slash commands (/reset, /budget, /debug, …)
│   ├── config/          # YAML config parser, migration & defaults
│   ├── discord/         # Discord bot integration
│   ├── invasion/        # Invasion Control (egg/nest distributed system)
│   ├── inventory/       # SSH device inventory (SQLite)
│   ├── llm/             # LLM client, failover, retry, context detection
│   ├── media/           # Media registry and metadata management
│   ├── memory/          # All memory subsystems (STM, LTM, graph, personality, journal, notes)
│   ├── meshcentral/     # MeshCentral remote desktop integration
│   ├── mqtt/            # MQTT client for IoT integration
│   ├── prompts/         # Dynamic system prompt builder with analytics
│   ├── push/            # Push notification manager (ntfy, Pushover)
│   ├── remote/          # Remote agent communication protocol
│   ├── rocketchat/      # Rocket.Chat bot integration
│   ├── scraper/         # Web scraping utilities
│   ├── security/        # AES-GCM vault, tokens, LLM Guardian, scrubber
│   ├── server/          # HTTP/HTTPS server, SSE, REST handlers, TLS, WebSocket bridge
│   ├── services/        # Content ingestion and indexing services
│   ├── setup/           # First-run setup wizard logic
│   ├── telegram/        # Telegram bot (text, voice, vision)
│   ├── tools/           # All tool implementations (50+ tools including document creator, PDF extractor, cheatsheets)
│   └── webhooks/        # Incoming & outgoing webhook engine
├── agent_workspace/
│   ├── prompts/         # Modular system prompt markdown files & personalities
│   ├── skills/          # Pre-built Python skills (search, scraping, Google, …)
│   ├── tools/           # Agent-created tools + manifest
│   └── workdir/         # Sandboxed execution directory
├── prompts/             # System prompts, personalities, and tool manuals
│   ├── tools_manuals/   # RAG-indexed tool documentation
│   ├── personalities/   # Personality profiles (friend, punk, professional, …)
│   └── templates/       # Prompt templates
├── ui/                  # Embedded Web UI (single-file SPA)
├── data/                # Runtime data (SQLite DBs, vector store, vault, state)
├── knowledge/           # Local knowledge base for indexing
├── documentation/       # Detailed setup guides & concepts
├── deploy/              # Deployment configurations
└── config.yaml          # Main configuration file
```

---

## Chat Commands

| Command | Description |
|---|---|
| `/help` | List available commands |
| `/reset` | Clear conversation history and start fresh |
| `/stop` | Interrupt the current agent action |
| `/restart` | Restart the agent process |
| `/debug on\|off` | Toggle detailed error reporting |
| `/budget` | Show daily token cost breakdown |
| `/personality <name>` | Switch to a different personality profile |
| `/sudo` | Temporarily elevate privileges for sensitive operations (requires vault-stored sudo password) |
| `/journal` | Open journal management interface |
| `/addssh <host> <user>` | Quick-add SSH device to inventory |

---

## Web UI Features

| Feature | Description |
|---|---|
| **Chat** | Real-time streaming conversation with tool execution feedback |
| **Dashboard** | System metrics, mood history, prompt builder analytics, memory stats, token usage |
| **Configuration** | Full config editor organized by section (LLM, Docker, Proxmox, Webhooks, Firewall, …) |
| **Quick Setup** | First-run wizard for essential settings |
| **Login & 2FA** | Optional authentication with bcrypt passwords and TOTP two-factor |
| **API Tokens** | Create and manage API tokens for webhook integrations |
| **Outgoing Webhooks** | Configure external webhooks the agent can call with parameters |
| **Incoming Webhooks** | Receive and process events from external services |
| **Firewall Monitor** | View and monitor Linux firewall rules (ufw/iptables) — Linux only |
| **Danger Zone** | Toggle agent capabilities (shell, Python, filesystem, network, self-update) |
| **Vault Management** | View vault status and safely reset/regenerate the master key |
| **Personality Editor** | Create and manage personality profiles directly in the browser |
| **MCP Servers** | Manage Model Context Protocol server connections |
| **MCP Server Config** | Configure AuraGo as an MCP server for external clients |
| **Invasion Control** | Distributed agent management — spawn and monitor remote "egg" agents |
| **Document Creator** | Configure PDF generation backend (maroto/Gotenberg) |
| **Remote Control** | Manage remote agent connections for distributed execution |
| **Media Registry** | Browse and search indexed local media files |
| **Homepage Registry** | Manage Homepage dashboard deployments |
| **Image Gallery** | View and manage generated images |
| **Memory Maintenance** | Archive, optimize, and manage memory stores |
| **Journal** | View and search chronological event logs |
| **Knowledge Graph** | Visualize entity relationships |
| **Notes** | Manage persistent notes and to-dos |
| **Device Inventory** | SSH device management and connection testing |
| **Sandbox** | Configure and monitor sandboxed execution environments |

---

## Outgoing Webhooks

AuraGo supports **outgoing webhooks** — configurable HTTP calls the agent can trigger by name. This enables seamless integration with external services:

```
Example: "Send Slack Notification"
  Method: POST
  URL: https://hooks.slack.com/services/...
  Parameters:
    - message [string, required] — The message to send
    - channel [string] — Target Slack channel
```

Once configured in the UI, the agent can call any webhook by name:  
_"Hey, send a Slack notification to #alerts: Proxmox backup completed."_

---

## HTTPS with Let's Encrypt

For internet-facing installs, AuraGo handles TLS automatically:

```yaml
server:
  https:
    enabled: true
    domain: "home.example.com"  # Your public domain
    email: "you@example.com"    # For Let's Encrypt notifications
```

- Certificates are obtained and renewed automatically
- HTTP traffic on port 80 is redirected to HTTPS
- When HTTPS is active, login **cannot be disabled** in the UI

---

## User Manual

Comprehensive user guides are available in multiple languages:

- **[German Manual](documentation/manual/de/README.md)** – Complete user guide in German
- **[English Manual](documentation/manual/en/README.md)** – Complete user guide in English

The manuals cover installation, configuration, all 50+ features, security settings, and troubleshooting.

## Documentation

Additional technical documentation is available in the [`documentation/`](documentation/) folder:

- [Configuration Reference](documentation/configuration.md)
- [Installation Guide](documentation/installation.md)
- [Telegram Setup](documentation/telegram_setup.md)
- [Google Workspace Setup](documentation/google_setup.md)
- [Docker Integration](documentation/docker.md)
- [Docker Installation (Container)](documentation/docker_installation.md)
- [WebDAV Integration](documentation/webdav.md)
- [Co-Agent Concept](documentation/co_agent_concept.md)
- [Personality Engine V2](documentation/personality_engine_v2.md)
- [Memory Analysis & Consolidation](documentation/memory_analysis.md)
- [Invasion Control (Distributed Agents)](documentation/invasion_control.md) *(coming soon)*
- [MCP Integration](documentation/mcp.md) *(coming soon)*
- [LLM Guardian Security](documentation/llm_guardian.md) *(coming soon)*
- [Cloudflare Tunnel Setup](documentation/cloudflare_tunnel.md) *(coming soon)*
- [Image Generation](documentation/image_generation.md) *(coming soon)*
- [Sandbox Setup](documentation/sandbox.md) *(coming soon)*
- [n8n Integration](documentation/n8n-node-plan.md) – Workflow automation with n8n
- [Telnyx Integration](documentation/telnyx_integration_plan.md) – SMS and voice call integration
- [Adaptive Tool Schemas](documentation/adaptive_tool_schema_plan.md) – Context-aware tool filtering
- [Tool Context Optimization](documentation/tool_context_optimization_plan_v2.md) – Efficient context usage
- [Security Introduction](documentation/security_introduction.md) – Security concepts and best practices
- [UI/UX Analysis](documentation/ui_ux_analysis.md) – Interface design principles

---

## License

This project is provided as-is for personal and educational use.

---

<details>
<summary><strong>Dependencies</strong></summary>

| Library | Purpose |
|---|---|
| [go-openai](https://github.com/sashabaranov/go-openai) | OpenAI-compatible LLM client |
| [chromem-go](https://github.com/philippgille/chromem-go) | Embedded vector database for RAG |
| [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | Pure Go SQLite driver (no CGO) |
| [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) | Telegram bot integration |
| [discordgo](https://github.com/bwmarrin/discordgo) | Discord bot integration |
| [gopsutil](https://github.com/shirou/gopsutil) | System metrics (CPU, memory, disk) |
| [sftp](https://github.com/pkg/sftp) | SFTP file transfers for remote execution |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | SSH client, bcrypt password hashing & crypto |
| [golang.org/x/crypto/acme](https://pkg.go.dev/golang.org/x/crypto/acme) | Let's Encrypt / ACME TLS automation |
| [cron/v3](https://github.com/robfig/cron) | Cron-based task scheduler |
| [vishen/go-chromecast](https://github.com/vishen/go-chromecast) | Chromecast LAN discovery and CASTV2 control |
| [hashicorp/mdns](https://github.com/hashicorp/mdns) | Multicast DNS discovery |
| [flock](https://github.com/gofrs/flock) | File-based lock to prevent duplicate instances |
| [uuid](https://github.com/google/uuid) | UUID generation |
| [tiktoken-go](https://github.com/pkoukk/tiktoken-go) | Token counting for context management |
| [yaml.v3](https://github.com/go-yaml/yaml) | YAML configuration parsing |
| [oauth2](https://golang.org/x/oauth2) | OAuth2 authentication for Google Workspace |
| [qrcode](https://github.com/skip2/go-qrcode) | TOTP QR code generation |
| [gonote](https://github.com/mazznoer/gonote) | Colorful console output |

</details>

<!-- logo for light mode -->
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="ui/aurago_logo.png">
  <source media="(prefers-color-scheme: light)" srcset="ui/aurago_logo_dark.png">
  <img alt="AuraGo" src="ui/aurago_logo_dark.png" width="360">
</picture>

# AuraGo - your personal AI agent with personality

**A self-contained, self-improving AI agent framework — single binary, zero external dependencies.**

> **⚠️ Work in Progress** — This project is under active development. Not all features are fully tested. Expect rough edges, breaking changes, and experimental behavior.

> **🚨 Security Warning** — AuraGo can execute arbitrary shell commands, write files, and modify system state. **Never expose the Web UI to the public internet without proper security measures** (VPN, reverse proxy with authentication, firewall rules). Running this unprotected on a public-facing server is a serious security risk.

> **🖥️ Installation Recommendation** — While it is technically possible to run AuraGo on your daily-use workstation, it is **strongly recommended** to install it in an isolated environment: a **virtual machine**, a **Docker container**, or a **dedicated PC**. AuraGo executes code, modifies files, and manages processes on the host system — mistakes by the LLM or a misconfigured prompt can have unintended effects on the surrounding system.

AuraGo is a fully autonomous AI agent written in Go that ships as one portable binary with an embedded Web UI. Connect it to any OpenAI-compatible LLM provider (OpenRouter, Ollama, local models, …) and it becomes a personal assistant that can execute code, manage files, control smart-home devices, send emails, remember everything, and even improve its own source code — all from a clean chat interface or via Telegram and Discord.

---

## Key Features

### Agent Core
- **30+ built-in tools** — WebDAV, Koofr, Chromecast, Text-to-Speech, shell & Python execution, file system, HTTP requests, cron scheduling, process management, system metrics, Proxmox, Ollama, and many more
- **Native Function Calling** — OpenAI-style tool calls with auto-detection for DeepSeek and compatible models; optional **Structured Outputs** mode for constrained decoding
- **Dynamic tool creation** — the agent can write, save, and register new Python tools at runtime
- **Multi-step reasoning loop** with automatic tool dispatch, error recovery, and corrective feedback
- **Co-Agent system** — spawn parallel sub-agents with independent LLM contexts for complex tasks
- **Intelligent Prompt Builder** — reduces costs via recursive character-based context compression, background summarization (Persistent Summary), and automatic RAG-based factual recall; includes analytics dashboard
- **Configurable personalities** — friend, professional, punk, neutral, mistress and more
- **Personality Engine** — V1 with zero extra LLM calls and an advanced V2 requiring a lightweight external model. The V2 engine dynamically adapts the agent's mood and motivation to the situation and your reactions, making him more human-like and giving him a natural desire to evolve and improve
- **User Profiling** (V2) — automatic detection and storage of user preferences, interests, and communication style
- **Temperature Modulation** — personality engine can dynamically adjust LLM temperature based on mood

### Memory & Knowledge
- **Short-term memory** — SQLite sliding-window conversation context
- **Long-term memory (RAG)** — embedded vector database (chromem-go) with semantic search
- **Knowledge graph** — entity-relationship store for structured facts
- **Persistent notes & to-dos** — categorized, prioritized, with due dates
- **Core memory** — permanent facts the agent always remembers

### Integrations
| Integration | Description |
|---|---|
| **Web UI** | Embedded single-page chat app with dark/light theme, file uploads, image lightbox, **system dashboard**, and **full configuration editor** |
| **Quick Setup Wizard** | First-run assistant that guides you through LLM, personality, and integration setup |
| **Telegram** | Full bot with voice messages, image analysis, inline commands |
| **Discord** | Bot integration with message bridge |
| **Rocket.Chat** | Bot integration for self-hosted Rocket.Chat instances |
| **Email** | IMAP inbox watcher + SMTP sending |
| **Home Assistant** | Smart-home device control (states, services, toggle) |
| **Docker** | Container, image, network & volume management |
| **Proxmox** | VM and LXC container management (start, stop, status, snapshots) |
| **Ollama** | Local model management (list, pull, delete, run) |
| **Device Inventory** | Execute commands on remote SSH servers and manage generic network devices |
| **Chromecast & Audio**| Discover LAN speakers, adjust volume, and stream Text-to-Speech audio |
| **Google Workspace** | Manage Gmail, Calendar, Drive, and Docs via internal tool |
| **Cloud Storage** | WebDAV & Koofr access (Nextcloud, ownCloud, Synology, Koofr, etc.) |
| **Budget Tracking** | Optional per-model cost tracking with daily limits and enforcement modes |
| **Notifications** | Push notifications via **ntfy** and **Pushover** |
| **Webhooks** | Incoming webhooks with token authentication for external integrations |

### Self-Improvement
- **Maintenance loop** — scheduled nightly agent run for housekeeping, memory cleanup, and autonomous tasks
- **Lifeboat system** — companion binary that hot-swaps the main process for self-updates
- **Code surgery** — the agent can modify its own codebase via a structured plan/execute workflow
- **Daily reflection** — morning briefing generation at 03:00 AM

### Security
- **AES-256-GCM encrypted vault** for API keys and secrets — manageable via Web UI with safe key rotation
- **Web UI Authentication** — optional login protection with **bcrypt** password hashing and **TOTP 2FA**
- **Danger Zone** — granular capability gates to enable/disable shell, Python, filesystem, network, remote shell, and self-update tools
- **Sandboxed execution** — Python runs in an isolated venv workspace
- **File lock** — prevents duplicate instances
- **LLM failover** — automatic switch to a backup provider on consecutive errors
- **Circuit breaker** — configurable limits on tool calls, timeouts, and retry intervals
- **Rate limiting** — login attempts, webhook requests, and API calls can be rate-limited

---

## Quick Start

### Option A — One-Liner Install (Linux x86_64)

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

The script clones the repo, sets permissions on the pre-built binary, generates a master key into `.env`, and optionally installs a systemd service. Afterwards:

1. Edit `~/aurago/config.yaml` — set at minimum `llm.api_key`
2. `source ~/aurago/.env`
3. `cd ~/aurago && ./start.sh`
4. Open **http://localhost:8088**

#### Linux Service Installation (Optional)

To keep AuraGo running in the background and start automatically on boot:

```bash
sudo ./install_service_linux.sh
```

This will create a systemd service, handle root/user permissions, ensure your master key is set, and enable the service.

---

### Option B — Build from Source

### Prerequisites

- **Go 1.21+** (for building from source)
- **Python 3.10+** — required for the agent to create and execute custom tools, run skills (web scraping, Google Workspace, etc.), and use the sandboxed Python environment
- **[Gemini CLI](https://github.com/google-gemini/gemini-cli)** (optional) — required only for the self-modification feature (code surgery via lifeboat). Install and authenticate it before enabling `maintenance.lifeboat_enabled`
- An API key for an OpenAI-compatible LLM provider (e.g. [OpenRouter](https://openrouter.ai/))

### 1. Clone & Build

```bash
git clone https://github.com/your-username/AuraGo.git
cd AuraGo
go build -o aurago cmd/aurago/main.go
```

On Windows:
```powershell
go build -o aurago.exe cmd/aurago/main.go
```

> The binary is fully portable — pure Go SQLite driver, no CGO required. The Web UI is baked in via `go:embed`.

### 2. Configure

Edit `config.yaml` in the project root:

```yaml
server:
  host: "127.0.0.1"
  port: 8088

llm:
  provider: openrouter                          # or "ollama", any OpenAI-compatible
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-..."                          # ← your API key
  model: "arcee-ai/trinity-large-preview:free"  # ← your preferred model
```

See `config.yaml` for all available options (Telegram, Discord, email, Home Assistant, Docker, co-agents, budget, etc.).

### 3. Set Master Key

AuraGo encrypts its secrets vault with a 64-character hex key (32 bytes AES-256):

**Linux / macOS:**
```bash
export AURAGO_MASTER_KEY="$(openssl rand -hex 32)"
```

**Windows (PowerShell):**
```powershell
$env:AURAGO_MASTER_KEY = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })
```

> **Keep this key safe.** Without it, the encrypted vault cannot be decrypted.

### 4. Start

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

Open **http://localhost:8088** in your browser — done.

---

## Project Structure

```
AuraGo/
├── cmd/
│   ├── aurago/          # Main agent entry point
│   └── lifeboat/        # Self-update companion binary
├── internal/
│   ├── agent/           # Core agent loop, tool dispatch, co-agents, maintenance
│   ├── budget/          # Token cost tracking & enforcement
│   ├── commands/        # Slash commands (/reset, /budget, /debug, …)
│   ├── config/          # YAML config parser & defaults
│   ├── discord/         # Discord bot integration
│   ├── inventory/       # SSH server inventory (SQLite)
│   ├── llm/             # LLM client, failover, retry, context detection
│   ├── memory/          # All memory subsystems (STM, LTM, graph, personality, …)
│   ├── prompts/         # Dynamic system prompt builder
│   ├── remote/          # SSH remote execution
│   ├── security/        # AES-GCM vault & guardian
│   ├── server/          # HTTP server, SSE, REST handlers
│   ├── telegram/        # Telegram bot (text, voice, vision)
│   └── tools/           # All tool implementations + process/cron managers
├── agent_workspace/
│   ├── prompts/         # Modular system prompt markdown files & personalities
│   ├── skills/          # Pre-built Python skills (search, scraping, Google, …)
│   ├── tools/           # Agent-created tools + manifest
│   └── workdir/         # Sandboxed execution directory
├── ui/                  # Embedded Web UI (single-file SPA)
├── data/                # Runtime data (SQLite DBs, vector store, vault, state)
├── documentation/       # Detailed setup guides & concepts
└── config.yaml          # Main configuration file
```

---

## Chat Commands

| Command | Description |
|---|---|
| `/help` | List available commands |
| `/reset` | Clear conversation history and start fresh |
| `/stop` | Interrupt the current agent action |
| `/debug on\|off` | Toggle detailed error reporting |
| `/budget` | Show daily token cost breakdown |
| `/personality <name>` | Switch to a different personality profile |

---

## Web UI Features

The embedded Web UI provides a complete control center for AuraGo:

| Feature | Description |
|---|---|
| **Chat** | Real-time streaming conversation with tool execution feedback |
| **Dashboard** | System metrics, mood history, prompt builder analytics, memory stats |
| **Configuration** | Full YAML config editor organized by section (LLM, Agent, Integrations, etc.) |
| **Quick Setup** | First-run wizard for essential settings (LLM API key, personality, integrations) |
| **Login & 2FA** | Optional authentication with bcrypt passwords and TOTP two-factor |
| **Danger Zone** | Toggle agent capabilities (shell, Python, filesystem, network, self-update) |
| **Vault Management** | View vault status and safely reset/regenerate the master key |
| **Personality Editor** | Create, edit, and manage personality profiles directly in the browser |

---

## Documentation

Detailed guides are available in the [`documentation/`](documentation/) folder:

- [Configuration Reference](documentation/configuration.md)
- [Installation Guide](documentation/installation.md)
- [Telegram Setup](documentation/telegram_setup.md)
- [Google Workspace Setup](documentation/google_setup.md)
- [Docker Integration](documentation/docker.md)
- [Docker Installation (Container)](documentation/docker_installation.md)
- [WebDAV Integration](documentation/webdav.md)
- [Co-Agent Concept](documentation/co_agent_concept.md)
- [Personality Engine V2](documentation/personality_engine_v2.md)

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
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | SSH client, bcrypt password hashing & cryptographic primitives |
| [cron/v3](https://github.com/robfig/cron) | Cron-based task scheduler |
| [vishen/go-chromecast](https://github.com/vishen/go-chromecast) | Chromecast LAN discovery and CASTV2 control |
| [hashicorp/mdns](https://github.com/hashicorp/mdns) | Multicast DNS discovery |
| [flock](https://github.com/gofrs/flock) | File-based lock to prevent duplicate instances |
| [uuid](https://github.com/google/uuid) | UUID generation |
| [tiktoken-go](https://github.com/pkoukk/tiktoken-go) | Token counting for context management |
| [yaml.v3](https://github.com/go-yaml/yaml) | YAML configuration parsing |

</details>

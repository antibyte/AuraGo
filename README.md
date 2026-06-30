<!-- logo for light mode -->
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="ui/aurago_logo.png">
  <source media="(prefers-color-scheme: light)" srcset="ui/aurago_logo_dark.png">
  <img alt="AuraGo" src="ui/aurago_logo_dark.png" width="360">
</picture>

# AuraGo — The Self-Hosted AI Agent Framework for Homelabs

**Self-hosted AI agents. Zero cloud. Zero compromise.**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![Website](https://img.shields.io/badge/Website-aurago--web-blue?logo=githubpages)](https://antibyte.github.io/aurago-web/)
[![Windows Companion App](https://img.shields.io/badge/Windows%20Companion-Agodesk-0078D4?logo=windows)](https://github.com/antibyte/agodesk/releases)
[![Discord](https://img.shields.io/badge/Discord-Join%20Us-5865F2?logo=discord&logoColor=white)](https://discord.gg/aurago)

> **🛠️ Work in Progress** — AuraGo is under active development. Expect rapid feature additions and some rough edges. Linux is the primary target; Windows and macOS companion builds exist but are experimental.

> **🔒 Total Security Control** — Every permission is individually granular. File system access, shell execution, python sandboxes, network requests, and self-updates can be enabled or disabled via the **Danger Zone**. For internet-facing setups, auto-HTTPS (Let's Encrypt), secure login, and TOTP 2FA are fully built-in.

---

## 🚀 Why AuraGo?

Most modern AI solutions require cloud dependencies, sending your sensitive local configurations and logs to third-party servers. **AuraGo runs entirely on your own local hardware.** It keeps your secrets safe inside an AES-256 vault, acts as your 24/7 autonomous homelab administrator, and securely bridges LLMs directly to your home infrastructure.

### ✨ Marquee Features

*   🧠 **Personality Engine V2** — Learns your preferences, tech stack, and workflow over time, adapting to your homelab's unique environment.
*   🛡️ **LLM Guardian** — Real-time AI security engine that scans all incoming data, documents, and tool calls to block prompt-injection attacks.
*   ⚡ **Adaptive Tool Filtering** — Dynamically loads and filters 100+ built-in tools based on conversation context to save LLM tokens and maximize execution accuracy.
*   🔐 **AES-256 Secure Vault** — Securely encrypts and stores your API keys and credentials locally. The agent never sees raw vault secrets, and access is secured via bcrypt and TOTP 2FA.
*   🎨 **Immersive Chat Themes** — Tailor your terminal/web interface with custom retro themes, including *Cyberwar*, *Retro CRT*, *Dark Sun*, and *Lollipop*.
*   📱 **Installable PWA & Mobile** — Fully responsive Web UI installable as a Progressive Web App (PWA) with native mobile styling, full-featured Voice Control, and local Text-To-Speech (TTS).
*   🎬 **Generative Media Registry** — Seamlessly generate images, music, and short video clips using configured local or cloud providers, with full history tracking.
*   🎙️ **Whisper & Piper Integrations** — Native local voice transcription (Whisper) and fast local text-to-speech (Piper) running right on your server.
*   💻 **Virtual Desktop in Browser** — Interact with a virtual GUI environment sandboxed inside your browser for visual and complex tasks.

---

## 🛠️ Integrations & Capabilities at a Glance

AuraGo comes preloaded with **over 100+ native tools**, completely eliminating the need to install unsecured scripts or risky plugins from the internet.

<details>
<summary><b>🏠 Homelab & Infrastructure Orchestration</b> — Docker, Proxmox, Home Assistant, TrueNAS</summary>

*   **Docker** — Inspect, start, stop, restart containers, manage networks/volumes, and control Docker Compose stacks.
*   **Proxmox** — Monitor hypervisor resources, start/stop VMs/LXCs, manage snapshots, and inspect clustering.
*   **Home Assistant** — Monitor sensors and orchestrate home automation devices, scenes, and routines with read-only/write guards.
*   **TrueNAS** — View storage pool status, dataset properties, manage snapshots, and monitor SMB/NFS share status.
*   **Wake-on-LAN** — Power on networked homelab hardware remotely from your agent.
*   **AdGuard Home & Fritz!Box** — Toggle DNS filters, manage blocklists, check bandwidth, monitor router logs, and trigger reconnects.
</details>

<details>
<summary><b>💻 System & Process Automation</b> — Shell Sandboxing, Python, SSH, Ansible</summary>

*   **Isolated Sandboxes** — Run python scripts and shell commands inside locked-down python virtual environments or separate Docker containers.
*   **SSH Inventory** — Securely connect to routers, NAS systems, and remote servers to run commands and collect stats.
*   **Ansible Orchestration** — Trigger playbooks via an Ansible sidecar or the native Ansible API.
*   **Cron & Automated Missions** — Schedule repeating maintenance tasks and trigger automated workflows based on system metrics.
</details>

<details>
<summary><b>☁️ Private Cloud & Web API Integrations</b> — S3, GitHub, Google, OneDrive, Webhooks</summary>

*   **Object Storage (S3)** — Connect to MinIO, Wasabi, Amazon S3, or DigitalOcean Spaces.
*   **Version Control & Repo Sync** — Sync files, manage issues, open Pull Requests, and trigger workflows on GitHub.
*   **Google Workspace & OneDrive** — Access Gmail, update Google Calendar, read sheets, and sync files to Microsoft OneDrive.
*   **Cloudflare Tunnels** — Securely expose your local Web UI without port-forwarding or public IP exposure.
*   **Webhooks** — Receive alerts from Alertmanager, GitHub, or Home Assistant, and dispatch webhooks to external services.
</details>

<details>
<summary><b>📡 Notifications & Messaging Bridges</b> — Telegram, Discord, Email, Voice</summary>

*   **Telegram & Discord Bots** — Control your homelab agent via Telegram and Discord. Send logs, trigger commands, and feed images to the agent via mobile.
*   **ntfy & Pushover** — Deliver instant push notifications directly to your phone for system alerts or mission completions.
*   **Email Sync** — Read incoming server alerts via IMAP and dispatch notifications via SMTP.
*   **IVR & SMS (Telnyx)** — Route incoming calls, trigger automated voice menus, or receive/send SMS.
</details>

<details>
<summary><b>🔧 Developer Tools & Advanced Media</b> — PDF generation, web scrapers, SQL, search</summary>

*   **Web Capture & Scraping** — Take web screenshots, fetch HTML contents, and search using DuckDuckGo/Brave Search.
*   **SQL Client** — Programmatic access to safely query PostgreSQL, MySQL, MariaDB, and SQLite databases.
*   **Document Processing** — Parse PDFs, fill out PDF forms, and generate clean reports/invoices locally.
*   **Network Tools** — Run pings, trace routes, perform local port scanning, and discover mDNS/UPnP services on the LAN.
</details>

---

## ⚡ Quick Start Deployment Options

Deploy AuraGo on your local Linux server or Raspberry Pi using your preferred method:

### Option A — 3-Step Installer (Recommended)

1. **Run the One-Liner Installer**:
   Execute the automated installation script to download dependencies, verify your Docker setup, and establish directories:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
   ```
   *The installer checks Docker, provisions HTTPS for public hostnames, generates a secure first-login password, and sets up optional systemd units.*

2. **Enter Directory & Start Server**:
   Change into your new configuration directory, initialize your environment configurations, and launch the start script:
   ```bash
   cd ~/aurago
   source .env
   ./start.sh
   ```

3. **Open the Interactive Setup Wizard**:
   Navigate to **http://localhost:8088** in your web browser. You'll be greeted by an interactive Web UI Setup Wizard that will guide you through:
   *   Setting up your preferred local or cloud LLM provider (OpenAI, OpenRouter, Local Ollama, etc.)
   *   Creating your secure login password and enabling TOTP 2FA
   *   Initializing your local AES-256 credentials vault
   *   Selecting your active homelab integrations

   No manual `.yaml` or `.json` configuration file editing is required!

---

### Option B — Docker Compose

```yaml
services:
  aurago:
    image: ghcr.io/antibyte/aurago:latest
    ports:
      - "8088:8088"
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/config.yaml
      - ./secrets:/run/optional-secrets:ro
```

Create `./secrets/aurago_master.key` (64 hex characters) or let AuraGo create the vault key in `./data` on first start.

---

### Option C — Build From Source

```bash
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo
go build -o aurago ./cmd/aurago
./aurago
```

---

## First run: setup and configuration

1. **Setup wizard** (`/setup`) - Provider, model, trust level, optional vault and web login. On success you get a clear next step: open chat or jump into key **Config** areas (providers, security, server, backups).
2. **Config UI** (`/config`) - Full settings browser with search, unsaved-change protection when switching sections, sticky save bar, and keyboard-friendly toggles.
3. **Advanced** - Edit `config.yaml` or use env overrides; see [configuration reference](documentation/configuration.md).

No manual YAML is required for a first successful run.

---

## 📸 Screenshots

| Dashboard | Chat | Containers | Configuration |
|:---------:|:----:|:----------:|:-------------:|
| ![Dashboard](documentation/screenshots/dashboard.png) | ![Chat](documentation/screenshots/chat.png) | ![Containers](documentation/screenshots/containers.png) | ![Config](documentation/screenshots/config.png) |

| Virtual desktop (experimental) |
|:------------------------------:|
| ![Virtual Desktop in Browser](documentation/screenshots/desktop.png) |

### Built-in chat themes

| Cyberwar | Retro CRT | Dark Sun | Lollipop |
|:--------:|:---------:|:--------:|:--------:|
| ![Cyberwar](documentation/screenshots/theme1.png) | ![Retro CRT](documentation/screenshots/theme2.png) | ![Dark Sun](documentation/screenshots/theme3.png) | ![Lollipop](documentation/screenshots/theme4.png) |

---

## Highlights

| Area | What you get |
|------|----------------|
| **Personality engine** | Profiles and long-term preference learning |
| **LLM Guardian** | Scans tool calls and external content for risky patterns |
| **Adaptive tools** | Context-aware tool subsets to save tokens |
| **Vault** | AES-256-GCM for secrets; bcrypt + TOTP for web login |
| **Memory** | Short-term history, RAG, knowledge graph, core memory, journal |
| **Media** | Image, music, and video generation with registry and limits |
| **PWA** | Installable UI; voice features need HTTPS |
| **Integrations** | 100+ tools without third-party "skills" for most homelab tasks |

---

## Memory system

| Type | Role |
|------|------|
| **Short-term** | Sliding conversation window (SQLite) |
| **Long-term (RAG)** | Semantic search over past chats |
| **Knowledge graph** | Entities and relations |
| **Core memory** | Always-on facts in context |
| **Journal** | Timestamped events with importance |
| **Notes and to-dos** | Persistent lists with due dates |

Background jobs can consolidate and analyze memory; see the manuals for tuning.

---

## Security

| Layer | Mechanism |
|-------|-----------|
| **Vault** | AES-256-GCM for API keys and secrets |
| **Web auth** | bcrypt passwords, optional TOTP 2FA |
| **Danger zone** | Per-capability toggles for execution and network |
| **LLM Guardian** | Policy checks on tools and ingested content |
| **Sandbox** | Python isolation (venv or containers) |
| **TLS** | Let's Encrypt; login enforced when HTTPS is on |
| **Prompt boundaries** | External payloads wrapped for injection defense |

---

## Chat commands

| Command | Description |
|---------|-------------|
| `/help` | List commands |
| `/reset` | Clear current conversation |
| `/stop` | Cancel in-flight work |
| `/debug on\|off` | Verbose errors |
| `/budget` | Token cost breakdown |
| `/personality <name>` | Switch profile |
| `/restart` | Restart server process |
| `/voice on\|off` | Voice output |
| `/warnings` | Active system warnings |
| `/sudopwd` | Store or clear sudo password in vault |
| `/addssh` | Add SSH host to inventory |
| `/credits` | OpenRouter balance (if applicable) |

---

## Documentation

| Topic | Link |
|-------|------|
| User guide (DE) | [documentation/manual/de/README.md](documentation/manual/de/README.md) |
| User guide (EN) | [documentation/manual/en/README.md](documentation/manual/en/README.md) |
| Configuration | [documentation/configuration.md](documentation/configuration.md) |
| Docker | [documentation/docker_installation.md](documentation/docker_installation.md) |
| Architecture | [documentation/architecture.md](documentation/architecture.md) |
| Telegram | [documentation/telegram_setup.md](documentation/telegram_setup.md) |
| Google OAuth | [documentation/google_setup.md](documentation/google_setup.md) |

Contributors: see [AGENTS.md](AGENTS.md) for build, test, and layout conventions.

---

## Project layout

```
AuraGo/
├── cmd/aurago/       # Main binary
├── internal/         # Agent, memory, tools, HTTP server
├── ui/               # Embedded web UI (go:embed)
├── agent_workspace/  # Skills, sandbox, agent tools
├── prompts/          # System prompts and tool manuals
├── documentation/    # Guides and screenshots
└── config.yaml       # Runtime config (template: config_template.yaml)
```

---

## Development

```bash
go build -o aurago ./cmd/aurago
go test ./...
go test ./ui -count=1    # UI regression tests (config, setup, chat)
```

Use `rtk` prefixed commands in local workflows when [RTK](https://github.com/antibyte/rtk) is installed (see project agent docs).

---

## License

This project is licensed under the [MIT License](LICENSE). Refer to the framework Business Plan for AGPLv3 dual-licensing conversion steps.

---

<details>
<summary><b>Key dependencies</b></summary>

| Library | Purpose |
|---------|---------|
| [go-openai](https://github.com/sashabaranov/go-openai) | OpenAI-compatible LLM client |
| [chromem-go](https://github.com/philippgille/chromem-go) | Embedded vector database |
| [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | Pure Go SQLite |
| [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) | Telegram |
| [discordgo](https://github.com/bwmarrin/discordgo) | Discord |
| [gopsutil](https://github.com/shirou/gopsutil) | System metrics |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | SSH, bcrypt, ACME |
| [cron/v3](https://github.com/robfig/cron) | Scheduler |

</details>

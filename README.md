<!-- logo for light mode -->
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="ui/aurago_logo.png">
  <source media="(prefers-color-scheme: light)" srcset="ui/aurago_logo_dark.png">
  <img alt="AuraGo" src="ui/aurago_logo_dark.png" width="360">
</picture>

# AuraGo — The Self-Hosted AI Agent Framework for Homelabs

**A privacy-first, open-source agentic framework purpose-built for homelabs. Seamlessly monitor your services, automate maintenance, manage media stacks, respond to incidents, and orchestrate home automation—all locally on modest hardware with zero data leaving your home network.**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![Discord](https://img.shields.io/badge/Discord-Join%20Us-5865F2?logo=discord&logoColor=white)](https://discord.gg/aurago)

> **🛠️ Work in Progress** — AuraGo is under active development. Expect rapid feature additions and some rough edges. Windows and macOS support is experimental; Linux servers and Raspberry Pi are fully supported.

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

## ⚡ 3-Step Quick Start

Deploy AuraGo on your local Linux server or Raspberry Pi in less than two minutes:

### 1. Run the One-Liner Installer
Execute the automated installation script to download dependencies, verify your Docker setup, and establish directories:
```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

### 2. Enter Directory & Start Server
Change into your new configuration directory, initialize your environment configurations, and launch the start script:
```bash
cd ~/aurago
source .env
./start.sh
```

### 3. Open the Interactive Setup Wizard
Navigate to **http://localhost:8088** in your web browser. You'll be greeted by an interactive Web UI Setup Wizard that will guide you through:
*   Setting up your preferred local or cloud LLM provider (OpenAI, OpenRouter, Local Ollama, etc.)
*   Creating your secure login password and enabling TOTP 2FA
*   Initializing your local AES-256 credentials vault
*   Selecting your active homelab integrations

No manual `.yaml` or `.json` configuration file editing is required!

---

## 📸 Screenshots

### Web UI Dashboard & Management
| Dashboard | Chat Interface | Container Management | Configuration |
| :---: | :---: | :---: | :---: |
| ![Dashboard](documentation/screenshots/dashboard.png) | ![Chat](documentation/screenshots/chat.png) | ![Containers](documentation/screenshots/containers.png) | ![Config](documentation/screenshots/config.png) |

### Sandbox Virtual Desktop in Browser
| Virtual Desktop GUI |
| :---: |
| ![Virtual Desktop](documentation/screenshots/desktop.png) |

### Cyberpunk & Retro CRT UI Themes
| Cyberwar Theme | Retro CRT Theme | Dark Sun Theme | Lollipop Theme |
| :---: | :---: | :---: | :---: |
| ![Cyberwar](documentation/screenshots/theme1.png) | ![Retro CRT](documentation/screenshots/theme2.png) | ![Dark Sun](documentation/screenshots/theme3.png) | ![Lollipop](documentation/screenshots/theme4.png) |

---

## 🧠 Memory & Context Systems

AuraGo keeps a detailed, persistent understanding of your homelab via a **multi-tiered cognitive architecture**:

*   **Short-Term Window**: Manages immediate conversation history using a sliding SQLite buffer.
*   **Long-Term Memory (RAG)**: Uses an embedded pure-Go vector database (`chromem-go`) to semantically search past interactions.
*   **Knowledge Graph**: Builds a structured mapping of your homelab's entities, devices, IP addresses, and services.
*   **Core Memory**: Stores permanent user profile facts, server specs, and preferences that are always injected into the LLM context.
*   **Reflective Archiving**: Triggers a nightly batch consolidation to archive old threads, scoring logs by importance, and reflecting on your weekly usage patterns to identify optimizations.

---

## 🔒 Security Architecture

| Security Layer | Implemented Protections |
| --- | --- |
| **Encrypted Vault** | Master secrets encrypted locally with AES-256-GCM. Decrypted in-memory only. |
| **Authentication** | Enforced bcrypt password hashing for all sessions, paired with optional TOTP 2FA. |
| **Sandboxed Execution** | Shell/Python execution is fully isolated inside virtual environments or dedicated Docker containers. |
| **LLM Guardian** | Secondary security-specialized LLM that checks the inputs/outputs of actions to prevent indirect injection. |
| **Network & SSL** | Native ACME / Let's Encrypt support for automatic TLS validation on your domain. |
| **Danger Zone** | Hard-coded environment toggles to disable high-risk tools (file write, script execution, network fetch). |

---

## 📖 Documentation Directory

*   **[English User Manual](documentation/manual/en/README.md)** — Comprehensive user and admin guide
*   **[German User Manual (Handbuch)](documentation/manual/de/README.md)** — Complete German documentation
*   **[Deployment Guide](documentation/docker_installation.md)** — Comprehensive Docker and container setup
*   **[Configuration Reference](documentation/configuration.md)** — Explanation of all settings and parameters
*   **[LLM Guardian & Safety](documentation/guardian_llm_system.md)** — Deeper look into prompt-injection defenses
*   **[Architecture Overview](documentation/architecture.md)** — Detailed block diagram of internal components
*   **[Telegram Setup](documentation/telegram_setup.md)** — Configure mobile chat-bot integrations
*   **[Google API Setup](documentation/google_setup.md)** — OAuth2 configuration for Workspace and Gmail

---

## 🤝 Community & Contributing

We welcome all contributions from homelab hobbyists, developers, and sysadmins! 
*   Before contributing, please read our **[Contributing Guide](CONTRIBUTING.md)** and **[Code of Conduct](CODE_OF_CONDUCT.md)**.
*   Have a question, feature request, or cool homelab automation to share? Join us on **[GitHub Discussions](https://github.com/antibyte/AuraGo/discussions)**!

### 🏷️ Discoverability Topics
To help users find this project on GitHub, we suggest configuring the following topics:
`#homelab` `#ai-agent` `#self-hosted` `#go` `#privacy-first` `#docker` `#home-assistant` `#proxmox` `#sysadmin` `#automation`

---

## ⚖️ License & Attribution

This project is licensed under the **AGPLv3 License** — see the [LICENSE](LICENSE) file for details. 

*Includes embedded dependencies like `chromem-go` (embedded vector DB), `modernc.org/sqlite` (pure Go SQLite), and `go-openai` (LLM communication).*

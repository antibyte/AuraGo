# Reddit r/selfhosted & r/homelab Announcement Post

**Target Audience:** r/selfhosted, r/homelab
**Suggested Title:** `[Open Source] AuraGo – A self-hosted, privacy-first AI agent to manage and automate your homelab`

---

Hey self-hosters and homelabbers,

I want to share a project I’ve been actively developing for the last few months to automate my home infrastructure without losing sleep over data privacy: **AuraGo** ([GitHub Repo](https://github.com/antibyte/AuraGo)).

### 🧠 What is AuraGo?

AuraGo is a lightweight, open-source (AGPLv3) AI agentic framework written in pure Go. It runs entirely on your local hardware, has **direct access to your local services**, keeps all passwords and API keys encrypted in an AES-256 vault, and never sends your configurations or logs to a third-party cloud.

It’s designed to act as your 24/7 autonomous homelab administrator. You can chat with it, assign it automated "missions," schedule maintenance crons, or set up mobile triggers.

### 🔌 Out-of-the-Box Homelab Integrations (100+ Tools)

Unlike general web-agents, AuraGo comes built-in with native, secure adapters. You don't have to download third-party Python extensions or shady plugins. Out of the box, you can tell your agent to:

*   **Docker**: "Check why the Nextcloud container is restarting and recreate it if necessary."
*   **Proxmox**: "Take a snapshot of VM 101, then shut it down and allocate more RAM."
*   **Home Assistant**: "Read the temperature sensor in the living room and turn on the fan scene if it's over 24°C" (comes with explicit read-only/write guards).
*   **TrueNAS**: "Check ZFS storage pool health and send me a daily digest."
*   **Networking & Utilities**: Wake-on-LAN remote servers, run local pings/port-scans, query SQLite/PostgreSQL, and trigger custom outgoing webhooks.

### 🛡️ Local-First & Highly Secure

Giving an LLM access to your local infrastructure is inherently a massive security risk. We built AuraGo with safety as a core feature:
1.  **The LLM Guardian**: Every tool call, document analysis, and external webhook payload is scanned in real-time by a secondary, specialized local security LLM to detect and block indirect prompt-injection threats (e.g. if the agent parses a malicious alert email).
2.  **Isolated Python Sandboxing**: Python scripts are executed inside strictly locked down virtual environments or separate Docker containers.
3.  **Hard Toggles (Danger Zone)**: Don't want the agent running shell scripts or hitting the external web? You can completely toggle off filesystem access, shell execution, or network requests at the environment level.
4.  **TOTP 2FA & Secure Vault**: Includes built-in bcrypt credentials, TOTP 2FA login, and ACME (Let's Encrypt) auto-HTTPS.

### ⚡ Built in Go — Lightweight & Raspberry Pi Ready

Most agent frameworks are written in Python and draw massive amounts of idle RAM. We built AuraGo in **Go**:
*   It compiles to a **single binary** with zero OS dependencies.
*   It has an incredibly low footprint and runs smoothly on a **Raspberry Pi 4/5** or older NUC hardware.
*   Uses `chromem-go` (a lightweight, vector DB written in pure Go) for local RAG and long-term memory.
*   Features a responsive Web UI Setup Wizard—so you can configure your Ollama, OpenRouter, or OpenAI connections right from the browser. **No manual YAML config files required.**

### 🎨 CRT Retro & Cyberpunk Themes!

Because it's a homelab project, we had some fun with the UI. The web console has custom themes you can switch between, including a terminal-style **Retro CRT theme**, a cyberpunk **Cyberwar** theme, and a flat **Lollipop** style. (Screenshots are in our README!).

### 📦 Quick Start (One-Liner Install)

You can spin up AuraGo on any Linux server or Pi in under two minutes using our automated installer:

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

Once finished, change directory into `~/aurago`, source the `.env`, run `./start.sh`, and open **http://localhost:8088** to start the interactive web onboarding wizard.

I'm incredibly passionate about building a privacy-first AI standard for the homelab community. I'd love to know what hypervisor adapters or home automation integrations you'd like to see next, and I'd be happy to answer any questions about the security architecture or Go design choices!

*   **GitHub**: [https://github.com/antibyte/AuraGo](https://github.com/antibyte/AuraGo)
*   **Discord**: [https://discord.gg/aurago](https://discord.gg/aurago)

# AuraGo User Manual

Welcome to the AuraGo User Manual – your comprehensive guide to the personal AI agent.

> 📅 **Updated:** April 12, 2026  
> 🔄 **Version:** 2.x compatible  
> 📝 **Last Update:** Documentation sync with current codebase (chat commands, tools, integrations, config)

---

## What is AuraGo?

AuraGo is a fully autonomous AI agent shipped as a single portable binary with an embedded web UI. Connect it to any OpenAI-compatible LLM provider and it becomes a personal assistant that can execute code, manage files, control smart-home devices, send emails, remember everything, and even improve its own source code.

### Highlights

| Feature | Description |
|---------|-------------|
| **🧠 Personality Engine V2** | Learns your preferences and adapts to you |
| **🛡️ LLM Guardian** | AI-based security monitoring |
| **⚡ Adaptive Tools** | Intelligent tool filtering saves tokens |
| **📄 Document AI** | PDF creation and analysis |
| **🔐 AES-256 Vault** | Secure storage of all secrets |
| **🌐 50+ Integrations** | From S3 to OneDrive to TrueNAS |
| **☁️ Cloudflare Tunnel** | Secure remote access without public IP |
| **🗄️ SQL Connections** | Direct database queries (PostgreSQL, MySQL) |
| **📱 Chromecast** | Send TTS and media to Cast devices |
| **🔍 Network Tools** | Ping, port scan, mDNS/UPnP discovery |

---

## Who is this manual for?

| If you are... | Start with... |
|---------------|---------------|
| New to AuraGo | [Chapter 1: Introduction](01-introduction.md) → [Chapter 2: Installation](02-installation.md) |
| Want to get started quickly | [Chapter 3: Quick Start](03-quickstart.md) |
| Want to understand the interface | [Chapter 4: Web UI](04-webui.md) |
| Want to learn about features | [Chapter 6: Tools](06-tools.md) |
| Looking for advanced topics | [Chapters 11-15](11-missions.md) |
| Have a problem | [Chapter 16: Troubleshooting](16-troubleshooting.md) |

---

## Manual Structure

### Part 1: Basics
1. [Introduction](01-introduction.md) – What is AuraGo?
2. [Installation](02-installation.md) – System setup
3. [Quick Start](03-quickstart.md) – First 5 minutes
4. [Web Interface](04-webui.md) – Navigation & UI
5. [Chat Basics](05-chat-basics.md) – Communication

### Part 2: Features in Detail
6. [Tools](06-tools.md) – Using 50+ tools
7. [Configuration](07-configuration.md) – Fine-tuning with provider system
8. [Integrations](08-integrations.md) – Telegram, Discord, email, etc.
9. [Memory & Knowledge](09-memory.md) – Understanding storage
10. [Personality](10-personality.md) – Customizing character

### Part 3: Advanced (Web UI/API)
11. [Mission Control](11-missions.md) – Automation
12. [Invasion Control](12-invasion.md) – Remote deployment
13. [Dashboard](13-dashboard.md) – Analytics & metrics

### Part 4: For Professionals
14. [Security](14-security.md) – Vault, auth, best practices
15. [Co-Agents](15-coagents.md) – Parallel agents
16. [Troubleshooting](16-troubleshooting.md) – Problem solving
17. [Glossary](17-glossary.md) – Terms explained
18. [Appendix](18-appendix.md) – Reference material
19. [Skills](19-skills.md) – Creating custom Python skills

### Part 5: Reference
20. [Chat Commands](20-chat-commands.md) – All available chat commands
21. [API Reference](21-api-reference.md) – Complete REST API documentation
22. [Internal Tools](22-internal-tools.md) – All 100+ internal agent tools

---

## Important Notes

### ⚠️ CLI vs. Web UI

Some advanced features (Mission Control, Invasion Control) are **primarily available via the Web UI and REST API**. CLI commands for these do not exist in the current version.

### 🆕 Provider System (New in 2.x)

The configuration now uses a central provider system for LLM connections. See [Chapter 7: Configuration](07-configuration.md).

### 🔒 Security

> **Important:** AuraGo can execute arbitrary shell commands and modify system files. Never expose the Web UI unprotected to the internet. Always use VPN, reverse proxy with authentication, or firewall rules.

---

## Quick Navigation

### Most important chat commands
```
/help          - Show all commands
/reset         - Clear chat history
/stop          - Cancel current action
/debug on/off  - Toggle debug mode
/budget        - Show cost overview
/personality   - Switch personality
/voice         - Toggle voice output
/warnings      - Show system warnings
/sudo          - Activate sudo mode (requires sudo_enabled)
/sudopwd       - Store sudo password in vault
/journal       - Open journal
/addssh        - Register SSH server
/credits       - Show OpenRouter credits
```

### All Agent Tools
A complete overview of all 100+ internal tools can be found in the [Internal Tools](22-internal-tools.md) section.

### Quick Links
- [Manual Start Page](../README.md)
- [FAQ](faq.md)
- [Complete configuration reference](../../configuration.md)
- [Telegram Setup](../../telegram_setup.md)
- [Docker Installation Guide](../../docker_installation.md)

---

## Updates

| Date | Change |
|------|--------|
| 2026-03 | Revision for version 2.x (Provider system, tool documentation, LLM Guardian) |
| 2026-03 | Added Adaptive Tools documentation |
| 2026-03 | Added Document Creator & PDF Extractor |
| 2026-03 | **Documented SQL Connections, OneDrive, S3, Homepage integrations** |
| 2026-03 | **Added Cloudflare Tunnel, AI Gateway, Chromecast** |
| 2026-03 | **Documented Network Tools, Web Capture, Form Automation** |
| 2026-03 | **Added Skill Manager, Media Registry, Egg Mode** |
| 2026-04 | Documentation sync with current codebase: added chat commands (/voice, /warnings), cleaned up internal tools, corrected integrations, updated config references |
| 2026-03 | **Added Chat Command /sudopwd** |

---

*This manual is continuously updated. The German version can be found [here](../de/README.md).*

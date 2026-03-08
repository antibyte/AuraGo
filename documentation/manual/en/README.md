# AuraGo User Manual

Welcome to the AuraGo User Manual – your comprehensive guide to the personal AI agent.

## What is AuraGo?

AuraGo is a fully autonomous AI agent shipped as a single portable binary with an embedded web UI. Connect it to any OpenAI-compatible LLM provider and it becomes a personal assistant that can execute code, manage files, control smart-home devices, send emails, remember everything, and even improve its own source code.

## Who is this manual for?

| If you are... | Start with... |
|---------------|---------------|
| New to AuraGo | [Chapter 1: Introduction](01-introduction.md) → [Chapter 2: Installation](02-installation.md) |
| Want to get started quickly | [Chapter 3: Quick Start](03-quickstart.md) |
| Want to understand the interface | [Chapter 4: Web UI](04-webui.md) |
| Want to learn about features | [Chapter 6: Tools](06-tools.md) |
| Looking for advanced topics | [Chapters 11-15](11-missions.md) |
| Have a problem | [Chapter 16: Troubleshooting](16-troubleshooting.md) |

## Manual Structure

### Part 1: Basics
1. [Introduction](01-introduction.md) – What is AuraGo?
2. [Installation](02-installation.md) – System setup
3. [Quick Start](03-quickstart.md) – First 5 minutes
4. [Web Interface](04-webui.md) – Navigation & UI
5. [Chat Basics](05-chat-basics.md) – Communication

### Part 2: Features in Detail
6. [Tools](06-tools.md) – Using 30+ tools
7. [Configuration](07-configuration.md) – Fine-tuning
8. [Integrations](08-integrations.md) – Telegram, Discord, etc.
9. [Memory & Knowledge](09-memory.md) – Understanding storage
10. [Personality](10-personality.md) – Customizing character

### Part 3: Advanced
11. [Mission Control](11-missions.md) – Automation
12. [Invasion Control](12-invasion.md) – Remote deployment
13. [Dashboard](13-dashboard.md) – Analytics & metrics

### Part 4: For Professionals
14. [Security](14-security.md) – Vault, auth, best practices
15. [Co-Agents](15-coagents.md) – Parallel agents
16. [Troubleshooting](16-troubleshooting.md) – Problem solving
17. [Glossary](17-glossary.md) – Terms explained
18. [Appendix](18-appendix.md) – Reference material

## Quick Navigation

### Most important chat commands
```
/help          - Show all commands
/reset         - Clear chat history
/stop          - Cancel current action
/debug on/off  - Toggle debug mode
/budget        - Show cost overview
```

### Quick Links
- [Complete configuration reference](../configuration.md)
- [Telegram Setup](../telegram_setup.md)
- [Docker Guide](../docker.md)

## Security Notice

> ⚠️ **Important:** AuraGo can execute arbitrary shell commands and modify system files. Never expose the Web UI unprotected to the internet. Always use VPN, reverse proxy with authentication, or firewall rules.

---

*This manual is continuously updated. The German version can be found [here](../de/README.md).*

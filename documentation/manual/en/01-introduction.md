# Chapter 1: Introduction

Welcome to AuraGo – your personal, autonomous AI agent.

## What is AuraGo?

AuraGo is a fully autonomous AI agent written in Go and shipped as a single portable binary. Unlike simple chatbots, AuraGo can:

- **Execute code** – Python and shell commands in an isolated environment
- **Manage files** – Read, write, organize
- **Control smart homes** – Home Assistant, Chromecast, network devices
- **Send emails** – IMAP/SMTP integration
- **Remember everything** – Short and long-term memory with semantic search
- **Self-improve** – Modify its own source code
- **Parallel tasks** – Co-agents for complex workflows

### The Core Idea

Imagine a personal assistant that:
- **Is available** – 24/7 via Web, Telegram, Discord, or Email
- **Has context** – Remembers all previous conversations and facts
- **Takes action** – Executes tasks, not just gives answers
- **Adapts** – Personality evolves over time
- **Is secure** – AES-256 encryption, vault system, access control

## Who is AuraGo for?

| Profile | Usage |
|---------|-------|
| **Private users** | Personal assistant for daily tasks, research, organization |
| **Developers** | Code reviews, automation, system administration, API testing |
| **System administrators** | Server monitoring, Docker management, backup automation |
| **Smart home enthusiasts** | Central control of all devices, automations |
| **AI researchers** | Experiments with personality engines, co-agents, memory systems |

## Key Features Overview

### 🤖 Agent Core
- **30+ built-in tools** – From filesystem to Docker, from WebDAV to Proxmox
- **Native Function Calling** – OpenAI-compatible tool calls
- **Dynamic tool creation** – Agent can write new Python tools at runtime
- **Multi-step reasoning** – Automatic tool dispatch, error recovery
- **Co-agent system** – Parallel sub-agents for complex tasks

### 🧠 Memory & Knowledge
- **Short-term memory** – SQLite-based conversation history
- **Long-term memory (RAG)** – Vector-based semantic search
- **Knowledge graph** – Structured entities and relationships
- **Core memory** – Permanent facts the agent always remembers
- **Notes & to-dos** – Categorized, prioritized, with due dates

### 🎭 Personality
- **Personality Engine V1** – Heuristic mood and behavior adaptation
- **Personality Engine V2** – Advanced model with LLM-based analysis
- **Built-in personalities** – Friend, professional, punk, neutral, terminator and more
- **Custom profiles** – Create your own personalities

### 🔌 Integrations
- **Web UI** – Complete chat interface with dashboard
- **Telegram** – Voice messages, image analysis, inline commands
- **Discord** – Bot integration with message bridge
- **Email** – IMAP monitoring + SMTP sending
- **Home Assistant** – Smart home control
- **Docker & Proxmox** – Container and VM management
- **Google Workspace** – Gmail, Calendar, Drive, Docs
- **WebDAV/Koofr** – Cloud storage integration

### 🛡️ Security
- **AES-256-GCM vault** – Encrypted storage of all API keys
- **Web UI auth** – Optional with bcrypt password and TOTP 2FA
- **Danger zone** – Granular control over capabilities
- **Sandboxing** – Python runs in isolated venv
- **Rate limiting** – Protection against overload

## Architecture Briefly Explained

```
┌─────────────────────────────────────────────────────────┐
│  User Interfaces                                        │
│  (Web UI / Telegram / Discord / Email)                 │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  AuraGo Core                                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   Agent     │  │   Memory    │  │   Tools     │     │
│  │   Loop      │  │   System    │  │   (30+)     │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│  ┌─────────────┐  ┌─────────────┐                       │
│  │ Personality │  │   Vault     │                       │
│  │   Engine    │  │ (AES-256)   │                       │
│  └─────────────┘  └─────────────┘                       │
└─────────────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│  LLM Provider (OpenAI-compatible)                       │
│  OpenRouter, Ollama, OpenAI, etc.                      │
└─────────────────────────────────────────────────────────┘
```

## Important Security Notes

> ⚠️ **Critical: Isolated Environment**
> AuraGo executes code on your system. It is **strongly recommended** to run AuraGo in an isolated environment:
> - Virtual machine
> - Docker container
> - Dedicated PC/server
>
> LLM errors or misconfigured prompts can have unintended effects.

> ⚠️ **Never expose unprotected**
> The Web UI should never be directly reachable from the internet. Always use:
> - VPN (WireGuard, Tailscale)
> - Reverse proxy with authentication
> - Firewall rules
> - Or the integrated auth with 2FA

## Next Steps

1. **[Installation](02-installation.md)** – Set up AuraGo on your system
2. **[Quick Start](03-quickstart.md)** – First 5 minutes with AuraGo
3. **[Chat Basics](05-chat-basics.md)** – Communicate effectively

---

> 💡 **Tip for beginners:** Start with the Web UI and a simple chat. You'll be surprised how intuitive it is!

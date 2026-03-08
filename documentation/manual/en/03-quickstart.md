# Chapter 3: Quick Start

Your first 5 minutes with AuraGo – from installation to productive chat.

## Prerequisites

- AuraGo is [installed](02-installation.md) and running
- You have configured an [API key](02-installation.md#1-configure-api-key)
- The Web UI is accessible at http://localhost:8088

## The Quick Setup Wizard

On first start, the **Quick Setup Wizard** guides you through the most important settings:

### Step 1: Choose Language
```
🌍 Welcome to AuraGo!
Choose your preferred language:
- English
- Deutsch
- ...
```

> 💡 This setting affects the agent's and Web UI's language.

### Step 2: LLM Provider

If not already set in `config.yaml`:

```
🔌 LLM Provider
Choose your AI provider:
- OpenRouter (recommended) – Access to many models
- Ollama – Local models
- OpenAI – GPT-4, GPT-3.5
- Other (manual configuration)
```

**Recommended for beginners:** OpenRouter with a free model like `arcee-ai/trinity-large-preview:free`.

### Step 3: Choose Personality

```
🎭 Choose a personality:
- Friend – Casual, humorous, chatty
- Professional – Factual, efficient, direct
- Neutral – Balanced, neutral
- Punk – Rebellious, unconventional
```

> 💡 You can change the personality at any time later.

### Step 4: First Integrations (optional)

```
📱 Set up integrations (optional):
- Telegram Bot
- Discord
- Home Assistant
- Skip
```

You can complete these steps later as well.

## The First Chat

After setup, you'll land in the **Chat** – the heart of AuraGo.

### The Chat Interface

```
┌─────────────────────────────────────────────┐
│ ⚡ AURAGO              🌙         ≡         │  ← Header
├─────────────────────────────────────────────┤
│                                             │
│  👤 Hello! How can I help you?             │  ← Agent
│                                             │
│  🤖 Hi! I'm your new assistant.            │  ← You
│                                             │
├─────────────────────────────────────────────┤
│  📎  [Enter message...]         [➤]        │  ← Input
└─────────────────────────────────────────────┘
```

### Your First Messages

**Test 1: Simple greeting**
```
You: Hello!
Agent: Hello! Nice to meet you. How can I help you today?
```

**Test 2: Create a file**
```
You: Create a file test.txt with the content "Hello World"
Agent: ✅ I created the file test.txt.
   📄 Content: "Hello World"
   📁 Path: agent_workspace/workdir/test.txt
```

**Test 3: System information**
```
You: Show me system information
Agent: 🔍 System information:
   💻 CPU: 4 cores, 15% usage
   🧠 RAM: 8 GB total, 3.2 GB free
   💾 Disk: 100 GB total, 45 GB free
   🖥️  OS: Linux x86_64
```

## Important Chat Commands

AuraGo understands special commands starting with `/`:

| Command | Function |
|---------|----------|
| `/help` | Show all available commands |
| `/reset` | Clear chat history, fresh start |
| `/stop` | Cancel running agent action |
| `/debug on` | Show detailed tool outputs |
| `/debug off` | Compact outputs (default) |
| `/budget` | Show today's API costs |
| `/personality friend` | Switch personality |

### Examples

```
You: /help
Agent: 📋 Available commands:
   /help, /reset, /stop, /debug on/off, 
   /budget, /personality <name>

You: /budget
Agent: 💰 Budget Overview (today):
   Input: 1,245 tokens
   Output: 3,892 tokens
   Estimated cost: $0.0023
```

## Uploading Files

1. **Click** the paperclip icon 📎 below the chat
2. **Select** a file
3. **Send** a message with context

```
You: [File: document.pdf]
You: Summarize this document
Agent: 📄 Summary of document.pdf:
   The document covers...
```

> 💡 Supported formats: TXT, PDF, images (JPG, PNG), code files, and more.

## Getting to Know the First Tools

AuraGo has over 30 built-in tools. Here are some to try:

### Filesystem
```
You: List all files in the current directory
You: Create a folder "projects"
You: Read the file config.yaml
```

### Web & Search
```
You: Search the web for "Go 1.21 Release Notes"
You: Fetch the page example.com and show the title
```

### System
```
You: What time is it?
You: Show the current path
You: Which operating system is running here?
```

### Notes
```
You: Save as note: "Don't forget to move tomorrow"
You: Show all my notes
```

## Navigation in the Web UI

Click the **Radial Menu** (☰ top right) for the main menu:

| Area | Function |
|------|----------|
| 💬 Chat | Main chat interface |
| 📊 Dashboard | System metrics and analytics |
| 🚀 Missions | Automated tasks |
| 🥚 Invasion | Remote deployment |
| ⚙️ Config | Edit settings |

## Dark/Light Theme

Click the **Moon/Sun icon** (🌙/☀️) in the header to toggle between dark and light mode.

## Next Steps

| If you want to... | Then... |
|-------------------|---------|
| Learn more about the UI | → [Chapter 4: Web Interface](04-webui.md) |
| Learn to chat better | → [Chapter 5: Chat Basics](05-chat-basics.md) |
| Explore all tools | → [Chapter 6: Tools](06-tools.md) |
| Set up Telegram | → [Chapter 8: Integrations](08-integrations.md) |
| Understand configuration | → [Chapter 7: Configuration](07-configuration.md) |

## Quick Checklist

- [ ] AuraGo installed and started
- [ ] Quick Setup Wizard completed
- [ ] First chat message sent
- [ ] Tried a tool (/help, create file, etc.)
- [ ] Uploaded a file (optional)
- [ ] Switched theme (optional)
- [ ] Clicked all UI areas

> 🎉 **Congratulations!** You have successfully set up AuraGo and taken the first steps.

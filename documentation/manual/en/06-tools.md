# Chapter 6: Tools

AuraGo's **90+ built-in tools** transform it from a chatbot into an autonomous agent.

---

## Tool Categories Overview

| Category | Tools | Danger Zone |
|----------|-------|-------------|
| **🗂️ Filesystem** | Read, write, delete files | Yes |
| **🌐 Web & APIs** | Search, HTTP, scraping, screenshots | No (partial) |
| **🐳 Docker** | Containers, images, networks | Yes |
| **🖥️ Proxmox** | VMs, LXCs, snapshots | Yes |
| **🏠 Smart Home** | Home Assistant, MQTT, Wake-on-LAN | Yes |
| **☁️ Cloud** | Google Workspace, WebDAV, GitHub, S3, OneDrive | No (partial) |
| **📧 Communication** | Email, Telegram, Discord, Telnyx | No |
| **🔧 System** | Metrics, processes, cron, network tools | Partial |
| **🧠 Memory** | Memory, notes, knowledge graph | No |
| **🌐 Network** | Ping, port scan, mDNS, UPnP | No |
| **🖥️ Remote** | SSH, Invasion Control, MeshCentral | Yes |
| **📝 Documents** | PDF Creator/Extractor, Paperless NGX | No |

---

## New Platform Features

The current version includes several powerful extensions:

| Feature | Description |
|---------|-------------|
| **LLM Guardian** | Risk scanning of tool calls and external content before execution |
| **Adaptive Tools** | Token optimization through context-aware tool selection |
| **Document Creator** | PDF creation for invoices, reports |
| **PDF Extractor** | Text extraction with optional LLM summarization |
| **MCP Client/Server** | Model Context Protocol for interoperability |
| **Invasion Control** | Distributed orchestration across multiple hosts |
| **Sudo Execution** | Vault-backed credential handling for privileged commands |

---

## Configuration of Key Tools

### 1. Filesystem & Shell

```yaml
agent:
  allow_shell: true              # Allow shell commands
  allow_python: true             # Allow Python execution
  allow_filesystem_write: true   # Allow file write access
```

### 2. Docker

```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
  # Or for remote Docker:
  # host: "tcp://docker-host:2376"
```

### 3. Proxmox

```yaml
proxmox:
  enabled: true
  url: "https://proxmox.example.com:8006"
  token_id: "root@pam!aurago"
  token_secret: ""              # Stored in vault
  node: "pve"
  insecure: false               # true = accept insecure TLS
```

### 4. Home Assistant

```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  # AccessToken stored in vault
  readonly: false               # true = read-only mode
```

### 5. Google Workspace (OAuth2)

```yaml
agent:
  enable_google_workspace: true
```

Authentication via OAuth2 in the Web UI vault menu.

### 6. Email

```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  imap_port: 993
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  username: "your.email@gmail.com"
  watch_enabled: true
  watch_interval_seconds: 120
```

### 7. Web Search

```yaml
# DuckDuckGo (no API key required)
# Enabled by default

# Brave Search (optional, better results)
brave_search:
  enabled: true
  api_key: "BS..."
  country: "US"
  lang: "en"
```

---

## Adaptive Tools — Intelligent Filtering

**Save tokens, stay focused.** The Adaptive Tools system filters tools based on conversation context:

```yaml
agent:
  adaptive_tools:
    enabled: true
    max_tools: 20               # Max tools in context
    
    # Always available (don't filter):
    always_include:
      - "filesystem"
      - "shell"
      - "query_memory"
```

| Aspect | Without Adaptive | With Adaptive |
|--------|------------------|---------------|
| **Tokens** | 50+ tools in prompt | Only relevant tools |
| **Cost** | Higher | Lower |
| **Accuracy** | LLM overwhelmed | Precise tool selection |

---

## Read-Only vs. Read-Write

Many tools support a read-only mode:

```yaml
tools:
  docker:
    enabled: true
    readonly: true        # List/logs only, no start/stop
  home_assistant:
    enabled: true
    readonly: true        # Query status only, no control
```

---

## Danger Zone

The Danger Zone controls potentially dangerous operations:

```yaml
agent:
  allow_shell: true              # Shell commands
  allow_python: true             # Python execution
  allow_filesystem_write: true   # File write access
  allow_network_requests: true   # HTTP requests
  allow_remote_shell: true       # SSH to remote devices
  allow_self_update: true        # Self-updates
  allow_mcp: true                # MCP protocol
  allow_web_scraper: true        # Web scraping
```

> ⚠️ **Tip:** In production environments, only enable what you really need.

---

## Examples in Chat

### Filesystem
```
You: List all files in the project folder
Agent: 📁 Project folder:
       ├── main.go
       ├── go.mod
       └── config.yaml

You: Create a file notes.txt with "Important ideas"
Agent: ✅ File created: notes.txt
```

### Docker
```
You: Show all running containers
Agent: 🐳 Running containers:
       ┌─────────┬────────┬─────────────┐
       │ NAME    │ STATUS │ PORTS       │
       ├─────────┼────────┼─────────────┤
       │ nginx   │ Up 2h  │ 80→8080/tcp │
       │ postgres│ Up 2h  │ 5432/tcp    │
       └─────────┴────────┴─────────────┘

You: Restart the redis container
Agent: 🔄 Restarting container redis...
       ✅ Success (3.2s)
```

### Home Assistant
```
You: Turn off the living room light
Agent: 🏠 Smart Home:
       ✅ Living Room Light: OFF

You: What's the temperature in the bedroom?
Agent: 🌡️ Bedroom Sensor:
       Temperature: 21.5°C
       Humidity: 45%
```

### Web Search
```
You: Search for Go best practices
Agent: 🔍 Searching...
       Found: 5 results
       1. Go Code Review Comments
       2. Effective Go
       ...
```

---

## Creating Custom Tools

AuraGo can create new Python tools at runtime:

```
You: Create a tool that converts temperatures
Agent: 🛠️ Creating temperature converter...
       ✅ Created: temperature_converter.py
       
You: What's 25°C in Fahrenheit?
Agent: 🌡️ 25°C = 77°F
```

Created tools are saved to `agent_workspace/tools/` and available immediately.

---

## Summary

| Category | Highlights |
|----------|------------|
| **90+ Tools** | For nearly any home lab task |
| **Security** | Read-only mode, Danger Zone, LLM Guardian |
| **Flexibility** | Dynamic tool creation at runtime |
| **Efficiency** | Adaptive Tools save tokens |

> 💡 **Remember:** AuraGo's strength lies in combining tools. A single chat can chain shell commands, web searches, Docker operations, and email notifications — fully automatically!

---

**Next Steps**

- **[Configuration](07-configuration.md)** — Fine-tune tools
- **[Integrations](08-integrations.md)** — Connect external services

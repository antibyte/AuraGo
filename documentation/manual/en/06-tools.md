# Chapter 6: Tools

AuraGo's 50+ built-in tools transform it from a chatbot into an autonomous agent. This chapter covers all available tools, how to use them, and how to create your own.

## Overview of 50+ Built-in Tools

AuraGo comes with a comprehensive toolkit organized into logical categories. Each tool is a capability the agent can invoke to interact with the world.

### Complete Tool Inventory

| # | Tool | Purpose | Mode |
|---|------|---------|------|
| 1 | **filesystem** | File operations (read, write, list, delete) | RW |
| 2 | **shell** | Execute shell commands | RW |
| 3 | **python** | Run Python code in sandbox | RW |
| 4 | **web_search** | Search the web (DuckDuckGo) | RO |
| 5 | **fetch_url** | Download and parse web pages | RO |
| 6 | **docker** | Manage containers, images, networks | RW |
| 7 | **proxmox** | VM and LXC management | RW |
| 8 | **home_assistant** | Smart home device control | RW |
| 9 | **google_workspace** | Gmail, Calendar, Drive, Docs | RW |
| 10 | **email** | Send emails via SMTP | RW |
| 11 | **email_watcher** | Monitor IMAP inbox | RO |
| 12 | **webdav** | Cloud storage operations | RW |
| 13 | **koofr** | Koofr cloud storage access | RW |
| 14 | **system_metrics** | CPU, RAM, disk monitoring | RO |
| 15 | **process** | Process management | RW |
| 16 | **cron** | Scheduled task management | RW |
| 17 | **notes** | Persistent notes and to-dos | RW |
| 18 | **memory** | Store and retrieve facts | RW |
| 19 | **chromecast** | Audio streaming and control | RW |
| 20 | **tts** | Text-to-speech | RO |
| 21 | **transcribe** | Speech-to-text (Whisper) | RO |
| 22 | **vision** | Image analysis | RO |
| 23 | **ollama** | Local LLM management | RW |
| 24 | **tailscale** | VPN network operations | RO |
| 25 | **wake_on_lan** | Wake remote devices | RW |
| 26 | **mdns** | Network service discovery | RO |
| 27 | **git** | Git operations | RW |
| 28 | **github** | GitHub API interactions | RW |
| 29 | **wikipedia** | Wikipedia article search | RO |
| 30 | **notification** | Push notifications (ntfy/Pushover) | RW |
| 31 | **ansible** | Configuration management | RW |
| 32 | **surgery** | Self-modification capability | RW |

> 💡 **Tip:** The agent automatically selects appropriate tools based on your requests. You don't need to memorize tool names – just describe what you want to achieve.

---


## Recent platform additions

The current codebase includes several newer capabilities that are easy to miss when reading older docs:

- **LLM Guardian** for tool-call and content risk screening before execution.
- **Adaptive Tools** to reduce token usage by selecting context-relevant tools.
- **Document Creator + PDF Extractor** for PDF generation and extraction workflows.
- **MCP client + MCP server mode** for Model Context Protocol interoperability.
- **Invasion Control + Remote Control** for distributed multi-host orchestration.
- **Sudo execution tooling** with vault-backed credential handling and explicit gating.

> See also: [Chapter 8: Integrations](08-integrations.md), [Chapter 13: Dashboard](13-dashboard.md), and [Chapter 14: Security](14-security.md).

---
## Tool Categories

### Filesystem Tools

File operations with safety controls and path validation.

| Action | Example Query |
|--------|---------------|
| List files | `List files in /documents` |
| Read file | `Show me config.yaml` |
| Write file | `Create a file called test.txt with "Hello"` |
| Delete | `Delete old_logs.zip` |
| Move/Copy | `Copy report.pdf to /backup` |
| Search | `Find all Go files modified today` |

**Example Conversation:**
```
You: List all files in my workspace
Agent: 📁 Workspace contents:
   - main.go
   - README.md
   - config/
   - data/

You: Read main.go and show the first 50 lines
Agent: 📄 main.go (lines 1-50):
   package main
   
   import (
       "fmt"
       ...
```

### Web/APIs Tools

Interact with the internet and external services.

| Tool | Capability | Example |
|------|------------|---------|
| `web_search` | DuckDuckGo search | `Search for "Go 1.22 release"` |
| `fetch_url` | Download pages | `Fetch https://example.com` |
| `api_client` | HTTP requests | `POST to API with JSON payload` |
| `scraper` | Extract data | `Scrape all links from page` |

**Example Conversation:**
```
You: Search for latest Docker Compose documentation
Agent: 🔍 Searching...
   Found 5 results:
   1. Docker Compose Specification
   2. Compose File Reference
   ...

You: Fetch the first result and summarize it
Agent: 📄 Summary of Docker Compose docs:
   Docker Compose is a tool for defining...
```

### Docker Tools

Complete container and image management.

| Capability | Commands |
|------------|----------|
| **Containers** | List, start, stop, restart, remove, inspect |
| **Images** | List, pull, remove, build |
| **Networks** | List, create, remove |
| **Volumes** | List, create, remove, inspect |
| **Logs** | View, follow, search |
| **Compose** | Up, down, ps, logs |

**Example Conversation:**
```
You: Show all running containers
Agent: 🐳 Running Containers:
   ┌──────────┬────────┬─────────────┬────────────────┐
   │ NAME     │ STATUS │ PORTS       │ IMAGE          │
   ├──────────┼────────┼─────────────┼────────────────┤
   │ nginx    │ Up 2h  │ 80→8080/tcp │ nginx:latest   │
   │ postgres │ Up 2h  │ 5432/tcp    │ postgres:15    │
   │ redis    │ Up 2h  │ 6379/tcp    │ redis:alpine   │
   └──────────┴────────┴─────────────┴────────────────┘

You: Show logs for nginx (last 20 lines)
Agent: 📋 nginx logs:
   192.168.1.10 - - [12/Jan/2024:10:23:15] "GET /api..."
   ...

You: Restart the postgres container
Agent: 🔄 Restarting postgres...
   ✅ Container restarted (took 3.2s)
```

### Proxmox Tools

Virtual machine and LXC container management.

| Capability | Description |
|------------|-------------|
| List VMs/LXCs | View all virtual machines |
| Start/Stop | Power control |
| Status | Resource usage and health |
| Snapshots | Create, list, rollback, remove |
| Console | Access via VNC/SPICE |

**Example Conversation:**
```
You: List all VMs on my Proxmox server
Agent: 🖥️ Proxmox VMs:
   ┌──────┬────────────┬────────┬──────────┬─────────┐
   │ VMID │ NAME       │ STATUS │ CPU      │ MEM     │
   ├──────┼────────────┼────────┼──────────┼─────────┤
   │ 100  │ web-server │ running│ 12%      │ 4GB/8GB │
   │ 101  │ db-server  │ running│ 45%      │ 6GB/8GB │
   │ 102  │ backup-vm  │ stopped│ -        │ -       │
   └──────┴────────────┴────────┴──────────┴─────────┘

You: Create a snapshot of db-server called pre-update
Agent: 📸 Creating snapshot...
   ✅ Snapshot "pre-update" created for VM 101
   ⏱️  Time: 2.3s
   💾 Size: 245 MB
```

> ⚠️ **Warning:** Proxmox operations affect production infrastructure. Use carefully, especially with start/stop commands.

### Smart Home Tools

Control Home Assistant devices and automations.

| Capability | Examples |
|------------|----------|
| List devices | `Show all lights` / `List sensors` |
| Control states | `Turn on living room light` |
| Read sensors | `What's the temperature?` |
| Run services | `Activate scene "Movie Night"` |
| Automations | `List automations` / `Trigger alarm` |

**Example Conversation:**
```
You: Turn off all lights in the house
Agent: 🏠 Smart Home:
   Turning off 8 lights...
   ✅ Living room: OFF
   ✅ Kitchen: OFF
   ✅ Bedroom: OFF
   ...

You: What's the temperature in the bedroom?
Agent: 🌡️ Bedroom Sensor:
   Temperature: 21.5°C
   Humidity: 45%
   Last update: 2 min ago

You: Set living room light to 50% warm white
Agent: 💡 Living Room Light:
   Brightness: 50%
   Color: Warm White (2700K)
   ✅ Applied
```

### Google Workspace Tools

Manage Gmail, Calendar, Drive, and Docs.

| Service | Capabilities |
|---------|--------------|
| **Gmail** | Send, read, search, labels |
| **Calendar** | List events, create, delete |
| **Drive** | List, upload, download, share |
| **Docs** | Create, read, edit |

**Example Conversation:**
```
You: Send an email to team@company.com about tomorrow's meeting
Agent: 📧 Compose email:
   To: team@company.com
   Subject: Meeting Tomorrow
   Body: [Generated content]
   
   Send? (yes/no)

You: Yes, send it
Agent: ✅ Email sent successfully
   Message ID: <abc123@mail.gmail.com>

You: What's on my calendar for today?
Agent: 📅 Today's Events:
   10:00 - Standup meeting
   14:00 - Project review
   16:30 - 1:1 with Sarah
```

> 💡 **Tip:** Google Workspace requires OAuth setup. See [Google Setup Guide](../../google_setup.md) for configuration.

### Email Tools

IMAP monitoring and SMTP sending.

| Capability | Description |
|------------|-------------|
| Send email | SMTP with attachments |
| Watch inbox | IMAP idle monitoring |
| Search | Query by sender, subject, date |
| Templates | Pre-defined message formats |

**Example Conversation:**
```
You: Send email to admin@example.com: "Server backup completed"
Agent: 📧 Sending email...
   ✅ Sent successfully
   
You: Check for new emails from support
Agent: 🔍 Searching inbox...
   Found 3 emails from support:
   1. RE: Ticket #1234
   2. Scheduled maintenance notice
   3. Password reset request
```

### System Tools

Monitor and manage the host system.

| Tool | Purpose |
|------|---------|
| `system_metrics` | CPU, RAM, disk, network stats |
| `process` | List, kill, monitor processes |
| `shell` | Execute arbitrary commands |
| `cron` | Schedule recurring tasks |
| `tailscale` | VPN network status |
| `wake_on_lan` | Wake remote computers |

**Example Conversation:**
```
You: Show system resource usage
Agent: 📊 System Metrics:
   💻 CPU: 23% (4 cores)
   🧠 RAM: 8.2 GB / 16 GB (51%)
   💾 Disk: 234 GB / 500 GB (47%)
   🌐 Network: 1.2 MB/s in, 0.8 MB/s out

You: Show top 5 processes by CPU
Agent: 🔥 Top Processes:
   1. python (42%) - Data processing
   2. dockerd (15%) - Container daemon
   3. postgres (8%) - Database
   ...
```

---

## How to Use Tools in Chat

### Natural Language Invocation

The agent automatically determines which tools to use:

```
You: Create a backup of the database
Agent: [Automatically uses shell, docker, or postgres tools]
```

### Explicit Tool Requests

You can also specify tools explicitly:

```
You: Use the docker tool to list all images
You: Search web for "Go best practices"
You: Check system metrics
```

### Chaining Multiple Tools

Complex tasks often involve multiple tools:

```
You: Find all large log files, compress them, and email the archive
Agent: 🔍 Finding log files...
   [filesystem] Found 5 files (>100MB)
   
   📦 Compressing...
   [shell] Created logs_backup.tar.gz (245MB)
   
   📧 Sending email...
   [email] Archive sent to your address
```

### Tool Parameters

When precision matters, provide parameters:

```
You: Search web query="Docker Compose tutorial" max_results=3
You: Create file path="config.yml" content="[content]" overwrite=true
You: Docker container_start name="nginx" wait=true
```

---

## Tool Output Interpretation

### Understanding Tool Responses

Each tool returns structured output that the agent interprets:

```
[Tool: filesystem.list_files]
Result: {
  "path": "/workspace",
  "files": [
    {"name": "main.go", "size": 2450, "modified": "2024-01-15T10:30:00Z"},
    {"name": "README.md", "size": 1200, "modified": "2024-01-14T16:45:00Z"}
  ]
}
```

The agent presents this in human-readable format:
```
Agent: 📁 /workspace:
   main.go    2.4 KB  Jan 15 10:30
   README.md  1.2 KB  Jan 14 16:45
```

### Debug Mode

Enable debug mode to see raw tool outputs:

```
You: /debug on
You: List files
Agent: 📁 Files:
   
   [DEBUG] Tool: filesystem.list
   [DEBUG] Parameters: {"path": ".", "recursive": false}
   [DEBUG] Duration: 23ms
   [DEBUG] Raw result: [...]
   
   - file1.txt
   - file2.txt
```

### Error Handling

When tools fail, the agent provides details:

```
You: Read file /etc/secret.conf
Agent: ❌ Error reading file:
   Permission denied: /etc/secret.conf
   
   Suggestions:
   - Check file permissions
   - Run with elevated privileges (if safe)
   - Specify a different file
```

---

## Read-Only vs Read-Write Modes

### Understanding Tool Safety Levels

| Level | Description | Examples |
|-------|-------------|----------|
| **Read-Only (RO)** | Safe observation tools | web_search, system_metrics, vision |
| **Read-Write (RW)** | Can modify state | filesystem, shell, docker, email |

### Default Safety Settings

AuraGo defaults to conservative settings:
- Read-only tools: **Enabled**
- Read-write tools: **Disabled** (in Danger Zone)

### Enabling Write Capabilities

Access the **Danger Zone** in Web UI:

1. Open Web UI → Configuration → Danger Zone
2. Toggle specific capabilities:
   - ✓ Filesystem access
   - ✓ Shell execution
   - ✓ Python execution
   - ✓ Docker management
   - ✓ Email sending
   - ✓ Self-update

> ⚠️ **Warning:** Enable only what you need. Each write capability increases risk.

---

## Danger Zone Explanation

The Danger Zone is AuraGo's security control center for potentially destructive operations.

### Danger Zone Categories

| Category | Risks | Use Cases |
|----------|-------|-----------|
| **Shell** | Arbitrary command execution | System administration, automation |
| **Python** | Code execution in sandbox | Custom scripts, data processing |
| **Filesystem** | File creation/deletion/modification | File management, backups |
| **Network** | Outbound connections | API calls, web scraping |
| **Docker** | Container/lifecycle management | DevOps, deployment |
| **Email** | Sending emails | Notifications, reports |
| **Self-Update** | Binary modification | Automatic updates |

### Danger Zone Interface

```
┌─────────────────────────────────────┐
│ ⚠️  DANGER ZONE                     │
│                                     │
│  Shell Execution      [OFF] ●───○   │
│  Python Execution     [OFF] ●───○   │
│  Filesystem Access    [ON]  ○───●   │
│  Docker Management    [OFF] ●───○   │
│  Email Sending        [ON]  ○───●   │
│  Self-Update          [OFF] ●───○   │
│                                     │
│  [Reset to Defaults]  [Save]        │
└─────────────────────────────────────┘
```

### Best Practices

1. **Start minimal** - Enable only what you need
2. **Use read-only** - When possible, prefer observation
3. **Sandbox** - Run AuraGo in VM/container for isolation
4. **Audit** - Review what the agent does in debug mode
5. **Reset** - Clear history when testing unknown operations

> 🔍 **Deep Dive:** The Danger Zone stores settings in `config.yaml` under `danger_zone` section. Changes take effect immediately without restart.

---

## Creating Custom Tools

AuraGo can create new Python tools at runtime, extending its capabilities.

### How Custom Tools Work

1. Describe what you need
2. Agent generates Python code
3. Code is saved to `agent_workspace/tools/`
4. Tool is registered automatically
5. Available immediately for use

### Creating a Simple Tool

```
You: Create a tool that converts temperatures between Celsius and Fahrenheit
Agent: 🛠️ Creating temperature converter...
   ✅ Created: temperature_converter.py
   
   Functions:
   - celsius_to_fahrenheit(c: float) -> float
   - fahrenheit_to_celsius(f: float) -> float

You: Convert 25°C to Fahrenheit
Agent: 🌡️ 25°C = 77°F
```

### Tool Structure

Generated tools follow this pattern:

```python
# agent_workspace/tools/my_tool.py

def my_function(param1: str, param2: int = 10) -> dict:
    """
    Description of what this function does.
    
    Args:
        param1: Description of parameter
        param2: Description with default
        
    Returns:
        Dictionary with results
    """
    result = do_something(param1, param2)
    return {
        "success": True,
        "result": result,
        "message": f"Processed {param1}"
    }
```

### Advanced Custom Tool Example

```
You: Create a tool that fetches cryptocurrency prices
Agent: 🛠️ Creating crypto price tool...
   ✅ Created: crypto_prices.py
   
   Features:
   - Real-time price fetching via CoinGecko API
   - Support for BTC, ETH, and 100+ currencies
   - Price change percentages (24h, 7d)
   - Multi-currency conversion

You: Get Bitcoin price in USD and EUR
Agent: ₿ Bitcoin Prices:
   USD: $43,250.00 (+2.3% 24h)
   EUR: €39,840.00 (+2.1% 24h)
```

### Managing Custom Tools

| Action | Command |
|--------|---------|
| List custom tools | `List all custom tools` |
| View tool code | `Show me the code for [tool_name]` |
| Update tool | `Update [tool_name] to also do [feature]` |
| Delete tool | `Delete custom tool [tool_name]` |

> 💡 **Tip:** Custom tools persist across restarts. They're stored in `agent_workspace/tools/` with a manifest file.

---

## Tool Examples with Practical Use Cases

### Use Case 1: Automated Backup

```
You: Create a daily backup script for my database
Agent: 🛠️ Creating backup solution...
   
   [Step 1] Created backup script
   [Step 2] Scheduled via cron (daily 2 AM)
   [Step 3] Added notification on completion
   
   Components:
   - backup_database.py: Performs pg_dump
   - Cron job: 0 2 * * *
   - Notification: Email on success/failure

You: Test the backup now
Agent: 🧪 Running test backup...
   ✅ Backup completed: backup_20240115_143022.sql (245 MB)
   ⏱️ Duration: 12s
   📧 Notification sent
```

### Use Case 2: Website Monitoring

```
You: Monitor my website and alert me if it goes down
Agent: 🔍 Setting up monitoring...
   
   Created: website_monitor.py
   
   Configuration:
   - URL: https://mywebsite.com
   - Check interval: 5 minutes
   - Timeout: 10 seconds
   - Alert via: Email + Pushover
   
   Checks:
   ✓ HTTP 200 status
   ✓ Response time < 2s
   ✓ SSL certificate validity

You: Show me the last check results
Agent: 📊 Last Check (2 min ago):
   Status: ✅ UP
   Response time: 145ms
   HTTP code: 200
   SSL: Valid (expires in 87 days)
```

### Use Case 3: Smart Home Automation

```
You: Create a movie night scene
Agent: 🎬 Creating "Movie Night" scene...
   
   Scene Configuration:
   ├─ Living Room Light: OFF
   ├─ TV Backlight: ON (purple, 20%)
   ├─ Thermostat: 21°C
   ├─ Blinds: CLOSED
   └─ Chromecast: "Ready to cast"

You: Activate movie night
Agent: 🍿 Activating Movie Night...
   ✅ Living Room Light: OFF
   ✅ TV Backlight: ON (purple, 20%)
   ✅ Thermostat: Set to 21°C
   ✅ Blinds: Closing...
   ✅ Chromecast: Ready
   
   Scene active. Enjoy your movie!
```

### Use Case 4: Log Analysis

```
You: Analyze nginx logs for errors in the last hour
Agent: 🔍 Analyzing logs...
   
   [shell] Parsed /var/log/nginx/access.log
   [python] Processed 15,234 lines
   
   Summary:
   - Total requests: 15,234
   - Error rate: 0.8% (122 errors)
   - Top errors:
     * 404: 67 (missing resources)
     * 500: 34 (server errors)
     * 503: 21 (service unavailable)
   
   Top 404 URLs:
   1. /old-page.html (23 hits)
   2. /api/v1/deprecated (18 hits)

You: Create a report and save it
Agent: 📊 Created: nginx_report_20240115.html
   - Interactive charts
   - Error breakdown
   - Recommendations
```

### Use Case 5: Development Workflow

```
You: Set up a CI/CD pipeline check for my project
Agent: 🚀 Creating CI/CD helper...
   
   Tools created:
   1. pre_commit_check.py
      - Lint (golangci-lint)
      - Tests (go test)
      - Security scan (gosec)
   
   2. git_hooks_installer.sh
      - Installs pre-commit hooks
   
   3. pr_validator.py
      - Checks PR description
      - Validates branch naming
      - Links to tickets

You: Run pre-commit checks now
Agent: 🧪 Running checks...
   ✓ go fmt
   ✓ go vet
   ✓ golangci-lint
   ✓ go test (42/42 passed)
   ⚠ gosec: 2 low-severity issues found
   
   Pre-commit passed with warnings
```

### Use Case 6: Data Processing

```
You: I have a CSV with sales data. Help me analyze it.
Agent: 📊 Ready to analyze. Please upload the CSV.

You: [Upload: sales_2023.csv]
You: Show monthly revenue trends
Agent: 📈 Monthly Revenue 2023:
   
   January:  $45,230  ████████░░
   February: $52,100  █████████░
   March:    $48,500  ████████░░
   ...
   December: $78,400  █████████████
   
   Total: $675,430
   Growth: +73% YoY

You: Create a visualization and save it
Agent: 📊 Generated: sales_chart_2023.png
   - Line chart with trend
   - Bar chart by month
   - Saved to: /workspace/reports/
```

---

## Tool Reference Quick Guide

### Most Common Patterns

| I want to... | Say... |
|--------------|--------|
| Find information | `Search for...` / `Look up...` |
| Read a file | `Show me [filename]` / `Read...` |
| Write a file | `Create a file called...` |
| Run a command | `Execute...` / `Run...` |
| Check system | `Show system...` / `What's the status...` |
| Manage Docker | `Docker...` / `Container...` |
| Control smart home | `Turn on/off...` / `Set temperature...` |
| Send notification | `Send email...` / `Notify me...` |
| Schedule something | `Create a cron job...` / `Schedule...` |
| Remember something | `Remember that...` / `Save this...` |

---

> 🎓 **Next Chapter:** [Chapter 7: Configuration](07-configuration.md) – Learn how to fine-tune AuraGo's settings for your needs.

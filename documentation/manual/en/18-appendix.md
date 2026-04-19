# Chapter 18: Appendix

Comprehensive reference material for AuraGo including complete configuration options, API endpoints, command reference, and additional resources.

---

## Table of Contents

1. [Complete Configuration Reference](#complete-configuration-reference)
2. [API Endpoints](#api-endpoints)
3. [Chat Commands Reference](#chat-commands-reference)
4. [Tool Reference](#tool-reference)
5. [Example Configurations](#example-configurations)
6. [Environment Variables](#environment-variables)
7. [File Paths and Locations](#file-paths-and-locations)
8. [Update History / Changelog](#update-history--changelog)
9. [Useful Resources and Links](#useful-resources-and-links)
10. [License Information](#license-information)

---

## Complete Configuration Reference

### Full config.yaml Structure

```yaml
# ============================================
# AuraGo Configuration Reference
# Copy this file and fill in your values
# ============================================

# Server Settings
# ---------------
server:
  host: "127.0.0.1"           # Bind address (use 0.0.0.0 for LAN access)
  port: 8088                  # HTTP server port
  bridge_address: "localhost:8089"  # Internal bridge port
  max_body_bytes: 10485760    # Max request body (10MB)

# LLM Configuration
# -----------------
llm:
  provider: "openrouter"      # Provider name (informational)
  base_url: "https://openrouter.ai/api/v1"  # API endpoint
  api_key: ""                 # Your API key (REQUIRED)
  model: "arcee-ai/trinity-large-preview:free"  # Model ID
  use_native_functions: true  # Use OpenAI function calling
  temperature: 0.7            # Randomness (0.0-2.0)
  structured_outputs: false   # Enable constrained decoding

# Embeddings (for Long-Term Memory)
# ---------------------------------
embeddings:
  provider: "internal"        # internal/external/disabled
  external_url: "http://localhost:11434/v1"  # For external provider
  external_model: "nomic-embed-text"  # External model name
  api_key: "dummy_key"        # API key for external provider
  internal_model: "qwen/qwen3-embedding-8b"  # Internal model

# Agent Behavior
# --------------
agent:
  system_language: "Deutsch"  # Response language
  max_tool_calls: 12          # Max tool calls per request
  step_delay_seconds: 0       # Delay between calls (rate limiting)
  memory_compression_char_limit: 50000  # Compress context at this size
  personality_engine: true    # Enable mood adaptation (V1)
  personality_engine_v2: true # Enable advanced personality (V2)
  personality_v2_model: "qwen/qwen-2.5-7b-instruct"
  personality_v2_url: ""      # Custom V2 endpoint
  personality_v2_api_key: ""  # V2 API key
  core_personality: "friend"  # Base personality
  show_tool_results: false    # Show raw tool output
  debug_mode: false           # Enable debug instructions
  system_prompt_token_budget: 8192  # System prompt soft limit
  context_window: 131000      # Model context window (0=auto)
  core_memory_cap_mode: "soft"  # soft/hard cap mode
  core_memory_max_entries: 200  # Max core memory entries
  workflow_feedback: true     # Enable workflow feedback
  enable_google_workspace: true  # Enable Google Workspace tools

# Authentication
# --------------
auth:
  enabled: false              # Enable Web UI login
  password_hash: ""           # bcrypt hashed password
  session_secret: ""          # Session encryption key
  session_timeout_hours: 24   # Session duration
  totp_secret: ""             # TOTP secret for 2FA
  totp_enabled: false         # Enable 2FA
  max_login_attempts: 5       # Failed attempts before lockout
  lockout_minutes: 15         # Lockout duration

# Budget Tracking
# ---------------
budget:
  enabled: false              # Enable cost tracking
  daily_limit_usd: 1          # Daily spending limit
  warning_threshold: 0.8      # Warning at 80% of limit
  reset_hour: 0               # Reset time (midnight)
  enforcement: "warn"         # warn/partial/full
  default_cost:
    input_per_million: 1      # Default input cost per 1M tokens
    output_per_million: 3     # Default output cost per 1M tokens
  models:                     # Per-model pricing
    - name: "model-name"
      input_per_million: 0.5
      output_per_million: 1.5

# Circuit Breaker (Safety Limits)
# -------------------------------
circuit_breaker:
  max_tool_calls: 20          # Hard limit on tool calls
  llm_timeout_seconds: 180    # LLM call timeout
  maintenance_timeout_minutes: 10  # Maintenance timeout
  retry_intervals:            # Backoff intervals
    - "10s"
    - "2m"
    - "10m"

# Fallback LLM
# ------------
fallback_llm:
  enabled: false              # Enable failover
  base_url: "https://openrouter.ai/api/v1"
  api_key: ""                 # Fallback API key
  model: "meta-llama/llama-3.1-8b-instruct:free"
  error_threshold: 2          # Errors before failover
  probe_interval_seconds: 60  # Recovery check interval

# Co-Agents (Parallel Sub-Agents)
# -------------------------------
co_agents:
  enabled: true               # Enable co-agent system
  max_concurrent: 3           # Max parallel co-agents
  llm:
    provider: "openrouter"
    base_url: ""              # Fallback to main LLM
    api_key: ""               # Fallback to main LLM
    model: "stepfun/step-3.5-flash"
  circuit_breaker:
    max_tool_calls: 12
    timeout_seconds: 300
    max_tokens: 0             # 0 = unlimited

# Telegram Integration
# --------------------
telegram:
  bot_token: ""               # Bot token from @BotFather
  telegram_user_id: 0         # Your numeric user ID
  max_concurrent_workers: 5   # Message processing workers

# Discord Integration
# -------------------
discord:
  enabled: false
  bot_token: ""
  guild_id: ""                # Server ID
  default_channel_id: ""      # Default channel
  allowed_user_id: ""         # Restrict to specific user

# Email Integration
# -----------------
email:
  enabled: false
  imap_host: ""               # IMAP server
  imap_port: 993              # IMAP port
  smtp_host: ""               # SMTP server
  smtp_port: 587              # SMTP port
  username: ""
  password: ""                # App password
  from_address: ""            # Sender address
  watch_enabled: false        # Auto-check inbox
  watch_interval_seconds: 120 # Check frequency
  watch_folder: "INBOX"       # Folder to watch

# Home Assistant
# --------------
home_assistant:
  enabled: false
  url: "http://localhost:8123"
  access_token: ""            # Long-lived access token

# Docker Integration
# ------------------
docker:
  enabled: false
  host: ""                    # Docker socket/path

# Proxmox Integration
# -------------------
proxmox:
  enabled: false
  host: ""                    # Proxmox host
  username: ""
  password: ""                # In vault
  verify_ssl: true

# Chromecast
# ----------
chromecast:
  enabled: true
  tts_port: 8090              # TTS streaming port

# Text-to-Speech
# --------------
tts:
  provider: "google"          # google/elevenlabs
  language: "de"              # Language code
  elevenlabs:
    api_key: ""
    voice_id: ""
    model_id: "eleven_multilingual_v2"

# Vision (Image Analysis)
# -----------------------
vision:
  provider: "openrouter"
  base_url: "https://openrouter.ai/api/v1"
  api_key: ""                 # Falls back to llm.api_key
  model: "google/gemini-2.5-flash-lite-preview-09-2025"

# Whisper (Speech-to-Text)
# ------------------------
whisper:
  provider: "openrouter"
  base_url: "https://openrouter.ai/api/v1"
  api_key: ""                 # Falls back to llm.api_key
  model: "google/gemini-2.5-flash-lite-preview-09-2025"

# WebDAV Integration
# ------------------
webdav:
  enabled: false
  url: ""                     # WebDAV endpoint
  username: ""
  password: ""

# Koofr Integration
# -----------------
koofr:
  enabled: false
  base_url: "https://app.koofr.net"
  username: ""
  app_password: ""

# MQTT Integration
# ----------------
mqtt:
  enabled: false
  broker: ""                  # MQTT broker address
  client_id: "aurago"
  username: ""
  password: ""
  topics: []                  # Topics to subscribe
  qos: 0                      # Quality of Service
  relay_to_agent: false       # Forward to agent

# MeshCentral Integration
# -----------------------
meshcentral:
  enabled: false
  url: ""
  username: ""
  readonly: false
  blocked_operations: []      # Operations to block

# Maintenance (Scheduled Tasks)
# -----------------------------
maintenance:
  enabled: true
  time: "04:00"               # Run time (24h format)
  lifeboat_enabled: true      # Allow self-modification
  lifeboat_port: 8091         # Lifeboat communication port

# Logging
# -------
logging:
  enable_file_log: true
  log_dir: "./log"

# Web UI Configuration Editor
# ---------------------------
web_config:
  enabled: true               # Enable config editor in UI

# Invasion Control (Remote Deployment)
# ------------------------------------
invasion_control:
  enabled: true

# File Indexing
# -------------
indexing:
  enabled: true
  directories:
    - "./knowledge"          # Directories to index
  extensions:                 # File types to index
    - ".txt"
    - ".md"
    - ".json"
    - ".csv"
    - ".log"
    - ".yaml"
    - ".yml"
  poll_interval_seconds: 60   # Scan frequency

# Directory Paths
# ---------------
directories:
  data_dir: "./data"
  prompts_dir: "./prompts"
  skills_dir: "./agent_workspace/skills"
  tools_dir: "./agent_workspace/tools"
  vectordb_dir: "./data/vectordb"
  workspace_dir: "./agent_workspace/workdir"

# SQLite Database Paths
# ---------------------
sqlite:
  short_term_path: "./data/short_term.db"
  long_term_path: "./data/long_term.db"
  inventory_path: "./data/inventory.db"
  invasion_path: "./data/invasion.db"
```

---

## API Endpoints

### REST API

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/api/health` | Health check | No |
| GET | `/api/config` | Get configuration (masked) | Yes* |
| POST | `/api/config` | Update configuration | Yes* |
| GET | `/api/dashboard` | System metrics | Yes* |
| GET | `/api/memory/short-term` | Short-term memory | Yes* |
| GET | `/api/memory/long-term` | Long-term memory | Yes* |
| POST | `/api/memory/query` | Query memories | Yes* |
| GET | `/api/tools` | List available tools | Yes* |
| POST | `/api/tools/execute` | Execute tool | Yes* |
| GET | `/api/vault/status` | Vault status | Yes* |
| POST | `/api/vault/rotate` | Rotate master key | Yes* |
| GET | `/api/co-agents` | List co-agents | Yes* |
| GET | `/api/co-agents/:id/result` | Co-agent result | Yes* |
| POST | `/api/webhook/:token` | Incoming webhook | Token |

*If auth is enabled

### WebSocket/SSE Endpoints

| Endpoint | Description |
|----------|-------------|
| `/api/events` | Server-Sent Events stream |
| `/ws` | WebSocket for real-time chat |

### Static Files

| Endpoint | Description |
|----------|-------------|
| `/` | Web UI (SPA) |
| `/assets/*` | Static assets |
| `/attachments/*` | User file uploads |

---

## Chat Commands Reference

### Command Summary

| Command | Arguments | Description |
|---------|-----------|-------------|
| `/help` | - | List all commands |
| `/reset` | - | Clear chat history |
| `/stop` | - | Interrupt current action |
| `/restart` | - | Restart AuraGo server |
| `/debug` | `on`/`off` | Toggle debug mode |
| `/personality` | `[name]` | List or switch personality |
| `/budget` | - | Show API usage and costs |

### Detailed Command Reference

#### `/help`
```
Usage: /help

Lists all available slash commands with brief descriptions.

Example:
You: /help
Agent: 📜 Available Commands:
  • /help: Shows this help
  • /reset: Clears chat history
  ...
```

#### `/reset`
```
Usage: /reset

Clears the current session's conversation history from short-term 
memory. Does NOT affect:
- Long-term memories
- Knowledge graph
- Notes and todos
- Core memory

Use this when:
- Context becomes too long
- Agent seems confused
- Starting a new topic
```

#### `/stop`
```
Usage: /stop

Immediately interrupts the current agent action. Useful when:
- A tool call is taking too long
- The agent is in an infinite loop
- You want to cancel a running task

Note: Does not undo already completed actions.
```

#### `/restart`
```
Usage: /restart

Gracefully restarts the AuraGo server. All connections will be 
disconnected. The agent will:
1. Complete current operation
2. Save state
3. Exit with code 42
4. Auto-restart (if using systemd/service)
```

#### `/debug`
```
Usage: /debug [on|off]

Controls debug output verbosity.

Arguments:
  on   - Enable detailed error reporting
  off  - Disable detailed output
  (no arg) - Toggle current state

When enabled:
- Full tool output shown
- Stack traces in errors
- Verbose logging
```

#### `/personality`
```
Usage: /personality [name]

Without arguments: Lists available personalities
With argument: Switches to specified personality

Available personalities:
  - friend      (casual, warm)
  - professional (formal, efficient)
  - neutral     (balanced)
  - punk        (rebellious)
  - terminator  (direct, minimal)
  - mcp         (protocol-focused)

Example:
You: /personality professional
Agent: 🎭 Personality switched to professional
```

#### `/budget`
```
Usage: /budget

Displays today's API usage statistics:
- Input tokens used
- Output tokens used
- Estimated cost in USD
- Daily limit status
- Reset time

Only available when budget.enabled is true.
```

---

## Tool Reference

### Tool Categories

| Category | Tools | Description |
|----------|-------|-------------|
| System | execute_shell, execute_python, get_system_metrics | Core system operations |
| Filesystem | read_file, write_file, list_directory | File management |
| Memory | manage_memory, query_memory | Memory operations |
| Knowledge | knowledge_graph_query | Knowledge graph |
| Web | web_search, web_scraper, fetch_url | Internet access |
| Communication | send_email, telegram_send | Messaging |
| Smart Home | home_assistant_* | Home Assistant |
| Infrastructure | docker_*, proxmox_*, ollama_* | Infrastructure mgmt |
| Storage | webdav_*, koofr_* | Cloud storage |
| Scheduling | cron_scheduler, follow_up | Task scheduling |
| Development | git_*, github_* | Development tools |
| Utility | analyze_image, transcribe_audio | Utilities |

### Common Tool Parameters

#### File Operations
```yaml
read_file:
  path: string        # File path (required)
  offset: int         # Start reading at line (optional)
  limit: int          # Max lines to read (optional)

write_file:
  path: string        # File path (required)
  content: string     # Content to write (required)
  append: boolean     # Append instead of overwrite (optional)
```

#### Shell Execution
```yaml
execute_shell:
  command: string     # Shell command (required)
  timeout: int        # Timeout in seconds (optional, default: 60)
  background: boolean # Run in background (optional)
  workdir: string     # Working directory (optional)
```

#### Python Execution
```yaml
execute_python:
  code: string        # Python code (required)
  timeout: int        # Timeout in seconds (optional, default: 120)
  packages: []string  # pip packages to install (optional)
```

#### Web Search
```yaml
web_search:
  query: string       # Search query (required)
  num_results: int    # Number of results (optional, default: 5)
  source: string      # Search engine (optional: duckduckgo, google)

web_scraper:
  url: string         # URL to scrape (required)
  selector: string    # CSS selector (optional)
  extract_text: boolean  # Extract text only (optional)
```

#### Memory Operations
```yaml
manage_memory:
  operation: string   # add, update, delete, read (required)
  key: string         # Memory key (required for update/delete)
  content: string     # Memory content (required for add/update)
  category: string    # Memory category (optional)
  priority: int       # Priority 1-10 (optional)

query_memory:
  query: string       # Search query (required)
  limit: int          # Max results (optional, default: 5)
  threshold: float    # Similarity threshold (optional)
```

#### Email
```yaml
send_email:
  to: string          # Recipient (required)
  subject: string     # Subject (required)
  body: string        # Body text (required)
  attachments: []string  # File paths (optional)
```

#### Docker
```yaml
docker_list:
  type: string        # containers, images, volumes, networks
  all: boolean        # Include stopped containers

docker_start:
  name: string        # Container name/ID (required)

docker_stop:
  name: string        # Container name/ID (required)
  timeout: int        # Stop timeout (optional)

docker_logs:
  name: string        # Container name/ID (required)
  lines: int          # Number of lines (optional)
  follow: boolean     # Stream logs (optional)
```

#### Home Assistant
```yaml
home_assistant_get_state:
  entity_id: string   # Entity ID (required)

home_assistant_call_service:
  domain: string      # Service domain (required)
  service: string     # Service name (required)
  entity_id: string   # Target entity (optional)
  data: object        # Service data (optional)

home_assistant_toggle:
  entity_id: string   # Entity to toggle (required)
```

#### Scheduling
```yaml
cron_scheduler:
  operation: string   # add, list, remove (required)
  name: string        # Job name (required for add/remove)
  schedule: string    # Cron expression (required for add)
  command: string     # Command to run (required for add)
  enabled: boolean    # Enable/disable (optional)

follow_up:
  task: string        # Task description (required)
  delay: string       # Delay (e.g., "1h", "30m") (required)
```

---

## Example Configurations

### Minimal Configuration

```yaml
# Absolute minimum to run AuraGo
server:
  host: "127.0.0.1"
  port: 8088

llm:
  api_key: "sk-or-v1-YOUR-KEY-HERE"
  model: "arcee-ai/trinity-large-preview:free"
```

### Development Setup

```yaml
# For development and testing
server:
  host: "127.0.0.1"
  port: 8088

llm:
  provider: "openrouter"
  api_key: "sk-or-v1-DEV-KEY"
  model: "meta-llama/llama-3.1-8b-instruct:free"
  temperature: 0.8

agent:
  debug_mode: true
  show_tool_results: true
  max_tool_calls: 20

logging:
  enable_file_log: true
  log_dir: "./log"
```

### Production with Full Features

```yaml
# Production-ready configuration
server:
  host: "0.0.0.0"
  port: 8088

auth:
  enabled: true
  password_hash: "$2a$10$..."  # bcrypt hash
  session_secret: "your-session-secret"
  totp_enabled: true
  totp_secret: "BASE32SECRET"

llm:
  provider: "openrouter"
  api_key: "sk-or-v1-PROD-KEY"
  model: "anthropic/claude-3.5-sonnet"
  temperature: 0.7

embeddings:
  provider: "external"
  external_url: "http://localhost:11434/v1"
  external_model: "nomic-embed-text"

agent:
  system_language: "English"
  max_tool_calls: 15
  personality_engine: true
  core_personality: "professional"

budget:
  enabled: true
  daily_limit_usd: 10
  enforcement: "warn"
  warning_threshold: 0.8

circuit_breaker:
  max_tool_calls: 25
  llm_timeout_seconds: 300

fallback_llm:
  enabled: true
  api_key: "sk-or-v1-BACKUP-KEY"
  model: "google/gemini-2.5-flash-lite-preview-09-2025"

telegram:
  bot_token: "123456:ABC..."
  telegram_user_id: 123456789

discord:
  enabled: true
  bot_token: "..."
  allowed_user_id: "..."

home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  access_token: "..."

docker:
  enabled: true

maintenance:
  enabled: true
  time: "04:00"
  lifeboat_enabled: false  # Disable in production
```

### Docker Configuration

```yaml
# For Docker deployment
server:
  host: "0.0.0.0"  # Required for Docker!
  port: 8088

llm:
  api_key: "${LLM_API_KEY}"  # From environment
  model: "arcee-ai/trinity-large-preview:free"

# Access host services
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"

home_assistant:
  enabled: true
  url: "http://host.docker.internal:8123"
  access_token: "..."

embeddings:
  provider: "external"
  external_url: "http://host.docker.internal:11434/v1"
```

### Local LLM with Ollama

```yaml
# Using local models via Ollama
llm:
  provider: "ollama"
  base_url: "http://localhost:11434/v1"
  api_key: "dummy"  # Ollama doesn't require auth
  model: "llama3.1:8b"

embeddings:
  provider: "external"
  external_url: "http://localhost:11434/v1"
  external_model: "nomic-embed-text"

agent:
  context_window: 8192  # Adjust for local model
  max_tool_calls: 10
```

### Smart Home Focus

```yaml
# Optimized for home automation
server:
  host: "0.0.0.0"
  port: 8088

llm:
  api_key: "..."
  model: "google/gemini-2.5-flash-lite-preview-09-2025"
  temperature: 0.3  # More deterministic

agent:
  system_language: "English"
  core_personality: "neutral"
  show_tool_results: true

home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  access_token: "..."

chromecast:
  enabled: true
  tts_port: 8090

tts:
  provider: "google"
  language: "en"

telegram:
  bot_token: "..."
  telegram_user_id: 123456789
```

---

## Environment Variables

### Required Variables

| Variable | Description | Format |
|----------|-------------|--------|
| `AURAGO_MASTER_KEY` | Vault encryption key | 64-char hex string |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AURAGO_CONFIG_PATH` | Custom config file path | `./config.yaml` |
| `AURAGO_LOG_LEVEL` | Logging level | `info` |
| `AURAGO_DATA_DIR` | Data directory | `./data` |
| `LLM_API_KEY` | LLM API key | From config |
| `EMBEDDINGS_API_KEY` | Embeddings API key | From config |

### Master Key Generation

```bash
# Linux/macOS
export AURAGO_MASTER_KEY=$(openssl rand -hex 32)

# Windows PowerShell
$env:AURAGO_MASTER_KEY = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })

# Verify length (should be 64 characters)
echo ${#AURAGO_MASTER_KEY}  # Linux/macOS
$env:AURAGO_MASTER_KEY.Length  # PowerShell
```

### Docker Environment

```bash
# Create .env file
cat > .env << EOF
AURAGO_MASTER_KEY=$(openssl rand -hex 32)
EOF

# Or export directly
export AURAGO_MASTER_KEY="your-64-character-hex-key"
docker compose up -d
```

---

## File Paths and Locations

### Default Directory Structure

```
AuraGo/
├── aurago                    # Main executable
├── lifeboat                  # Update companion (optional)
├── config.yaml               # Configuration
├── .env                      # Environment variables
├── agent_workspace/
│   ├── prompts/              # System prompts
│   │   ├── personalities/    # Personality templates
│   │   └── tools_manuals/    # Tool documentation
│   ├── skills/               # Python skills
│   ├── tools/                # Agent-created tools
│   │   └── manifest.json     # Tool registry
│   └── workdir/              # Working directory
│       ├── attachments/      # Uploaded files
│       └── venv/             # Python virtual env
├── data/
│   ├── short_term.db         # Short-term memory (SQLite)
│   ├── long_term.db          # Long-term metadata (SQLite)
│   ├── inventory.db          # Device inventory (SQLite)
│   ├── invasion.db           # Remote deployment (SQLite)
│   ├── vectordb/             # Vector database
│   │   └── ...
│   ├── secrets.vault         # Encrypted secrets
│   ├── core_memory.md        # Permanent memory
│   └── chat_history.json     # Chat UI state
└── log/
    └── supervisor.log        # Application logs
```

### Platform-Specific Paths

| Platform | Default Install Path |
|----------|---------------------|
| Linux | `~/aurago/` |
| macOS | `~/aurago/` |
| Windows | `%USERPROFILE%\aurago\` |
| Docker | `/app/` (container) |

### Configuration File Locations

| File | Purpose | Search Order |
|------|---------|--------------|
| `config.yaml` | Main configuration | 1. Working directory 2. `~/.aurago/` 3. `/etc/aurago/` |
| `.env` | Environment variables | 1. Working directory 2. `~/.aurago/` |
| `prompts/` | System prompts | Config: `directories.prompts_dir` |

### Log Rotation

Logs are automatically rotated when they reach 100MB:
- `supervisor.log` - Current log
- `supervisor.log.1` - Previous log
- `supervisor.log.2.gz` - Compressed older logs

---

## Update History / Changelog

### Version History

#### v1.0.0 (Initial Release)
- Core agent loop with tool dispatch
- SQLite-based short-term memory
- Vector database for long-term memory
- Knowledge graph for structured facts
- Web UI with real-time chat
- Telegram and Discord integrations
- 90+ built-in tools
- AES-256-GCM encrypted vault
- Personality engine V1
- Co-agent system
- Budget tracking
- Circuit breaker protection

### Update Checklist

When updating AuraGo:

```
□ Read release notes for breaking changes
□ Backup data/ directory
□ Export current config: cp config.yaml config.yaml.backup
□ Download new binary
□ Compare config.yaml with new defaults
□ Restart service
□ Verify functionality with /help command
□ Check logs for errors
```

### Migration Notes

#### v0.x to v1.0
- Config format changed - review and update
- Database schemas auto-migrate on first start
- Vault format unchanged - master key still valid

---

## Useful Resources and Links

### Official Resources

| Resource | URL | Description |
|----------|-----|-------------|
| GitHub Repository | github.com/antibyte/AuraGo | Source code, issues |
| Releases | github.com/antibyte/AuraGo/releases | Pre-built binaries |
| Documentation | /documentation folder | Detailed guides |

### LLM Providers

| Provider | URL | Free Tier |
|----------|-----|-----------|
| OpenRouter | openrouter.ai | Yes |
| OpenAI | openai.com | No |
| Anthropic | anthropic.com | No |
| Ollama | ollama.com | Yes (local) |

### Related Projects

| Project | URL | Purpose |
|---------|-----|---------|
| chromem-go | github.com/philippgille/chromem-go | Vector database |
| go-openai | github.com/sashabaranov/go-openai | OpenAI client |
| telegram-bot-api | github.com/go-telegram-bot-api | Telegram integration |
| discordgo | github.com/bwmarrin/discordgo | Discord integration |

### Learning Resources

| Topic | Resource |
|-------|----------|
| YAML | yaml.org/spec |
| Markdown | markdownguide.org |
| Go | go.dev/doc |
| SQLite | sqlite.org/docs |
| Docker | docs.docker.com |

### Community

| Platform | Link | Purpose |
|----------|------|---------|
| GitHub Issues | github.com/antibyte/AuraGo/issues | Bug reports |
| GitHub Discussions | github.com/antibyte/AuraGo/discussions | Q&A |

---

## License Information

### AuraGo License

This project is provided as-is for personal and educational use.

```
Copyright (c) 2024 AuraGo Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

### Third-Party Licenses

AuraGo uses the following open-source components:

| Library | License | Purpose |
|---------|---------|---------|
| go-openai | MIT | OpenAI API client |
| chromem-go | MPL-2.0 | Vector database |
| modernc.org/sqlite | BSD-3 | SQLite driver |
| telegram-bot-api | MIT | Telegram bot |
| discordgo | BSD-3 | Discord bot |
| gopsutil | BSD-3 | System metrics |
| golang.org/x/crypto | BSD-3 | Cryptography |
| cron/v3 | MIT | Task scheduling |
| vishen/go-chromecast | MIT | Chromecast control |
| yaml.v3 | MIT | YAML parsing |

Full license texts are available in the `LICENSES/` directory or at the respective project repositories.

### Attribution

When using AuraGo in projects or publications, please consider acknowledging:
- The AuraGo project and contributors
- The underlying LLM providers
- Open-source libraries used

---

> 💡 **Tip:** Keep this appendix handy as a quick reference. The configuration reference and command tables are especially useful when setting up new instances or customizing behavior.

---

*End of Appendix - Last updated: 2024*

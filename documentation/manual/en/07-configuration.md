# Chapter 7: Configuration

AuraGo's behavior is controlled through a central configuration file (`config.yaml`) and environment variables. This chapter explains all configuration options and how to customize AuraGo for your needs.

## Table of Contents

1. [Configuration Overview](#configuration-overview)
2. [Server Settings](#server-settings)
3. [LLM Configuration](#llm-configuration)
4. [Embeddings Configuration](#embeddings-configuration)
5. [Agent Behavior Settings](#agent-behavior-settings)
6. [Logging Configuration](#logging-configuration)
7. [Editing Configuration](#editing-configuration)
8. [Configuration Validation](#configuration-validation)
9. [Environment Variables](#environment-variables)
10. [Common Configuration Examples](#common-configuration-examples)

---

## Configuration Overview

AuraGo uses YAML for configuration. The default configuration file is `config.yaml` in the application directory. A sample configuration is provided with the project.

### File Structure

```
config.yaml          # Main configuration file
config_debug.yaml    # Debug/testing configuration (optional)
```

### Configuration Sections

| Section | Purpose |
|---------|---------|
| `server` | Web server host, port, and limits |
| `llm` | Primary LLM provider and model settings |
| `providers` | Reusable provider definitions (OpenAI, OpenRouter, Ollama, etc.) |
| `embeddings` | Vector embedding provider for memory |
| `agent` | Core agent behavior and personality |
| `telegram` | Telegram bot integration |
| `discord` | Discord bot integration |
| `email` | IMAP/SMTP email settings |
| `home_assistant` | Smart home integration |
| `docker` | Docker container management |
| `webhooks` | Incoming webhook configuration |
| `budget` | Cost tracking and limits |
| `logging` | Log file settings |
| `auth` | Web UI authentication |

---

## Server Settings

The `server` section controls the web interface and API endpoints.

```yaml
server:
    host: 0.0.0.0           # Bind address (0.0.0.0 = all interfaces)
    port: 8088              # HTTP port
    bridge_address: "localhost:8089"  # Internal bridge for WebSocket
    max_body_bytes: 10485760          # Max request body size (10MB)
    master_key: ""          # Encryption key for sensitive data
```

### Server Options Explained

| Option | Default | Description |
|--------|---------|-------------|
| `host` | `0.0.0.0` | Network interface to bind to. Use `127.0.0.1` for localhost only |
| `port` | `8088` | HTTP port for the Web UI and API |
| `bridge_address` | `localhost:8089` | Internal bridge address for WebSocket connections |
| `max_body_bytes` | `10485760` (10MB) | Maximum size for uploaded files and request bodies |
| `master_key` | `""` | Master encryption key for sensitive configuration data |

> 💡 **Tip:** For production deployments, set `host` to `127.0.0.1` and use a reverse proxy (nginx, Caddy, Traefik) with HTTPS.

> ⚠️ **Warning:** Never expose AuraGo directly to the internet without authentication. Enable `auth` or use a VPN.

---

## LLM Configuration

The `llm` section defines how AuraGo connects to Large Language Models.

### Provider-Based Configuration (Recommended)

AuraGo supports reusable provider definitions that can be referenced by multiple components:

```yaml
providers:
  - id: openrouter-main
    name: "OpenRouter Main"
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-..."
    model: arcee-ai/trinity-large-preview:free

  - id: ollama-local
    name: "Local Ollama"
    type: ollama
    base_url: http://localhost:11434/v1
    api_key: "dummy_key"
    model: llama3.1

  - id: openai
    name: "OpenAI"
    type: openai
    base_url: https://api.openai.com/v1
    api_key: "sk-..."
    model: gpt-4o

llm:
    provider: openrouter-main    # Reference to provider ID
    use_native_functions: true   # Enable function calling
    temperature: 0.7             # Creativity (0.0-2.0)
    structured_outputs: false    # Enable structured output mode
```

### Supported Provider Types

| Type | Description | Example Base URL |
|------|-------------|------------------|
| `openai` | OpenAI API | `https://api.openai.com/v1` |
| `openrouter` | OpenRouter (unified API) | `https://openrouter.ai/api/v1` |
| `ollama` | Local Ollama instance | `http://localhost:11434/v1` |
| `anthropic` | Anthropic Claude | `https://api.anthropic.com/v1` |
| `google` | Google Gemini | `https://generativelanguage.googleapis.com/v1beta` |
| `custom` | Any OpenAI-compatible API | Variable |

### LLM Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `temperature` | `0.7` | Controls randomness. Lower = more deterministic, higher = more creative |
| `use_native_functions` | `true` | Enable native function calling (tool use) |
| `structured_outputs` | `false` | Force structured JSON outputs (for supported models) |

> 🔍 **Deep Dive: Temperature Settings**
> - `0.0` - Deterministic, same input always produces same output
> - `0.7` - Balanced creativity and consistency (recommended)
> - `1.0+` - More creative, may hallucinate more
> - `2.0` - Maximum randomness, often incoherent

---

## Embeddings Configuration

Embeddings power AuraGo's long-term memory and semantic search.

```yaml
embeddings:
    provider: internal           # Provider ID or "internal"
    internal_model: qwen/qwen3-embedding-8b
    external_url: http://localhost:11434/v1
    external_model: nomic-embed-text
```

### Embedding Providers

| Provider | Description | Best For |
|----------|-------------|----------|
| `internal` | Uses main LLM provider | Simplicity, API key reuse |
| `ollama` | Local embedding models | Privacy, no API costs |
| Dedicated endpoint | Custom embedding service | Performance optimization |

### Recommended Local Embedding Models

| Model | Dimensions | Size | Quality |
|-------|-----------|------|---------|
| `nomic-embed-text` | 768 | Small | Good |
| `mxbai-embed-large` | 1024 | Medium | Excellent |
| `snowflake-arctic-embed` | 768 | Small | Good |

> 💡 **Tip:** For local deployments, use Ollama with `nomic-embed-text` for fast, free embeddings without API costs.

---

## Agent Behavior Settings

The `agent` section controls how AuraGo thinks, remembers, and behaves.

```yaml
agent:
    # Core settings
    system_language: English
    core_personality: friend
    context_window: 131000
    
    # Memory settings
    memory_compression_char_limit: 50000
    core_memory_max_entries: 200
    core_memory_cap_mode: soft
    
    # Personality engine
    personality_engine: true
    personality_engine_v2: true
    personality_v2_model: qwen/qwen-2.5-7b-instruct
    
    # Tool execution
    max_tool_calls: 12
    step_delay_seconds: 0
    show_tool_results: false
    
    # System prompt
    system_prompt_token_budget: 8192
    workflow_feedback: true
    
    # Debugging
    debug_mode: false
    
    # Integrations
    enable_google_workspace: true
```

### Personality Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `core_personality` | `friend` | Base personality: `friend`, `professional`, `punk`, `neutral`, `terminator` |
| `personality_engine` | `true` | Enable V1 heuristic personality adaptation |
| `personality_engine_v2` | `true` | Enable V2 LLM-based personality analysis |

### Memory Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `context_window` | `131000` | Maximum tokens in conversation context |
| `memory_compression_char_limit` | `50000` | Compress memory entries larger than this |
| `core_memory_max_entries` | `200` | Maximum permanent memory entries |
| `core_memory_cap_mode` | `soft` | `soft` = compress old entries, `hard` = delete old entries |

### Tool Execution

| Setting | Default | Description |
|---------|---------|-------------|
| `max_tool_calls` | `12` | Maximum tools per agent response |
| `step_delay_seconds` | `0` | Delay between tool calls (for rate limiting) |
| `show_tool_results` | `false` | Include tool results in user-visible output |

> 🔍 **Deep Dive: Context Window**
> The `context_window` should match your LLM model's actual limit. Common values:
> - GPT-3.5: 16384
> - GPT-4: 8192 or 32768
> - Claude 3: 200000
> - Llama 3.1: 128000
> - Gemini: 1000000+

### Danger Zone: Capability Controls

```yaml
agent:
    # Tool capability gates (all default to true)
    allow_shell: true            # execute_shell tool
    allow_python: true           # execute_python, save_tool, execute_skill
    allow_filesystem_write: true # filesystem write operations
    allow_network_requests: true # api_request tool
    allow_remote_shell: true     # execute_remote_shell tool
    allow_self_update: true      # manage_updates tool
    allow_mcp: true              # MCP server connections
    sudo_enabled: false          # execute_sudo tool (requires vault entry)
```

> ⚠️ **Warning:** Disabling `allow_filesystem_write` prevents the agent from modifying files, which limits functionality but increases safety.

---

## Logging Configuration

```yaml
logging:
    enable_file_log: true
    log_dir: ./log
```

| Setting | Default | Description |
|---------|---------|-------------|
| `enable_file_log` | `true` | Write logs to files |
| `log_dir` | `./log` | Directory for log files |

Log files are rotated automatically and named by date:
- `log/aurago_2026-03-08.log`
- `log/aurago_2026-03-07.log`

---

## Editing Configuration

AuraGo provides two ways to edit configuration:

### Method 1: Web UI (Recommended)

1. Open the Web UI (`http://localhost:8088`)
2. Navigate to **Settings** → **Configuration**
3. Edit values in the form
4. Click **Save Changes**
5. Restart AuraGo for some changes to take effect

> 💡 **Tip:** The Web UI validates configuration before saving, preventing syntax errors.

### Method 2: Direct YAML Editing

1. Open `config.yaml` in a text editor
2. Make changes following YAML syntax
3. Save the file
4. Restart AuraGo

```yaml
# Example: Adding a new provider
providers:
  - id: my-custom-llm
    name: "My Custom LLM"
    type: custom
    base_url: https://api.example.com/v1
    api_key: "my-api-key"
    model: my-model-v1
```

### What Requires a Restart?

| Change | Restart Required? |
|--------|-------------------|
| Server port/host | Yes |
| LLM provider | No (runtime switchable) |
| Telegram bot token | Yes |
| Discord bot token | Yes |
| Personality settings | No |
| Memory settings | No |
| Tool permissions | No |

---

## Configuration Validation

AuraGo validates configuration on startup and will warn or fail on errors.

### Validation Levels

| Level | Behavior |
|-------|----------|
| **Warning** | Log warning but continue startup |
| **Error** | Log error, attempt fallback, or exit |
| **Fatal** | Exit immediately with error message |

### Common Validation Errors

```
Error: invalid provider reference "my-llm" in llm.provider
→ The provider ID doesn't exist in providers list

Error: telegram.bot_token is set but telegram.enabled is false
→ Enable telegram or remove the token

Error: cannot parse retry_intervals: invalid duration "invalid"
→ Use valid Go durations: "10s", "2m", "1h"
```

### Testing Configuration

```bash
# Start AuraGo and check for validation errors
./aurago

# Use debug configuration
./aurago -config config_debug.yaml
```

---

## Environment Variables

AuraGo supports environment variables for configuration overrides. This is useful for Docker deployments and secrets management.

### Variable Naming Convention

Environment variables use uppercase with underscores, prefixed with `AURAGO_`:

```bash
# Server settings
AURAGO_SERVER_HOST=0.0.0.0
AURAGO_SERVER_PORT=8088

# LLM settings
AURAGO_LLM_API_KEY=sk-or-v1-...
AURAGO_LLM_PROVIDER=openrouter
AURAGO_LLM_MODEL=arcee-ai/trinity-large-preview:free

# Telegram
AURAGO_TELEGRAM_BOT_TOKEN=123456:ABC-DEF...
AURAGO_TELEGRAM_USER_ID=123456789

# Database paths
AURAGO_SQLITE_SHORT_TERM_PATH=./data/short_term.db
```

### Priority Order

Configuration is loaded in this priority (highest first):

1. Environment variables
2. `config.yaml` values
3. Default values

### Docker Environment Example

```yaml
# docker-compose.yml
services:
  aurago:
    image: aurago:latest
    environment:
      - AURAGO_SERVER_HOST=0.0.0.0
      - AURAGO_LLM_API_KEY=${LLM_API_KEY}
      - AURAGO_TELEGRAM_BOT_TOKEN=${TELEGRAM_TOKEN}
      - AURAGO_TELEGRAM_USER_ID=${TELEGRAM_USER_ID}
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/config.yaml
```

> 💡 **Tip:** Use a `.env` file for local development to keep secrets out of version control.

---

## Common Configuration Examples

### Example 1: Local Development with Ollama

```yaml
providers:
  - id: ollama-local
    name: "Local Ollama"
    type: ollama
    base_url: http://localhost:11434/v1
    api_key: dummy_key
    model: llama3.1

llm:
    provider: ollama-local
    use_native_functions: true
    temperature: 0.7

embeddings:
    provider: ollama-local
    external_url: http://localhost:11434/v1
    external_model: nomic-embed-text

server:
    host: 127.0.0.1
    port: 8088

agent:
    system_language: English
    core_personality: friend
```

### Example 2: Production with OpenRouter

```yaml
providers:
  - id: openrouter-main
    name: "OpenRouter Main"
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}  # From environment
    model: anthropic/claude-3.5-sonnet

  - id: openrouter-vision
    name: "OpenRouter Vision"
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}
    model: google/gemini-2.5-flash-lite-preview-09-2025

llm:
    provider: openrouter-main
    use_native_functions: true
    temperature: 0.5

vision:
    provider: openrouter-vision

embeddings:
    provider: internal
    internal_model: qwen/qwen3-embedding-8b

server:
    host: 127.0.0.1
    port: 8088

auth:
    enabled: true
    password_hash: "$2a$10$..."  # bcrypt hash
    totp_enabled: true
    totp_secret: "BASE32SECRET..."

budget:
    enabled: true
    daily_limit_usd: 5.0
    enforcement: warn
    warning_threshold: 0.8
```

### Example 3: Multi-User with Discord and Telegram

```yaml
# Multiple interfaces enabled
telegram:
    bot_token: ${TELEGRAM_BOT_TOKEN}
    telegram_user_id: ${TELEGRAM_USER_ID}
    max_concurrent_workers: 5

discord:
    enabled: true
    bot_token: ${DISCORD_BOT_TOKEN}
    guild_id: "123456789"
    allowed_user_id: "987654321"
    default_channel_id: "123456789"

email:
    enabled: true
    imap_host: imap.gmail.com
    imap_port: 993
    smtp_host: smtp.gmail.com
    smtp_port: 587
    username: ${EMAIL_USER}
    password: ${EMAIL_PASS}
    from_address: ${EMAIL_USER}
    watch_enabled: true
    watch_interval_seconds: 120

home_assistant:
    enabled: true
    url: http://homeassistant.local:8123
    access_token: ${HA_TOKEN}
```

### Example 4: Minimal Headless Server

```yaml
# No Web UI, only Telegram
server:
    host: 127.0.0.1
    port: 8088

providers:
  - id: openrouter
    name: OpenRouter
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: ${API_KEY}
    model: meta-llama/llama-3.1-8b-instruct:free

llm:
    provider: openrouter

telegram:
    bot_token: ${TELEGRAM_TOKEN}
    telegram_user_id: ${USER_ID}

# Disable unnecessary features
web_config:
    enabled: false

chromecast:
    enabled: false

auth:
    enabled: false
```

### Example 5: High-Security Configuration

```yaml
# Restrict dangerous operations
agent:
    allow_shell: false
    allow_python: false
    allow_filesystem_write: false
    allow_network_requests: false
    allow_self_update: false
    sudo_enabled: false

# Read-only integrations
docker:
    enabled: true
    host: unix:///var/run/docker.sock
    readonly: true

home_assistant:
    enabled: true
    url: http://localhost:8123
    readonly: true

webdav:
    enabled: true
    url: https://cloud.example.com/webdav
    readonly: true

# Strong authentication
auth:
    enabled: true
    password_hash: "$2a$10$..."
    totp_enabled: true
    max_login_attempts: 3
    lockout_minutes: 60
```

---

## Next Steps

Now that you understand AuraGo configuration:

1. **[Integrations](08-integrations.md)** – Configure Telegram, Discord, Email, and more
2. **[Security](09-security.md)** – Harden your AuraGo installation
3. **[Troubleshooting](10-troubleshooting.md)** – Fix common configuration issues

> 💡 **Pro Tip:** Keep a backup of your working `config.yaml` before making major changes. Configuration errors can prevent AuraGo from starting.

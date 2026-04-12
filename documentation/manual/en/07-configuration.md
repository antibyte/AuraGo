# Chapter 7: Configuration

AuraGo's behavior is controlled through a central configuration file (`config.yaml`) and environment variables. This chapter explains all configuration options and how to customize AuraGo for your needs.

## Table of Contents

1. [Configuration Overview](#configuration-overview)
2. [Editing Configuration](#editing-configuration)
3. [Provider System](#provider-system)
4. [Server Settings](#server-settings)
5. [Agent Behavior Settings](#agent-behavior-settings)
6. [Tool Configuration, Skill Manager & Media Registry](#tool-configuration-skill-manager--media-registry)
7. [Embeddings Configuration](#embeddings-configuration)
8. [Personality Settings](#personality-settings)
9. [Logging Configuration](#logging-configuration)
10. [Environment Variables](#environment-variables)
11. [Common Configuration Examples](#common-configuration-examples)
12. [Compact YAML Reference](#compact-yaml-reference)

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
| `agent` | Core agent behavior and limits |
| `personality` | Personality engine and user profiling |
| `tools` | Built-in tool permissions and timeouts |
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

## Editing Configuration

### Web UI (Recommended)

The easiest way to configure AuraGo is through the built-in Web UI:

1. **Open the Web UI** at `http://localhost:8088` (or the host/port you configured)
2. **Click the radial menu** (≡) in the top-left corner
3. **Select "Config"**
4. **Navigate categories on the left sidebar**: Providers, Agent, Integrations, Tools, Server, etc.
5. **Toggle switches** and **fill in fields** as needed
6. **Click "Save"** at the bottom of the page

> 💡 **Tip:** The Web UI validates configuration before saving, preventing syntax errors.

#### Which Changes Require a Restart?

| Change | Restart Required? |
|--------|-------------------|
| Server port/host | Yes |
| LLM provider | No (runtime switchable) |
| Telegram bot token | Yes |
| Discord bot token | Yes |
| Personality settings | No |
| Memory settings | No |
| Tool permissions | No |

### Direct YAML Editing (Fallback)

For advanced use cases or headless setups, you can edit `config.yaml` directly:

1. Open `config.yaml` in a text editor
2. Make changes following YAML syntax
3. Save the file
4. Restart AuraGo

---

## Provider System

The `providers` section defines reusable LLM connections. Multiple components can reference the same provider by its `id`.

> 🖥️ **Web UI:** Go to **Config → Providers** to add, edit, or delete providers. Use the **Test** button to verify connectivity before saving.

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
| `helper_enabled` | `false` | Enable dedicated helper LLM for internal analysis/background features |
| `helper_provider` | `""` | Provider ID for helper LLM |
| `helper_model` | `""` | Model override for helper LLM (smaller/cheaper recommended) |

> 🔍 **Deep Dive: Temperature Settings**
> - `0.0` - Deterministic, same input always produces same output
> - `0.7` - Balanced creativity and consistency (recommended)
> - `1.0+` - More creative, may hallucinate more
> - `2.0` - Maximum randomness, often incoherent

---

## Server Settings

The `server` section controls the web interface and API endpoints.

> 🖥️ **Web UI:** Go to **Config → Server** to set the host, port, HTTPS mode, and upload limits.

```yaml
server:
    host: 127.0.0.1         # Bind address (127.0.0.1 = localhost only)
    port: 8088              # HTTP port
    bridge_address: ""      # Internal bridge for WebSocket
    max_body_bytes: 10485760          # Max request body size (10MB)
    ui_language: "en"       # Default Web UI language
    oauth_redirect_base_url: ""       # Base URL for OAuth callbacks (e.g. http://localhost:8088)
    https:
        enabled: false
        cert_mode: auto                     # "auto" (Let's Encrypt), "custom" (uploaded cert), "selfsigned" (auto-generated)
        domain: ""
        email: ""
        cert_file: ""                       # custom mode: path to PEM certificate file
        key_file: ""                        # custom mode: path to PEM private key file
        https_port: 443
        http_port: 80
        behind_proxy: false
```

### Server Options Explained

| Option | Default | Description |
|--------|---------|-------------|
| `host` | `127.0.0.1` | Network interface to bind to. Use `0.0.0.0` for all interfaces (Docker) |
| `port` | `8088` | HTTP port for the Web UI and API |
| `bridge_address` | `""` | Internal bridge address for WebSocket connections |
| `max_body_bytes` | `10485760` (10MB) | Maximum size for uploaded files and request bodies |
| `https.enabled` | `false` | Enable HTTPS/Let's Encrypt |
| `https.cert_mode` | `auto` | Certificate mode: `auto`, `custom`, or `selfsigned` |
| `https.domain` | `""` | Domain for Let's Encrypt |
| `https.email` | `""` | Contact email for Let's Encrypt |
| `https.cert_file` | `""` | Path to PEM certificate (custom mode) |
| `https.key_file` | `""` | Path to PEM private key (custom mode) |
| `https.https_port` | `443` | HTTPS port |
| `https.http_port` | `80` | HTTP port for ACME challenge |
| `https.behind_proxy` | `false` | AuraGo is behind a reverse proxy |

> 💡 **Tip:** For production deployments, set `host` to `127.0.0.1` and use a reverse proxy (nginx, Caddy, Traefik) with HTTPS.

> ⚠️ **Warning:** Never expose AuraGo directly to the internet without authentication. Enable `auth` or use a VPN.

---

## Agent Behavior Settings

The `agent` section controls how AuraGo thinks, remembers, and behaves.

> 🖥️ **Web UI:** Go to **Config → Agent** to adjust memory limits, tool execution, and danger-zone toggles.

```yaml
agent:
    # Core settings
    system_language: English
    context_window: 0           # 0 = auto-detect from provider API at startup

    # Memory settings
    memory_compression_char_limit: 60000
    core_memory_max_entries: 200
    core_memory_cap_mode: soft

    # Tool execution
    max_tool_calls: 15
    step_delay_seconds: 0
    show_tool_results: false
    tool_output_limit: 50000    # max characters of a single tool result fed into context (0 = unlimited)

    # System prompt
    system_prompt_token_budget: 12288
    adaptive_system_prompt_token_budget: true
    workflow_feedback: true
    max_tool_guides: 5          # max tool guide documents injected into prompt

    # Debugging
    debug_mode: false

    # Adaptive tool filtering (reduces token usage for tool schemas)
    adaptive_tools:
        enabled: false
        max_tools: 60
        decay_half_life_days: 7
        always_include: [filesystem, shell, manage_memory, query_memory, execute_python, docker, api_request]

    # Recovery settings
    recovery:
        max_provider_422_recoveries: 3      # automatic retries after provider-side 422 validation errors
        min_messages_for_empty_retry: 5     # retry empty LLM responses only when enough conversation context exists
        duplicate_consecutive_hits: 2       # circuit breaker after repeated identical tool calls in a row
        duplicate_frequency_hits: 3         # circuit breaker after repeated identical tool calls overall
        identical_tool_error_hits: 3        # stop retrying when the exact same tool error repeats

    # Background tasks
    background_tasks:
        enabled: true                       # persistent background task queue for follow_up, cron, and wait events
        follow_up_delay_seconds: 2          # small delay so the current response can finish first
        http_timeout_seconds: 120           # loopback timeout for background prompt execution
        max_retries: 2                      # retry failed background prompt executions
        retry_delay_seconds: 60             # retry delay for failed background prompt executions
        wait_poll_interval_seconds: 5       # poll interval for wait_for_event tasks
        wait_default_timeout_secs: 600      # default timeout for wait_for_event tasks
```

### Memory Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `context_window` | `0` | Maximum tokens in conversation context (0 = auto-detect) |
| `memory_compression_char_limit` | `60000` | Compress memory entries larger than this |
| `core_memory_max_entries` | `200` | Maximum permanent memory entries |
| `core_memory_cap_mode` | `soft` | `soft` = compress old entries, `hard` = delete old entries |

### Tool Execution

| Setting | Default | Description |
|---------|---------|-------------|
| `max_tool_calls` | `15` | Maximum tools per agent response |
| `step_delay_seconds` | `0` | Delay between tool calls (for rate limiting) |
| `show_tool_results` | `false` | Include tool results in user-visible output |
| `tool_output_limit` | `50000` | Max characters of a single tool result fed into context (0 = unlimited) |

### System Prompt

| Setting | Default | Description |
|---------|---------|-------------|
| `system_prompt_token_budget` | `12288` | Maximum tokens reserved for the system prompt |
| `adaptive_system_prompt_token_budget` | `true` | Automatically adjust system prompt token budget |
| `max_tool_guides` | `5` | Max tool guide documents injected into prompt |
| `workflow_feedback` | `true` | Provide workflow feedback in responses |

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
    # Danger Zone — disabled by default for fresh installs
    allow_shell: false           # execute_shell tool
    allow_python: false          # execute_python, save_tool, execute_skill
    allow_filesystem_write: false # filesystem write operations
    allow_network_requests: false # api_request tool
    allow_remote_shell: false    # execute_remote_shell tool
    allow_self_update: false     # manage_updates tool
    allow_mcp: false             # MCP server connections
    allow_web_scraper: false     # web scraper tool
    sudo_enabled: false          # execute_sudo tool (requires vault entry)
```

> ⚠️ **Warning:** Disabling `allow_filesystem_write` prevents the agent from modifying files, which limits functionality but increases safety.

---

## Tool Configuration, Skill Manager & Media Registry

Built-in tools, skill uploads, and media tracking can all be configured under `Config → Tools` in the Web UI.

```yaml
tools:
    memory:
        enabled: true
        readonly: false
    knowledge_graph:
        enabled: true
        readonly: false
    secrets_vault:
        enabled: true
        readonly: false
    scheduler:
        enabled: true
        readonly: false
    notes:
        enabled: true
        readonly: false
    missions:
        enabled: true
        readonly: false
    stop_process:
        enabled: true
    inventory:
        enabled: true
    memory_maintenance:
        enabled: true
    wol:
        enabled: true
    journal:
        enabled: true
        readonly: false
    daemon_skills:
        enabled: true
        max_concurrent_daemons: 3
        global_rate_limit_secs: 60
        max_wakeups_per_hour: 10
        max_budget_per_hour_usd: 1.0
    web_capture:
        enabled: true
    network_ping:
        enabled: true
    network_scan:
        enabled: true
    form_automation:
        enabled: false
    upnp_scan:
        enabled: true
    contacts:
        enabled: true
    python_secret_injection:
        enabled: false
    python_timeout_seconds: 30
    skill_timeout_seconds: 120
    background_timeout_seconds: 3600
    web_scraper:
        enabled: true
        summary_mode: false
    wikipedia:
        summary_mode: false
    ddg_search:
        summary_mode: false
    pdf_extractor:
        enabled: true
        summary_mode: false
    document_creator:
        enabled: false
        backend: maroto
        output_dir: data/documents
        gotenberg:
            url: "http://gotenberg:3000"
            timeout: 120
    skill_manager:
        enabled: true
        allow_uploads: true
        readonly: false
        require_scan: true
        max_upload_size_mb: 1
        auto_enable_clean: false
        scan_with_guardian: false

media_registry:
    enabled: true
```

### Key Options

| Setting | Default | Description |
|---------|---------|-------------|
| `python_timeout_seconds` | `30` | Foreground Python/shell execution timeout |
| `skill_timeout_seconds` | `120` | Skill execution timeout |
| `background_timeout_seconds` | `3600` | Background execution timeout before forced termination |
| `skill_manager.allow_uploads` | `true` | Allow users to upload new Python skills |
| `skill_manager.require_scan` | `true` | Require security scan before enabling new skills |
| `skill_manager.max_upload_size_mb` | `1` | Maximum upload file size in MB |
| `media_registry.enabled` | `true` | Track media files in a registry |

---

## Embeddings Configuration

Embeddings power AuraGo's long-term memory and semantic search.

```yaml
embeddings:
    provider: internal           # Provider ID, "internal", or "disabled"
    internal_model: qwen/qwen3-embedding-8b   # legacy: model when using main LLM provider
    external_url: http://localhost:11434/v1   # legacy: dedicated endpoint URL
    external_model: nomic-embed-text          # legacy: dedicated endpoint model
    multimodal: false
    multimodal_format: auto
    local_ollama:
        enabled: false
        model: nomic-embed-text
        container_port: 11435
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

## Personality Settings

The `personality` section controls AuraGo's personality engine, mood analysis, and user profiling.

```yaml
personality:
    engine: friend                     # active personality profile: friend, professional, punk, neutral, terminator
    engine_v2: true                    # enable advanced V2 engine with async LLM mood analysis
    user_profiling: false              # auto-detect user preferences from conversation
    user_profiling_threshold: 2        # confirmations needed before injecting a trait into prompt
    emotion_synthesizer:
        enabled: false                 # enable emotion synthesis
        min_interval_seconds: 60
        max_history_entries: 100       # max emotion history entries to keep
        trigger_on_mood_change: true
        trigger_always: false
    inner_voice:
        enabled: false                 # subconscious nudge engine
        min_interval_secs: 60
        max_per_session: 20
        decay_turns: 3
        error_streak_min: 2
```

### Personality Options Explained

| Setting | Default | Description |
|---------|---------|-------------|
| `engine` | `friend` | Base personality profile: `friend`, `professional`, `punk`, `neutral`, `terminator` |
| `engine_v2` | `true` | Enable V2 LLM-based personality analysis |
| `user_profiling` | `false` | Auto-detect user preferences from conversation history |
| `user_profiling_threshold` | `2` | Confirmations needed before injecting a detected trait into the prompt |
| `emotion_synthesizer.enabled` | `false` | Enable emotion synthesis for responses |
| `emotion_synthesizer.max_history_entries` | `100` | Maximum emotion history entries to keep |
| `inner_voice.enabled` | `false` | Subconscious nudge engine for subtle behavior hints |

---

## Co-Agents Configuration

The `co_agents` section configures parallel sub-agents (specialists) that AuraGo can spawn for complex tasks.

```yaml
co_agents:
    enabled: false
    max_concurrent: 3
    budget_quota_percent: 0            # daily budget share reserved for co-agents (0 = disabled)
    max_context_hints: 5
    max_context_hint_chars: 500
    max_result_bytes: 50000
    queue_when_busy: false
    cleanup_interval_minutes: 10
    cleanup_max_age_minutes: 30
    llm:
        provider: ""                   # provider ID for co-agent LLM calls
    circuit_breaker:
        max_tool_calls: 50
        timeout_seconds: 120
        max_tokens: 100000
    retry_policy:
        max_retries: 1
        retry_delay_seconds: 5
        retryable_error_patterns:
            - "rate limit"
            - "timeout"
            - "temporary"
    specialists:
        researcher:
            enabled: true
            system_prompt: ""
        coder:
            enabled: true
            system_prompt: ""
        designer:
            enabled: true
            system_prompt: ""
        security:
            enabled: true
            system_prompt: ""
        writer:
            enabled: true
            system_prompt: ""
```

### Co-Agents Options Explained

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Enable parallel co-agent execution |
| `max_concurrent` | `3` | Maximum simultaneous co-agents |
| `budget_quota_percent` | `0` | Daily budget percentage reserved for co-agents |
| `max_result_bytes` | `50000` | Truncate co-agent results after this many bytes |
| `queue_when_busy` | `false` | Queue requests instead of rejecting when at capacity |
| `circuit_breaker.max_tool_calls` | `50` | Maximum tool calls before aborting a co-agent |
| `retry_policy.max_retries` | `1` | Retries for transient LLM failures |
| `specialists.*.enabled` | `true` | Enable individual specialist types |

---

## Logging Configuration

```yaml
logging:
    enable_file_log: true
    enable_prompt_log: false    # write full LLM requests to log/prompts.log (may contain sensitive data)
    log_dir: ./log
```

| Setting | Default | Description |
|---------|---------|-------------|
| `enable_file_log` | `true` | Write logs to files |
| `enable_prompt_log` | `false` | Write full LLM requests to `log/prompts.log` (may contain sensitive data) |
| `log_dir` | `./log` | Directory for log files |

Log files are rotated automatically and named by date:
- `log/aurago_2026-03-08.log`
- `log/aurago_2026-03-07.log`

---

## Environment Variables

AuraGo supports environment variables for configuration overrides. This is useful for Docker deployments and secrets management.

### Variable Naming Convention

Environment variables use uppercase with underscores, prefixed with `AURAGO_`:

```bash
# Server settings
AURAGO_SERVER_HOST=127.0.0.1
AURAGO_SERVER_PORT=8088

# Security
AURAGO_MASTER_KEY=...               # 64-character hex encryption key

# LLM settings
LLM_API_KEY=sk-or-v1-...
OPENAI_API_KEY=sk-...

# Integrations
TAILSCALE_API_KEY=tskey-...
ANSIBLE_API_TOKEN=...
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
      - AURAGO_MASTER_KEY=${AURAGO_MASTER_KEY}
      - LLM_API_KEY=${LLM_API_KEY}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - TAILSCALE_API_KEY=${TAILSCALE_API_KEY}
      - ANSIBLE_API_TOKEN=${ANSIBLE_API_TOKEN}
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/config.yaml
```

> 💡 **Tip:** Use a `.env` file for local development to keep secrets out of version control.

---

## Common Configuration Examples

> 💡 **Tip:** These YAML examples work for headless or advanced setups, but the **Web UI is the easier way** to configure AuraGo today.

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

## Compact YAML Reference

The blocks below are available for advanced and headless setups. Most can be configured more easily in the **Web UI under Config → Integrations, Config → Security, and Config → Tools**.

| Block | Purpose | YAML Snippet |
|-------|---------|--------------|
| `auth` | Web UI authentication. | `auth:`<br>`  enabled: true`<br>`  session_timeout_hours: 24`<br>`  totp_enabled: false`<br>`  max_login_attempts: 5` |
| `llm_guardian` | Tool/document safety scanning. | `llm_guardian:`<br>`  enabled: false`<br>`  provider: ""`<br>`  model: ""`<br>`  default_level: medium` |
| `guardian` | Regex-based input scanning. | `guardian:`<br>`  max_scan_bytes: 16384`<br>`  scan_edge_bytes: 6144` |
| `ai_gateway` | Cloudflare AI Gateway. | `ai_gateway:`<br>`  enabled: false`<br>`  account_id: ""`<br>`  gateway_id: ""` |
| `mcp_server` | Expose AuraGo as MCP server. | `mcp_server:`<br>`  enabled: false`<br>`  allowed_tools: []`<br>`  require_auth: true` |
| `consolidation` | Nightly memory optimization. | `consolidation:`<br>`  enabled: true`<br>`  auto_optimize: true`<br>`  archive_retain_days: 30`<br>`  max_batch_messages: 200` |
| `web_config` | Web-based config editor. | `web_config:`<br>`  enabled: true` |
| `remote_control` | Distributed remote execution. | `remote_control:`<br>`  enabled: false`<br>`  readonly: false`<br>`  discovery_port: 8092`<br>`  max_file_size_mb: 50` |
| `mission_preparation` | Pre-analyze missions via LLM. | `mission_preparation:`<br>`  enabled: false`<br>`  provider: ""`<br>`  timeout_seconds: 120`<br>`  max_essential_tools: 5` |
| `s3` | S3-compatible storage. | `s3:`<br>`  enabled: false`<br>`  readonly: false`<br>`  endpoint: ""`<br>`  region: us-east-1`<br>`  bucket: ""` |
| `sql_connections` | External DB connections. | `sql_connections:`<br>`  enabled: false`<br>`  max_pool_size: 5`<br>`  connection_timeout_sec: 30`<br>`  query_timeout_sec: 120` |
| `homepage` | Personal dashboard deploy. | `homepage:`<br>`  enabled: false`<br>`  allow_deploy: false`<br>`  allow_container_management: true`<br>`  webserver_port: 8080` |
| `netlify` | Netlify site management. | `netlify:`<br>`  enabled: false`<br>`  readonly: false`<br>`  allow_deploy: true`<br>`  allow_site_management: false` |
| `cloudflare_tunnel` | cloudflared integration. | `cloudflare_tunnel:`<br>`  enabled: false`<br>`  readonly: false`<br>`  mode: auto`<br>`  auto_start: true` |
| `tailscale` | Tailscale VPN integration. | `tailscale:`<br>`  enabled: false`<br>`  readonly: false`<br>`  tailnet: ""`<br>`  tsnet:`<br>`    enabled: false`<br>`    hostname: "aurago"`<br>`    serve_http: false`<br>`    expose_homepage: false`<br>`    funnel: false`<br>`    allow_http_fallback: false` |
| `fritzbox` | AVM Fritz!Box TR-064. | `fritzbox:`<br>`  enabled: false`<br>`  host: fritz.box`<br>`  port: 49000`<br>`  https: true`<br>`  system:`<br>`    enabled: false`<br>`    readonly: false`<br>`  network:`<br>`    enabled: false`<br>`    readonly: false`<br>`  telephony:`<br>`    enabled: false`<br>`    polling:`<br>`      enabled: false` |
| `google_workspace` | Gmail, Calendar, Drive. | `google_workspace:`<br>`  enabled: false`<br>`  readonly: false`<br>`  gmail: false`<br>`  calendar: false`<br>`  drive: false` |
| `telnyx` | SMS and voice integration. | `telnyx:`<br>`  enabled: false`<br>`  readonly: false`<br>`  phone_number: ""`<br>`  messaging_profile_id: ""` |
| `adguard` | AdGuard Home integration. | `adguard:`<br>`  enabled: false`<br>`  readonly: false`<br>`  url: ""`<br>`  username: ""` |
| `image_generation` | AI image generation. | `image_generation:`<br>`  enabled: false`<br>`  provider: ""`<br>`  model: ""`<br>`  default_size: 1024x1024` |
| `onedrive` | Microsoft OneDrive. | `onedrive:`<br>`  enabled: false`<br>`  readonly: false`<br>`  client_id: ""`<br>`  tenant_id: common` |
| `paperless_ngx` | Document management. | `paperless_ngx:`<br>`  enabled: false`<br>`  readonly: false`<br>`  url: ""` |
| `proxmox` | Proxmox VE integration. | `proxmox:`<br>`  enabled: false`<br>`  readonly: false`<br>`  url: ""`<br>`  token_id: ""` |
| `meshcentral` | Remote desktop integration. | `meshcentral:`<br>`  enabled: false`<br>`  readonly: false`<br>`  url: ""`<br>`  username: ""` |
| `ansible` | Ansible sidecar integration. | `ansible:`<br>`  enabled: false`<br>`  readonly: false`<br>`  mode: sidecar`<br>`  url: ""`<br>`  timeout: 300` |
| `ollama` | Local Ollama management. | `ollama:`<br>`  enabled: false`<br>`  readonly: false`<br>`  url: ""`<br>`  managed_instance:`<br>`    enabled: false`<br>`    container_port: 11434`<br>`    use_host_gpu: false`<br>`    gpu_backend: auto`<br>`    default_models: []`<br>`    memory_limit: ""`<br>`    volume_path: ""` |
| `rocketchat` | Rocket.Chat bot. | `rocketchat:`<br>`  enabled: false`<br>`  url: ""`<br>`  user_id: ""`<br>`  channel: ""` |
| `github` | GitHub repository integration. | `github:`<br>`  enabled: false`<br>`  readonly: false`<br>`  owner: ""`<br>`  default_private: false` |
| `tts` | Text-to-speech config. | `tts:`<br>`  provider: google`<br>`  language: en`<br>`  cache_max_files: 500` |
| `notifications` | Push notification providers. | `notifications:`<br>`  ntfy:`<br>`    enabled: false`<br>`    url: ""`<br>`    topic: ""` |
| `budget` | Token cost tracking. | `budget:`<br>`  enabled: false`<br>`  daily_limit_usd: 5`<br>`  enforcement: warn`<br>`  warning_threshold: 0.8`<br>`  default_cost:`<br>`    input_per_million: 1.0`<br>`    output_per_million: 3.0` |
| `fallback_llm` | Failover LLM. | `fallback_llm:`<br>`  enabled: false`<br>`  provider: ""`<br>`  error_threshold: 2`<br>`  probe_interval_seconds: 60` |
| `a2a` | Agent-to-Agent protocol. | `a2a:`<br>`  server:`<br>`    enabled: false`<br>`    port: 0`<br>`    base_path: "/a2a"`<br>`    agent_name: "AuraGo"`<br>`    streaming: true`<br>`  client:`<br>`    enabled: false`<br>`    remote_agents: []` |
| `music_generation` | AI music generation. | `music_generation:`<br>`  enabled: false`<br>`  provider: ""`<br>`  model: ""`<br>`  max_daily: 0` |
| `security_proxy` | Public-facing protection layer. | `security_proxy:`<br>`  enabled: false`<br>`  domain: ""`<br>`  rate_limiting:`<br>`    enabled: true`<br>`    requests_per_second: 10`<br>`  ip_filter:`<br>`    enabled: false`<br>`    mode: blocklist`<br>`  geo_blocking:`<br>`    enabled: false` |
| `egg_mode` | Distributed cluster worker. | `egg_mode:`<br>`  enabled: false`<br>`  master_url: ""`<br>`  egg_id: ""`<br>`  nest_id: ""`<br>`  tls_skip_verify: false` |
| `indexing` | File indexing for RAG. | `indexing:`<br>`  enabled: false`<br>`  poll_interval_seconds: 60`<br>`  index_images: false`<br>`  directories: []` |
| `co_agents` | Parallel sub-agents. | `co_agents:`<br>`  enabled: false`<br>`  max_concurrent: 3`<br>`  budget_quota_percent: 0`<br>`  llm:`<br>`    provider: ""`<br>`  retry_policy:`<br>`    max_retries: 1`<br>`  specialists:`<br>`    researcher:`<br>`      enabled: true` |
| `tools.daemon_skills` | Background daemon tools. | `tools:`<br>`  daemon_skills:`<br>`    enabled: false`<br>`    max_concurrent_daemons: 5` |
| `journal` | Auto journal entries. | `journal:`<br>`  auto_entries: true`<br>`  daily_summary: true` |

---

## Next Steps

Now that you understand AuraGo configuration:

1. **[Integrations](08-integrations.md)** – Configure Telegram, Discord, Email, and more
2. **[Security](14-security.md)** – Harden your AuraGo installation
3. **[Troubleshooting](16-troubleshooting.md)** – Fix common configuration issues

> 💡 **Pro Tip:** Keep a backup of your working `config.yaml` before making major changes. Configuration errors can prevent AuraGo from starting.

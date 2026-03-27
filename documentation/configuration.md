# AuraGo Configuration Reference

All settings live in a single `config.yaml` file in the project root directory. Copy `config.yaml`, fill in your values, and start AuraGo.

> **Minimal required:** `llm.api_key` — everything else has sensible defaults.

---

## `server`

| Key | Default | Description |
|---|---|---|
| `host` | `"127.0.0.1"` | Bind address. Use `"0.0.0.0"` to allow LAN/network access. **Never expose 0.0.0.0 to the internet without a reverse proxy + authentication.** |
| `port` | `8088` | HTTP port for the Web UI and REST API. |

---

## `llm`

Primary LLM provider for all agent reasoning.

| Key | Default | Description |
|---|---|---|
| `provider` | `"openrouter"` | Provider name (informational, no functional effect). |
| `base_url` | `"https://openrouter.ai/api/v1"` | OpenAI-compatible API base URL. Works with OpenRouter, Ollama, LM Studio, vLLM, etc. |
| `api_key` | `""` | **Required.** Your API key. |
| `model` | `"arcee-ai/trinity-large-preview:free"` | Model identifier as used by the provider. |

---

## `embeddings`

Used for long-term memory (RAG) vector indexing.

| Key | Default | Description |
|---|---|---|
| `provider` | `"internal"` | `"internal"` = use the main LLM provider's API for embeddings. `"external"` = call a dedicated OpenAI-compatible embeddings endpoint (e.g. Ollama). `"disabled"` = completely disable long-term memory / VectorDB. |
| `external_url` | `"http://localhost:11434/v1"` | URL of the external embeddings API (e.g. Ollama). Only used when `provider = "external"`. |
| `external_model` | `"nomic-embed-text"` | Model name for external embeddings. |
| `api_key` | `""` | API key for external embeddings provider. Falls back to `EMBEDDINGS_API_KEY` env var, then to `llm.api_key`. Leave empty or `dummy_key` for Ollama (no auth required). |

---

## `agent`

Core agent behaviour settings.

| Key | Default | Description |
|---|---|---|
| `system_language` | `"German"` | Language for system prompts and agent responses. Any natural language name works (e.g. `"English"`, `"French"`). |
| `max_tool_calls` | `12` | Maximum consecutive tool calls the agent can make per user request before aborting. Prevents runaway loops. |
| `step_delay_seconds` | `0` | Pause (seconds) between tool calls. Useful to avoid rate-limiting (HTTP 429) errors with slow providers. |
| `memory_compression_char_limit` | `50000` | Character threshold at which the agent compresses older messages in the prompt. Roughly 50% of the model's context window in tokens. |
| `personality_engine` | `true` | Enables the heuristic mood & trait engine. The agent's tone subtly shifts based on interaction patterns. Zero extra LLM calls. |
| `core_personality` | `"friend"` | Base personality template. Built-in options include `neutral`, `friend`, `professional`, `punk`, `psycho`, `mcp`, `terminator`. Custom profiles live in `prompts/personalities/*.md` and can define additional `meta` fields like `anchor_traits`, `decay_resistance`, and `thresholds` for advanced behaviour tuning. |
| `system_prompt_token_budget` | `8192` | Soft cap on system prompt tokens. Auto-adjusted upward if the model's context window is detected and large enough. |
| `context_window` | `0` | Model context window size in tokens. `0` = auto-detect from provider API at startup. Override if auto-detect fails. |
| `use_native_functions` | `false` | `true` = send tool schemas via the OpenAI function-calling API. `false` = inject tools as text in the system prompt (more compatible with open-weight models). |
| `show_tool_results` | `false` | Show tool call results in the Web UI by default. Can be toggled live with `/debug on\|off`. |
| `debug_mode` | `true` | Inject debug instructions into the system prompt so the agent reports internal errors with helpful details. |

---

## `telegram`

| Key | Default | Description |
|---|---|---|
| `bot_token` | `""` | Telegram bot token from [@BotFather](https://t.me/botfather). Leave empty to disable Telegram. |
| `telegram_user_id` | `0` | Numeric Telegram user ID of the allowed user. `0` = silent discovery mode (first message sender becomes the owner). |

See [telegram_setup.md](telegram_setup.md) for full setup instructions.

---

## `discord`

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable Discord bot integration. |
| `read_only` | `false` | When `true`, the agent can only read Discord messages but cannot send them. |
| `bot_token` | `""` | Bot token from the [Discord Developer Portal](https://discord.com/developers/applications). |
| `guild_id` | `""` | Server (guild) ID for channel listing commands. |
| `allowed_user_id` | `""` | Restrict access to a specific Discord user ID. Leave empty to allow all users in the server. |
| `default_channel_id` | `""` | Default channel for outbound agent messages when no channel is specified. |

---

## `whisper`

Speech-to-text for Telegram voice messages.

| Key | Default | Description |
|---|---|---|
| `provider` | `"openrouter"` | Provider name (informational). |
| `api_key` | `""` | Falls back to `llm.api_key` if empty. |
| `base_url` | `"https://openrouter.ai/api/v1"` | API base URL. |
| `model` | `"google/gemini-2.5-flash-lite-preview-09-2025"` | Model used for transcription. |

---

## `vision`

Image analysis for the `analyze_image` tool and Telegram photo messages.

| Key | Default | Description |
|---|---|---|
| `provider` | `"openrouter"` | Provider name (informational). |
| `api_key` | `""` | Falls back to `llm.api_key` if empty. |
| `base_url` | `"https://openrouter.ai/api/v1"` | API base URL. |
| `model` | `"google/gemini-2.5-flash-lite-preview-09-2025"` | Vision-capable model (must support image inputs). |

---

## `maintenance`

Scheduled nightly agent run for housekeeping. The agent loads `agent_workspace/prompts/maintenance.md` and executes autonomously.

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable the nightly maintenance loop. |
| `time` | `"04:00"` | Time to run in `HH:MM` (24h, local system time). |
| `lifeboat_enabled` | `true` | Allow the agent to trigger self-modification (code surgery) via the lifeboat binary. **Use with caution.** |
| `lifeboat_port` | `8090` | Internal TCP port used for lifeboat ↔ aurago communication. |

---

## `fallback_llm`

Automatic failover to a second LLM endpoint when the primary fails repeatedly.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable LLM failover. |
| `base_url` | `""` | Fallback provider API URL. |
| `api_key` | `""` | Fallback API key. |
| `model` | `""` | Fallback model name. |
| `error_threshold` | `2` | Number of consecutive errors before switching to the fallback. |
| `probe_interval_seconds` | `60` | How often (seconds) the primary is probed for recovery. |

---

## `circuit_breaker`

Safeguards against infinite loops, hangs, and runaway tool calls.

| Key | Default | Description |
|---|---|---|
| `max_tool_calls` | `20` | Hard limit on tool calls per request (overrides `agent.max_tool_calls` if lower). |
| `llm_timeout_seconds` | `180` | Timeout for a single LLM API call. |
| `maintenance_timeout_minutes` | `10` | Maximum duration for a nightly maintenance run. |
| `retry_intervals` | `["10s","2m","10m"]` | Backoff intervals for LLM API errors before giving up. |

---

## `logging`

| Key | Default | Description |
|---|---|---|
| `log_dir` | `"./log"` | Directory for log files. |
| `enable_file_log` | `true` | Write logs to rotating files in `log_dir` in addition to stdout. |

---

## `email`

IMAP inbox monitoring and SMTP sending.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable email integration. |
| `read_only` | `false` | When `true`, the agent can only read emails but cannot send them. |
| `imap_host` | `""` | IMAP server hostname (e.g. `imap.gmail.com`). |
| `imap_port` | `993` | IMAP port. `993` = IMAPS (TLS). |
| `smtp_host` | `""` | SMTP server hostname (e.g. `smtp.gmail.com`). |
| `smtp_port` | `587` | SMTP port. `587` = STARTTLS. Use `465` for implicit TLS. |
| `username` | `""` | Email address / login. |
| `password` | `""` | App password (not your regular account password). |
| `from_address` | `""` | Sender address. Defaults to `username` if empty. |
| `watch_enabled` | `false` | Periodically poll inbox for new emails and wake the agent. |
| `watch_interval_seconds` | `120` | Poll interval in seconds. |
| `watch_folder` | `"INBOX"` | IMAP folder to watch. |

---

## `home_assistant`

Smart-home control via the Home Assistant REST API.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable Home Assistant integration. |
| `read_only` | `false` | When `true`, the agent can only read device states but cannot call services (no toggling devices). |
| `url` | `"http://localhost:8123"` | Home Assistant base URL. |
| `access_token` | `""` | Long-Lived Access Token (generate in your HA profile). |

---

## `docker`

Container management via the Docker Engine API.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable Docker integration. |
| `read_only` | `false` | When `true`, the agent can only list/inspect containers and images but cannot start, stop, create, or remove them. |
| `host` | `""` | Docker socket/host. Empty = auto-detect (`/var/run/docker.sock` on Linux, TCP on Windows). |

See [docker.md](docker.md) for details.

---

## `co_agents`

Parallel sub-agent system — spawn independent LLM workers for complex sub-tasks.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable the co-agent system. |
| `max_concurrent` | `3` | Maximum number of simultaneously running co-agents. |
| `llm.provider` | `"openrouter"` | Co-agent LLM provider (informational). |
| `llm.base_url` | `""` | Falls back to `llm.base_url` if empty. Use a faster/cheaper model here. |
| `llm.api_key` | `""` | Falls back to `llm.api_key` if empty. |
| `llm.model` | `""` | Falls back to `llm.model` if empty. Recommended: a smaller, faster model. |
| `circuit_breaker.max_tool_calls` | `10` | Tool call limit per co-agent task. |
| `circuit_breaker.timeout_seconds` | `300` | Max runtime per co-agent in seconds. |
| `circuit_breaker.max_tokens` | `0` | Token budget per co-agent task. `0` = unlimited. |

See [co_agent_concept.md](co_agent_concept.md) for details.

---

## `budget`

Optional daily token cost tracking and enforcement.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable budget tracking. |
| `daily_limit_usd` | `1.00` | Daily spending limit in USD. `0` = track only, never block. |
| `enforcement` | `"warn"` | `warn` = log + UI warning only. `partial` = block co-agents, vision, STT when exceeded. `full` = block all LLM calls. |
| `reset_hour` | `0` | Hour (0–23) for daily counter reset. `0` = midnight. |
| `warning_threshold` | `0.8` | Trigger warning at this fraction of `daily_limit_usd` (e.g. `0.8` = 80%). |
| `models` | *(list)* | Per-model cost rates. Each entry: `name`, `input_per_million`, `output_per_million` (USD). |
| `default_cost.input_per_million` | `1.00` | Fallback input cost for models not listed above. |
| `default_cost.output_per_million` | `3.00` | Fallback output cost for models not listed above. |

---

## `directories`

Override default runtime directory paths. All paths are relative to the working directory (where `aurago` is started from).

| Key | Default |
|---|---|
| `data_dir` | `"./data"` |
| `workspace_dir` | `"./agent_workspace/workdir"` |
| `tools_dir` | `"./agent_workspace/tools"` |
| `prompts_dir` | `"./prompts"` |
| `skills_dir` | `"./agent_workspace/skills"` |
| `vectordb_dir` | `"./data/vectordb"` |

---

## `sqlite`

Paths for the three SQLite databases.

| Key | Default |
|---|---|
| `short_term_path` | `"./data/short_term.db"` |
| `long_term_path` | `"./data/long_term.db"` |
| `inventory_path` | `"./data/inventory.db"` |

---

## `tools`

Enable/disable built-in agent tools and set read-only mode. All tools are enabled by default when the `tools:` section is not present in config.yaml (backward compatible).

### `tools.memory`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable core memory tools (`manage_memory`, `query_memory`). |
| `read_only` | `false` | When `true`, the agent can only read memories but cannot add, update, or delete them. |

### `tools.knowledge_graph`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable the knowledge graph tool. |
| `read_only` | `false` | When `true`, the agent can only search the graph but cannot add/delete nodes or edges. |

### `tools.secrets_vault`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable the secrets vault tool. |
| `read_only` | `false` | When `true`, the agent can only read secrets but cannot store new ones. |

### `tools.scheduler`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable the cron scheduler tool. |
| `read_only` | `false` | When `true`, the agent can only list scheduled jobs but cannot add or remove them. |

### `tools.notes`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable notes/todo management. |
| `read_only` | `false` | When `true`, the agent can only view notes but cannot create, modify, or delete them. |

### `tools.missions`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable Mission Control. |
| `read_only` | `false` | When `true`, the agent can only view missions but cannot create, modify, delete, or run them. |

### `tools.stop_process`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable the `stop_process` tool. When disabled, the agent can only list processes. |

### `tools.inventory`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable device inventory queries. |

### `tools.memory_maintenance`

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable memory maintenance tools (`archive_memory`, `optimize_memory`). |

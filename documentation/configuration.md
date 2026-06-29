# AuraGo Configuration Reference

All settings live in a single `config.yaml` file in the project root directory. Copy `config.yaml`, fill in your values, and start AuraGo.

> **Authoritative defaults:** When this document and your running `config.yaml` disagree, treat **`config_template.yaml`** in the repository root as the source of truth for default values and available keys.

> **Minimal required:** At least one `providers` entry with `api_key`, and `llm.provider` referencing it — everything else has sensible defaults.

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
| `provider` | `""` | **Required.** Provider entry ID from the `providers` list. |
| `use_native_functions` | `true` | Deprecated compatibility fallback for native function calling. Provider/model `capabilities.tool_calling` decides this for known models. |
| `temperature` | `0.7` | Creativity/randomness (0.0–2.0). |
| `structured_outputs` | `false` | Deprecated compatibility fallback for strict structured tool schemas. Provider/model `capabilities.structured_outputs` decides this for known models. |
| `multimodal` | `false` | Deprecated compatibility fallback for image-capable chat models. Provider/model `capabilities.multimodal` decides this for known models. |
| `helper_enabled` | `false` | Enable dedicated helper LLM for internal analysis. |
| `helper_provider` | `""` | Provider ID for helper LLM (smaller/cheaper recommended). |
| `helper_model` | `""` | Model override for helper LLM. |

> ⚠️ **Legacy:** `base_url`, `api_key`, and `model` under `llm` still work for backward compatibility, but the provider system (`providers` + `llm.provider`) is the recommended approach.

---

## `providers`

Provider entries describe concrete API endpoints and models. Model capabilities are stored per provider/model so tool calling, structured outputs, and multimodal uploads can be enabled only where the selected model supports them.

```yaml
providers:
  - id: main
    type: openrouter
    name: "Main LLM"
    base_url: "https://openrouter.ai/api/v1"
    model: "openai/gpt-4o"
    capabilities:
      auto: true
      tool_calling: true
      structured_outputs: true
      multimodal: true
      detected_model: "openai/gpt-4o"
      source: "openrouter"
```

| Key | Default | Description |
|---|---|---|
| `capabilities.auto` | `true` | When `true` or omitted, AuraGo refreshes capability checkboxes from model metadata when the provider type/model changes. |
| `capabilities.tool_calling` | `false` | Enables native tool calling for this provider/model when effective. |
| `capabilities.structured_outputs` | `false` | Enables strict structured output schemas for this provider/model when effective. |
| `capabilities.multimodal` | `false` | Enables image/file input promotion for the main chat path when effective. |
| `capabilities.detected_model` | `""` | Model ID the stored detected values belong to. Used to refresh stale auto values after model changes. |
| `capabilities.source` | `""` | Detection source: `manual`, `openrouter`, `models.dev`, `heuristic`, or `legacy_fallback`. |

Detection order is manual provider overrides first, then live OpenRouter metadata when available, the generated `models.dev` registry, conservative local heuristics, and finally the deprecated global `llm` fallback fields for unknown models.

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
| `system_language` | `"English"` | Language for system prompts and agent responses. Any natural language name works (e.g. `"German"`, `"French"`). |
| `max_tool_calls` | `15` | Maximum consecutive tool calls the agent can make per user request before aborting. Prevents runaway loops. |
| `step_delay_seconds` | `0` | Pause (seconds) between tool calls. Useful to avoid rate-limiting (HTTP 429) errors with slow providers. |
| `memory_compression_char_limit` | `60000` | Character threshold at which the agent compresses older messages in the prompt. |
| `tool_output_limit` | `50000` | Max characters of a single tool result fed into context (`0` = unlimited). |
| `discover_tools_snapshot_ttl_minutes` | `5` | Minutes to retain `discover_tools` snapshots; values `<=0` fall back to `5`. |
| `context_window` | `0` | Model context window size in tokens. `0` = auto-detect from provider API at startup. |
| `system_prompt_token_budget` | `0` | Soft cap on system prompt tokens (`0` = automatic). |
| `adaptive_system_prompt_token_budget` | `true` | Automatically adjust system prompt token budget. |
| `show_tool_results` | `false` | Show tool call results in the Web UI by default. Can be toggled live with `/debug on\|off`. |
| `debug_mode` | `false` | Inject debug instructions into the system prompt so the agent reports internal errors with helpful details. |
| `adaptive_tools.enabled` | `true` | Enable adaptive tool filtering to reduce token usage. Explicit `false` keeps all enabled tools visible. |
| `adaptive_tools.max_tools` | `10` | Maximum adaptive/preferred tool schemas before required and always-include tools are added. |
| `adaptive_tools.max_total_tools` | `20` | Maximum final native tool schemas after hard-required tools are kept. |
| `adaptive_tools.max_schema_tokens` | `6500` | Optional schema-token cap for adaptive native tools. Explicit `0` disables the token cap. |
| `adaptive_tools.provider_profiles_enabled` | `true` | Apply provider-specific stability overlays for tool budgets and response-header timeouts. |
| `adaptive_tools.session_tool_retention_turns` | `8` | Keep tools used in recent turns visible as soft always-include tools. |
| `importance_scoring.enabled` | `true` | Score messages so context trimming keeps the most useful conversation entries. |
| `importance_scoring.mode` | `"active"` | `"active"` applies importance-based trimming; `"log_only"` records diagnostics only. |
| `auto_learning.enabled` | `true` | Learn small reusable rules from recurring tool/error patterns and successful recoveries. |
| `auto_learning.mode` | `"active"` | `"active"` injects relevant learned rules into prompts; `"log_only"` keeps them out of prompts. |
| `reuse_first.auto_materialize` | `true` | Automatically create cheatsheets/skills after substantial reusable runs. |
| `reuse_first.require_success_signal` | `true` | Only auto-create reuse artifacts when the turn completed without tool errors. |
| `reuse_first.min_steps` | `3` | Minimum distinct tool steps before automatic reuse materialization can run. |
| `reuse_first.max_artifacts_per_session` | `1` | Maximum automatic cheatsheets and skills per session. |
| `history_compaction.enabled` | `true` | Compact older complete native tool-call rounds before LLM history compression. |
| `history_compaction.keep_recent_tool_rounds_full` | `2` | Keep this many recent complete tool-call rounds unchanged. |
| `recovery.max_provider_422_recoveries` | `3` | Automatic retries after provider 422 errors. |
| `background_tasks.enabled` | `true` | Enable persistent background task execution. |

### Output Compression

Reduces token usage by applying `tool_output_limit` truncation to oversized outputs first, then compressing the retained content. Enabled by default.

```yaml
agent:
    output_compression:
        enabled: true
        min_chars: 500
        preserve_errors: true
        shell_compression: true
        python_compression: true
        api_compression: true
        reversible:
            enabled: true
            primary_output_vault: true
            max_inline_chars: 6000
```

> See [output_compression.md](output_compression.md) for details.

## `personality`

> 🆕 Moved from `agent` to its own top-level section in recent versions.

| Key | Default | Description |
|---|---|---|
| `engine` | `true` | Enable heuristic mood & trait engine. |
| `engine_v2` | `true` | Enable LLM-based mood analysis. |
| `core_personality` | `"friend"` | Base personality template. |
| `user_profiling` | `false` | Auto-detect user preferences from conversation. |
| `user_profiling_threshold` | `2` | Confirmations needed before injecting a detected trait into the prompt. |
| `emotion_synthesizer.enabled` | `false` | Enable emotion synthesis. |
| `inner_voice.enabled` | `false` | Enable subconscious nudge engine (requires `emotion_synthesizer` + `engine_v2`). |

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
| `allowed_user_id` | `""` | Required Discord user ID for inbound control. Leave empty to block user messages. |
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

While nightly maintenance runs, user chats are briefly treated as maintenance via the global busy lock. The dashboard shows this as `busy`.

### `maintenance.retention`

Retention windows for deterministic nightly cleanups. Defaults match the previous hardcoded values.

| Key | Default | Maps to |
|---|---|---|
| `patterns_days` | `90` | Old interaction patterns |
| `archive_events_days` | `90` | Archive event log entries |
| `mood_log_days` | `30` | Mood log entries |
| `error_patterns_days` | `7` | Stale unresolved error patterns |
| `profile_stale_days` | `30` | Low-confidence user profile entries |
| `done_notes_days` | `7` | Completed notes marked done |
| `operational_issues_days` | `30` | Planner operational issues |

The 03:00 daily reflection loop skips its LLM pass when nightly maintenance recently produced a daily summary, avoiding duplicate work on the same archive data.

Each nightly run is recorded in the `maintenance_runs` SQLite table (status, timestamps, phase results JSON). The dashboard overview and `/api/dashboard/maintenance/status` expose `last_run`, `last_status`, and `next_run`.

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
| `llm_timeout_seconds` | `600` | Timeout for a single LLM API call. |
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
| `relay_cheatsheet_id` | `""` | Optional cheatsheet appended as trusted instructions when the inbox watcher forwards new mail to the agent. |

---

## `agentmail`

AgentMail API inboxes, messages, labels, drafts, attachments, and optional inbound relay.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable the AgentMail integration and expose focused `agentmail_*` tools. Legacy `agentmail` dispatch remains accepted. |
| `readonly` | `false` | When `true`, the agent can list/read AgentMail data but cannot create, update, delete, send, reply, or forward. |
| `api_key` | vault-only | Store via the UI or vault key `agentmail_api_key`; it is not written to `config.yaml`. |
| `inbox_id` | `""` | Primary AgentMail inbox ID used by the UI, relay service, and tool defaults. |
| `auto_create_inbox` | `false` | Reserved for setup flows that create the configured inbox automatically. |
| `username` | `""` | Inbox username to use when creating an inbox. |
| `domain` | `""` | Optional AgentMail domain for inbox creation. |
| `display_name` | `""` | Display name for the inbox. |
| `use_websocket` | `true` | Prefer AgentMail WebSockets for inbound mail relay. |
| `poll_interval_seconds` | `120` | Poll interval used as fallback or when WebSockets are disabled. |
| `relay_to_agent` | `false` | Wake the agent when new messages arrive in `inbox_id`. Disabled in egg mode. |
| `relay_cheatsheet_id` | `""` | Optional cheat sheet whose content is appended as instructions to each relayed new-mail prompt. |
| `max_attachment_mb` | `10` | Maximum size for attachments sent through AgentMail tools. |
| `base_url` | `"https://api.agentmail.to"` | AgentMail REST API base URL. |
| `websocket_url` | `"wss://ws.agentmail.to/v0"` | AgentMail WebSocket endpoint. |

AgentMail is separate from the legacy IMAP/SMTP `email` integration, so existing `fetch_email` and `send_email` tools keep their current behavior.

---

## `home_assistant`

Smart-home control via the Home Assistant REST API.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable Home Assistant integration. |
| `read_only` | `false` | When `true`, the agent can only read device states but cannot call services (no toggling devices). |
| `url` | `"http://localhost:8123"` | Home Assistant base URL. |
| `access_token` | `""` | Long-Lived Access Token (generate in your HA profile). |
| `allowed_services` | `[]` | Optional `call_service` allowlist. Empty allows all services unless blocked by `blocked_services`. |
| `blocked_services` | `[]` | Explicit `call_service` denylist, evaluated before `allowed_services`. |

---

## `docker`

Container management via the Docker Engine API.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable Docker integration. |
| `read_only` | `false` | When `true`, the agent can only list/inspect containers and images but cannot start, stop, create, or remove them. |
| `host` | `""` | Docker socket/host. Empty = auto-detect (`/var/run/docker.sock` on Linux, `npipe:////./pipe/docker_engine` on Windows). |

See [docker_installation.md](docker_installation.md) and [manual/en/08-integrations.md#docker-integration](manual/en/08-integrations.md#docker-integration) for details.

---

## `co_agents`

Parallel sub-agent system — spawn independent LLM workers for complex sub-tasks.

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable the co-agent system. |
| `max_concurrent` | `3` | Maximum number of simultaneously running co-agents. |
| `budget_quota_percent` | `25` | Daily budget percentage reserved for co-agents (`0` = disabled). |
| `max_result_bytes` | `100000` | Truncate co-agent results after this many bytes. |
| `queue_when_busy` | `true` | Queue requests when all co-agent slots are busy. |
| `cleanup_interval_minutes` | `10` | Interval for stale co-agent cleanup. |
| `cleanup_max_age_minutes` | `30` | Max age before a finished co-agent entry is removed. |
| `max_context_hints` | `6` | Max context hints passed into co-agent prompts. |
| `max_context_hint_chars` | `180` | Max characters per context hint. |
| `llm.provider` | `""` | Provider ID for co-agents; empty inherits the main LLM provider. |
| `llm.base_url` | `""` | Falls back to main provider `base_url` if empty. |
| `llm.api_key` | `""` | Falls back to main provider `api_key` if empty. |
| `llm.model` | `""` | Falls back to main provider `model` if empty. Recommended: a smaller, faster model. |
| `circuit_breaker.max_tool_calls` | `12` | Tool call limit per co-agent task. |
| `circuit_breaker.timeout_seconds` | `300` | Max runtime per co-agent in seconds. |
| `circuit_breaker.max_tokens` | `0` | Token budget per co-agent task. `0` = unlimited. |
| `retry_policy.max_retries` | `1` | Retries for transient LLM failures. |
| `retry_policy.retry_delay_seconds` | `5` | Delay between co-agent retries. |
| `specialists.*.enabled` | `false` | Enable individual specialist types (`researcher`, `coder`, `designer`, `security`, `writer`). |

See [manual/en/15-coagents.md](manual/en/15-coagents.md) for details.

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

## `rules`

Task rules are Markdown guardrails loaded before matching agent workflows. Built-in defaults live in the binary; user overrides are stored under `prompts/rules/<id>/rule.md`, with optional `prompts/rules/homepage/DESIGN.md` for homepage design guidance. Disk files override embedded defaults and can be restored from the Config UI.

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Enable automatic task-rule injection based on tool, workflow, and keyword matching. |

Manage rules in **Config → Rules**. Homepage tasks also receive the global homepage `DESIGN.md`; if a homepage project contains its own `DESIGN.md`, AuraGo reads it as design context only.

---

## `sqlite`

Paths for AuraGo's SQLite databases. Long-term semantic memory is **not** stored here — it lives in `directories.vectordb_dir` (chromem). Back up the vector store separately with `include_vectordb` in backup settings.

| Key | Default | Notes |
|---|---|---|
| `short_term_path` | `"./data/short_term.db"` | Conversation short-term memory |
| `long_term_path` | `"./data/long_term.db"` | **Deprecated** — legacy SQLite LTM; included in backups only when the file still exists |
| `inventory_path` | `"./data/inventory.db"` | SSH device inventory |
| `invasion_path` | `"./data/invasion.db"` | Invasion Control |
| `cheatsheet_path` | `"./data/cheatsheets.db"` | Cheatsheets |
| `image_gallery_path` | `"./data/image_gallery.db"` | Image gallery metadata |
| `remote_control_path` | `"./data/remote_control.db"` | Remote control sessions |
| `media_registry_path` | `"./data/media_registry.db"` | Media registry |
| `homepage_registry_path` | `"./data/homepage_registry.db"` | Homepage projects |
| `contacts_path` | `"./data/contacts.db"` | Address book |
| `planner_path` | `"./data/planner.db"` | Planner |
| `virtual_desktop_path` | `"./data/virtual_desktop.db"` | Virtual desktop state |
| `site_monitor_path` | `"./data/site_monitor.db"` | Site monitoring |
| `sql_connections_path` | `"./data/sql_connections.db"` | External SQL connection configs |
| `skills_path` | `"./data/skills.db"` | Skills registry |
| `knowledge_graph_path` | `"./data/knowledge_graph.db"` | Knowledge graph |
| `optimization_path` | `"./data/optimization.db"` | Optimizer state |
| `prepared_missions_path` | `"./data/prepared_missions.db"` | Prepared missions |
| `mission_history_path` | `"./data/mission_history.db"` | Mission history |
| `push_path` | `"./data/push.db"` | Push notifications |
| `launchpad_path` | `"./data/launchpad.db"` | Launchpad apps |

Additional SQLite files under `directories.data_dir` are also backed up when present: `system_tasks.db`, `galaxa.db`, and `desktop_store.db`.

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
| `enabled` | `true` | Enable the cron scheduler runtime and tool. When `false`, persisted jobs are loaded but not run. |
| `read_only` | `false` | When `true`, active jobs keep running but the agent can only list scheduled jobs and cannot add, remove, enable, or disable them. |

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

---

# Kosten und LLM-Auswahl

Dieser Abschnitt gibt einen Überblick über die Kostenstruktur beim Betrieb von AuraGo und hilft bei der Auswahl der passenden LLM-Provider und Modelle.

## LLM-Auswahl: Was ist wichtig?

Bei der Auswahl eines LLMs für AuraGo sollten folgende Faktoren berücksichtigt werden:

| Faktor | Bedeutung |
|--------|-----------|
| **Kontextfenster** | Größere Kontextfenster (128K+ Token) ermöglichen längere Gespräche und komplexere Aufgaben ohne Speicherverlust. |
| **Tool-Calling-Fähigkeit** | Das Modell sollte zuverlässig Funktionsaufrufe (Tools) generieren können. Flash-Modelle mit Funktionsaufruf-Unterstützung sind oft ausreichend. |
| **Geschwindigkeit** | Für Echtzeit-Interaktionen sind schnelle Modelle (Flash, Lite) besser geeignet. |
| **Kosten** | Die Kosten pro 1 Million Token variieren stark zwischen kostenlosen, günstigen und Premium-Modellen. |
| **Verfügbarkeit** | Einige Modelle haben Rate-Limits oder sind nur zu bestimmten Zeiten verfügbar. |

### Empfohlene Modellkategorien

- **Haupt-Agent**: Ein mittelgroßes Modell mit gutem Preis-Leistungs-Verhältnis (z.B. Gemini Flash, MiniMax)
- **Co-Agenten (Sub-Agenten)**: Kleinere, schnelle und besonders preiswerte Modelle, da hier die Token-Menge schnell summiert
- **Vision**: Ein multimodales Modell mit Bildverständnis (z.B. Gemini Flash mit Vision)
- **Whisper (STT)**: Ein schnelles Modell für Sprach-zu-Text (Flash-Kategorie)

---

## OpenRouter: Kostenlos experimentieren

**OpenRouter** (openrouter.ai) ist ein universeller API-Gateway, der Zugriff auf über 200 LLMs verschiedener Anbieter bietet.

### Vorteile für Einsteiger

| Feature | Beschreibung |
|---------|--------------|
| **Gratis-Modelle** | OpenRouter bietet eine Vielzahl von `:free`-Modellen, die **komplett kostenlos** nutzbar sind |
| **Keine Kreditkarte** | Für die kostenlosen Modelle ist keine Zahlungsmethode erforderlich |
| **Breite Auswahl** | Experimentieren Sie mit verschiedenen Modellen, ohne bei jedem Anbieter ein Konto erstellen zu müssen |
| **Fallback** | Automatisches Routing zu alternativen Modellen bei Ausfällen |

### Kostenlose Modelle für den Start

Für erste Experimente eignen sich folgende `:free`-Modelle:

```yaml
# config.yaml - Beispiel für kostenlose OpenRouter-Modelle
llm:
  provider: openrouter
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-IHRER_KEY_HIER"
  model: "arcee-ai/trinity-large-preview:free"  # Standard: kostenlos, gute Tool-Ausführung

# Alternative kostenlose Optionen:
# model: "google/gemini-2.0-flash-exp:free"     # Schnell, gutes Kontextfenster
# model: "meta-llama/llama-3.3-70b-instruct:free" # Open-Source-Modell
```

> **Hinweis**: Kostenlose Modelle haben oft Rate-Limits (z.B. 20 Anfragen/Minute). Für produktiven Einsatz sollten Sie auf bezahlte Modelle umsteigen.

---

## MiniMax: Preiswerter Betrieb

**MiniMax** (minimaxi.com) bietet einige der **preiswertesten Token-Pläne** auf dem Markt und ist ideal für kostensensiblen Dauerbetrieb.

### Preisvorteile

| Aspekt | Details |
|--------|---------|
| **Token-Preise** | Extrem niedrige Kosten pro 1M Token – oft um die 0,10–0,30 $ |
| **Günstige Co-Agenten** | Besonders für Sub-Agenten geeignet, da hier viele parallele Anfragen entstehen |
| **Gute Leistung** | Die MiniMax-Modelle bieten solide Qualität für Tool-Calling und Textgenerierung |

### Konfiguration für MiniMax

```yaml
# config.yaml - Beispiel für MiniMax als Haupt-Provider
providers:
  - id: minimax
    type: minimax
    name: "MiniMax"
    base_url: "https://api.minimax.io/v1"      # China: https://api.minimaxi.com/v1
    api_key: "IHR_MINIMAX_KEY"
    model: "MiniMax-M2.7"

llm:
  provider: minimax

# Co-Agenten mit MiniMax (besonders preiswert)
co_agents:
  enabled: true
  max_concurrent: 3
  llm:
    provider: minimax
    model: "MiniMax-M2.7-highspeed"  # Optional schnelleres Modell für parallele Aufgaben
```

> **Empfehlung**: MiniMax eignet sich besonders für den Co-Agenten-Betrieb, da hier die Token-Kosten durch parallele Ausführung schnell summieren können.

---

## Sub-Agenten (Co-Agenten): Anforderungen

Die **Co-Agenten** sind parallele Sub-Agenten, die komplexe Aufgaben in kleinere Teilaufgaben aufteilen. Sie haben spezifische Anforderungen:

### Grobe Anforderungen an Co-Agenten-LLMs

| Anforderung | Begründung |
|-------------|------------|
| **Niedrige Latenz** | Co-Agenten laufen oft parallel – langsame Modelle blockieren den Gesamtfortschritt |
| **Kostengünstig** | Bei 3+ parallelen Agenten summieren sich Token-Kosten schnell |
| **Tool-Calling** | Muss zuverlässig Tools aufrufen können, aber nicht perfekt |
| **Kleines Kontextfenster ausreichend** | Co-Agenten bearbeiten meist spezifische, begrenzte Teilaufgaben |
| **Rate-Limit-tolerant** | Viele parallele Anfragen erfordern gute Rate-Limits oder günstige Preise |

### Empfohlene Strategie

1. **Haupt-Agent**: Qualitativ hochwertiges Modell mit großem Kontextfenster (z.B. Gemini Flash, GPT-4o-mini)
2. **Co-Agenten**: Preiswerte, schnelle Modelle (z.B. MiniMax, kleinere OpenRouter-Modelle)
3. **Vision/STT**: Dedizierte, schnelle Flash-Modelle

### Beispiel: Optimierte Kostenstruktur

```yaml
# Haupt-Agent: Qualität
llm:
  provider: openrouter
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-..."
  model: "google/gemini-2.0-flash-001"

# Co-Agenten: Preiswert & schnell
co_agents:
  enabled: true
  max_concurrent: 3
  llm:
    provider: openrouter
    base_url: "https://openrouter.ai/api/v1"
    api_key: "sk-or-v1-..."
    model: "google/gemini-2.0-flash-lite-preview-06-17"  # Günstiger Flash-Variante

# Vision: Schnelles Bildverständnis
vision:
  provider: openrouter
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-..."
  model: "google/gemini-2.0-flash-lite-preview-09-2025"
```

---

## Budget-Tracking aktivieren

AuraGo bietet integriertes Budget-Tracking, um die Kosten im Blick zu behalten:

```yaml
budget:
  enabled: true
  daily_limit_usd: 2.00        # Tägliches Limit in USD
  enforcement: warn            # warn = nur Warnung, partial = Co-Agenten blockieren, full = alle LLM-Calls blockieren
  reset_hour: 0                # Reset um Mitternacht
  warning_threshold: 0.8       # Warnung bei 80% des Limits
  models:
    - name: "google/gemini-2.0-flash-001"
      input_per_million: 0.075   # $0,075 pro 1M Input-Token
      output_per_million: 0.30   # $0,30 pro 1M Output-Token
    - name: "MiniMax-Text-01"
      input_per_million: 0.10
      output_per_million: 0.10
```

Mit dieser Konfiguration behalten Sie die Kontrolle über Ihre LLM-Kosten und können das System entsprechend Ihrem Budget optimieren.


---

## `guardian` — PromptSec / Prompt Injection Defense

AuraGo uses the local `promptsec` Go library as the first line of defense against prompt injection. All guards run in-process without external API calls unless LLM-as-Judge escalation is enabled.

```yaml
guardian:
  max_scan_bytes: 16384
  scan_edge_bytes: 6144
  promptsec:
    preset: strict
    spotlight: true
    canary: true
    sanitizer:
      normalize: true
      dehomoglyph: true
      decode: true
    embedding:
      enabled: false
      threshold: 0.65
    policy: ""                       # "", "rag", "support", "coding", "translation", "custom"
    custom_policy:
      disallowed_tasks: []
    taint:
      enabled: false
      default_level: untrusted
    structure:
      enabled: false
      mode: sandwich
    llm_judge:
      enabled: false
      mode: uncertain
      timeout_secs: 2
      policy: ""
    use_sanitized_output: false
```

| Key | Default | Description |
|---|---|---|
| `max_scan_bytes` | `16384` | Maximum bytes scanned by the regex/heuristic guard before windowing. |
| `scan_edge_bytes` | `6144` | Bytes kept from start and end when windowing large inputs. |
| `promptsec.preset` | `"strict"` | Heuristic preset: `strict`, `moderate`, `lenient`. |
| `promptsec.spotlight` | `true` | Data-mark untrusted content for isolation. |
| `promptsec.canary` | `true` | Inject canary tokens to detect prompt/data leakage. |
| `promptsec.sanitizer.normalize` | `true` | Strip zero-width/invisible Unicode characters. |
| `promptsec.sanitizer.dehomoglyph` | `true` | Replace confusable/homoglyph characters (e.g. Cyrillic look-alikes). |
| `promptsec.sanitizer.decode` | `true` | Decode base64/hex-obfuscated payloads before scanning. |
| `promptsec.embedding.enabled` | `false` | Enable local cosine-similarity classifier against known attack vectors. |
| `promptsec.embedding.threshold` | `0.65` | Similarity threshold for the embedding guard (0.0–1.0). |
| `promptsec.policy` | `""` | Context-aware policy: `rag`, `support`, `coding`, `translation`, `custom`. |
| `promptsec.custom_policy.disallowed_tasks` | `[]` | Tasks blocked when `policy: custom`. |
| `promptsec.taint.enabled` | `false` | Track data provenance/trust levels through the pipeline. |
| `promptsec.taint.default_level` | `"untrusted"` | Default trust level: `untrusted`, `suspicious`, `trusted`. |
| `promptsec.structure.enabled` | `false` | Enable sandwich/XML/random enclosure structure enforcement. |
| `promptsec.structure.mode` | `"sandwich"` | Structure mode: `sandwich`, `xml`, `random`. |
| `promptsec.llm_judge.enabled` | `false` | Escalate uncertain/policy detections to the LLM Guardian. |
| `promptsec.llm_judge.mode` | `"uncertain"` | When to call the judge: `uncertain`, `always`, `threat_detected`, `no_threat`. |
| `promptsec.llm_judge.timeout_secs` | `2` | Maximum wait time for the judge LLM. |
| `promptsec.llm_judge.policy` | `""` | Optional app-specific policy text for the judge. |
| `promptsec.use_sanitized_output` | `false` | Forward the sanitized promptsec output to the agent/LLM. |

> **Recommendation:** Keep `sanitizer` enabled (default). Enable `embedding` or `llm_judge` only when you need stronger detection and accept the small latency/cost trade-off.

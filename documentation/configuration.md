# AuraGo Configuration Reference

All settings live in a single `config.yaml` file in the project root directory. Copy `config.yaml`, fill in your values, and start AuraGo.

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
| `use_native_functions` | `true` | Enable native function calling (tool use). |
| `temperature` | `0.7` | Creativity/randomness (0.0–2.0). |
| `structured_outputs` | `false` | Force structured JSON outputs (for supported models). |
| `helper_enabled` | `false` | Enable dedicated helper LLM for internal analysis. |
| `helper_provider` | `""` | Provider ID for helper LLM (smaller/cheaper recommended). |
| `helper_model` | `""` | Model override for helper LLM. |

> ⚠️ **Legacy:** `base_url`, `api_key`, and `model` under `llm` still work for backward compatibility, but the provider system (`providers` + `llm.provider`) is the recommended approach.

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
| `adaptive_tools.enabled` | `false` | Enable adaptive tool filtering to reduce token usage. |
| `adaptive_tools.max_tools` | `60` | Maximum tool schemas to send to the LLM. |
| `recovery.max_provider_422_recoveries` | `3` | Automatic retries after provider 422 errors. |
| `background_tasks.enabled` | `true` | Enable persistent background task execution. |

## `personality`

> 🆕 Moved from `agent` to its own top-level section in recent versions.

| Key | Default | Description |
|---|---|---|
| `engine` | `true` | Enable heuristic mood & trait engine. |
| `engine_v2` | `true` | Enable LLM-based mood analysis. |
| `core_personality` | `"friend"` | Base personality template. |
| `user_profiling` | `false` | Auto-detect user preferences from conversation. |
| `emotion_synthesizer.enabled` | `false` | Enable emotion synthesis. |
| `inner_voice.enabled` | `false` | Enable subconscious nudge engine. |
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
| `host` | `""` | Docker socket/host. Empty = auto-detect (`/var/run/docker.sock` on Linux, `npipe:////./pipe/docker_engine` on Windows). |

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

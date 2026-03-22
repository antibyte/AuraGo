# Kapitel 18: Anhang

Referenzmaterial, vollständige Konfigurationen und nützliche Ressourcen.

## Vollständige Konfigurationsreferenz

### config.yaml Struktur

```yaml
# ============================================
# AuraGo Configuration Reference
# ============================================

server:
  host: "127.0.0.1"           # Bind address (0.0.0.0 for LAN)
  port: 8088                  # HTTP port
  max_body_bytes: 33554432    # Max upload size (32MB)

llm:
  provider: "openrouter"      # Provider name
  base_url: "https://openrouter.ai/api/v1"
  api_key: ""                 # Your API key
  model: "arcee-ai/trinity-large-preview:free"

embeddings:
  provider: "internal"        # internal/external/disabled
  external_url: "http://localhost:11434/v1"
  external_model: "nomic-embed-text"
  api_key: ""

agent:
  system_language: "German"
  max_tool_calls: 12
  enable_google_workspace: true
  step_delay_seconds: 0
  memory_compression_char_limit: 50000
  personality_engine: true
  core_personality: "friend"
  system_prompt_token_budget: 8192
  context_window: 0
  use_native_functions: false
  show_tool_results: false
  debug_mode: true

telegram:
  bot_token: ""
  telegram_user_id: 0

discord:
  enabled: false
  read_only: false
  bot_token: ""
  guild_id: ""
  allowed_user_id: ""
  default_channel_id: ""

whisper:
  provider: "openrouter"
  api_key: ""
  base_url: "https://openrouter.ai/api/v1"
  model: "google/gemini-2.5-flash-lite-preview-09-2025"

vision:
  provider: "openrouter"
  api_key: ""
  base_url: "https://openrouter.ai/api/v1"
  model: "google/gemini-2.5-flash-lite-preview-09-2025"

maintenance:
  enabled: true
  time: "04:00"
  lifeboat_enabled: true
  lifeboat_port: 8090

fallback_llm:
  enabled: false
  base_url: ""
  api_key: ""
  model: ""
  error_threshold: 2
  probe_interval_seconds: 60

circuit_breaker:
  max_tool_calls: 20
  llm_timeout_seconds: 180
  maintenance_timeout_minutes: 10
  retry_intervals: ["10s", "2m", "10m"]

logging:
  log_dir: "./log"
  enable_file_log: true

email:
  enabled: false
  read_only: false
  imap_host: ""
  imap_port: 993
  smtp_host: ""
  smtp_port: 587
  username: ""
  password: ""
  from_address: ""
  watch_enabled: false
  watch_interval_seconds: 120
  watch_folder: "INBOX"

home_assistant:
  enabled: false
  read_only: false
  url: "http://localhost:8123"
  access_token: ""

docker:
  enabled: false
  read_only: false
  host: ""

co_agents:
  enabled: false
  max_concurrent: 3
  llm:
    provider: "openrouter"
    base_url: ""
    api_key: ""
    model: ""
  circuit_breaker:
    max_tool_calls: 10
    timeout_seconds: 300
    max_tokens: 8000

budget:
  enabled: false
  daily_limit_usd: 1.00
  enforcement: "warn"
  reset_hour: 0
  warning_threshold: 0.8
  models: []
  default_cost:
    input_per_million: 1.00
    output_per_million: 3.00

directories:
  data_dir: "./data"
  workspace_dir: "./agent_workspace/workdir"
  tools_dir: "./agent_workspace/tools"
  prompts_dir: "./prompts"
  skills_dir: "./agent_workspace/skills"
  vectordb_dir: "./data/vectordb"

sqlite:
  short_term_path: "./data/short_term.db"
  long_term_path: "./data/long_term.db"
  inventory_path: "./data/inventory.db"

tools:
  memory:
    enabled: true
    read_only: false
  knowledge_graph:
    enabled: true
    read_only: false
  secrets_vault:
    enabled: true
    read_only: false
  scheduler:
    enabled: true
    read_only: false
  notes:
    enabled: true
    read_only: false
  missions:
    enabled: true
    read_only: false
  stop_process:
    enabled: true
  inventory:
    enabled: true
  memory_maintenance:
    enabled: true

auth:
  enabled: false
  password_hash: ""
  session_timeout_hours: 24
  totp_enabled: false
  totp_secret: ""

web_config:
  enabled: true
```

## API Endpoints

### REST API

| Methode | Endpoint | Beschreibung |
|---------|----------|--------------|
| GET | `/api/health` | Health-Check |
| POST | `/api/chat` | Chat-Nachricht senden |
| GET | `/api/history` | Chat-Verlauf abrufen |
| POST | `/api/reset` | Chat zurücksetzen |
| GET | `/api/config` | Konfiguration abrufen |
| PUT | `/api/config` | Konfiguration aktualisieren |
| GET | `/api/memory` | Speicher abfragen |
| GET | `/api/budget` | Budget-Status |
| GET | `/api/co-agents` | Co-Agenten auflisten |

### WebSocket / SSE

| Endpoint | Zweck |
|----------|-------|
| `/events` | Server-Sent Events für Echtzeit-Updates |
| `/ws` | WebSocket für bidirektionale Kommunikation |

## Chat-Befehle Referenz

| Befehl | Beschreibung | Beispiel |
|--------|--------------|----------|
| `/help` | Alle Befehle anzeigen | `/help` |
| `/reset` | Chat zurücksetzen | `/reset` |
| `/stop` | Aktuelle Aktion stoppen | `/stop` |
| `/debug on` | Debug-Modus ein | `/debug on` |
| `/debug off` | Debug-Modus aus | `/debug off` |
| `/budget` | Kosten anzeigen | `/budget` |
| `/personality <name>` | Persönlichkeit wechseln | `/personality professional` |

## Tool-Referenz

### Filesystem-Tools

| Tool | Parameter | Beschreibung |
|------|-----------|--------------|
| `filesystem` | `operation`, `path`, `content` | Datei-Operationen |
| `execute_shell` | `command`, `timeout` | Shell-Befehle |
| `execute_python` | `code`, `timeout` | Python-Code |

### Web-Tools

| Tool | Parameter | Beschreibung |
|------|-----------|--------------|
| `web_search` | `query`, `max_results` | Websuche |
| `fetch_url` | `url`, `extract_text` | URL abrufen |
| `api_request` | `method`, `url`, `headers`, `body` | API-Calls |

### Memory-Tools

| Tool | Parameter | Beschreibung |
|------|-----------|--------------|
| `manage_memory` | `operation`, `key`, `value` | Speicher verwalten |
| `query_memory` | `query`, `limit` | Semantische Suche |
| `knowledge_graph` | `operation`, `entity`, `relation` | Wissensgraph |

### Docker-Tools

| Tool | Parameter | Beschreibung |
|------|-----------|--------------|
| `docker` | `operation`, `container`, `image` | Docker-Management |

### Scheduling-Tools

| Tool | Parameter | Beschreibung |
|------|-----------|--------------|
| `cron_scheduler` | `operation`, `expression`, `task` | Cron-Jobs |
| `missions` | `operation`, `mission_id` | Missionen |

## Beispiel-Konfigurationen

### Minimal (nur Chat)

```yaml
llm:
  api_key: "sk-or-v1-..."
```

Alles andere nutzt Defaults.

### Entwicklung

```yaml
server:
  host: "127.0.0.1"
  port: 8088

llm:
  provider: "openrouter"
  api_key: "sk-or-v1-..."
  model: "mistralai/mixtral-8x7b-instruct"

agent:
  debug_mode: true
  show_tool_results: true
  personality_engine: false

logging:
  enable_file_log: true
```

### Produktion mit Auth

```yaml
server:
  host: "127.0.0.1"
  port: 8088

llm:
  api_key: "sk-or-v1-..."

auth:
  enabled: true
  password_hash: "$2a$10$..."  # bcrypt
  session_timeout_hours: 8
  totp_enabled: true

tools:
  execute_shell:
    enabled: false
  docker:
    enabled: false
```

### Mit Telegram

```yaml
llm:
  api_key: "sk-or-v1-..."

telegram:
  bot_token: "123456:ABC-DEF..."
  telegram_user_id: 123456789
```

### Mit Home Assistant

```yaml
llm:
  api_key: "sk-or-v1-..."

home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  access_token: "eyJ0eXAiOiJKV1Q..."
```

### Lokales Modell (Ollama)

```yaml
llm:
  provider: "ollama"
  base_url: "http://localhost:11434/v1"
  api_key: "dummy"
  model: "llama3.1:8b"

embeddings:
  provider: "external"
  external_url: "http://localhost:11434/v1"
  external_model: "nomic-embed-text"
```

## Umgebungsvariablen

| Variable | Beschreibung | Beispiel |
|----------|--------------|----------|
| `AURAGO_MASTER_KEY` | Vault-Verschlüsselung | `a1b2c3d4...` (64 Hex) |
| `LLM_API_KEY` | Fallback für LLM | `sk-or-v1-...` |
| `EMBEDDINGS_API_KEY` | Fallback für Embeddings | `sk-...` |
| `AURAGO_CONFIG_PATH` | Alternativer Config-Pfad | `/etc/aurago/config.yaml` |
| `AURAGO_DATA_DIR` | Alternatives Datenverzeichnis | `/var/lib/aurago` |
| `LOG_LEVEL` | Logging-Level | `debug`, `info`, `warn`, `error` |

### Master-Key generieren

```bash
# Linux/macOS
export AURAGO_MASTER_KEY=$(openssl rand -hex 32)

# Windows PowerShell
$env:AURAGO_MASTER_KEY = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })
```

## Datei-Pfade

### Standard-Struktur

```
~/aurago/
├── aurago                    # Binary
├── config.yaml               # Konfiguration
├── .env                      # Umgebungsvariablen
├── agent_workspace/
│   ├── prompts/              # System-Prompts
│   ├── skills/               # Python-Skills
│   ├── tools/                # Erstellte Tools
│   └── workdir/              # Arbeitsverzeichnis
├── data/
│   ├── vault.dat             # Verschlüsselter Vault
│   ├── short_term.db         # SQLite STM
│   ├── long_term.db          # SQLite LTM
│   └── vectordb/             # ChromaDB
└── log/
    └── supervisor.log        # Haupt-Log
```

### Plattform-spezifische Pfade

| OS | Config | Daten | Logs |
|----|--------|-------|------|
| Linux | `~/.config/aurago/` | `~/.local/share/aurago/` | `~/.local/share/aurago/log/` |
| macOS | `~/Library/Application Support/aurago/` | `~/Library/Application Support/aurago/` | `~/Library/Logs/aurago/` |
| Windows | `%APPDATA%\aurago\` | `%LOCALAPPDATA%\aurago\` | `%LOCALAPPDATA%\aurago\log\` |

## Update-Historie (Changelog)

### v1.0.0 (Template)
- ✨ Initiale Version
- 🤖 Agent Core mit 50+ Tools
- 🧠 Memory System (STM, LTM, Knowledge Graph)
- 🎭 Personality Engine V1/V2
- 🔐 AES-256 Vault
- 💬 Web UI, Telegram, Discord
- 📊 Dashboard & Analytics

## Nützliche Ressourcen

### Offizielle Ressourcen
- GitHub Repository: github.com/antibyte/AuraGo
- Dokumentation: Ordner `documentation/`
- Issues: github.com/antibyte/AuraGo/issues

### LLM Provider
- OpenRouter: openrouter.ai
- OpenAI: platform.openai.com
- Ollama: ollama.com
- LM Studio: lmstudio.ai

### Community
- Discord: [Link im Repository]
- Reddit: r/AuraGo

### Tools & Hilfsmittel
- YAML Validator: yamllint.com
- Cron-Generator: crontab.guru
- bcrypt Generator: bcrypt-generator.com
- TOTP Apps: Google Authenticator, Authy, Aegis

---

## Lizenz

AuraGo wird bereitgestellt als Open Source Software für persönliche und Bildungszwecke.

Siehe LICENSE-Datei im Repository für Details.

---

> 💡 **Tipp:** Speichere dir diese Seite als Lesezeichen – sie ist die ultimative Referenz für alle technischen Details!

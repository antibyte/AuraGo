# Kapitel 7: Konfiguration

Dieses Kapitel erklärt die vollständige Konfiguration von AuraGo über die `config.yaml`-Datei.

---

## Übersicht

AuraGo wird über eine zentrale YAML-Konfigurationsdatei gesteuert: `config.yaml`.

> 💡 **Tipp:** Änderungen an der Konfiguration erfordern in der Regel einen Neustart von AuraGo.

---

## Die config.yaml-Struktur

```yaml
# Hauptabschnitte
providers:        # LLM-Provider (mehrere möglich)
llm:              # Haupt-LLM Konfiguration
embeddings:       # Embedding-Modell für RAG
agent:            # Agent-Verhalten
directories:      # Verzeichnispfade
sqlite:           # SQLite-Datenbanken
logging:          # Log-Konfiguration
server:           # Web-Server Einstellungen
circuit_breaker:  # Schutzmechanismen

# Integrationen
telegram:         # Telegram Bot
discord:          # Discord Bot
email:            # E-Mail (IMAP/SMTP)
home_assistant:   # Home Assistant
docker:           # Docker API
webhooks:         # Eingehende Webhooks
mcp:              # Model Context Protocol

# Weitere Features
budget:           # Kostenkontrolle
fallback_llm:     # Backup-LLM
co_agents:        # Parallele Sub-Agenten
maintenance:      # Nightly Maintenance
invasion_control: # Remote Deployment
indexing:         # Datei-Indexierung
sandbox:          # Isolierte Ausführung
notifications:    # Push-Benachrichtigungen

# Tool-Konfigurationen
tools:            # Tool-Berechtigungen
brave_search:     # Brave Search API
virustotal:       # VirusTotal API
koofr:            # Koofr Cloud
webdav:           # WebDAV-Zugriff
tts:              # Text-to-Speech
whisper:          # Speech-to-Text
vision:           # Bildanalyse
chromecast:       # Chromecast-Steuerung
mqtt:             # MQTT Broker
tailscale:        # Tailscale VPN
proxmox:          # Proxmox VE
meshcentral:      # MeshCentral
ansible:          # Ansible-Integration
ollama:           # Ollama-Verwaltung
rocketchat:       # Rocket.Chat
github:           # GitHub API
```

---

## Das Provider-System (Neu)

> 🆕 **Ab Version 2.x:** Das Provider-System ermöglicht die zentrale Verwaltung mehrerer LLM-Verbindungen.

### Provider definieren

```yaml
providers:
  - id: "main"                    # Eindeutige ID
    name: "Haupt-LLM"
    type: "openrouter"            # openai, openrouter, ollama, anthropic, google
    base_url: "https://openrouter.ai/api/v1"
    api_key: "sk-or-v1-DEIN-KEY"
    model: "arcee-ai/trinity-large-preview:free"

  - id: "vision"
    name: "Vision-Modell"
    type: "openrouter"
    base_url: "https://openrouter.ai/api/v1"
    api_key: "sk-or-v1-DEIN-KEY"
    model: "google/gemini-2.5-flash-lite-preview-09-2025"

  - id: "local-ollama"
    name: "Lokales Ollama"
    type: "ollama"
    base_url: "http://localhost:11434/v1"
    api_key: "dummy"              # Ollama braucht keinen Key
    model: "llama3.1:8b"
```

### Provider referenzieren

```yaml
llm:
  provider: "main"                # Referenz zu providers[].id
  use_native_functions: true
  temperature: 0.7
  structured_outputs: false

vision:
  provider: "vision"              # Eigenes Vision-Modell

whisper:
  provider: "main"                # Kann auch Haupt-LLM nutzen

embeddings:
  provider: "local-ollama"        # Dedizierter Embedding-Provider
  # oder: "disabled" / "internal"
```

### Unterstützte Provider-Typen

| Typ | Beschreibung |
|-----|--------------|
| `openrouter` | OpenRouter (empfohlen für Vielfalt) |
| `openai` | OpenAI API |
| `anthropic` | Anthropic Claude |
| `ollama` | Lokales Ollama |
| `google` | Google Gemini |

---

## Legacy-Konfiguration (noch unterstützt)

Die alte "inline" Konfiguration funktioniert weiterhin:

```yaml
llm:
  provider: openrouter
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-..."
  model: "arcee-ai/trinity-large-preview:free"
```

> ⚠️ **Empfohlene Migration:** Nutze das neue Provider-System für mehr Flexibilität.

---

## Server-Einstellungen

```yaml
server:
  host: "127.0.0.1"               # IP-Adresse (0.0.0.0 = alle Interfaces)
  port: 8088                      # Port-Nummer
  bridge_address: "localhost:8089" # Interne Bridge für Invasion
  max_body_bytes: 10485760        # Max. Upload-Größe (10 MB)
```

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `host` | `127.0.0.1` | Bind-Adresse |
| `port` | `8088` | HTTP-Port |
| `max_body_bytes` | `10485760` | Maximale Request-Größe |

> ⚠️ **Wichtig:** Für Docker muss `host` auf `0.0.0.0` gesetzt werden.

---

## Embeddings-Konfiguration

```yaml
embeddings:
  provider: "internal"            # "disabled", "internal", oder Provider-ID
  internal_model: "qwen/qwen3-embedding-8b"
  external_url: "http://localhost:11434/v1"
  external_model: "nomic-embed-text"
```

| Modus | Beschreibung |
|-------|--------------|
| `disabled` | Kein Langzeitgedächtnis |
| `internal` | Nutzt Haupt-LLM für Embeddings |
| Provider-ID | Verwendet dedizierten Provider |

---

## Agent-Verhalten

```yaml
agent:
  # Sprache und Persönlichkeit
  system_language: "German"
  core_personality: "friend"
  personality_engine: true
  personality_engine_v2: false
  
  # Speicher-Einstellungen
  context_window: 131000
  core_memory_max_entries: 200
  core_memory_cap_mode: "soft"
  memory_compression_char_limit: 50000
  
  # Verhalten
  max_tool_calls: 12
  step_delay_seconds: 0
  show_tool_results: false
  workflow_feedback: true
  debug_mode: false
  
  # Erweiterte Features
  enable_google_workspace: false
  system_prompt_token_budget: 8192
  user_profiling: false
```

### Danger Zone - Tool-Berechtigungen

```yaml
agent:
  allow_shell: true               # Shell-Befehle erlauben
  allow_python: true              # Python-Code ausführen
  allow_filesystem_write: true    # Dateien schreiben
  allow_network_requests: true    # HTTP-Requests
  allow_remote_shell: true        # SSH-Remote-Shell
  allow_self_update: true         # Auto-Update
  allow_mcp: true                 # MCP-Server
  allow_web_scraper: true         # Web-Scraping
  sudo_enabled: false             # Sudo-Befehle (Passwort im Vault)
```

---

## Tool-Konfiguration

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
    enabled: false
  web_scraper:
    enabled: true
    summary_mode: false
    summary_provider: ""
```

---

## Integrationen

### Telegram

```yaml
telegram:
  bot_token: "123456789:ABC..."   # Wird im Vault gespeichert
  telegram_user_id: 12345678
  max_concurrent_workers: 5
```

### Discord

```yaml
discord:
  enabled: true
  bot_token: "..."
  guild_id: "..."
  default_channel_id: "..."
  allowed_user_id: ""             # Optional: Beschränkung auf User
```

### E-Mail

```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  imap_port: 993
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  username: "user@gmail.com"
  from_address: "user@gmail.com"
  watch_enabled: true
  watch_interval_seconds: 120
  watch_folder: "INBOX"
```

### Home Assistant

```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  access_token: ""                # Wird im Vault gespeichert
```

### Docker

```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
```

---

## Erweiterte Features

### Budget-Tracking

```yaml
budget:
  enabled: true
  daily_limit_usd: 2.0
  enforcement: "warn"             # "warn", "partial", "full"
  warning_threshold: 0.8
  reset_hour: 0
  default_cost:
    input_per_million: 1.0
    output_per_million: 3.0
  models:
    - name: "gpt-4"
      input_per_million: 30.0
      output_per_million: 60.0
```

### Fallback-LLM

```yaml
fallback_llm:
  enabled: true
  provider: "fallback"            # Provider-ID
  probe_interval_seconds: 60
  error_threshold: 2
```

### Co-Agents

```yaml
co_agents:
  enabled: true
  max_concurrent: 3
  llm:
    provider: "coagent"           # Provider-ID (empfohlen: schnelles Modell)
  circuit_breaker:
    max_tool_calls: 12
    timeout_seconds: 300
    max_tokens: 0
```

### Circuit Breaker

```yaml
circuit_breaker:
  max_tool_calls: 20
  llm_timeout_seconds: 180
  maintenance_timeout_minutes: 10
  retry_intervals:
    - "10s"
    - "2m"
    - "10m"
```

---

## Sandbox (Sichere Ausführung)

```yaml
sandbox:
  enabled: true
  backend: docker
  docker_host: ""
  image: "python:3.11-slim"
  auto_install: true
  pool_size: 0
  timeout_seconds: 30
  network_enabled: false
  keep_alive: false
```

---

## Notifications

```yaml
notifications:
  ntfy:
    enabled: true
    url: "https://ntfy.sh"
    topic: "aurago-alerts"
  pushover:
    enabled: true
    # Token über Web-UI/Vault konfigurieren
```

---

## Konfiguration über Web UI vs. YAML

### Web UI (empfohlen für Anfänger)

1. Öffne die Web-Oberfläche
2. Klicke auf das Radial-Menü (≡)
3. Wähle "Config"
4. Bearbeite die Werte
5. Klicke "Save"

### Direkte YAML-Bearbeitung

```bash
nano config.yaml    # oder vim, code, notepad
```

**Vorteile:**
- Vollständige Kontrolle
- Kommentare möglich
- Schneller für fortgeschrittene Nutzer

---

## Umgebungsvariablen

Bestimmte Einstellungen können via Umgebungsvariablen überschrieben werden:

| Variable | Beschreibung |
|----------|--------------|
| `AURAGO_SERVER_HOST` | Server-Host |
| `AURAGO_MASTER_KEY` | Master-Key für Vault |
| `LLM_API_KEY` | API-Key für Haupt-LLM |
| `OPENAI_API_KEY` | Fallback für LLM_API_KEY |
| `ANTHROPIC_API_KEY` | Fallback für LLM_API_KEY |
| `EMBEDDINGS_API_KEY` | API-Key für Embeddings |
| `VISION_API_KEY` | API-Key für Vision |
| `WHISPER_API_KEY` | API-Key für Whisper/STT |

### Docker-Compose Beispiel

```yaml
services:
  aurago:
    environment:
      - AURAGO_SERVER_HOST=0.0.0.0
      - LLM_API_KEY=${LLM_API_KEY}
      - AURAGO_MASTER_KEY=${MASTER_KEY}
```

---

## Häufige Konfigurationsbeispiele

### Minimal-Konfiguration

```yaml
server:
  host: "127.0.0.1"
  port: 8088

providers:
  - id: "main"
    type: "openrouter"
    base_url: "https://openrouter.ai/api/v1"
    api_key: "DEIN-API-KEY"
    model: "arcee-ai/trinity-large-preview:free"

llm:
  provider: "main"
```

### Mit Telegram und Embeddings

```yaml
# ... Basis-Konfiguration ...

telegram:
  telegram_user_id: 12345678
  max_concurrent_workers: 5

embeddings:
  provider: "internal"
  internal_model: "qwen/qwen3-embedding-8b"
```

### Für Docker-Deployment

```yaml
server:
  host: "0.0.0.0"           # Wichtig für Docker!
  port: 8088

directories:
  data_dir: "./data"
  workspace_dir: "./agent_workspace"
```

---

## Konfigurationsvalidierung

AuraGo validiert die Konfiguration beim Start:

```
[INFO] Loading config from ./config.yaml
[INFO] Configuration validated successfully
[ERROR] Invalid config: llm.api_key is required
```

### Häufige Validierungsfehler

| Fehler | Lösung |
|--------|--------|
| `llm.api_key is required` | API-Key in Provider konfigurieren |
| `invalid yaml` | YAML-Syntax prüfen (Einrückungen!) |
| `invalid port number` | Port zwischen 1-65535 wählen |
| `directory not found` | Verzeichnispfad korrigieren |

---

## Zusammenfassung

| Abschnitt | Zweck |
|-----------|-------|
| `providers` | Zentrale LLM-Verwaltung |
| `llm` | Haupt-LLM Auswahl |
| `embeddings` | RAG/Langzeitgedächtnis |
| `agent` | Verhalten & Persönlichkeit |
| `tools.*` | Tool-Berechtigungen |
| `server` | Web-UI Einstellungen |
| `telegram/discord/email` | Integrationen |

> 💡 **Profi-Tipp:** Beginne mit der Minimal-Konfiguration und aktiviere Features nach Bedarf. Die Web-UI bietet eine sichere Methode, die Konfiguration zu erweitern.

---

**Vorheriges Kapitel:** [Kapitel 6: Werkzeuge](./06-tools.md)  
**Nächstes Kapitel:** [Kapitel 8: Integrationen](./08-integrations.md)

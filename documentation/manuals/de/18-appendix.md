# Kapitel 18: Anhang

<p align="center">
  <a href="../images/manual-hero.webp"><img src="../images/manual-hero.webp" width="400" alt="AuraGo-Gopher mit Handbuch"></a>
</p>

Kurzreferenz. Die lange Wahrheit liegt in `config_template.yaml` und [Kapitel 21](21-api-reference.md).

## Inhaltsverzeichnis

1. [Vollständige Konfigurationsreferenz](#vollständige-konfigurationsreferenz)
2. [API Endpoints](#api-endpoints)
3. [Chat-Befehle Referenz](#chat-befehle-referenz)
4. [Tool-Referenz](#tool-referenz)
5. [Beispiel-Konfigurationen](#beispiel-konfigurationen)
6. [Umgebungsvariablen](#umgebungsvariablen)
7. [Datei-Pfade](#datei-pfade)
8. [Update-Historie (Changelog)](#update-historie-changelog)
9. [Nützliche Ressourcen](#nützliche-ressourcen)
10. [Aktualisierte Kurzreferenz aktueller Endpunkte](#aktualisierte-kurzreferenz-aktueller-endpunkte)
11. [Aktualisierte Tool-Kurzreferenz](#aktualisierte-tool-kurzreferenz)
12. [Lizenz](#lizenz)

---

## Vollständige Konfigurationsreferenz

> [!TIP]
> **Web-UI zuerst:** Änderungen im Alltag über **Menü → Config** vornehmen. Dieser Anhang dokumentiert die vollständige `config.yaml`-Struktur als Referenz für Automatisierung und Headless-Deployments.

### config.yaml Struktur

```yaml
# ============================================
# AuraGo Configuration Reference
# ============================================

# ── LLM Providers ── API keys and models are defined here
providers: []                  # Configured via Setup Wizard or Config UI
  # - id: main
  #   type: openrouter
  #   name: "Haupt-LLM"
  #   base_url: https://openrouter.ai/api/v1
  #   api_key: "sk-or-..."
  #   model: "google/gemini-2.0-flash-001"

llm:
  provider: ""                 # References provider id from providers list
  multimodal: false
  helper_enabled: false        # Dedicated helper LLM for analysis/background tasks
  helper_provider: ""          # Helper provider id (empty = main provider)
  helper_model: ""             # Helper model override
  use_native_functions: true
  temperature: 0.7

server:
  host: "127.0.0.1"           # Bind address (0.0.0.0 for LAN)
  port: 8088                  # HTTP port
  max_body_bytes: 10485760    # Max upload size (10MB, Template-Default)

embeddings:
  provider: "local-granite"   # Standard bei Neuinstallationen; eigene Provider bleiben möglich
  local:
    backend: "auto"
    context_size: 2048
    batch_size: 2048
  external_url: "http://localhost:11434/v1"
  external_model: "nomic-embed-text"
  api_key: ""

agent:
  system_language: "English"
  max_tool_calls: 15
  step_delay_seconds: 0
  memory_compression_char_limit: 60000
  system_prompt_token_budget: 0   # 0 = automatic
  adaptive_system_prompt_token_budget: true
  context_window: 0               # 0 = auto-detect from provider API
  show_tool_results: false
  debug_mode: false
  core_memory_cap_mode: "hard"
  core_memory_max_entries: 80
  tool_output_limit: 50000
  workflow_feedback: true
  # Danger Zone (all disabled by default)
  sudo_enabled: false
  sudo_unrestricted: false
  allow_shell: false
  allow_python: false
  allow_filesystem_write: false
  allow_network_requests: false
  allow_remote_shell: false
  allow_self_update: false
  allow_mcp: false
  # Web-Scraping wird jetzt über `tools.web_scraper.enabled` gesteuert (deprecated)

  output_compression:
    enabled: true
    min_chars: 500
    preserve_errors: true
    shell_compression: true
    python_compression: true
    api_compression: true
    repetitive_substitution:
      enabled: false
      lzw_enabled: true
      ltsc_lite_enabled: false
      min_phrase_chars: 15
      min_occurrences: 3
      min_savings_percent: 15
      max_input_chars: 50000
      max_dictionary_entries: 16
    toon_json:
      enabled: false
      min_savings_percent: 10
      max_rows: 200

circuit_breaker:
  max_tool_calls: 20
  llm_timeout_seconds: 600
  llm_per_attempt_timeout_seconds: 120
  llm_stream_chunk_timeout_seconds: 30
  maintenance_timeout_minutes: 10
  retry_intervals: ["10s", "2m", "10m"]

fallback_llm:
  enabled: false
  base_url: ""
  api_key: ""
  model: ""
  error_threshold: 2
  probe_interval_seconds: 60

co_agents:
  enabled: false
  max_concurrent: 3
  llm:
    provider: ""
    base_url: ""
    api_key: ""
    model: ""
  circuit_breaker:
    max_tool_calls: 12
    timeout_seconds: 300
    max_tokens: 0              # 0 = unlimited
  budget_quota_percent: 25
  cleanup_interval_minutes: 10
  cleanup_max_age_minutes: 30
  max_context_hints: 6
  max_context_hint_chars: 180
  max_result_bytes: 100000

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

auth:
  enabled: true               # Login protection active by default
  password_hash: ""
  session_timeout_hours: 24
  totp_enabled: false
  totp_secret: ""
  max_login_attempts: 5
  lockout_minutes: 15

logging:
  log_dir: "./log"
  enable_file_log: true
  enable_prompt_log: false

maintenance:
  enabled: true
  time: "04:00"

telegram:
  bot_token: ""
  telegram_user_id: 0

discord:
  enabled: false
  readonly: false
  bot_token: ""
  guild_id: ""
  allowed_user_id: ""
  default_channel_id: ""

email:
  enabled: false
  readonly: false
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
  readonly: false
  url: ""
  access_token: ""

docker:
  enabled: false
  readonly: false
  host: ""

directories:
  data_dir: "./data"
  workspace_dir: "./agent_workspace/workdir"
  tools_dir: "./agent_workspace/tools"
  prompts_dir: "./prompts"
  skills_dir: "./agent_workspace/skills"
  vectordb_dir: "./data/vectordb"

sqlite:
  short_term_path: "./data/short_term.db"
  inventory_path: "./data/inventory.db"
  launchpad_path: "./data/launchpad.db"
  # LTM nutzt directories.vectordb_dir (nicht sqlite.long_term_path)

tools:
  memory:
    enabled: true
    readonly: false
    ondemand_retrieval:
      enabled: true
      max_essential_memories: 1
      max_available_memories: 6
      max_available_kg_nodes: 6
      max_available_chars: 1600
      dedupe_scope: turn
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

web_config:
  enabled: true
```

## API Endpoints

### REST API

| Methode | Endpoint | Beschreibung | Auth |
|---------|----------|--------------|------|
| GET | `/api/health` | Health-Check | Nein |
| GET | `/api/config` | Konfiguration abrufen (maskiert) | Ja* |
| POST | `/api/config` | Konfiguration aktualisieren | Ja* |
| GET | `/api/dashboard` | System-Kennzahlen | Ja* |
| GET | `/api/memory/short-term` | Kurzzeitgedächtnis | Ja* |
| GET | `/api/memory/long-term` | Langzeitgedächtnis | Ja* |
| POST | `/api/memory/query` | Erinnerungen abfragen | Ja* |
| GET | `/api/tools` | Verfügbare Tools auflisten | Ja* |
| GET | `/api/vault/status` | Vault-Status | Ja* |
| POST | `/api/vault/rotate` | Master-Key rotieren | Ja* |
| GET | `/api/dashboard/activity` | Dashboard-Aktivität (inkl. `coagents`) | Ja* |
| POST | `/api/webhook/:token` | Eingehender Webhook | Token |

\* Wenn Authentifizierung aktiviert ist

### WebSocket / SSE

| Endpoint | Zweck |
|----------|-------|
| `GET /events` | Server-Sent Events (Chat und Live-UI) |
| `GET /api/agodesk/ws` | AgoDesk-Desktopclient |
| `WS /api/invasion/ws` | Invasion-Eggs |

Es gibt keinen öffentlichen Chat-`/ws` und kein `/api/tools/execute`. Die vollständige UI ist ein Ressourcenpaket; `/` ohne Pin ist die Reparaturseite. Vollständige Liste: [Kapitel 21](21-api-reference.md).

### Statische Dateien

| Endpoint | Beschreibung |
|----------|--------------|
| `/` | Chat oder Reparaturseite, je nach UI-Paket |
| `/desktop`, `/config`, `/dashboard` | Mehrseitige Oberfläche |
| `/attachments/*` | Hochgeladene Dateien der Nutzer |

## Chat-Befehle Referenz

### Befehlsübersicht

| Befehl | Beschreibung | Beispiel |
|--------|--------------|----------|
| `/help` | Alle Befehle anzeigen | `/help` |
| `/reset` | Chat zurücksetzen | `/reset` |
| `/stop` | Aktuelle Aktion stoppen | `/stop` |
| `/restart` | Agent neu starten | `/restart` |
| `/debug on` | Debug-Modus ein | `/debug on` |
| `/debug off` | Debug-Modus aus | `/debug off` |
| `/personality <name>` | Persönlichkeit wechseln | `/personality professional` |
| `/budget [en]` | Kosten anzeigen (optional: Englisch) | `/budget` |
| `/voice on` | Sprachausgabe ein | `/voice on` |
| `/voice off` | Sprachausgabe aus | `/voice off` |
| `/warnings` | Warnungen anzeigen | `/warnings` |
| `/sudopwd <pw>` | Sudo-Passwort setzen | `/sudopwd meinpass` |
| `/addssh` | SSH-Gerät hinzufügen | `/addssh` |
| `/credits` | OpenRouter Credits | `/credits` |

### Ausführliche Befehlsreferenz

#### `/reset`
```text
Verwendung: /reset

Löscht den Gesprächsverlauf der aktuellen Sitzung aus dem
Kurzzeitgedächtnis. Betrifft NICHT:
- Langzeit-Erinnerungen
- Knowledge Graph
- Notizen und To-Dos
- Core Memory

Sinnvoll wenn:
- Der Kontext zu lang wird
- Der Agent verwirrt wirkt
- Ein neues Thema beginnt
```

#### `/stop`
```text
Verwendung: /stop

Unterbricht die aktuelle Agenten-Aktion sofort. Sinnvoll wenn:
- Ein Tool-Aufruf zu lange dauert
- Der Agent in einer Endlosschleife steckt
- Du eine laufende Aufgabe abbrechen willst

Hinweis: Bereits abgeschlossene Aktionen werden nicht rückgängig gemacht.
```

#### `/restart`
```text
Verwendung: /restart

Startet den AuraGo-Server kontrolliert neu. Alle Verbindungen werden
getrennt. Der Agent:
1. Schließt die aktuelle Operation ab
2. Speichert den Zustand
3. Beendet sich mit Code 42
4. Startet neu (bei systemd/Service-Betrieb automatisch)
```

#### `/debug`
```text
Verwendung: /debug [on|off]

Steuert die Ausführlichkeit der Debug-Ausgabe.

Argumente:
  on        - Detaillierte Fehlermeldungen einschalten
  off       - Detaillierte Ausgabe ausschalten
  (keins)   - Aktuellen Zustand umschalten

Wenn aktiv:
- Vollständige Tool-Ausgaben
- Stacktraces bei Fehlern
- Ausführliches Logging
```

#### `/personality`
```text
Verwendung: /personality [name]

Ohne Argument: Listet verfügbare Persönlichkeiten
Mit Argument:  Wechselt zur angegebenen Persönlichkeit

Verfügbare Persönlichkeiten:
  - friend        (locker, warm)
  - professional  (förmlich, effizient)
  - neutral       (ausgewogen)
  - punk          (aufmüpfig)
  - terminator    (direkt, knapp)
  - mcp           (protokollorientiert)

Beispiel:
Du: /personality professional
Agent: 🎭 Persönlichkeit zu professional gewechselt
```

#### `/budget`
```text
Verwendung: /budget

Zeigt die API-Nutzung des heutigen Tages:
- Eingabe-Tokens
- Ausgabe-Tokens
- Geschätzte Kosten in USD
- Status des Tageslimits
- Zeitpunkt der Zurücksetzung

Nur verfügbar, wenn budget.enabled true ist.
```

## Tool-Referenz

### Tool-Kategorien

| Kategorie | Tools | Beschreibung |
|-----------|-------|--------------|
| System | execute_shell, execute_python, get_system_metrics | Grundlegende Systemoperationen |
| Dateisystem | read_file, write_file, list_directory | Dateiverwaltung |
| Gedächtnis | manage_memory, query_memory | Gedächtnisoperationen |
| Wissen | knowledge_graph_query | Knowledge Graph |
| Web | web_search, web_scraper, fetch_url | Internetzugriff |
| Kommunikation | send_email, telegram_send | Nachrichten |
| Smart Home | home_assistant_* | Home Assistant |
| Infrastruktur | docker_*, proxmox_*, ollama_* | Infrastruktur-Verwaltung |
| Speicher | webdav_*, koofr_* | Cloud-Speicher |
| Planung | cron_scheduler, follow_up | Aufgabenplanung |
| Entwicklung | git_*, github_* | Entwicklerwerkzeuge |
| Hilfswerkzeuge | analyze_image, transcribe_audio | Sonstige |

### Häufige Tool-Parameter

#### Dateioperationen
```yaml
read_file:
  path: string        # Dateipfad (erforderlich)
  offset: int         # Ab Zeile lesen (optional)
  limit: int          # Maximale Zeilenzahl (optional)

write_file:
  path: string        # Dateipfad (erforderlich)
  content: string     # Zu schreibender Inhalt (erforderlich)
  append: boolean     # Anhängen statt überschreiben (optional)
```

#### Shell-Ausführung
```yaml
execute_shell:
  command: string     # Shell-Befehl (erforderlich)
  timeout: int        # Zeitlimit in Sekunden (optional, Standard: 60)
  background: boolean # Im Hintergrund ausführen (optional)
  workdir: string     # Arbeitsverzeichnis (optional)
```

#### Python-Ausführung
```yaml
execute_python:
  code: string        # Python-Code (erforderlich)
  timeout: int        # Zeitlimit in Sekunden (optional, Standard: 120)
  packages: []string  # Zu installierende pip-Pakete (optional)
```

#### Websuche
```yaml
web_search:
  query: string       # Suchanfrage (erforderlich)
  num_results: int    # Anzahl Ergebnisse (optional, Standard: 5)
  source: string      # Suchmaschine (optional: duckduckgo, google)

web_scraper:
  url: string         # Zu lesende URL (erforderlich)
  selector: string    # CSS-Selektor (optional)
  extract_text: boolean  # Nur Text extrahieren (optional)
```

#### Gedächtnisoperationen
```yaml
manage_memory:
  operation: string   # add, update, delete, read (erforderlich)
  key: string         # Gedächtnis-Schlüssel (erforderlich für update/delete)
  content: string     # Inhalt (erforderlich für add/update)
  category: string    # Kategorie (optional)
  priority: int       # Priorität 1-10 (optional)

query_memory:
  query: string       # Suchanfrage (erforderlich)
  limit: int          # Maximale Treffer (optional, Standard: 5)
  threshold: float    # Ähnlichkeitsschwelle (optional)

recall_memory:
  ids: [string]       # Gedächtnis-IDs aus dem AVAILABLE CONTEXT INDEX

explore_kg:
  ids: [string]       # KG-Knoten-IDs aus dem AVAILABLE CONTEXT INDEX
  depth: int          # Traversierungstiefe 1-3 (optional)
  limit: int          # Max. Knoten/Kanten pro ID (optional)
```

#### E-Mail
```yaml
send_email:
  to: string          # Empfänger (erforderlich)
  subject: string     # Betreff (erforderlich)
  body: string        # Nachrichtentext (erforderlich)
  attachments: []string  # Dateipfade (optional)
```

#### Docker
```yaml
docker_list:
  type: string        # containers, images, volumes, networks
  all: boolean        # Auch gestoppte Container

docker_start:
  name: string        # Container-Name oder -ID (erforderlich)

docker_stop:
  name: string        # Container-Name oder -ID (erforderlich)
  timeout: int        # Stopp-Zeitlimit (optional)

docker_logs:
  name: string        # Container-Name oder -ID (erforderlich)
  lines: int          # Zeilenzahl (optional)
  follow: boolean     # Logs streamen (optional)
```

#### Home Assistant
```yaml
home_assistant_get_state:
  entity_id: string   # Entity-ID (erforderlich)

home_assistant_call_service:
  domain: string      # Service-Domäne (erforderlich)
  service: string     # Service-Name (erforderlich)
  entity_id: string   # Ziel-Entity (optional)
  data: object        # Service-Daten (optional)

home_assistant_toggle:
  entity_id: string   # Umzuschaltende Entity (erforderlich)
```

#### Planung
```yaml
cron_scheduler:
  operation: string   # add, list, remove (erforderlich)
  name: string        # Job-Name (erforderlich für add/remove)
  schedule: string    # Cron-Ausdruck (erforderlich für add)
  command: string     # Auszuführender Befehl (erforderlich für add)
  enabled: boolean    # Aktivieren/Deaktivieren (optional)

follow_up:
  task: string        # Aufgabenbeschreibung (erforderlich)
  delay: string       # Verzögerung (z. B. "1h", "30m") (erforderlich)
```

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
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-..."
    model: "google/gemini-2.0-flash-001"

llm:
  provider: main
```

Alles andere nutzt Defaults.

### Entwicklung

```yaml
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-..."
    model: "mistralai/mixtral-8x7b-instruct"

llm:
  provider: main

server:
  host: "127.0.0.1"
  port: 8088

agent:
  debug_mode: true
  show_tool_results: true

logging:
  enable_file_log: true
```

### Produktion mit Auth

```yaml
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-..."
    model: "google/gemini-2.0-flash-001"

llm:
  provider: main

server:
  host: "127.0.0.1"
  port: 8088

auth:
  enabled: true
  password_hash: "$2a$10$..."  # bcrypt
  session_timeout_hours: 8
  totp_enabled: true
```

### Mit Telegram

```yaml
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-..."
    model: "google/gemini-2.0-flash-001"

llm:
  provider: main

telegram:
  bot_token: "123456:ABC-DEF..."
  telegram_user_id: 123456789
```

### Mit Home Assistant

```yaml
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-..."
    model: "google/gemini-2.0-flash-001"

llm:
  provider: main

home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  access_token: "eyJ0eXAiOiJKV1Q..."
```

### Lokales Modell (Ollama)

```yaml
providers:
  - id: local
    type: ollama
    base_url: http://localhost:11434/v1
    model: "llama3.1:8b"

llm:
  provider: local

embeddings:
  provider: "external"
  external_url: "http://localhost:11434/v1"
  external_model: "nomic-embed-text"
```

## Umgebungsvariablen

### Erforderliche Variablen

| Variable | Beschreibung | Format |
|----------|--------------|--------|
| `AURAGO_MASTER_KEY` | Vault-Verschlüsselungsschlüssel | 64 Zeichen Hex |

### Optionale Variablen

| Variable | Beschreibung | Standard |
|----------|--------------|----------|
| `AURAGO_CONFIG_PATH` | Alternativer Config-Pfad | `./config.yaml` |
| `AURAGO_LOG_LEVEL` | Logging-Level | `info` |
| `AURAGO_DATA_DIR` | Datenverzeichnis | `./data` |
| `LLM_API_KEY` | LLM-API-Key | Aus der Config |
| `EMBEDDINGS_API_KEY` | Embeddings-API-Key | Aus der Config |

### Master-Key generieren

```bash
# Linux/macOS
export AURAGO_MASTER_KEY=$(openssl rand -hex 32)

# Windows PowerShell
$env:AURAGO_MASTER_KEY = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })

# Länge prüfen (sollte 64 Zeichen sein)
echo ${#AURAGO_MASTER_KEY}        # Linux/macOS
$env:AURAGO_MASTER_KEY.Length     # PowerShell
```

### Docker-Umgebung

```bash
# .env-Datei anlegen
cat > .env << EOF
AURAGO_MASTER_KEY=$(openssl rand -hex 32)
EOF

# Oder direkt exportieren
export AURAGO_MASTER_KEY="dein-64-Zeichen-Hex-Key"
docker compose up -d
```

## Datei-Pfade

### Standard-Struktur

```text
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
│   ├── vault.bin             # Verschlüsselter Vault (AES-256-GCM)
│   ├── short_term.db         # SQLite STM
│   ├── system_tasks.db       # Cron + Hintergrundtasks (SQLite)
│   └── vectordb/             # Langzeit-Speicher (chromem)
└── log/
    ├── aurago.log            # Anwendung
    └── web_access.log        # HTTP-Zugriff
```

### Plattform-spezifische Pfade

| OS | Config | Daten | Logs |
|----|--------|-------|------|
| Linux | `~/.config/aurago/` | `~/.local/share/aurago/` | `~/.local/share/aurago/log/` |
| macOS | `~/Library/Application Support/aurago/` | `~/Library/Application Support/aurago/` | `~/Library/Logs/aurago/` |
| Windows | `%APPDATA%\aurago\` | `%LOCALAPPDATA%\aurago\` | `%LOCALAPPDATA%\aurago\log\` |

### Konfigurationsdatei-Orte

| Datei | Zweck | Suchreihenfolge |
|-------|-------|-----------------|
| `config.yaml` | Hauptkonfiguration | 1. Arbeitsverzeichnis 2. `~/.aurago/` 3. `/etc/aurago/` |
| `.env` | Umgebungsvariablen | 1. Arbeitsverzeichnis 2. `~/.aurago/` |
| `prompts/` | System-Prompts | Config: `directories.prompts_dir` |

### Logs

Die Anwendungsausgabe läuft in `log/aurago.log`, HTTP-Zugriffe in `log/web_access.log`. Es gibt kein Rotationsschema über eine `supervisor.log`.

## Update-Historie (Changelog)

Versionsnummern und Feature-Listen gehören in GitHub Releases. Dieses Handbuch beschreibt den Stand vom **3. Oktober 2026**.

### Update-Checkliste

Beim Aktualisieren von AuraGo:

```text
□ Release Notes auf Breaking Changes prüfen
□ Verzeichnis data/ sichern
□ Aktuelle Konfiguration sichern: cp config.yaml config.yaml.backup
□ Neues Binary herunterladen
□ config.yaml mit den neuen Standardwerten vergleichen
□ Dienst neu starten
□ Funktion mit dem Befehl /help prüfen
□ Logs auf Fehler prüfen
□ Remote Control: neue Enrollment-Tokens erstellen und vor dem Update installierte Remote-Agenten ersetzen, wenn die Release Notes das verlangen
```

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

### Verwandte Projekte
- chromem-go: github.com/philippgille/chromem-go — Vektordatenbank
- go-openai: github.com/sashabaranov/go-openai — OpenAI-Client
- telegram-bot-api: github.com/go-telegram-bot-api — Telegram-Integration
- discordgo: github.com/bwmarrin/discordgo — Discord-Integration

### Lernressourcen
- YAML: yaml.org/spec
- Markdown: markdownguide.org
- Go: go.dev/doc
- SQLite: sqlite.org/docs
- Docker: docs.docker.com

### Tools & Hilfsmittel
- YAML Validator: yamllint.com
- Cron-Generator: crontab.guru
- bcrypt Generator: bcrypt-generator.com
- TOTP Apps: Google Authenticator, Authy, Aegis

---

## Aktualisierte Kurzreferenz aktueller Endpunkte

| Feature | Endpunkte |
|---------|-----------|
| Security Proxy | `GET /api/proxy/status`, `POST /api/proxy/start`, `POST /api/proxy/stop`, `POST /api/proxy/reload`, `GET /api/proxy/logs` |
| Web Push | `GET /api/push/vapid-pubkey`, `POST /api/push/subscribe`, `POST /api/push/unsubscribe`, `GET /api/push/status` |
| Video Generation | `POST /api/video-generation/test` |
| Managed Ollama | `GET /api/ollama/managed/status`, `POST /api/ollama/managed/recreate` |
| File KG Sync | `GET /api/debug/file-sync-status`, `GET /api/debug/kg-file-sync-stats`, `POST /api/debug/kg-file-sync-cleanup` |
| A2A | `GET /api/a2a/status`, `GET /api/a2a/card`, `GET /api/a2a/remote-agents`, `POST /api/a2a/test` |
| Backup/Restore | `POST /api/backup/create`, `POST /api/backup/import` |
| go2rtc | `GET /api/go2rtc/status`, `POST /api/go2rtc/test` |
| MeshCore | `GET /api/meshcore/status`, `POST /api/meshcore/test` |
| Bluetooth | `GET /api/bluetooth/status`, `POST /api/bluetooth/reprobe` |
| CYD | `GET /api/cyd/status`, `POST /api/cyd/test` |

## Aktualisierte Tool-Kurzreferenz

| Tool | Zweck |
|------|-------|
| `generate_video` | Kurze KI-Videos aus Text- oder Bildvorgaben erstellen |
| `send_video` | Videodateien mit Inline-Player an den Benutzer senden |
| `ldap` | LDAP/Active Directory durchsuchen und authentifizieren |
| `wait_for_event` | Hintergrundprozess, HTTP-Endpunkt oder Dateiänderung asynchron abwarten |
| `follow_up` | Autonome Folgeaufgabe nach der aktuellen Antwort planen |

---

## Lizenz

### AuraGo-Lizenz

Dieses Projekt wird wie besehen für den persönlichen und Bildungseinsatz bereitgestellt.

```text
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

### Drittanbieter-Lizenzen

AuraGo verwendet die folgenden Open-Source-Komponenten:

| Bibliothek | Lizenz | Zweck |
|------------|--------|-------|
| go-openai | Apache-2.0 | OpenAI-API-Client |
| chromem-go | MPL-2.0 | Vektordatenbank |
| diago | MPL-2.0 | Nativer SIP-Dialog und Medienverarbeitung |
| sipgo | BSD-2-Clause | SIP-Protokollstack |
| go-mp3 | Apache-2.0 | Reines Go-Telefonie-TTS-Audio-Decoding |
| modernc.org/sqlite | BSD-3 | SQLite-Treiber |
| telegram-bot-api | MIT | Telegram-Bot |
| discordgo | BSD-3 | Discord-Bot |
| gopsutil | BSD-3 | Systemmetriken |
| golang.org/x/crypto | BSD-3 | Kryptografie |
| cron/v3 | MIT | Aufgabenplanung |
| vishen/go-chromecast | Apache-2.0 | Chromecast-Steuerung |
| yaml.v3 | MIT AND Apache-2.0 | YAML-Parsing |

Das kompakte Fremdprojekt-Inventar und die Lizenztexte, die bei einer Weiterverbreitung mitgegeben werden müssen, liegen in `licences/UEBERSICHT.md`. Die Upstream-Repositories bleiben die Quelle für einen gepinnten Image-Digest.

### Attribution

Wenn du AuraGo in Projekten oder Publikationen verwendest, denke bitte daran anzugeben:
- Das AuraGo-Projekt und seine Beitragenden
- Die zugrunde liegenden LLM-Provider
- Die verwendeten Open-Source-Bibliotheken

---

> [!TIP]
> Speichere dir diese Seite als Lesezeichen – sie ist die ultimative Referenz für alle technischen Details!

---

**Vorheriges Kapitel:** [Kapitel 17: Glossar](./17-glossary.md)
**Nächstes Kapitel:** [Kapitel 19: Skills](./19-skills.md)

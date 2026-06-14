# Kapitel 7: Konfiguration

Dieses Kapitel erklärt die Konfiguration von AuraGo. Der bevorzugte Weg ist die **Web-UI** – YAML-Änderungen sind nur noch für spezielle Szenarien oder Docker-Deployments nötig.

---

## Übersicht

AuraGo wird über eine zentrale YAML-Konfigurationsdatei gesteuert: `config.yaml`.

> 💡 **Tipp:** Die meisten Einstellungen lassen sich heute komfortabel über die Web-UI vornehmen. Änderungen an `config.yaml` erfordern in der Regel einen Neustart von AuraGo.

---

## Konfiguration über Web UI vs. YAML

### Web UI (empfohlen)

Die einfachste und sicherste Methode, AuraGo zu konfigurieren:

1. **Öffne die Web-Oberfläche** unter `http://localhost:8088` (bzw. der in `server` konfigurierten Adresse)
2. **Klicke auf das Radial-Menü** (≡) oben links
3. **Wähle "Config"**
4. **Navigiere links durch die Kategorien:**
   - **Provider** – LLM-Verbindungen anlegen und testen
   - **Agent** – Sprache, Verhalten, Danger-Zone-Toggles
   - **Integrations** – Telegram, Discord, Home Assistant, Docker, etc.
   - **Tools** – Einzelne Tools aktivieren/deaktivieren
   - **Regeln** – aufgabenbezogene Agent-Leitplanken bearbeiten
   - **Server** – Host, Port, HTTPS
   - **Memory / Tasks / Sandbox** – weitere Systemeinstellungen
5. **Aktiviere Toggles, fülle Felder aus** und klicke unten auf **"Save"**
6. Einige Änderungen (z. B. Server-Port, Provider-Wechsel, Danger-Zone-Berechtigungen) erfordern einen **Neustart** – die UI zeigt dies entsprechend an

> 💡 **Tipp:** Sensible Werte wie API-Keys oder Passwörter werden automatisch im Vault gespeichert, wenn sie über die Web-UI eingegeben werden.

### Config-UX (Sidebar, Save-Leiste, ungespeicherte Änderungen)

| Feature | Verhalten |
|---------|----------|
| **Sidebar-Suche** | Bereiche filtern; Tastatur mit Pfeiltasten und Enter |
| **Fixe Save-Leiste** | Speichern und Status bleiben beim Scrollen sichtbar |
| **Ungespeicherte Änderungen** | Wechsel der Sidebar oder Browser Zurück/Vor → Bestätigungsdialog |
| **Save-Status** | Fortschritt für Screenreader (`role="status"`) |
| **Schalter** | Switch-Semantik und Tastaturbedienung |

> Tipp: Hash-URLs wie `/config#server` springen direkt in einen Bereich.


### Direkte YAML-Bearbeitung

Fortgeschrittene Nutzer oder Docker-Deployments können `config.yaml` direkt bearbeiten:

```bash
nano config.yaml    # oder vim, code, notepad
```

**Vorteile:**
- Vollständige Kontrolle
- Kommentare möglich
- Schneller für Bulk-Änderungen oder Git-Ops-Workflows

---

## Die config.yaml-Struktur

```yaml
# Hauptabschnitte
providers:        # LLM-Provider (mehrere möglich)
llm:              # Haupt-LLM Konfiguration
embeddings:       # Embedding-Modell für RAG
agent:            # Agent-Verhalten
server:           # Web-Server Einstellungen
tools:            # Tool-Berechtigungen
rules:            # Aufgabenbezogene Agent-Leitplanken

# Integrationen (Auswahl)
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
personality:      # Persönlichkeit & Stimmung
a2a:              # Agent-to-Agent Protokoll
music_generation: # KI-Musikgenerierung
security_proxy:   # Reverse Proxy & Schutz
egg_mode:         # Cluster-Worker-Mode
firewall:         # Firewall-Integration
journal:          # Auto-Journaleinträge
```

---

## Aufgabenbezogene Regeln

Rules sind Markdown-Leitplanken, die AuraGo vor passenden Workflows oder Tool-Aufrufen lädt. Sie liegen unter `prompts/rules/<id>/rule.md`, werden in der Web-UI unter **Config → Regeln** bearbeitet und über Toolnamen, Workflow-Tags oder Keywords gematcht. Eingebaute Regeln sind im Binary enthalten, Dateien auf Disk überschreiben sie, und die UI kann die eingebaute Version wiederherstellen.

```yaml
rules:
  enabled: true
```

Homepage-Workflows erhalten zusätzlich das globale `prompts/rules/homepage/DESIGN.md`. Wenn im Homepage-Projektroot eine eigene `DESIGN.md` liegt, wird sie nur als Designsystem-Kontext angehängt; sie überschreibt keine Security-Policy und keine Tool-Berechtigungen.

---

## Das Provider-System

> 🆕 **Ab Version 2.x:** Das Provider-System ermöglicht die zentrale Verwaltung mehrerer LLM-Verbindungen.

In der **Web-UI** findest du unter *Config → Provider* eine Liste aller konfigurierten Verbindungen. Über "Add Provider" kannst du neue Endpunkte hinzufügen und direkt mit "Test" die Erreichbarkeit prüfen. Die aktive Verbindung für `llm`, `vision`, `whisper` oder `embeddings` lässt sich per Dropdown zuweisen.

### YAML-Beispiel

```yaml
providers:
  - id: "main"
    name: "Haupt-LLM"
    type: "openrouter"
    base_url: "https://openrouter.ai/api/v1"
    api_key: "sk-or-v1-DEIN-KEY"
    model: "arcee-ai/trinity-large-preview:free"

llm:
  provider: "main"
  use_native_functions: true
  temperature: 0.7

embeddings:
  provider: "local-ollama"
```

| Typ | Beschreibung |
|-----|--------------|
| `openrouter` | OpenRouter (empfohlen für Vielfalt) |
| `openai` | OpenAI API |
| `anthropic` | Anthropic Claude |
| `ollama` | Lokales Ollama |
| `google` | Google Gemini |

> ⚠️ **Empfohlene Migration:** Nutze das neue Provider-System für mehr Flexibilität. Die alte "inline" Konfiguration funktioniert zwar weiterhin, ist aber nicht mehr empfohlen.

---

## Server-Einstellungen

Unter *Config → Server* stellst du Host, Port und HTTPS-Optionen ein. Für Docker muss `host` auf `0.0.0.0` gesetzt werden.

```yaml
server:
  host: "127.0.0.1"
  port: 8088
  max_body_bytes: 10485760
  bridge_address: ""
  https:
    enabled: false
    cert_mode: auto          # "auto" (Let's Encrypt), "custom", "selfsigned"
    domain: ""
    email: ""
    https_port: 443
    http_port: 80
    behind_proxy: false
```

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `host` | `127.0.0.1` | Bind-Adresse |
| `port` | `8088` | HTTP-Port |
| `max_body_bytes` | `10485760` | Maximale Request-Größe |
| `bridge_address` | `""` | Bridge-Adresse für Telegram/Discord |
| `https.enabled` | `false` | HTTPS aktivieren |
| `https.cert_mode` | `auto` | Zertifikatsmodus: `auto`, `custom`, `selfsigned` |

---

## Agent-Verhalten

In der **Web-UI** unter *Config → Agent* lassen sich Sprache, Kontextfenster, Tool-Limit und die **Danger Zone** komfortabel einstellen. Änderungen an den Danger-Zone-Toggles werden nach dem Speichern meist erst nach einem Neustart wirksam.

```yaml
agent:
  system_language: "English"
  context_window: 0              # 0 = automatisch vom Provider ermitteln
  max_tool_calls: 15
  debug_mode: false
  memory_compression_char_limit: 60000
  tool_output_limit: 50000
  discover_tools_snapshot_ttl_minutes: 5
  system_prompt_token_budget: 0  # 0 = automatisch
  adaptive_system_prompt_token_budget: true
  workflow_feedback: true
  
  # Danger Zone - bei Neuinstallationen standardmäßig deaktiviert
  allow_shell: false
  allow_python: false
  allow_filesystem_write: false
  allow_network_requests: false
  allow_remote_shell: false
  allow_self_update: false
  allow_mcp: false
  allow_web_scraper: false
  sudo_enabled: false
```

### Danger Zone

Diese Berechtigungen schalten potenziell kritische Aktionen frei:

- `allow_shell` – Shell-Befehle erlauben
- `allow_python` – Python-Code ausführen
- `allow_filesystem_write` – Dateien schreiben
- `allow_network_requests` – HTTP-Requests
- `allow_remote_shell` – SSH-Remote-Shell
- `allow_self_update` – Auto-Update
- `allow_mcp` – MCP-Server
- `tools.web_scraper.enabled` – Web-Scraping (ersetzt `allow_web_scraper`)
- `sudo_enabled` – Sudo-Befehle (Passwort im Vault)

### Output Compression

Kürzt übergroße Tool-Ausgaben zuerst über `tool_output_limit` und komprimiert danach den behaltenen Inhalt. Standard: aktiv.

### Einrichtung in der Web-UI
1. Öffne **Config → Agent → Output-Kompression**.
2. Aktiviere die Kompression und passe Schwellenwerte sowie Shell-/Python-/API-Filter an.
3. Speichern.

### YAML-Referenz
```yaml
agent:
  output_compression:
    enabled: true
    min_chars: 500
    preserve_errors: true
    shell_compression: true
    python_compression: true
    api_compression: true
    repetitive_substitution:
      enabled: false
    toon_json:
      enabled: false
```

Details: `documentation/output_compression.md`

---

## Tool-Konfiguration

Unter *Config → Tools* kannst du einzelne Tools aktivieren, deaktivieren oder auf Nur-Lesen stellen. Besonders nützlich für `memory`, `knowledge_graph`, `secrets_vault`, `web_scraper` oder `document_creator`.

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
  journal:
    enabled: true
    readonly: false
  daemon_skills:
    enabled: true
    max_concurrent_daemons: 3
    global_rate_limit_secs: 60
    max_wakeups_per_hour: 10
    max_budget_per_hour_usd: 1.0
  web_scraper:
    enabled: true
    summary_mode: false
    summary_provider: ""
  document_creator:
    enabled: false
    backend: maroto
    output_dir: data/documents
  python_tool_bridge:
    enabled: false
    allowed_tools: []
    allowed_sql_connections: []
  python_timeout_seconds: 30
  skill_timeout_seconds: 120
  background_timeout_seconds: 3600
```

---

## Skill Manager

Der Skill Manager ist in der **Web-UI** unter *Config → Tools → Fähigkeiten-Manager* erreichbar. Dort kannst du Python-Skills hochladen, aktivieren, deaktivieren und über den integrierten Sicherheits-Scan prüfen lassen.

```yaml
tools:
  skill_manager:
    enabled: true
    allow_uploads: true
    readonly: false
    require_scan: true
    max_upload_size_mb: 1
    auto_enable_clean: false
    scan_with_guardian: false
```

- Alle hochgeladenen Skills werden auf verdächtige Muster gescannt
- **Python Secret Injection** unter **Config → Tools → Tool-Berechtigungen** muss aktiv sein, damit Skills Vault-Secrets nutzen können (YAML: `tools.python_secret_injection.enabled`)
- Der Guardian kann optional Code-Reviews durchführen

---

## Media Registry

Die Media Registry wird primär über **`media_registry.enabled`** in der `config.yaml` aktiviert (kein eigener Config-Menüpunkt). Verwaltung generierter Medien erfolgt über die Seite **Galerie** (`/gallery`). Sie kümmert sich um das Katalogisieren von Bildern, Videos und Audiodateien inklusive EXIF-Metadaten und Duplikat-Erkennung.

```yaml
media_registry:
  enabled: true
```

---

## Background Tasks

Hintergrund-Verarbeitung für Follow-ups, Cron-Jobs und Wait-Events. In der **Web-UI** unter *Config → Agent → Optimierungen* (Gruppe **Hintergrundverarbeitung**) lassen sich Timeout- und Retry-Werte anpassen.

```yaml
agent:
  background_tasks:
    enabled: true
    follow_up_delay_seconds: 2
    http_timeout_seconds: 120
    max_retries: 2
    retry_delay_seconds: 60
    wait_poll_interval_seconds: 5
    wait_default_timeout_secs: 600
```

**Anwendungsfälle:**
- **Follow-ups**: "Erinnere mich morgen daran..."
- **Cron-Jobs**: Regelmäßige Aufgaben planen
- **Wait-Events**: Auf externe Ereignisse warten

---

## Personality – Persönlichkeit

Unter *Config → Personality* lässt sich das Verhalten und die Stimmung von AuraGo anpassen.

```yaml
personality:
  core_personality: friend
  engine: true
  engine_v2: true
  user_profiling: false
  emotion_synthesizer:
    enabled: false
    min_interval_seconds: 60
    max_history_entries: 100
    trigger_on_mood_change: true
    trigger_always: false
  inner_voice:
    enabled: false
    min_interval_secs: 60
    max_per_session: 20
```

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `core_personality` | `friend` | Basisprofil: `friend`, `professional`, `punk`, `neutral`, `terminator` |
| `engine` | `true` | Personality Engine aktivieren |
| `engine_v2` | `true` | LLM-basierte Stimmungsanalyse |
| `user_profiling` | `false` | Präferenzen aus Gesprächen lernen |
| `emotion_synthesizer.enabled` | `false` | Emotionssynthese für Antworten |
| `inner_voice.enabled` | `false` | Unterbewusste Verhaltensanpassung |

---

## Co-Agents – Parallele Sub-Agenten

Aktiviere spezialisierte Co-Agents für komplexe Aufgaben unter *Config → Co-Agents*.

```yaml
co_agents:
  enabled: false
  max_concurrent: 3
  budget_quota_percent: 25
  max_context_hints: 6
  max_context_hint_chars: 180
  max_result_bytes: 100000
  queue_when_busy: true
  llm:
    provider: ""
  circuit_breaker:
    max_tool_calls: 12
    timeout_seconds: 300
    max_tokens: 0
  retry_policy:
    max_retries: 1
    retry_delay_seconds: 5
  specialists:
    researcher:
      enabled: false
      additional_prompt: ""
    coder:
      enabled: false
      additional_prompt: ""
    designer:
      enabled: false
      additional_prompt: ""
    security:
      enabled: false
      additional_prompt: ""
    writer:
      enabled: false
      additional_prompt: ""
```

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `enabled` | `false` | Co-Agents aktivieren |
| `max_concurrent` | `3` | Maximale parallele Co-Agents |
| `budget_quota_percent` | `25` | Tagesbudget-Reserve (`0` = deaktiviert) |
| `queue_when_busy` | `true` | Warteschlange bei voller Auslastung |
| `circuit_breaker.timeout_seconds` | `300` | Maximale Laufzeit pro Co-Agent |
| `specialists.*.additional_prompt` | `""` | Zusätzliche Anweisungen pro Spezialist |
| `retry_policy.max_retries` | `1` | Wiederholungen bei temporären Fehlern |

---

## Weitere Konfigurationsblöcke (Übersicht)

Die folgenden Blöcke können ebenfalls über die Web-UI oder ergänzend in `config.yaml` konfiguriert werden. Die wichtigsten Parameter sind hier kompakt zusammengefasst:

| Block | Zweck | YAML-Kurzbeispiel |
|-------|-------|-------------------|
| `telegram` | Telegram Bot | `telegram:\n  bot_token: "..."\n  telegram_user_id: 12345678` |
| `discord` | Discord Bot | `discord:\n  enabled: true\n  bot_token: "..."\n  guild_id: "..."` |
| `email_accounts` | E-Mail IMAP/SMTP (Array) | `email_accounts:
  - id: personal
    name: Personal
    imap_host: imap.gmail.com
    smtp_host: smtp.gmail.com` |
| `email` | *(Legacy)* E-Mail IMAP/SMTP | `email:\n  enabled: true\n  imap_host: imap.gmail.com\n  smtp_host: smtp.gmail.com` |
| `home_assistant` | Home Assistant | `home_assistant:\n  enabled: true\n  url: "http://homeassistant.local:8123"` |
| `docker` | Docker API | `docker:\n  enabled: true\n  host: "unix:///var/run/docker.sock"` |
| `budget` | Kostenkontrolle | `budget:\n  enabled: false\n  daily_limit_usd: 5\n  enforcement: warn` |
| `fallback_llm` | Backup-LLM | `fallback_llm:\n  enabled: true\n  provider: fallback` |
| `co_agents` | Parallele Sub-Agenten | `co_agents:\n  enabled: true\n  max_concurrent: 3` |
| `circuit_breaker` | Resilienz | `circuit_breaker:\n  max_tool_calls: 20\n  llm_timeout_seconds: 600` |
| `auth` | Login-Schutz | `auth:\n  enabled: true\n  session_timeout_hours: 24` |
| `llm_guardian` | Sicherheitsprüfung | `llm_guardian:\n  enabled: false\n  default_level: medium` |
| `mcp_server` | MCP-Interoperabilität | `mcp_server:\n  enabled: false\n  require_auth: true` |
| `sandbox` | Isolierte Ausführung | `sandbox:\n  enabled: true\n  backend: docker\n  network_enabled: false` |
| `notifications` | Push-Benachrichtigungen | `notifications:\n  ntfy:\n    enabled: false\n  pushover:\n    enabled: true` |
| `tailscale` | Tailscale VPN | `tailscale:\n  enabled: false\n  tsnet:\n    enabled: false` |
| `ollama` | Lokale LLM-Verwaltung | `ollama:
  enabled: false
  url: ""
  managed_instance:
    enabled: false
    container_port: 11434
    use_host_gpu: false
    gpu_backend: auto` |
| `proxmox` / `meshcentral` / `ansible` | Infrastruktur | Jeweils `enabled: false`, `url: "", readonly: false` |
| `github` | GitHub API | `github:\n  enabled: false\n  owner: ""` |
| `s3` / `onedrive` / `webdav` / `koofr` | Cloud-Speicher | Jeweils `enabled: false`, Endpoint/Credentials in UI/Vault |
| `paperless_ngx` | Dokumentenworkflow | `paperless_ngx:\n  enabled: false\n  url: ""` |
| `telnyx` / `rocketchat` | Telefonie/Chat | `enabled: false`, Details über Web-UI |
| `google_workspace` | Google APIs | `google_workspace:\n  enabled: false\n  gmail: false\n  drive: false` |
| `egg_mode` | Worker-Mode | `egg_mode:
  enabled: false
  master_url: ""
  egg_id: ""
  nest_id: ""` |
| `invasion_control` | Remote Deployment | `invasion_control:
  enabled: false
  readonly: false` |
| `security_proxy` | Reverse Proxy | `security_proxy:
  enabled: false
  domain: ""
  rate_limiting:
    enabled: true` |
| `indexing` | Datei-Indexierung | `indexing:
  enabled: false
  poll_interval_seconds: 60
  index_images: false` |
| `a2a` | Agent-to-Agent | `a2a:
  server:
    enabled: false
    port: 0
  client:
    enabled: false` |
| `music_generation` | KI-Musik | `music_generation:
  enabled: false
  provider: ""
  max_daily: 0` |
| `firewall` | Firewall-Integration | `firewall:
  enabled: false
  mode: readonly` |
| `journal` | Journaleinträge | `journal:
  auto_entries: true
  daily_summary: true` |

> 📖 Für Details zu allen verfügbaren Parametern siehe `config_template.yaml` im Projektverzeichnis.

---

## Minimal-Konfiguration

Heute lässt sich die Basis-Konfiguration bequem über die Web-UI einrichten. Für ein schnelles Starten per YAML genügt:

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
    volumes:
      - ./secrets:/run/optional-secrets:ro
```

Lege den Vault-Master-Key bevorzugt in `./secrets/aurago_master.key` ab oder nutze Docker Secrets. Wenn kein Key gemountet ist, erzeugt AuraGo beim ersten Start einen Key im persistenten Datenvolume.

---

## Konfigurationsvalidierung

AuraGo validiert die Konfiguration beim Start:

```
[INFO] Loading config from ./config.yaml
[INFO] Configuration validated successfully
[ERROR] Invalid config: providers section requires at least one provider
```

### Häufige Validierungsfehler

| Fehler | Lösung |
|--------|--------|
| `providers section requires at least one provider` | Provider mit API-Key in `providers`-Liste konfigurieren |
| `invalid yaml` | YAML-Syntax prüfen (Einrückungen!) |
| `invalid port number` | Port zwischen 1-65535 wählen |
| `directory not found` | Verzeichnispfad korrigieren |

---

## Erweiterte Konfigurationsblöcke

Diese Ergänzung synchronisiert die deutsche Referenz mit den aktuellen englischen Kapiteln und dem aktuellen `config_template.yaml`.

### Medien- und Generierungsfeatures

In der Web-UI unter **Config → Integrationen → Bildgenerierung / Musikgenerierung / Videogenerierung** konfigurierbar. `media_registry` siehe Hinweis oben.

### YAML-Referenz
```yaml
image_generation:
  enabled: true
  provider: "openai"

music_generation:
  enabled: true
  provider: "minimax"
  max_daily: 10

video_generation:
  enabled: true
  provider: "minimax"
  model: "hailuo-02"
  duration_seconds: 6
  resolution: "720p"
  aspect_ratio: "16:9"
  max_daily: 5

media_registry:
  enabled: true
```

### Background Services

| Block | Zweck |
|-------|-------|
| `indexing` | Dateien überwachen, chunking und RAG-Index aktualisieren |
| `mission_preparation` | Missionen per LLM voranalysieren und Ergebnisse cachen |
| `maintenance` | Nightly Maintenance, Konsolidierung und Speicherpflege |
| `agent.output_compression` | Behaltene Tool-Ausgaben vor dem LLM-Kontext komprimieren |
| `co_agents` | Spezialisten-Agenten mit Budgets und Circuit Breakern konfigurieren |

### Externe Protokolle und Bridges

| Block | Zweck |
|-------|-------|
| `mcp` | Externe MCP-Server als Client nutzen |
| `mcp_server` | AuraGo selbst als MCP-Server bereitstellen |
| `a2a` | Agent-to-Agent-Kommunikation und Remote Agents |
| `python_tool_bridge` | Ausgewählte native Tools für Python-Skills freigeben |

### Sicherheit und Public Exposure

| Block | Zweck |
|-------|-------|
| `auth` | Login, Sessions, TOTP und Lockout-Regeln |
| `vault` | AES-GCM-Secret-Storage |
| `llm_guardian` | LLM-basierte Risikoanalyse für Tool Calls und externe Inhalte |
| `security_proxy` | Verwalteter Caddy-Proxy mit Rate-Limiting/IP-Filter |
| `cloudflare_tunnel` | Cloudflare Tunnel für Remote-Zugriff |
| `tailscale.tsnet` | Eingebetteter Tailscale-Node mit optionalem Funnel |

---

## Zusammenfassung

| Abschnitt | Zweck |
|-----------|-------|
| `providers` | Zentrale LLM-Verwaltung |
| `llm` | Haupt-LLM Auswahl |
| `embeddings` | RAG/Langzeitgedächtnis |
| `agent` | Verhalten & Berechtigungen |
| `tools.*` | Tool-Berechtigungen |
| `server` | Web-UI Einstellungen |
| `telegram/discord/email` | Integrationen |

> 💡 **Profi-Tipp:** Beginne mit der Minimal-Konfiguration und aktiviere Features nach Bedarf. Die Web-UI bietet die sicherste und komfortabelste Methode, die Konfiguration zu erweitern.

---

**Vorheriges Kapitel:** [Kapitel 6: Werkzeuge](./06-tools.md)  
**Nächstes Kapitel:** [Kapitel 8: Integrationen](./08-integrations.md)

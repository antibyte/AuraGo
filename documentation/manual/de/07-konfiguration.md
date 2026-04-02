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
   - **Server** – Host, Port, HTTPS
   - **Memory / Tasks / Sandbox** – weitere Systemeinstellungen
5. **Aktiviere Toggles, fülle Felder aus** und klicke unten auf **"Save"**
6. Einige Änderungen (z. B. Server-Port, Provider-Wechsel, Danger-Zone-Berechtigungen) erfordern einen **Neustart** – die UI zeigt dies entsprechend an

> 💡 **Tipp:** Sensible Werte wie API-Keys oder Passwörter werden automatisch im Vault gespeichert, wenn sie über die Web-UI eingegeben werden.

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
```

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
  https:
    enabled: false
    cert_mode: auto
    domain: ""
    email: ""
```

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `host` | `127.0.0.1` | Bind-Adresse |
| `port` | `8088` | HTTP-Port |
| `max_body_bytes` | `10485760` | Maximale Request-Größe |

---

## Agent-Verhalten

In der **Web-UI** unter *Config → Agent* lassen sich Sprache, Kontextfenster, Tool-Limit und die **Danger Zone** komfortabel einstellen. Änderungen an den Danger-Zone-Toggles werden nach dem Speichern meist erst nach einem Neustart wirksam.

```yaml
agent:
  system_language: "German"
  context_window: 131000
  max_tool_calls: 12
  debug_mode: false
  
  # Danger Zone - Tool-Berechtigungen
  allow_shell: true
  allow_python: true
  allow_filesystem_write: true
  allow_network_requests: true
  allow_remote_shell: true
  allow_self_update: true
  allow_mcp: true
  allow_web_scraper: true
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
- `allow_web_scraper` – Web-Scraping
- `sudo_enabled` – Sudo-Befehle (Passwort im Vault)

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
  web_scraper:
    enabled: true
    summary_mode: false
    summary_provider: ""
  document_creator:
    enabled: false
    backend: maroto
    output_dir: data/documents
  python_timeout_seconds: 30
  skill_timeout_seconds: 120
  background_timeout_seconds: 3600
```

---

## Skill Manager

Der Skill Manager ist in der **Web-UI** unter *Config → Skills* erreichbar. Dort kannst du Python-Skills hochladen, aktivieren, deaktivieren und über den integrierten Sicherheits-Scan prüfen lassen.

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
- `python_secret_injection` muss aktiviert sein, damit Skills Vault-Secrets nutzen können
- Der Guardian kann optional Code-Reviews durchführen

---

## Media Registry

Die Media Registry wird in der **Web-UI** unter *Config → Media* aktiviert. Sie kümmert sich um das Katalogisieren von Bildern, Videos und Audiodateien inklusive EXIF-Metadaten und Duplikat-Erkennung.

```yaml
media_registry:
  enabled: true
```

---

## Background Tasks

Hintergrund-Verarbeitung für Follow-ups, Cron-Jobs und Wait-Events. In der **Web-UI** unter *Config → Background Tasks* lassen sich Timeout- und Retry-Werte anpassen.

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

## Weitere Konfigurationsblöcke (Übersicht)

Die folgenden Blöcke können ebenfalls über die Web-UI oder ergänzend in `config.yaml` konfiguriert werden. Die wichtigsten Parameter sind hier kompakt zusammengefasst:

| Block | Zweck | YAML-Kurzbeispiel |
|-------|-------|-------------------|
| `telegram` | Telegram Bot | `telegram:\n  bot_token: "..."\n  telegram_user_id: 12345678` |
| `discord` | Discord Bot | `discord:\n  enabled: true\n  bot_token: "..."\n  guild_id: "..."` |
| `email` | E-Mail IMAP/SMTP | `email:\n  enabled: true\n  imap_host: imap.gmail.com\n  smtp_host: smtp.gmail.com` |
| `home_assistant` | Home Assistant | `home_assistant:\n  enabled: true\n  url: "http://homeassistant.local:8123"` |
| `docker` | Docker API | `docker:\n  enabled: true\n  host: "unix:///var/run/docker.sock"` |
| `budget` | Kostenkontrolle | `budget:\n  enabled: false\n  daily_limit_usd: 5\n  enforcement: warn` |
| `fallback_llm` | Backup-LLM | `fallback_llm:\n  enabled: true\n  provider: fallback` |
| `co_agents` | Parallele Sub-Agenten | `co_agents:\n  enabled: true\n  max_concurrent: 3` |
| `circuit_breaker` | Resilienz | `circuit_breaker:\n  max_tool_calls: 20\n  llm_timeout_seconds: 180` |
| `auth` | Login-Schutz | `auth:\n  enabled: true\n  session_timeout_hours: 24` |
| `llm_guardian` | Sicherheitsprüfung | `llm_guardian:\n  enabled: false\n  default_level: medium` |
| `mcp_server` | MCP-Interoperabilität | `mcp_server:\n  enabled: false\n  require_auth: true` |
| `sandbox` | Isolierte Ausführung | `sandbox:\n  enabled: true\n  backend: docker\n  network_enabled: false` |
| `notifications` | Push-Benachrichtigungen | `notifications:\n  ntfy:\n    enabled: false\n  pushover:\n    enabled: true` |
| `tailscale` | Tailscale VPN | `tailscale:\n  enabled: false\n  tsnet:\n    enabled: false` |
| `proxmox` / `meshcentral` / `ansible` / `ollama` | Infrastruktur | Jeweils `enabled: false`, `url: "", readonly: false` |
| `github` | GitHub API | `github:\n  enabled: false\n  owner: ""` |
| `s3` / `onedrive` / `webdav` / `koofr` | Cloud-Speicher | Jeweils `enabled: false`, Endpoint/Credentials in UI/Vault |
| `paperless_ngx` | Dokumentenworkflow | `paperless_ngx:\n  enabled: false\n  url: ""` |
| `telnyx` / `rocketchat` | Telefonie/Chat | `enabled: false`, Details über Web-UI |
| `google_workspace` | Google APIs | `google_workspace:\n  enabled: false\n  gmail: false\n  drive: false` |
| `egg_mode` / `invasion_control` | Verteilte Agenten | Siehe [Kapitel 12: Invasion Control](./12-invasion.md) |

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
      - AURAGO_MASTER_KEY=${MASTER_KEY}
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

> 💡 **Profi-Tipp:** Beginne mit der Minimal-Konfiguration und aktiviere Features nach Bedarf. Die Web-UI bietet die sicherste und komfortabelste Methode, die Konfiguration zu erweitern.

---

**Vorheriges Kapitel:** [Kapitel 6: Werkzeuge](./06-tools.md)  
**Nächstes Kapitel:** [Kapitel 8: Integrationen](./08-integrations.md)

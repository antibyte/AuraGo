# Kapitel 7: Konfiguration

Dieses Kapitel erklärt die vollständige Konfiguration von AuraGo über die `config.yaml`-Datei und die Web-Oberfläche.

## Übersicht

AuraGo wird über eine zentrale YAML-Konfigurationsdatei gesteuert: `config.yaml`. Diese Datei enthält alle Einstellungen für Server, LLM-Provider, Integrationen und Agent-Verhalten.

> 💡 **Tipp:** Änderungen an der Konfiguration erfordern in der Regel einen Neustart von AuraGo, um wirksam zu werden.

## Die config.yaml-Struktur

Die Konfigurationsdatei ist in thematische Abschnitte (Sections) unterteilt:

```yaml
server:           # Web-Server Einstellungen
llm:              # Haupt-LLM Konfiguration
providers:        # LLM-Provider (mehrere möglich)
agent:            # Agent-Verhalten
directories:      # Verzeichnispfade
embeddings:       # Embedding-Modell
logging:          # Log-Konfiguration
budget:           # Kostenkontrolle
telegram:         # Telegram Bot
discord:          # Discord Bot
email:            # E-Mail (IMAP/SMTP)
home_assistant:   # Home Assistant
webhooks:         # Eingehende Webhooks
# ... und weitere
```

## Server-Einstellungen

Der `server`-Abschnitt konfiguriert die Web-Oberfläche:

```yaml
server:
    host: "0.0.0.0"           # IP-Adresse (0.0.0.0 = alle Interfaces)
    port: 8088                # Port-Nummer
    bridge_address: "localhost:8089"  # Interne Bridge
    max_body_bytes: 10485760  # Max. Upload-Größe (10 MB)
```

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `host` | `127.0.0.1` | IP-Adresse für den Server |
| `port` | `8088` | Port für die Web-UI |
| `max_body_bytes` | `10485760` | Maximale Request-Größe |

> ⚠️ **Wichtig:** Für Docker muss `host` auf `0.0.0.0` gesetzt werden, damit der Container von außen erreichbar ist.

## LLM-Konfiguration

Der `llm`-Abschnitt definiert den KI-Provider:

```yaml
llm:
    provider: "main"                    # Provider-Eintrags-ID
    use_native_functions: true          # Native Funktionen nutzen
    temperature: 0.7                    # Kreativität (0.0-2.0)
    structured_outputs: false           # Strukturierte Ausgaben

providers:
    - id: "main"
      name: "Haupt-LLM"
      type: "openrouter"                # Provider-Typ
      base_url: "https://openrouter.ai/api/v1"
      api_key: "sk-or-v1-DEIN-KEY"
      model: "arcee-ai/trinity-large-preview:free"
```

### Unterstützte Provider-Typen

| Typ | Beschreibung | Beispiel-URL |
|-----|--------------|--------------|
| `openrouter` | OpenRouter (empfohlen) | `https://openrouter.ai/api/v1` |
| `openai` | OpenAI API | `https://api.openai.com/v1` |
| `anthropic` | Anthropic Claude | `https://api.anthropic.com` |
| `ollama` | Lokales Ollama | `http://localhost:11434/v1` |
| `google` | Google Gemini | `https://generativelanguage.googleapis.com` |

> 🔍 **Deep Dive:** Mit mehreren Provider-Einträgen kannst du verschiedene Modelle für verschiedene Zwecke nutzen (z.B. günstiges Modell für einfache Aufgaben, teures für komplexe).

## Embeddings-Konfiguration

Embeddings werden für die semantische Suche im Gedächtnis verwendet:

```yaml
embeddings:
    provider: "internal"                # "internal", "disabled" oder Provider-ID
    internal_model: "qwen/qwen3-embedding-8b"
    external_url: "http://localhost:11434/v1"
    external_model: "nomic-embed-text"
```

| Modus | Beschreibung |
|-------|--------------|
| `internal` | Nutzt das Haupt-LLM mit `internal_model` |
| `external` | Verwendet dedizierten Endpoint (z.B. Ollama) |
| `disabled` | Deaktiviert Embeddings vollständig |

## Agent-Verhalten

Der `agent`-Abschnitt steuert das Verhalten des KI-Agenten:

```yaml
agent:
    # Sprache und Persönlichkeit
    system_language: "Deutsch"
    core_personality: "friend"
    personality_engine: true
    personality_engine_v2: true
    personality_v2_model: "qwen/qwen-2.5-7b-instruct"
    
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
    enable_google_workspace: true
    system_prompt_token_budget: 8192
```

### Persönlichkeits-Engine

| Einstellung | Beschreibung |
|-------------|--------------|
| `personality_engine` | Aktiviert die Stimmungsanalyse |
| `personality_engine_v2` | Nutzt erweiterte V2-Analyse |
| `core_personality` | Basispersönlichkeit (z.B. `friend`, `professional`) |

### Sicherheitseinstellungen

```yaml
agent:
    # Danger Zone - Tool-Berechtigungen
    allow_shell: true                   # Shell-Befehle erlauben
    allow_python: true                  # Python-Code ausführen
    allow_filesystem_write: true        # Dateien schreiben
    allow_network_requests: true        # HTTP-Requests
    allow_remote_shell: true            # SSH-Remote-Shell
    allow_self_update: true             # Auto-Update
    allow_mcp: true                     # MCP-Server
    sudo_enabled: false                 # Sudo-Befehle
```

> ⚠️ **Warnung:** Deaktiviere `allow_shell` und `allow_python` für maximale Sicherheit, wenn du AuraGo mit fremden Personen teilst.

## Logging-Konfiguration

```yaml
logging:
    enable_file_log: true
    log_dir: "./log"
```

Log-Dateien werden in `log/supervisor.log` gespeichert. Die Rotation erfolgt automatisch.

## Konfiguration über Web UI vs. YAML

AuraGo bietet zwei Wege zur Konfiguration:

### Web UI (empfohlen für Anfänger)

1. Öffne die Web-Oberfläche
2. Klicke auf das Radial-Menü (≡)
3. Wähle "Config"
4. Bearbeite die Werte
5. Klicke "Save"

**Vorteile:**
- Intuitive Oberfläche
- Echtzeit-Validierung
- Kein Syntax-Wissen nötig

**Nachteile:**
- Manche erweiterte Einstellungen nicht verfügbar
- Keine Kommentare möglich

### Direkte YAML-Bearbeitung

```bash
nano config.yaml    # oder vim, code, notepad
```

**Vorteile:**
- Vollständige Kontrolle
- Kommentare möglich
- Schneller für fortgeschrittene Nutzer

**Nachteile:**
- YAML-Syntax muss korrekt sein
- Keine Validierung während der Eingabe

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
| `llm.api_key is required` | API-Key in Provider-Eintrag einfügen |
| `invalid yaml` | YAML-Syntax prüfen (Einrückungen!) |
| `invalid port number` | Port zwischen 1-65535 wählen |
| `directory not found` | Verzeichnispfad korrigieren |

## Umgebungsvariablen

Bestimmte Einstellungen können via Umgebungsvariablen überschrieben werden:

| Variable | Beschreibung | Priorität |
|----------|--------------|-----------|
| `AURAGO_SERVER_HOST` | Server-Host | Höchste |
| `AURAGO_MASTER_KEY` | Master-Key für Vault | Hoch |
| `LLM_API_KEY` | API-Key für Haupt-LLM | Mittel |
| `OPENAI_API_KEY` | Fallback für LLM_API_KEY | Mittel |
| `ANTHROPIC_API_KEY` | Fallback für LLM_API_KEY | Mittel |
| `EMBEDDINGS_API_KEY` | API-Key für Embeddings | Mittel |
| `VISION_API_KEY` | API-Key für Vision | Mittel |
| `WHISPER_API_KEY` | API-Key für Whisper/STT | Mittel |
| `CO_AGENTS_LLM_API_KEY` | API-Key für Co-Agents | Mittel |
| `FALLBACK_LLM_API_KEY` | API-Key für Fallback-LLM | Mittel |

### Beispiel: Docker mit Umgebungsvariablen

```yaml
# docker-compose.yml
services:
  aurago:
    environment:
      - AURAGO_SERVER_HOST=0.0.0.0
      - LLM_API_KEY=${LLM_API_KEY}
      - AURAGO_MASTER_KEY=${MASTER_KEY}
```

## Häufige Konfigurationsbeispiele

### Minimal-Konfiguration

```yaml
server:
    host: "127.0.0.1"
    port: 8088

llm:
    provider: "main"

providers:
    - id: "main"
      type: "openrouter"
      base_url: "https://openrouter.ai/api/v1"
      api_key: "DEIN-API-KEY"
      model: "arcee-ai/trinity-large-preview:free"
```

### Mit Telegram-Bot

```yaml
# ... Basis-Konfiguration ...

telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    telegram_user_id: 12345678
    max_concurrent_workers: 5
```

### Mit Home Assistant

```yaml
# ... Basis-Konfiguration ...

home_assistant:
    enabled: true
    url: "http://homeassistant.local:8123"
    # Access Token wird im Vault gespeichert
```

### Mit Budget-Tracking

```yaml
# ... Basis-Konfiguration ...

budget:
    enabled: true
    daily_limit_usd: 2.0
    enforcement: "warn"           # "warn", "partial", "full"
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

### Mit Fallback-LLM

```yaml
# ... Basis-Konfiguration ...

fallback_llm:
    enabled: true
    provider: "fallback"
    probe_interval_seconds: 60
    error_threshold: 2

providers:
    - id: "main"
      name: "Haupt-LLM"
      type: "openrouter"
      base_url: "https://openrouter.ai/api/v1"
      api_key: "MAIN-KEY"
      model: "anthropic/claude-3-opus"
    
    - id: "fallback"
      name: "Fallback-LLM"
      type: "openrouter"
      base_url: "https://openrouter.ai/api/v1"
      api_key: "FALLBACK-KEY"
      model: "meta-llama/llama-3.1-8b-instruct:free"
```

### Für Docker-Deployment

```yaml
server:
    host: "0.0.0.0"           # Wichtig für Docker!
    port: 8088

directories:
    data_dir: "./data"
    workspace_dir: "./agent_workspace"
    
# Pfade bleiben relativ, werden beim Start aufgelöst
```

## Circuit Breaker (Schutzmechanismen)

Der Circuit Breaker verhindert Endlosschleifen und übermäßige Kosten:

```yaml
circuit_breaker:
    max_tool_calls: 20              # Maximale Tool-Aufrufe pro Anfrage
    llm_timeout_seconds: 180        # Timeout für LLM-Antworten
    maintenance_timeout_minutes: 10 # Wartungsmodus-Timeout
    retry_intervals:
        - "10s"                     # Erster Retry nach 10 Sekunden
        - "2m"                      # Zweiter Retry nach 2 Minuten
        - "10m"                     # Dritter Retry nach 10 Minuten
```

## Co-Agents Konfiguration

Co-Agents ermöglichen parallele Aufgabenausführung:

```yaml
co_agents:
    enabled: true
    max_concurrent: 3
    llm:
        provider: "coagent"
    circuit_breaker:
        max_tool_calls: 12
        timeout_seconds: 300
        max_tokens: 0
```

## Nächste Schritte

- **[Integrationen](08-integrations.md)** – Telegram, Discord & mehr verbinden
- **[Gedächtnis & Wissen](09-memory.md)** – Speicherverwaltung verstehen
- **[Troubleshooting](16-troubleshooting.md)** – Probleme lösen

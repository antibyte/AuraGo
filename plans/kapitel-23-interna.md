# Plan: Kapitel 23 – Interna

## Ziel

Neues Handbuch-Kapitel **"Interna"** (Kapitel 23) das die interne Arbeitsweise von AuraGo im Detail beschreibt – alle Module, Komponenten, Datenflüsse und Architektur-Entscheidungen. Zielgruppe: Entwickler, fortgeschrittene Nutzer und Beitragende, die verstehen wollen, wie das System unter der Haube funktioniert.

## Dateien

| Datei | Sprache |
|-------|---------|
| `documentation/manual/de/23-interna.md` | Deutsch |
| `documentation/manual/en/23-internals.md` | Englisch |
| `documentation/manual/de/README.md` | Update: Kapitel 23 ergänzen |
| `documentation/manual/en/README.md` | Update: Kapitel 23 ergänzen |
| `documentation/manuals/README.md` | Update: Link ergänzen |

## Abgrenzung zu bestehenden Kapiteln

| Bestehendes Kapitel | Fokus | Abgrenzung |
|---------------------|-------|------------|
| Kap. 6 – Tools | Welche Tools gibt es, wie nutzt man sie | Kap. 23 beschreibt die **Tool-Infrastruktur** (Registry, Dispatch, Pipeline, Policy) |
| Kap. 9 – Memory | Gedächtnis aus Nutzersicht | Kap. 23 beschreibt die **internen Speichersubsysteme** (SQLite-Schema, Vector-DB, Embedding-Pipeline) |
| Kap. 14 – Sicherheit | Sicherheitskonzepte für Nutzer | Kap. 23 beschreibt die **internen Sicherheitsmodule** (Guardian-Algorithmus, Vault-Verschlüsselung, SSRF-Filter) |
| Kap. 22 – Interne Tools | Referenz aller 100+ Tool-Definitionen | Kap. 23 beschreibt die **Architektur dahinter** (Kategorien, Feature-Flags, Tool-Call-Pipeline) |

---

## Geplante Gliederung

### 23.1 Systemarchitektur – Überblick
- Hocharchitektur-Diagramm (Mermaid): Core → Memory → Tools → Integrations
- Schichtenmodell: Presentation Layer → Agent Layer → Service Layer → Data Layer
- Single-Binary-Konzept und `go:embed`
- Nebenläufigkeitsmodell: Goroutines, Channels, `sync.Mutex`, `errgroup`

### 23.2 Startprozess und Initialisierung
- `cmd/aurago/main.go` – Boot-Reihenfolge
- CLI-Flags (`-debug`, `-setup`, `-init-only`, `-config`, `--sandbox-exec`)
- Secrets laden: systemd → Docker Secrets → `/etc/aurago/master.key` → `.env`
- Konfiguration laden und validieren (`internal/config`)
- Datenbank-Initialisierung (SQLite, Migrations via `internal/dbutil`)
- Vault entschlüsseln (`internal/security/vault.go`)
- LLM-Client und Failover-Manager erstellen (`internal/llm`)
- Memory-Subsysteme initialisieren (STM, LTM, KG)
- Server starten (`internal/server`)
- Background-Services starten (Indexer, Ingestion)
- Setup-Wizard (`internal/setup`)

### 23.3 Der Agent-Loop
- `ExecuteAgentLoop()` – Hauptschleife
- `RunConfig` – Alle Abhängigkeiten gebündelt
- Ablauf: System-Prompt → LLM-Call → Tool-Call parsen → Ausführen → Antwort → SSE-Stream
- Concurrency-Limiter (`maxConcurrentAgentLoops = 8`)
- Multi-Turn-Reasoning: Schleife bis `finish_reason=stop` oder `<done/>`
- Streaming vs. synchroner Modus
- `FeedbackBroker` – SSE-Events an den Client

### 23.4 Tool-System
- **Tool Registry**: Registrierung und Feature-Flags (`ToolFeatureFlags`)
- **Tool-Kategorien**: system, files, network, media, communication, smart_home, infrastructure, memory, database, devtools
- **Native Function Calling**: OpenAI-kompatible JSON-Schema-Definitionen
- **Tool-Call-Pipeline**: `parseToolResponse()` → Native vs. Content-JSON vs. Reasoning-JSON
- **Tooling Policy**: Modellfähigkeiten erkennen, adaptives Verhalten
- **Dispatch Context**: `DispatchContext` – alle Abhängigkeiten für Tool-Ausführung
- **Adaptive Tools**: Nutzungsbasierte Tool-Filterung (Token sparen)
- **Tool Execution Policy**: Berechtigungsprüfung, Rate-Limiting
- **Tool Recovery**: Fehlerbehandlung und Wiederholung

### 23.5 Memory-Subsystem
- **Short-Term Memory (STM)**: SQLite sliding-window, `HistoryManager`, pinned messages
- **Long-Term Memory (LTM)**: chromem-go Vector-DB, Embeddings, Collections
- **Knowledge Graph**: SQLite + FTS5, Entitäten und Relationen
- **Core Memory**: Permanente Fakten, immer im Kontext
- **Embedding-Pipeline**: Ollama-Embeddings, Batch-Verarbeitung
- **Memory Analysis**: Effektivitätsmessung, Konflikterkennung, Priorisierung
- **Predictive Memory**: Vorabladen relevanter Erinnerungen
- **Context Compression**: Token-budgetbewusste Kontextverdichtung
- **Journal**: Tagebuchfunktion mit Pending-Queue

### 23.6 LLM-Client-Schicht
- **ChatClient-Interface**: `CreateChatCompletion`, `CreateChatCompletionStream`
- **FailoverManager**: Primary/Fallback mit automatischem Switch, Health-Probes
- **Retry-Logik**: Exponential Backoff, Error-Klassifikation
- **Provider-System**: OpenRouter, OpenAI, Anthropic, Ollama, Custom
- **Modellfähigkeiten**: `ModelCapabilities` – Provider-spezifische Quirks
- **Token-Tracking**: `TokenAccounting`, `TokenCountCache`
- **Pricing**: Kostenberechnung pro Provider/Modell

### 23.7 Prompt-System
- **Prompt Builder**: Dynamischer System-Prompt aus Modulen
- **Prompt Modules**: Identität, Regeln, Persönlichkeit, Tool-Guides, Kontext
- **Caching**: Datei-basiertes Cache mit ModTime-Invalidierung
- **Tiktoken**: Token-Zählung für Budget-Steuerung
- **Dynamic Guide Strategy**: Tool-Guides basierend auf Nutzung adaptieren
- **Prompt Budget**: Token-Budget für System-Prompt-Komponenten

### 23.8 Sicherheitsarchitektur
- **Vault**: AES-256-GCM, file-basiertes Locking (`flock`), Master-Key
- **LLM Guardian**: KI-gestützte Tool-Call-Prüfung, GuardianLevel (Off/Low/Medium/High)
- **Regex Guardian**: Pattern-basierte Bedrohungserkennung (ThreatLevel)
- **SSRF-Schutz**: URL-Validierung, interne Netzwerk-Blockliste
- **Scrubber**: Sensible Daten aus Logs und LLM-Outputs entfernen
- **Sandbox**: Landlock (Linux), Prozess-Isolation, venv für Python

### 23.9 Server und API
- **HTTP/HTTPS Server**: `internal/server/server.go`
- **REST API**: Handler-Struktur, Routen-Registry
- **SSE (Server-Sent Events)**: Streaming-Infrastruktur, Broker-Adapter
- **TLS/HTTPS**: Let's Encrypt Integration
- **Auth**: Session-basierte Authentifizierung
- **i18n**: Internationalisierung (15 Sprachen)
- **Fileserver**: Statische Dateien und Uploads

### 23.10 Co-Agenten
- **CoAgentRegistry**: Parallele Agenten verwalten
- **CoAgentRequest**: Task, Specialist, Priority
- **Specialist-Rollen**: researcher, coder, designer, security, writer
- **LLM-Auswahl**: Separater Provider/Modell für Co-Agenten
- **Broker-System**: Events zwischen Hauptagent und Co-Agenten

### 23.11 Invasion Control
- **Invasion-System**: Remote-Deployment von AuraGo-Instanzen
- **Connectors**: SSH (`connector_ssh.go`), Docker (`connector_docker.go`)
- **Egg-Config**: Konfiguration verteilter Instanzen
- **Bridge-Protokoll**: Kommunikation zwischen Nest und Eggs
- **Vault-Export**: Sichere Übertragung von Secrets

### 23.12 Remote-Ausführung
- **RemoteHub**: Verwaltung von SSH-Verbindungen
- **Protokoll**: Binäres Protokoll für Remote-Kommandos
- **Inventory**: SQLite-basierte Geräteverwaltung
- **Geräte-Registrierung**: `/addssh` Command

### 23.13 Background-Services
- **File Indexer**: Dateien indizieren und in Vector-DB speichern
- **Knowledge Graph Extraction**: Automatische Entitätsextraktion
- **Mission Preparation**: Vorbereitung langlaufender Missionen
- **Optimizer**: Kontinuierliche Optimierung der Indizes

### 23.14 A2A-Protokoll (Agent-to-Agent)
- **A2A Server/Client**: Inter-Agent-Kommunikation
- **gRPC**: Binäres Protokoll für A2A
- **Task Management**: Aufgabenverteilung zwischen Agenten
- **Auth**: Authentifizierung zwischen Agenten

### 23.15 Budget und Kostenkontrolle
- **Budget Tracker**: Token-Verbrauch und Kosten pro Session
- **Cost Optimizer**: Automatische Kostenoptimierung
- **OpenRouter Credits**: Kreditstand abfragen

### 23.16 Planner und Automatisierung
- **Planner**: Mehrstufige Ausführungspläne
- **Cron Manager**: Zeitgesteuerte Aufgaben
- **Daemon Supervisor**: Hintergrundprozesse verwalten
- **Follow-Up**: Autonome Hintergrundaufgaben
- **Wait-for-Event**: Ereignisbasierte Triggers

### 23.17 Kommunikations-Integrationen
- **Telegram Bot**: `internal/telegram` – Text, Voice, Vision
- **Discord Bot**: `internal/discord`
- **Rocket.Chat**: `internal/rocketchat`
- **Telnyx**: SMS/Voice über `internal/telnyx`
- **Push Notifications**: `internal/push`

### 23.18 Smart Home und IoT
- **Fritz!Box**: TR-064, AHA-Client, Smart Home, Telefonie
- **Home Assistant**: Poller-basierte Integration
- **MQTT**: Publish/Subscribe Messaging
- **Wyoming**: Voice-Assistant-Protokoll

### 23.19 Infrastruktur-Integrationen
- **Docker**: Container-Verwaltung, Docker Compose
- **Proxmox**: VM/Container-Management
- **Tailscale**: VPN über tsnet
- **Cloudflare Tunnel**: Sicherer Remote-Zugriff
- **Homepage**: Dashboard-Builder-Integration

### 23.20 Medien und Content
- **Jellyfin**: Media-Server-Integration
- **Chromecast**: Medien an Cast-Geräte
- **TTS/Piper**: Text-to-Speech
- **Image Generation**: DALL-E, Stable Diffusion, Ideogram, MiniMax
- **Music Generation**: KI-basierte Musikgenerierung
- **Media Registry**: Medien-Verwaltung

### 23.21 Datenfluss-Diagramme
- Kompletter Request-Lebenszyklus (Mermaid-Sequenzdiagramm)
- Tool-Call-Dispatch-Flow (Mermaid-Flussdiagramm)
- Memory-Retrieval-Flow (Mermaid-Flussdiagramm)

---

## Mermaid-Diagramme

Folgende Diagramme sind geplant:

1. **Systemübersicht** – Layered Architecture
2. **Startprozess** – Sequenzdiagramm der Initialisierung
3. **Agent-Loop** – Detaillierter Ablauf mit Verzweigungen
4. **Tool-Call-Pipeline** – Von LLM-Response bis Tool-Ausführung
5. **Memory-Retrieval** – STM → LTM → KG → Context Assembly
6. **LLM-Failover** – Primary/Fallback Switch
7. **Request-Lebenszyklus** – Vom HTTP-Request bis zur SSE-Response

---

## Nächste Schritte

1. Plan mit Nutzer abstimmen
2. Deutsche Version schreiben
3. Englische Version schreiben
4. README-Dateien aktualisieren

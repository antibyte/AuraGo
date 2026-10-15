# Kapitel 17: Glossar

Dieses Glossar erklärt alle Fachbegriffe, Abkürzungen und Konzepte von AuraGo.

---

## Inhaltsverzeichnis

1. [Abkürzungen und Akronyme](#abkürzungen-und-akronyme)
2. [Agent-Terminologie](#agent-terminologie)
3. [Gedächtnis-System](#gedächtnis-system)
4. [Tools und Aktionen](#tools-und-aktionen)
5. [Konfigurationsbegriffe](#konfigurationsbegriffe)
6. [Sicherheitsvokabular](#sicherheitsvokabular)
7. [Integrationen](#integrationen)
8. [Persönlichkeit und Verhalten](#persönlichkeit-und-verhalten)
9. [Technische Architektur](#technische-architektur)
10. [Cross-Referenzen](#cross-referenzen)
11. [Aktuelle Begriffe](#aktuelle-begriffe)

---

## Abkürzungen und Akronyme

| Abkürzung | Bedeutung | Erklärung |
|-----------|-----------|-----------|
| **AES** | Advanced Encryption Standard | Symmetrisches Verschlüsselungsverfahren, im Vault verwendet |
| **AI** | Artificial Intelligence | Computersysteme, die Aufgaben mit menschenähnlicher Intelligenz lösen |
| **API** | Application Programming Interface | Schnittstelle für die Kommunikation zwischen Software |
| **CLI** | Command Line Interface | Textbasierte Benutzeroberfläche |
| **CGO** | C Go | Gos FFI zum Aufruf von C-Code (AuraGo nutzt es nicht) |
| **CSS** | Cascading Style Sheets | Beschreibungssprache für das Aussehen der Web-UI |
| **DB** | Database | Strukturierte Datenspeicherung |
| **FAQ** | Frequently Asked Questions | Häufige Fragen und Antworten |
| **FFI** | Foreign Function Interface | Schnittstelle zum Aufruf von Code anderer Sprachen |
| **GC** | Garbage Collection | Automatische Speicherverwaltung |
| **GCM** | Galois/Counter Mode | Verschlüsselungsmodus für AES (AES-256-GCM) |
| **HA** | Home Assistant | Open-Source-Plattform für Heimautomation |
| **HTML** | HyperText Markup Language | Auszeichnungssprache der Web-UI |
| **HTTP** | HyperText Transfer Protocol | Protokoll für Web-Kommunikation |
| **IMAP** | Internet Message Access Protocol | Protokoll zum Abruf von E-Mails |
| **IoT** | Internet of Things | Netzwerk verbundener Geräte |
| **JSON** | JavaScript Object Notation | Datenaustauschformat |
| **LLM** | Large Language Model | Sprachmodell für natürliche Sprache |
| **LAN** | Local Area Network | Lokales Rechnernetz |
| **LTM** | Long-Term Memory | Dauerhafter Speicher mit semantischer Suche |
| **MCP** | Model Context Protocol | Protokoll für geteilten Kontext |
| **MQTT** | Message Queuing Telemetry Transport | Schlankes Nachrichtenprotokoll |
| **NAT** | Network Address Translation | Verfahren zur Umadressierung von IPs |
| **Ollama** | Ollama | Werkzeug zum Betrieb lokaler LLMs |
| **OS** | Operating System | Systemsoftware zur Verwaltung der Hardware |
| **PID** | Process Identifier | Eindeutige Nummer eines Prozesses |
| **Proxmox** | Proxmox VE | Open-Source-Virtualisierungsplattform |
| **RAG** | Retrieval-Augmented Generation | Kombination aus Abruf und Textgenerierung |
| **REST** | Representational State Transfer | Architekturstil für APIs |
| **ROM** | Read-Only Memory | Speicher, der nur gelesen werden kann |
| **SDK** | Software Development Kit | Werkzeuge für die Softwareentwicklung |
| **SFTP** | SSH File Transfer Protocol | Sicheres Dateiübertragungsprotokoll |
| **SMTP** | Simple Mail Transfer Protocol | Protokoll zum Versand von E-Mails |
| **SPA** | Single Page Application | Web-App, die eine einzelne HTML-Seite lädt |
| **SQL** | Structured Query Language | Sprache für Datenbankabfragen |
| **SQLite** | SQLite | Eingebettete SQL-Datenbank-Engine |
| **SSE** | Server-Sent Events | Technik für Streaming vom Server zum Client |
| **SSH** | Secure Shell | Kryptographisches Netzwerkprotokoll |
| **SSL** | Secure Sockets Layer | Veraltetes Sicherheitsprotokoll (stattdessen TLS) |
| **STM** | Short-Term Memory | Speicher der letzten Gesprächsbeiträge |
| **STT** | Speech-to-Text | Umwandlung von Sprache in Text |
| **TLS** | Transport Layer Security | Kryptographisches Protokoll für gesicherte Kommunikation |
| **TOTP** | Time-based One-Time Password | 2FA-Verfahren mit zeitsynchronen Codes |
| **TS** | TypeScript | Programmiersprache (Web-UI) |
| **TTS** | Text-to-Speech | Umwandlung von Text in gesprochene Sprache |
| **UI** | User Interface | Visuelle Schnittstelle zur Interaktion |
| **URI** | Uniform Resource Identifier | Zeichenkette zur Identifikation einer Ressource |
| **URL** | Uniform Resource Locator | Web-Adresse |
| **UUID** | Universally Unique Identifier | 128-Bit-eindeutige Kennung |
| **VM** | Virtual Machine | Emuliertes Computersystem |
| **VPN** | Virtual Private Network | Gesicherte Netzwerkverbindung |
| **WAL** | Write-Ahead Logging | Technik für Datenbank-Dauerhaftigkeit |
| **WebDAV** | Web Distributed Authoring and Versioning | HTTP-Erweiterung für Dateiverwaltung |
| **YAML** | YAML Ain't Markup Language | Menschenlesbares Serialisierungsformat |
| **2FA** | Two-Factor Authentication | Absicherung durch zwei Nachweisverfahren |

---

## Agent-Terminologie

### Kernkonzepte des Agenten

| Begriff | Definition | Verwandte Begriffe |
|---------|------------|--------------------|
| **Agent Loop** | Der Hauptzyklus, in dem der Agent Eingaben verarbeitet, Aktionen wählt und Antworten erzeugt | Tool Dispatch, Reasoning |
| **Chain of Thought** | Das schrittweise Begründungsverfahren des Agenten | Reasoning Loop |
| **Co-Agent** | Paralleler Sub-Agent für komplexe Aufgaben | Sub-Agent, Parallelverarbeitung |
| **Context Window** | Die Textmenge (in Tokens), die das LLM gleichzeitig verarbeiten kann | Token-Limit, Gedächtnis |
| **Function Calling** | LLM-Fähigkeit, Funktionen/Tools mit strukturierten Parametern aufzurufen | Native Functions, Tools |
| **Inference** | Der Vorgang, aus einem LLM eine Ausgabe zu erzeugen | Prediction, Generation |
| **Orchestrator** | Haupt-Agent, der Co-Agenten koordiniert | Main Agent, Controller |
| **Prompt** | Anweisungen an das LLM | System Prompt, User Prompt |
| **Reasoning** | Der logische Denkprozess des Agenten | Chain of Thought |
| **Session** | Ein durchgehender Gesprächskontext | Chat, Konversation |
| **System Prompt** | Grundlegende Anweisungen, die das Agentenverhalten definieren | Persönlichkeit, Kontext |
| **Temperature** | Parameter für die Zufälligkeit der LLM-Ausgabe | Kreativität, Randomness |
| **Token** | Verarbeitungseinheit für Text (ca. 4 Zeichen im Englischen) | Tokenisierung |
| **Tool Call** | Aufruf einer bestimmten Fähigkeit durch den Agenten | Action, Function |
| **Tool Dispatch** | Weiterleitung von Tool-Aufrufen an die Implementierungen | Tool Registry |

### Agenten-Aktionen

| Aktion | Beschreibung | Beispiel-Tools |
|--------|--------------|----------------|
| **Analyze** | Inhalte auf Erkenntnisse untersuchen | analyze_image, analyze_document |
| **Create** | Neue Inhalte oder Dateien erzeugen | write_file, create_note |
| **Execute** | Code oder Befehle ausführen | execute_shell, execute_python |
| **Modify** | Bestehende Inhalte ändern | edit_file, update_memory |
| **Query** | Informationen abfragen | query_memory, web_search |
| **Schedule** | Künftige Aktionen einplanen | cron_scheduler, follow_up |
| **Transform** | Format von Inhalten umwandeln | transcribe, translate |

---

## Gedächtnis-System

### Gedächtnistypen

| Begriff | Definition | Dauerhaftigkeit | Zugriff |
|---------|------------|-----------------|---------|
| **Core Memory** | Dauerhafte Fakten, an die der Agent sich immer erinnert | Persistent | Immer enthalten |
| **Ephemeral Memory** | Temporärer Speicher für eine einzelne Sitzung | Nur Sitzung | Aktueller Chat |
| **Knowledge Graph** | Strukturierte Speicherung von Entitäten und Beziehungen | Persistent | Abfrage/Suche |
| **Langzeitgedächtnis (LTM)** | Vektorbasiertes semantisches Gedächtnis | Persistent | Ähnlichkeitssuche |
| **Persistent Summary** | Komprimierte Hintergrundinformationen | Persistent | Hintergrundkontext |
| **Kurzzeitgedächtnis (STM)** | Letzte Gesprächsbeiträge | Persistent | Gleitendes Fenster |
| **Working Memory** | Aktiver Verarbeitungsbereich | Temporär | Aktuelle Operation |

### Gedächtnisoperationen

| Operation | Beschreibung | Tool |
|-----------|--------------|------|
| **Compression** | Verkleinern des Kontexts bei erhaltenem Sinn | Automatisch |
| **Embedding** | Umwandlung von Text in numerische Vektoren | query_memory |
| **Indexing** | Aufnahme von Inhalten in den durchsuchbaren Speicher | store_memory |
| **Query** | Gezieltes Abfragen von Informationen | query_memory |
| **Recall** | Abruf gespeicherter Informationen | Verschiedene |
| **Storage** | Speichern von Informationen für später | manage_memory |
| **Summarization** | Erzeugen verdichteter Fassungen | Automatisch |
| **Vector Search** | Semantische Ähnlichkeitssuche | query_memory |

### Datenstrukturen des Gedächtnisses

| Begriff | Beschreibung | Verwendet in |
|---------|--------------|--------------|
| **Chunk** | Textabschnitt für das Embedding | LTM-Speicher |
| **Document** | Einheit gespeicherter Information | VectorDB |
| **Entity** | Benanntes Objekt im Knowledge Graph | Knowledge Graph |
| **Node** | Element im Knowledge Graph | Knowledge Graph |
| **Edge** | Beziehung zwischen Entitäten | Knowledge Graph |
| **Vector** | Numerische Repräsentation von Text | Embeddings |

---

## Tools und Aktionen

### System-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **execute_shell** | Shell-Befehle ausführen | System |
| **execute_python** | Python-Code ausführen | System |
| **read_file** | Dateiinhalte lesen | Dateisystem |
| **write_file** | Dateien schreiben/erstellen | Dateisystem |
| **list_directory** | Verzeichnisinhalt auflisten | Dateisystem |
| **get_system_metrics** | CPU-, Speicher- und Platteninformationen abrufen | System |
| **get_current_time** | Aktuelles Datum und Uhrzeit abrufen | System |

### Kommunikations-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **send_email** | E-Mail per SMTP senden | Kommunikation |
| **telegram_send** | Telegram-Nachricht senden | Kommunikation |
| **discord_send** | Discord-Nachricht senden | Kommunikation |
| **tts_speak** | Sprachausgabe (Text-to-Speech) | Kommunikation |
| **notify_push** | Push-Benachrichtigung senden | Kommunikation |

### Web- und Such-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **web_search** | Im Web suchen | Web |
| **web_scraper** | Inhalte von Websites extrahieren | Web |
| **fetch_url** | Inhalt einer Webseite abrufen | Web |
| **wikipedia_search** | Wikipedia durchsuchen | Web |
| **ddg_search** | DuckDuckGo-Suche | Web |

### Smart-Home-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **home_assistant_get_state** | Gerätezustände lesen | Smart Home |
| **home_assistant_call_service** | HA-Services ausführen | Smart Home |
| **home_assistant_toggle** | Geräte ein-/ausschalten | Smart Home |
| **chromecast_control** | Chromecast-Geräte steuern | Smart Home |
| **chromecast_tts** | Sprachausgabe an Chromecast senden | Smart Home |

### Infrastruktur-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **docker_list** | Docker-Container auflisten | Infrastruktur |
| **docker_start** | Container starten | Infrastruktur |
| **docker_stop** | Container stoppen | Infrastruktur |
| **docker_logs** | Container-Logs ansehen | Infrastruktur |
| **proxmox_list_vms** | Proxmox-VMs auflisten | Infrastruktur |
| **proxmox_start_vm** | VM starten | Infrastruktur |
| **proxmox_stop_vm** | VM stoppen | Infrastruktur |
| **ollama_list** | Ollama-Modelle auflisten | Infrastruktur |
| **ollama_pull** | Ollama-Modell herunterladen | Infrastruktur |

### Gedächtnis- und Wissens-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **manage_memory** | Erinnerungen hinzufügen/ändern/löschen | Gedächtnis |
| **query_memory** | Langzeitgedächtnis durchsuchen | Gedächtnis |
| **knowledge_graph_query** | Knowledge Graph abfragen | Wissen |
| **manage_notes** | Notizen verwalten (CRUD) | Gedächtnis |
| **manage_todos** | Aufgaben verwalten | Gedächtnis |

### Entwickler-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **git_clone** | Git-Repositories klonen | Entwicklung |
| **git_commit** | Änderungen committen | Entwicklung |
| **github_create_issue** | GitHub-Issues erstellen | Entwicklung |
| **github_list_repos** | Repositories auflisten | Entwicklung |
| **code_surgery** | Quellcode gezielt ändern | Entwicklung |

### Integrations-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **webdav_list** | WebDAV-Dateien auflisten | Speicher |
| **webdav_download** | Von WebDAV herunterladen | Speicher |
| **webdav_upload** | Nach WebDAV hochladen | Speicher |
| **koofr_list** | Koofr-Dateien auflisten | Speicher |
| **google_workspace_query** | Google-Dienste abfragen | Produktivität |
| **mqtt_publish** | MQTT-Nachricht veröffentlichen | IoT |
| **mqtt_subscribe** | MQTT-Topic abonnieren | IoT |

### Hilfs-Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| **cron_scheduler** | Wiederkehrende Aufgaben planen | Hilfswerkzeug |
| **follow_up** | Künftige Aktionen einplanen | Hilfswerkzeug |
| **analyze_image** | Bildinhalte analysieren | Vision |
| **transcribe_audio** | Sprache in Text umwandeln | Audio |
| **stop_process** | Hintergrundprozesse beenden | System |

---

## Konfigurationsbegriffe

### Server-Konfiguration

| Begriff | Beschreibung | Standard |
|---------|--------------|----------|
| **host** | Netzwerkschnittstelle, an die gebunden wird | `127.0.0.1` |
| **port** | HTTP-Server-Port | `8088` |
| **max_body_bytes** | Maximale Größe eines Anfrage-Bodys | `10485760` (10 MB) |

### Agent-Konfiguration

| Begriff | Beschreibung | Standard |
|---------|--------------|----------|
| **context_window** | Kontextgröße des LLM in Tokens | Automatische Erkennung |
| **max_tool_calls** | Maximale Tool-Aufrufe pro Anfrage | `12` |
| **step_delay_seconds** | Pause zwischen Tool-Aufrufen | `0` |
| **system_language** | Antwortsprache des Agenten | `Englisch` |
| **personality_engine** | Stimmungsanpassung aktivieren | `true` |
| **core_personality** | Basis-Persönlichkeitsvorlage | `friend` |
| **debug_mode** | Debug-Anweisungen aktivieren | `false` |

### LLM-Konfiguration

| Begriff | Beschreibung | Beispiel |
|---------|--------------|----------|
| **provider** | LLM-Dienstanbieter | `openrouter` |
| **base_url** | API-Endpunkt-Adresse | `https://openrouter.ai/api/v1` |
| **api_key** | Authentifizierungstoken | `sk-or-v1-...` |
| **model** | Kennung des konkreten Modells | `gpt-4` |
| **temperature** | Zufälligkeit der Ausgabe (0–2) | `0.7` |
| **structured_outputs** | Eingeschränkte Dekodierung verwenden | `false` |

### Circuit Breaker

| Begriff | Beschreibung | Standard |
|---------|--------------|----------|
| **max_tool_calls** | Harte Grenze für Tool-Aufrufe | `20` |
| **llm_timeout_seconds** | Zeitlimit für LLM-Aufrufe | `180` |
| **retry_intervals** | Wartezeiten zwischen Wiederholungen | `[10s, 2m, 10m]` |

---

## Sicherheitsvokabular

### Authentifizierung

| Begriff | Beschreibung | Umsetzung |
|---------|--------------|-----------|
| **API Key** | Token für den Zugriff auf den LLM-Dienst | Config-Datei |
| **bcrypt** | Algorithmus zum Passwort-Hashing | Web-UI-Authentifizierung |
| **Master Key** | AES-256-Verschlüsselungsschlüssel | Umgebungsvariable |
| **Session Secret** | Schlüssel zur Cookie-Verschlüsselung | Config-Datei |
| **TOTP** | Zeitbasiertes Einmalpasswort | 2FA-Umsetzung |

### Autorisierung

| Begriff | Beschreibung | Verwendung |
|---------|--------------|------------|
| **Danger Zone** | Schalter für Fähigkeiten | Tools ein-/ausschalten |
| **Guardian** | System zur Sicherheitsprüfung | Pfadvalidierung |
| **Read-Only Mode** | Schreiboperationen einschränken | Pro Integration |
| **User ID** | Einschränkung auf Telegram-/Discord-Nutzer | Zugriffskontrolle |

### Verschlüsselung

| Begriff | Beschreibung | Algorithmus |
|---------|--------------|-------------|
| **AES-256-GCM** | Symmetrische Verschlüsselung | AES im GCM-Modus |
| **Vault** | Verschlüsselter Speicher für Secrets | AES-256-GCM |
| **TLS** | Transportsicherheit | HTTPS-Verbindungen |

### Bedrohungen und Gegenmaßnahmen

| Begriff | Beschreibung | Gegenmaßnahme |
|---------|--------------|---------------|
| **Path Traversal** | Unbefugter Dateizugriff | Guardian-Validierung |
| **Prompt Injection** | Manipulation durch bösartige Prompts | Eingabevalidierung |
| **Rate Limiting** | Schutz vor Missbrauch | Drosselung von Anfragen |
| **Sandbox** | Isolierte Ausführungsumgebung | Python-venv |

---

## Integrationen

### Messenger-Plattformen

| Begriff | Beschreibung | Protokoll |
|---------|--------------|-----------|
| **Bot Token** | Authentifizierung des Bots | Telegram Bot API |
| **Guild ID** | Kennung eines Discord-Servers | Discord API |
| **Channel ID** | Verweis auf einen bestimmten Kanal | Discord API |
| **User ID** | Plattformspezifische Nutzerkennung | Verschiedene |
| **Webhook** | HTTP-Callback-Mechanismus | HTTP POST |

### Smart Home

| Begriff | Beschreibung | Protokoll |
|---------|--------------|-----------|
| **Access Token** | Home-Assistant-Authentifizierung | REST API |
| **Entity ID** | Gerätekennung in Home Assistant | HA-Konventionen |
| **Service Call** | HA-Aktion ausführen | HA WebSocket/REST |
| **State** | Aktueller Zustand eines Geräts | HA-Zustandsmaschine |

### Container-Orchestrierung

| Begriff | Beschreibung | Befehl |
|---------|--------------|--------|
| **Container** | Laufende Instanz eines Images | `docker ps` |
| **Image** | Vorlage für Container | `docker images` |
| **Volume** | Dauerhafter Datenspeicher | `docker volume` |
| **Network** | Kommunikation zwischen Containern | `docker network` |

### Cloud-Speicher

| Begriff | Beschreibung | Dienst |
|---------|--------------|--------|
| **App Password** | Koofr-Authentifizierung | Koofr |
| **Base URL** | Endpunkt des Dienstes | Verschiedene |
| **Mount Point** | Ort des Zugriffs | WebDAV |

---

## Persönlichkeit und Verhalten

### Personality Engine

| Begriff | Beschreibung | Wertebereich |
|---------|--------------|--------------|
| **Trait** | Dauerhaftes Verhaltensmerkmal | 0,0 – 1,0 |
| **Mood** | Vorübergehender Gemütszustand | Verschiedene |
| **Confidence** | Grad der Selbstsicherheit | 0,0 – 1,0 |
| **Curiosity** | Drang zu erkunden und zu lernen | 0,0 – 1,0 |
| **Empathy** | Verständnis für Nutzeremotionen | 0,0 – 1,0 |
| **Thoroughness** | Liebe zum Detail | 0,0 – 1,0 |
| **Creativity** | Originalität der Antworten | 0,0 – 1,0 |

### Stimmungen

| Stimmung | Beschreibung | Auslöser |
|----------|--------------|----------|
| **Curious** | Fragend, erkundend | Fragen, neue Themen |
| **Focused** | Konzentriert, aufgabenorientiert | Komplexe Aufgaben |
| **Creative** | Einfallsreich, unkonventionell | Offene Anfragen |
| **Analytical** | Logisch, detailgenau | Datenanalyse |
| **Cautious** | Vorsichtig, risikoscheu | Fehler, heikle Themen |
| **Playful** | Humorvoll, leicht | Beiläufige Unterhaltung |

### Persönlichkeitsprofile

| Profil | Merkmale | Einsatz |
|--------|----------|---------|
| **Friend** | Locker, warm, gesprächig | Persönliche Assistenz |
| **Professional** | Förmlich, effizient, direkt | Geschäftliche Aufgaben |
| **Neutral** | Ausgewogen, anpassungsfähig | Allgemeiner Einsatz |
| **Punk** | Aufmüpfig, unkonventionell | Kreative Aufgaben |
| **Terminator** | Direkt, knapp, maschinell | Systemadministration |
| **MCP** | Protokollorientiert | Technische Fehlersuche |

---

## Technische Architektur

### Kernkomponenten

| Komponente | Zuständigkeit | Ort im Quellcode |
|------------|---------------|------------------|
| **Agent** | Hauptschleife des Denkens | `internal/agent/` |
| **Server** | HTTP/WebSocket-Verarbeitung | `internal/server/` |
| **Memory** | Alle Gedächtnis-Subsysteme | `internal/memory/` |
| **Tools** | Implementierung der Tools | `internal/tools/` |
| **LLM** | Client für das Sprachmodell | `internal/llm/` |
| **Config** | Konfigurationsverwaltung | `internal/config/` |
| **Security** | Vault und Verschlüsselung | `internal/security/` |
| **Prompts** | Aufbau des System-Prompts | `internal/prompts/` |

### Datenspeicherung

| Speicher | Technik | Zweck |
|----------|---------|-------|
| **Kurzzeitgedächtnis** | SQLite | Gesprächsverlauf |
| **Langzeitgedächtnis** | chromem-go | Vektor-Embeddings |
| **Knowledge Graph** | SQLite | Beziehungen zwischen Entitäten |
| **Vault** | AES-256-GCM-Datei | Verschlüsselte Secrets |
| **Chat History** | JSON-Datei | Zustand der Oberfläche |
| **Config** | YAML-Datei | Einstellungen |

### Laufzeitkonzepte

| Begriff | Beschreibung | Beispiel |
|---------|--------------|----------|
| **Goroutine** | Leichtgewichtiger Thread (Go) | Ausführung von Co-Agenten |
| **Mutex** | Sperre für gegenseitigen Ausschluss | Datenbankzugriff |
| **Context** | Anfragebezogene Werte | Abbruchsignale |
| **Broker** | Verteilung von Ereignissen | SSE-Updates |
| **Registry** | Verfolgung von Tools und Prozessen | Hintergrundprozesse |

### Daten-Speicherorte

| Pfad | Inhalt |
|------|--------|
| `prompts/` | Identity, Regeln, Persönlichkeiten |
| `agent_workspace/skills/` | Python-Skills |
| `agent_workspace/tools/` | Vom Agent erstellte Tools |
| `agent_workspace/workdir/` | Arbeitsverzeichnis (Sandkasten) |
| `data/` | Datenbanken, Vault, Vektordatenbank |
| `log/` | Anwendungs-Logs |

---

## Cross-Referenzen

### Nach Kategorie

#### Gedächtnis-System
- Kurzzeitgedächtnis (STM) → [Gedächtnis-System](#gedächtnis-system)
- Langzeitgedächtnis (LTM) → [Gedächtnis-System](#gedächtnis-system)
- Knowledge Graph → [Gedächtnis-System](#gedächtnis-system)
- Core Memory → [Gedächtnis-System](#gedächtnis-system)
- Vector Search → [Gedächtnisoperationen](#gedächtnisoperationen)

#### Sicherheit
- Vault → [Verschlüsselung](#verschlüsselung)
- Master Key → [Authentifizierung](#authentifizierung)
- TOTP → [Authentifizierung](#authentifizierung)
- Danger Zone → [Autorisierung](#autorisierung)
- AES-256-GCM → [Verschlüsselung](#verschlüsselung)

#### Integrationen
- Telegram → [Messenger-Plattformen](#messenger-plattformen)
- Discord → [Messenger-Plattformen](#messenger-plattformen)
- Home Assistant → [Smart Home](#smart-home)
- Docker → [Container-Orchestrierung](#container-orchestrierung)
- WebDAV → [Cloud-Speicher](#cloud-speicher)

#### Agentenverhalten
- Personality Engine → [Personality Engine](#personality-engine)
- Co-Agent → [Kernkonzepte des Agenten](#kernkonzepte-des-agenten)
- Context Window → [Kernkonzepte des Agenten](#kernkonzepte-des-agenten)
- Tool Call → [Kernkonzepte des Agenten](#kernkonzepte-des-agenten)
- Temperature → [Kernkonzepte des Agenten](#kernkonzepte-des-agenten)

### Synonyme

| Begriff | Auch bekannt als |
|---------|------------------|
| Kurzzeitgedächtnis | STM, Working Memory, Gesprächsverlauf |
| Langzeitgedächtnis | LTM, Vector Memory, RAG Memory |
| System Prompt | Basis-Prompt, Anweisungen, Kontext |
| Tool | Function, Action, Capability |
| Agent Loop | Reasoning Loop, Hauptschleife |
| Vault | Secrets-Speicher, verschlüsselter Speicher |
| Master Key | Verschlüsselungsschlüssel, AURAGO_MASTER_KEY |
| Circuit Breaker | Sicherheitslimits, Zeitlimits |
| Co-Agent | Sub-Agent, Worker, paralleler Agent |

### Abkürzungen auf einen Blick

```text
STM  → Short-Term Memory
LTM  → Long-Term Memory
RAG  → Retrieval-Augmented Generation
LLM  → Large Language Model
API  → Application Programming Interface
TOTP → Time-based One-Time Password
MCP  → Model Context Protocol
SSE  → Server-Sent Events
UI   → User Interface
URL  → Uniform Resource Locator
YAML → YAML Ain't Markup Language
JSON → JavaScript Object Notation
SQL  → Structured Query Language
```

---

## Aktuelle Begriffe

| Begriff | Bedeutung |
|---------|-----------|
| **A2A** | Agent-to-Agent-Protokoll für Aufgaben zwischen kompatiblen KI-Agenten |
| **Agent Card** | Maschinenlesbare Beschreibung eines A2A-Agenten mit Name, Fähigkeiten, Endpunkten und Auth-Anforderungen |
| **File KG Sync** | Hintergrunddienst, der indexierte Dateien in Knowledge-Graph-Entitäten und Beziehungen überführt |
| **Managed Ollama** | Von AuraGo verwalteter Ollama-Docker-Container mit persistentem Modell-Volume und optionaler GPU-Erkennung |
| **Security Proxy** | Verwaltete Caddy-Schutzschicht für öffentlich erreichbare AuraGo-Instanzen |
| **VAPID** | Schlüsselmechanismus für Browser-Web-Push-Benachrichtigungen |
| **Video Generation** | KI-Generierung kurzer Videos aus Text- oder Bildvorgaben |
| **Media Registry** | Datenbank für generierte oder importierte Medien mit Suche, Tags und Metadaten |
| **Mission Preparation** | LLM-gestützte Voranalyse von Missionen vor der Ausführung |
| **Web-Assets** | Versioniertes UI-Paket neben der Binary; ungepinntes Build = nur Recovery |
| **Workspace-Jail** | Datei-Tools bleiben in `agent_workspace`; `config.yaml` und `data/` sind tabu |
| **MeshCore** | Companion-Funk mit Vertrauensmodell und Desktop-Messenger |
| **AgoDesk** | Desktop-Client (Windows/Linux), nicht die Web-UI |
| **Managed Garage** | Von AuraGo verwalteter S3-Speicher für Virtual Computers |
| **Speech Lab** | Lokaler ASR-/TTS-Stack; Chat-Routing nur nach Lab-ASR |
| **Operational Issues** | Sanitized Betriebsprobleme; der Agent benachrichtigt nicht selbst |

---

> [!TIP]
> Viele Begriffe haben im Kontext von AuraGo spezifische Bedeutungen. Bei Unklarheiten immer auch die entsprechenden Kapitel konsultieren.

---

**Vorheriges Kapitel:** [Kapitel 16: Troubleshooting](./16-troubleshooting.md)
**Nächstes Kapitel:** [Kapitel 18: Anhang](./18-appendix.md)

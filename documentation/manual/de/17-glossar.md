# Kapitel 17: Glossar

Dieses Glossar erklärt alle Fachbegriffe, Abkürzungen und Konzepte von AuraGo.

## Abkürzungen und Akronyme

| Abkürzung | Bedeutung | Erklärung |
|-----------|-----------|-----------|
| **AES** | Advanced Encryption Standard | Verschlüsselungsstandard (256-Bit) |
| **API** | Application Programming Interface | Schnittstelle für Programm-zu-Programm-Kommunikation |
| **CB** | Circuit Breaker | Schutzmechanismus gegen Endlosschleifen |
| **GCM** | Galois/Counter Mode | Authentifizierte Verschlüsselung für den Vault |
| **LLM** | Large Language Model | Großes Sprachmodell (KI) |
| **LTM** | Long-Term Memory | Langzeitgedächtnis (Vektorbasiert) |
| **RAG** | Retrieval-Augmented Generation | Abrufgestützte Textgenerierung |
| **SSE** | Server-Sent Events | Technologie für Echtzeit-Updates |
| **STM** | Short-Term Memory | Kurzzeitgedächtnis (Chat-Verlauf) |
| **TOTP** | Time-based One-Time Password | Zeitbasiertes Einmalpasswort (2FA) |
| **YAML** | YAML Ain't Markup Language | Konfigurationsdatei-Format |

## Agent-Terminologie

| Begriff | Bedeutung |
|---------|-----------|
| **Agent Loop** | Die Haupt-Schleife, in der der Agent arbeitet: Empfangen → Verarbeiten → Antworten |
| **Co-Agent** | Paralleler Sub-Agent für komplexe Aufgaben |
| **Function Calling** | LLM-Feature zum Strukturierten Tool-Aufruf |
| **Maintenance Loop** | Nächtlicher Wartungslauf (um 03:00 Uhr) |
| **Native Functions** | OpenAI-kompatible Funktionsaufrufe (vs. Text-Parsing) |
| **Orchestrator** | Der Haupt-Agent, der Co-Agenten verwaltet |
| **Session** | Ein einzelner Chat-Verlauf mit Kontext |
| **System Prompt** | Der initiale Prompt, der das Verhalten des Agents definiert |
| **Temperature** | Kreativitäts-Parameter (0.0 = deterministisch, 1.0 = kreativ) |
| **Token** | Einheit für Textverarbeitung (ca. 0.75 Wörter) |
| **Tool** | Ein ausführbares Werkzeug des Agents |

## Gedächtnis-System

| Begriff | Bedeutung |
|---------|-----------|
| **Chromem** | Die eingebettete Vektor-Datenbank für LTM |
| **Core Memory** | Permanente, wichtige Fakten (immer im Kontext) |
| **Embedding** | Vektorielle Text-Repräsentation für semantische Suche |
| **Entity** | Ein Objekt im Knowledge Graph (Person, Ort, Konzept) |
| **Knowledge Graph** | Graph-basierte Wissensrepräsentation mit Relationen |
| **Note** | Eine strukturierte Notiz mit Kategorie und Priorität |
| **Persistent Summary** | Komprimierte Zusammenfassung alter Konversationen |
| **Relationship** | Verbindung zwischen zwei Entities im Knowledge Graph |
| **Semantic Search** | Bedeutungsbasierte Suche (nicht nur Keywords) |
| **Vector DB** | Datenbank für Embeddings (ChromaDB) |

## Tools und Aktionen

| Tool/Action | Funktion |
|-------------|----------|
| **api_client** | HTTP-Requests an externe APIs |
| **co_agent** | Co-Agenten verwalten (spawn, list, stop) |
| **docker** | Docker-Container und Images verwalten |
| **execute_python** | Python-Code in Sandbox ausführen |
| **execute_shell** | Shell-Befehle ausführen (mit Einschränkungen) |
| **filesystem** | Datei-Operationen (lesen, schreiben, löschen) |
| **home_assistant** | Smart-Home-Geräte steuern |
| **knowledge_graph** | Wissensgraph abfragen/modifizieren |
| **manage_memory** | Speicher verwalten (save, search, delete) |
| **manage_notes** | Notizen und To-Dos verwalten |
| **missions** | Mission Control nutzen |
| **web_search** | Websuche durchführen |

## Konfigurationsbegriffe

| Begriff | Bedeutung |
|---------|-----------|
| **Base URL** | Die API-Endpunkt-Adresse des LLM-Providers |
| **Capability Gate** | Ein-/Ausschalter für bestimmte Fähigkeiten (Danger Zone) |
| **Circuit Breaker** | Sicherheitslimit für Tool-Aufrufe |
| **Embedding Provider** | Dienst für Text-Embeddings (internal/external) |
| **Host** | Bind-Adresse des Webservers |
| **Max Tool Calls** | Maximale Anzahl Tool-Aufrufe pro Anfrage |
| **Model** | Das spezifische LLM (z.B. gpt-4, claude-3-opus) |
| **Provider** | Der LLM-Dienst (OpenRouter, OpenAI, Ollama) |
| **Read-Only Mode** | Nur lesender Zugriff auf Tools |
| **Step Delay** | Pause zwischen Tool-Aufrufen (Rate-Limiting) |
| **Token Budget** | Maximale Token-Anzahl pro Anfrage |

## Sicherheitsvokabular

| Begriff | Bedeutung |
|---------|-----------|
| **AEAD** | Authenticated Encryption with Associated Data |
| **bcrypt** | Passwort-Hashing-Algorithmus |
| **Capability** | Eine spezifische Fähigkeit des Agents (z.B. "shell execution") |
| **Danger Zone** | Bereich für gefährliche Einstellungen |
| **File Lock** | Mechanismus zur Verhinderung paralleler Instanzen |
| **Guardian** | Sicherheitsmodul für Tool-Überprüfung |
| **Master Key** | Der 64-Zeichen Schlüssel für den Vault |
| **Nonce** | Einmalig verwendete Zahl in der Kryptographie |
| **Rate Limiting** | Begrenzung der Anfragen pro Zeiteinheit |
| **Salt** | Zufallswert beim Passwort-Hashing |
| **Vault** | Verschlüsselter Speicher für Secrets |

## Integrationen

| Begriff | Bedeutung |
|---------|-----------|
| **Bot Token** | Authentifizierungs-Token für Telegram/Discord |
| **IMAP** | Internet Message Access Protocol (Email-Empfang) |
| **OAuth** | Open Authorization (Login mit Google/etc.) |
| **OAuth2** | Version 2 von OAuth für API-Zugriff |
| **Scope** | Berechtigungsumfang (z.B. "gmail.readonly") |
| **SMTP** | Simple Mail Transfer Protocol (Email-Versand) |
| **User ID** | Numerische ID eines Telegram/Discord-Nutzers |
| **Webhook** | Callback-URL für Echtzeit-Benachrichtigungen |

## Persönlichkeit und Verhalten

| Begriff | Bedeutung |
|---------|-----------|
| **Core Personality** | Die Basispersönlichkeit (friend, professional, etc.) |
| **Mood** | Aktuelle Stimmung des Agents (curious, focused, etc.) |
| **Personality Engine** | System zur dynamischen Verhaltensanpassung |
| **Temperature** | Parameter für Kreativität/Zufälligkeit |
| **Trait** | Ein Persönlichkeitsmerkmal (curiosity, thoroughness) |
| **User Profiling** | Automatische Analyse der Nutzerpräferenzen |

## Technische Architektur

| Begriff | Bedeutung |
|---------|-----------|
| **Broker** | Event-Verteiler für Echtzeit-Updates |
| **Cron** | Zeitplaner für wiederkehrende Aufgaben |
| **Ephemeral** | Nicht-persistent, temporär (z.B. Co-Agent Memory) |
| **Goroutine** | Leichtgewichtiger Go-Thread |
| **Handler** | Funktion zur Verarbeitung eines HTTP-Requests |
| **Middleware** | Zwischenschicht für HTTP-Verarbeitung |
| **Mux** | HTTP-Request-Router |
| **Registry** | Verwaltungsstruktur (z.B. für Co-Agenten) |
| **Sandbox** | Isolierte Umgebung für Code-Ausführung |
| **WAL** | Write-Ahead Logging (SQLite) |

## Daten-Speicherorte

| Pfad | Inhalt |
|------|--------|
| `agent_workspace/prompts/` | System-Prompts und Persönlichkeiten |
| `agent_workspace/skills/` | Python-Skills |
| `agent_workspace/tools/` | Vom Agent erstellte Tools |
| `agent_workspace/workdir/` | Arbeitsverzeichnis (Sandkasten) |
| `data/` | Datenbanken, Vault, Vektordatenbank |
| `log/` | Anwendungs-Logs |

## Cross-Referenzen

### Synonyme
- **LTM** = Langzeitgedächtnis = Long-Term Memory
- **Co-Agent** = Sub-Agent = Helfer-Agent
- **Vault** = Secrets-Speicher = Verschlüsselter Speicher
- **Circuit Breaker** = Sicherheitslimit = Schutzschalter

### Verwandte Begriffe
- **LLM** → siehe auch: Provider, Model, Token, Temperature
- **Memory** → siehe auch: STM, LTM, Core Memory, Knowledge Graph
- **Security** → siehe auch: Vault, AES, bcrypt, TOTP, Danger Zone
- **Tools** → siehe auch: Action, Function Calling, Capability

## Ergänzte aktuelle Begriffe

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
| **n8n Scope** | Berechtigung, die festlegt, welche AuraGo-Funktionen ein n8n-Workflow nutzen darf |

---

> 💡 **Tipp:** Viele Begriffe haben im Kontext von AuraGo spezifische Bedeutungen. Bei Unklarheiten immer auch die entsprechenden Kapitel konsultieren.

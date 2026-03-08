# AuraGo User Manual - Plan

## Zielgruppen
- **Anfänger**: Erste Schritte, grundlegende Konfiguration, einfache Nutzung
- **Fortgeschrittene**: Feintuning, Integrationen, Workflows
- **Profis**: API-Nutzung, Co-Agents, Personality Engine, Sicherheitskonfiguration

## Ordnerstruktur

```
documentation/
├── manual/
│   ├── PLAN.md (diese Datei)
│   ├── de/
│   │   ├── README.md              # Einstieg & Navigation
│   │   ├── 01-einfuehrung.md      # Was ist AuraGo?
│   │   ├── 02-installation.md     # Installation & Erststart
│   │   ├── 03-schnellstart.md     # Erste Schritte
│   │   ├── 04-webui.md            # Die Web-Oberfläche
│   │   ├── 05-chatgrundlagen.md   # Chatten mit dem Agenten
│   │   ├── 06-tools.md            # Werkzeuge nutzen
│   │   ├── 07-konfiguration.md    # Konfiguration im Detail
│   │   ├── 08-integrations.md     # Integrationen (Telegram, Discord, etc.)
│   │   ├── 09-memory.md           # Gedächtnis & Wissen
│   │   ├── 10-personality.md      # Persönlichkeit anpassen
│   │   ├── 11-missions.md         # Mission Control (Automatisierung)
│   │   ├── 12-invasion.md         # Invasion Control (Deployment)
│   │   ├── 13-dashboard.md        # Dashboard & Analytics
│   │   ├── 14-sicherheit.md       # Sicherheit & Vault
│   │   ├── 15-coagents.md         # Co-Agenten (für Profis)
│   │   ├── 16-troubleshooting.md  # Problemlösung
│   │   ├── 17-glossar.md          # Glossar
│   │   └── 18-anhang.md           # Referenz & Beispiele
│   └── en/
│       ├── README.md
│       ├── 01-introduction.md
│       ├── 02-installation.md
│       ├── 03-quickstart.md
│       ├── 04-webui.md
│       ├── 05-chat-basics.md
│       ├── 06-tools.md
│       ├── 07-configuration.md
│       ├── 08-integrations.md
│       ├── 09-memory.md
│       ├── 10-personality.md
│       ├── 11-missions.md
│       ├── 12-invasion.md
│       ├── 13-dashboard.md
│       ├── 14-security.md
│       ├── 15-coagents.md
│       ├── 16-troubleshooting.md
│       ├── 17-glossary.md
│       └── 18-appendix.md
```

## Kapitelübersicht

### Teil 1: Grundlagen (für alle)

#### 01 - Einführung / Introduction
- Was ist AuraGo? (Konzept, Architektur)
- Für wen ist es gedacht?
- Hauptfeatures (30+ Tools, Web UI, Persönlichkeit)
- Sicherheitshinweise (wichtig!)

#### 02 - Installation
- Systemanforderungen
- Installationsmethoden:
  - One-Liner (Linux/macOS)
  - Docker
  - Manuelle Installation
  - Build from Source
- Erstkonfiguration (Master Key)
- Systemd-Service einrichten
- Deinstallation

#### 03 - Schnellstart
- Die ersten 5 Minuten
- Der Quick Setup Wizard
- Erster Chat
- Grundlegende Befehle (/help, /reset)

#### 04 - Die Web-Oberfläche
- Navigation erklärt
- Header-Elemente
- Radial Menu
- Theme (Dark/Light)
- Mobile Nutzung

#### 05 - Chat-Grundlagen
- Wie kommuniziert man mit AuraGo?
- Nachrichten senden & Empfangen
- Datei-Uploads
- Bildanalyse
- Sprachnachrichten (Telegram)
- Chat-Verlauf verwalten

### Teil 2: Features im Detail

#### 06 - Werkzeuge (Tools)
- Überblick der 30+ Tools
- Tool-Kategorien:
  - Dateisystem & Shell
  - Web & APIs
  - Docker & Proxmox
  - Smart Home (Home Assistant)
  - Google Workspace
  - Email
  - und mehr
- Tools im Chat nutzen
- Ausgabe von Tool-Ergebnissen
- Read-Only vs. Read-Write Modus

#### 07 - Konfiguration
- config.yaml im Detail
- Wichtige Einstellungen:
  - LLM Provider
  - Embedding Modelle
  - Agent-Verhalten
  - Logging
- Änderungen über Web UI
- Änderungen per YAML
- Konfiguration validieren

#### 08 - Integrationen
- Übersicht aller Integrationen
- Telegram Bot Setup
- Discord Bot Setup
- Email (IMAP/SMTP)
- Home Assistant
- Docker
- Webhooks
- Budget Tracking

#### 09 - Gedächtnis & Wissen
- Short-Term Memory (Chat)
- Long-Term Memory (RAG)
- Knowledge Graph
- Core Memory
- Notizen & To-Dos
- Wissen speichern & abrufen
- Speicher optimieren

#### 10 - Persönlichkeit
- Persönlichkeits-Engine V1
- Persönlichkeits-Engine V2
- Vorhandene Persönlichkeiten
- Eigene Persönlichkeiten erstellen
- Mood-Tracking
- User Profiling

### Teil 3: Fortgeschrittene Features

#### 11 - Mission Control
- Was sind Missions?
- Nester & Eggs
- Missionen erstellen
- Scheduling (Cron)
- Manuelles Ausführen
- Monitoring

#### 12 - Invasion Control
- Konzept (Nester/Eggs)
- SSH-Verbindungen
- Docker Deployment
- Remote Agents deployen
- Lifecycle Management

#### 13 - Dashboard
- System-Metriken
- Mood-Verlauf
- Prompt-Builder Analytics
- Budget-Tracking
- Memory-Statistiken

### Teil 4: Für Profis

#### 14 - Sicherheit
- AES-256 Vault
- Web UI Authentifizierung (bcrypt + TOTP)
- Danger Zone (Capability Gates)
- Datei-Locks
- Rate Limiting
- Best Practices

#### 15 - Co-Agenten
- Was sind Co-Agenten?
- Konfiguration
- Spawnen & Verwalten
- Use Cases
- Limitierungen

#### 16 - Troubleshooting
- Häufige Probleme & Lösungen
- Logs lesen
- Debug-Mode
- Support & Community

### Teil 5: Referenz

#### 17 - Glossar
- Alle Fachbegriffe erklärt
- Abkürzungen

#### 18 - Anhang
- Komplette Konfigurationsreferenz
- API Endpoints
- Chat-Befehle-Referenz
- Tool-Referenz
- Beispiel-Konfigurationen
- Update-Historie

## Integrations-Strategie

### Bestehende Dokumentation integrieren:

| Bestehende Datei | Ziel im Manual |
|------------------|----------------|
| `installation.md` | Kapitel 02 + Teile von 03 |
| `configuration.md` | Kapitel 07 + Anhang |
| `docker_installation.md` | Kapitel 02 (Docker-Section) |
| `telegram_setup.md` | Kapitel 08 (Telegram-Section) |
| `google_setup.md` | Kapitel 08 (Google-Section) |
| `docker.md` | Kapitel 08 (Docker-Section) |
| `webdav.md` | Kapitel 08 (WebDAV-Section) |
| `personality_engine_v2.md` | Kapitel 10 |
| `co_agent_concept.md` | Kapitel 15 |
| `compression_to_embeddings_concept.md` | Kapitel 09 (LTM-Section) |

### Mehrsprachigkeit
- Jedes Kapitel als separate Datei
- Gleiche Struktur in DE und EN
- Cross-References relativ (../en/...)

## Schreibstil

### Für Anfänger:
- Schritt-für-Schritt Anleitungen
- Screenshots (später hinzufügen)
- Warnungen & Tipps hervorheben
- Wenig Fachjargon

### Für Profis:
- Technische Details in "Deep Dive"-Boxen
- Konfigurationsbeispiele
- API-Referenzen
- Architektur-Erklärungen

### Formatierungskonventionen:
```markdown
# Kapiteltitel

## Abschnitt

> 💡 **Tipp:** Hilfreiche Information

> ⚠️ **Warnung:** Wichtiger Sicherheitshinweis

> 🔍 **Deep Dive:** Technische Details für Profis

```yaml
# Code-Beispiele
```

| Tabelle | Für Vergleiche |
|---------|---------------|
```

## Nächste Schritte

1. README.md erstellen (Navigation)
2. Kapitel 01-03 erstellen (Einstieg)
3. Kapitel 04-06 erstellen (Grundlagen)
4. Kapitel 07-10 erstellen (Features)
5. Kapitel 11-15 erstellen (Fortgeschritten)
6. Kapitel 16-18 erstellen (Referenz)
7. Review & Korrektur

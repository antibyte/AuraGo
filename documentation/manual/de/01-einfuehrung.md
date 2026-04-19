# Kapitel 1: Einführung

Willkommen bei AuraGo – deinem persönlichen, autonomen KI-Agenten.

## Was ist AuraGo?

AuraGo ist ein vollständig autonomer KI-Agent, geschrieben in Go und als einzelne portable Binary ausgeliefert. Anders als einfache Chatbots kann AuraGo aktiv handeln:

- **🧠 Denken & Planen** — Multi-Step Reasoning mit automatischer Fehlerbehebung
- **💻 Code ausführen** — Python und Shell-Befehle in isolierter Umgebung
- **📁 Dateien verwalten** — Lesen, schreiben, organisieren
- **🏠 Smart-Home steuern** — Home Assistant, Chromecast, Netzwerkgeräte
- **📧 Kommunizieren** — E-Mails, Telegram, Discord, SMS/Voice
- **🧠 Sich alles merken** — Kurz- und Langzeitgedächtnis mit semantischer Suche
- **🔄 Sich selbst verbessern** — Eigene Quellcode-Modifikationen
- **⚡ Parallele Aufgaben** — Co-Agenten für komplexe Workflows

### Die Kernidee

Stell dir einen persönlichen Assistenten vor, der:

| Eigenschaft | Beschreibung |
|-------------|--------------|
| **Verfügbar ist** | 24/7 über Web, Telegram, Discord oder E-Mail |
| **Kontext hat** | Er erinnert sich an alle vorherigen Gespräche und Fakten |
| **Handelt** | Er führt Aufgaben aus, nicht nur Antworten gibt |
| **Sich anpasst** | Seine Persönlichkeit entwickelt sich mit der Zeit |
| **Sicher ist** | AES-256-Verschlüsselung, Vault-System, Zugriffskontrolle |

## Für wen ist AuraGo gedacht?

| Profil | Nutzung |
|--------|---------|
| **🏠 Privatanwender** | Persönlicher Assistent für Alltagsaufgaben, Recherche, Organisation |
| **👨‍💻 Entwickler** | Code-Reviews, Automatisierung, Systemverwaltung, API-Tests |
| **🖥️ Systemadministratoren** | Server-Monitoring, Docker-Management, Backup-Automatisierung |
| **🏡 Smart-Home-Enthusiasten** | Zentrale Steuerung aller Geräte, Automationen |
| **🔬 KI-Forscher** | Experimente mit Personality Engines, Co-Agenten, Memory-Systemen |

## Hauptfeatures im Überblick

### 🤖 Agent Core
- **90+ eingebaute Tools** — Von Dateisystem bis Docker, von WebDAV bis Proxmox
- **Native Function Calling** — OpenAI-kompatible Tool-Aufrufe
- **Dynamische Tool-Erstellung** — Der Agent kann neue Python-Tools zur Laufzeit schreiben
- **Multi-Step Reasoning** — Automatische Werkzeug-Dispatch, Fehlerbehebung
- **Co-Agent System** — Parallele Sub-Agenten für komplexe Aufgaben
- **Adaptive Tools** — Intelligente Tool-Filterung spart Tokens

### 🧠 Memory & Knowledge
- **Short-Term Memory** — SQLite-basierte Konversationshistorie
- **Long-Term Memory (RAG)** — Vektorbasierte semantische Suche
- **Knowledge Graph** — Strukturierte Entitäten und Beziehungen
- **Core Memory** — Permanente Fakten, die der Agent immer behält
- **Notizen & To-Dos** — Kategorisiert, priorisiert, mit Fälligkeitsdaten
- **Journal** — Chronologisches Ereignisprotokoll

### 🎭 Persönlichkeit
- **Personality Engine V2** — LLM-basierte Stimmungs- und Verhaltensanalyse
- **User Profiling** — Automatische Erkennung deiner Präferenzen
- **Eingebaute Persönlichkeiten** — Freund, Profi, Punk, Neutral, Terminator und mehr
- **Eigenes Profil** — Erstelle deine eigenen Persönlichkeiten

### 🛡️ Sicherheit
- **AES-256-GCM Vault** — Verschlüsselte Speicherung aller API-Keys
- **Web UI Auth** — Optional mit bcrypt-Passwort und TOTP 2FA
- **LLM Guardian** — KI-basierte Überwachung aller Tool-Aufrufe
- **Danger Zone** — Granulare Kontrolle über Fähigkeiten
- **Sandboxing** — Python läuft in isolierter venv oder Docker

### 🔌 Integrationen
- **Web UI** — Vollständige Chat-Oberfläche mit Dashboard
- **Telegram** — Sprachnachrichten, Bildanalyse, Inline-Befehle
- **Discord** — Bot-Integration mit Nachrichten-Bridge
- **E-Mail** — IMAP-Monitoring + SMTP-Versand
- **Home Assistant** — Smart-Home-Steuerung
- **Docker & Proxmox** — Container- und VM-Management
- **Google Workspace** — Gmail, Kalender, Drive, Docs

## Architektur kurz erklärt

```
┌─────────────────────────────────────────────────────────┐
│  User Interfaces                                        │
│  (Web UI / Telegram / Discord / Email)                 │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  AuraGo Core                                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   Agent     │  │   Memory    │  │   Tools     │     │
│  │   Loop      │  │   System    │  │   (90+)     │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │ Personality │  │   Vault     │  │   LLM       │     │
│  │   Engine    │  │ (AES-256)   │  │  Guardian   │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│  LLM Provider (OpenAI-kompatibel)                       │
│  OpenRouter, Ollama, OpenAI, etc.                      │
└─────────────────────────────────────────────────────────┘
```

## Wichtige Sicherheitshinweise

> ⚠️ **Kritisch: Isolierte Umgebung**
> AuraGo führt Code auf deinem System aus. Es wird **dringend empfohlen**, AuraGo in einer isolierten Umgebung zu betreiben:
> - Virtuelle Maschine
> - Docker-Container
> - Dedizierter PC/Server
>
> Fehler des LLMs oder falsch konfigurierte Prompts können unbeabsichtigte Auswirkungen haben.

> ⚠️ **Niemals ungeschützt exponieren**
> Die Web-UI sollte niemals direkt aus dem Internet erreichbar sein. Nutze immer:
> - VPN (WireGuard, Tailscale)
> - Reverse Proxy mit Authentifizierung
> - Firewall-Regeln
> - Oder die integrierte Auth mit 2FA

## Nächste Schritte

1. **[Installation](02-installation.md)** – AuraGo auf deinem System einrichten
2. **[Schnellstart](03-schnellstart.md)** – Die ersten 5 Minuten mit AuraGo
3. **[Chat-Grundlagen](05-chatgrundlagen.md)** – Effektiv kommunizieren

---

> 💡 **Tipp für Neulinge:** Starte mit der Web-UI und einem einfachen Chat. Du wirst überrascht sein, wie intuitiv die Bedienung ist!

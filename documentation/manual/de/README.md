# AuraGo Benutzerhandbuch

Willkommen zum AuraGo Benutzerhandbuch – deiner umfassenden Anleitung für den persönlichen KI-Agenten.

> 📅 **Stand:** 26. April 2026
> 🔄 **Version:** 2.x kompatibel  
> 📝 **Letzte Aktualisierung:** Dokumentations-Sync mit aktuellem Code-Stand (Chat-Commands, Tools, Integrationen, Config)

---

## Was ist AuraGo?

AuraGo ist ein vollständig autonomer KI-Agent, der als einzelne portable Binary mit eingebetteter Web-Oberfläche ausgeliefert wird. Verbinde ihn mit einem beliebigen OpenAI-kompatiblen LLM-Provider und er wird zu einem persönlichen Assistenten, der Code ausführen, Dateien verwalten, Smart-Home-Geräte steuern, E-Mails senden, sich alles merken und sogar seinen eigenen Quellcode verbessern kann.

### Highlights

| Feature | Beschreibung |
|---------|--------------|
| **🧠 Personality Engine V2** | Lernt deine Präferenzen und passt sich an |
| **🛡️ LLM Guardian** | KI-basierte Sicherheitsüberwachung |
| **⚡ Adaptive Tools** | Intelligente Tool-Filterung spart Tokens |
| **📄 Document AI** | PDF-Erstellung und -Analyse |
| **🎬 Video Generation** | KI-Text-zu-Video und Bild-zu-Video-Generierung |
| **🔐 AES-256 Vault** | Sichere Speicherung aller Secrets |
| **🌐 50+ Integrationen** | Von S3 über OneDrive bis TrueNAS |
| **☁️ Cloudflare Tunnel** | Sicherer Remote-Zugriff ohne öffentliche IP |
| **🗄️ SQL Connections** | Direkte Datenbank-Abfragen (PostgreSQL, MySQL) |
| **📱 Chromecast** | TTS und Medien an Cast-Geräte senden |
| **🔍 Netzwerk-Tools** | Ping, Port-Scan, mDNS/UPnP Discovery |
| **📱 PWA & Mobile** | Installierbar als PWA mit Sprachsteuerung und TTS |
| **🎨 Themes** | Wähle aus Cyberwar, Retro CRT, Dark Sun oder Lollipop |
| **📊 YepAPI** | SEO, SERP, Scraping, YouTube/TikTok/Instagram/Amazon-Daten |
| **🗄️ Inventar + WOL** | Geräteregistrierung mit Wake-on-LAN |
| **⏰ Heartbeat** | Hintergrund-Wake-up-Scheduler für den Agenten |
| **🧠 Knowledge Graph** | LLM-basierte Entity-Extraktion aus Konversationen |
| **📦 Browser Automation** | Headless Chrome für Formulare und Screenshots |
| **📝 Obsidian** | Anbindung an deinen persönlichen Knowledge Vault |
| **🔧 Output Compression** | Token-sparende Deduplizierung von Tool-Ausgaben |

---

## Für wen ist dieses Handbuch?

| Wenn du... | Starte mit... |
|------------|---------------|
| Neu bei AuraGo bist | [Kapitel 1: Einführung](01-einfuehrung.md) → [Kapitel 2: Installation](02-installation.md) |
| Schnell loslegen willst | [Kapitel 3: Schnellstart](03-schnellstart.md) |
| Die Oberfläche verstehen willst | [Kapitel 4: Web-Oberfläche](04-webui.md) |
| Mehr über Features erfahren willst | [Kapitel 6: Werkzeuge](06-tools.md) |
| Fortgeschrittene Themen suchst | [Kapitel 11-15](11-missions.md) |
| Ein Problem hast | [Kapitel 16: Troubleshooting](16-troubleshooting.md) |

---

## Screenshots

AuraGo hat einige eingebaute Themes zur Auswahl:

| Cyberwar | Retro CRT | Dark Sun | Lollipop |
|:--------:|:---------:|:--------:|:--------:|
| ![Cyberwar](../../screenshots/theme1.png) | ![Retro CRT](../../screenshots/theme2.png) | ![Dark Sun](../../screenshots/theme3.png) | ![Lollipop](../../screenshots/theme4.png) |

### Haupt-Interface Screenshots

| Chat-Interface | Dashboard |
|----------------|-----------|
| ![Chat](../../screenshots/chat.png) | ![Dashboard](../../screenshots/dashboard.png) |

| Konfiguration | Container |
|---------------|------------|
| ![Config](../../screenshots/config.png) | ![Container](../../screenshots/containers.png) |

---

## Handbuch-Struktur

### Teil 1: Grundlagen
1. [Einführung](01-einfuehrung.md) – Was ist AuraGo?
2. [Installation](02-installation.md) – System einrichten
3. [Schnellstart](03-schnellstart.md) – Die ersten 5 Minuten
4. [Web-Oberfläche](04-webui.md) – Navigation & UI
5. [Chat-Grundlagen](05-chatgrundlagen.md) – Kommunikation

### Teil 2: Features im Detail
6. [Werkzeuge](06-tools.md) – 100+ Tools nutzen
7. [Konfiguration](07-konfiguration.md) – Feintuning mit Provider-System
8. [Integrationen](08-integrations.md) – Telegram, Discord, Email, etc.
9. [Gedächtnis & Wissen](09-memory.md) – Speicher verstehen
10. [Persönlichkeit](10-personality.md) – Charakter anpassen

### Teil 3: Fortgeschritten (Web-UI/API)
11. [Mission Control](11-missions.md) – Automatisierung
12. [Invasion Control](12-invasion.md) – Remote Deployment
13. [Dashboard](13-dashboard.md) – Analytics & Metriken

### Teil 4: Für Profis
14. [Sicherheit](14-sicherheit.md) – Vault, Auth, Best Practices
15. [Co-Agenten](15-coagents.md) – Parallele Agenten
16. [Troubleshooting](16-troubleshooting.md) – Problemlösung
17. [Glossar](17-glossar.md) – Begriffe erklärt
18. [Anhang](18-anhang.md) – Referenzmaterial
19. [Skills](19-skills.md) – Eigene Python-Skills erstellen

### Teil 5: Referenz
20. [Chat-Commands](20-chat-commands.md) – Alle verfügbaren Chat-Befehle
21. [API Referenz](21-api-reference.md) – Vollständige REST API Dokumentation
22. [Interne Tools](22-interne-tools.md) – Alle 100+ internen Agent-Tools

### Teil 6: Interna
23. [Interna](23-interna.md) – Architektur, Module und interne Arbeitsweise

---

## Wichtige Hinweise

### ⚠️ CLI vs. Web-UI

Einige fortgeschrittene Features (Mission Control, Invasion Control) sind **primär über die Web-UI und REST API** verfügbar. CLI-Befehle dafür existieren in der aktuellen Version nicht.

### 🆕 Provider-System (Neu in 2.x)

Die Konfiguration verwendet jetzt ein zentrales Provider-System für LLM-Verbindungen. Siehe [Kapitel 7: Konfiguration](07-konfiguration.md).

### 🔒 Sicherheit

> **Wichtig:** AuraGo kann beliebige Shell-Befehle ausführen und Systemdateien ändern. Exponiere die Web-UI niemals ungeschützt im Internet. Verwende immer VPN, Reverse Proxy mit Authentifizierung oder Firewall-Regeln.

---

## Schnell-Navigation

### Die wichtigsten Befehle im Chat
```
/help          - Alle Befehle anzeigen
/reset         - Chat-Verlauf löschen
/stop          - Aktuelle Aktion abbrechen
/restart       - AuraGo neu starten
/debug on/off  - Debug-Modus umschalten
/budget        - Kostenübersicht anzeigen
/personality   - Persönlichkeit wechseln
/voice         - Sprachausgabe ein-/ausschalten
/warnings      - System-Warnungen anzeigen
/sudopwd       - Sudo-Passwort im Vault speichern
/addssh        - SSH-Server registrieren
/credits       - OpenRouter Credits anzeigen
```

### Alle Agent-Tools
Eine vollständige Übersicht aller 100+ internen Tools findest du im Abschnitt [Interne Tools](22-interne-tools.md). Darüber hinaus können dynamisch weitere Python-Skills und benutzerdefinierte Tools hinzugefügt werden.

### Schnell-Links
- [Handbuch-Startseite](../README.md)
- [FAQ](faq.md)
- [Vollständige Konfigurationsreferenz](../../configuration.md)
- [Telegram-Einrichtung](../../telegram_setup.md)
- [Docker-Installationsguide](../../docker_installation.md)

---

## Aktualisierungen

| Datum | Änderung |
|-------|----------|
| 2026-03 | Überarbeitung für Version 2.x (Provider-System, Tool-Dokumentation, LLM Guardian) |
| 2026-03 | Adaptive Tools Dokumentation hinzugefügt |
| 2026-03 | Dokument Creator & PDF Extractor hinzugefügt |
| 2026-03 | **SQL Connections, OneDrive, S3, Homepage Integrationen** dokumentiert |
| 2026-03 | **Cloudflare Tunnel, AI Gateway, Chromecast** hinzugefügt |
| 2026-03 | **Netzwerk-Tools, Web Capture, Form Automation** dokumentiert |
| 2026-03 | **Skill Manager, Media Registry, Egg Mode** ergänzt |
| 2026-04 | **Kapitel 23: Interna** – Architektur, Module und interne Arbeitsweise dokumentiert |
| 2026-04 | Dokumentations-Sync mit aktuellem Code-Stand: Chat-Commands ergänzt (/voice, /warnings), interne Tools bereinigt, Integrationen korrigiert, Config-Referenzen aktualisiert |
| 2026-04 | Video Generation, send_video, LDAP, n8n-Scopes, A2A-Nutzung, Web Push, Managed Ollama, File KG Sync, Backup/Restore, Mission Preparation und Security-Proxy-API dokumentiert |
| 2026-04 | YepAPI, Inventar/WOL, Heartbeat, Knowledge Graph Extraction, Browser Automation, Obsidian, Output Compression zu Integrations-Kapitel hinzugefügt |
| 2026-03 | **Chat-Commands /sudopwd** hinzugefügt |

---

*Dieses Handbuch wird kontinuierlich aktualisiert. Die englische Version findest du [hier](../en/README.md).*

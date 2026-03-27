# Kapitel 6: Werkzeuge

AuraGo verfügt über **50+ eingebaute Werkzeuge**, die ihn von einem einfachen Chatbot zu einem autonomen Agenten machen.

---

## Tool-Kategorien im Überblick

| Kategorie | Tools | Danger Zone |
|-----------|-------|-------------|
| **🗂️ Dateisystem** | Dateien lesen, schreiben, löschen | Ja |
| **🌐 Web & APIs** | Suche, HTTP, Scraping | Nein (teilweise) |
| **🐳 Docker** | Container, Images, Netzwerke | Ja |
| **🖥️ Proxmox** | VMs, LXCs, Snapshots | Ja |
| **🏠 Smart Home** | Home Assistant, MQTT, Wake-on-LAN | Ja |
| **☁️ Cloud** | Google Workspace, WebDAV, GitHub | Nein (teilweise) |
| **📧 Kommunikation** | E-Mail, Telegram, Discord | Nein |
| **🔧 System** | Metriken, Prozesse, Cron | Teilweise |
| **🧠 Memory** | Gedächtnis, Notizen, Knowledge Graph | Nein |

---

## Neue Plattform-Features

Die aktuelle Version enthält mehrere leistungsstarke Erweiterungen:

| Feature | Beschreibung |
|---------|--------------|
| **LLM Guardian** | Risiko-Scanning von Tool-Calls und externen Inhalten vor der Ausführung |
| **Adaptive Tools** | Token-Optimierung durch kontextabhängige Tool-Auswahl |
| **Document Creator** | PDF-Erstellung für Rechnungen, Berichte |
| **PDF Extractor** | Text-Extraktion mit optionaler LLM-Zusammenfassung |
| **MCP Client/Server** | Model Context Protocol für Interoperabilität |
| **Invasion Control** | Verteilte Orchestrierung über mehrere Hosts |
| **Sudo-Execution** | Vault-gestütztes Credential-Handling für privilegierte Befehle |

---

## Konfiguration der wichtigsten Tools

### 1. Dateisystem & Shell

```yaml
agent:
  allow_shell: true              # Shell-Befehle erlauben
  allow_python: true             # Python-Ausführung erlauben
  allow_filesystem_write: true   # Datei-Schreibzugriff
```

### 2. Docker

```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
  # Oder für remote Docker:
  # host: "tcp://docker-host:2376"
```

### 3. Proxmox

```yaml
proxmox:
  enabled: true
  url: "https://proxmox.example.com:8006"
  token_id: "root@pam!aurago"
  token_secret: ""              # Wird im Vault gespeichert
  node: "pve"
  insecure: false               # true = unsichere TLS akzeptieren
```

### 4. Home Assistant

```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  # AccessToken wird im Vault gespeichert
  readonly: false               # true = nur lesen
```

### 5. Google Workspace (OAuth2)

```yaml
agent:
  enable_google_workspace: true
```

Die Authentifizierung erfolgt über OAuth2 im Vault-Menü der Web-UI.

### 6. E-Mail

```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  imap_port: 993
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  username: "dein.email@gmail.com"
  watch_enabled: true
  watch_interval_seconds: 120
```

### 7. Web-Suche

```yaml
# DuckDuckGo (kein API-Key nötig)
# Standardmäßig aktiviert

# Brave Search (optional, bessere Ergebnisse)
brave_search:
  enabled: true
  api_key: "BS..."
  country: "DE"
  lang: "de"
```

---

## Adaptive Tools – Intelligente Filterung

**Spare Tokens, bleib fokussiert.** Das Adaptive Tools System filtert Tools basierend auf dem Gesprächskontext:

```yaml
agent:
  adaptive_tools:
    enabled: true
    max_tools: 20               # Max. Anzahl Tools im Kontext
    
    # Immer verfügbar (nicht filtern):
    always_include:
      - "filesystem"
      - "shell"
      - "query_memory"
```

| Aspekt | Ohne Adaptive | Mit Adaptive |
|--------|---------------|--------------|
| **Tokens** | 50+ Tools im Prompt | Nur relevante Tools |
| **Kosten** | Höher | Niedriger |
| **Genauigkeit** | LLM überfordert | Präzisere Tool-Wahl |

---

## Read-Only vs. Read-Write

Viele Tools unterstützen einen Read-Only-Modus:

```yaml
tools:
  docker:
    enabled: true
    readonly: true        # Nur Listen/Logs, kein Start/Stop
  home_assistant:
    enabled: true
    readonly: true        # Nur Status abfragen, nicht steuern
```

---

## Danger Zone

Die Danger Zone kontrolliert potenziell gefährliche Operationen:

```yaml
agent:
  allow_shell: true              # Shell-Befehle
  allow_python: true             # Python-Ausführung
  allow_filesystem_write: true   # Datei-Schreibzugriff
  allow_network_requests: true   # HTTP-Anfragen
  allow_remote_shell: true       # SSH auf Remote-Geräte
  allow_self_update: true        # Selbst-Updates
  allow_mcp: true                # MCP-Protokoll
  allow_web_scraper: true        # Web-Scraping
```

> ⚠️ **Tipp:** In Produktionsumgebungen nur das aktivieren, was wirklich benötigt wird.

---

## Beispiele im Chat

### Dateisystem
```
Du: Liste alle Dateien im Projektordner
Agent: 📁 Projektordner:
       ├── main.go
       ├── go.mod
       └── config.yaml

Du: Erstelle eine Datei notes.txt mit "Wichtige Ideen"
Agent: ✅ Datei erstellt: notes.txt
```

### Docker
```
Du: Zeige alle laufenden Container
Agent: 🐳 Laufende Container:
       ┌─────────┬────────┬─────────────┐
       │ NAME    │ STATUS │ PORTS       │
       ├─────────┼────────┼─────────────┤
       │ nginx   │ Up 2h  │ 80→8080/tcp │
       │ postgres│ Up 2h  │ 5432/tcp    │
       └─────────┴────────┴─────────────┘

Du: Starte den redis Container neu
Agent: 🔄 Container redis wird neu gestartet...
       ✅ Erfolgreich (3.2s)
```

### Home Assistant
```
Du: Schalte das Wohnzimmerlicht aus
Agent: 🏠 Smart Home:
       ✅ Wohnzimmer Licht: AUS

Du: Wie warm ist es im Schlafzimmer?
Agent: 🌡️ Schlafzimmer-Sensor:
       Temperatur: 21.5°C
       Luftfeuchtigkeit: 45%
```

### Web-Suche
```
Du: Suche nach aktuellen Go Best Practices
Agent: 🔍 Suche läuft...
       Gefunden: 5 Ergebnisse
       1. Go Code Review Comments
       2. Effective Go
       ...
```

---

## Eigene Tools erstellen

AuraGo kann zur Laufzeit neue Python-Tools erstellen:

```
Du: Erstelle ein Tool, das Temperaturen umrechnet
Agent: 🛠️ Erstelle Temperatur-Konverter...
       ✅ Erstellt: temperature_converter.py
       
Du: Wie viel sind 25°C in Fahrenheit?
Agent: 🌡️ 25°C = 77°F
```

Erstellte Tools werden in `agent_workspace/tools/` gespeichert und sind sofort verfügbar.

---

## Zusammenfassung

| Kategorie | Highlights |
|-----------|------------|
| **50+ Tools** | Für nahezu jede Home-Lab-Aufgabe |
| **Sicherheit** | Read-Only-Modus, Danger Zone, LLM Guardian |
| **Flexibilität** | Dynamische Tool-Erstellung zur Laufzeit |
| **Effizienz** | Adaptive Tools sparen Tokens |

> 💡 **Merke:** AuraGos Stärke liegt in der Kombination von Tools. Ein einzelner Chat kann Shell-Befehle, Web-Suchen, Docker-Operationen und E-Mail-Benachrichtigungen verketten – vollautomatisch!

---

**Nächste Schritte**

- **[Konfiguration](07-konfiguration.md)** — Tools feinjustieren
- **[Integrationen](08-integrations.md)** — Externe Dienste verbinden

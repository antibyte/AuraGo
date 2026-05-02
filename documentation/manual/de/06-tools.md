# Kapitel 6: Werkzeuge

AuraGo verfügt über **100+ eingebaute Werkzeuge**, die ihn von einem einfachen Chatbot zu einem autonomen Agenten machen.

---

## Tool-Kategorien im Überblick

| Kategorie | Tools | Danger Zone |
|-----------|-------|-------------|
| **🗂️ Dateisystem** | Dateien lesen, schreiben, löschen | Ja |
| **🌐 Web & APIs** | Suche, HTTP, Scraping, Screenshots | Nein (teilweise) |
| **🐳 Docker** | Container, Images, Netzwerke | Ja |
| **🖥️ Proxmox** | VMs, LXCs, Snapshots | Ja |
| **🏠 Smart Home** | Home Assistant, MQTT, Wake-on-LAN | Ja |
| **☁️ Cloud** | Google Workspace, WebDAV, GitHub, S3, OneDrive | Nein (teilweise) |
| **📧 Kommunikation** | E-Mail, Telegram, Discord, Telnyx | Nein |
| **🎬 Medien-Generierung** | Bilder, Musik, Videos, TTS, Media Registry | Nein (Provider-Limits gelten) |
| **🔧 System** | Metriken, Prozesse, Cron, Netzwerk-Tools | Teilweise |
| **🧠 Memory** | Gedächtnis, Notizen, Knowledge Graph | Nein |
| **🌐 Netzwerk** | Ping, Port-Scan, mDNS, UPnP | Nein |
| **🖥️ Remote** | SSH, Invasion Control, MeshCentral | Ja |
| **📝 Dokumente** | PDF Creator/Extractor, Paperless NGX | Nein |
| **🎬 Medien-Konvertierung** | FFmpeg, ImageMagick, Video-Download | Nein |

---

## Neue Plattform-Features

Die aktuelle Version enthält mehrere leistungsstarke Erweiterungen:

| Feature | Beschreibung |
|---------|--------------|
| **LLM Guardian** | Risiko-Scanning von Tool-Calls und externen Inhalten vor der Ausführung |
| **Adaptive Tools** | Token-Optimierung durch kontextabhängige Tool-Auswahl |
| **Document Creator** | PDF-Erstellung für Rechnungen, Berichte |
| **PDF Extractor** | Text-Extraktion mit optionaler LLM-Zusammenfassung |
| **Video Generation** | KI-Text-zu-Video und Bild-zu-Video mit MiniMax Hailuo oder Google Veo |
| **Media Registry** | Generierte Bilder, Audio, Musik und Videos suchen, taggen und wiederverwenden |
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
google_workspace:
  enabled: true
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

### 8. Medien-Generierung

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
  provider: "minimax"      # oder "google"
  model: "hailuo-02"       # provider-spezifisch
  max_daily: 5
```

Die zugehörigen Tools sind `generate_image`, `generate_music` und `generate_video`. Generierte Dateien werden lokal gespeichert und in der Media Registry registriert, damit sie später gesucht, getaggt, erneut gesendet oder weiterverwendet werden können.

---

## Adaptive Tools – Intelligente Filterung

**Spare Tokens, bleib fokussiert.** Das Adaptive Tools System filtert Tools basierend auf dem Gesprächskontext:

```yaml
agent:
  adaptive_tools:
    enabled: true
    max_tools: 16               # Max. Anzahl Tools im Kontext (Standard: 16)
    
    # Immer verfügbar (nicht filtern):
    always_include:
      - "filesystem"
      - "file_editor"
      - "execute_shell"
      - "manage_memory"
      - "query_memory"
      - "execute_python"
      - "docker"
      - "api_request"
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

### Video Generation
```
Du: Erstelle ein kurzes 16:9-Video von einem Sonnenaufgang über einem futuristischen Home-Lab
Agent: 🎬 Video wird mit dem konfigurierten Provider generiert...
       ✅ Video erstellt und in der Media Registry registriert

Du: Sende mir das generierte Video
Agent: ▶️ Video mit Inline-Player angehängt
```

---

## Netzwerk-Tools

Diagnose-Tools für Netzwerk-Scanning und -Überwachung.

### Konfiguration

```yaml
tools:
  network_ping:
    enabled: true                 # ICMP Ping und Port-Scanner
  network_scan:
    enabled: true                 # mDNS/Bonjour Discovery
  upnp_scan:
    enabled: true                 # UPnP/SSDP Geräte-Discovery
```

### Verfügbare Tools

| Tool | Beschreibung |
|------|--------------|
| `network_ping` | ICMP Ping zu einem Host |
| `port_scanner` | TCP-Port-Scan auf einem Host |
| `mdns_scan` | mDNS/Bonjour Geräte im LAN finden |
| `upnp_scan` | UPnP/SSDP Geräte im LAN finden |

### Beispiele im Chat

```
Ping google.com
Scanne die Ports auf 192.168.1.1
Finde alle Geräte im lokalen Netzwerk
Welche UPnP-Geräte sind verfügbar?
```

---

## Web Capture & Form Automation

Screenshots, PDF-Generierung und Browser-Automatisierung.

### Konfiguration

```yaml
tools:
  web_capture:
    enabled: true                 # Screenshots und PDF von Webseiten
  form_automation:
    enabled: false                # Formular-Automatisierung (erfordert web_capture)
```

### Verfügbare Tools

| Tool | Beschreibung |
|------|--------------|
| `web_capture` | Screenshot oder PDF einer Webseite erstellen |
| `web_performance_audit` | Core Web Vitals messen (TTFB, FCP, LCP) |
| `form_automation` | Formular automatisch ausfüllen und absenden |

### Anforderungen

- Headless Chromium (wird bei Bedarf automatisch gestartet)
- Mehr RAM für große Seiten

### Beispiele im Chat

```
Erstelle einen Screenshot von google.com
Speichere die Dokumentation als PDF
Fülle das Kontaktformular auf example.com aus
```

---

## Medien-Konvertierung & Video-Download

Konvertierung von Audio-, Video- und Bilddateien sowie Video-Download von Plattformen wie YouTube.

### Konfiguration

```yaml
tools:
  media_conversion:
    enabled: true
    ffmpeg_path: ""
    imagemagick_path: ""
  video_download:
    enabled: true
    mode: "docker"                # docker oder native
    download_dir: "data/downloads"
    allow_transcribe: false
  send_youtube_video:
    enabled: true
```

### Verfügbare Tools

| Tool | Beschreibung |
|------|--------------|
| `media_conversion` | Dateien zwischen Formaten konvertieren (FFmpeg/ImageMagick) |
| `video_download` | Videos von YouTube und anderen Plattformen herunterladen |
| `send_youtube_video` | YouTube-Videos als eingebetteten Player senden |

### Anforderungen

- FFmpeg und/oder ImageMagick (systemweit installiert oder Pfade konfiguriert)
- Für Video-Download: yt-dlp (im Docker-Container oder systemweit)

### Beispiele im Chat

```
Konvertiere video.mp4 in audio.mp3
Lade das YouTube-Video herunter
Sende mir das YouTube-Video als eingebetteten Player
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
| **100+ Tools** | Für nahezu jede Home-Lab-Aufgabe |
| **Sicherheit** | Read-Only-Modus, Danger Zone, LLM Guardian |
| **Flexibilität** | Dynamische Tool-Erstellung zur Laufzeit |
| **Effizienz** | Adaptive Tools sparen Tokens |
| **Medien** | Bilder, Musik, Audio, Dokumente und Videos generieren und ausliefern |

> 💡 **Merke:** AuraGos Stärke liegt in der Kombination von Tools. Ein einzelner Chat kann Shell-Befehle, Web-Suchen, Docker-Operationen und E-Mail-Benachrichtigungen verketten – vollautomatisch!

---

**Nächste Schritte**

- **[Konfiguration](07-konfiguration.md)** — Tools feinjustieren
- **[Integrationen](08-integrations.md)** — Externe Dienste verbinden

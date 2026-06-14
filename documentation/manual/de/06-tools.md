# Kapitel 6: Werkzeuge

AuraGo verfügt über **100+ eingebaute Werkzeuge**, die ihn von einem einfachen Chatbot zu einem autonomen Agenten machen.

---

## Tool-Kategorien im Überblick

| Kategorie | Tools | Danger Zone |
|-----------|-------|-------------|
| **🗂️ Dateisystem** | Dateien lesen, schreiben, löschen | Ja |
| **🌐 Web & APIs** | Suche, HTTP, Scraping, Screenshots | Nein (teilweise) |
| **🌐 Web & Sites** | Homepage scaffold, Build, Deploy, Registry | Nein (`homepage.enabled`) |
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
| **Homepage / Website-Projekte** | Docker-Dev-Workspace, fokussierte Tools, Registry & Historie |
| **Sudo-Execution** | Vault-gestütztes Credential-Handling für privilegierte Befehle |

---

## Konfiguration der wichtigsten Tools

> 💡 **Tipp:** Die bevorzugte Konfiguration erfolgt über die **Web-UI**. Die YAML-Beispiele unten sind eine alternative Referenz für Docker-Deployments und Git-Ops.

### 1. Dateisystem & Shell

### Einrichtung in der Web-UI
1. Öffne **Config → Gefahrenzone**.
2. Aktiviere bei Bedarf **Shell**, **Python** und **Dateisystem-Schreibzugriff**.
3. Speichern — ein Neustart kann erforderlich sein.

### YAML-Referenz
```yaml
agent:
  allow_shell: true              # Shell-Befehle erlauben
  allow_python: true             # Python-Ausführung erlauben
  allow_filesystem_write: true   # Datei-Schreibzugriff
```

### 2. Docker

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Docker**.
2. Aktiviere die Integration und trage die Host-URL ein (z. B. `unix:///var/run/docker.sock`).
3. Optional: **Nur-Lesen** für sicheres Monitoring aktivieren.
4. Speichern und bei Bedarf neu starten.

### YAML-Referenz
```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
  # Oder für remote Docker:
  # host: "tcp://docker-host:2376"
```

### 3. Proxmox

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Proxmox**.
2. Aktiviere die Integration, trage URL und Node-Name ein.
3. Speichere Token-ID und Token-Secret im Vault.
4. Speichern und bei Bedarf neu starten.

### YAML-Referenz
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

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Home Assistant**.
2. Aktiviere die Integration und trage die URL ein.
3. Speichere den Long-Lived Access Token im Vault.
4. Optional: **Nur-Lesen** aktivieren.
5. Speichern und bei Bedarf neu starten.

### YAML-Referenz
```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  # AccessToken wird im Vault gespeichert
  readonly: false               # true = nur lesen
```

### 5. Google Workspace (OAuth2)

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Google Workspace**.
2. Aktiviere die gewünschten Dienste und trage die OAuth2-Client-ID ein.
3. Starte die Authentifizierung über die Web-UI — das Token wird im Vault gespeichert.
4. Speichern und bei Bedarf neu starten.

### YAML-Referenz
```yaml
google_workspace:
  enabled: true
```

Die Authentifizierung erfolgt über OAuth2 im Vault-Menü der Web-UI.

### 6. E-Mail

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → E-Mail**.
2. Aktiviere die Integration und trage IMAP-/SMTP-Einstellungen ein.
3. Speichere das Passwort im Vault (nicht in der Config!).
4. Optional: **Watch Enabled** für Posteingangs-Überwachung aktivieren.
5. Speichern und bei Bedarf neu starten.

### YAML-Referenz
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

### Einrichtung in der Web-UI
1. DuckDuckGo ist standardmäßig aktiv (**Config → Tools → Informations-Tools**).
2. Für Brave Search: **Config → Tools → Brave Suche** → API-Key im Vault speichern.
3. Speichern.

### YAML-Referenz
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

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Bildgenerierung**, **Musikgenerierung** und **Videogenerierung**.
2. Aktiviere die gewünschten Provider und setze Tageslimits.
3. Speichere API-Keys im Vault.
4. Speichern und bei Bedarf neu starten.

### YAML-Referenz
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

### 9. Homepage & statische Sites

Marketing-Sites und persönliche Homepages liegen im **Homepage-Workspace** (`data/homepage/` standardmäßig), nicht in `agent_workspace/workdir/`.

### Einrichtung in der Web-UI
1. **Config → Integrationen → Homepage** aktivieren.
2. Workspace- und Registry-Pfade setzen; optional **Lokalen Server erlauben** ohne Docker.
3. Speichern. Für Deploy: **Netlify** und/oder **Vercel** aktivieren.
4. Optional: **Virtual Desktop → Homepage Studio**.

### Fokussierte Agent-Tools (bevorzugt)

| Tool | Rolle |
|------|-------|
| `homepage_project` | Workspace, `init_project`, `exec`, `install_deps` |
| `homepage_file` | Dateien im Homepage-Workspace lesen/schreiben |
| `homepage_deploy` | `build`, `dev`, Publish, `deploy_netlify`, `deploy_vercel` |
| `homepage_quality` | `lint`, `check_js`, `lighthouse`, `screenshot` |
| `homepage_git` | Git-Workflow im Projekt |
| `homepage_registry` | Projektliste, Deploy-/Edit-Logs, **Projekt-Historie** |

Das Legacy-Tool `homepage` akzeptiert weiterhin ältere `operation`-Werte.

**Regeln:** Quellen nur mit `homepage_file` — nicht `filesystem`. Kein `/workspace/...` per `execute_shell`. Vor größeren Änderungen `list_history`, danach `add_history`. Design-Regel: `prompts/rules/homepage/DESIGN.md`.

### YAML-Referenz
```yaml
homepage:
  enabled: true
  allow_local_server: false
```

Siehe [Integrationen](08-integrations.md#homepage--und-website-projekte) und [Interne Tools](22-interne-tools.md#homepage--statische-website-tool-familie).


---

## Adaptive Tools – Intelligente Filterung

**Spare Tokens, bleib fokussiert.** Das Adaptive Tools System filtert Tools basierend auf dem Gesprächskontext:

### Einrichtung in der Web-UI
1. Öffne **Config → Agent → Optimierungen**.
2. Aktiviere **Adaptive Tools** und passe `max_tools` / `max_total_tools` an.
3. Speichern.

### YAML-Referenz
```yaml
agent:
  adaptive_tools:
    enabled: true
    max_tools: 10               # Limit fuer adaptive/bevorzugte Tools
    max_total_tools: 20         # Gesamtlimit fuer native Tool-Schemas
    max_schema_tokens: 6500     # Grobe Schema-Token-Grenze (0 = unbegrenzt)
    provider_profiles_enabled: true
    session_tool_retention_turns: 8
    
    # Immer verfügbar (nicht filtern):
    always_include:
      - "filesystem"
      - "query_memory"
      - "manage_memory"
      - "execute_shell"
```

| Aspekt | Ohne Adaptive | Mit Adaptive |
|--------|---------------|--------------|
| **Tokens** | 50+ Tools im Prompt | Relevante Tools innerhalb eines Gesamtbudgets |
| **Kosten** | Höher | Niedriger |
| **Genauigkeit** | LLM überfordert | Präzisere Tool-Wahl |

Die Einstellung `max_tools` begrenzt nur adaptive beziehungsweise bevorzugte Tools. AuraGo behält zuerst hart benötigte Recovery-Tools, danach den kleinen weichen Always-Include-Kern und zuletzt genutzte Session-Tools, dann adaptive Tools. Tools wie `ddg_search`, `api_request`, `docker`, `execute_python`, `file_editor`, `manage_missions` und Virtual-Desktop-Helfer werden normalerweise über Intent, Kanal, letzte Nutzung oder `discover_tools` eingeblendet.

Das finale native Schema-Budget wird ueber `max_total_tools` gesteuert, wo es moeglich ist. Providerprofile sind nur Stabilitaets-Overlays; normaler Chat, Bots, Missionen, Hintergrundaufgaben und Desktop-Sessions nutzen denselben Budgetpfad.

---

## Read-Only vs. Read-Write

Viele Tools unterstützen einen Read-Only-Modus — jeweils in der jeweiligen Integrationssektion der Web-UI (**Config → Integrationen → …**):

### YAML-Referenz
```yaml
docker:
  enabled: true
  readonly: true        # nur Listen/Inspect/Logs

home_assistant:
  enabled: true
  readonly: true        # nur Status lesen; blockiert call_service
```

---

## Danger Zone

Die Danger Zone kontrolliert potenziell gefährliche Operationen:

### Einrichtung in der Web-UI
1. Öffne **Config → Gefahrenzone** für Shell, Python, Netzwerk, MCP und Paketverwaltung.
2. Öffne **Config → Tools → Web-Scraper** für Scraping (ersetzt `allow_web_scraper`).
3. Speichern — ein Neustart kann erforderlich sein.

### YAML-Referenz
```yaml
agent:
  allow_shell: true              # Shell-Befehle
  allow_python: true             # Python-Ausführung
  allow_filesystem_write: true   # Datei-Schreibzugriff
  allow_network_requests: true   # HTTP-Anfragen
  allow_remote_shell: true       # SSH auf Remote-Geräte
  allow_self_update: true        # Selbst-Updates
  allow_mcp: true                # MCP-Protokoll (zusätzlich mcp.enabled)
  allow_package_manager: false   # erfordert zusätzlich package_manager.enabled

tools:
  web_scraper:
    enabled: true                # ersetzt veraltetes allow_web_scraper
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

### Einrichtung in der Web-UI
1. Öffne **Config → Tools → Netzwerk-Tools**.
2. Aktiviere **Ping**, **Port-Scanner**, **mDNS-Scan** und **UPnP-Scan** nach Bedarf.
3. Speichern.

### YAML-Referenz
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

### Einrichtung in der Web-UI
1. Öffne **Config → Tools → Netzwerk-Tools** und aktiviere **Web Capture**.
2. Optional: **Form Automation** aktivieren (erfordert Web Capture).
3. Speichern.

### YAML-Referenz
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

## PDF Extractor

Text aus PDF-Dokumenten extrahieren mit optionaler LLM-Zusammenfassung.

### Konfiguration

### Einrichtung in der Web-UI
1. Öffne **Config → Tools → Informations-Tools**.
2. Aktiviere **PDF Extractor** und optional **Summary Mode**.
3. Speichern.

### YAML-Referenz
```yaml
tools:
  pdf_extractor:
    enabled: true
    summary_mode: false           # Bei true: extrahierten Text per LLM zusammenfassen
```

### Verfügbare Tools

| Tool | Beschreibung |
|------|--------------|
| `pdf_extractor` | Text und Metadaten aus PDF-Dateien extrahieren |

### Beispiele im Chat

```
Extrahiere den Text aus report.pdf
Fasse die wichtigsten Punkte aus invoice.pdf zusammen
```

---

## Medien-Konvertierung & Video-Download

Konvertierung von Audio-, Video- und Bilddateien sowie Video-Download von Plattformen wie YouTube.

### Konfiguration

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Medienkonvertierung** und **Video Download**.
2. Konfiguriere FFmpeg-/ImageMagick-Pfade und Download-Modus.
3. Optional: **Config → Integrationen → Send YouTube Video** aktivieren.
4. Speichern.

### YAML-Referenz
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

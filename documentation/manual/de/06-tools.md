# Kapitel 6: Werkzeuge

Das Herzstück von AuraGo: Über 30 eingebaute Werkzeuge für nahezu jede Aufgabe.

---

## Übersicht der Tool-Landschaft

AuraGo besitzt ein modulares Tool-System:

```
┌─────────────────────────────────────────────────────────────┐
│                     Tool-Architektur                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   User Request → Agent Loop → Tool Dispatcher → Tool Exec   │
│                         ↑                          ↓        │
│                         └──────── Result ←──────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Tool-Kategorien

### 1. Dateisystem-Tools

| Tool | Funktion | Danger Zone |
|------|----------|-------------|
| `filesystem` | Dateien lesen, schreiben, löschen | Ja |
| `shell` | Shell-Befehle ausführen | Ja |
| `shell_background` | Hintergrund-Prozesse starten | Ja |
| `python` | Python-Code ausführen | Ja |

#### Beispiele: Dateisystem

```
Du: Liste alle Dateien im aktuellen Verzeichnis
Agent: 🛠️ Tool: filesystem (list_dir)
       📁 agent_workspace/workdir/
       ├── dokumente/
       ├── projekte/
       └── backup.tar.gz

Du: Erstelle einen neuen Ordner "mein-projekt"
Agent: 🛠️ Tool: filesystem (create_dir)
       ✅ Ordner erstellt: mein-projekt/
```

> ⚠️ **Achtung:** Shell- und Python-Tools können das System verändern. Nutze Read-Only-Modus für sichere Umgebungen.

---

### 2. Web & API-Tools

| Tool | Funktion | Besonderheit |
|------|----------|--------------|
| `web_search` | DuckDuckGo-Suche | Kein API-Key nötig |
| `brave_search` | Brave Search API | Erfordert API-Key |
| `fetch_url` | Webseiten abrufen | Mit User-Agent-Rotation |
| `web_scraper` | Webseiten scrapen | Extrahiert Hauptinhalt |
| `api_client` | Generischer HTTP-Client | Alle HTTP-Methoden |
| `wikipedia` | Wikipedia-Artikel suchen | Direkter Zugriff |
| `github` | GitHub API-Integration | Repos, Issues, PRs |
| `virustotal` | Datei/URL auf Viren prüfen | Erfordert API-Key |

#### Konfiguration: Brave Search

```yaml
brave_search:
  enabled: true
  api_key: "BS..."        # Brave Search API Key
  country: "DE"           # Optional: Ländercode
  lang: "de"              # Optional: Sprache
```

#### Konfiguration: VirusTotal

```yaml
virustotal:
  enabled: true
  api_key: "..."          # VirusTotal API Key
```

---

### 3. Docker-Tools

| Tool | Funktion | Danger Zone |
|------|----------|-------------|
| `docker_list` | Container/Images auflisten | Nein |
| `docker_logs` | Container-Logs anzeigen | Nein |
| `docker_start` | Container starten | Ja |
| `docker_stop` | Container stoppen | Ja |
| `docker_run` | Neuen Container starten | Ja |
| `docker_exec` | Befehl in Container ausführen | Ja |
| `docker_build` | Image bauen | Ja |
| `docker_prune` | Cleanup durchführen | Ja |

#### Konfiguration

```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
```

---

### 4. Proxmox-Tools

| Tool | Funktion | Danger Zone |
|------|----------|-------------|
| `proxmox_list_nodes` | Nodes im Cluster anzeigen | Nein |
| `proxmox_list_vms` | VMs auflisten | Nein |
| `proxmox_vm_status` | VM-Status prüfen | Nein |
| `proxmox_vm_start` | VM starten | Ja |
| `proxmox_vm_stop` | VM stoppen | Ja |
| `proxmox_vm_shutdown` | VM herunterfahren | Ja |
| `proxmox_vm_snapshot` | Snapshot erstellen | Ja |

#### Konfiguration

```yaml
proxmox:
  enabled: true
  url: "https://proxmox.example.com:8006"
  token_id: "root@pam!aurago"
  node: "pve"
  insecure: false         # true = unsichere TLS akzeptieren
```

---

### 5. Smart Home & IoT

| Tool | Funktion | Integration |
|------|----------|-------------|
| `home_assistant` | Geräte steuern, Zustände abfragen | Home Assistant |
| `chromecast` | Audio/Video streamen | Google Cast |
| `wol` | Wake-on-LAN senden | Netzwerk |
| `mqtt_publish` | MQTT-Nachrichten senden | MQTT Broker |

#### MQTT Konfiguration

```yaml
mqtt:
  enabled: true
  broker: "mqtt.example.com"
  client_id: "aurago"
  username: ""            # Optional
  topics: []              # Zu abonnierende Topics
  qos: 0                  # Quality of Service
  relay_to_agent: false   # MQTT-Nachrichten an Agent weiterleiten
```

---

### 6. Google Workspace

| Tool | Funktion | Berechtigung |
|------|----------|--------------|
| `google_workspace` | Gmail, Kalender, Drive, Docs | OAuth2 |

#### Konfiguration

```yaml
agent:
  enable_google_workspace: true
```

Die Authentifizierung erfolgt über OAuth2 im Vault.

---

### 7. Email-Tools

| Tool | Funktion | Protokoll |
|------|----------|-----------|
| `email_fetch` | Emails abrufen | IMAP |
| `email_send` | Email senden | SMTP |
| `email_search` | Emails suchen | IMAP |

#### Konfiguration

```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  imap_port: 993
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  username: "dein.email@gmail.com"
  from_address: "dein.email@gmail.com"
  watch_enabled: true           # Automatisches Überwachen
  watch_interval_seconds: 120
  watch_folder: "INBOX"
```

---

### 8. System-Tools

| Tool | Funktion | Plattform |
|------|----------|-----------|
| `system_metrics` | CPU, RAM, Disk-Nutzung | Alle |
| `system_time` | Aktuelle Zeit/Datum | Alle |
| `system_info` | Betriebssystem-Info | Alle |
| `process_list` | Laufende Prozesse | Alle |
| `stop_process` | Prozess beenden | Ja (Danger Zone) |

---

### 9. Memory-Tools

| Tool | Funktion | Persistenz |
|------|----------|------------|
| `manage_memory` | Information speichern/abrufen | Core Memory |
| `query_memory` | Semantische Suche | Long-Term Memory |
| `create_note` | Notiz erstellen | Permanent |
| `list_notes` | Notizen auflisten | Permanent |

#### Konfiguration

```yaml
tools:
  memory:
    enabled: true
    readonly: false
  notes:
    enabled: true
    readonly: false
```

---

### 10. Knowledge Graph

| Tool | Funktion |
|------|----------|
| `knowledge_graph` | Entitäten und Beziehungen verwalten |

#### Konfiguration

```yaml
tools:
  knowledge_graph:
    enabled: true
    readonly: false
```

---

### 11. Scheduler & Missions

| Tool | Funktion |
|------|----------|
| `scheduler` | Cron-Jobs verwalten |
| `missions` | Automatisierte Aufgaben |

#### Konfiguration

```yaml
tools:
  scheduler:
    enabled: true
    readonly: false
  missions:
    enabled: true
    readonly: false
```

---

### 12. Vault & Sicherheit

| Tool | Funktion |
|------|----------|
| `secrets_vault` | Secrets speichern/abrufen |

#### Konfiguration

```yaml
tools:
  secrets_vault:
    enabled: true
    readonly: false
```

---

### 13. Inventory (SSH-Geräte)

| Tool | Funktion |
|------|----------|
| `inventory` | SSH-Server-Inventar verwalten |

Ermöglicht Verbindung zu remote SSH-Servern für Befehle.

---

### 14. Cloud-Speicher

| Tool | Funktion |
|------|----------|
| `webdav` | WebDAV-Operationen |
| `koofr` | Koofr Cloud-Speicher |

#### WebDAV Konfiguration

```yaml
webdav:
  enabled: true
  url: "https://cloud.example.com/remote.php/dav/files/user/"
  username: "user"
  password: ""            # Wird im Vault gespeichert
```

#### Koofr Konfiguration

```yaml
koofr:
  enabled: true
  username: "user@example.com"
  app_password: ""        # App-spezifisches Passwort
  base_url: "https://app.koofr.net"
```

---

### 15. Medien-Tools

| Tool | Funktion |
|------|----------|
| `tts` | Text-to-Speech |
| `transcribe` | Audio zu Text (Whisper) |
| `vision` | Bildanalyse |
| `chromecast` | Audio-Streaming |

#### TTS Konfiguration

```yaml
tts:
  provider: google        # oder "elevenlabs"
  language: de
  elevenlabs:
    api_key: ""
    voice_id: ""
    model_id: "eleven_multilingual_v2"
```

#### Whisper/Vision Konfiguration

```yaml
whisper:
  provider: ""            # Provider-ID für STT
  mode: whisper           # "whisper", "multimodal", "local"
  
vision:
  provider: ""            # Provider-ID für Bildanalyse
```

---

### 16. Netzwerk-Tools

| Tool | Funktion |
|------|----------|
| `mdns` | mDNS/Bonjour Discovery |
| `wol` | Wake-on-LAN |
| `tailscale` | Tailscale VPN Status |

#### Tailscale Konfiguration

```yaml
tailscale:
  enabled: true
  readonly: false
  tailnet: "tailnet.ts.net"
```

---

### 17. Chat-Integrationen

| Tool | Funktion |
|------|----------|
| `discord_bridge` | Discord-Nachrichten senden |
| `rocketchat` | Rocket.Chat Integration |
| `handoff` | An anderen Agent übergeben |

#### Rocket.Chat Konfiguration

```yaml
rocketchat:
  enabled: true
  url: "https://chat.example.com"
  user_id: "..."
  channel: "#general"
  alias: "AuraGo"
```

---

### 18. Entwicklungstools

| Tool | Funktion |
|------|----------|
| `git` | Git-Operationen |
| `github` | GitHub API |
| `github_projects` | GitHub Projects |
| `ollama` | Lokale LLM-Verwaltung |
| `mcp` | Model Context Protocol |

#### GitHub Konfiguration

```yaml
github:
  enabled: true
  readonly: false
  owner: "username"
  default_private: false
  base_url: ""            # Für GitHub Enterprise
```

#### Ollama Konfiguration

```yaml
ollama:
  enabled: true
  readonly: false         # false = erlaubt pull/delete
  url: "http://localhost:11434"
```

#### MCP Konfiguration

```yaml
mcp:
  enabled: true
  servers:
    - name: "example"
      url: "http://localhost:3000/sse"
```

---

### 19. Remote Management

| Tool | Funktion |
|------|----------|
| `meshcentral` | MeshCentral Fernwartung |
| `ansible` | Ansible Playbooks |

#### MeshCentral Konfiguration

```yaml
meshcentral:
  enabled: true
  readonly: false
  url: "https://mesh.example.com"
  username: "admin"
  blocked_operations: ["shutdown", "reboot"]  # Optional
```

#### Ansible Konfiguration

```yaml
ansible:
  enabled: true
  readonly: false
  mode: sidecar           # "sidecar" oder "remote"
  url: "http://localhost:5000"  # Für remote mode
  timeout: 300
  playbooks_dir: "/path/to/playbooks"
  default_inventory: "/path/to/inventory"
```

---

### 20. Benachrichtigungen

| Tool | Funktion |
|------|----------|
| `notifications` | Push-Benachrichtigungen |

#### Konfiguration

```yaml
notifications:
  ntfy:
    enabled: true
    url: "https://ntfy.sh"
    topic: "aurago-alerts"
  pushover:
    enabled: true
    # Konfiguration über Web-UI/Vault
```

---

### 21. Sandbox

| Tool | Funktion |
|------|----------|
| `sandbox` | Isolierte Code-Ausführung |

Führt Python-Code in isolierten Docker-Containern aus.

#### Konfiguration

```yaml
sandbox:
  enabled: true
  backend: docker
  docker_host: ""
  image: "python:3.11-slim"
  pool_size: 0            # 0 = dynamisch
  timeout_seconds: 30
  network_enabled: false  # Internet im Sandbox?
  keep_alive: false       # Container am Leben halten?
```

---

### 22. Spezialtools

| Tool | Funktion |
|------|----------|
| `daily_reflection` | Tägliche Zusammenfassung |
| `surgery` | Code-Modifikation (Lifeboat) |
| `state` | Zustandsverwaltung |
| `skills` | Skill-Management |

---

## Tool-Nutzung im Chat

### Implizite Tool-Auswahl

Der Agent wählt automatisch die passenden Tools:

```
Du: Erstelle eine Datei
→ filesystem (write_file)

Du: Suche im Web
→ web_search oder brave_search

Du: Starte einen Container
→ docker_run
```

### Explizite Tool-Anfrage

```
Du: Nutze das filesystem Tool, um die Datei config.yaml zu lesen
Du: Führe ein docker_list aus
```

---

## Read-Only vs. Read-Write Modus

### Konfiguration

```yaml
tools:
  memory:
    enabled: true
    readonly: true        # Nur lesen, nicht schreiben
  filesystem:
    enabled: true
    readonly: true
```

### Verhalten im Read-Only Modus

| Tool-Kategorie | Read-Only | Read-Write |
|----------------|-----------|------------|
| filesystem | Nur lesen | Lesen, schreiben, löschen |
| shell | Keine Ausführung | Alle Befehle |
| docker | Nur Listen/Logs | Starten, Stoppen, Erstellen |
| missions | Nur ansehen | Erstellen, Bearbeiten, Löschen |

---

## Danger Zone

Die Danger Zone kennzeichnet Tools, die das System verändern können:

```yaml
agent:
  # Einzelne Tool-Kategorien erlauben/verbieten
  allow_shell: true
  allow_python: true
  allow_filesystem_write: true
  allow_network_requests: true
  allow_remote_shell: true
  allow_self_update: true
  allow_mcp: true
  allow_web_scraper: true
```

> ⚠️ **Wichtig:** In Produktionsumgebungen sollte AuraGo immer in einer isolierten Umgebung laufen (VM, Docker, dedizierter Server).

---

## Tool-Statistik anzeigen

Nutze `/budget` für Nutzungsstatistiken:

```
Du: /budget
Agent: 💰 Tool-Nutzung (heute):
       
       ┌───────────────────┬────────┬──────────┐
       │ Tool              │ Aufrufe│ Tokens   │
       ├───────────────────┼────────┼──────────┤
       │ web_search        │ 12     │ 2,400    │
       │ filesystem        │ 8      │ 800      │
       │ shell             │ 5      │ 1,200    │
       └───────────────────┴────────┴──────────┘
```

---

## Zusammenfassung

| Kategorie | Anzahl | Danger Zone |
|-----------|--------|-------------|
| Dateisystem | 4 | Ja |
| Web/API | 7 | Nein |
| Docker | 8 | Ja |
| Proxmox | 7 | Ja |
| Smart Home/IoT | 4 | Ja |
| Google Workspace | 1 | Nein |
| Email | 3 | Nein |
| System | 5 | Teilweise |
| Memory | 4 | Nein |
| Cloud | 2 | Ja |
| Medien | 4 | Nein |
| Netzwerk | 3 | Nein |
| Chat | 3 | Nein |
| Entwicklung | 5 | Ja |
| Remote Mgmt | 2 | Ja |
| **Gesamt** | **60+** | - |

> 💡 **Merke:** AuraGos Stärke liegt in der Kombination von Tools. Ein einzelner Chat kann Shell-Befehle, Web-Suchen, Docker-Operationen und Email-Benachrichtigungen verketten – vollautomatisch!

---

**Nächste Schritte**

- **[Konfiguration](07-konfiguration.md)** – Tools feinjustieren und sichern
- **[Integrationen](08-integrations.md)** – Externe Dienste verbinden

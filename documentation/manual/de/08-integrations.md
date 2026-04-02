# Kapitel 8: Integrationen

AuraGo lässt sich nahtlos in verschiedene Dienste und Plattformen integrieren.

---

## Übersicht aller Integrationen

| Integration | Typ | Zweck | Konfiguration |
|-------------|-----|-------|---------------|
| **Telegram** | Chat | Mobile Benachrichtigungen | `telegram:` |
| **Discord** | Chat | Community-Integration | `discord:` |
| **Rocket.Chat** | Chat | Self-hosted Chat | `rocketchat:` |
| **E-Mail** | Kommunikation | IMAP/SMTP | `email:` |
| **Home Assistant** | Smart Home | Gerätesteuerung | `home_assistant:` |
| **MQTT** | IoT | Geräte-Kommunikation | `mqtt:` |
| **Docker** | Infrastruktur | Container-Verwaltung | `docker:` |
| **Proxmox** | Infrastruktur | VM-Verwaltung | `proxmox:` |
| **FritzBox** | Infrastruktur | FRITZ!Box Router | `fritzbox:` |
| **Webhooks** | API | Eingehende HTTP-Events | `webhooks:` |
| **Budget Tracking** | Finanzen | Kostenkontrolle | `budget:` |
| **Google Workspace** | Produktivität | Gmail, Kalender, Drive | `google_workspace:` |
| **WebDAV/Koofr** | Speicher | Cloud-Dateizugriff | `webdav:`, `koofr:` |
| **OneDrive** | Speicher | Microsoft OneDrive | `onedrive:` |
| **S3** | Speicher | S3-kompatibler Storage | `s3:` |
| **Tailscale** | Netzwerk | VPN-Status | `tailscale:` |
| **Cloudflare Tunnel** | Netzwerk | Sicherer Remote-Zugriff | `cloudflare_tunnel:` |
| **Brave Search** | Suche | Websuche API | `brave_search:` |
| **GitHub** | Entwicklung | Repository-Verwaltung | `github:` |
| **Ollama** | AI | Lokale Modelle | `ollama:` |
| **MeshCentral** | Remote | Fernwartung | `meshcentral:` |
| **Ansible** | Automation | Playbook-Ausführung | `ansible:` |
| **Homepage** | Web | Persönliche Startseite | `homepage:` |
| **Netlify** | Web | Static Site Deployment | `netlify:` |
| **SQL Connections** | Datenbank | Externe DB-Verbindungen | `sql_connections:` |
| **Chromecast** | Media | Casting zu Geräten | `chromecast:` |
| **Notifications** | Alerts | Push-Benachrichtigungen | `notifications:` |
| **LLM Guardian** | Sicherheit | Inhaltsfilterung | `llm_guardian:` |
| **Fallback LLM** | AI | Failover für LLM | `fallback_llm:` |
| **MCP Client/Server** | AI | Model Context Protocol | `mcp:` |
| **Image Generation** | AI | Bildgenerierung | `image_generation:` |
| **Piper TTS** | Audio | Lokale Sprachsynthese | `piper_tts:` |
| **Paperless-ngx** | Dokumentenmanagement | Dokumentenablage | `paperless_ngx:` |
| **AdGuard** | Sicherheit | AdGuard Home | `adguard:` |
| **n8n** | Automation | Workflow-Automatisierung | `n8n:` |
| **Memory Analysis** | Analyse | Gedächtnis-Analytik | `memory_analysis:` |
| **Skill Manager** | Skills | Python-Skill-Verwaltung | `skill_manager:` |
| **Personality V2** | Persönlichkeit | Erweiterte Persönlichkeit | `personality_v2:` |
| **User Profiling** | Analyse | Nutzerverhaltensprofil | `user_profiling:` |
| **Indexer** | Suche | Inhaltsindexierung | `indexing:` |
| **AI Gateway** | AI | Multi-Provider Gateway | `ai_gateway:` |
| **Security Proxy** | Sicherheit | Sicherheits-Proxy | `security_proxy:` |
| **VirusTotal** | Sicherheit | Datei-/URL-Scanning | `virustotal:` |
| **Firewall** | Sicherheit | Firewall-Verwaltung | `firewall:` |
| **A2A** | Agent | Agent-zu-Agent Protokoll | `a2a:` |
| **Co-Agents** | Agent | Sub-Agent-Koordination | `co_agents:` |
| **Remote Control** | Remote | Fernsteuerung | `remote_control:` |
| **Invasion** | Distributed | Verteilte Bereitstellung | `invasion:` |
| **Sandbox** | Sicherheit | Isolierte Ausführung | `sandbox:` |
| **Helper LLM** | AI | Helper-LLM für Analysen | `helper_llm:` |

---

## Telegram Bot Setup

### Schritt 1: Bot bei BotFather erstellen

1. Öffne Telegram und suche nach **@BotFather**
2. Starte den Bot mit `/start`
3. Erstelle einen neuen Bot: `/newbot`
4. Gib einen Namen ein (z.B. "Mein AuraGo")
5. Gib einen Benutzernamen ein (muss mit "bot" enden)
6. **Speichere den Token** (z.B. `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

### Schritt 2: Deine User-ID ermitteln

1. Suche nach **@userinfobot**
2. Starte den Bot
3. Erhalte deine ID (z.B. `12345678`)

### Schritt 3: AuraGo konfigurieren

```yaml
telegram:
  bot_token: "123456789:ABC..."      # Wird im Vault gespeichert
  telegram_user_id: 12345678
  max_concurrent_workers: 5
```

> 💡 **Sicherheit:** Das Bot-Token wird automatisch im verschlüsselten Vault gespeichert, nicht in der config.yaml.

### Schritt 4: Testen

1. Starte AuraGo neu
2. Sende eine Nachricht an deinen Bot
3. Der Bot sollte antworten

---

## Discord Bot Setup

### Schritt 1: Discord-Anwendung erstellen

1. Besuche [Discord Developer Portal](https://discord.com/developers/applications)
2. Klicke "New Application"
3. Gib einen Namen ein (z.B. "AuraGo")
4. Gehe zu "Bot" → "Add Bot"

### Schritt 2: Token und Berechtigungen

1. Kopiere den **Token**
2. Aktiviere Intents:
   - Message Content Intent
   - Server Members Intent

### Schritt 3: Bot zum Server einladen

1. Gehe zu "OAuth2" → "URL Generator"
2. Scopes: `bot`, `applications.commands`
3. Permissions: `Send Messages`, `Read Messages`
4. Öffne die URL und wähle deinen Server

### Schritt 4: AuraGo konfigurieren

```yaml
discord:
  enabled: true
  bot_token: "DEIN-TOKEN"           # Wird im Vault gespeichert
  guild_id: "123456789012345678"
  default_channel_id: "123456789012345678"
  allowed_user_id: ""               # Optional: nur dieser User
```

---

## Rocket.Chat Integration

Für selbst-gehostete Rocket.Chat-Instanzen.

```yaml
rocketchat:
  enabled: true
  url: "https://chat.example.com"
  user_id: "..."
  channel: "#general"
  alias: "AuraGo"
```

---

## E-Mail (IMAP/SMTP) Konfiguration

### Einzelnes E-Mail-Konto

```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  imap_port: 993
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  username: "dein.email@gmail.com"
  from_address: "dein.email@gmail.com"
  watch_enabled: true
  watch_interval_seconds: 120
  watch_folder: "INBOX"
```

> ⚠️ **Wichtig:** Das Passwort wird **nicht** in der `config.yaml` gespeichert, sondern im verschlüsselten Vault. Konfiguriere es über die Web-UI oder den Chat-Befehl `/store_secret`.

### Gmail App-Passwort verwenden

Für Gmail musst du ein [App-Passwort](https://myaccount.google.com/apppasswords) erstellen:

1. Google-Konto → Sicherheit → 2-Schritt-Verification aktivieren
2. App-Passwörter → Andere (benutzerdefinierter Name)
3. Das generierte Passwort im Vault speichern:
   ```
   /store_secret email_password "dein-app-passwort"
   ```

### Provider-Einstellungen

| Provider | IMAP-Host | SMTP-Host |
|----------|-----------|-----------|
| Gmail | `imap.gmail.com` | `smtp.gmail.com` |
| Outlook | `outlook.office365.com` | `smtp.office365.com` |
| GMX | `imap.gmx.net` | `mail.gmx.net` |
| Web.de | `imap.web.de` | `smtp.web.de` |

---

## Home Assistant Integration

Steuere Smart-Home-Geräte über AuraGo.

### Einrichtung

```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  access_token: ""                  # Wird im Vault gespeichert
```

### Access Token erstellen

1. Öffne Home Assistant
2. Gehe zu deinem Profil (unten links)
3. Scrollen zu "Long-Lived Access Tokens"
4. Klicke "Create Token"
5. Speichere den Token im Vault

### Verwendung im Chat

```
Schalte das Licht im Wohnzimmer an.
Wie ist die Temperatur im Schlafzimmer?
Starte die Staubsauger-Routine.
```

---

## MQTT Integration

Für IoT-Geräte und Smart-Home-Automation.

```yaml
mqtt:
  enabled: true
  broker: "mqtt.example.com"
  client_id: "aurago"
  username: ""                      # Optional
  topics:                           # Zu abonnierende Topics
    - "home/+/sensors"
    - "aurago/commands"
  qos: 0                            # Quality of Service (0, 1, 2)
  relay_to_agent: false             # MQTT-Nachrichten an Agent weiterleiten
```

---

## Docker Integration

Verwalte Docker-Container über AuraGo.

### Konfiguration

```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
```

### Docker Socket mounten (Docker-Compose)

```yaml
services:
  aurago:
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

> ⚠️ **Sicherheit:** Der Docker-Zugriff ermöglicht volle Kontrolle über den Host. Aktiviere `readonly` für mehr Sicherheit.

---

## Proxmox Integration

VM- und Container-Verwaltung.

```yaml
proxmox:
  enabled: true
  url: "https://proxmox.example.com:8006"
  token_id: "root@pam!aurago"
  node: "pve"
  insecure: false                   # true = unsichere TLS akzeptieren
```

Das Token wird im Vault gespeichert.

---

## Webhooks

Webhooks ermöglichen es externen Diensten, AuraGo zu benachrichtigen.

### Konfiguration

```yaml
webhooks:
  enabled: true
  readonly: false
  max_payload_size: 65536
  rate_limit: 60
```

### Webhook erstellen (API)

```bash
curl -X POST http://localhost:8088/api/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GitHub Push",
    "slug": "github-push",
    "format": {
      "accepted_content_types": ["application/json"],
      "fields": [
        {"source": "repository.name", "alias": "repo"},
        {"source": "pusher.name", "alias": "author"}
      ]
    },
    "delivery": {
      "mode": "message",
      "priority": "immediate"
    }
  }'
```

---

## Budget Tracking

Überwache die Kosten für LLM-API-Aufrufe.

```yaml
budget:
  enabled: true
  daily_limit_usd: 1.0
  enforcement: "warn"               # "warn", "partial", "full"
  warning_threshold: 0.8
  reset_hour: 0
  default_cost:
    input_per_million: 1.0
    output_per_million: 3.0
  models:
    - name: "gpt-4"
      input_per_million: 30.0
      output_per_million: 60.0
```

---

## Google Workspace

Zugriff auf Gmail, Kalender und Drive.

```yaml
agent:
  enable_google_workspace: true
```

Die OAuth2-Authentifizierung erfolgt über die Web-UI.

---

## WebDAV/Koofr

### WebDAV (Nextcloud, ownCloud, Synology)

```yaml
webdav:
  enabled: true
  url: "https://cloud.example.com/remote.php/dav/files/username/"
  username: "username"
  password: ""                      # Wird im Vault gespeichert
```

### Koofr

```yaml
koofr:
  enabled: true
  username: "user@example.com"
  app_password: ""                  # App-spezifisches Passwort
  base_url: "https://app.koofr.net"
```

---

## Tailscale

VPN-Status und -Verwaltung.

```yaml
tailscale:
  enabled: true
  readonly: false
  tailnet: "tailnet.ts.net"
```

---

## Brave Search

Erweiterte Websuche über Brave Search API.

```yaml
brave_search:
  enabled: true
  api_key: "BS..."
  country: "DE"
  lang: "de"
```

---

## GitHub Integration

Repository- und Issue-Verwaltung.

```yaml
github:
  enabled: true
  readonly: false
  owner: "username"
  default_private: false
  base_url: ""                      # Für GitHub Enterprise
```

---

## Ollama Integration

Lokale LLM-Verwaltung.

```yaml
ollama:
  enabled: true
  readonly: false                   # false = erlaubt pull/delete
  url: "http://localhost:11434"
```

---

## MeshCentral

Remote-Desktop und -Verwaltung.

```yaml
meshcentral:
  enabled: true
  readonly: false
  url: "https://mesh.example.com"
  username: "admin"
  blocked_operations: ["shutdown"]  # Optional: Operationen blockieren
```

---

## Ansible Integration

Playbook-Ausführung.

```yaml
ansible:
  enabled: true
  readonly: false
  mode: sidecar                     # "sidecar" oder "remote"
  url: "http://localhost:5000"      # Für remote mode
  timeout: 300
  playbooks_dir: "/path/to/playbooks"
  default_inventory: "/path/to/inventory"
```

---

## Notifications

Push-Benachrichtigungen.

```yaml
notifications:
  ntfy:
    enabled: true
    url: "https://ntfy.sh"
    topic: "aurago-alerts"
  pushover:
    enabled: true
    # Token über Web-UI konfigurieren
```

---

Verwalte ZFS-Storage-Pools, Datasets, Snapshots und SMB/NFS-Shares auf TrueNAS SCALE oder CORE.

### Konfiguration

```yaml
truenas:
  enabled: true
  host: "truenas.local"        # Hostname oder IP
  port: 443                    # API-Port (Standard: 443)
  use_https: true              # HTTPS verwenden (empfohlen)
  insecure_ssl: false          # Zertifikatsprüfung überspringen (nur Test)
  readonly: false              # Nur Lesezugriff
  allow_destructive: false     # Löschen/Rollback erlauben
```

### API-Key erstellen

1. In TrueNAS Web-UI: **System → API Keys**
2. Auf **Add** klicken
3. Name: "AuraGo" und Key kopieren
4. In AuraGo Vault speichern (Web-UI → Konfiguration → TrueNAS)

### Verfügbare Operationen

| Operation | Beschreibung | Berechtigung |
|-----------|--------------|--------------|
| Pools anzeigen | Storage-Pools mit Status und Kapazität | Lesen |
| Pool scrub | Datenintegritätsprüfung starten | Schreiben |
| Datasets anzeigen | Alle ZFS-Datasets auflisten | Lesen |
| Dataset erstellen | Neues ZFS-Dataset anlegen | Schreiben |
| Dataset löschen | Dataset entfernen (destructive) | Destructive |
| Snapshots anzeigen | Snapshots eines Datasets | Lesen |
| Snapshot erstellen | Point-in-Time Snapshot | Schreiben |
| Snapshot löschen | Snapshot entfernen | Destructive |
| Rollback | Dataset zu Snapshot wiederherstellen | Destructive |
| SMB-Shares | SMB-Freigaben verwalten | Schreiben |
| NFS-Shares | NFS-Freigaben verwalten | Schreiben |
| Speicherplatz | Pool/Dataset-Kapazität prüfen | Lesen |

### Beispiel-Befehle

```
Zeige mir alle Storage-Pools auf TrueNAS
Erstelle ein neues Dataset namens tank/backups
Erstelle einen Snapshot von tank/media
Erstelle eine SMB-Freigabe für tank/media namens Media
Wie viel freier Speicher ist auf dem tank-Pool?
```

> 💡 **Tipp:** `readonly: true` für rein monitoring. `allow_destructive` nur aktivieren wenn nötig.

---

## FritzBox Integration

Steuere AVM Fritz!Box-Router über TR-064-Protokoll.

### Konfiguration

```yaml
fritzbox:
  enabled: true
  host: "fritz.box"
  username: "admin"              # Falls in FritzBox gesetzt
  password: ""                   # Wird im Vault gespeichert
  insecure: false                # Selbstsignierte Zertifikate erlauben
```

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| Geräteinfo | Modell, Firmware, Uptime |
| Online-Status | Internet-Verbindung prüfen |
| Verbundene Geräte | LAN/WiFi-Clients anzeigen |
| Bandbreite | Aktuelle Up/Down-Geschwindigkeit |
| Neu verbinden | Neue WAN-Verbindung erzwingen |
| Portfreigaben | Port-Forwarding-Regeln anzeigen |

---

## AdGuard Home Integration

DNS-Filterung und -Blockierung mit AdGuard Home verwalten.

### Konfiguration

```yaml
adguard:
  enabled: true
  host: "adguard.local"
  port: 3000
  username: "admin"
  password: ""                   # Wird im Vault gespeichert
  use_https: false
  readonly: false
```

### Verfügbare Operationen

| Operation | Beschreibung | Berechtigung |
|-----------|--------------|--------------|
| Status | Filter-Status und Statistiken | Lesen |
| Query-Log | DNS-Abfragen anzeigen | Lesen |
| Top-Clients | Aktivste Geräte | Lesen |
| Blocklisten | Filterlisten verwalten | Schreiben |
| Custom Rules | DNS-Rewrites hinzufügen | Schreiben |
| Safe Search | Safe Search umschalten | Schreiben |

> 💡 **Tipp:** `readonly: true` um Netzwerkaktivität zu überwachen ohne Filter zu ändern.

---

## TrueNAS Integration

Verwalte TrueNAS/TrueNAS Scale Storage-Systeme über die API.

### Konfiguration

```yaml
truenas:
  enabled: true
  host: "truenas.local"          # TrueNAS Hostname oder IP
  api_key: ""                     # API Key (wird im Vault gespeichert)
  use_https: true                 # HTTPS verwenden
  verify_ssl: true                # SSL-Zertifikat prüfen
  readonly: false                 # Nur-Lesen Modus
```

### API Key erstellen

1. TrueNAS Web-UI öffnen
2. Einstellungen (Zahnrad oben rechts) → API Keys
3. "Add" klicken
4. Name vergeben (z.B. "AuraGo")
5. Key kopieren und in Vault speichern

### Verfügbare Operationen

| Kategorie | Operation | Beschreibung |
|-----------|-----------|--------------|
| **Pools** | List Pools | Alle ZFS-Pools anzeigen |
| | Pool Status | Pool-Health prüfen |
| **Datasets** | List Datasets | Datasets auflisten |
| | Create Dataset | Neues Dataset erstellen |
| | Delete Dataset | Dataset löschen |
| | Set Quota | Speicherquota festlegen |
| **Snapshots** | List Snapshots | Snapshots anzeigen |
| | Create Snapshot | Snapshot erstellen |
| | Delete Snapshot | Snapshot löschen |
| | Rollback | Zu Snapshot zurücksetzen |
| **Shares** | List Shares | SMB/NFS Shares anzeigen |
| | Create Share | Share erstellen |
| | Delete Share | Share löschen |

### Beispiele im Chat

```
Du: Zeige mir alle ZFS-Pools auf dem TrueNAS
Agent: 🛠️ Tool: truenas_list_pools
       
       📊 Pools:
       ┌────────────┬──────────┬─────────────┬──────────┐
       │ Name       │ Größe    │ Verfügbar   │ Status   │
       ├────────────┼──────────┼─────────────┼──────────┤
       │ tank       │ 12 TB    │ 8.5 TB      │ ONLINE   │
       │ backup     │ 6 TB     │ 5.2 TB      │ ONLINE   │
       └────────────┴──────────┴─────────────┴──────────┘

Du: Erstelle einen Snapshot von tank/data
Agent: 🛠️ Tool: truenas_create_snapshot
       ✅ Snapshot erstellt: tank/data@auto-20260326-120000

Du: Wie viel Platz ist noch im Pool tank?
Agent: 🛠️ Tool: truenas_pool_status
       📊 Pool "tank":
       - Gesamt: 12 TB
       - Verwendet: 3.5 TB (29%)
       - Verfügbar: 8.5 TB
       - Status: HEALTHY
```

### Read-Only Modus

Für Monitoring ohne Schreibrechte:

```yaml
truenas:
  enabled: true
  readonly: true              # Keine Änderungen möglich
```

Im Read-Only Modus können nur List- und Get-Operationen ausgeführt werden.

---

## n8n Integration

Verbindung mit n8n Workflow-Automatisierungsplattform.

### Konfiguration

```yaml
n8n:
  enabled: true
  base_url: "https://n8n.deinedomain.com"
  api_key: ""                    # Wird im Vault gespeichert
  readonly: false
```

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| Workflows anzeigen | Alle n8n-Workflows auflisten |
| Workflow-Details | Spezifische Workflow-Details |
| Aktivieren/Deaktivieren | Workflow-Status umschalten |
| Workflow ausführen | Manuell auslösen |
| Ausführungen anzeigen | Ausführungsverlauf |

### n8n Node für AuraGo

AuraGo bietet einen offiziellen n8n Community Node:
- Chatte mit AuraGo aus n8n-Workflows
- Trigger Tools und Missions
- Zugriff auf Memory und Knowledge

Installation: `@antibyte/n8n-nodes-aurago`

---

## Telnyx Integration

SMS senden/empfangen und Sprachanrufe über Telnyx-Telefonie.

### Konfiguration

```yaml
telnyx:
  enabled: true
  api_key: ""                    # Wird im Vault gespeichert
  public_key: ""
  phone_number: "+491234567890"
  messaging_enabled: true
  voice_enabled: true
```

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| SMS senden | Textnachrichten versenden |
| SMS empfangen | Eingehende Nachrichten verarbeiten |
| Anrufe tätigen | Sprachanrufe initiieren |
| Voicemail | Voicemail-Nachrichten verwalten |

---

## VirusTotal Integration

Dateien und URLs auf Malware prüfen mit VirusTotal API.

### Konfiguration

```yaml
virustotal:
  enabled: true
  api_key: ""                    # Wird im Vault gespeichert
```

### API-Key erhalten

1. Bei [VirusTotal](https://www.virustotal.com) registrieren
2. Profil → API Key
3. Key kopieren

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| URL scannen | URL-Reputation prüfen |
| Datei scannen | Datei-Hash analysieren |
| Report abrufen | Scan-Ergebnisse anzeigen |

> 💡 **Tipp:** Der Agent verwendet dies automatisch bei verdächtigen Links oder Dateien.

---

## MCP (Model Context Protocol)

Verbinde externe MCP-Server oder stelle AuraGo als MCP-Server bereit.

### MCP Client (externe Server)

```yaml
mcp:
  enabled: true
  allowed_tools:
    - "fetch"
    - "filesystem"
  servers:
    fetch:
      command: "uvx"
      args: ["mcp-server-fetch"]
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/allowed/path"]
```

### MCP Server (AuraGo für andere Clients)

```yaml
mcp_server:
  enabled: true
  allowed_tools:
    - "shell"
    - "docker_*"
  auth_token: "secure-token"
```

Verbindung von Claude Desktop, Cursor oder anderen MCP-Clients möglich.

---

## SQL Connections - Externe Datenbanken

Verbinde AuraGo mit externen Datenbanken (PostgreSQL, MySQL/MariaDB, SQLite) für direkte SQL-Abfragen.

### Konfiguration

```yaml
sql_connections:
  enabled: true
  max_pool_size: 5                # Maximale gleichzeitige Verbindungen pro DB
  connection_timeout_sec: 30      # Verbindungs-Timeout
  query_timeout_sec: 120          # Abfrage-Timeout
  max_result_rows: 1000           # Maximale Zeilen pro Abfrage
```

### Verbindungen im Chat hinzufügen

```
Füge eine PostgreSQL-Verbindung hinzu:
- Name: produktion
- Host: db.example.com
- Port: 5432
- Datenbank: myapp
- Benutzer: readonly_user
```

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| SQL-Abfragen | SELECT, INSERT, UPDATE, DELETE |
| Schema-Info | Tabellen und Spalten auflisten |
| Verbindungstest | Konnektivität prüfen |

### Sicherheitshinweise

- Verwende dedizierte Read-Only Benutzer wenn möglich
- SSL/TLS wird automatisch verwendet wenn verfügbar
- Credentials werden im Vault gespeichert

---

## S3-kompatible Cloud Storage

Zugriff auf S3, MinIO, Wasabi, DigitalOcean Spaces und andere S3-kompatible Speicher.

### Konfiguration

```yaml
s3:
  enabled: true
  readonly: false                 # true = nur lesen
  endpoint: "https://s3.amazonaws.com"  # oder MinIO: http://minio.local:9000
  region: "us-east-1"
  bucket: "my-bucket"             # Standard-Bucket (optional)
  use_path_style: true            # Für MinIO erforderlich
  insecure: false                 # HTTP erlauben (nicht empfohlen)
  # Zugangsdaten werden im Vault gespeichert
```

### Vault-Keys

- `s3_access_key` - Access Key ID
- `s3_secret_key` - Secret Access Key

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| List Buckets | Alle Buckets anzeigen |
| List Objects | Dateien in einem Bucket auflisten |
| Upload | Dateien hochladen |
| Download | Dateien herunterladen |
| Delete | Dateien löschen |
| Copy | Dateien zwischen Buckets kopieren |

---

## OneDrive Integration

Zugriff auf Microsoft OneDrive über Microsoft Graph API.

### Konfiguration

```yaml
onedrive:
  enabled: true
  readonly: false                 # true = nur lesen
  client_id: ""                   # Azure App Registration Client ID
  tenant_id: "common"             # "common", "consumers", "organizations" oder Tenant UUID
```

### Einrichtung

1. **Azure App Registration erstellen:**
   - [Azure Portal](https://portal.azure.com) → App Registrations → New
   - Name: "AuraGo"
   - Supported account types: Accounts in any organizational directory + personal
   - Redirect URI: Web → `http://localhost:8088/api/onedrive/callback`

2. **API Permissions hinzufügen:**
   - Microsoft Graph → Delegated permissions
   - `Files.Read.All` (oder `Files.ReadWrite.All` für Schreibzugriff)
   - `User.Read`

3. **Client ID kopieren** und in Config einfügen

4. **OAuth über Web-UI durchführen**

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| Dateien auflisten | Ordnerinhalte anzeigen |
| Dateien downloaden | Inhalt herunterladen |
| Dateien hochladen | Neue Dateien speichern |
| Dateien löschen | In den Papierkorb verschieben |

---

## Homepage Integration

Erstelle und deploye persönliche Startseiten/Dashboards mit Homepage.

### Konfiguration

```yaml
homepage:
  enabled: true
  allow_deploy: true              # Deployment erlauben
  allow_container_management: true  # Docker-Container verwalten
  allow_local_server: false       # Lokaler Python-Server (Security Risk!)
  deploy_host: "server.example.com"
  deploy_port: 22
  deploy_user: "deploy"
  deploy_path: "/var/www/homepage"
  deploy_method: "sftp"           # "sftp" oder "scp"
  webserver_enabled: false        # Eingebauter Webserver
  webserver_port: 8080
  webserver_domain: ""
  webserver_internal_only: false
  circuit_breaker_max_calls: 35   # Max Tool-Calls für komplexe Workflows
  allow_temporary_token_budget_overflow: false  # Temporäres Budget-Overflow
```

### Verfügbare Operationen

| Operation | Beschreibung |
|-----------|--------------|
| Seite erstellen | Neue Homepage generieren |
| Seite deployen | Auf Server hochladen |
| Container hinzufügen | Docker-Container zur Seite hinzufügen |
| Service-Status | Live-Status von Services anzeigen |

---

## Cloudflare Tunnel

Sicherer Tunnel für Remote-Zugriff ohne öffentliche IP oder Port-Forwarding.

### Konfiguration

```yaml
cloudflare_tunnel:
  enabled: true
  readonly: false
  mode: auto                      # "auto", "docker", "native"
  auto_start: true                # Automatisch beim Start aktivieren
  auth_method: token              # "token", "named", "quick"
  tunnel_name: ""                 # Named tunnel: Name aus CF Dashboard
  account_id: ""                  # Named tunnel: Cloudflare Account ID
  expose_web_ui: true             # AuraGo Web-UI durch Tunnel erreichbar
  expose_homepage: true           # Homepage durch Tunnel erreichbar
  custom_ingress: []              # Zusätzliche Ingress-Regeln
  metrics_port: 0                 # Metrics-Endpoint (0 = deaktiviert)
  log_level: info                 # "debug", "info", "warn", "error"
```

### Auth-Methoden

| Methode | Beschreibung |
|---------|--------------|
| `token` | Connector Token (empfohlen für einfachen Einsatz) |
| `named` | Benannter Tunnel mit credentials.json |
| `quick` | Temporärer Tunnel ohne Account |

### Connector Token erhalten

1. [Cloudflare Zero Trust](https://one.dash.cloudflare.com) → Networks → Tunnels
2. "Create a tunnel" → Cloudflared
3. Name vergeben → Connector auswählen
4. Token wird angezeigt (im Vault speichern unter `cloudflare_tunnel_token`)

---

## Cloudflare AI Gateway

Routing und Monitoring für LLM-Traffic über Cloudflare AI Gateway.

### Konfiguration

```yaml
ai_gateway:
  enabled: true
  account_id: ""                  # Cloudflare Account ID
  gateway_id: ""                  # Gateway ID
```

### Vorteile

- **Rate Limiting**: Kontrolliere API-Nutzung
- **Caching**: Wiederholte Anfragen aus Cache bedienen
- **Logging**: Vollständiges Request/Response Logging
- **Analytics**: Nutzungsstatistiken

---

## Chromecast Integration

Sende Text-to-Speech und Medien an Chromecast-Geräte.

### Konfiguration

```yaml
chromecast:
  enabled: true
  tts_port: 8090                  # Port für TTS-Streaming
```

### Verwendung

```
Sage "Hallo Welt" auf dem Wohnzimmer-Chromecast
Spiele die Nachrichten im Küche-Chromecast ab
```

### Anforderungen

- Chromecast-Gerät im gleichen Netzwerk
- TTS muss konfiguriert sein (Google, ElevenLabs, oder Piper)

---

## Media Registry

Zentrale Verwaltung von Mediendateien mit Metadaten-Tracking.

### Konfiguration

```yaml
media_registry:
  enabled: true                   # Ermöglicht Medienverwaltung
```

### Funktionen

- Bilder, Videos und Audio-Dateien katalogisieren
- Metadaten-Extraktion (EXIF, etc.)
- Duplikat-Erkennung
- Kategorisierung und Tagging
- Integration mit Paperless NGX

---

## Zusätzliche Integrationen (Vollständigkeits-Ergänzung)

Neben den oben beschriebenen Kernintegrationen umfasst die aktuelle Plattform weitere Integrationen/Features:

| Integration/Feature | Typischer Zweck | Wichtige Config-Blöcke |
|---|---|---|
| Cloudflare Tunnel + AI Gateway | sicherer Internetzugang und AI-Traffic-Routing | `cloudflare_tunnel`, `ai_gateway` |
| AdGuard / FRITZ!Box / MQTT | Heimnetz- und Smart-Home-Anbindung | `adguard`, `fritzbox`, `mqtt` |
| Paperless NGX + Media Registry + Homepage | Dokumenten-/Medien-/Site-Verwaltung | `paperless_ngx`, `media_registry`, `homepage` |
| Netlify | Deployment statischer Seiten | `netlify` |
| S3 + OneDrive + WebDAV/Koofr | Multi-Backend Cloud-Storage | `s3`, `onedrive`, `webdav`, `koofr` |
| Telnyx + Rocket.Chat | Telefonie und Self-Hosted-Chat | `telnyx`, `rocketchat` |
| Image Generation / TTS / Whisper | multimodale Generierung und Sprach-Pipelines | `image_generation`, `tts`, `whisper` |
| MCP-Server-Modus | AuraGo-Funktionen für externe MCP-Clients bereitstellen | `mcp_server` |
| LLM Guardian | Sicherheits- und Policy-Kontrollen | `llm_guardian` |

> Best Practice: Integrationen zuerst read-only (`read_only`/`readonly`) aktivieren und Schreibzugriffe erst nach erfolgreichen Tests freischalten.

---

## Integrationen testen

### Test über Chat

```
Zeige meine Telegram-Config.
Sende eine Test-E-Mail an mich.
Liste alle Docker-Container.
Wie ist der Status meiner Home Assistant-Geräte?
```

### Test über Dashboard

1. Öffne die Web-UI
2. Klicke auf "Dashboard"
3. Scrolle zu "Integrationen"
4. Grüner Punkt = Verbindung OK

### Debug-Logging

```yaml
agent:
  debug_mode: true
```

Logs prüfen:
```bash
tail -f log/supervisor.log | grep -i telegram
```

---

## Fehlerbehebung

| Problem | Lösung |
|---------|--------|
| "Connection refused" | URL und Port prüfen |
| "Unauthorized" | API-Key/Token prüfen |
| "Timeout" | Firewall/Netzwerk prüfen |
| Integration erscheint nicht | `enabled: true` in config.yaml |

---

**Nächstes Kapitel:** [Kapitel 9: Gedächtnis & Wissen](./09-memory.md)

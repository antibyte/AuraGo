# Kapitel 8: Integrationen

AuraGo lässt sich nahtlos in verschiedene Dienste und Plattformen integrieren.

## Integrationen über die Web-UI einrichten

Die bevorzugte Art, Integrationen zu konfigurieren, ist die Web-UI:

1. Öffne die AuraGo Web-UI im Browser.
2. Navigiere zu **Menü → Config → Integrationen**.
3. Suche die gewünschte Integration in der Liste.
4. Aktiviere den Toggle **Enabled**.
5. Fülle die Pflichtfelder aus (z. B. URL, Host, Username).
6. Speichere Credentials sicher im **Vault** – niemals direkt in der `config.yaml`!
7. Klicke auf **Speichern** und starte AuraGo bei Bedarf neu.

> 💡 **Tipp:** Für fast alle Integrationen gibt es zusätzlich einen `readonly`-Modus. Aktiviere diesen zuerst, um die Verbindung zu testen, bevor du Schreibzugriffe erlaubst.

---

## Telegram Bot Setup

### Bot erstellen
1. Öffne Telegram und suche nach **@BotFather**.
2. Starte mit `/start` und erstelle einen neuen Bot mit `/newbot`.
3. Gib einen Namen und einen Benutzernamen (muss mit "bot" enden) ein.
4. **Speichere den Token** (z. B. `123456789:ABC...`).

### User-ID ermitteln
1. Suche nach **@userinfobot** und starte ihn.
2. Notiere deine numerische ID.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Telegram**.
2. Aktiviere die Integration.
3. Trage die **User-ID** ein.
4. Speichere das **Bot-Token** im Vault.
5. Speichern und AuraGo neu starten.
6. Sende eine Testnachricht an deinen Bot.

### YAML-Referenz
```yaml
telegram:
  bot_token: "123456789:ABC..."
  telegram_user_id: 12345678
```

## Discord Bot Setup

### Discord-Anwendung erstellen
1. Besuche das [Discord Developer Portal](https://discord.com/developers/applications).
2. Klicke auf **New Application** und gib einen Namen ein (z. B. "AuraGo").
3. Gehe zu **Bot → Add Bot**.

### Token und Berechtigungen
1. Kopiere den **Bot-Token**.
2. Aktiviere unter **Privileged Gateway Intents**:
   - **Message Content Intent**
   - **Server Members Intent**

### Bot zum Server einladen
1. Gehe zu **OAuth2 → URL Generator**.
2. Scopes: `bot`, `applications.commands`
3. Permissions: `Send Messages`, `Read Messages`
4. Öffne die generierte URL und wähle deinen Server.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Discord**.
2. Aktiviere die Integration.
3. Trage **Guild ID** und **Default Channel ID** ein.
4. Speichere das **Bot-Token** im Vault.
5. Trage eine **Allowed User ID** ein. Ohne diese ID blockiert AuraGo eingehende Discord-Nachrichten.

### YAML-Referenz
```yaml
discord:
  enabled: true
  bot_token: "DEIN-TOKEN"
  guild_id: "123456789012345678"
  allowed_user_id: "987654321098765432"
  default_channel_id: "123456789012345678"
```

## E-Mail (IMAP/SMTP) Konfiguration

Verbinde AuraGo mit einem E-Mail-Konto, um E-Mails zu senden und empfangen.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → E-Mail**.
2. Aktiviere die Integration.
3. Trage IMAP-Host, IMAP-Port, SMTP-Host und SMTP-Port ein.
4. Gib die E-Mail-Adresse als Username und From-Adresse an.
5. Speichere das **Passwort** im Vault (nicht in der Config!).
6. Aktiviere bei Bedarf **Watch Enabled**, um den Posteingang regelmäßig zu prüfen.

### Gmail App-Passwort verwenden
Für Gmail musst du ein [App-Passwort](https://myaccount.google.com/apppasswords) erstellen:
1. Google-Konto → Sicherheit → 2-Schritt-Verification aktivieren.
2. App-Passwörter → Andere (benutzerdefinierter Name).
3. Das generierte Passwort im Vault speichern.

### Provider-Einstellungen

| Provider | IMAP-Host | SMTP-Host |
|----------|-----------|-----------|
| Gmail | `imap.gmail.com` | `smtp.gmail.com` |
| Outlook | `outlook.office365.com` | `smtp.office365.com` |
| GMX | `imap.gmx.net` | `mail.gmx.net` |
| Web.de | `imap.web.de` | `smtp.web.de` |

### YAML-Referenz
```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  smtp_host: "smtp.gmail.com"
  username: "dein.email@gmail.com"
```

## Home Assistant Integration

Steuere Smart-Home-Geräte über AuraGo.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Home Assistant**.
2. Aktiviere die Integration und trage die URL ein (z. B. `http://homeassistant.local:8123`).
3. Erstelle in Home Assistant ein **Long-Lived Access Token**:
   - Home Assistant → Profil (unten links) → Long-Lived Access Tokens → Create Token.
4. Speichere den Token im AuraGo-Vault.

### Verwendung im Chat
- "Schalte das Licht im Wohnzimmer an."
- "Wie ist die Temperatur im Schlafzimmer?"

### YAML-Referenz
```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  allowed_services:
    - "light.turn_on"
    - "light.turn_off"
  blocked_services:
    - "lock.unlock"
```

`allowed_services` ist eine explizite Allowlist für `call_service`; leer blockiert Service-Aufrufe, während Statusabfragen weiter möglich sind.

## MQTT Integration

Für IoT-Geräte und Smart-Home-Automation.

**Web-UI:** Config → Integrationen → MQTT → Broker-URL, Client-ID und optional Username/Passwort eingeben. Topics zur Subscription hinzufügen.

### YAML-Referenz
```yaml
mqtt:
  enabled: true
  broker: "mqtt.example.com"
  topics:
    - "home/+/sensors"
```

## Docker Integration

Verwalte Docker-Container über AuraGo.

**Web-UI:** Config → Integrationen → Docker → Host-URL eingeben (z. B. `unix:///var/run/docker.sock`).

> ⚠️ **Sicherheit:** Der Docker-Zugriff ermöglicht volle Host-Kontrolle. Aktiviere `readonly` für mehr Sicherheit.

### YAML-Referenz
```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
```

## Proxmox Integration

VM- und Container-Verwaltung.

**Web-UI:** Config → Integrationen → Proxmox → URL, Node-Name und Token-ID eingeben. Das Token wird im Vault gespeichert.

### YAML-Referenz
```yaml
proxmox:
  enabled: true
  readonly: false
  allow_destructive: false
  url: "https://proxmox.example.com:8006"
  node: "pve"
  token_id: "user@pam!tokenname"
```

## Webhooks

Webhooks ermöglichen es externen Diensten, AuraGo zu benachrichtigen.

**Web-UI:** Config → Integrationen → Webhooks → aktivieren und Limits konfigurieren. Einzelne Webhooks werden über die API oder das Dashboard verwaltet.

### YAML-Referenz
```yaml
webhooks:
  enabled: true
  max_payload_size: 65536
  rate_limit: 60
```

## Budget Tracking

Überwache die Kosten für LLM-API-Aufrufe.

**Web-UI:** Config → Integrationen → Budget → Tageslimit, Warnschwelle und Durchsetzungsmodus einstellen.

### YAML-Referenz
```yaml
budget:
  enabled: true
  daily_limit_usd: 1.0
  enforcement: "warn"
```

## Google Workspace

Zugriff auf Gmail, Kalender, Drive, Docs und Sheets.

**Web-UI:** Config → Integrationen → Google Workspace → Gewünschte Dienste aktivieren und OAuth2-Client-ID eintragen. Die Authentifizierung läuft über die Web-UI, das Token wird im Vault gespeichert.

### YAML-Referenz
```yaml
google_workspace:
  enabled: true
  client_id: ""
```

## WebDAV/Koofr

### WebDAV
**Web-UI:** Config → Integrationen → WebDAV → URL und Username eingeben. Passwort im Vault speichern.

### Koofr
**Web-UI:** Config → Integrationen → Koofr → Username und App-Passwort eingeben.

### YAML-Referenz
```yaml
webdav:
  enabled: true
  url: "https://cloud.example.com/remote.php/dav/files/username/"

koofr:
  enabled: true
  username: "user@example.com"
```

## Tailscale

VPN-Status und -Verwaltung.

**Web-UI:** Config → Integrationen → Tailscale → Tailnet-Name eingeben. Für den eingebetteten tsnet-Node können Hostname, Ports und Funnel separat aktiviert werden.

### YAML-Referenz
```yaml
tailscale:
  enabled: true
  tailnet: "tailnet.ts.net"
```

## Brave Search

Erweiterte Websuche über Brave Search API.

**Web-UI:** Config → Integrationen → Brave Search → API-Key eingeben (wird im Vault gespeichert).

### YAML-Referenz
```yaml
brave_search:
  enabled: true
  api_key: "BS..."
```

## GitHub Integration

Repository- und Issue-Verwaltung.

**Web-UI:** Config → Integrationen → GitHub → Username und optional GitHub Enterprise Base-URL eingeben.

### YAML-Referenz
```yaml
github:
  enabled: true
  owner: "username"
```

## Ollama Integration

Lokale LLM-Verwaltung.

**Web-UI:** Config → Integrationen → Ollama → URL eingeben (z. B. `http://localhost:11434`). Optional: Verwaltung eines lokalen Docker-Containers aktivieren.

### Managed Ollama

Wenn Managed Mode aktiviert ist, verwaltet AuraGo einen Docker-Container `aurago_ollama_managed`. AuraGo kann Modell-Daten in einem persistenten Volume halten, verfügbare GPU-Unterstützung erkennen (NVIDIA, AMD, Intel soweit Docker-seitig verfügbar), Speicherlimits setzen, Standardmodelle automatisch ziehen und den Container über Web-UI oder API neu erstellen.

Prüfe den Status mit `GET /api/ollama/managed/status`; erstelle den Container nach Konfigurationsänderungen mit `POST /api/ollama/managed/recreate` neu.

### YAML-Referenz
```yaml
ollama:
  enabled: true
  url: "http://localhost:11434"
  managed:
    enabled: true
    image: "ollama/ollama:latest"
    default_models: ["llama3.1"]
```

## MeshCentral

Remote-Desktop und -Verwaltung.

**Web-UI:** Config → Integrationen → MeshCentral → URL und Username eingeben. Passwort im Vault speichern.

### YAML-Referenz
```yaml
meshcentral:
  enabled: true
  url: "https://mesh.example.com"
  username: "admin"
```

## Ansible Integration

Playbook-Ausführung.

**Web-UI:** Config → Integrationen → Ansible → Modus (sidecar/remote), URL, Timeout und Verzeichnisse konfigurieren.

### YAML-Referenz
```yaml
ansible:
  enabled: true
  mode: sidecar
  url: "http://localhost:5000"
```

## TrueNAS Integration

Verwalte ZFS-Storage-Pools, Datasets, Snapshots und Shares.

**Web-UI:** Config → Integrationen → TrueNAS → Host, Port und HTTPS aktivieren. API-Key im Vault speichern.

### YAML-Referenz
```yaml
truenas:
  enabled: true
  readonly: false
  host: "truenas.local"
  port: 443
  use_https: true
  allow_destructive: false
```

## FritzBox Integration

Steuere AVM Fritz!Box-Router über TR-064.

**Web-UI:** Config → Integrationen → FritzBox → Host, Username und gewünschte Module (System, Netzwerk, Smart-Home, etc.) aktivieren. Passwort im Vault speichern.

### YAML-Referenz
```yaml
fritzbox:
  enabled: true
  host: "fritz.box"
  username: "admin"
```

## AdGuard Home Integration

DNS-Filterung und -Blockierung verwalten.

**Web-UI:** Config → Integrationen → AdGuard → URL und Username eingeben. Passwort im Vault speichern.

### YAML-Referenz
```yaml
adguard:
  enabled: true
  url: "http://adguard.local:3000"
```

## n8n Integration

Verbindung mit der n8n Workflow-Automatisierungsplattform. AuraGo kann n8n gezielt API-Zugriff geben, ausgewählte Tools ausführen, isolierte Chat-Sessions starten, Memory durchsuchen oder speichern und Missionen aus Workflows heraus erstellen oder verwalten.

**Web-UI:** Config → Integrationen → n8n → Webhook Base-URL, erlaubte Tools/Scopes, Token/HMAC-Schutz und Rate-Limits konfigurieren.

> 💡 AuraGo bietet einen offiziellen n8n Community Node: `@antibyte/n8n-nodes-aurago`

### Scopes und Fähigkeiten

| Scope | Erlaubt |
|-------|---------|
| `n8n:chat` | Isolierte AuraGo-Chat-Sessions aus n8n starten oder fortsetzen |
| `n8n:tools` | Explizit erlaubte AuraGo-Tools aus Workflows ausführen |
| `n8n:memory` | Memory aus Workflows durchsuchen oder Einträge speichern |
| `n8n:missions` | Mission-Control-Aufgaben erstellen, aktualisieren, auslösen oder prüfen |
| `n8n:admin` | Administrative Operationen; nur aktivieren, wenn wirklich nötig |

Nutze `readonly: true` für Workflows, die nur lesen dürfen. `scopes` und `allowed_tools` sind explizite Allowlists; leer deaktiviert die jeweilige Fähigkeit.

### YAML-Referenz
```yaml
n8n:
  enabled: true
  readonly: false
  webhook_base_url: "https://n8n.deinedomain.com/webhook"
  allowed_events: ["message", "tool_result"]
  require_token: true
  allowed_tools: ["query_memory", "manage_missions"]
  rate_limit_rps: 10
  scopes: ["n8n:chat", "n8n:tools", "n8n:memory"]
```

## Notifications

Push-Benachrichtigungen über ntfy oder Pushover.

**Web-UI:** Config → Integrationen → Notifications → ntfy-URL/Topic oder Pushover-Credentials eingeben.

### YAML-Referenz
```yaml
notifications:
  ntfy:
    enabled: true
    topic: "aurago-alerts"
```

## Web Push / PWA-Benachrichtigungen

AuraGo unterstützt zusätzlich Browser-Web-Push für die installierbare PWA. Das ist unabhängig von ntfy und Pushover: Browser abonnieren Push über VAPID-Schlüssel, Subscriptions werden in `data/push.db` gespeichert und AuraGo kann lokale Browser-Benachrichtigungen senden.

### API-Endpunkte

| Endpunkt | Zweck |
|----------|-------|
| `GET /api/push/vapid-pubkey` | Öffentlichen VAPID-Schlüssel abrufen |
| `POST /api/push/subscribe` | Browser-Push-Subscription registrieren |
| `POST /api/push/unsubscribe` | Aktuelle Subscription entfernen |
| `GET /api/push/status` | Push-Verfügbarkeit und Subscription-Status prüfen |

Web Push benötigt HTTPS oder `localhost`, da Browser Service-Worker-Push auf unsicheren Origins blockieren.

## Telnyx Integration

SMS senden/empfangen und Sprachanrufe über Telnyx.

**Web-UI:** Config → Integrationen → Telnyx → Telefonnummer, Messaging Profile ID und Connection ID eingeben. API-Key im Vault speichern.

### YAML-Referenz
```yaml
telnyx:
  enabled: true
  phone_number: "+491234567890"
  allowed_numbers:
    - "+491234567890"
```

`allowed_numbers` ist eine explizite E.164-Allowlist für eingehende Anrufe/SMS und ausgehende Benachrichtigungen. Leer bedeutet: Telnyx-Verkehr bleibt blockiert, bis Nummern konfiguriert sind.

## VirusTotal Integration

Dateien und URLs auf Malware prüfen.

**Web-UI:** Config → Integrationen → VirusTotal → API-Key eingeben.

### YAML-Referenz
```yaml
virustotal:
  enabled: true
```

## MCP (Model Context Protocol)

Verbinde externe MCP-Server (Client) oder stelle AuraGo selbst als MCP-Server bereit.

**Web-UI:** Config → Integrationen → MCP → Client/Server konfigurieren.

### MCP-Client
Erlaubt dem Agenten, Tools von externen MCP-Servern zu nutzen.

```yaml
mcp:
  enabled: true
  servers:
    - name: "fetch-server"
      command: "uvx"
      args: ["mcp-server-fetch"]
      allowed_tools: []  # optionale Allowlist; leer bedeutet alle entdeckten nicht-destruktiven Tools
      allow_destructive: false
```

Beim MCP-Client ist `allowed_tools` pro Server optional. Leer lassen oder weglassen erlaubt alle entdeckten nicht-destruktiven Tools; trage Toolnamen nur ein, wenn Ausführung und Routing auf diese Teilmenge begrenzt werden sollen.

### MCP-Server
Stellt AuraGo-Tools für externe Clients bereit.

```yaml
mcp_server:
  enabled: true
  port: 8089
  allowed_tools:
    - "execute_shell"
    - "filesystem"
```

`allowed_tools` ist eine explizite serverseitige Allowlist. Leer veröffentlicht keine AuraGo-Tools; `vscode_debug_bridge` nutzt ein eigenes begrenztes Debugging-Preset.

## SQL Connections – Externe Datenbanken

Verbinde AuraGo mit PostgreSQL, MySQL/MariaDB oder SQLite.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → SQL Connections**.
2. Aktiviere die Integration.
3. Lege Verbindungen an: Name, Datenbank-Typ, Host, Port, Datenbank, Benutzer.
4. Speichere Passwörter im Vault.
5. Passe bei Bedarf `max_result_rows` und Timeouts an.

> 💡 **Sicherheit:** Verwende nach Möglichkeit dedizierte Read-Only-Benutzer.

### YAML-Referenz
```yaml
sql_connections:
  enabled: true
  max_result_rows: 1000
  connections:
    - name: "produktion"
      driver: "postgres"
      host: "db.example.com"
      port: 5432
      database: "aurago"
      username: "aurago_readonly"
      password_vault_key: "sql_prod_password"
      read_only: true
      max_pool_size: 5
      connect_timeout: 10
      query_timeout: 30
```

## S3-kompatible Cloud Storage

Zugriff auf S3, MinIO, Wasabi und andere S3-kompatible Speicher.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → S3**.
2. Aktiviere die Integration.
3. Trage Endpoint, Region und optional Standard-Bucket ein.
4. Aktiviere **Path Style** für MinIO.
5. Speichere Access Key und Secret Key im Vault.

### YAML-Referenz
```yaml
s3:
  enabled: true
  endpoint: "https://s3.amazonaws.com"
  region: "us-east-1"
```

## OneDrive Integration

Zugriff auf Microsoft OneDrive über Microsoft Graph API.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → OneDrive**.
2. Aktiviere die Integration.
3. Trage **Client ID** und **Tenant ID** ein.
4. Starte die OAuth2-Authentifizierung über die Web-UI.
5. Speichere und starte neu.

### YAML-Referenz
```yaml
onedrive:
  enabled: true
  client_id: "YOUR_CLIENT_ID"
  tenant_id: "common"
  client_secret_vault_key: "onedrive_client_secret"
  graph_scopes:
    - "Files.Read"
    - "Files.ReadWrite"
  upload_folder: "AuraGo"
```

## Homepage Integration

Erstelle und deploye persönliche Startseiten/Dashboards.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Homepage**.
2. Aktiviere die Integration.
3. Konfiguriere Deployment-Host, Benutzer und Zielpfad.
4. Optional: Aktiviere lokalen Webserver (`allow_local_server`).
5. Speichere und starte neu.

### YAML-Referenz
```yaml
homepage:
  enabled: true
  deploy_host: "server.example.com"
  deploy_user: "deploy"
  deploy_path: "/var/www/homepage"
  webserver_config_path: "/etc/nginx/sites-available/homepage"
  allow_deploy: true
  allow_local_server: false
```

## Cloudflare Tunnel

Sicherer Tunnel für Remote-Zugriff ohne öffentliche IP oder Port-Forwarding.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Cloudflare Tunnel**.
2. Wähle den Modus (`auto`, `docker`, `native`) und die Auth-Methode (`token`, `named`, `quick`).
3. Trage **Account ID** und optional **Tunnel Name** ein.
4. Speichere den **Connector Token** im Vault.

### Connector Token erhalten
1. [Cloudflare Zero Trust](https://one.dash.cloudflare.com) → Networks → Tunnels.
2. "Create a tunnel" → Cloudflared → Name vergeben.
3. Kopiere den Token und speichere ihn im Vault unter `cloudflare_tunnel_token`.

### YAML-Referenz
```yaml
cloudflare_tunnel:
  enabled: true
  mode: auto
  auth_method: token
```

## Cloudflare AI Gateway

Routing und Monitoring für LLM-Traffic über Cloudflare AI Gateway.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → AI Gateway**.
2. Aktiviere die Integration.
3. Trage **Account ID** und **Gateway ID** ein.
4. Optional: Aktiviere `log_requests` für detailliertes Logging.
5. Speichere und starte neu.

### YAML-Referenz
```yaml
ai_gateway:
  enabled: true
  account_id: "YOUR_ACCOUNT_ID"
  gateway_id: "YOUR_GATEWAY_ID"
  log_requests: false
```

## Chromecast Integration

Sende Text-to-Speech und Medien an Chromecast-Geräte.

**Web-UI:** Config → Integrationen → Chromecast → TTS-Port konfigurieren.

> 💡 Voraussetzung: Chromecast-Gerät im gleichen Netzwerk und TTS konfiguriert.

### YAML-Referenz
```yaml
chromecast:
  enabled: true
  tts_port: 8090
```

## Media Registry

Zentrale Verwaltung von Mediendateien mit Metadaten-Tracking.

**Web-UI:** Config → Integrationen → Media Registry → aktivieren.

### YAML-Referenz
```yaml
media_registry:
  enabled: true
```

## Netlify Integration

Deploye statische Webseiten direkt auf Netlify.

**Web-UI:** Config → Integrationen → Netlify → Site-ID und Team-Slug eingeben. Personal Access Token im Vault speichern.

### YAML-Referenz
```yaml
netlify:
  enabled: true
```

## Paperless NGX

Dokumentenmanagement und Durchsuchung.

**Web-UI:** Config → Integrationen → Paperless NGX → URL eingeben. API-Key im Vault speichern.

### YAML-Referenz
```yaml
paperless_ngx:
  enabled: true
  url: "https://paperless.local"
```

## LLM Guardian

Sicherheits- und Policy-Engine für eingehende und ausgehende Inhalte.

**Web-UI:** Config → Integrationen → LLM Guardian → Provider, Modell und Stärke-Level konfigurieren.

### YAML-Referenz
```yaml
llm_guardian:
  enabled: true
  default_level: "medium"
```

## Remote Control

Empfange Fernsteuerungs-Befehle von anderen AuraGo-Instanzen.

**Web-UI:** Config → Integrationen → Remote Control → Discovery-Port und erlaubte Pfade konfigurieren.

> ⚠️ **Sicherheit:** Aktiviere `auto_approve` nur in vertrauenswürdigen Netzwerken.

### YAML-Referenz
```yaml
remote_control:
  enabled: true
  discovery_port: 8092
  allowed_paths:
    - "/home/aurago"
```

`allowed_paths` ist eine explizite Allowlist für Remote-Dateioperationen. Leer blockiert Remote-Dateilesen, -schreiben und Verzeichnislisten.

## Sandbox

Isolierte Ausführung von Python-Code und externen Befehlen.

**Web-UI:** Config → Integrationen → Sandbox → Backend, Timeout und Netzwerkzugriff konfigurieren.

### YAML-Referenz
```yaml
sandbox:
  enabled: true
  backend: docker
```

## Skill Manager

Verwalte hochgeladene Python-Skills.

**Web-UI:** Config → Integrationen → Skill Manager → Uploads erlauben und Guardian-Scan aktivieren.

### YAML-Referenz
```yaml
tools:
  skill_manager:
    enabled: true
    allow_uploads: true
```

## Jellyfin Integration

Media-Server-Verwaltung.

**Web-UI:** Config → Integrationen → Jellyfin → URL eingeben. API-Key im Vault speichern.

### YAML-Referenz
```yaml
jellyfin:
  enabled: true
  url: "https://jellyfin.local:8096"
  readonly: false
  allow_destructive: false
```

## Image Generation

Generiere Bilder über unterstützte Provider.

**Web-UI:** Config → Integrationen → Image Generation → Provider, Modell und Limits einstellen. API-Key im Vault speichern.

### YAML-Referenz
```yaml
image_generation:
  enabled: true
  provider: ""
```

## Video Generation

Generiere kurze Videos aus Text-Prompts oder Bildvorlagen. Unterstützte Provider sind je nach Konfiguration MiniMax Hailuo und Google Veo.

### Fähigkeiten

| Fähigkeit | Beschreibung |
|-----------|--------------|
| Text-zu-Video | Video direkt aus einem Prompt generieren |
| Bild-zu-Video | Erstes Frame als visuelle Vorgabe nutzen |
| Start-/Endframe | Provider-unterstützte Kontrolle über erstes/letztes Frame |
| Referenzbilder | Provider-unterstützte Bildreferenzen |
| Media Registry | Generierte MP4-Dateien werden automatisch registriert |
| Tageslimits | Kosten und Provider-Nutzung mit `max_daily` begrenzen |

**Web-UI:** Config → Integrationen → Video Generation → Provider (`minimax` oder `google`), Modell, Dauer, Auflösung, Seitenverhältnis und Tageslimit konfigurieren. Zugangsdaten im Vault speichern.

### YAML-Referenz
```yaml
video_generation:
  enabled: true
  provider: "minimax"
  model: "hailuo-02"
  duration_seconds: 6
  resolution: "720p"
  aspect_ratio: "16:9"
  max_daily: 5
```

Im Chat nutzt der Agent dafür `generate_video`. Bereits vorhandene Videodateien kann `send_video` mit Inline-Player an den Benutzer senden.

## Fallback LLM

Failover-LLM, der automatisch aktiviert wird, wenn der Haupt-Provider ausfällt.

**Web-UI:** Config → Integrationen → Fallback LLM → Modell und Schwellenwert konfigurieren.

### YAML-Referenz
```yaml
fallback_llm:
  enabled: true
  model: ""
```

## Co-Agents

Spezialisierte Sub-Agenten für Recherche, Coding, Design und mehr.

**Web-UI:** Config → Integrationen → Co-Agents → Spezialisten einzeln aktivieren und eigene Provider zuweisen.

### YAML-Referenz
```yaml
co_agents:
  enabled: true
  max_concurrent: 3
```

## Mission Preparation

Analysiert Missionen vor der Ausführung. Der Dienst lässt ein LLM vor geplanten oder manuellen Missionen strukturierte Ausführungshinweise erstellen.

Mission Preparation kann benötigte Tools, Schrittpläne, mögliche Fallstricke, Entscheidungspunkte, Preload-Hinweise und einen Confidence-Score erzeugen. Ergebnisse werden anhand eines Mission-Checksums gecacht und bei Änderungen invalidiert. Geplante Missionen können automatisch vorbereitet werden.

**Web-UI:** Config → Integrationen → Mission Preparation → aktivieren und Timeout/Confidence-Level einstellen.

### YAML-Referenz
```yaml
mission_preparation:
  enabled: true
  timeout_seconds: 120
  auto_prepare_scheduled: true
  min_confidence: 0.6
```

## Rocket.Chat Integration

Für selbst-gehostete Rocket.Chat-Instanzen.

**Web-UI:** Config → Integrationen → Rocket.Chat → URL, User-ID und Channel eingeben.

### YAML-Referenz
```yaml
rocketchat:
  enabled: true
  url: "https://chat.example.com"
  channel: "#general"
```

## TTS / Whisper

Sprachsynthese (TTS) und Spracherkennung.

**Web-UI:** Config → Integrationen → TTS → Provider (Piper, ElevenLabs, Google) und Voice-Einstellungen konfigurieren.

### YAML-Referenz
```yaml
tts:
  provider: "piper"
  language: "de"
  cache_retention_hours: 24
  cache_max_files: 100
  piper:
    voice: "de_DE-thorsten-high"
    container_port: 10200
```

## A2A Protocol

AuraGo unterstützt das Google A2A (Agent-to-Agent) Protokoll für die Kommunikation zwischen KI-Agenten. AuraGo kann eine Agent Card veröffentlichen, damit andere A2A-Clients Name, Fähigkeiten, Endpunkte und Auth-Anforderungen erkennen. Umgekehrt kann AuraGo als A2A-Client Remote Agents registrieren und Aufgaben delegieren.

A2A eignet sich, wenn mehrere autonome Agenten Aufgaben austauschen sollen, ohne eine gemeinsame Chat-Session zu teilen. AuraGo unterstützt REST-, JSON-RPC- und gRPC-Bindings, Streaming und Push Notifications, sofern aktiviert.

**Web-UI:** Config → Integrationen → A2A → Server-Port, Agent Card und Remote Agents konfigurieren.

### YAML-Referenz
```yaml
a2a:
  server:
    enabled: true
    port: 0
    base_path: "/a2a"
    agent_name: "AuraGo"
    streaming: true
    push_notifications: true
    bindings:
      rest: true
      json_rpc: true
      grpc: true
      grpc_port: 50051
  client:
    enabled: true
    remote_agents: []
  auth:
    api_key_enabled: true
    bearer_enabled: true
```

Die öffentliche Agent Card bleibt für Discovery ohne Authentifizierung erreichbar. Alle anderen A2A-Endpunkte benötigen mindestens eine konfigurierte Auth-Methode; API-Key oder Bearer-Secret müssen vor dem Exponieren im Vault liegen.

## Music Generation

KI-Musik-Generierung über unterstützte Provider.

**Web-UI:** Config → Integrationen → Music Generation → Provider und Limits einstellen.

### YAML-Referenz
```yaml
music_generation:
  enabled: true
  provider: ""
  model: ""
  max_daily: 10
```

## Firewall

Linux-Firewall-Überwachung und -Verwaltung (iptables/ufw).

**Web-UI:** Config → Integrationen → Firewall → Modus und Polling-Intervall konfigurieren.

### YAML-Referenz
```yaml
firewall:
  enabled: true
  mode: "readonly"
  poll_interval_seconds: 60
```

## Invasion Control

Remote-Deployment-System für AuraGo-Worker (Eggs) in verschiedenen Nests.

**Web-UI:** Config → Invasion Control → Nests und Eggs verwalten.

### YAML-Referenz
```yaml
invasion_control:
  enabled: true
  readonly: false
```

## Document Creator (Gotenberg)

PDF-Erstellung und Dokumenten-Konvertierung. Unterstützt den eingebauten Maroto-Backend oder einen externen Gotenberg-Sidecar.

**Web-UI:** Config → Integrationen → Document Creator → Backend wählen (maroto/gotenberg).

### YAML-Referenz
```yaml
tools:
  document_creator:
    enabled: true
    backend: "maroto"
    output_dir: "data/documents"
    gotenberg:
      url: "http://gotenberg:3000"
      timeout: 120
```

---

## Security Proxy

Schutzschicht für öffentlich erreichbare AuraGo-Instanzen mit Rate-Limiting, IP-Filter und Geo-Blocking. AuraGo verwaltet den Proxy als Caddy-basierten Docker-Container, lädt die generierte Konfiguration neu und stellt Lifecycle-Aktionen sowie Logs per API bereit.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Security Proxy**.
2. Aktiviere den Proxy.
3. Konfiguriere Rate-Limiting (Requests pro Minute).
4. Optional: Definiere erlaubte/blockierte IPs oder Länder.
5. Speichere und starte neu.

### YAML-Referenz
```yaml
security_proxy:
  enabled: true
  domain: "aurago.example.com"
  rate_limiting:
    enabled: true
    requests_per_minute: 60
  ip_filter:
    enabled: false
    allowed_ips: []
    blocked_ips: []
  geo_blocking:
    enabled: false
    blocked_countries: []
```

### Runtime API

| Endpunkt | Zweck |
|----------|-------|
| `GET /api/proxy/status` | Aktueller Proxy-/Containerstatus |
| `POST /api/proxy/start` | Verwalteten Proxy-Container starten |
| `POST /api/proxy/stop` | Proxy stoppen |
| `POST /api/proxy/destroy` | Verwalteten Proxy-Container entfernen |
| `POST /api/proxy/reload` | Caddy-Konfiguration neu generieren und laden |
| `GET /api/proxy/logs` | Aktuelle Proxy-Logs abrufen |

---

## Egg Mode (Invasion Control)

Verbinde mehrere AuraGo-Instanzen zu einem verteilten Nest (Cluster). Einzelne Instanzen werden als „Eggs“ bezeichnet.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Egg Mode**.
2. Aktiviere **Egg Mode**.
3. Trage die **Master-URL** der Hauptinstanz ein.
4. Optional: Vergebe **Egg ID** und **Nest ID**.
5. Speichere und starte neu.

### YAML-Referenz
```yaml
egg_mode:
  enabled: false
  master_url: "https://master.aurago.local:8088"
  egg_id: "egg-01"
  nest_id: "nest-main"
  api_key_vault_key: "egg_api_key"
```

## LDAP Integration

Authentifizierung und Benutzerverwaltung über LDAP/Active Directory.

**Web-UI:** Config → Integrationen → LDAP → Server-URL, Base DN und Bind-Credentials konfigurieren.

### YAML-Referenz
```yaml
ldap:
  enabled: true
  url: "ldap://ldap.example.com:389"
  base_dn: "dc=example,dc=com"
  bind_dn: "cn=admin,dc=example,dc=com"
  use_tls: true
  insecure_skip_verify: false
```

## Obsidian Integration

Verknüpfe AuraGo mit deinem Obsidian-Vault für Notizen und Wissensmanagement.

**Web-UI:** Config → Integrationen → Obsidian → Vault-Pfad und Synchronisationsmodus konfigurieren.

### YAML-Referenz
```yaml
obsidian:
  enabled: true
  vault_path: "/home/user/obsidian-vault"
```

## Uptime Kuma Integration

Überwache die Erreichbarkeit von Diensten mit Uptime Kuma.

**Web-UI:** Config → Integrationen → Uptime Kuma → URL eingeben. API-Key im Vault speichern.

### YAML-Referenz
```yaml
uptime_kuma:
  enabled: true
  url: "https://uptime-kuma.example.com"
```

## Vercel Integration

Deploye Web-Projekte direkt auf Vercel.

**Web-UI:** Config → Integrationen → Vercel → Team-Slug eingeben. API-Token im Vault speichern.

### YAML-Referenz
```yaml
vercel:
  enabled: true
  team_slug: "my-team"
```

## Browser Automation

Headless-Browser-Automatisierung für Formulare, Screenshots und Web-Interaktionen.

**Web-UI:** Config → Integrationen → Browser Automation → aktivieren und Headless-Modus konfigurieren.

### YAML-Referenz
```yaml
browser_automation:
  enabled: true
  headless: true
  screenshots_dir: "browser_screenshots"
```

## Output Compression

Reduziert den Token-Verbrauch durch Filterung und Deduplizierung von Tool-Ausgaben, bevor sie in den LLM-Kontext gelangen.

**Web-UI:** Config → Agent → Output Compression → aktivieren und Schwellenwerte anpassen.

### YAML-Referenz
```yaml
agent:
  output_compression:
    enabled: true
    min_chars: 500
    preserve_errors: true
    shell_compression: true
    python_compression: true
    api_compression: true
```

---

## YepAPI Integration

Einheitliche API für SEO, SERP, Web-Scraping und Social-Media-Daten (YouTube, TikTok, Instagram, Amazon).

### Web-UI Einrichtung
1. Öffne **Config → Integrationen → YepAPI**.
2. Aktiviere die Integration.
3. Speichere den API-Key im Vault (Schlüssel: `yepapi_api_key`).
4. Konfiguriere bei Bedarf einzelne Dienste ein/aus.
5. Speichere und starte neu.

### Fähigkeiten

| Dienst | Fähigkeiten |
|--------|--------------|
| **SEO** | Keyword-Recherche, Domain-Übersicht, Wettbewerbsanalyse, Backlinks, On-Page-Analyse, Trends |
| **SERP** | Google/Bing/Yahoo/Baidu-Suche, Maps, Bilder, News, YouTube, Autocomplete |
| **Scraping** | Standard, JavaScript-gestützt, Stealth, Screenshots, KI-Extraktion |
| **YouTube** | Video-Suche, Transkripte, Kommentare, Kanäle, Playlists, Shorts |
| **TikTok** | Video-/Benutzersuche, Profile, Posts, Kommentare, Musik, Challenges |
| **Instagram** | Benutzer-/Hashtag-Suche, Profile, Posts, Reels, Stories, Kommentare |
| **Amazon** | Produktsuche, ASIN-Lookup, Rezensionen, Deals, Bestseller |

### YAML-Referenz
```yaml
yepapi:
    enabled: true
    seo:
        enabled: true
    serp:
        enabled: true
    scraping:
        enabled: true
    youtube:
        enabled: true
    tiktok:
        enabled: true
    instagram:
        enabled: true
    amazon:
        enabled: true
```

---

## Inventar-System

Geräteregistrierung mit SSH-Credential-Verwaltung und Wake-on-LAN-Unterstützung.

### Web-UI Einrichtung
1. Öffne **Config → Integrationen → Inventar**.
2. Aktiviere die Integration.
3. Konfiguriere `inventory_path` für einen benutzerdefinierten Datenbankspeicherort.
4. Aktiviere **Wake-on-LAN** falls benötigt.
5. Speichere und starte neu.

### YAML-Referenz
```yaml
sqlite:
    inventory_path: ./data/inventory.db

tools:
    inventory:
        enabled: true
    wol:
        enabled: true
```

### Wichtige Funktionen
- **Geräteregistrierung**: Geräte (Server, VMs, Docker, Netzwerkgeräte) mit IP, Port, SSH-Credentials speichern
- **Wake-on-LAN**: Magic Packets senden um Geräte über gespeicherte MAC-Adressen aufzuwecken
- **Credential-Sicherheit**: SSH-Passwörter und Private Keys im verschlüsselten Vault gespeichert
- **Tag-basierte Organisation**: Geräte nach Tags gruppieren und durchsuchen

Nutze die Tools `register_device`, `query_inventory` und `wake_on_lan` im Chat.

---

## Heartbeat-System

Hintergrund-Wake-up-Scheduler für autonome Statusprüfungen in konfigurierbaren Intervallen.

### Web-UI Einrichtung
1. Öffne **Config → Integrationen → Heartbeat**.
2. Aktiviere die Integration.
3. Konfiguriere **Tagesfenster** (Standard: 08:00–22:00, alle 1h) und **Nachtfenster** (Standard: 22:00–08:00, alle 4h).
4. Wähle was geprüft werden soll: Aufgaben, Termine, E-Mails.
5. Optional: Eigenes Prompt für Heartbeat-Wake-ups hinzufügen.
6. Speichere und starte neu.

### YAML-Referenz
```yaml
heartbeat:
    enabled: true
    check_tasks: true
    check_appointments: true
    check_emails: true
    additional_prompt: "Benachrichtige mich nur bei kritischen Problemen."
    day_time_window:
        start: "08:00"
        end: "22:00"
        interval: "1h"
    night_time_window:
        start: "22:00"
        end: "08:00"
        interval: "4h"
```

### Wichtige Funktionen
- **Zeitbasierte Führung**: Unterschiedliche Wake-up-Prioritäten je nach Tageszeit (Morgen-Check, Mittag-Review, Abend-Zusammenfassung, Nacht-Ruhe-Modus)
- **Overlap-Schutz**: Verhindert parallele Heartbeat-Ausführungen
- **State-Persistenz**: Speichert letzten Laufzeitpunkt um Neustarts zu überstehen
- **Eigene Prompts**: Benutzerdefinierte Anweisungen an jeden Heartbeat-Wake-up anhängen

---

## Knowledge Graph Extraction

LLM-basierte Entity- und Beziehungsextraktion aus Konversationen und Dateien.

### Web-UI Einrichtung
1. Öffne **Config → Integrationen → Knowledge Graph**.
2. Aktiviere die Integration.
3. Konfiguriere **Auto Extraction** für nächtliche Batch-Entity-Extraktion.
4. Aktiviere **Prompt Injection** um KG-Kontext in System-Prompts einzubinden.
5. Setze Limits für Prompt-Node-Anzahl und Zeichenanzahl.
6. Speichere und starte neu.

### YAML-Referenz
```yaml
tools:
    knowledge_graph:
        enabled: true
        readonly: false
        auto_extraction: true
        prompt_injection: true
        max_prompt_nodes: 5
        max_prompt_chars: 800
        retrieval_fusion: true
```

### Wichtige Funktionen
- **Entity-Typen**: person, device, service, software, location, project, concept, event
- **Beziehungen**: runs_on, owns, manages, uses, depends_on, connected_to, related_to, part_of, deployed_on, located_in
- **Confidence Scoring**: Heuristische Qualitätsbewertung (0.0–1.0) pro Extraktion
- **Kreuzanreicherung**: RAG ↔ Knowledge Graph bidirektionale Fusion

---

## Integrationen testen

### Test über Chat
- "Zeige meine Telegram-Config."
- "Sende eine Test-E-Mail an mich."
- "Liste alle Docker-Container."

### Test über Dashboard
1. Öffne die Web-UI und klicke auf **Dashboard**.
2. Scrolle zu **Integrationen**.
3. Grüner Punkt = Verbindung OK.

### Debug-Logging
```yaml
agent:
  debug_mode: true
```

Logs prüfen:
```bash
tail -f log/supervisor.log | grep -i telegram
```

## Fehlerbehebung

| Problem | Lösung |
|---------|--------|
| "Connection refused" | URL und Port prüfen |
| "Unauthorized" | API-Key/Token prüfen |
| "Timeout" | Firewall/Netzwerk prüfen |
| Integration erscheint nicht | `enabled: true` prüfen |

---

**Nächstes Kapitel:** [Kapitel 9: Gedächtnis & Wissen](./09-memory.md)

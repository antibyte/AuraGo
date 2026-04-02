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
5. Optional: Eine **Allowed User ID** eintragen, um den Bot auf einen einzigen Nutzer zu beschränken.

### YAML-Referenz
```yaml
discord:
  enabled: true
  bot_token: "DEIN-TOKEN"
  guild_id: "123456789012345678"
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
```

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
  url: "https://proxmox.example.com:8006"
  node: "pve"
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

### YAML-Referenz
```yaml
ollama:
  enabled: true
  url: "http://localhost:11434"
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
  host: "truenas.local"
  use_https: true
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

Verbindung mit der n8n Workflow-Automatisierungsplattform.

**Web-UI:** Config → Integrationen → n8n → Base-URL und API-Key eingeben.

> 💡 AuraGo bietet einen offiziellen n8n Community Node: `@antibyte/n8n-nodes-aurago`

### YAML-Referenz
```yaml
n8n:
  enabled: true
  base_url: "https://n8n.deinedomain.com"
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

## Telnyx Integration

SMS senden/empfangen und Sprachanrufe über Telnyx.

**Web-UI:** Config → Integrationen → Telnyx → Telefonnummer, Messaging Profile ID und Connection ID eingeben. API-Key im Vault speichern.

### YAML-Referenz
```yaml
telnyx:
  enabled: true
  phone_number: "+491234567890"
```

## VirusTotal Integration

Dateien und URLs auf Malware prüfen.

**Web-UI:** Config → Integrationen → VirusTotal → API-Key eingeben.

### YAML-Referenz
```yaml
virustotal:
  enabled: true
```

## MCP (Model Context Protocol)

Verbinde externe MCP-Server oder stelle AuraGo als MCP-Server bereit.

**Web-UI:** Config → Integrationen → MCP → Allowed Tools auswählen und Server-Konfiguration hinzufügen.

### YAML-Referenz
```yaml
mcp:
  enabled: true
  allowed_tools:
    - "fetch"
```

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

**Web-UI:** Config → Integrationen → OneDrive → Client ID und Tenant ID eingeben. Die OAuth2-Authentifizierung läuft über die Web-UI.

### YAML-Referenz
```yaml
onedrive:
  enabled: true
  tenant_id: "common"
```

## Homepage Integration

Erstelle und deploye persönliche Startseiten/Dashboards.

**Web-UI:** Config → Integrationen → Homepage → Deployment-Host, Benutzer und Zielpfad konfigurieren.

### YAML-Referenz
```yaml
homepage:
  enabled: true
  deploy_host: "server.example.com"
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

**Web-UI:** Config → Integrationen → AI Gateway → Account ID und Gateway ID eingeben.

### YAML-Referenz
```yaml
ai_gateway:
  enabled: true
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
```

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

**Web-UI:** Config → Integrationen → Jellyfin → URL und Username eingeben. Passwort im Vault speichern.

### YAML-Referenz
```yaml
jellyfin:
  enabled: true
  url: "https://jellyfin.local:8096"
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

Analysiert Missionen vor der Ausführung.

**Web-UI:** Config → Integrationen → Mission Preparation → aktivieren und Timeout/Confidence-Level einstellen.

### YAML-Referenz
```yaml
mission_preparation:
  enabled: true
  timeout_seconds: 120
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
  enabled: true
  provider: "piper"
```

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

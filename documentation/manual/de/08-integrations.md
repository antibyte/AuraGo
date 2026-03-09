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
| **Webhooks** | API | Eingehende HTTP-Events | `webhooks:` |
| **Budget Tracking** | Finanzen | Kostenkontrolle | `budget:` |
| **Google Workspace** | Produktivität | Gmail, Kalender, Drive | `agent.enable_google_workspace` |
| **WebDAV/Koofr** | Speicher | Cloud-Dateizugriff | `webdav:`, `koofr:` |
| **Tailscale** | Netzwerk | VPN-Status | `tailscale:` |
| **Brave Search** | Suche | Websuche API | `brave_search:` |
| **GitHub** | Entwicklung | Repository-Verwaltung | `github:` |
| **Ollama** | AI | Lokale Modelle | `ollama:` |
| **MeshCentral** | Remote | Fernwartung | `meshcentral:` |
| **Ansible** | Automation | Playbook-Ausführung | `ansible:` |
| **Notifications** | Alerts | Push-Benachrichtigungen | `notifications:` |

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

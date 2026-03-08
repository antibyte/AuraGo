# Kapitel 8: Integrationen

AuraGo lässt sich nahtlos in verschiedene Dienste und Plattformen integrieren. Dieses Kapitel erklärt alle verfügbaren Integrationen und deren Einrichtung.

## Übersicht aller Integrationen

| Integration | Typ | Zweck |
|-------------|-----|-------|
| **Telegram** | Chat | Mobile Benachrichtigungen & Chat |
| **Discord** | Chat | Community-Integration |
| **E-Mail** | Kommunikation | IMAP/SMTP für Senden/Empfangen |
| **Home Assistant** | Smart Home | Gerätesteuerung |
| **Docker** | Infrastruktur | Container-Verwaltung |
| **Webhooks** | API | Eingehende HTTP-Events |
| **Budget Tracking** | Finanzen | Kostenkontrolle |
| **Google Workspace** | Produktivität | Kalender, Gmail, Drive |
| **WebDAV/Koofr** | Speicher | Cloud-Dateizugriff |
| **MQTT** | IoT | Geräte-Kommunikation |
| **Proxmox** | Infrastruktur | VM-Verwaltung |
| **Tailscale** | Netzwerk | VPN-Status |

> 💡 **Tipp:** Integrationen können kombiniert werden – z.B. Telegram für Benachrichtigungen + Home Assistant für Smart Home.

---

## Telegram Bot Setup

Mit dem Telegram-Bot kannst du mit AuraGo chatten und Benachrichtigungen empfangen.

### Schritt 1: Bot bei BotFather erstellen

1. Öffne Telegram und suche nach **@BotFather**
2. Starte den Bot mit `/start`
3. Erstelle einen neuen Bot: `/newbot`
4. Gib einen Namen ein (z.B. "Mein AuraGo")
5. Gib einen Benutzernamen ein (muss mit "bot" enden, z.B. `mein_aurago_bot`)
6. **Speichere den Token** (sieht aus wie: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

### Schritt 2: Deine User-ID ermitteln

1. Suche nach **@userinfobot**
2. Starte den Bot
3. Erhalte deine ID (z.B. `12345678`)

### Schritt 3: AuraGo konfigurieren

```yaml
telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    telegram_user_id: 12345678
    max_concurrent_workers: 5
```

### Schritt 4: Testen

1. Starte AuraGo neu
2. Sende eine Nachricht an deinen Bot
3. Der Bot sollte antworten

> ⚠️ **Sicherheit:** Aus Sicherheitsgründen antwortet der Bot nur auf den konfigurierten `telegram_user_id`.

---

## Discord Bot Setup

Für Community-Server oder Team-Kollaboration.

### Schritt 1: Discord-Anwendung erstellen

1. Besuche [Discord Developer Portal](https://discord.com/developers/applications)
2. Klicke "New Application"
3. Gib einen Namen ein (z.B. "AuraGo")
4. Gehe zu "Bot" → "Add Bot"

### Schritt 2: Token und Berechtigungen

1. Kopiere den **Token** (unter "Bot" → "Token")
2. Aktiviere folgende Intents:
   - Message Content Intent
   - Server Members Intent

### Schritt 3: Bot zum Server einladen

1. Gehe zu "OAuth2" → "URL Generator"
2. Scopes: `bot`, `applications.commands`
3. Permissions: `Send Messages`, `Read Messages`, `Embed Links`
4. Kopiere die URL und öffne sie im Browser
5. Wähle deinen Server aus

### Schritt 4: Server- und Channel-ID ermitteln

- Aktiviere Discord Developer Mode (Einstellungen → Erweitert)
- Rechtsklick auf Server → "Server-ID kopieren"
- Rechtsklick auf Channel → "Channel-ID kopieren"

### Schritt 5: AuraGo konfigurieren

```yaml
discord:
    enabled: true
    bot_token: "DEIN-DISCORD-TOKEN"
    guild_id: "123456789012345678"
    default_channel_id: "123456789012345678"
    allowed_user_id: "123456789012345678"  # Optional: nur dieser User
    readonly: false
```

| Option | Beschreibung |
|--------|--------------|
| `readonly` | `true` = Nur lesen, keine Antworten senden |
| `allowed_user_id` | Beschränkt auf spezifischen Discord-User |

---

## E-Mail (IMAP/SMTP) Konfiguration

AuraGo kann E-Mails senden und empfangen.

### Einzelnes E-Mail-Konto

```yaml
email_accounts:
    - id: "private"
      name: "Privat"
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

### Passwort im Vault speichern

Das Passwort wird **nicht** in der `config.yaml` gespeichert, sondern im verschlüsselten Vault:

```bash
# Über die Web-UI oder Chat
/store_secret email_private_password "dein-app-passwort"
```

> 💡 **Gmail-Tipp:** Verwende ein [App-Passwort](https://myaccount.google.com/apppasswords), nicht dein normales Passwort.

### Provider-Einstellungen

| Provider | IMAP-Host | SMTP-Host |
|----------|-----------|-----------|
| Gmail | `imap.gmail.com` | `smtp.gmail.com` |
| Outlook | `outlook.office365.com` | `smtp.office365.com` |
| GMX | `imap.gmx.net` | `mail.gmx.net` |
| Web.de | `imap.web.de` | `smtp.web.de` |

### Automatisches E-Mail-Monitoring

Mit `watch_enabled: true` überwacht AuraGo den Posteingang:
- Prüft alle `watch_interval_seconds` auf neue E-Mails
- Benachrichtigt den Agenten bei neuen Nachrichten
- Ermöglicht automatische Antworten

---

## Home Assistant Integration

Steuere Smart-Home-Geräte über AuraGo.

### Einrichtung

```yaml
home_assistant:
    enabled: true
    url: "http://homeassistant.local:8123"
```

### Access Token erstellen

1. Öffne Home Assistant
2. Gehe zu deinem Profil (unten links auf deinen Namen klicken)
3. Scrollen zu "Long-Lived Access Tokens"
4. Klicke "Create Token"
5. Kopiere den Token und speichere ihn:

```bash
/store_secret home_assistant "dein-langlebiger-token"
```

### Verwendung im Chat

```
Schalte das Licht im Wohnzimmer an.
Wie ist die Temperatur im Schlafzimmer?
Starte die Staubsauger-Routine.
```

> 🔍 **Deep Dive:** AuraGo erkennt verfügbare Entities automatisch und kann alle Services aufrufen, die Home Assistant bereitstellt.

| Option | Beschreibung |
|--------|--------------|
| `readonly` | `true` = Nur Status abfragen, keine Aktionen |

---

## Docker Integration

Verwalte Docker-Container über AuraGo.

### Konfiguration

```yaml
docker:
    enabled: true
    host: "unix:///var/run/docker.sock"
    readonly: false
```

### Host-Optionen

| Host-URL | Beschreibung |
|----------|--------------|
| `unix:///var/run/docker.sock` | Lokaler Docker (Linux/macOS) |
| `tcp://localhost:2375` | Remote Docker (unsicher) |
| `tcp://localhost:2376` | Remote Docker mit TLS |

> ⚠️ **Sicherheit:** Der Docker-Zugriff ermöglicht volle Kontrolle über den Host. Aktiviere `readonly` für mehr Sicherheit.

### Docker Socket mounten (Docker-Compose)

```yaml
services:
  aurago:
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

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

### Webhook erstellen

Über die Web-UI oder API:

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

### Delivery Modes

| Modus | Beschreibung |
|-------|--------------|
| `message` | Payload wird als Nachricht an den Agenten gesendet |
| `notify` | Nur Benachrichtigung im UI, kein Agent |
| `silent` | Nur Logging, keine Aktion |

### Beispiel: GitHub Integration

```yaml
# Webhook-URL in GitHub eintragen:
# http://localhost:8088/api/webhooks/github-push?token=DEIN-TOKEN
```

---

## Budget Tracking Setup

Überwache die Kosten für LLM-API-Aufrufe.

### Aktivierung

```yaml
budget:
    enabled: true
    daily_limit_usd: 1.0
    enforcement: "warn"           # "warn", "partial", "full"
    warning_threshold: 0.8
    reset_hour: 0
    default_cost:
        input_per_million: 1.0
        output_per_million: 3.0
```

### Enforcement-Modi

| Modus | Beschreibung |
|-------|--------------|
| `warn` | Warnung bei Überschreitung, aber weiterarbeiten |
| `partial` | Nur günstige Modelle erlauben |
| `full` | Alle Anfragen blockieren bis Reset |

### Modell-spezifische Kosten

```yaml
budget:
    models:
        - name: "gpt-4"
          input_per_million: 30.0
          output_per_million: 60.0
        - name: "gpt-3.5-turbo"
          input_per_million: 0.5
          output_per_million: 1.5
        - name: "arcee-ai/trinity-large-preview:free"
          input_per_million: 0
          output_per_million: 0
```

> 💡 **Tipp:** Setze kostenlose Modelle auf `0`, damit sie nicht zum Budget zählen.

---

## Google Workspace Setup

Zugriff auf Gmail, Kalender und Drive.

### Aktivierung

```yaml
agent:
    enable_google_workspace: true
```

### OAuth2 Einrichtung

1. Gehe zu [Google Cloud Console](https://console.cloud.google.com/)
2. Erstelle ein Projekt
3. Aktiviere die APIs:
   - Gmail API
   - Google Calendar API
   - Google Drive API
4. Erstelle OAuth2-Anmeldedaten (Desktop-Anwendung)
5. Client-ID und Secret in AuraGo eintragen

```yaml
providers:
    - id: "google"
      name: "Google"
      type: "google"
      auth_type: "oauth2"
      oauth_auth_url: "https://accounts.google.com/o/oauth2/auth"
      oauth_token_url: "https://oauth2.googleapis.com/token"
      oauth_client_id: "DEINE-CLIENT-ID"
      oauth_client_secret: "DEIN-CLIENT-SECRET"
      oauth_scopes: "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/calendar https://www.googleapis.com/auth/drive"
```

### Autorisierung

1. Starte AuraGo
2. Rufe die Auth-URL auf: `http://localhost:8088/api/auth/google`
3. Melde dich bei Google an
4. Das Token wird automatisch gespeichert

---

## WebDAV/Koofr Setup

Dateizugriff auf WebDAV-kompatible Cloud-Speicher.

### WebDAV (Nextcloud, ownCloud, etc.)

```yaml
webdav:
    enabled: true
    url: "https://cloud.example.com/remote.php/dav/files/username/"
    username: "dein-username"
    readonly: false
```

Passwort speichern:
```bash
/store_secret webdav "dein-app-passwort"
```

### Koofr

```yaml
koofr:
    enabled: true
    username: "dein@email.com"
    base_url: "https://app.koofr.net"
    readonly: false
```

Passwort speichern:
```bash
/store_secret koofr "dein-app-password"
```

> 💡 **Koofr-Tipp:** Erstelle ein dediziertes App-Passwort unter Einstellungen → Passwörter.

---

## Integrationen testen

### Test über Chat

Die meisten Integrationen können direkt im Chat getestet werden:

```
Zeige meine Telegram-Config.
Sende eine Test-E-Mail an mich.
Liste alle Docker-Container.
Wie ist der Status meiner Home Assistant-Geräte?
```

### Test über Dashboard

Das Dashboard zeigt den Status aller Integrationen:

1. Öffne die Web-UI
2. Klicke auf "Dashboard"
3. Scrolle zu "Integrationen"
4. Grüner Punkt = Verbindung OK

### Fehlerbehebung

| Problem | Lösung |
|---------|--------|
| "Connection refused" | URL und Port prüfen |
| "Unauthorized" | API-Key/Token prüfen |
| "Timeout" | Firewall/Netzwerk prüfen |
| Integration erscheint nicht | `enabled: true` in config.yaml |

### Debug-Logging aktivieren

```yaml
agent:
    debug_mode: true
```

Logs prüfen:
```bash
tail -f log/supervisor.log | grep -i telegram
```

---

## Nächste Schritte

- **[Gedächtnis & Wissen](09-memory.md)** – Wie AuraGo Informationen speichert
- **[Persönlichkeit](10-personality.md)** – Den Agenten anpassen
- **[Mission Control](11-missions.md)** – Automatisierung einrichten

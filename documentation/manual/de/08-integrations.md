# Kapitel 8: Integrationen

AuraGo lässt sich nahtlos in verschiedene Dienste und Plattformen integrieren.

> 💡 **Web-UI zuerst:** Jede Integration in diesem Kapitel wird über **Menü → Config** konfiguriert. Nutze die Sidebar-Suche oder Gruppen wie **Messenger**, **Smart Home**, **Netzwerk & Remote**, **Externe KI** und **Gefahrenzone**. YAML-Blöcke sind Alternativen für Headless- oder Skript-Setups.

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

### Konfigurations-Referenz (Kapitel 7)

Dieses Kapitel beschreibt Einrichtung, Anwendungsfälle und Web-UI-Workflows. Für YAML-Schlüssel, Defaults und plattformweite Blöcke siehe **[Kapitel 7: Konfiguration](07-konfiguration.md)**:

| Thema | Abschnitt in Kapitel 7 |
|-------|------------------------|
| Provider-System, LLM, Embeddings, Agent-Verhalten | [Das Provider-System](07-konfiguration.md#das-provider-system), [Agent-Verhalten](07-konfiguration.md#agent-verhalten) |
| Tool-Berechtigungen, Skill Manager, Media Registry, Daemon Skills | [Tool-Konfiguration](07-konfiguration.md#tool-konfiguration), [Skill Manager](07-konfiguration.md#skill-manager) |
| Co-Agents, Personality, Logging, Umgebungsvariablen | [Co-Agents](07-konfiguration.md#co-agents--parallele-sub-agenten), [Personality](07-konfiguration.md#personality--persönlichkeit), [Umgebungsvariablen](07-konfiguration.md#umgebungsvariablen) |
| Erweiterte Integrations-YAML-Blöcke | [Weitere Konfigurationsblöcke](07-konfiguration.md#weitere-konfigurationsblöcke-übersicht), [Erweiterte Konfigurationsblöcke](07-konfiguration.md#erweiterte-konfigurationsblöcke) |
| Vollständige Parameterliste | `config_template.yaml` im Projektverzeichnis |

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

### YAML-Referenz (Mehrere Konten)
Die moderne Konfiguration verwendet eine **Liste** von E-Mail-Konten unter `email_accounts`. Beim Start wird eine eventuell vorhandene `email:`-Sektion automatisch in einen `email_accounts`-Eintrag mit `id: "default"` migriert (siehe `internal/config/config_migrate.go:MigrateEmailAccounts`).

```yaml
email_accounts:
  - id: "personal"
    name: "Privat"
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
    readonly: false            # true = nur lesen, kein Versand
    disabled: false
  - id: "work"
    name: "Arbeit"
    imap_host: "imap.office365.com"
    smtp_host: "smtp.office365.com"
    # ...
```

### Legacy: Einzel-Konto (`email:`)
```yaml
email:                         # deprecated – wird beim Start zu email_accounts migriert
  enabled: true
  imap_host: "imap.gmail.com"
  smtp_host: "smtp.gmail.com"
  username: "dein.email@gmail.com"
```

## AgentMail Integration

API-basierte E-Mail-Postfächer über [AgentMail](https://agentmail.to). Getrennt von der Legacy-IMAP/SMTP-`email`-Integration — bestehende `fetch_email`- und `send_email`-Tools behalten ihr bisheriges Verhalten.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → AgentMail**.
2. Aktiviere die Integration.
3. Trage die **Inbox-ID** ein oder aktiviere **Auto Create Inbox**.
4. Optional: Aktiviere **Relay to Agent**, um AuraGo bei neuen Nachrichten zu wecken.
5. Speichere den API-Key im Vault (`agentmail_api_key`).
6. Speichern und AuraGo neu starten.

### YAML-Referenz
```yaml
agentmail:
  enabled: true
  readonly: false
  inbox_id: ""
  auto_create_inbox: false
  username: ""
  domain: ""
  display_name: ""
  use_websocket: true
  poll_interval_seconds: 120
  relay_to_agent: false
  relay_cheatsheet_id: ""
  max_attachment_mb: 10
  base_url: https://api.agentmail.to
  websocket_url: wss://ws.agentmail.to/v0
```

> 🔒 Der API-Key wird im Vault als `agentmail_api_key` gespeichert, nicht in der `config.yaml`.

Nutze die fokussierten Tools `agentmail_inboxes`, `agentmail_messages`, `agentmail_threads` und `agentmail_drafts` im Chat. Der alte Toolname `agentmail` bleibt als Kompatibilitätsalias akzeptiert.

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

`allowed_services` ist eine optionale Allowlist für `call_service`; leer erlaubt alle Services, solange sie nicht in `blocked_services` stehen.

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

## Package Manager Integration

Strukturierte OS-Paketverwaltung über das Tool `package_manager`. Unterstützt apt, dnf, yum, pacman, zypper, apk, brew, winget, choco und scoop.

> ⚠️ **Sicherheit:** Paketverwaltung ermöglicht erheblichen Systemzugriff. Nur aktivieren, wenn nötig, und `readonly` für reine Überwachung bevorzugen.

### Voraussetzungen

Beide Schalter müssen aktiv sein:

1. `package_manager.enabled: true`
2. `agent.allow_package_manager: true`

### Einrichtung in der Web-UI
1. Öffne **Config → Gefahrenzone** und aktiviere **Paketverwaltung (package_manager)**.
2. Setze `package_manager.enabled` und Detail-Optionen (`readonly`, `allow_install`, …) in der `config.yaml` (kein eigener Config-Menüpunkt für den `package_manager`-Block).
3. Speichern und AuraGo neu starten.

### YAML-Referenz
```yaml
package_manager:
  enabled: true
  readonly: false
  auto_detect: true
  override: ""
  allow_install: true
  allow_remove: true
  allow_upgrade: true

agent:
  allow_package_manager: true
```

`override` erzwingt einen bestimmten Paketmanager (z. B. `apt`, `brew`, `winget`); leer lassen für Auto-Erkennung über PATH.

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
  readonly: false
  auth_type: basic
  url: "https://cloud.example.com/remote.php/dav/files/username/"
  username: "user@example.com"

koofr:
  enabled: true
  readonly: false
  username: "user@example.com"
```

> Passwörter und Tokens werden im Vault gespeichert, nicht als `password` oder `token` in der `config.yaml`.

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

GitHub-Repositories, Issues, Pull Requests, Branches, Dateien, Commits und Workflow-Runs über das native Tool `github` verwalten. Eingehende GitHub-Webhooks werden separat konfiguriert.

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → GitHub**.
2. Aktiviere die Integration und trage den Standard-**owner** (Benutzername oder Organisation) ein.
3. Für GitHub Enterprise: **base_url** setzen (z. B. `https://github.example.com/api/v3`).
4. Personal Access Token im Vault speichern (`github_token`).
5. Optional **read-only** aktivieren, um Schreiboperationen zu blockieren.
6. Erlaubte Repositories auswählen. Neue Auswahlen werden als `owner/repo` gespeichert; alte reine Repo-Namen gelten nur für den konfigurierten Owner.
7. Für eingehende Webhooks: **Config → Integrationen → Webhooks**.
8. Speichern und neu starten.

### YAML-Referenz
```yaml
github:
  enabled: false
  readonly: false
  owner: ""
  default_private: false
  base_url: ""                  # GitHub Enterprise API-Basis-URL (optional)
  allowed_repos: []             # bevorzugt owner/repo; leer = nur von AuraGo erstellte Repos
```

`allowed_repos` ist eine strikte Freigabeliste. Eine leere Liste bedeutet nicht "alle Repos"; sie erlaubt nur Repositories, die AuraGo über `create_repo` erstellt und mit `agent_created=true` getrackt hat. Manuelle `track_project`-Einträge sind nur lokale Inventur und geben keinen Remote-Zugriff frei.

### Agent-Tool: `github`

| Operation | Beschreibung |
|-----------|--------------|
| `list_repos`, `search_repos` | Repositories auflisten oder suchen |
| `create_repo`, `delete_repo`, `get_repo` | Repository-Lifecycle |
| `list_issues`, `create_issue`, `close_issue` | Issue-Verwaltung |
| `list_pull_requests`, `list_branches` | PR- und Branch-Listen |
| `get_file`, `create_or_update_file`, `list_commits` | Datei- und Commit-Zugriff |
| `list_workflow_runs` | CI/CD-Workflow-Runs |
| `list_projects`, `track_project`, `untrack_project` | Lokales Projekt-Tracking |

> 💡 **Vault:** Token als `github_token` speichern. API-Tokens nie in `config.yaml` ablegen.

### Webhooks (separat)

### Einrichtung in der Web-UI
1. Öffne **Config → Integrationen → Webhooks**.
2. Aktiviere Webhooks und konfiguriere Limits.
3. Speichern.

### YAML-Referenz
```yaml
webhooks:
  enabled: true
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

Remote-Desktop und Geräteverwaltung über MeshCentral.

AuraGo unterstützt die Kernfunktionen für Homelab-Automation: Serverinfos, Gerätegruppen, Gerätelisten, Gerätedetails, Ereignisse, Wake-on-LAN, unterstützte Power-Aktionen (`off`, `reset`, `sleep`, `amt_on`, `amt_off`, `amt_reset`) und MeshAgent-`run_command`.

Die Integration bildet bewusst noch nicht die gesamte MeshCtrl-Oberfläche ab. Benutzer-/Gruppenverwaltung, Datei-Upload/-Download, interaktive Shell, Desktop-Relay, WebRelay, Invite-Links, Device Sharing, Reports und weitere Relay-lastige MeshCentral-Funktionen benötigen zusätzliche Command- oder `meshrelay.ashx`-Unterstützung.

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
      transport: stdio
      command: "uvx"
      args: ["mcp-server-fetch"]
      allowed_tools: []  # optionale Allowlist; leer bedeutet alle entdeckten nicht-destruktiven Tools
      allow_destructive: false

    - name: "remote-tools"
      transport: streamable_http # stdio | streamable_http | sse | websocket
      url: "https://example.com/mcp"
      headers:
        Authorization: "Bearer {{remote-mcp-token}}"
      allowed_tools: []
      allow_destructive: false
```

Wenn `transport` fehlt, bleibt AuraGo beim bisherigen Verhalten und startet den Server als lokalen stdio-Prozess. Netzwerk-Transports brauchen eine URL; Header-Werte koennen MCP-Vault-Secrets mit `{{alias}}` referenzieren. In der Web-UI prueft **Verbindung testen** Initialize und Tool-Discovery vor dem Speichern.

Beim MCP-Client ist `allowed_tools` pro Server optional. Leer lassen oder weglassen erlaubt alle entdeckten nicht-destruktiven Tools; trage Toolnamen nur ein, wenn Ausführung und Routing auf diese Teilmenge begrenzt werden sollen.

### MCP-Server
Stellt AuraGo-Tools für externe Clients bereit.

**Web-UI:** **Config → Externe KI → MCP-Server** — aktivieren, Auth und `allowed_tools` setzen. Zusätzlich **Config → Gefahrenzone** → **MCP** für Client-Zugriff (`agent.allow_mcp`).

### YAML-Referenz
```yaml
mcp_server:
  enabled: true
  require_auth: true
  allowed_tools:
    - "execute_shell"
    - "filesystem"
  vscode_debug_bridge: false
```

> Der MCP-Server teilt sich den Haupt-HTTP-Server — es gibt kein separates `port`-Feld. MCP-Client-Zugriff erfordert zusätzlich **Config → Gefahrenzone** → MCP (`agent.allow_mcp: true`).

`allowed_tools` ist eine explizite serverseitige Allowlist. Leer veröffentlicht keine AuraGo-Tools; `vscode_debug_bridge` nutzt ein eigenes begrenztes Debugging-Preset.

## Composio Integration

Verbinde AuraGo mit [Composio](https://composio.dev)-Toolkits (GitHub, Slack, Gmail und Hunderte weitere) über das native Tool `composio_call`.

### Einrichtung in der Web-UI
1. Registriere dich bei [Composio](https://composio.dev) und erstelle einen API-Key.
2. Öffne **Config → Integrationen → Composio**.
3. Aktiviere die Integration.
4. Setze die **User ID** und konfiguriere Toolkit-Richtlinien.
5. Speichere den API-Key im Vault (`composio_api_key`).
6. Verbinde Konten im Composio-Dashboard für die konfigurierten Toolkits.
7. Speichern und AuraGo neu starten.

### YAML-Referenz
```yaml
composio:
  enabled: true
  base_url: https://backend.composio.dev/api/v3.1
  user_id: aurago-default
  readonly: true
  allow_destructive: false
  allow_natural_language_input: false
  request_timeout_seconds: 60
  cache_ttl_seconds: 300
  max_result_bytes: 262144
  toolkits: []
  # - slug: github
  #   enabled: true
  #   readonly: true
  #   allow_destructive: false
  #   allowed_tool_slugs: []
  #   blocked_tool_slugs: []
```

> 🔒 Der API-Key wird im Vault als `composio_api_key` gespeichert, nicht in der `config.yaml`.

Standardmäßig blockiert `readonly: true` mutierende Aktionen. Setze `allow_destructive: true` nur, wenn Delete/Remove/Revoke-Operationen explizit benötigt werden. Pro Toolkit steuern `allowed_tool_slugs` und `blocked_tool_slugs` die Feingranularität.

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
      readonly: true
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

**Web-UI:** `media_registry.enabled` primär per YAML (kein eigener Config-Menüpunkt). Verwaltung über **Galerie** (`/gallery`).

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
  allow_deploy: false
  allow_site_management: false
  allow_env_management: false
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

### AgoDesk / AgoChat Desktop-Begleiter

AuraGo kann sich mit dem **AgoDesk**-Desktop-Client per WebSocket koppeln. Bei verbundenem Gerät kann der Agent proaktive Nachrichten über `send_agodesk_chat` senden und Remote-Desktop-Befehle über das AgoDesk-Protokoll ausführen.

**Voraussetzungen:**
- `remote_control.enabled: true`
- AgoDesk-Client verbunden und gekoppelt (`/api/agodesk/ws`)

**Einrichtung:**
1. AgoDesk-Desktop-Client installieren und öffnen.
2. Mit AuraGo koppeln (`pairing_token` oder gespeichertes `device_id` + `shared_key_proof`).
3. Das Gerät erscheint im Agent-Kontext als **REACHABLE CHAT CHANNEL** mit `device_id`.

**Agent-Tool:** `send_agodesk_chat`

```json
{
  "action": "send_agodesk_chat",
  "device_id": "dev-abc123",
  "message": "Dein Backup wurde erfolgreich abgeschlossen."
}
```

> 📖 Vollständiges Protokoll: [`documentation/agodesk_backend_protocol.md`](../../agodesk_backend_protocol.md)

**API:** `GET /api/remote/devices` listet verbundene RemoteHub/AgoDesk-Geräte.

## Sandbox

Isolierte Ausführung von Python-Code und externen Befehlen.

**Web-UI:** Config → Integrationen → Sandbox → Backend, Timeout und Netzwerkzugriff konfigurieren.

### YAML-Referenz
```yaml
sandbox:
  enabled: true
  backend: docker
```

## Python Tool Bridge

Erlaubt **Python-Skills**, ausgewählte native AuraGo-Tools über eine interne HTTP-Bridge (`POST /api/internal/tool-bridge/`) aufzurufen. Standard: deaktiviert.

**Web-UI:** **Config → Tools → Fähigkeiten-Manager** — Python Tool Bridge aktivieren und `allowed_tools`-Whitelist pflegen.

### YAML-Referenz
```yaml
tools:
  python_tool_bridge:
    enabled: false
    allowed_tools: []              # explizite Whitelist, z. B. ["api_request", "sql_query"]
    allowed_sql_connections: []    # SQL-Verbindungsnamen; leer = SQL-Bridge blockiert
```

Skills deklarieren Bridge-Nutzung im Manifest über `internal_tools`.

> ⚠️ **Sicherheit:** Nur wirklich benötigte Tools whitelisten. Niemals `get_secret` oder `execute_shell` ohne volles Vertrauen in den Skill-Code.

## Skill Manager

Verwalte hochgeladene Python-Skills.

**Web-UI:** **Config → Tools → Fähigkeiten-Manager** → Uploads erlauben und Guardian-Scan aktivieren.

### YAML-Referenz
```yaml
tools:
  skill_manager:
    enabled: true
    allow_uploads: true
```

## Daemon Skills

Langlaufende Python-Skills, die im Hintergrund aktiv bleiben und den Agent bei Ereignissen wecken können (z. B. Datei-Watcher, Polling-Schleifen). Verwaltung über das Tool `manage_daemon` und die Dashboard-Karte **Daemon Skills**.

### Einrichtung in der Web-UI
1. Öffne **Config → Tools → Daemon Skills**.
2. Daemon Skills aktivieren (Opt-in, standardmäßig deaktiviert).
3. Parallelität und Wake-up-Kostenlimits konfigurieren.
4. Skill mit `daemon: true` im Manifest hochladen oder aktivieren.
5. Speichern und neu starten.

### YAML-Referenz
```yaml
tools:
  daemon_skills:
    enabled: false
    max_concurrent_daemons: 5
    global_rate_limit_secs: 60
    max_wakeups_per_hour: 6
    max_budget_per_hour: 0.50
```

### Agent-Tool: `manage_daemon`

| Operation | Beschreibung |
|-----------|--------------|
| `list` | Alle laufenden Daemons auflisten |
| `status` | Status eines Daemons abfragen (`skill_id` erforderlich) |
| `start` / `stop` | Daemon per Skill-ID starten oder stoppen |
| `reenable` | Auto-deaktivierten Daemon wieder aktivieren |
| `refresh` | Skills von der Festplatte neu scannen und Daemons abgleichen |

> ⚠️ **Kostenkontrolle:** `max_wakeups_per_hour` und `max_budget_per_hour` wirken als Circuit Breaker gegen unkontrollierte LLM-Kosten durch häufige Daemon-Wake-ups.

## Jellyfin Integration

Media-Server-Verwaltung.

**Web-UI:** Config → Integrationen → Jellyfin → Host und Port eintragen. API-Key im Vault speichern.

### YAML-Referenz
```yaml
jellyfin:
  enabled: true
  readonly: false
  allow_destructive: false
  host: "jellyfin.local"
  port: 8096
  use_https: true
  insecure_ssl: false
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
1. Auf dem **Master:** **Invasion Control** (`/invasion`) — Nests/Eggs verwalten und hatch deployen (setzt `egg_mode` auf dem Worker automatisch).
2. Auf einer **standalone Worker-Instanz:** `egg_mode` nur per YAML oder Umgebungsvariablen (`AURAGO_EGG_MODE`, `AURAGO_MASTER_URL`, …) — kein eigener Config-Menüpunkt.

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

**Web-UI:** Config → Integrationen → Obsidian → Local-REST-API Host/Port und `obsidian_api_key` im Vault.

### YAML-Referenz
```yaml
obsidian:
  enabled: true
  host: "127.0.0.1"
  port: 27124
  use_https: true
  readonly: false
```

## Uptime Kuma Integration

Überwache die Erreichbarkeit von Diensten mit Uptime Kuma.

**Web-UI:** Config → Integrationen → Uptime Kuma → Base-URL eingeben. API-Key im Vault speichern.

### YAML-Referenz
```yaml
uptime_kuma:
  enabled: true
  base_url: "https://uptime-kuma.example.com:3001"
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

**Web-UI:** Config → Integrationen → Browser Automation → aktivieren, Headless-Modus und Screenshot-Verzeichnis konfigurieren.

Der Browser-Automation-Sidecar verlangt standardmäßig `AURAGO_BROWSER_AUTOMATION_TOKEN`. AuraGo setzt es bei verwalteten Sidecars automatisch; bei manuell gestarteten Sidecars muss es explizit gesetzt werden. Nutze `AURAGO_BROWSER_AUTOMATION_ALLOW_UNAUTH=1` nur für isolierte lokale Entwicklung.

### YAML-Referenz
```yaml
browser_automation:
  enabled: true
  headless: true
  screenshots_dir: "browser_screenshots"
```

## Output Compression

Reduziert den Token-Verbrauch durch Filterung und Deduplizierung von Tool-Ausgaben, bevor sie in den LLM-Kontext gelangen.

**Web-UI:** Config → Agent → Output Compression → aktivieren, Schwellenwerte anpassen und Shell-/Python-/API- sowie Advanced-Filter setzen.

### Erweiterte Modi

| Modus | Standard | Einsatz |
|-------|----------|---------|
| `repetitive_substitution` | deaktiviert | Ersetzt lange, wiederholte Phrasen in log-artigen Ausgaben durch ein kleines Wörterbuch. Fehler, Diffs, Code-/Source-Reads, JSON-Dokumente und exakte Kopierausgaben werden übersprungen. |
| `toon_json` | deaktiviert | Wandelt bekannte homogene API-Arrays in eine kompakte TOON-artige Darstellung um, wenn genug Tokens gespart werden. `api_request` und Datei-Reads werden übersprungen. |

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
    repetitive_substitution:
      enabled: false
      lzw_enabled: true
      ltsc_lite_enabled: false
      min_phrase_chars: 15
      min_occurrences: 3
      min_savings_percent: 15
      max_input_chars: 50000
      max_dictionary_entries: 16
    toon_json:
      enabled: false
      min_savings_percent: 10
      max_rows: 200
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

> ⚠️ **Hinweis:** Die Heartbeat-Konfiguration erfolgt derzeit vollständig über die **Web-UI** (`Config → Integrationen → Heartbeat`). Ein dedizierter `heartbeat:`-Block in `config.yaml` wird aktuell nicht ausgewertet – die oben gezeigten Werte (Tag-/Nacht-Fenster, Intervalle, `additional_prompt` etc.) werden intern im Heartbeat-Service persistiert.

Die Standardeinstellungen sind:

- **Tag-Fenster:** 08:00 – 22:00, Intervall 1h
- **Nacht-Fenster:** 22:00 – 08:00, Intervall 4h
- **Geprüfte Quellen:** Aufgaben, Termine, E-Mails (in der UI aktivierbar)

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

## 3D-Drucker-Integration

Überwache und steuere 3D-Drucker mit dem `three_d_printer`-Tool. Unterstützt **Elegoo Centauri Carbon** (SDCP WebSocket) und **Klipper/Moonraker** (HTTP API).

**Web-UI:** Config → Integrationen → 3D Printers → Drucker hinzufügen, `readonly: true` für Nur-Monitor.

### YAML-Referenz
```yaml
three_d_printers:
  enabled: false
  readonly: true
  default_printer: ""
  elegoo_centauri_carbon:
    enabled: false
    printers:
      - id: "lab-printer"
        name: "Elegoo Centauri Carbon"
        url: "ws://192.168.1.50/websocket"
        timeout_seconds: 10
  klipper:
    enabled: false
    printers:
      - id: "voron"
        name: "Voron 2.4"
        url: "http://192.168.1.60:7125"
        api_key: ""
        timeout_seconds: 10
```

**API:** `GET /api/3d-printers/test`, Kamera-Snapshot/Stream pro `printer_id`.

**Agent-Tool:** `three_d_printer` — u. a. `status`, `camera_snapshot`, `show_live_stream`, `start_print`, `pause_print` (Schreibzugriff erfordert `readonly: false`).

---

## Frigate Integration

Videoüberwachung und NVR-Management über Frigate.

**Web-UI:** Config → Integrationen → Frigate → URL und API-Token eingeben. Optional: Event-Relay, Review-Relay und Media-Speicherung aktivieren.

### YAML-Referenz
```yaml
frigate:
  enabled: true
  readonly: false
  url: "https://frigate.local:8971"
  internal_port: false
  insecure: false
  default_camera: ""
  event_relay: false
  review_relay: false
  store_media: false
  mqtt_topic_prefix: "frigate"
```

## Grafana Integration

Monitoring und Observability über Grafana.

**Web-UI:** Config → Integrationen → Grafana → Base-URL eingeben. API-Key im Vault speichern.

### YAML-Referenz
```yaml
grafana:
  enabled: true
  base_url: "http://grafana.local:3000"
  readonly: false
  insecure_ssl: false
  request_timeout: 30
```

## Manifest-Integration

[Manifest](https://manifest.build) ist ein OpenAI-kompatibles LLM-Gateway mit verwaltetem Dashboard. AuraGo kann Manifest als **managed Docker-Sidecar** (Manifest + Postgres) oder als externe Instanz betreiben.

**Web-UI:** Config → Integrationen → Manifest → Modus wählen, Secrets im Vault speichern, Test: `GET /api/manifest/test`.

### Provider-Routing
```yaml
providers:
  - id: manifest-main
    type: manifest
    name: Manifest Gateway
    base_url: http://127.0.0.1:2099
    api_key: ""                    # Vault: manifest_api_key
    model: gpt-4o
```

### YAML-Referenz
```yaml
manifest:
  enabled: false
  auto_start: true
  mode: managed
  url: "http://127.0.0.1:2099"
  external_base_url: "https://app.manifest.build/v1"
  host: "127.0.0.1"
  port: 2099
  host_port: 2099
  image: manifestdotbuild/manifest:5
  container_name: aurago_manifest
  postgres_container_name: aurago_manifest_postgres
  postgres_image: postgres:15-alpine
```

Vault-Keys: `manifest_api_key`, `manifest_postgres_password`, `manifest_better_auth_secret`. Tailscale: `tailscale.tsnet.expose_manifest`.

---

## Dograh-Integration

[Dograh](https://dograh.com) ist eine Voice/Telephony-AI-Plattform. AuraGo deployt einen **managed Multi-Container-Stack** und verbindet ihn per MCP — es gibt kein natives `dograh`-Agent-Tool.

**Web-UI:** Config → Integrationen → Dograh → aktivieren, Vault-Keys setzen, Test: `GET /api/dograh/test`.

### YAML-Referenz
```yaml
dograh:
  enabled: false
  auto_start: true
  mode: managed
  readonly: true
  allow_test_calls: false
  api_url: "http://127.0.0.1:8000"
  ui_url: "http://127.0.0.1:3010"
  telemetry_enabled: false
  turn_enabled: false
```

Vault-Keys: `dograh_api_key`, `dograh_super_api_key`, `dograh_encryption_key`, `dograh_postgres_password`, `dograh_minio_secret_key`. MCP-Bridge über `mcp_call` / `/mcp`.

---

## Space Agent Integration

Verwalteter Docker-Sidecar für den Space Agent – eine eigenständige AuraGo-Instanz für isolierte Aufgaben.

**Web-UI:** Config → Integrationen → Space Agent → Repository-URL, Host, Port und HTTPS konfigurieren.

### YAML-Referenz
```yaml
space_agent:
  enabled: true
  auto_start: true
  repo_url: ""
  git_ref: ""
  container_name: ""
  image: ""
  host: ""
  port: 0
  https_enabled: false
  https_port: 0
  customware_path: ""
  data_path: ""
  admin_user: ""
  public_url: ""
```

## Virtual Desktop Integration

Workspace-basierter Browser-Desktop für lokale Apps, generierte Apps, Dateiarbeit, Code Studio und verwaltete Docker-Software.

**Web-UI:** Config → Integrationen → Virtual Desktop → Workspace aktivieren, Agent-Control konfigurieren und Code-Studio-Limits anpassen.

### Zugehörige Tool-Toggles

Die fokussierten Tools `virtual_desktop_files`, `virtual_desktop_apps` und `virtual_desktop_widgets` werden nur freigeschaltet, wenn sowohl `tools.virtual_desktop.enabled` als auch `virtual_desktop.allow_agent_control` aktiv sind. Office-Dokument- und Workbook-Tools benötigen ebenfalls den aktivierten Virtual Desktop.

### Desktop Software Store

Der Software Store installiert von AuraGo verwaltete Docker-Apps in die Virtual-Desktop-Umgebung. Aktuelle Katalogeinträge sind unter anderem Node-RED, Dozzle, code-server, Beszel, RomM, OliveTin, Manifest und Termix. Termix startet mit einem `guacd`-Companion-Container, damit seine Web-UI SSH-, RDP-, VNC- und Telnet-Sitzungen verwalten kann.

### YAML-Referenz
```yaml
tools:
  virtual_desktop:
    enabled: false
  office_document:
    enabled: false
    readonly: false
  office_workbook:
    enabled: false
    readonly: false

virtual_desktop:
  enabled: false
  readonly: false
  allow_agent_control: false
  allow_generated_apps: true
  allow_python_jobs: false
  workspace_dir: agent_workspace/virtual_desktop
  max_file_size_mb: 50
  control_level: confirm_destructive
  max_ws_clients: 8
  code_studio:
    enabled: true
    image: ghcr.io/antibyte/aurago-code-studio:latest
    auto_start: false
    auto_stop_minutes: 30
    max_memory_mb: 4096
    max_cpu_cores: 2
```

## Shell Sandbox Integration

Linux-Landlock-basierte Sandbox für Shell-Befehle. Einschränkung von Dateisystemzugriffen, CPU-Zeit und Speicher für Shell-Operationen.

**Web-UI:** **Config → Tools → Sandkasten** — Shell-Sandbox aktivieren und Limits konfigurieren (unterhalb der Docker-Sandbox-Einstellungen).

> 💡 Nur auf Linux verfügbar. Bei Fehlschlag kann ein unsicherer Fallback erlaubt werden (`allow_unsafe_fallback`).

### YAML-Referenz
```yaml
shell_sandbox:
  enabled: false
  allow_unsafe_fallback: false
  max_memory_mb: 1024
  max_cpu_seconds: 30
  max_processes: 50
  max_file_size_mb: 100
  allowed_paths:
    - path: "/tmp"
      readonly: false
```

## Whisper Integration

Spracherkennung (Speech-to-Text) über Whisper oder kompatible Provider.

**Web-UI:** Config → Integrationen → Whisper → Provider und Modus konfigurieren.

### YAML-Referenz
```yaml
whisper:
  provider: ""
  mode: "whisper"   # whisper, multimodal, local
```

## Media Conversion Integration

Medienkonvertierung über FFmpeg und ImageMagick. Konvertiert Audio-, Video- und Bilddateien zwischen Formaten.

**Web-UI:** Config → Integrationen → Media Conversion → Pfade zu FFmpeg/ImageMagick konfigurieren.

### YAML-Referenz
```yaml
tools:
  media_conversion:
    enabled: true
    readonly: false
    ffmpeg_path: ""
    imagemagick_path: ""
    timeout_seconds: 0
    max_file_size_mb: 0
```

## Video Download Integration

Video-Download über yt-dlp (YouTube und andere Plattformen). Unterstützt Docker- und Native-Modus.

**Web-UI:** Config → Integrationen → Video Download → Modus (docker/native) und Download-Verzeichnis konfigurieren.

### YAML-Referenz
```yaml
tools:
  video_download:
    enabled: true
    readonly: false
    allow_download: true
    allow_transcribe: false
    mode: "docker"
    yt_dlp_path: ""
    download_dir: ""
    max_file_size_mb: 0
    timeout_seconds: 0
    default_format: ""
    max_search_results: 0
    container_image: ""
    auto_pull: false
```

## Send YouTube Video Integration

Ermöglicht das Senden von YouTube-Videos als eingebettete Player in Chat-Nachrichten.

**Web-UI:** Config → Integrationen → Send YouTube Video → aktivieren.

### YAML-Referenz
```yaml
tools:
  send_youtube_video:
    enabled: true
```

## GolangciLint Integration

Code-Qualitäts-Checks für Go-Code über golangci-lint.

**Web-UI:** Kein eigener Config-Menüpunkt — `golangci_lint.enabled` nur per YAML.

### YAML-Referenz
```yaml
golangci_lint:
  enabled: true
```

## Homepage- und Website-Projekte

AuraGo kann **statische Sites / Homepages** per Agent-Tools anlegen, bearbeiten, bauen, deployen und in einer Registry nachverfolgen.

**Web-UI:** Config → Integrationen → Homepage → aktivieren und Pfade setzen.

| Bereich | Tools |
|---------|-------|
| Registry & Historie | `homepage_registry` — Projekte, Edits/Deploys/Probleme, **Projekt-Historie** |
| Dateien | `homepage_file`, `homepage_project` |
| Build & Deploy | `homepage_deploy`, `homepage_quality` |
| Design-Regeln | Globales `prompts/rules/homepage/DESIGN.md`; projekt-eigenes `DESIGN.md` nur als Design-Kontext |

Vor größeren Änderungen `list_history`, danach `add_history`. Bei `init_project` automatische Registrierung.

### YAML-Referenz
```yaml
homepage:
  enabled: true
```


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

**Web-UI:** **Config → Agent → KI-Agent** → Debug Mode aktivieren.

### YAML-Referenz
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

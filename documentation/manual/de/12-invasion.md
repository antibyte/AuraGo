# Kapitel 12: Invasion Control

<p align="center">
  <a href="../images/manual-missions.webp"><img src="../images/manual-missions.webp" width="480" alt="AuraGo-Gopher mit Helfern und Autopilot-Werkbank"></a>
</p>

Eggs schlüpfen auf Nests. Die Namen sind ernst gemeint.

> Web-UI und REST API. Agent-Tools: `invasion_nests`, `invasion_tasks`, `invasion_artifacts`. Der alte Name `invasion_control` bleibt kompatibel. Extra-CLI gibt es nicht.

Invasion Control deployt **AuraGo-Sub-Agenten** (Eggs) auf Remote- oder lokale Ziele (Nests). Der Master überträgt ein Worker-Binary plus generierte `config.yaml`, das Egg startet im **egg_mode** und verbindet sich per WebSocket zurück zum Master.

> **Hinweis:** Eggs sind **LLM-Sub-Agenten-Konfigurationsvorlagen**, keine Shell-Skripte, Cronjobs oder Docker-Image-Definitionen. Nests und Eggs werden in der Invasion-SQLite-Datenbank gespeichert, nicht in `config.yaml`.

---

## Konzept: Nester & Eier

### Nester (Deployment-Ziele)

Ein **Nest** beschreibt, *wo* ein Egg deployed wird:

| Feld | Werte | Beschreibung |
|------|-------|--------------|
| `access_type` | `ssh`, `docker`, `local` | Wie der Master das Ziel erreicht |
| `deploy_method` | `ssh`, `docker_remote`, `docker_ssh`, `docker_local` | Wie das Egg-Binary deployt wird |
| `docker_tls` | `""` (aus), `tls`, `mtls` | Nur `docker_remote`: unverschlüsseltes HTTP (Standard), TLS oder Mutual TLS zur Docker-Engine |
| `route` | `direct`, `ssh_tunnel`, `tailscale`, `wireguard`, `custom` | Wie das Egg den Master-WebSocket erreicht |
| `target_arch` | `linux/amd64`, `linux/arm64` | Ziel-Architektur des Binaries |
| `egg_id` | UUID | Zugewiesene Egg-Vorlage (für Hatch erforderlich) |
| `hatch_status` | siehe unten | Aktueller Deployment-Status |

Unterstützte Zugriffstypen sind **SSH**, **Docker API** und **Local**. Kubernetes ist nicht implementiert.

### Eier (Sub-Agenten-Vorlagen)

Ein **Egg** beschreibt, *wie* der deployte Worker arbeitet:

| Feld | Beschreibung |
|------|--------------|
| `name`, `description` | Lesbare Bezeichnungen |
| `model`, `provider`, `base_url` | LLM-Einstellungen (wenn `inherit_llm` aus ist) |
| `api_key_ref` | Vault-Referenz für den Egg-API-Key |
| `inherit_llm` | Master-LLM-Konfiguration verwenden (Standard: an) |
| `allowed_tools` | JSON-Array, z. B. `["shell","python"]` (leer = Shell + Python) |
| `egg_port` | HTTP-Port auf dem Ziel (Standard: `8099`) |
| `permanent` | Als systemd-Service installieren (`true`) oder einmalig starten (`false`) |
| `include_vault` | Verschlüsselten Vault-Export zum Ziel senden: den API-Key des Eggs und bei Nests mit **Secret dieses Nests in den Egg-Vault kopieren** das Secret des Nests (nur auf vertrauenswürdigen Hosts) |
| `active` | Ob das Egg zugewiesen werden kann |

```
┌─────────────────────────────────────────────────────────────┐
│  AuraGo Master (HQ)                                         │
│                                                             │
│  Eggs (Vorlagen)           Nests (Ziele)                    │
│  ├─ analytics-agent        ├─ prod-server (SSH)             │
│  ├─ edge-worker            ├─ docker-host (Docker API)      │
│  └─ inherit-llm-default    └─ local-docker (local)          │
│           │                         │                       │
│           └──────── Hatch ──────────┘                       │
│                     │                                       │
│                     ▼                                       │
│            Deploytes Egg (egg_mode Worker)                  │
│            verbindet per WS → /api/invasion/ws              │
└─────────────────────────────────────────────────────────────┘
```

---

## Voraussetzungen

### Einrichtung in der Web-UI
1. Öffne **Config → Server → Web-Konfiguration & Login** und stelle sicher, dass die Web-UI aktiviert ist (`web_config.enabled`).
2. Öffne **Invasion Control** unter `/invasion` (Radial-Menü) für Nests und Eggs.
3. Für die Agent-Tools `invasion_nests`, `invasion_tasks` und `invasion_artifacts`: setze `invasion_control.enabled` in der `config.yaml` (kein eigener Config-Menüpunkt).
4. Optional: **Config → Server → SQLite** → Pfad `invasion_path` anpassen.

### YAML-Referenz
```yaml
# config.yaml
web_config:
  enabled: true          # erforderlich für /api/invasion/* REST-Endpunkte

invasion_control:
  enabled: false         # aktiviert die fokussierten Invasion-Control-Agent-Tools (Standard: false)
  readonly: false        # true = Hatch/Stop/send_task/send_secret und andere Mutationen blockieren

sqlite:
  invasion_path: ./data/invasion.db   # Nests, Eggs, Tasks, Deployment-Historie
```

Die Web-UI ist unter `/invasion` erreichbar. REST-API-Routen sind verfügbar, wenn `web_config.enabled` true ist und die Invasion-Datenbank erfolgreich initialisiert wurde.

Bei `invasion_control.readonly: true` liefern mutierende API-Aufrufe (Hatch, Stop, send-task, send-secret, safe-reconfigure, rollback, rotate-key usw.) HTTP 403.

---

## Web-UI

Öffne **Invasion Control** unter `/invasion` (auch über das Radial-Menü erreichbar).

Die Oberfläche hat **nur zwei Tabs**:

| Tab | Zweck |
|-----|-------|
| **Nests** | Deployment-Ziele verwalten, Eggs zuweisen, hatching, stoppen, rekonfigurieren |
| **Eggs** | LLM-Sub-Agenten-Konfigurationsvorlagen verwalten |

Es gibt **keinen Deployments-Tab**. Deployment-Historie ist nur über die REST API verfügbar (`/api/invasion/nests/{id}/deployments`).

### Aktionen auf Nest-Karten

- **Bearbeiten** — Verbindungseinstellungen, zugewiesenes Egg, Deploy-Methode, Route
- **Hatch** — zugewiesenes Egg deployen (bei Status `idle`, `failed` oder `stopped`)
- **Stop** — laufendes Egg stoppen
- **Safe Reconfigure** — whitelisted Config-Patch ohne vollständiges Redeploy
- **Config History** — sichere Config-Revisionen anzeigen und zurückrollen
- **Aktivieren / Deaktivieren**
- **Löschen** — erfordert exakte Eingabe des Nest-Namens

### Aktionen auf Egg-Karten

- **Bearbeiten** — LLM-Einstellungen, Tools, Port, permanent/vault/inherit-Flags
- **Aktivieren / Deaktivieren**
- **Löschen** — erfordert exakte Eingabe des Egg-Namens

---

## Nest erstellen

### Über die Web-UI

1. Tab **Nests** → **Create New**
2. Formular ausfüllen:

| Feld | Hinweise |
|------|----------|
| Name | Pflichtfeld |
| Notes | Optional |
| Access Type | `SSH`, `Docker API` oder `Local` |
| Host / Port / Username | Für SSH und Docker; bei Local ausgeblendet, außer mit der Deploy-Methode `Docker (über SSH)` |
| Secret | SSH-Key oder Passwort; wird im Vault gespeichert. `Docker (Entfernt)` und `Docker (Lokal)` nutzen es nicht; dort ist das Feld ausgeblendet, außer ein Secret ist gespeichert, **Secret dieses Nests in den Egg-Vault kopieren** ist an oder Du hast etwas eingegeben |
| Secret dieses Nests in den Egg-Vault kopieren | Bei neuen Nests aus. Mit einem Egg mit `include_vault` kopiert es das Secret dieses Nests (`nest_<id>`) in den Vault des Eggs, wo das Egg es lesen kann. Nests von vor dieser Option behalten es eingeschaltet |
| Assign Egg | Egg auswählen oder leer lassen |
| Deploy Method | `SSH`, `Docker (Entfernt)`, `Docker (über SSH)` oder `Docker (Lokal)` |
| Docker-TLS | Nur `Docker (Entfernt)`: `Aus`, `TLS` oder `Mutual TLS`, dazu CA / Client-Zertifikat / Schlüssel |
| Target Architecture | `linux/amd64` oder `linux/arm64` |
| Route | Wie das Egg den Master-WebSocket erreicht |
| Route Config | JSON, z. B. `{"tunnel_port":8443}` oder volle WebSocket-URL bei `custom` |

3. Speichern, dann **Test Connection** (nur im Bearbeitungsmodus) zur Verbindungsprüfung

### Über die REST API

```bash
curl -X POST http://localhost:8088/api/invasion/nests \
  -H "Content-Type: application/json" \
  -d '{
    "name": "produktion-server-01",
    "access_type": "ssh",
    "host": "192.168.1.10",
    "port": 22,
    "username": "deploy",
    "secret": "-----BEGIN OPENSSH PRIVATE KEY-----\n...",
    "deploy_method": "ssh",
    "target_arch": "linux/amd64",
    "route": "direct",
    "active": true
  }'
```

Verbindung testen:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/validate
```

> 💡 **Tipp:** SSH-Keys und Passwörter beim Erstellen über UI/API im Vault speichern. Secrets werden in API-Antworten nie zurückgegeben (`has_secret: true` zeigt ein gespeichertes Credential an).

REST-Feld `export_nest_secret` (Boolean): Ein Anlegen ohne das Feld beginnt mit `false`; ein Update ohne das Feld behält den aktuellen Wert. Beim ersten Start dieses Releases werden bestehende Nests auf `true` gesetzt, sie kopieren ihr Secret also weiter, und eine bereits vorhandene `invasion.db` wird vorher nach `invasion.db.pre-export-nest-secret.bak` daneben kopiert.

### Transportsicherheit für Docker-Nests

Ohne **Docker-TLS** (Standard, siehe unten) spricht `Docker (Entfernt)` (`docker_remote`) die Docker-Engine-API des Ziels über **unverschlüsseltes HTTP** an (Standardport `2375`). Jeder Hatch und jedes Reconfigure kopiert die `config.yaml` des Eggs über diese Verbindung in den Container. Die Datei enthält den Egg-Shared-Key, den Egg-Vault-Schlüssel und mit `inherit_llm` den LLM-API-Key des Masters. Wer den Verkehr mitlesen kann, erhält diese Secrets. Wer den Engine-Port erreicht, steuert den entfernten Docker-Daemon, weil eine Engine ohne TLS Aufrufer nicht authentifiziert.

Bestehende Nests funktionieren weiter. AuraGo warnt an drei Stellen:
- im Nest-Formular
- im Bereich **Sicherheitsaudit** der Konfiguration, als Hinweis `invasion_docker_remote_plaintext`
- im Log, bei jedem Hatch und Reconfigure

Nutze `Docker (Entfernt)` ohne TLS nur in einem isolierten Netz, setze **Docker-TLS** am Nest oder stelle das Nest auf `Docker (über SSH)` (derselbe Container, siehe unten) oder auf `SSH` (das Binary) um. Beide übertragen alle Dateien über die verschlüsselte SSH-Verbindung.

Das Umstellen eines Nests schließt den unverschlüsselten TCP-Listener der Engine nicht. Solange `dockerd` mit `-H tcp://…:2375` läuft, steuert jeder, der diesen Port erreicht, den Docker-Daemon. Entferne diesen Listener auf dem Zielhost.

**Verschlüsseltes Docker (Entfernt).** Setze **Docker-TLS** am Nest:

| Docker-TLS | Wirkung |
|------------|---------|
| Aus (Standard) | Unverschlüsseltes HTTP wie bisher, Standardport `2375` |
| TLS | HTTPS. AuraGo prüft das Engine-Zertifikat gegen die eingefügte CA oder, wenn das CA-Feld leer ist, gegen die Systemzertifikate. Standardport `2376` |
| Mutual TLS | Wie TLS, zusätzlich legt AuraGo ein Client-Zertifikat mit Schlüssel vor: das Setup von `dockerd --tlsverify` |

CA, Client-Zertifikat und Schlüssel liegen im Vault (`nest_docker_tls_<nest-id>`), nie in der Invasion-Datenbank, und die API gibt sie nie zurück. Sie werden mit dem Nest oder beim Abschalten von TLS gelöscht. AuraGo überspringt die Zertifikatsprüfung nie. `HTTP_PROXY` gilt für unverschlüsselte Nests, `HTTPS_PROXY` für TLS-Nests und `NO_PROXY` für beide; durch einen Proxy läuft TLS Ende-zu-Ende.

REST-Felder: `docker_tls` (`""`, `"tls"`, `"mtls"`), `docker_tls_ca`, `docker_tls_cert`, `docker_tls_key`. Ein Update ohne `docker_tls` behält den aktuellen Modus; leere PEM-Felder behalten das gespeicherte Material.

Ein leeres CA-Feld behält eine gespeicherte CA. Um bei TLS wieder die Systemzertifikate zu nutzen, speichere zweimal: zuerst mit **Docker-TLS** auf `Aus`, was das gespeicherte Material löscht, dann mit `TLS` und leerem CA-Feld.

Bevor Du auf ein Release ohne Docker-TLS zurückgehst, schalte TLS bei den Nests ab oder lösche sie. Ein älteres Release ignoriert `docker_tls` und spricht den TLS-Port mit unverschlüsseltem HTTP an, diese Nests funktionieren dann nicht mehr.

**Docker über SSH.** Die Deploy-Methode `Docker (über SSH)` (`docker_ssh`) startet denselben Egg-Container wie `Docker (Entfernt)`. Sie erreicht den Engine-Socket `/var/run/docker.sock` über eine SSH-Verbindung mit Host, Port (Standard `22`), Benutzername und Secret des Nests. Jede Anfrage, auch die Kopie der Konfiguration, ist verschlüsselt, und Host-Keys werden wie bei SSH-Deploys gegen `known_hosts` geprüft.

Voraussetzungen auf dem Ziel:
- der SSH-Benutzer darf den Docker-Socket nutzen, zum Beispiel als Mitglied der Gruppe `docker`. Die Mitgliedschaft in `docker` kommt Root-Rechten auf diesem Host gleich.
- `sshd` erlaubt Stream-Local-Forwarding (`AllowStreamLocalForwarding`, Standard `yes`). Auch `DisableForwarding yes` in der `sshd_config` verhindert es, ebenso `restrict` oder `no-port-forwarding` in der Zeile des Schlüssels in `authorized_keys`.
- der Engine-Socket ist `/var/run/docker.sock`. Der Pfad ist fest, deshalb wird Rootless Docker (Socket unter `$XDG_RUNTIME_DIR`) nicht unterstützt.
- der Host-Key steht in `~/.ssh/known_hosts` des Benutzers, unter dem der AuraGo-Dienst läuft, nicht in dem Deines eigenen Logins. Für einen anderen Port als `22` lautet der Eintrag `[host]:port`, zum Beispiel mit `ssh-keyscan -p 2222 host >> ~/.ssh/known_hosts`.

Wähle für solche Nests den Zugriffstyp `SSH`. `Docker (über SSH)` ignoriert `HTTP_PROXY`, `HTTPS_PROXY` und `NO_PROXY`: Die SSH-Verbindung geht direkt zum Host.

| Fehler bei Test Connection oder in `hatch_error` | Ursache |
|--------------------------------------------------|---------|
| `open /var/run/docker.sock: ssh: rejected: connect failed ("open failed")` | Entweder hat eine Forwarding-Richtlinie den Socket abgelehnt (`AllowStreamLocalForwarding no`, `DisableForwarding yes` oder `restrict` / `no-port-forwarding` in `authorized_keys`), oder der Socket fehlt (Docker läuft nicht, Rootless Docker) bzw. der SSH-Benutzer darf ihn nicht öffnen. OpenSSH antwortet in beiden Fällen gleich; nur bei einer Ablehnung durch eine Richtlinie steht im `sshd`-Log des Ziels `refused streamlocal port forward` |
| `negotiate Docker API: context deadline exceeded` | SSH-Anmeldung, Öffnen des Sockets und Versionsantwort waren nicht innerhalb von 20 Sekunden fertig (die SSH-Anmeldung allein darf bis zu 10 dauern), siehe [Troubleshooting](#verbindung-verweigert--timeout) |

> ⚠️ Ältere AuraGo-Versionen behandeln unbekannte Deploy-Methoden als `SSH`. Nach einem Downgrade würde ein `docker_ssh`-Nest das Binary per SSH statt des Containers deployen. Stelle solche Nests vor einem Downgrade auf eine andere Methode um.

---

## Egg erstellen

### Über die Web-UI

1. Tab **Eggs** → **Create New**
2. Konfigurieren:

| Feld | Hinweise |
|------|----------|
| Name | Pflichtfeld |
| Description | Was dieser Sub-Agent tut |
| Provider / Model / Base URL | Wenn **Inherit LLM** aus ist |
| API Key | Im Vault gespeichert |
| Egg Port | Standard `8099` |
| Allowed Tools | JSON-Array, z. B. `["shell","python"]` |
| Permanent | systemd-Service vs. einmaliger Lauf |
| Include Vault | Master-Vault exportieren (sicherheitskritisch) |
| Inherit LLM | Master-LLM-Einstellungen nutzen (Standard: an) |

### Über die REST API

```bash
curl -X POST http://localhost:8088/api/invasion/eggs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "edge-analytics",
    "description": "Leichter Analytics-Sub-Agent",
    "inherit_llm": true,
    "egg_port": 8099,
    "allowed_tools": "[\"shell\",\"python\"]",
    "permanent": true,
    "active": true
  }'
```

Egg einem Nest zuweisen (UI-Dropdown oder API):

```bash
curl -X PUT http://localhost:8088/api/invasion/nests/{nest-id} \
  -H "Content-Type: application/json" \
  -d '{"egg_id": "{egg-id}", "name": "produktion-server-01", ...}'
```

---

## Hatch (Egg deployen)

**Hatch** deployt das zugewiesene Egg auf das Nest:

1. Master generiert Shared HMAC-Key und Egg-`config.yaml` (mit aktiviertem `egg_mode`)
2. Binary (`linux/amd64` oder `linux/arm64`), `resources.dat` und Config werden übertragen
3. Egg-Prozess startet auf dem Ziel (systemd bei `permanent`, sonst einmalig). Auf einem SSH-Nest wird ein Egg, das von einem früheren Hatch noch läuft, ersetzt: Der systemd-Service wird neu gestartet, und ein einmalig gestarteter Prozess bekommt SIGTERM und bis zu 10 Sekunden zum Beenden (danach SIGKILL), bevor der neue startet
4. Egg verbindet sich mit `ws[s]://<master>/api/invasion/ws` und authentifiziert sich
5. Master setzt den Nest-Status auf `running`, wenn die WebSocket-Verbindung steht

### Über die Web-UI

1. Nest muss ein Egg zugewiesen und **aktiv** sein
2. **Hatch** auf der Nest-Karte klicken
3. Status aktualisiert sich automatisch (`hatching` → `running` oder `failed`)

### Über die REST API

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/hatch
```

Status abfragen:

```bash
curl http://localhost:8088/api/invasion/nests/{nest-id}/status
```

Egg stoppen:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/stop
```

---

## Hatch-Status & Lifecycle

### Nest-`hatch_status`-Werte

| Status | Bedeutung |
|--------|-----------|
| `idle` | Kein aktives Deployment (Anfangszustand) |
| `hatching` | Deployment läuft |
| `running` | Egg deployt; WebSocket verbunden (oder kürzlich verbunden) |
| `failed` | Deployment- oder Heartbeat-Fehler (`hatch_error` enthält Details) |
| `stopped` | Egg manuell gestoppt oder Verbindung verloren |

Status-Übergänge:

```
idle ──Hatch──► hatching ──Erfolg──► running
                  │                      │
                  │ Fehler               ├── disconnect / stop ──► stopped
                  ▼                      │
               failed ◄── heartbeat timeout
                  │
                  └── erneut Hatch (von idle/failed/stopped)
```

Die UI zeigt außerdem:

- **WebSocket connected / disconnected** Badge
- **Config drift / synced** Badge (`desired_config_rev` vs `applied_config_rev`)
- **Telemetry** (CPU, RAM, Uptime) bei aktiver Verbindung

Heartbeat-Monitor: Prüfung alle 30 Sekunden; nach 90 Sekunden ohne Heartbeat → `failed` mit `heartbeat timeout`.

---

## Routing-Optionen

Das Feld `route` steuert, wie das deployte Egg den Master-WebSocket (`/api/invasion/ws`) erreicht:

| Route | Verhalten |
|-------|-----------|
| `direct` | Egg verbindet sich mit Nest-`host` (oder Master-Host als Fallback) |
| `ssh_tunnel` | Egg nutzt localhost; Tunnel über `route_config` |
| `tailscale` | Verbindung über Tailscale-IP/Hostname |
| `wireguard` | Verbindung über WireGuard-Endpoint |
| `custom` | Volle WebSocket-URL in `route_config` |

Bei `docker_local`-Deployments nutzt der Master `host.docker.internal`, damit der Container den Host erreichen kann.

---

## Tasks, Artefakte & Nachrichten

Sobald ein Egg verbunden ist (`ws_connected: true`):

### Task senden

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/send-task \
  -H "Content-Type: application/json" \
  -d '{"description": "Prüfe Festplattenbelegung und fasse zusammen", "timeout": 120}'
```

Task-Status: `pending` → `sent` → `acked` → `completed` / `failed` / `timeout`

```bash
curl http://localhost:8088/api/invasion/nests/{nest-id}/tasks
curl http://localhost:8088/api/invasion/tasks/{task-id}
```

### Runtime-Secret senden

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/send-secret \
  -H "Content-Type: application/json" \
  -d '{"key": "openrouter_api_key", "value": "sk-..."}'
```

### Artefakte

- `POST /api/invasion/artifacts/offer` — Egg bietet Datei an (HMAC-signiert)
- `POST /api/invasion/artifacts/upload/{token}` — Upload
- `GET /api/invasion/artifacts/{id}` — Download

### Egg-Nachrichten

- `POST /api/invasion/messages` — Alerts/Benachrichtigungen vom Egg zum Master

Ausstehende Tasks werden nach einem Reconnect automatisch erneut gesendet.

---

## Safe Reconfigure & Config History

**Safe Reconfigure** wendet whitelisted Änderungen auf ein laufendes Egg an, ohne vollständiges Redeploy. Verfügbar in der Web-UI (🔧) oder per API:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/safe-reconfigure \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "allowed_tools": ["shell", "python"],
    "allow_filesystem_write": true
  }'
```

Erlaubte Patch-Felder: `provider`, `base_url`, `model`, `allowed_tools`, `allow_filesystem_write`, `allow_network_requests`, `allow_remote_shell`, `allow_self_update`.

> ⚠️ Das Egg wird nach dem Anwenden neu gestartet.

Config-Historie und Rollback:

```bash
curl "http://localhost:8088/api/invasion/nests/{nest-id}/config-history?limit=20"
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/config-rollback \
  -H "Content-Type: application/json" \
  -d '{"revision_id": "{revision-id}"}'
```

Revisions-Status: `pending`, `applying`, `applied`, `failed`, `rolled_back`

---

## Deployment-Rollback & Historie

Deployment-Historie pro Nest (nur API, kein UI-Tab):

```bash
curl http://localhost:8088/api/invasion/nests/{nest-id}/deployments
```

Deployment-Status: `started`, `deployed`, `verified`, `failed`, `rolled_back`

Manueller Rollback:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/rollback
```

Shared Key rotieren:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/rotate-key
```

Bei fehlgeschlagenem Health-Check nach Deploy versucht das System einen **automatischen Rollback**.

Auf einem SSH-Nest liegt das Egg in `~/.aurago-egg-<erste 8 Zeichen der Nest-ID>` des SSH-Benutzers. Health-Check, Status und **Stop** suchen einen Prozess dieses Benutzers, dessen Programmdatei das `aurago` in diesem Verzeichnis ist; ein beendetes Egg lässt den Health-Check scheitern. Ein `permanent`-Egg ist die User-Unit `aurago-egg-<…>`, deren Pfade `%h` (das Home-Verzeichnis des Benutzers) nutzen. Ohne `loginctl enable-linger <SSH-Benutzer>` (braucht root) beenden sich der User-Manager und ein `permanent`-Egg etwa 10 Sekunden nach der letzten Sitzung des Benutzers und starten erst mit der nächsten Anmeldung wieder.

---

## Egg Mode (Worker-Konfiguration)

Deployte Eggs laufen mit aktiviertem `egg_mode` in ihrer generierten `config.yaml`. Auf Worker-Instanzen: **Config → Integrationen → Egg Mode** (Master-URL, Egg-/Nest-ID) — auf dem Master wird Egg Mode beim Hatch automatisch gesetzt.

### YAML-Referenz
```yaml
egg_mode:
  enabled: true
  master_url: "wss://aurago.example.com/api/invasion/ws"
  shared_key: ""         # hex-codierter AES-256-Key (beim Deploy gesetzt)
  egg_id: ""
  nest_id: ""
  tls_skip_verify: false # true bei selbstsigniertem Master-TLS
```

Der Master generiert diese Konfiguration beim Hatch. Für verwaltete Eggs wird `egg_mode` nicht manuell bearbeitet.

---

## Agent-Tools: `invasion_nests`, `invasion_tasks`, `invasion_artifacts`

Bei `invasion_control.enabled: true` kann der Agent Nests, Eggs, Tasks und Artefakte programmatisch verwalten. Die fokussierten Tools sind `invasion_nests`, `invasion_tasks` und `invasion_artifacts`; der alte Dispatch-Name `invasion_control` bleibt kompatibel.

| Operation | Beschreibung |
|-----------|--------------|
| `list_nests`, `list_eggs` | Alle Einträge auflisten (ohne Secrets) |
| `nest_status`, `egg_status` | Statusabfrage |
| `assign_egg` | Egg einem Nest zuweisen |
| `hatch_egg`, `stop_egg` | Deployen oder stoppen |
| `send_task`, `task_status`, `get_result` | Task-Verwaltung |
| `send_secret` | Runtime-Secret an verbundenes Egg senden |
| `list_artifacts`, `get_artifact`, `read_artifact` | Artefakt-Zugriff |
| `list_egg_messages`, `ack_egg_message` | Egg-Benachrichtigungen |

Details: [Kapitel 22: Interne Tools](./22-interne-tools.md)

---

## REST API Referenz

| Endpunkt | Methode | Beschreibung |
|----------|---------|--------------|
| `/api/invasion/nests` | GET, POST | Nests auflisten / erstellen |
| `/api/invasion/nests/{id}` | GET, PUT, DELETE | Nest abrufen / bearbeiten / löschen |
| `/api/invasion/nests/{id}/toggle` | POST | Nest aktivieren/deaktivieren |
| `/api/invasion/nests/{id}/validate` | POST | Verbindung testen |
| `/api/invasion/nests/{id}/hatch` | POST | Zugewiesenes Egg deployen |
| `/api/invasion/nests/{id}/stop` | POST | Laufendes Egg stoppen |
| `/api/invasion/nests/{id}/status` | GET | Hatch-Status + Telemetry |
| `/api/invasion/nests/{id}/send-task` | POST | Task an verbundenes Egg senden |
| `/api/invasion/nests/{id}/send-secret` | POST | Verschlüsseltes Secret senden |
| `/api/invasion/nests/{id}/tasks` | GET | Task-Historie |
| `/api/invasion/nests/{id}/rotate-key` | POST | Shared Key rotieren |
| `/api/invasion/nests/{id}/rollback` | POST | Deployment zurückrollen |
| `/api/invasion/nests/{id}/deployments` | GET | Deployment-Historie |
| `/api/invasion/nests/{id}/safe-reconfigure` | POST | Sicheren Config-Patch anwenden |
| `/api/invasion/nests/{id}/config-history` | GET | Config-Revisionshistorie |
| `/api/invasion/nests/{id}/config-rollback` | POST | Config-Revision zurückrollen |
| `/api/invasion/eggs` | GET, POST | Eggs auflisten / erstellen |
| `/api/invasion/eggs/{id}` | GET, PUT, DELETE | Egg abrufen / bearbeiten / löschen |
| `/api/invasion/eggs/{id}/toggle` | POST | Egg aktivieren/deaktivieren |
| `/api/invasion/tasks/{id}` | GET | Task nach ID |
| `/api/invasion/artifacts/offer` | POST | Artefakt-Angebot vom Egg |
| `/api/invasion/artifacts/upload/{token}` | POST | Artefakt-Upload |
| `/api/invasion/artifacts/{id}` | GET | Artefakt-Download |
| `/api/invasion/messages` | POST | Egg-Nachrichten |
| `/api/invasion/ws` | WS | Egg ↔ Master Bridge |

---

## Troubleshooting

### Verbindung verweigert / Timeout

1. Ziel erreichbar? (`ping`, `ssh`)
2. Firewall und Port prüfen (22 für SSH und Docker über SSH, 2375 für Docker API, 2376 für Docker API mit TLS)
3. **Test Connection** oder `POST .../validate` ausführen
4. Bei SSH-Nests: Secret muss konfiguriert sein
5. `Docker (über SSH)` scheitert mit `negotiate Docker API: context deadline exceeded`: Die Prüfung der Docker-API-Version zu Beginn jeder Operation schließt die SSH-Anmeldung und das Öffnen des Sockets ein. Sie hat 20 Sekunden; die SSH-Anmeldung allein darf bis zu 10 davon dauern. Reverse-DNS-Abfragen (`UseDNS yes`), Verzögerungen durch PAM oder LDAP oder eine Verbindung mit hoher Latenz können die Anmeldung verlangsamen. Beschleunige die Anmeldung auf dem Ziel, zum Beispiel mit `UseDNS no`, oder nutze eine andere Deploy-Methode.

### Authentifizierung fehlgeschlagen

1. Benutzername und SSH-Key/Passwort prüfen
2. Key-Berechtigungen lokal (`chmod 600`)
3. `authorized_keys` auf dem Ziel prüfen

### Hatch fehlgeschlagen

1. `hatch_error` am Nest prüfen (UI oder `GET /api/invasion/nests/{id}`)
2. Korrektes `target_arch`-Binary auf dem Master vorhanden?
3. Bei Docker: Daemon-Zugriff und `deploy_method` prüfen
4. Scheitert ein Hatch, bevor AuraGo die neue Egg-Konfiguration gesendet hat, behält AuraGo den bisherigen Shared Key des Eggs, und ein Egg, das noch läuft, verbindet sich weiter. Das gilt für einen fehlgeschlagenen Image-Pull, ein abgelehntes Anlegen des Containers und bei SSH-Nests für Fehler bis zum Hochladen des Binarys. Scheitert der Hatch später, oder schlägt sein Health-Check fehl und AuraGo kehrt zum vorherigen Egg zurück, kann sich dieses Egg erst nach einem erfolgreichen Hatch wieder verbinden.

### Egg verbindet nicht (`running`, aber `ws_connected: false`)

1. `route` und `route_config` prüfen
2. Bei `docker_local`: `host.docker.internal` erreichbar?
3. Bei HTTPS-Master: TLS/`tls_skip_verify` prüfen

### Heartbeat-Timeout → `failed`

WebSocket-Verbindung verloren oder Egg antwortet nicht. Re-Hatch oder Remote-Prozess prüfen.

| Fehler | Ursache | Lösung |
|--------|---------|--------|
| `No egg assigned` | Kein `egg_id` | Egg zuweisen vor Hatch |
| `Hatch already in progress` | Paralleler Hatch | Auf laufenden Hatch warten |
| `No active WebSocket connection` | Egg offline | Re-Hatch oder Remote-Prozess prüfen |
| `Shared key not found` | Fehlender Deploy-Status | Nest erneut hatchen |

---

## Protokoll-Upgrade

Master und Eggs müssen Protokoll v2 unterstützen. Eine ältere Gegenstelle wird mit
`invasion_protocol_upgrade_required` abgewiesen. Aktualisiere beide Binärdateien
und starte das betroffene Egg neu oder führe einen neuen Hatch aus. Es gibt keinen
Rückfall auf die alte Authentifizierung. Halte die Systemuhren synchron.
Jede Verbindung erhält eine neue Challenge; Identitäten, Nachrichten-IDs und
Sequenznummern sind signiert. Aufgezeichnete Nachrichten können keine andere
Verbindung authentifizieren. Nutze WSS oder ein authentifiziertes verschlüsseltes
Netz für Vertraulichkeit. SSH-Deployment und Rekonfiguration übertragen private
Dateien über verschlüsselte Standardeingabe und veröffentlichen sie atomar.

## Sicherheitshinweise

> ⚠️ **Wichtig:**
> - SSH-Keys, Passwörter und API-Keys im Vault speichern
> - `include_vault` nur auf vertrauenswürdigen Hosts nutzen
> - **Secret dieses Nests in den Egg-Vault kopieren** gibt dem Egg das Passwort oder den SSH-Key, mit dem sich der Master an seinem Host anmeldet. Bei neuen Nests ist es aus; schalte es bei älteren Nests ab, wenn ihr Egg es nicht braucht. Das Abschalten wirkt beim nächsten Hatch; ein bereits ausgerolltes Egg behält seine Kopie bis dahin.
> - `inherit_llm` kopiert den Master-API-Key in die Egg-Config — Egg-Host muss vertrauenswürdig sein
> - `invasion_control.readonly: true` für reine Monitoring-Setups
> - Bei Verdacht auf Kompromittierung Shared Keys mit `/rotate-key` rotieren
> - `Docker (Entfernt)` über unverschlüsseltes HTTP sendet Egg-Secrets im Klartext; nutze es nur in isolierten Netzen, setze Docker-TLS oder stelle auf `Docker (über SSH)` oder `SSH` um

---

## Zusammenfassung

| Feature | Verfügbarkeit |
|---------|--------------|
| **Web-UI** (`/invasion`) | ✅ Nests + Eggs Tabs |
| **REST API** | ✅ Vollständig |
| **CLI-Befehle** | ❌ Nicht implementiert |
| **SSH Deployment** | ✅ `access_type: ssh`, `deploy_method: ssh` |
| **Docker Deployment** | ✅ `docker_remote`, `docker_ssh`, `docker_local` |
| **Kubernetes** | ❌ Nicht implementiert |
| **Deployments-Tab** | ❌ Nur API (`/deployments`) |

---

**Vorheriges Kapitel:** [Kapitel 11: Mission Control](./11-missions.md)  
**Nächstes Kapitel:** [Kapitel 13: Dashboard](./13-dashboard.md)

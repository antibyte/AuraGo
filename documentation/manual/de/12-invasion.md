# Kapitel 12: Invasion Control

> ⚠️ **Hinweis:** Invasion Control ist über **Web-UI** und **REST API** verfügbar. Dedizierte CLI-Befehle für Nest-/Egg-Verwaltung sind nicht implementiert. Der Agent kann bei aktiviertem Feature das Tool `invasion_control` nutzen.

Invasion Control deployt **AuraGo-Sub-Agenten** (Eggs) auf Remote- oder lokale Ziele (Nests). Der Master überträgt ein Worker-Binary plus generierte `config.yaml`, das Egg startet im **egg_mode** und verbindet sich per WebSocket zurück zum Master.

> **Hinweis:** Eggs sind **LLM-Sub-Agenten-Konfigurationsvorlagen**, keine Shell-Skripte, Cronjobs oder Docker-Image-Definitionen. Nests und Eggs werden in der Invasion-SQLite-Datenbank gespeichert, nicht in `config.yaml`.

---

## Konzept: Nester & Eier

### Nester (Deployment-Ziele)

Ein **Nest** beschreibt, *wo* ein Egg deployed wird:

| Feld | Werte | Beschreibung |
|------|-------|--------------|
| `access_type` | `ssh`, `docker`, `local` | Wie der Master das Ziel erreicht |
| `deploy_method` | `ssh`, `docker_remote`, `docker_local` | Wie das Egg-Binary deployt wird |
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
| `include_vault` | Verschlüsselten Vault-Export zum Ziel senden (nur auf vertrauenswürdigen Hosts) |
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
| Host / Port / Username | Für SSH und Docker; bei Local ausgeblendet |
| Secret | SSH-Key oder Passwort; wird im Vault gespeichert |
| Assign Egg | Egg auswählen oder leer lassen |
| Deploy Method | `SSH`, `Docker (Remote)` oder `Docker (Local)` |
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
3. Egg-Prozess startet auf dem Ziel (systemd bei `permanent`, sonst einmalig)
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

Details: [Kapitel 22: Interne Tools](./22-internal-tools.md)

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
2. Firewall und Port prüfen (22 für SSH, 2375 für Docker API)
3. **Test Connection** oder `POST .../validate` ausführen
4. Bei SSH-Nests: Secret muss konfiguriert sein

### Authentifizierung fehlgeschlagen

1. Benutzername und SSH-Key/Passwort prüfen
2. Key-Berechtigungen lokal (`chmod 600`)
3. `authorized_keys` auf dem Ziel prüfen

### Hatch fehlgeschlagen

1. `hatch_error` am Nest prüfen (UI oder `GET /api/invasion/nests/{id}`)
2. Korrektes `target_arch`-Binary auf dem Master vorhanden?
3. Bei Docker: Daemon-Zugriff und `deploy_method` prüfen

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

## Sicherheitshinweise

> ⚠️ **Wichtig:**
> - SSH-Keys, Passwörter und API-Keys im Vault speichern
> - `include_vault` nur auf vertrauenswürdigen Hosts nutzen
> - `inherit_llm` kopiert den Master-API-Key in die Egg-Config — Egg-Host muss vertrauenswürdig sein
> - `invasion_control.readonly: true` für reine Monitoring-Setups
> - Bei Verdacht auf Kompromittierung Shared Keys mit `/rotate-key` rotieren

---

## Zusammenfassung

| Feature | Verfügbarkeit |
|---------|--------------|
| **Web-UI** (`/invasion`) | ✅ Nests + Eggs Tabs |
| **REST API** | ✅ Vollständig |
| **CLI-Befehle** | ❌ Nicht implementiert |
| **SSH Deployment** | ✅ `access_type: ssh`, `deploy_method: ssh` |
| **Docker Deployment** | ✅ `docker_remote`, `docker_local` |
| **Kubernetes** | ❌ Nicht implementiert |
| **Deployments-Tab** | ❌ Nur API (`/deployments`) |

---

**Vorheriges Kapitel:** [Kapitel 11: Mission Control](./11-missions.md)  
**Nächstes Kapitel:** [Kapitel 13: Dashboard](./13-dashboard.md)

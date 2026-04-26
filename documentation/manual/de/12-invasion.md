# Kapitel 12: Invasion Control

> ⚠️ **Hinweis:** Invasion Control ist primär über die **Web-UI** und **REST API** verfügbar. CLI-Befehle sind in der aktuellen Version nicht implementiert.

Invasion Control ermöglicht das Deployment und die Verwaltung von AuraGo-Agenten auf Remote-Servern.

---

## Konzept: Nester & Eier

Das Invasion Control-System nutzt die gleiche Metapher wie Mission Control:

### Nester (Nests)

Ein **Nest** ist ein Zielserver oder eine Umgebung, auf der ein Agent deployed wird.

```
┌─────────────────────────────────────────────────────────────┐
│                     DEIN AURAGO (HQ)                        │
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │   Nest A    │    │   Nest B    │    │   Nest C    │     │
│  │  (AWS EC2)  │◄──►│  (On-Prem)  │◄──►│  (Raspberry)│     │
│  │             │    │             │    │             │     │
│  │ 🥚 Agent v1 │    │ 🥚 Agent v2 │    │ 🥚 Agent v1 │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│         ▲                                                   │
│         │ SSH / Docker / Tailscale                         │
│         ▼                                                   │
│  ┌─────────────┐                                           │
│  │   Nest D    │                                           │
│  │  (Docker)   │                                           │
│  └─────────────┘                                           │
└─────────────────────────────────────────────────────────────┘
```

### Eier (Eggs)

Ein **Ei** ist eine Agent-Konfiguration, die auf ein Nest deployed wird.

> 🔍 **Deep Dive:** Die Begriffe stammen aus der Vorstellung, dass AuraGo "Eier" (Agent-Pakete) in "Nester" (Server) legt, wo sie dann "schlüpfen" (starten).

---

## Voraussetzungen

Invasion Control erfordert:

```yaml
# config.yaml
invasion_control:
  enabled: true

tools:
  inventory:
    enabled: true  # Für SSH-Verbindungen
```

---

## SSH-Verbindungen einrichten

### Voraussetzungen

- SSH-Zugriff auf den Zielserver
- SSH-Key (empfohlen) oder Passwort

### SSH-Key erstellen

```bash
# Neues Key-Paar generieren (falls nicht vorhanden)
ssh-keygen -t ed25519 -C "aurago-deploy"

# Public Key auf Zielserver kopieren
ssh-copy-id -i ~/.ssh/id_ed25519.pub user@zielserver

# Verbindung testen
ssh -i ~/.ssh/id_ed25519 user@zielserver "echo 'OK'"
```

### Nest-Konfiguration

```yaml
# config.yaml
nests:
  - name: "produktion-server-01"
    type: "ssh"
    host: "203.0.113.10"
    port: 22
    user: "aurago"
    key_file: "~/.ssh/id_ed25519"
    working_dir: "/opt/aurago"
```

> ⚠️ **Sicherheit:** Verwende niemals Passwörter im Klartext. Nutze immer SSH-Keys.

---

## Docker Deployment

### Docker Nest konfigurieren

```yaml
nests:
  - name: "docker-local"
    type: "docker"
    socket: "unix:///var/run/docker.sock"
```

### Docker Ei erstellen

```yaml
eggs:
  - name: "aurago-edge-agent"
    type: "docker"
    image: "aurago/edge-agent:latest"
    
    environment:
      - "AURAGO_MODE=edge"
      - "AURAGO_HUB=wss://hq.example.com/ws"
    
    resources:
      memory: "512m"
      cpus: "1.0"
    
    ports:
      - "8088:8088"
    
    volumes:
      - "edge-data:/app/data"
```

---

## Remote Agents deployen

### Über die Web-UI

1. **Öffne** Invasion Control (🥚 im Radial-Menü)
2. **Wähle** das Tab "Deploy"
3. **Wähle** ein Nest aus der Liste
4. **Wähle** ein Ei oder erstelle eine neue Konfiguration
5. **Konfiguriere**:
   - Agent-Name
   - Verbindungsmodus
   - Ressourcen-Limits
6. **Klicke** "Deploy"
7. **Warte** auf den Status "Running"

### Über die REST API

```bash
# Nester verwalten
curl http://localhost:8088/api/invasion/nests              # Alle Nester auflisten
curl -X POST http://localhost:8088/api/invasion/nests      # Nest erstellen
curl http://localhost:8088/api/invasion/nests/{id}         # Nest abrufen
curl -X PUT http://localhost:8088/api/invasion/nests/{id}  # Nest bearbeiten
curl -X DELETE http://localhost:8088/api/invasion/nests/{id}  # Nest löschen
curl -X POST http://localhost:8088/api/invasion/nests/{id}/toggle   # Nest aktivieren/deaktivieren
curl -X POST http://localhost:8088/api/invasion/nests/{id}/validate # Nest-Verbindung prüfen

# Eier verwalten
curl http://localhost:8088/api/invasion/eggs               # Alle Eier auflisten
curl -X POST http://localhost:8088/api/invasion/eggs       # Ei erstellen
curl http://localhost:8088/api/invasion/eggs/{id}          # Ei abrufen
curl -X PUT http://localhost:8088/api/invasion/eggs/{id}   # Ei bearbeiten
curl -X DELETE http://localhost:8088/api/invasion/eggs/{id}   # Ei löschen
curl -X POST http://localhost:8088/api/invasion/eggs/{id}/toggle  # Ei aktivieren/deaktivieren

# Ei auf Nest ausbrüten (deployen)
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/hatch \
  -H "Content-Type: application/json" \
  -d '{"egg_id": "aurago-edge-agent"}'

# WebSocket-Verbindung
ws://localhost:8088/api/invasion/ws
```

---

## Lifecycle Management

### Status-Übergänge

```
┌─────────┐    Deploy    ┌──────────┐
│  None   │─────────────►│ Creating │
└─────────┘              └────┬─────┘
                              │
                              ▼ Download
                         ┌──────────┐
              Success    │Installing│
              ┌──────────┤          │
              │          └────┬─────┘
              ▼               │
         ┌─────────┐          │
   ┌────►│ Running │◄─────────┘
   │     └────┬────┘
   │          │
   │ Stop     │ Health-Check
   │          │ fehlgeschlagen
   ▼          ▼
┌─────────┐  ┌─────────┐
│  Idle   │  │  Error  │
└────┬────┘  └────┬────┘
     │            │
     │ Start      │ Restart
     └────────────┘
```

### Verwaltung über API

| Aktion | API-Endpunkt |
|--------|-------------|
| Nester auflisten | `GET /api/invasion/nests` |
| Nest erstellen | `POST /api/invasion/nests` |
| Nest abrufen | `GET /api/invasion/nests/{id}` |
| Nest bearbeiten | `PUT /api/invasion/nests/{id}` |
| Nest löschen | `DELETE /api/invasion/nests/{id}` |
| Nest aktivieren/deaktivieren | `POST /api/invasion/nests/{id}/toggle` |
| Nest-Verbindung prüfen | `POST /api/invasion/nests/{id}/validate` |
| Ei ausbrüten (deployen) | `POST /api/invasion/nests/{id}/hatch` |
| Eier auflisten | `GET /api/invasion/eggs` |
| Ei erstellen | `POST /api/invasion/eggs` |
| Ei abrufen | `GET /api/invasion/eggs/{id}` |
| Ei bearbeiten | `PUT /api/invasion/eggs/{id}` |
| Ei löschen | `DELETE /api/invasion/eggs/{id}` |
| Ei aktivieren/deaktivieren | `POST /api/invasion/eggs/{id}/toggle` |
| WebSocket | `WS /api/invasion/ws` |

---

## Verbindungstypen

### SSH

Direkte Verbindung über SSH-Protokoll.

```yaml
nest:
  type: "ssh"
  host: "server.example.com"
  user: "admin"
  key_file: "~/.ssh/id_rsa"
```

| Vorteil | Nachteil |
|---------|----------|
| Universell verfügbar | Manuelle Key-Verwaltung |
| Kein zusätzlicher Port | Latenz bei vielen Nodes |
| Sichere Authentifizierung | |

### Docker API

Verbindung über Docker API (lokal oder remote).

```yaml
nest:
  type: "docker"
  socket: "unix:///var/run/docker.sock"
```

| Vorteil | Nachteil |
|---------|----------|
| Schnell und effizient | Nur für Docker-Umgebungen |
| Einfache Verwaltung | Remote API oft unsicher |

### Local

Lokale Ausführung auf dem HQ-Server.

```yaml
nest:
  type: "local"
```

---

## Status Monitoring

### Web-UI Anzeige

Die Invasion Control-Oberfläche zeigt den Status jedes Deployments:

```
┌─────────────────────────────────────────────────────────────┐
│ Invasion Control                              [+ Deploy]    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  🟢 edge-berlin-01                                          │
│     ├─ Nest: produktion-server-01                          │
│     ├─ Version: 2.1.4                                       │
│     ├─ Status: Running (seit 3 Tagen)                      │
│     └─ Letzte Aktivität: Vor 5 Minuten                     │
│                                                             │
│  🟡 edge-munich-01                                          │
│     ├─ Status: Updating                                     │
│     └─ Fortschritt: 75%                                     │
│                                                             │
│  🔴 edge-hamburg-01                                         │
│     ├─ Status: Error                                        │
│     └─ Fehler: Connection timeout                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Status-Bedeutungen

| Status | Icon | Beschreibung |
|--------|------|--------------|
| `Creating` | 🟡 | Deployment wird erstellt |
| `Installing` | 🟡 | Software wird installiert |
| `Running` | 🟢 | Agent läuft normal |
| `Idle` | ⚪ | Agent pausiert |
| `Error` | 🔴 | Fehler aufgetreten |
| `Updating` | 🟡 | Update läuft |

### API-Abfragen

```bash
# Nest-Status prüfen
curl http://localhost:8088/api/invasion/nests/{id}

# Nest-Verbindung validieren
curl -X POST http://localhost:8088/api/invasion/nests/{id}/validate

# Ei-Status prüfen
curl http://localhost:8088/api/invasion/eggs/{id}

# Aufgabe auf einem Nest verwalten
curl http://localhost:8088/api/invasion/tasks/{task-id}
```

---

## Troubleshooting

### Verbindungsprobleme

**Symptom:** `Connection refused` oder `Timeout`

**Lösungen:**

```bash
# 1. Netzwerk-Verbindung testen
ping zielserver
ssh -v user@zielserver

# 2. Firewall prüfen
ssh user@zielserver "sudo ufw status"

# 3. SSH-Service prüfen
ssh user@zielserver "sudo systemctl status sshd"
```

### Berechtigungsfehler

**Symptom:** `Permission denied`

**Lösungen:**

```bash
# SSH-Key Berechtigungen
chmod 700 ~/.ssh
chmod 600 ~/.ssh/id_ed25519
chmod 644 ~/.ssh/id_ed25519.pub
```

### Docker-Probleme

**Symptom:** Container startet nicht

```bash
# Container-Logs prüfen
docker logs aurago-edge-01

# Ressourcen prüfen
docker stats aurago-edge-01
```

### Häufige Fehler

| Fehler | Ursache | Lösung |
|--------|---------|--------|
| `No route to host` | Netzwerk-Problem | Routing, Firewall prüfen |
| `Authentication failed` | Falscher Key | SSH-Key testen |
| `Disk full` | Kein Speicherplatz | Auf Nest aufräumen |
| `Port already in use` | Port belegt | Anderen Port wählen |

---

## Sicherheitshinweise

> ⚠️ **Wichtig:**
> - Nutze immer SSH-Keys statt Passwörtern
> - Aktiviere 2FA für externe Zugriffe
> - Beschränke Nest-Zugriff auf notwendige IPs
> - Rotiere Deployment-Tokens regelmäßig
> - Überwache ungewöhnliche Verbindungen

---

## Zusammenfassung

| Feature | Verfügbarkeit |
|---------|--------------|
| **Web-UI** | ✅ Vollständig |
| **REST API** | ✅ Vollständig |
| **CLI-Befehle** | ❌ Nicht implementiert |
| **SSH Deployment** | ✅ Unterstützt |
| **Docker Deployment** | ✅ Unterstützt |

> 💡 **Tipp:** Für das Management mehrerer AuraGo-Instanzen ist Invasion Control über die Web-UI die bevorzugte Methode.

## Synchronisierte Hinweise zu aktuellen Invasion-Features

Die aktuelle Implementierung verwaltet Nests, Eggs, Tasks, Artefakte, Deployment-Historie, sichere Konfigurationsrevisionen und Rollbacks. Nests können über lokale Docker-Connectoren, SSH-Connectoren oder andere unterstützte Transportwege angebunden werden. Eggs sind keine Tool-Namen, sondern Deployment-Konfigurationen; Aufgaben an Eggs laufen über das Tool `invasion_control` oder die `/api/invasion/*`-Endpunkte.

Wichtige Zusatzfunktionen sind `send_task`, `send_secret`, Artefakt-Angebote und Uploads, Key-Rotation, Safe-Reconfigure, Config-History und Config-Rollback. Für Live-Status nutzt die Web-UI den Invasion-WebSocket `/api/invasion/ws`.

Remote-Routing kann direkt, über SSH-Tunnel, VPN/Tailscale oder über die konfigurierte Bridge erfolgen. Secrets werden nicht frei exportiert, sondern gezielt an Eggs übertragen, wenn der Benutzer oder eine erlaubte Operation dies auslöst.

---

**Vorheriges Kapitel:** [Kapitel 11: Mission Control](./11-missions.md)  
**Nächstes Kapitel:** [Kapitel 13: Dashboard](./13-dashboard.md)

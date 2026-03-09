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
# Deployment starten
curl -X POST http://localhost:8088/api/invasion/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "egg": "aurago-edge-agent",
    "nest": "produktion-server-01",
    "name": "edge-berlin-01"
  }'

# Alle Deployments anzeigen
curl http://localhost:8088/api/invasion/deployments

# Deployment verwalten
curl -X POST http://localhost:8088/api/invasion/deployments/edge-berlin-01/stop
curl -X POST http://localhost:8088/api/invasion/deployments/edge-berlin-01/start
curl -X DELETE http://localhost:8088/api/invasion/deployments/edge-berlin-01
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
| Deploy | `POST /api/invasion/deploy` |
| Start | `POST /api/invasion/deployments/{name}/start` |
| Stop | `POST /api/invasion/deployments/{name}/stop` |
| Restart | `POST /api/invasion/deployments/{name}/restart` |
| Remove | `DELETE /api/invasion/deployments/{name}` |
| Logs | `GET /api/invasion/deployments/{name}/logs` |

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
# Status prüfen
curl http://localhost:8088/api/invasion/deployments/edge-berlin-01/status

# Logs abrufen
curl http://localhost:8088/api/invasion/deployments/edge-berlin-01/logs

# Metriken abrufen
curl http://localhost:8088/api/invasion/deployments/edge-berlin-01/metrics
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

---

**Vorheriges Kapitel:** [Kapitel 11: Mission Control](./11-missions.md)  
**Nächstes Kapitel:** [Kapitel 13: Dashboard](./13-dashboard.md)

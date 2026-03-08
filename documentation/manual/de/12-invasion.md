# Kapitel 12: Invasion Control

Invasion Control ermöglicht das Deployment und die Verwaltung von AuraGo-Agenten auf Remote-Servern. Ob Cloud-Instanzen, On-Premise-Server oder Edge-Geräte – mit Invasion Control breitest du AuraGo überall aus.

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
│  │ 🥚 Agent v2 │    │             │    │             │     │
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

Ein **Ei** ist eine Agent-Konfiguration, die auf ein Nest deployed wird. Es enthält:

- Agent-Version und -Konfiguration
- Umgebungsvariablen
- Zu installierende Tools
- Verbindungsparameter

> 🔍 **Deep Dive:** Die Begriffe stammen aus der Vorstellung, dass AuraGo "Eier" (Agent-Pakete) in "Nester" (Server) legt, wo sie dann "schlüpfen" (starten) und eigenständig arbeiten.

## SSH-Verbindungen einrichten

### Voraussetzungen

- SSH-Zugriff auf den Zielserver
- SSH-Key (empfohlen) oder Passwort
- Sudo-Rechte (für Systemd-Service)

### SSH-Key erstellen

```bash
# Neues Key-Paar generieren (falls nicht vorhanden)
ssh-keygen -t ed25519 -C "aurago-deploy"

# Public Key auf Zielserver kopieren
ssh-copy-id -i ~/.ssh/id_ed25519.pub user@zielserver

# Verbindung testen
ssh -i ~/.ssh/id_ed25519 user@zielserver "echo 'OK'"
```

### Nest-Konfiguration (SSH)

```yaml
nests:
  - name: "produktion-server-01"
    type: "ssh"
    host: "203.0.113.10"
    port: 22
    user: "aurago"
    auth:
      type: "key"
      private_key: "~/.ssh/id_ed25519"
      # oder:
      # type: "password"
      # password: "${SSH_PASSWORD}"  # Aus Umgebungsvariable
    
    # Optionale Einstellungen
    sudo: true                    # Sudo für Installation
    working_dir: "/opt/aurago"    # Installationspfad
    
    # ProxyJump (Bastion Host)
    proxy:
      host: "bastion.example.com"
      user: "jumpuser"
      key: "~/.ssh/bastion_key"
```

> ⚠️ **Sicherheit:** Verwende niemals Passwörter im Klartext. Nutze immer SSH-Keys oder Umgebungsvariablen.

## Docker Deployment

### Docker Nest konfigurieren

```yaml
nests:
  - name: "docker-local"
    type: "docker"
    # Lokaler Docker Socket
    socket: "unix:///var/run/docker.sock"
    
    # ODER: Remote Docker API (nicht empfohlen für Produktion)
    # host: "tcp://docker.example.com:2376"
    # tls:
    #   ca_file: "/path/to/ca.pem"
    #   cert_file: "/path/to/cert.pem"
    #   key_file: "/path/to/key.pem"
    
    network: "aurago-network"     # Docker-Netzwerk
    volumes:
      - "aurago-data:/data"       # Persistente Daten
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
      - "AURAGO_TOKEN=${EDGE_TOKEN}"
    
    resources:
      memory: "512m"              # RAM-Limit
      cpus: "1.0"                 # CPU-Limit
    
    ports:
      - "8088:8088"              # Web-UI
    
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "edge-data:/app/data"
    
    restart_policy: "unless-stopped"
```

### Multi-Stage Deployment

```bash
# Auf HQ: Ei auf Nest deployen
./aurago invasion deploy \
  --egg aurago-edge-agent \
  --nest docker-local \
  --name "edge-node-01"

# Status prüfen
./aurago invasion status edge-node-01

# Logs anzeigen
./aurago invasion logs edge-node-01 --follow
```

## Remote Agents deployen

### Deployment-Prozess

```
┌──────────┐     ┌──────────────────────────────────────────┐
│   HQ     │────►│ 1. Verbindung zum Nest herstellen        │
│ (aurago) │     │ 2. Vorab-Checks (Speicher, Ports, OS)   │
└──────────┘     │ 3. Binärdateien übertragen               │
                 │ 4. Konfiguration schreiben               │
                 │ 5. Service registrieren (systemd)       │
                 │ 6. Agent starten                         │
                 │ 7. Health-Check durchführen              │
                 └──────────────────────────────────────────┘
```

### Über die Web-UI

1. **Öffne** Invasion Control (🥚 im Radial-Menü)
2. **Wähle** das Tab "Deploy"
3. **Wähle** ein Nest aus der Liste
4. **Wähle** ein Ei oder erstelle eine neue Konfiguration
5. **Konfiguriere**:
   - Agent-Name
   - Verbindungsmodus (siehe unten)
   - Ressourcen-Limits
6. **Klicke** "Deploy"
7. **Warte** auf den Hatch-Status "Running"

### Über die CLI

```bash
# Deployment starten
./aurago invasion deploy \
  --egg aurago-edge \
  --nest produktion-server-01 \
  --name "edge-berlin-01" \
  --mode "tunnel" \
  --resources memory=1g,cpus=2

# Alle Deployments anzeigen
./aurago invasion list

# Spezifisches Deployment verwalten
./aurago invasion stop edge-berlin-01
./aurago invasion start edge-berlin-01
./aurago invasion restart edge-berlin-01
./aurago invasion remove edge-berlin-01
```

### Konfigurations-Template

```yaml
# invasion.yaml
deployments:
  - name: "edge-berlin-01"
    egg: "aurago-edge"
    nest: "produktion-server-01"
    
    # Agent-Konfiguration
    config:
      server:
        port: 8088
      llm:
        provider: "openrouter"
        model: "anthropic/claude-3-sonnet"
      
    # Verbindung zu HQ
    upstream:
      url: "wss://aurago-hq.example.com/ws"
      token: "${UPSTREAM_TOKEN}"
      reconnect_interval: "30s"
    
    # Lokale Ressourcen
    resources:
      max_memory: "2g"
      max_storage: "10g"
      
    # Auto-Update
    updates:
      enabled: true
      channel: "stable"
      schedule: "0 4 * * *"
```

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

### Verwaltungsbefehle

| Befehl | Beschreibung |
|--------|--------------|
| `deploy` | Neues Deployment erstellen |
| `start` | Gestoppten Agenten starten |
| `stop` | Laufenden Agenten stoppen |
| `restart` | Agenten neu starten |
| `remove` | Deployment komplett entfernen |
| `update` | Auf neue Version aktualisieren |
| `logs` | Logs anzeigen |
| `exec` | Befehl auf dem Agenten ausführen |
| `shell` | Interaktive Shell öffnen |

### Beispiel: Rolling Update

```bash
# Alle Edge-Nodes aktualisieren
for node in edge-berlin-01 edge-munich-01 edge-hamburg-01; do
    echo "Updating $node..."
    ./aurago invasion update "$node" --version latest
    
    # Warte auf erfolgreichen Start
    until ./aurago invasion status "$node" | grep -q "Running"; do
        echo "Waiting for $node..."
        sleep 5
    done
    
    echo "$node updated successfully"
done
```

## Verbindungstypen

### SSH

Direkte Verbindung über SSH-Protokoll.

```yaml
nest:
  type: "ssh"
  host: "server.example.com"
  user: "admin"
  key: "~/.ssh/id_rsa"
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
| Isolierte Container | |

### Local

Lokale Ausführung auf dem HQ-Server.

```yaml
nest:
  type: "local"
```

| Vorteil | Nachteil |
|---------|----------|
| Keine Netzwerkabhängigkeit | Nur lokale Ressourcen |
| Schnellste Ausführung | Keine Skalierung |

## Routing-Optionen

### Direct

Direkte Verbindung ohne Proxy oder Tunnel.

```
HQ ──────► Nest (öffentliche IP)
```

**Wann nutzen:**
- Nest hat öffentliche IP
- Keine Firewall-Einschränkungen
- Geringste Latenz benötigt

```yaml
deployment:
  routing:
    type: "direct"
    address: "203.0.113.10:8088"
```

### SSH Tunnel

Verschlüsselter Tunnel über SSH.

```
HQ ──SSH──► Bastion ──SSH──► Nest (privates Netzwerk)
```

**Wann nutzen:**
- Nest im privaten Netzwerk
- Existierende SSH-Infrastruktur
- Keine zusätzliche Software nötig

```yaml
deployment:
  routing:
    type: "ssh_tunnel"
    bastion:
      host: "bastion.example.com"
      user: "tunnel"
      key: "~/.ssh/bastion"
    local_port: 18088  # Lokaler Port auf HQ
    remote_port: 8088  # Port auf Nest
```

> 💡 SSH Tunnels werden automatisch bei HQ-Start aufgebaut und bei Verbindungsverlust neu gestartet.

### Tailscale

VPN-basierte Verbindung über Tailscale.

```
HQ ──Tailscale Mesh──► Nest (überall)
```

**Wann nutzen:**
- Verteilte Infrastruktur
- Dynamische IPs
- Einfache Verwaltung

```yaml
deployment:
  routing:
    type: "tailscale"
    hostname: "nest-berlin.tailnet.ts.net"
    # Oder:
    ip: "100.64.0.1"
```

**Voraussetzungen:**
- Tailscale auf HQ installiert
- Tailscale auf Nest installiert
- Beide im gleichen Tailnet

### Routing-Vergleich

| Feature | Direct | SSH Tunnel | Tailscale |
|---------|--------|------------|-----------|
| Einrichtung | Einfach | Mittel | Mittel |
| Sicherheit | TLS | SSH + TLS | WireGuard + TLS |
| Latenz | Niedrig | Mittel | Niedrig |
| Skalierbarkeit | Gut | Mittel | Exzellent |
| Firewall-tauglich | Nein | Ja | Ja |
| Dynamische IPs | Nein | Nein | Ja |

## Hatch Status Monitoring

### Status-Anzeige

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
│     ├─ Routing: Tailscale (100.64.0.5)                     │
│     └─ Letzte Aktivität: Vor 5 Minuten                     │
│                                                             │
│  🟡 edge-munich-01                                          │
│     ├─ Nest: produktion-server-02                          │
│     ├─ Version: 2.1.3 → 2.1.4 (Update läuft)              │
│     ├─ Status: Updating                                     │
│     └─ Fortschritt: 75%                                     │
│                                                             │
│  🔴 edge-hamburg-01                                         │
│     ├─ Nest: cloud-vm-03                                    │
│     ├─ Version: 2.1.4                                       │
│     ├─ Status: Error                                        │
│     └─ Fehler: Connection timeout zu HQ                    │
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
| `Hatching` | 🐣 | Initialisierung läuft |
| `Unknown` | ⚫ | Status nicht verfügbar |

### Health-Checks

```yaml
deployment:
  health_check:
    enabled: true
    interval: "30s"
    timeout: "10s"
    
    checks:
      - type: "http"
        endpoint: "/health"
        expected_status: 200
      
      - type: "tcp"
        port: 8088
      
      - type: "custom"
        command: "aurago health-check"
        
    on_failure:
      action: "restart"  # restart | alert | ignore
      max_restarts: 3
```

## Troubleshooting Deployments

### Verbindungsprobleme

**Symptom:** `Connection refused` oder `Timeout`

**Lösungen:**

```bash
# 1. Netzwerk-Verbindung testen
ping zielserver
ssh -v user@zielserver  # Verbose-Modus

# 2. Firewall prüfen
ssh user@zielserver "sudo ufw status"
ssh user@zielserver "sudo iptables -L"

# 3. SSH-Service prüfen
ssh user@zielserver "sudo systemctl status sshd"

# 4. Port erreichbar?
nc -zv zielserver 22
```

### Berechtigungsfehler

**Symptom:** `Permission denied` oder `sudo: no tty`

**Lösungen:**

```bash
# 1. SSH-Key Berechtigungen
chmod 700 ~/.ssh
chmod 600 ~/.ssh/id_rsa
chmod 644 ~/.ssh/id_rsa.pub

# 2. Sudo ohne Passwort (auf Zielserver)
ssh user@zielserver "sudo visudo"
# Füge hinzu: aurago ALL=(ALL) NOPASSWD:ALL

# 3. SELinux/AppArmor prüfen
ssh user@zielserver "sudo getenforce"  # SELinux
ssh user@zielserver "sudo aa-status"   # AppArmor
```

### Docker-Probleme

**Symptom:** Container startet nicht oder crashed

**Lösungen:**

```bash
# 1. Container-Logs prüfen
docker logs aurago-edge-01

# 2. Ressourcen prüfen
docker stats aurago-edge-01

# 3. Image existiert?
docker images | grep aurago

# 4. Netzwerk-Verbindung
docker network inspect aurago-network
```

### Hatch-Fehler

**Symptom:** Deployment bleibt im Status "Error"

**Vorgehen:**

```bash
# 1. Detaillierten Fehler anzeigen
./aurago invasion logs <name> --lines 100

# 2. Auf Nest manuell prüfen
ssh user@nest "sudo journalctl -u aurago -n 50"

# 3. Bereinigung und Neustart
./aurago invasion remove <name> --force
./aurago invasion deploy --egg <egg> --nest <nest> --name <name>
```

### Häufige Fehler und Lösungen

| Fehler | Ursache | Lösung |
|--------|---------|--------|
| `No route to host` | Netzwerk-Problem | Routing, Firewall prüfen |
| `Authentication failed` | Falscher Key/Passwort | SSH-Key testen, Berechtigungen |
| `Disk full` | Kein Speicherplatz | Auf Nest aufräumen |
| `Port already in use` | Port belegt | Anderen Port wählen oder Prozess beenden |
| `TLS handshake error` | Zertifikatsproblem | Systemzeit prüfen, Zertifikat erneuern |
| `Cannot pull image` | Docker Registry | Internet-Verbindung, Credentials prüfen |

### Debug-Modus

```bash
# Verbose Logging aktivieren
./aurago invasion deploy ... --debug

# SSH-Debug
./aurago invasion deploy ... --ssh-flags="-vvv"

# Lokale Tests
AURAGO_DEBUG=1 ./aurago invasion status <name>
```

## Sicherheitshinweise

> ⚠️ **Wichtig:**
> - Nutze immer SSH-Keys statt Passwörtern
> - Aktiviere 2FA für Tailscale

> - Beschränke Nest-Zugriff auf notwendige IPs
> - Rotiere Deployment-Tokens regelmäßig
> - Überwache ungewöhnliche Verbindungen

## Nächste Schritte

- **[Dashboard](13-dashboard.md)** – Alle Nodes im Überblick
- **[Mission Control](11-missions.md)** – Aufgaben auf Remote-Nodes
- **Security Guide** – Härtung deiner Infrastruktur

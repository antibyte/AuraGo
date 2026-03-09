# Kapitel 11: Mission Control

> ⚠️ **Hinweis:** Mission Control ist primär über die **Web-UI** und **REST API** verfügbar. CLI-Befehle sind in der aktuellen Version nicht implementiert.

Mission Control ist das Automatisierungszentrum von AuraGo. Hier definierst du wiederkehrende Aufgaben, die der Agent eigenständig ausführt – von einfachen Backups bis zu komplexen Monitoring-Routinen.

---

## Was sind Missions?

**Missions** sind automatisierte Aufgaben, die zu festgelegten Zeiten oder bei bestimmten Ereignissen ausgeführt werden. Sie bestehen aus:

- **Befehlen** – Was soll ausgeführt werden?
- **Zeitplan** – Wann soll es passieren?
- **Bedingungen** – Unter welchen Umständen?
- **Aktionen** – Was danach geschieht?

```
┌─────────────────────────────────────────────────────────────┐
│  Mission: "Tägliches Backup"                                 │
│  ├─ Befehl: Backup-Skript ausführen                         │
│  ├─ Zeitplan: Täglich um 02:00 Uhr                          │
│  ├─ Bedingung: Nur wenn genug Speicherplatz                 │
│  └─ Aktion: Bei Erfolg → Email senden                       │
└─────────────────────────────────────────────────────────────┘
```

> 💡 Missions laufen im Hintergrund und beeinträchtigen den normalen Chat-Betrieb nicht.

---

## Voraussetzungen

Mission Control erfordert die Aktivierung des Scheduler-Tools:

```yaml
# config.yaml
tools:
  scheduler:
    enabled: true
    readonly: false   # false = erlaubt Erstellen/Bearbeiten
```

---

## Konzepte: Nester & Eier

Das Mission Control-System basiert auf zwei zentralen Konzepten:

### Nester (Nests)

Ein **Nest** ist ein Zielserver oder eine Ausführungsumgebung, auf dem Missionen laufen können.

| Nest-Typ | Beschreibung | Anwendungsfall |
|----------|--------------|----------------|
| `local` | Lokale Maschine | Datei-Backups, lokale Skripte |
| `ssh` | SSH-Verbindung | Remote-Server verwalten |
| `docker` | Docker-Container | Containerisierte Aufgaben |

```yaml
# Beispiel-Nest in config.yaml
nests:
  - name: "produktion-db"
    type: "ssh"
    host: "db.example.com"
    user: "admin"
    key_file: "~/.ssh/id_rsa"
```

### Eier (Eggs)

Ein **Ei** ist eine wiederverwendbare Vorlage für Missionen. Es definiert die auszuführenden Befehle und Konfigurationen.

```yaml
# Beispiel-Ei
eggs:
  - name: "postgres-backup"
    type: "shell"
    command: |
      pg_dump mydb > /backups/mydb_$(date +%Y%m%d).sql
    timeout: "30m"
```

> 🔍 **Deep Dive:** Die Namensgebung stammt aus der Idee, dass ein Nest (Server) mehrere Eier (Aufgaben) "ausbrüten" kann. Ein Ei kann in mehreren Nestern deployed werden.

---

## Missions erstellen

### Über die Web-UI (Empfohlen)

1. **Öffne** Mission Control im Radial-Menü (🚀)
2. **Klicke** auf "Neue Mission"
3. **Wähle** ein vorhandenes Ei oder erstelle ein Neues
4. **Konfiguriere** den Zeitplan
5. **Speichere** die Mission

### Über die REST API

```bash
# Mission erstellen
curl -X POST http://localhost:8088/api/missions \
  -H "Content-Type: application/json" \
  -d '{
    "name": "tägliches-backup",
    "egg": "postgres-backup",
    "nest": "produktion-db",
    "schedule": "0 2 * * *",
    "enabled": true
  }'

# Alle Missionen auflisten
curl http://localhost:8088/api/missions

# Mission manuell ausführen
curl -X POST http://localhost:8088/api/missions/tägliches-backup/run
```

### Mission-Typen

| Typ | Beschreibung | Beispiel |
|-----|--------------|----------|
| `shell` | Shell-Befehl ausführen | `ls -la`, `pg_dump` |
| `script` | Skript-Datei ausführen | Python, Bash, etc. |
| `http` | HTTP-Request senden | API-Aufruf, Webhook |
| `agent` | Agent-Aktion ausführen | KI-gestützte Aufgabe |

---

## Scheduling mit Cron

AuraGo verwendet **Cron-Ausdrücke** für die Zeitplanung. Das Format ist:

```
┌───────────── Minute (0 - 59)
│ ┌───────────── Stunde (0 - 23)
│ │ ┌───────────── Tag des Monats (1 - 31)
│ │ │ ┌───────────── Monat (1 - 12)
│ │ │ │ ┌───────────── Wochentag (0 - 6, Sonntag = 0)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

### Häufige Cron-Muster

| Ausdruck | Bedeutung |
|----------|-----------|
| `0 2 * * *` | Täglich um 02:00 Uhr |
| `0 */6 * * *` | Alle 6 Stunden |
| `0 0 * * 0` | Jeden Sonntag um Mitternacht |
| `0 9-17 * * 1-5` | Stündlich von 9-17 Uhr, Mo-Fr |
| `*/15 * * * *` | Alle 15 Minuten |
| `0 0 1 * *` | Am 1. jeden Monats |

> 💡 Nutze [crontab.guru](https://crontab.guru) zum Testen deiner Cron-Ausdrücke.

### Spezielle Scheduler

```yaml
# Einmalig zu einem bestimmten Zeitpunkt
schedule: "once:2024-12-25T10:00:00"

# Intervall-basiert (alle 30 Minuten)
schedule: "interval:30m"

# Bei Systemstart
schedule: "@startup"

# Manuelle Auslösung nur
trigger: "manual"
```

---

## Manuelle Ausführung

Missions können jederzeit manuell gestartet werden – unabhängig vom Zeitplan.

### Über die Web-UI

1. **Öffne** Mission Control
2. **Finde** die gewünschte Mission
3. **Klicke** auf den ▶️ "Run Now"-Button
4. **Warte** auf die Ausführung

### Über die REST API

```bash
# Mission ausführen
curl -X POST http://localhost:8088/api/missions/tägliches-backup/run

# Mit Debug-Output
curl -X POST http://localhost:8088/api/missions/tägliches-backup/run?debug=true
```

---

## Monitoring von Missions

### Status-Übersicht (Web-UI)

Die Mission Control-Oberfläche zeigt eine Echtzeit-Übersicht:

```
┌─────────────────────────────────────────────────────────────┐
│ Mission Control                              [+ Neue]       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  🟢 tägliches-backup          Letzter Lauf: Vor 2h          │
│     ├─ Status: Running                                      │
│     ├─ Nächster Lauf: Morgen 02:00                          │
│     └─ Erfolgsrate: 98% (59/60)                            │
│                                                             │
│  🟡 wöchentlicher-report      Letzter Lauf: Vor 5d          │
│     ├─ Status: Scheduled                                    │
│     ├─ Nächster Lauf: Sonntag 00:00                         │
│     └─ Erfolgsrate: 100% (12/12)                           │
│                                                             │
│  🔴 health-check              Letzter Lauf: Vor 10m         │
│     ├─ Status: Failed                                       │
│     └─ Fehler: Connection timeout                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Status-Bedeutungen

| Status | Icon | Bedeutung |
|--------|------|-----------|
| `Scheduled` | 🕐 | Wartet auf nächsten Ausführungszeitpunkt |
| `Running` | 🟡 | Wird aktuell ausgeführt |
| `Success` | 🟢 | Erfolgreich abgeschlossen |
| `Failed` | 🔴 | Fehler aufgetreten |
| `Disabled` | ⚫ | Manuell deaktiviert |

### API-Abfrage

```bash
# Status einer Mission prüfen
curl http://localhost:8088/api/missions/tägliches-backup/status

# Ausführungsverlauf abrufen
curl http://localhost:8088/api/missions/tägliches-backup/history
```

---

## Best Practices für Automation

### 1. Idempotenz sicherstellen

Missionen sollten mehrfach ausführbar sein, ohne Probleme zu verursachen:

```bash
# ❌ Schlecht: Append ohne Prüfung
echo "backup done" >> backup.log

# ✅ Gut: Idempotent mit Prüfung
if ! grep -q "$(date +%Y-%m-%d)" backup.log; then
    echo "$(date +%Y-%m-%d): backup done" >> backup.log
fi
```

### 2. Retry-Strategien

| Szenario | Retries | Begründung |
|----------|---------|------------|
| Netzwerk-Request | 5 | Temporäre Ausfälle |
| Datenbank-Backup | 2 | Lock-Konflikte |
| API-Aufruf | 3 | Rate Limiting |

### 3. Zeitpläne verteilen

```yaml
# ❌ Schlecht: Alles zur gleichen Zeit
- "0 0 * * *"  # Alle 3 Missionen um Mitternacht

# ✅ Gut: Gleichmäßig verteilt
- "0 2 * * *"  # Backup um 02:00
- "0 3 * * *"  # Reports um 03:00
- "0 4 * * *"  # Cleanup um 04:00
```

---

## Beispiele

### Beispiel 1: Datenbank-Backup

```yaml
# config.yaml
eggs:
  - name: "postgres-backup"
    type: "shell"
    working_dir: "/opt/backups"
    command: |
      #!/bin/bash
      BACKUP_DIR="/opt/backups/postgres"
      TIMESTAMP=$(date +%Y%m%d_%H%M%S)
      FILENAME="mydb_${TIMESTAMP}.sql"
      
      # Backup erstellen
      pg_dump -h localhost -U postgres mydb > "${BACKUP_DIR}/${FILENAME}"
      
      # Alte Backups löschen (älter als 30 Tage)
      find "${BACKUP_DIR}" -name "mydb_*.sql" -mtime +30 -delete
      
      echo "Backup erstellt: ${FILENAME}"
    env:
      PGPASSWORD: "${DB_PASSWORD}"  # Aus Umgebungsvariable
```

Erstelle die Mission über die Web-UI oder API mit:
- **Egg:** postgres-backup
- **Nest:** local (oder dein Datenbank-Server)
- **Schedule:** `0 2 * * *` (täglich um 2 Uhr)

### Beispiel 2: System-Monitoring

```yaml
eggs:
  - name: "disk-space-monitor"
    type: "script"
    interpreter: "python3"
    script: |
      import shutil
      import sys
      
      disk = shutil.disk_usage('/')
      percent_used = (disk.used / disk.total) * 100
      
      print(f"Disk usage: {percent_used:.1f}%")
      
      if percent_used > 90:
          print("CRITICAL: Disk usage above 90%!")
          sys.exit(1)
      elif percent_used > 80:
          print("WARNING: Disk usage above 80%")
          sys.exit(2)
```

### Beispiel 3: API-Health-Check

```yaml
eggs:
  - name: "api-health-check"
    type: "http"
    method: "GET"
    url: "https://api.example.com/health"
    expected_status: 200
    timeout: "10s"
```

---

## Fehlerbehebung

| Problem | Ursache | Lösung |
|---------|---------|--------|
| Mission bleibt im Status "Running" | Hängender Prozess | Timeout in Egg-Config setzen |
| Cron wird nicht ausgelöst | Falscher Zeitpunkt | Cron-Ausdruck mit crontab.guru prüfen |
| Berechtigungsfehler | Falsche Rechte | Nutzer/Gruppe prüfen |
| "Scheduler tool disabled" | Tool nicht aktiviert | `tools.scheduler.enabled: true` |

### Debug-Logging

```yaml
# config.yaml
agent:
  debug_mode: true
```

Logs prüfen:
```bash
tail -f log/supervisor.log | grep -i mission
```

---

## Zusammenfassung

| Feature | Verfügbarkeit |
|---------|--------------|
| **Web-UI** | ✅ Vollständig |
| **REST API** | ✅ Vollständig |
| **CLI-Befehle** | ❌ Nicht implementiert |
| **Cron-Scheduling** | ✅ Unterstützt |
| **Manuelle Ausführung** | ✅ Über Web-UI/API |

> 💡 **Tipp:** Für komplexe Automatisierungen nutze die Web-UI. Für Integrationen in externe Systeme verwende die REST API.

---

**Vorheriges Kapitel:** [Kapitel 10: Persönlichkeit](./10-personality.md)  
**Nächstes Kapitel:** [Kapitel 12: Invasion Control](./12-invasion.md)

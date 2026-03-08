# Kapitel 11: Mission Control

Mission Control ist das Automatisierungszentrum von AuraGo. Hier definierst du wiederkehrende Aufgaben, die der Agent eigenständig ausführt – von einfachen Backups bis zu komplexen Monitoring-Routinen.

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

## Missions erstellen

### Über die Web-UI

1. **Öffne** Mission Control im Radial-Menü (🚀)
2. **Klicke** auf "Neue Mission"
3. **Wähle** ein vorhandenes Ei oder erstelle ein Neues
4. **Konfiguriere** den Zeitplan
5. **Speichere** die Mission

### Über die Config-Datei

```yaml
missions:
  - name: "tägliches-backup"
    egg: "postgres-backup"           # Referenz zum Ei
    nest: "produktion-db"            # Wo ausführen
    schedule: "0 2 * * *"           # Cron-Ausdruck
    enabled: true
    retries: 3                       # Bei Fehler wiederholen
    notifications:
      on_success: false
      on_failure: true
```

### Mission-Typen

| Typ | Beschreibung | Beispiel |
|-----|--------------|----------|
| `shell` | Shell-Befehl ausführen | `ls -la`, `pg_dump` |
| `script` | Skript-Datei ausführen | Python, Bash, etc. |
| `http` | HTTP-Request senden | API-Aufruf, Webhook |
| `agent` | Agent-Aktion ausführen | KI-gestützte Aufgabe |

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

## Manuelle Ausführung

Missions können jederzeit manuell gestartet werden – unabhängig vom Zeitplan.

### Über die Web-UI

1. **Öffne** Mission Control
2. **Finde** die gewünschte Mission
3. **Klicke** auf den ▶️ "Run Now"-Button
4. **Warte** auf die Ausführung

### Über das Terminal

```bash
# Alle Missionen auflisten
./aurago missions list

# Spezifische Mission ausführen
./aurago missions run tägliches-backup

# Mit spezifischem Nest überschreiben
./aurago missions run tägliches-backup --nest=staging-db
```

### API-Aufruf

```bash
curl -X POST http://localhost:8088/api/missions/tägliches-backup/run \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Monitoring von Missions

### Status-Übersicht

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
│     ├─ Fehler: Connection timeout                          │
│     └─ Versuche: 2/3                                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Ausführungsverlauf

Jede Mission protokolliert ihre Ausführungen:

| Zeitpunkt | Status | Dauer | Ausgabe |
|-----------|--------|-------|---------|
| 2024-01-15 02:00:05 | ✅ Success | 45s | [Anzeigen] |
| 2024-01-14 02:00:03 | ✅ Success | 42s | [Anzeigen] |
| 2024-01-13 02:00:08 | ❌ Failed | 30s | [Anzeigen] |

> 💡 Klicke auf "[Anzeigen]" um vollständige Logs zu sehen – hilfreich bei Fehlern.

## Mission-Status und Lifecycle

### Status-Übergänge

```
                    ┌─────────────┐
                    │   Created   │
                    └──────┬──────┘
                           │ enable
                           ▼
                    ┌─────────────┐
         ┌─────────│  Scheduled  │◀────────┐
         │         └──────┬──────┘         │
         │                │ trigger        │
         │                ▼                │
  retry  │         ┌─────────────┐         │ complete
  ┌──────┤         │   Running   │─────────┤
  │      │         └──────┬──────┘         │
  │      │                │                 │
  │      │    ┌───────────┼───────────┐     │
  │      │    ▼           ▼           ▼     │
  │      │ ┌──────┐   ┌──────┐   ┌────────┐ │
  │      │ │Success│   │Failed│   │Timeout │ │
  │      │ └──┬───┘   └──┬───┘   └───┬────┘ │
  │      │    │          │           │      │
  │      └────┘    ┌─────┘           │      │
  │                │ retry limit?    │      │
  │           yes  ▼                 │      │
  │         ┌──────────┐             │      │
  └────────▶│ PermFail │◀────────────┘      │
            └──────────┘────────────────────┘
```

### Status-Bedeutungen

| Status | Icon | Bedeutung |
|--------|------|-----------|
| `Created` | ⚪ | Mission erstellt, aber nicht aktiv |
| `Scheduled` | 🕐 | Wartet auf nächsten Ausführungszeitpunkt |
| `Running` | 🟡 | Wird aktuell ausgeführt |
| `Success` | 🟢 | Erfolgreich abgeschlossen |
| `Failed` | 🔴 | Fehler aufgetreten |
| `Timeout` | ⏱️ | Zeitlimit überschritten |
| `PermFail` | 🚫 | Dauerhafter Fehler (max. Retries erreicht) |
| `Disabled` | ⚫ | Manuell deaktiviert |

> ⚠️ **Achtung:** Bei `PermFail` wird die Mission automatisch deaktiviert. Du musst sie manuell reaktivieren nachdem das Problem behoben wurde.

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

### 2. Ressourcen-Monitoring

```yaml
missions:
  - name: "speicher-intensiv"
    pre_conditions:
      - type: "disk_space"
        min_gb: 10
      - type: "memory"
        min_percent: 20
```

### 3. Retry-Strategien

| Szenario | Retries | Delay | Begründung |
|----------|---------|-------|------------|
| Netzwerk-Request | 5 | 30s | Temporäre Ausfälle |
| Datenbank-Backup | 2 | 5m | Lock-Konflikte |
| API-Aufruf | 3 | Exponential | Rate Limiting |

### 4. Benachrichtigungen konfigurieren

```yaml
notifications:
  channels:
    - type: "telegram"
      chat_id: "123456789"
    - type: "email"
      to: "admin@example.com"
  rules:
    - on: "failure"
      throttle: "1h"  # Max. 1 Benachrichtigung pro Stunde
    - on: "permanent_failure"
      priority: "high"
```

### 5. Zeitpläne verteilen

```yaml
# ❌ Schlecht: Alles zur gleichen Zeit
- "0 0 * * *"  # Alle 3 Missionen um Mitternacht

# ✅ Gut: Gleichmäßig verteilt
- "0 2 * * *"  # Backup um 02:00
- "0 3 * * *"  # Reports um 03:00
- "0 4 * * *"  # Cleanup um 04:00
```

## Beispiele

### Beispiel 1: Datenbank-Backup

```yaml
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

missions:
  - name: "nächtliches-db-backup"
    egg: "postgres-backup"
    nest: "db-server"
    schedule: "0 2 * * *"
    timeout: "1h"
    retries: 2
    notifications:
      on_failure: true
```

> 💡 Nutze Umgebungsvariablen für sensible Daten – niemals Passwörter im Klartext speichern!

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

missions:
  - name: "disk-check"
    egg: "disk-space-monitor"
    nest: "local"
    schedule: "*/15 * * * *"  # Alle 15 Minuten
    exit_code_handling:
      0: "success"      # OK
      1: "critical"     # > 90%
      2: "warning"      # > 80%
```

### Beispiel 3: Wöchentlicher Report

```yaml
eggs:
  - name: "weekly-report"
    type: "agent"
    prompt: |
      Erstelle einen Wochenbericht mit:
      1. Zusammenfassung der System-Logs der letzten 7 Tage
      2. Anzahl der API-Requests pro Endpunkt
      3. Fehler-Rate und kritische Events
      4. Empfehlungen für Optimierungen
      
      Speichere den Report als PDF unter /reports/weekly/
    output_format: "pdf"

missions:
  - name: "sonntags-report"
    egg: "weekly-report"
    nest: "local"
    schedule: "0 8 * * 0"  # Sonntag 08:00
    notifications:
      on_success:
        type: "email"
        to: "team@example.com"
        attach_output: true
```

### Beispiel 4: Health-Check mit Webhook

```yaml
eggs:
  - name: "api-health-check"
    type: "http"
    method: "GET"
    url: "https://api.example.com/health"
    expected_status: 200
    timeout: "10s"
    headers:
      Authorization: "Bearer ${API_TOKEN}"

missions:
  - name: "api-monitor"
    egg: "api-health-check"
    nest: "local"
    schedule: "*/5 * * * *"  # Alle 5 Minuten
    on_failure:
      - type: "webhook"
        url: "https://alerts.example.com/webhook"
        payload: |
          {
            "severity": "critical",
            "service": "api",
            "message": "API health check failed"
          }
      - type: "telegram"
        message: "🚨 API ist nicht erreichbar!"
```

## Fehlerbehebung

| Problem | Ursache | Lösung |
|---------|---------|--------|
| Mission bleibt im Status "Running" | Hängender Prozess | Timeout verringern, Prozess prüfen |
| Cron wird nicht ausgelöst | Falscher Zeitpunkt | Cron-Ausdruck mit crontab.guru prüfen |
| Umgebungsvariablen fehlen | Shell-Kontext | Volle Pfade verwenden, env explizit setzen |
| Berechtigungsfehler | Falsche Rechte | Nutzer/Gruppe prüfen, sudo konfigurieren |
| SSH-Verbindung fehlschlägt | Key-Problem | SSH-Key testen: `ssh -i key user@host` |

## Nächste Schritte

- **[Invasion Control](12-invasion.md)** – Remote-Deployment verstehen
- **[Dashboard](13-dashboard.md)** – Mission-Metriken visualisieren
- **Missions API** – Programmatische Steuerung

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

## Mission Control Konzepte

Mission Control basiert auf folgenden Kernkonzepten:

### Missionen

Eine **Mission** ist eine geplante Aufgabe, die der Agent zu festgelegten Zeiten oder bei bestimmten Ereignissen ausführt. Missionen werden über die Web-UI oder REST API verwaltet.

| Mission-Typ | Beschreibung | Anwendungsfall |
|-------------|--------------|----------------|
| `Agent-Task` | KI-gestützte Aufgabe | Automatisierte Analysen, Berichte |
| `Cron` | Zeitgesteuert | Backups, Monitoring |
| `Event-getriggert` | Bei bestimmten Ereignissen | Webhook-gesteuerte Aktionen |

### Mission Preparation (Optional)

Mit **Mission Preparation** kann der Agent vor der eigentlichen Ausführung eine Vorbereitungsphase durchlaufen, in der er relevante Tools, Pläne und Entscheidungspunkte analysiert.

```yaml
# config.yaml
mission_preparation:
  enabled: false
  provider: ""                    # Provider-ID; leer = Haupt-LLM
  timeout_seconds: 120
  max_essential_tools: 5
  cache_expiry_hours: 24
  min_confidence: 0.5
  auto_prepare_scheduled: true
```

> 💡 Mission Preparation ist rein beratend – es blockiert niemals die Ausführung.

### Abhängigkeiten und Warteschlange

Missionen können Abhängigkeiten untereinander haben und werden in einer Warteschlange verwaltet:

- **Dependencies:** Mission B startet erst nach Abschluss von Mission A
- **Queue:** Missionen werden sequenziell abgearbeitet, wenn Ressourcen begrenzt sind
- **Triggers:** Manuelle, zeitgesteuerte oder ereignisbasierte Auslösung

---

## Missions erstellen

### Über die Web-UI (Empfohlen)

1. **Öffne** Mission Control im Radial-Menü (🚀)
2. **Klicke** auf "Neue Mission"
3. **Konfiguriere** die Mission (Name, Anweisungen, Zeitplan)
4. **Speichere** die Mission

### Über die REST API

```bash
# Mission erstellen (v2 API)
curl -X POST http://localhost:8088/api/missions/v2 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "tägliches-backup",
    "instructions": "Erstelle ein Backup der Datenbank",
    "schedule": "0 2 * * *",
    "enabled": true
  }'

# Alle Missionen auflisten
curl http://localhost:8088/api/missions/v2

# Mission manuell ausführen
curl -X POST http://localhost:8088/api/missions/v2/{mission-id}/run

# Warteschlange anzeigen
curl http://localhost:8088/api/missions/v2/queue

# Ausführungsverlauf abrufen
curl http://localhost:8088/api/missions/v2/history?limit=10

# Abhängigkeiten anzeigen
curl http://localhost:8088/api/missions/v2/dependencies
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
curl -X POST http://localhost:8088/api/missions/v2/{mission-id}/run
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
curl http://localhost:8088/api/missions/v2/{mission-id}

# Ausführungsverlauf abrufen
curl http://localhost:8088/api/missions/v2/history?mission_id={mission-id}
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

Missionen werden über die Web-UI oder REST API erstellt. Der Agent führt die Anweisungen dann zur geplanten Zeit aus.

### Beispiel 1: Tägliche System-Prüfung

Erstelle eine Mission über die Web-UI mit folgenden Einstellungen:
- **Name:** `tägliches-system-check`
- **Anweisungen:** `Prüfe Festplattenplatz, CPU-Auslastung und laufende Docker-Container. Erstelle einen kurzen Bericht.`
- **Schedule:** `0 8 * * *` (täglich um 8 Uhr)
- **Enabled:** `true`

### Beispiel 2: Wöchentlicher Bericht

- **Name:** `wöchentlicher-report`
- **Anweisungen:** `Erstelle eine Zusammenfassung aller wichtigen Ereignisse der letzten Woche aus den Logs und Memory.`
- **Schedule:** `0 9 * * 1` (jeden Montag um 9 Uhr)
- **Enabled:** `true`

### Beispiel 3: API-Health-Check

- **Name:** `api-health-check`
- **Anweisungen:** `Prüfe ob die folgenden APIs erreichbar sind: https://api.example.com/health, https://grafana.local:3000/api/health. Berichte bei Fehlern.`
- **Schedule:** `*/15 * * * *` (alle 15 Minuten)
- **Enabled:** `true`

---

## Fehlerbehebung

| Problem | Ursache | Lösung |
|---------|---------|--------|
| Mission bleibt im Status "Running" | Hängender Prozess | Timeout prüfen, Mission manuell stoppen |
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

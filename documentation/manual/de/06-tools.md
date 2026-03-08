# Kapitel 6: Werkzeuge

Das Herzstück von AuraGo: Über 30 eingebaute Werkzeuge für nahezu jede Aufgabe.

## Übersicht der Tool-Landschaft

AuraGo besitzt ein modulares Tool-System, das dynamisch erweitert werden kann. Jedes Tool ist eine spezialisierte Funktion, die der Agent autonom ausführen kann.

```
┌─────────────────────────────────────────────────────────────┐
│                     Tool-Architektur                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   User Request → Agent Loop → Tool Dispatcher → Tool Exec   │
│                         ↑                          ↓        │
│                         └──────── Result ←──────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Wie Tools ausgewählt werden

1. **Intent-Erkennung:** Der Agent analysiert deine Anfrage
2. **Tool-Matching:** Passende Tools werden identifiziert
3. **Parameter-Extraktion:** Argumente werden aus dem Kontext gewonnen
4. **Ausführung:** Tool wird mit korrekten Parametern aufgerufen
5. **Result-Interpretation:** Ausgabe wird verarbeitet und dir präsentiert

## Tool-Kategorien

### 1. Dateisystem-Tools

| Tool | Funktion | Danger Zone |
|------|----------|-------------|
| `filesystem` | Dateien lesen, schreiben, löschen, verschieben | Ja |
| `shell` | Shell-Befehle ausführen | Ja |
| `shell_background` | Hintergrund-Prozesse starten | Ja |
| `python` | Python-Code ausführen | Ja |

#### Beispiele: Dateisystem

```
Du: Liste alle Dateien im aktuellen Verzeichnis
Agent: 🛠️ Tool: filesystem (list_dir)
       📁 agent_workspace/workdir/
       ├── dokumente/
       ├── projekte/
       └── backup.tar.gz

Du: Erstelle einen neuen Ordner "mein-projekt"
Agent: 🛠️ Tool: filesystem (create_dir)
       ✅ Ordner erstellt: mein-projekt/

Du: Lies die Datei config.yaml
Agent: 🛠️ Tool: filesystem (read_file)
       📄 Inhalt von config.yaml:
       [Dateiinhalt wird angezeigt]
```

#### Beispiele: Shell

```
Du: Zeige den freien Speicherplatz
Agent: 🛠️ Tool: shell
       $ df -h
       
       📄 Ausgabe:
       Dateisystem    Größe Benutzt Verf. Verw% Eingehängt auf
       /dev/sda1       100G   45G   55G   45% /

Du: Starte einen Ping zu google.de im Hintergrund
Agent: 🛠️ Tool: shell_background
       🔄 Prozess gestartet (PID: 12345)
       Nutze /status 12345 für Fortschritt
```

> ⚠️ **Achtung:** Shell- und Python-Tools können das System verändern. Nutze Read-Only-Modus für sichere Umgebungen.

### 2. Web & API-Tools

| Tool | Funktion | Besonderheit |
|------|----------|--------------|
| `web_search` | DuckDuckGo-Suche | Kein API-Key nötig |
| `fetch_url` | Webseiten abrufen | Mit User-Agent-Rotation |
| `api_client` | Generischer HTTP-Client | Unterstützt alle Methoden |
| `wikipedia` | Wikipedia-Artikel suchen | Direkter Zugriff |
| `github` | GitHub API-Integration | Repositories, Issues, PRs |
| `virustotal` | Datei/URL auf Viren prüfen | API-Key erforderlich |

#### Beispiele: Web-Tools

```
Du: Suche nach "Go 1.22 Release Notes"
Agent: 🛠️ Tool: web_search
       🔍 5 Ergebnisse gefunden:
       1. Go 1.22 Release Notes - go.dev
       2. What's new in Go 1.22 - Blogpost
       ...

Du: Rufe die erste Seite ab und zeige mir die Highlights
Agent: 🛠️ Tool: fetch_url
       📄 Extrahierte Highlights:
       - for-Schleifen über Integer (for i := range 10)
       - Verbesserte HTTP-Routing
       - Rand-Package Verbesserungen

Du: Suche auf Wikipedia nach "Kubernetes"
Agent: 🛠️ Tool: wikipedia
       📚 Kubernetes:
       Open-Source-System zur Automatisierung der 
       Bereitstellung, Skalierung und Verwaltung von 
       containerisierten Anwendungen...
```

### 3. Docker-Tools

| Tool | Funktion | Danger Zone |
|------|----------|-------------|
| `docker_list` | Container/Images auflisten | Nein |
| `docker_logs` | Container-Logs anzeigen | Nein |
| `docker_start` | Container starten | Ja |
| `docker_stop` | Container stoppen | Ja |
| `docker_run` | Neuen Container starten | Ja |
| `docker_exec` | Befehl in Container ausführen | Ja |
| `docker_build` | Image bauen | Ja |
| `docker_prune` | Auf_cleanup durchführen | Ja |

#### Voraussetzungen

In `config.yaml`:
```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"  # oder tcp:// für Remote
```

#### Beispiele: Docker

```
Du: Zeige alle laufenden Container
Agent: 🛠️ Tool: docker_list (containers)
       🐳 Laufende Container:
       ┌─────────────┬─────────┬────────┬──────────────┐
       │ NAME        │ IMAGE   │ STATUS │ PORTS        │
       ├─────────────┼─────────┼────────┼──────────────┤
       │ nginx-web   │ nginx   │ Up 2h  │ 0.0.0.0:80   │
       │ postgres-db │ postgres│ Up 1d  │ 5432/tcp     │
       └─────────────┴─────────┴────────┴──────────────┘

Du: Zeige die Logs des nginx Containers (letzte 50 Zeilen)
Agent: 🛠️ Tool: docker_logs
       📋 Letzte Logs:
       [14:23:01] GET /index.html 200
       [14:23:05] GET /style.css 200
       ...

Du: Starte einen neuen Redis-Container
Agent: 🛠️ Tool: docker_run
       🚀 Container gestartet:
       Name: redis-cache
       Image: redis:latest
       Ports: 6379:6379
       Status: Running
```

### 4. Proxmox-Tools

| Tool | Funktion | Danger Zone |
|------|----------|-------------|
| `proxmox_list_nodes` | Nodes im Cluster anzeigen | Nein |
| `proxmox_list_vms` | VMs auflisten | Nein |
| `proxmox_vm_status` | VM-Status prüfen | Nein |
| `proxmox_vm_start` | VM starten | Ja |
| `proxmox_vm_stop` | VM stoppen | Ja |
| `proxmox_vm_shutdown` | VM herunterfahren | Ja |
| `proxmox_vm_snapshot` | Snapshot erstellen | Ja |

#### Voraussetzungen

In `config.yaml`:
```yaml
# API-Token wird im Vault gespeichert
# In UI: Config → Proxmox Integration
```

#### Beispiele: Proxmox

```
Du: Liste alle VMs auf dem Proxmox-Server
Agent: 🛠️ Tool: proxmox_list_vms
       🖥️ Virtuelle Maschinen:
       ┌────┬───────────────┬────────┬──────────┐
       │ ID │ NAME          │ STATUS │ MEMORY   │
       ├────┼───────────────┼────────┼──────────┤
       │ 100│ web-server    │ running│ 4 GB     │
       │ 101│ db-server     │ running│ 8 GB     │
       │ 102│ dev-machine   │ stopped│ 2 GB     │
       └────┴───────────────┴────────┴──────────┘

Du: Starte die VM "dev-machine"
Agent: 🛠️ Tool: proxmox_vm_start
       ✅ VM 102 wird gestartet...
       Status: Starting (ETA: ~30 Sekunden)
```

### 5. Smart Home-Tools

| Tool | Funktion | Integration |
|------|----------|-------------|
| `home_assistant` | Geräte steuern, Zustände abfragen | Home Assistant |
| `chromecast` | Audio/Video auf Chromecast streamen | Google Cast |
| `wol` | Wake-on-LAN senden | Netzwerk |
| `mqtt_publish` | MQTT-Nachrichten senden | MQTT Broker |

#### Home Assistant Beispiele

```
Du: Zeige mir alle Lichter im Wohnzimmer
Agent: 🛠️ Tool: homeassistant (get_states)
       💡 Lichter im Bereich "Wohnzimmer":
       - light.wohnzimmer_deckenlampe: an (80%)
       - light.wohnzimmer_ambient: aus
       - light.wohnzimmer_leseecke: an (45%)

Du: Schalte alle Lichter im Wohnzimmer aus
Agent: 🛠️ Tool: homeassistant (call_service)
       ✅ light.turn_off für Bereich "Wohnzimmer"
       3 Geräte wurden ausgeschaltet

Du: Wie warm ist es im Schlafzimmer?
Agent: 🛠️ Tool: homeassistant (get_states)
       🌡️ Schlafzimmer:
       - Temperatur: 21.5°C
       - Luftfeuchtigkeit: 45%
       - Fenster: geschlossen
```

> 💡 **Tipp:** Home Assistant muss in der Config mit URL und Access Token konfiguriert werden.

### 6. Google Workspace-Tools

| Tool | Funktion | Berechtigung |
|------|----------|--------------|
| `google_docs_list` | Dokumente auflisten | drive.readonly |
| `google_docs_read` | Dokument lesen | documents.readonly |
| `google_docs_create` | Dokument erstellen | documents.create |
| `google_docs_append` | Text anhängen | documents.write |
| `google_calendar_list` | Kalendereinträge anzeigen | calendar.readonly |
| `google_calendar_create` | Termin erstellen | calendar.write |

#### Voraussetzungen

In `config.yaml`:
```yaml
agent:
  enable_google_workspace: true
```

Die Authentifizierung erfolgt über den Vault (OAuth2).

#### Beispiele: Google Workspace

```
Du: Liste meine Google Docs Dokumente
Agent: 🛠️ Tool: google_workspace (list_docs)
       📄 Dokumente:
       1. "Projekt AuraGo Konzept" (bearbeitet: gestern)
       2. "Meeting Notizen Q1" (bearbeitet: vor 3 Tagen)
       3. "Urlaubsplanung 2024" (bearbeitet: letzte Woche)

Du: Erstelle ein neues Dokument "Einkaufsliste"
Agent: 🛠️ Tool: google_workspace (create_doc)
       ✅ Dokument erstellt
       Titel: Einkaufsliste
       URL: https://docs.google.com/...

Du: Füge hinzu: Milch, Brot, Eier
Agent: 🛠️ Tool: google_workspace (append_doc)
       ✅ Text hinzugefügt
       Inhalt aktualisiert
```

### 7. Email-Tools

| Tool | Funktion | Protokoll |
|------|----------|-----------|
| `email_fetch` | Emails abrufen | IMAP |
| `email_send` | Email senden | SMTP |
| `email_search` | Emails suchen | IMAP |
| `email_watcher` | Neue Emails überwachen | IMAP IDLE |

#### Voraussetzungen

In `config.yaml`:
```yaml
email:
  enabled: true
  imap_host: imap.example.com
  imap_port: 993
  smtp_host: smtp.example.com
  smtp_port: 587
  username: user@example.com
  # Passwort wird im Vault gespeichert
```

#### Beispiele: Email

```
Du: Zeige meine letzten 5 Emails
Agent: 🛠️ Tool: email_fetch
       📧 Posteingang:
       ┌────┬──────────────────┬──────────────┬────────┐
       │ ID │ Von              │ Betreff      │ Datum  │
       ├────┼──────────────────┼──────────────┼────────┤
       │ 42 │ boss@company.com │ Projektupdate│ Heute  │
       │ 41 │ amazon@amazon.de │ Bestellung   │ Gestern│
       │ ...│ ...              │ ...          │ ...    │
       └────┴──────────────────┴──────────────┴────────┘

Du: Sende eine Email an max@example.com mit dem Betreff 
    "Besprechung verschoben" und dem Text "Hallo Max, ..."
Agent: 🛠️ Tool: email_send
       ✉️ Email vorbereitet
       An: max@example.com
       Betreff: Besprechung verschoben
       
       Soll ich die Email senden? (ja/nein)

Du: Ja
Agent: ✅ Email gesendet
       Message-ID: <abc123@example.com>
```

### 8. System-Tools

| Tool | Funktion | Plattform |
|------|----------|-----------|
| `system_metrics` | CPU, RAM, Disk-Nutzung | Alle |
| `system_time` | Aktuelle Zeit/Datum | Alle |
| `system_info` | Betriebssystem-Info | Alle |
| `process_list` | Laufende Prozesse | Alle |
| `process_kill` | Prozess beenden | Ja (Danger Zone) |

#### Beispiele: System

```
Du: Zeige mir Systeminformationen
Agent: 🛠️ Tool: system_metrics
       💻 System-Status:
       ┌──────────┬────────────┬─────────────┐
       │ Metrik   │ Wert       │ Status      │
       ├──────────┼────────────┼─────────────┤
       │ CPU      │ 15%        │ 🟢 Normal   │
       │ RAM      │ 4.2/16 GB  │ 🟢 Normal   │
       │ Disk     │ 45/100 GB  │ 🟢 Normal   │
       │ Uptime   │ 3d 12h 34m │ 🟢 Stabil   │
       └──────────┴────────────┴─────────────┘

Du: Welches Betriebssystem läuft hier?
Agent: 🛠️ Tool: system_info
       🖥️ System-Informationen:
       OS: Linux (Ubuntu 22.04 LTS)
       Architektur: x86_64
       Kernel: 5.15.0-91-generic
       Go-Version: 1.21.5
```

### 9. Speicher-Tools (Memory)

| Tool | Funktion | Persistenz |
|------|----------|------------|
| `memory_save` | Information speichern | Core Memory |
| `memory_search` | Informationen suchen | Long-Term Memory |
| `note_create` | Notiz erstellen | Permanent |
| `note_list` | Notizen auflisten | Permanent |
| `todo_create` | Aufgabe erstellen | Permanent |
| `todo_list` | Aufgaben anzeigen | Permanent |

#### Beispiele: Memory

```
Du: Merke dir: Meine Lieblings-API ist die GitHub API
Agent: 🛠️ Tool: memory_save (core)
       💾 In Core Memory gespeichert:
       "Lieblings-API: GitHub API"

Du: Suche nach Informationen über "API"
Agent: 🛠️ Tool: memory_search
       🔍 Gefunden:
       - "Lieblings-API: GitHub API" (Core Memory)
       - "REST API Best Practices" (Chat vom 15.03.)
       - "API-Key für OpenRouter" (Notiz)

Du: Erstelle eine Todo: Dokumentation aktualisieren
Agent: 🛠️ Tool: todo_create
       ✅ Aufgabe erstellt:
       ID: todo-123
       Text: Dokumentation aktualisieren
       Fällig: Heute, 23:59
       Priorität: Normal
```

### 10. Weitere Tools

| Tool | Funktion | Kategorie |
|------|----------|-----------|
| `git` | Git-Operationen (clone, commit, push) | Entwicklung |
| `github` | GitHub API (Issues, PRs, Repos) | Entwicklung |
| `transcribe` | Audio zu Text (Whisper) | Medien |
| `tts` | Text zu Sprache | Medien |
| `mdns` | mDNS/Bonjour Discovery | Netzwerk |
| `wol` | Wake-on-LAN | Netzwerk |
| `koofr` | Koofr Cloud-Speicher | Cloud |
| `webdav` | WebDAV-Operationen | Cloud |
| `handoff` | An anderen Agent übergeben | Co-Agent |
| `co_agent` | Parallele Sub-Agenten starten | Co-Agent |
| `skills` | Skill-Management | Erweiterung |
| `cron` | Cron-Job Verwaltung | Automatisierung |

## Tool-Nutzung im Chat

### Implizite Tool-Auswahl

Der Agent wählt automatisch die passenden Tools:

```
Du: Erstelle eine Datei
→ filesystem (write_file)

Du: Suche im Web
→ web_search

Du: Starte einen Container
→ docker_run
```

### Explizite Tool-Anfrage

Du kannst auch direkt ein Tool nennen:

```
Du: Nutze das filesystem Tool, um die Datei config.yaml zu lesen
Du: Führe ein docker_list aus
Du: Starte einen co_agent für die Recherche
```

### Tool-Ketten

Mehrere Tools werden automatisch verkettet:

```
Du: Suche nach Go-Tutorials und speichere die beste URL als Notiz

Agent: 🛠️ Tool: web_search
       🔍 Suche nach "Go Tutorials"...
       
       🛠️ Tool: note_create
       💾 Notiz gespeichert:
       Titel: Bestes Go Tutorial
       Inhalt: https://go.dev/doc/tutorial/...
```

## Tool-Ausgaben interpretieren

### Standard-Ausgabeformat

```
🛠️ Tool: tool_name
   [Parameter oder kurze Beschreibung]
   
   📄 Ausgabe:
   [Formatierte Ergebnisse]
```

### Status-Anzeigen

| Symbol | Bedeutung |
|--------|-----------|
| ✅ | Erfolg |
| ❌ | Fehler |
| ⚠️ | Warnung |
| 🔄 | In Bearbeitung |
| ⏳ | Warten auf Eingabe |

### Debug-Modus

Mit `/debug on` siehst du mehr Details:

```
🛠️ Tool: filesystem
   Operation: write_file
   Path: /workspace/test.txt
   Content length: 245 bytes
   
   ⏱️ Execution time: 12ms
   📄 Raw output:
   {"status":"success","bytes_written":245}
```

## Read-Only vs. Read-Write Modus

### Konfiguration

In `config.yaml` können einzelne Tools oder Kategorien auf Read-Only gesetzt werden:

```yaml
# Beispiel für Read-Only Einstellungen
meshcentral:
  readonly: true
  blocked_operations: ["shutdown", "reboot"]
```

### Verhalten im Read-Only Modus

| Tool-Kategorie | Read-Only | Read-Write |
|----------------|-----------|------------|
| filesystem | Nur lesen | Lesen, schreiben, löschen |
| shell | Keine Ausführung | Alle Befehle |
| docker | Nur Listen/Logs | Starten, Stoppen, Erstellen |
| proxmox | Nur Status | VMs steuern |

### Anzeige im Chat

```
Du: Lösche die Datei wichtig.txt
Agent: ⚠️ Dateisystem ist im Read-Only Modus.
       Lösch-Operationen sind nicht erlaubt.
       
       Möchtest du stattdessen:
       1. Die Datei umbenennen (backup)?
       2. In den Papierkorb verschieben?
       3. Read-Write Modus aktivieren (erfordert Bestätigung)?
```

## Danger Zone

### Was ist die Danger Zone?

Die Danger Zone kennzeichnet Tools und Operationen, die das System verändern können:

- **Dateisystem:** Schreiben, Löschen, Verschieben
- **Shell:** Alle Befehle
- **Docker:** Container starten/stoppen, Images bauen
- **Proxmox:** VMs steuern
- **Process Management:** Prozesse beenden

### Visualisierung

```
┌─────────────────────────────────────────────────────────┐
│                    🚨 DANGER ZONE 🚨                    │
│                                                         │
│   Diese Operation kann:                                 │
│   • Dateien löschen oder überschreiben                  │
│   • Systemprozesse beenden                              │
│   • Container starten/stoppen                           │
│                                                         │
│   Bestätige mit: /confirm danger123                     │
└─────────────────────────────────────────────────────────┘
```

### Konfiguration der Danger Zone

In `config.yaml`:
```yaml
# Danger Zone ist implizit bei allen schreibenden Operationen
# Zusätzliche Sicherheiten:
agent:
  max_tool_calls: 12  # Limitiert automatische Tool-Ketten
```

> ⚠️ **Wichtig:** In Produktionsumgebungen sollte AuraGo immer in einer isolierten Umgebung laufen (VM, Docker, dedizierter Server).

## Eigene Tools erstellen

### Überblick

AuraGo kann dynamisch neue Tools zur Laufzeit erstellen:

1. Python-Skript schreiben
2. In das Tools-Verzeichnis speichern
3. Automatisch im Manifest registrieren

### Ein einfaches Beispiel

```
Du: Erstelle ein Tool namens "hello_tool", das eine Begrüßung 
    mit dem aktuellen Datum ausgibt.

Agent: 🛠️ Tool: skills (create_tool)
       ✅ Tool "hello_tool.py" erstellt
       ✅ Im Manifest registriert
       ✅ Sofort verfügbar

Du: Teste das Tool mit dem Namen "Max"
Agent: 🛠️ Tool: hello_tool
       👋 Hallo Max!
       Heute ist Samstag, 8. März 2025
```

### Tool-Struktur

Ein benutzerdefiniertes Tool ist eine Python-Datei mit folgender Struktur:

```python
# hello_tool.py
import json
from datetime import datetime

def main():
    # Input vom Agent (JSON)
    import sys
    input_data = json.loads(sys.stdin.read())
    
    name = input_data.get("name", "Welt")
    today = datetime.now().strftime("%A, %d. %B %Y")
    
    # Output als JSON
    result = {
        "status": "success",
        "greeting": f"Hallo {name}!",
        "date": today
    }
    
    print(json.dumps(result))

if __name__ == "__main__":
    main()
```

### Tool mit Parametern

```
Du: Erstelle ein Tool "password_generator" mit Parametern:
    - length (Standard: 12)
    - include_special (Standard: true)
    - count (wie viele Passwörter, Standard: 1)

Agent: ✅ Tool erstellt: password_generator.py
       Parameter:
       - length: integer, default=12
       - include_special: boolean, default=true  
       - count: integer, default=1

Du: Generiere 3 Passwörter mit 16 Zeichen ohne Sonderzeichen
Agent: 🛠️ Tool: password_generator
       🔑 Generierte Passwörter:
       1. k9mP2vL8nQ5rT4wX
       2. h7jF3bN6cV9dK2mP
       3. p4tY8gH2jL5kN7qR
```

> 💡 **Tipp:** Benutzerdefinierte Tools werden im Verzeichnis `agent_workspace/tools/` gespeichert und sind persistent.

## Praktische Tool-Beispiele

### Beispiel 1: Server-Monitoring

```
Du: Erstelle ein Monitoring-Skript, das:
1. Alle 5 Minuten Systemmetriken prüft
2. Bei CPU > 80% eine Warnung ausgibt
3. Bei Disk > 90% eine Email sendet

Agent: 🛠️ Erstelle Multi-Tool Workflow...

Schritt 1: Monitoring-Skript
✅ Python-Skript erstellt mit:
   - system_metrics Abfrage
   - Schwellenwert-Prüfung
   - Logging

Schritt 2: Cron-Job
✅ Eintrag: */5 * * * * monitor.py

Schritt 3: Email-Alert
✅ Email-Template konfiguriert
   Empfänger: admin@example.com

Testlauf:
🛠️ Tool: system_metrics
   Aktuell: CPU 23%, Disk 45%
   Status: 🟢 Alle Werte normal
```

### Beispiel 2: Automatisierte Backups

```
Du: Setze ein automatisiertes Backup-System auf:
1. Täglich um 2 Uhr: Backup ~/Documents
2. Komprimierung als tar.gz
3. Upload zu WebDAV
4. Löschung alter Backups (>30 Tage)
5. Status-Benachrichtigung

Agent: 🛠️ Konfiguriere Backup-Workflow...

✅ Shell-Skript: backup_daily.sh
✅ Cron: 0 2 * * * backup_daily.sh
✅ WebDAV-Integration: verbunden
✅ Cleanup-Logik: >30 Tage
✅ Notification: E-Mail konfiguriert

Erster Test-Backup:
🛠️ Tool: shell
   $ ./backup_daily.sh --test
   📦 Backup erstellt: backup_20250308.tar.gz (245 MB)
   ☁️  Upload: erfolgreich (12s)
   🧹 Cleanup: 3 alte Backups gelöscht
   ✉️  Benachrichtigung: gesendet
```

### Beispiel 3: Smart-Home Automation

```
Du: Erstelle eine Morgen-Routine:
1. Um 7:00: Wohnzimmer-Licht langsam einschalten
2. Kaffeemaschine einschalten (Steckdose)
3. Wetter-Information abrufen und ansagen
4. Wenn Wochenende: 30 Minuten später starten

Agent: 🛠️ Erstelle Smart-Home Routine...

✅ Home Assistant Automation:
   Trigger: Zeitplan (7:00 Wochentag, 7:30 Wochenende)
   
✅ Aktionen konfiguriert:
   - light.wohnzimmer: brightness 0→100% über 10min
   - switch.kaffeemaschine: turn_on
   - weather.home: Aktuelle Daten
   
✅ TTS-Integration:
   "Guten Morgen! Heute werden es 18 Grad bei leichtem Regen."

Nächster Lauf:
⏰ Morgen, 7:00 Uhr ( falls Werktag)
```

### Beispiel 4: Entwicklungs-Workflow

```
Du: Hilf mir bei einem typischen Dev-Workflow:
1. Neue Branch erstellen für Feature XYZ
2. Änderungen an den Dateien vornehmen
3. Commit mit sinnvoller Message
4. Push zu GitHub
5. PR erstellen mit Template

Agent: 🛠️ Entwicklungs-Workflow...

Schritt 1: Git Branch
🛠️ Tool: git (branch)
   $ git checkout -b feature/xyz
   ✅ Branch erstellt: feature/xyz

Schritt 2: Datei-Änderungen
🛠️ Tool: filesystem (write_file)
   ✅ src/feature.py aktualisiert

Schritt 3: Commit
🛠️ Tool: git (commit)
   $ git add .
   $ git commit -m "feat: add XYZ functionality
   
   - Implement core XYZ logic
   - Add error handling
   - Update tests"
   ✅ Commit erstellt: a1b2c3d

Schritt 4: Push
🛠️ Tool: git (push)
   $ git push origin feature/xyz
   ✅ Gepusht zu origin

Schritt 5: PR erstellen
🛠️ Tool: github (create_pr)
   ✅ Pull Request erstellt:
   #42: feat: add XYZ functionality
   URL: https://github.com/.../pull/42
```

## Tool-Statistik anzeigen

Nutze `/budget` für Nutzungsstatistiken:

```
Du: /budget
Agent: 💰 Tool-Nutzung (heute):
       
       ┌───────────────────┬────────┬──────────┐
       │ Tool              │ Aufrufe│ Tokens   │
       ├───────────────────┼────────┼──────────┤
       │ web_search        │ 12     │ 2,400    │
       │ filesystem        │ 8      │ 800      │
       │ shell             │ 5      │ 1,200    │
       │ docker            │ 3      │ 600      │
       │ system_metrics    │ 2      │ 200      │
       ├───────────────────┼────────┼──────────┤
       │ Gesamt            │ 30     │ 5,200    │
       └───────────────────┴────────┴──────────┘
       
       Geschätzte Kosten: $0.0032
       Tageslimit: $1.00 (0.3% erreicht)
```

## Zusammenfassung

| Kategorie | Anzahl | Danger Zone |
|-----------|--------|-------------|
| Dateisystem | 4 | Ja |
| Web/API | 6 | Nein |
| Docker | 8 | Ja |
| Proxmox | 7 | Ja |
| Smart Home | 4 | Ja |
| Google Workspace | 6 | Nein |
| Email | 4 | Nein |
| System | 5 | Teilweise |
| Memory | 6 | Nein |
| Sonstige | 10 | Teilweise |
| **Gesamt** | **60+** | - |

> 💡 **Merke:** AuraGo's Stärke liegt in der Kombination von Tools. Ein einzelner Chat kann Shell-Befehle, Web-Suchen, Docker-Operationen und Email-Benachrichtigungen verketten – vollautomatisch!

## Nächste Schritte

- **[Konfiguration](07-konfiguration.md)** – Tools feinjustieren und sichern
- **[Integrationen](08-integrationen.md)** – Externe Dienste verbinden
- **[Troubleshooting](09-troubleshooting.md)** – Probleme mit Tools lösen

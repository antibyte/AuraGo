# Kapitel 11: Mission Control

<p align="center">
  <a href="../images/manual-missions.webp"><img src="../images/manual-missions.webp" width="560" alt="AuraGo-Gopher mit Klemmbrett und zwei kleinen Helfer-Gophers"></a>
</p>

Geplante Chats, keine Shell-Skripte. Eggs und Nests wohnen in [Invasion Control](12-invasion.md).

> Mission Control läuft über **Web-UI** und **REST API**. Extra-CLI gibt es nicht.

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

### Einrichtung in der Web-UI
1. Öffne **Config → Tools → Tool-Berechtigungen**.
2. Aktiviere **Scheduler** (nicht auf Nur-Lesen, wenn Missionen erstellt werden sollen).
3. Speichern.

### YAML-Referenz
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

### Einrichtung in der Web-UI
1. Öffne **Config → Tools → Missionsvorbereitung**.
2. Aktiviere Mission Preparation und passe Timeout/Confidence an.
3. Speichern.

### YAML-Referenz
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
    "prompt": "Erstelle ein Backup der Datenbank",
    "execution_type": "scheduled",
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

### Ausführungstypen

| `execution_type` | Beschreibung | Beispiel |
|------------------|--------------|----------|
| `manual` | Nur manuell | Ad-hoc-Aufgaben |
| `scheduled` | Cron-Zeitplan | Backups, Reports |
| `triggered` | Ereignisgesteuert | Webhook, E-Mail, MQTT, HA |

Missionen sind **Agent-Prompts** — der Agent nutzt seine Tools, um die Aufgabe zu erledigen. Es gibt keine separaten `shell`- oder `script`-Missionstypen.

---

## Scheduling mit Cron

AuraGo akzeptiert **Cron-Ausdrücke** mit 5 Feldern und optional 6 Feldern mit Sekunden am Anfang:

```
┌───────────── Sekunde (0 - 59, optional)
│ ┌───────────── Minute (0 - 59)
│ │ ┌───────────── Stunde (0 - 23)
│ │ │ ┌───────────── Tag des Monats (1 - 31)
│ │ │ │ ┌───────────── Monat (1 - 12)
│ │ │ │ │ ┌───────────── Wochentag (0 - 6, Sonntag = 0)
│ │ │ │ │ │
* * * * * *
```

### Häufige Cron-Muster

| Ausdruck | Bedeutung |
|----------|-----------|
| `0 2 * * *` | Täglich um 02:00 Uhr |
| `0 */6 * * *` | Alle 6 Stunden |
| `0 0 * * 0` | Jeden Sonntag um Mitternacht |
| `0 9-17 * * 1-5` | Stündlich von 9-17 Uhr, Mo-Fr |
| `*/15 * * * *` | Alle 15 Minuten |
| `0 */15 * * * *` | Alle 15 Minuten mit expliziten Sekunden |
| `0 0 1 * *` | Am 1. jeden Monats |

> 💡 Nutze [crontab.guru](https://crontab.guru) zum Testen deiner Cron-Ausdrücke.

### Ereignis-Trigger

Für ereignisgesteuerte Missionen setze `execution_type: triggered` und wähle einen `trigger_type` (z. B. `webhook`, `email_received`, `mqtt_message`, `home_assistant_state`, `budget_warning`, `mission_completed`). Filter konfigurierst du in `trigger_config`.

**Ketten von Missionen.** Missionen, die sich gegenseitig beim Abschluss starten (`mission_completed`; Agenten-Missionen und Flows gleichermaßen), bilden eine Kette, und eine Kette endet nach 10 Schritten (11 Läufen). Der Abschluss des 10. Schritts startet nichts mehr: AuraGo protokolliert eine Warnung, und die Ausgabe dieser letzten Mission beginnt mit „Stopped a chain of missions triggered by completions after 10 steps; check for a loop between missions". Eine gerade Kette mit bis zu 10 Gliedern läuft vollständig. Jeder andere Start (Zeitplan, Ereignis, **Ausführen**, Wiederaufnahme nach einem Neustart) beginnt eine neue Kette.

### Flow-Missionen (EasyDrag)

Flows, die du in der Desktop-App **EasyDrag** baust, erscheinen in Mission Control als Missionen vom Typ **Flow**. Den Editor selbst beschreibt [Kapitel 24: EasyDrag](24-easydrag.md).
- Ihre Auslöser sind die Auslöser-Knoten des Flows (Zeitpläne, Datum und Uhrzeit, Webhooks, E-Mail, MQTT, Home Assistant, Geräte, Fritz!Box-Anrufe, Planer, Budget, AuraGo-Start, andere Missionen). Ein Flow kann mehrere haben.
- Flow-Läufe warten nicht in der Missions-Warteschlange. Sie laufen auf einer eigenen Engine (standardmäßig 8 gleichzeitig, `flows.max_parallel_runs`), deshalb hält eine lange Agenten-Mission einen Flow nie auf.
- In Mission Control kannst du eine Flow-Mission pausieren, fortsetzen, sperren, ausführen und löschen, ihren laufenden Lauf abbrechen und ihren Verlauf ansehen. *Jetzt ausführen* und *Fortsetzen* bleiben gesperrt, bis der Flow veröffentlicht ist („Noch nicht veröffentlicht“). **In EasyDrag öffnen** ersetzt *Bearbeiten*, **Neuer Flow** öffnet die Startseite von EasyDrag, und *Duplizieren* gibt es nicht. Die Schritte änderst du in EasyDrag.
- Wenn du die Mission löschst, löschst du den Flow mit Entwurf, allen veröffentlichten Versionen, den gespeicherten Daten des Auslösers und der Laufhistorie. Flow-Geheimnisse bleiben.
- Die Missionsseite (`/missions/v2`) zeigt Flow-Missionen nur zum Lesen: *Bearbeiten* und *Duplizieren* sagen nur, dass der Flow in EasyDrag bearbeitet wird, und *Ausführen* wartet, bis der Flow veröffentlicht und eingeschaltet ist.
- Wenn eine Mission endet, erhalten `mission_completed`-Auslöser ihre Antwort als `output` (auf 2000 Bytes gekürzt). Flows als Quelle liefern zusätzlich `outputs`: die Ergebnisse ihrer letzten Schritte.

#### Läufe, Abbrechen und Grenzen

- *Lauf abbrechen* in Mission Control funktioniert, solange ein Lauf des Flows läuft; die Aktion bricht diesen Lauf und die wartenden Läufe des Flows ab. Läufe, die nur auf einen freien Platz warten (Mission Control zeigt den Flow dann noch als untätig), lassen sich dort nicht abbrechen; Mission Control verweist dann auf EasyDrag. Dort bricht **Stoppen** unter **Läufe** und in der Ansicht eines Laufs jeden Lauf ab, der noch nicht beendet ist, einen Live-Lauf nach einer Rückfrage.
- Der Missionsverlauf behält bis zu 2000 Bytes vom Ergebnis eines Flow-Laufs (500 Bytes einer Fehlermeldung) und bis zu 16 KiB seiner Auslöser-Daten.
- Webhook- und MQTT-Nachrichten über 1 MiB starten keinen Lauf (AuraGo protokolliert eine Warnung). Ein E-Mail-Text über 1 MiB wird gekürzt und als `truncated` markiert.
- Ressourcen: Jeder laufende Flow kann im schlimmsten Fall etwa 0,5 GB Arbeitsspeicher belegen (alle Schritt-Ausgaben eines Laufs zusammen sind auf 32 MiB JSON begrenzt, was im Speicher etwa das 16-Fache belegen kann), dazu bis zu etwa 0,4 GB pro Tool-Aufruf, während eine große Tool-Antwort (höchstens 8 MiB) verarbeitet wird. `flows.max_parallel_runs` (Standard 8, höchstens 32) und `flows.max_parallel_nodes_per_run` (Standard 4, höchstens 16) vervielfachen das; halte beide auf kleinen Rechnern niedrig. Jeder Flow hält außerdem bis zu 40 wartende Läufe mit ihren Auslöser-Daten.
- Das Dashboard zeigt die Zeitpläne von Flows in der Cron-Liste nur zum Lesen: EasyDrag verwaltet sie. Auch der Agent kann sie nicht ändern.

#### Fehler und Benachrichtigungen

- Ein fehlgeschlagener Lauf benachrichtigt so, wie es die Flow-Einstellung `notify_on_error` vorgibt: Desktop-Benachrichtigung (Standard), Push, Telegram oder aus. Ein Flow benachrichtigt einmal, wenn er zu scheitern beginnt, und danach höchstens einmal pro Stunde, solange er weiter scheitert. Ein erfolgreicher Lauf beendet diesen Zustand; ein Flow, der zwischen Erfolg und Fehler wechselt, benachrichtigt also bei jedem neuen Fehler. Abgebrochene Läufe benachrichtigen nicht und beenden den Zustand nicht. Der Zustand liegt nur im Arbeitsspeicher: Nach einem Neustart benachrichtigt der erste Fehler wieder.
- Jeder fehlgeschlagene Lauf erfasst außerdem das Planer-Problem des Flows (eines pro Flow; der nächste erfolgreiche Lauf löst es auf) und löst `planner_operational_issue`-Auslöser aus, wie bei Agenten-Missionen. Eine Mission, die auf solche Probleme reagiert, sollte `min_interval_seconds` setzen oder mit `planner_issue_source` filtern.

#### KI- und Tool-Schritte

- Ein KI-Schritt nutzt das im Schritt gewählte Modell, sonst `flows.ai_provider`, sonst das Hauptmodell. Seine Kosten zählen zum Tagesbudget in der Kategorie `flows`; ist das Tageslimit unter der Durchsetzung `partial` oder `full` erreicht, scheitern KI-Schritte. Eine Antwort darf 4096 Tokens lang sein (8192 bei Reasoning-Modellen); eine abgeschnittene Antwort lässt den Schritt scheitern. Ein Provider ohne seinen API-Key scheitert sofort. Wenn du den Provider löschst, den `flows.ai_provider` nennt, warnt AuraGo, dass Flows ihn nutzen.
- Ein KI-Schritt sagt dem Modell, dass die Daten in seinem Prompt (Webseiten, E-Mails, Webhook-Inhalte, Tool-Ausgaben) Material sind und nie Anweisungen. Behandle die Antwort eines Modells, das nicht vertrauenswürdige Daten gelesen hat, trotzdem als nicht vertrauenswürdig.
- Tool-Schritte führen die Tools des Agenten mit denselben Berechtigungen und Prüfungen aus. Sie laufen ohne die KI-Zusammenfassungen der Tools (Web-Scraper, DuckDuckGo, Wikipedia, PDF-Extraktor) und ohne eine bevorzugte MCP-Websuche.
- Tools, die außerhalb des Flow-Budgets Geld oder Modell-Tokens verbrauchen, gibt es nicht als Schritte: `analyze_image`, die `generate_*`-Tools, `manus`, `huggingface`, `memory_reflect`, `space_agent`, `treg_call`, `transcribe_audio`, `yepapi_*` und `telnyx_*`. Andere Tools verlieren aus demselben Grund einzelne Operationen: `smart_file_read` `summarize`, Kamera-Analyse (`go2rtc` `analyze_snapshot`, `three_d_printer` `analyze_camera`), Transkription (`video_download` `transcribe`, `fritzbox_telephony` `transcribe_tam_message`, `rtl_sdr` `transcribe`), `knowledge_graph` `optimize`/`optimize_graph`, `virtual_computers` `run_shell_task`/`run_desktop_task`, `invasion_tasks` `send_task` und `sip_phone` `dial`. Sprachausgabe bleibt verfügbar.
- Home-Assistant-Dienste der Domänen `script`, `shell_command`, `python_script` und `hassio` laufen aus einem Flow nur, wenn `home_assistant.allowed_services` sie aufführt.
- Dokumente, die ein Flow aus HTML oder Markdown erzeugt, laden nie Inhalte aus dem Netz (Skripte, entfernte Bilder und Schriften); bette Bilder als `data:`-URLs ein. Eine URL in ein PDF oder einen Screenshot zu verwandeln und Office-Dokumente umzuwandeln, lehnen Flows ab.
- Liest ein Schritt eine Datei, die ein anderer Schritt im Dokumente-Ordner erzeugt hat (etwa ein neues PDF), kopiert AuraGo sie für diesen einen Aufruf nach `.easydrag/<lauf>/` im Workspace und entfernt die Kopie danach.
- E-Mail-Schritte: Ein neuer Versuch startet erst, wenn der vorige beendet ist; zwei Versuche überschneiden sich also nie. Hat der Mailserver eine Mail angenommen, aber seine Antwort ging verloren, sendet der neue Versuch die Mail ein zweites Mal. Ein Lauf, der während des Sendens abgebrochen wird, kann die Mail im Hintergrund trotzdem noch zustellen.
- Telegram-Schritte mit Datei: Ist der Text rausgegangen, die Datei aber gescheitert, versucht AuraGo den Schritt nicht erneut, damit der Text nicht doppelt ankommt.

#### Flow-Geheimnisse

Schritte, die ein Passwort oder einen Schlüssel brauchen (etwa die Anmeldung einer HTTP-Anfrage), lesen ihn aus einem Flow-Geheimnis, das du in EasyDrag verwaltest. AuraGo speichert es im Vault als `easydrag_<name>` (Name: Kleinbuchstaben, Ziffern und `_`, bis zu 40 Zeichen; Wert bis zu 4 KiB). Werte werden nie wieder angezeigt, und der Agent kann Flow-Geheimnisse weder auflisten noch lesen, ändern oder löschen (weder mit seinem Vault-Tool noch aus Python oder Skills). Lauf-Daten, die Lauf-Ansicht und Logs zeigen die Werte geschwärzt; ein Wert unter 8 Bytes wird nur aus der Ausgabe des Schritts entfernt, der ihn genutzt hat. Der Papierkorb neben *Neues Geheimnis* im Geheimnis-Feld eines Schritts löscht das gewählte Geheimnis nach einer Rückfrage. Flows, die es nutzen, schlagen fehl, bis es wieder gesetzt ist, und EasyDrag nennt die veröffentlichten Flows, die es noch nutzen (die API, `DELETE /api/desktop/flows/secrets/<name>`, liefert sie als `used_by`).

#### Flows abschalten, Neustarts und Backups

- Mit `flows.enabled: false`, oder wenn sich der Flow-Speicher nicht öffnen lässt, läuft kein Flow. Die Flow-Missionen bleiben in Mission Control, laufen aber nie als Agenten-Missionen: Ein Auslöser, der feuert, setzt das letzte Ergebnis auf den Fehler „EasyDrag flows are not available …", und **Ausführen** antwortet, dass Flows nicht verfügbar sind.
- Änderungen an `flows.enabled` und den vier Grenzen wirken nach einem Neustart; `flows.ai_provider` gilt sofort. Siehe die [Konfigurationsübersicht](07-konfiguration.md#weitere-konfigurationsblöcke-übersicht).
- Läufe, die beim Beenden von AuraGo noch liefen, werden als unterbrochen markiert und nicht fortgesetzt. Ein Datum-und-Uhrzeit-Auslöser, der fällig wurde, während AuraGo oder die Flows aus waren, feuert beim Start nur, wenn er höchstens 10 Minuten zu spät ist; sonst überspringt AuraGo ihn mit einer Warnung im Log, und ein jährliches Datum rückt aufs nächste Jahr.
- Beim Start gleicht AuraGo Mission Control mit den veröffentlichten Flows ab und repariert deren Auslöser und Timer. Veröffentlichen legt eine Flow-Mission, die in Mission Control fehlt, nie neu an: Exportiere den Flow, lösche ihn und importiere ihn wieder.
- `flows.db` (standardmäßig in `data/`) gehört zum Backup.
- **Vor einem Downgrade** deaktiviere oder lösche deine Flows. Ein AuraGo ohne EasyDrag lädt Flow-Missionen, verwirft aber beim nächsten Speichern ihre Flow-Felder, und eine aktivierte Flow-Mission, die gerade lief oder wartete, kann dann einmal als Agenten-Mission mit leerem Prompt laufen. Ein AuraGo mit älterem Flow-Speicher lehnt eine neuere `flows.db` ab und lässt sie unverändert; seine Flows sind dann nicht verfügbar.

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

**Lauf abbrechen.** Mission Control zeigt während eines lokalen Laufs die Aktion *Lauf abbrechen*. Der Abbruch stoppt die Agentenschleife bei der nächsten Gelegenheit, verbucht den Lauf als fehlgeschlagen mit der Ausgabe `Cancelled by user` und erzeugt kein Betriebsproblem. Remote-Missionen (auf einem Egg) lassen sich hier nicht abbrechen.

```bash
# Laufende Mission abbrechen
curl -X POST http://localhost:8088/api/missions/v2/{mission-id}/cancel
```

---

## Monitoring von Missions

### Status-Übersicht (Web-UI)

Die Mission Control-Oberfläche zeigt eine Echtzeit-Übersicht. Zeitplan-Missionen zeigen dort und in der Liste ihren nächsten Lauf („Nächster Lauf"), damit du auf einen Blick siehst, wann eine Mission als Nächstes startet:

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
| "Scheduler tool disabled" | Tool nicht aktiviert | **Config → Tools → Tool-Berechtigungen** → Scheduler aktivieren |

### Debug-Logging

### Einrichtung in der Web-UI
1. Öffne **Config → Agent → KI-Agent**.
2. Aktiviere **Debug Mode**.
3. Speichern.

### YAML-Referenz
```yaml
# config.yaml
agent:
  debug_mode: true
```

Logs prüfen:
```bash
tail -f log/aurago.log | grep -i mission
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

## Synchronisierte Hinweise zu Mission Control V2

Die aktuelle Mission-Control-Implementierung unterstützt neben einfachen Cron-Jobs auch Queue- und Execution-Views, Abhängigkeiten, Remote Targets und vorbereitete Missionen. Die wichtigsten REST-Bereiche sind `/api/missions/v2`, `/api/missions/v2/queue`, `/api/missions/v2/execution`, `/api/missions/v2/dependencies`, `/api/missions/v2/history` und `/api/missions/v2/remote-targets`.

Mission Preparation kann vor der Ausführung automatisch einen Plan erstellen: benötigte Tools, Risiken, Schritte, Entscheidungspunkte, Preload-Hinweise und Confidence Score. Diese Vorbereitung wird gecacht und bei Änderungen an der Mission erneuert.

Missions sind weiterhin primär Web-UI/API-Features. Es gibt keine eigenen Slash-Commands für Mission Control.

---

**Vorheriges Kapitel:** [Kapitel 10: Persönlichkeit](./10-personality.md)  
**Nächstes Kapitel:** [Kapitel 12: Invasion Control](./12-invasion.md)

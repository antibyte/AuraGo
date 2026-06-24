# Kapitel 13: Dashboard

Das AuraGo-Dashboard ist dein operatives Kontrollzentrum für den Agenten. Es zeigt Systemzustand, Gedächtnis, Persönlichkeit, Missionen, Knowledge-Graph-Qualität, Audit-Protokolle und mehr — organisiert in Tabs, die bei Bedarf nachgeladen werden.

Erreichbar unter `http://localhost:8088/dashboard` oder über das Radialmenü (☰ → 📊 Dashboard). Bei aktivierter Web-UI-Authentifizierung ist ein Login erforderlich.

## Aufbau

Über dem Tab-Inhalt gibt es zwei dauerhaft sichtbare Elemente:

1. **Agent-Status-Banner** — immer sichtbar: aktives Modell, Persönlichkeitsprofil, Kontext-Auslastung, verbundene Integrationen und letzte Aktivität.
2. **Tab-Navigation** — acht Tabs; beim ersten Öffnen wird nur der aktive Tab geladen.

```
┌─────────────────────────────────────────────────────────────────────┐
│ 📊 AURAGO DASHBOARD                                    🌙         ≡ │
├─────────────────────────────────────────────────────────────────────┤
│ 🤖 Agent-Status: Modell · Persönlichkeit · Kontext · Integrationen │
├─────────────────────────────────────────────────────────────────────┤
│ Übersicht │ Agent │ Benutzer │ Knowledge Graph │ Datei-Sync │ …    │
├─────────────────────────────────────────────────────────────────────┤
│  [Karten des aktiven Tabs — einklappbar, Raster-Layout]            │
└─────────────────────────────────────────────────────────────────────┘
```

**Persistenz:**
- Der zuletzt gewählte Tab wird in der URL-Hash (`#overview`, `#agent`, …) und in `localStorage` gespeichert.
- Einzelne Karten lassen sich einklappen; der Zustand wird pro Karte in `localStorage` gemerkt.

Es gibt **keinen** Dashboard-weiten Widget-Editor, Export-Menü oder YAML-basierte Dashboard-Konfiguration in der aktuellen UI.

---

## Tab: Übersicht

Der Tab **Übersicht** ist die kompakte Gesundheitsansicht.

| Karte | Inhalt | API |
|-------|--------|-----|
| **Systemstatus** | CPU-, RAM- und Festplatten-Gauges; Netzwerk ↑/↓; SSE-Clients; Betriebszeit | `GET /api/dashboard/system` |
| **Quick Status** | Missionen, Integrationen, Tunnel, Planner, Skills und weitere Snapshot-Badges | `GET /api/dashboard/overview` |
| **Budget & Tokens** | Tagesausgaben vs. Limit, Modell-Aufschlüsselung, OpenRouter-Guthaben, Ø Token/Anfrage | `GET /api/budget`, `GET /api/credits` |
| **Optimization** | Aktive Prompt-Overrides, laufende Shadow-Tests, abgelehnte Mutationen, Trace-Events, Erfolgsrate | `GET /api/dashboard/optimization` |
| **Output Compression** | Kompressionsanzahl, gesparte Zeichen, Top-Tools und -Filter (wenn aktiv) | `GET /api/dashboard/compression` |
| **Mission History** | Letzte Mission-Läufe mit Status, Trigger, Startzeit und Dauer | `GET /api/dashboard/mission-history` |

Die Budget-Karte erscheint nur, wenn Budget-Tracking in der Config aktiv ist. Sonst wird ein Hinweis „deaktiviert“ angezeigt.

Die Mission History unterstützt Pagination über **Mehr laden** (`limit` + `offset` als Query-Parameter).

---

## Tab: Agent

Der Tab **Agent** fokussiert Persönlichkeit und Gedächtnis.

### Persönlichkeit

- **Trait-Radar** für die sieben Persönlichkeitsmerkmale der Personality Engine.
- **Aktuelle Stimmung** (Mood-Badge) und Auslöser-Text.
- **Emotionaler Zustand** (mit aktivem Emotion Synthesizer): Beschreibung, Ursache, Antwortstil und Emotionsverlauf.
- **Stimmungsverlauf** mit wählbaren Zeiträumen: 1 h, 6 h, 24 h, 7 d, 30 d.

| API | Zweck |
|-----|-------|
| `GET /api/personality/state` | Traits, Mood, Aktivierungsstatus |
| `GET /api/dashboard/mood-history?hours=N` | Mood-Verlauf für das Diagramm |
| `GET /api/dashboard/emotion-history?hours=N` | Emotions-Historie |

Ist die Personality Engine deaktiviert, zeigt die Karte einen leeren Zustand.

### Gedächtnis

- Zähler für Core Memory, Chat-Nachrichten, Vektordatenbank und Knowledge-Graph-Knoten/Kanten.
- **Memory Health**-Zusammenfassung: Retrieval-Statistiken, Confidence, veraltete Kandidaten, Konflikte, Strategie-Modus.
- **Memory Health / Curation** mit Dry-Run-Vorschau und Admin-only-Übernahme.
- **Recent Episodes** und Meilenstein-Liste.
- **Fehler-Muster** aus wiederholten Fehlern.

Ein Klick auf **Core Memory** öffnet ein Modal zum Anzeigen und Löschen einzelner Kernfakten (Massenlöschung nur mit Bestätigung `DELETE_ALL_CORE_MEMORY`).

| API | Zweck |
|-----|-------|
| `GET /api/dashboard/memory` | Speicher-Statistiken und Health |
| `GET /api/dashboard/core-memory` | Kernfakten-Liste |
| `DELETE /api/dashboard/core-memory/mutate` | Kernfakten löschen |
| `GET /api/dashboard/memory/curation` | Curation-Status |
| `POST /api/dashboard/memory/curation/dry-run` | Sichere Bereinigung vorschauen |
| `POST /api/dashboard/memory/curation/apply` | Curation anwenden (Admin) |
| `GET /api/dashboard/errors` | Fehler-Muster |

---

## Tab: Benutzer

Der Tab **Benutzer** zeigt Profilierung und Journal.

| Karte | Inhalt | API |
|-------|--------|-----|
| **Nutzerprofil** | Durchsuchbare Profil-Einträge nach Kategorie | `GET /api/dashboard/profile` |
| **Last 7 Days** | Aktivitätsübersicht mit Highlights und offenen Punkten | `GET /api/memory/activity-overview?days=7` |
| **Journal** | Timeline, Stimmungs-Verlauf und Schwerpunktthemen | `GET /api/dashboard/journal`, `GET /api/dashboard/journal/summaries` |

Profil-Einträge können über `PUT /api/dashboard/profile/entry` aktualisiert werden.

---

## Tab: Knowledge Graph

Der Tab **Knowledge Graph** ist die vollständige KG-Verwaltungsansicht.

| Bereich | Funktion |
|---------|----------|
| **Zusammenfassung** | Knoten-/Kanten-Zahlen, Typ-Verteilung, Suchfeld | `GET /api/knowledge-graph/stats`, `GET /api/knowledge-graph/search` |
| **Graph Quality** | Geschützte, isolierte, untypisierte und Duplikat-Kandidaten | `GET /api/knowledge-graph/quality` |
| **KG Explorer** | Suchergebnisse, letzte Knoten und Kanten | `GET /api/knowledge-graph/nodes`, `GET /api/knowledge-graph/edges`, `GET /api/knowledge-graph/important` |
| **Graph View** | Interaktive Übersichts-/Fokus-Grafik (force-graph) | Aus geladenen Knoten und Kanten |
| **Node Inspector** | Eigenschaften, Nachbarn, Schutz/Bearbeitung | `GET /api/knowledge-graph/node`, `POST /api/knowledge-graph/node/protect` |

Klicke einen Knoten in Liste oder Grafik, um den Node Inspector zu öffnen.

---

## Tab: Datei-Sync

Der Tab **Datei-Sync** zeigt Indexer- und Knowledge-Graph-Synchronisationsstatus.

Vier Spalten:

- **File Indexer** — Laufstatus und Indexer-Statistiken
- **Knowledge Graph** — KG-Sync-Statistiken
- **Collections** — Übersicht indexierter Collections
- **Last Synchronization** — Zeitstempel des letzten Laufs

**Refresh** lädt den Status neu, **Rescan** startet einen manuellen Indexer-Rescan (`POST /api/indexing/rescan`).

Status-Daten kommen von:

```bash
GET /api/debug/file-sync-status
GET /api/debug/kg-file-sync-stats
GET /api/debug/file-sync-last-run
```

---

## Tab: Audit

Der Tab **Audit** ist die zentrale Aktivitätsspur für Aktionen, die AuraGo ausführt. Er erfasst Agent-Tool-Aufrufe, Mission-Läufe, Heartbeat-Wake-ups und Remote-Geräteereignisse (Verbindungen, Trennungen, Heartbeats, Remote-Befehlsergebnisse).

Jeder Audit-Eintrag enthält Zeit, Quelle, Ereignistyp, Ziel, Status, Zusammenfassung, Dauer und bereinigte Detaildaten. Sensible Werte werden vor dem Speichern entfernt; lange Details werden gekürzt.

### Filter und Suche

Über das Suchfeld findest du Einträge nach Zusammenfassung, Ziel, Akteur, Ereignistyp oder bereinigten Details. Quellen-, Status-, Typ- und Zeitraumfilter lassen sich kombinieren; Pagination hält große Protokolle bedienbar.

### Audit-Einträge löschen

Admins können einzelne Audit-Zeilen über die Zeilenaktion löschen oder alle Einträge entfernen, die den aktuellen Filtern entsprechen. Massenlöschung verlangt serverseitig die Bestätigung `DELETE_AUDIT_EVENTS`.

### API-Zugriff

```bash
GET /api/dashboard/audit?limit=25&offset=0&q=mission&source=mission&status=success
DELETE /api/dashboard/audit/{id}
DELETE /api/dashboard/audit
```

DELETE-Endpunkte benötigen Admin-Zugriff.

---

## Tab: Cronjobs

Der Tab **Cronjobs** zeigt interne geplante Aufgaben aus dem eingebauten CronManager — inklusive Scheduler-Tool-Jobs und zeitgesteuerter Missionen im gleichen Scheduler.

Die Tabelle zeigt Job-ID, Quelle, Cron-Ausdruck, nächsten Lauf, Status (`enabled`, `disabled` oder `error`), Prompt und Zeilenaktionen. Fehlerzeilen zeigen `last_error` im Status-Hinweis, damit ungültige gespeicherte Jobs und deaktivierte Scheduler-Laufzeit sichtbar sind.

Admins können Cron-Ausdruck, Prompt und Aktivierung über die Zeilenaktion bearbeiten. Die Quelle wird im Dialog angezeigt, aber automatisch beibehalten.

```bash
GET /api/dashboard/cronjobs?q=backup&source=agent&status=enabled
GET /api/dashboard/cronjobs?status=error
PUT /api/dashboard/cronjobs
DELETE /api/dashboard/cronjobs/{id}
```

PUT und DELETE benötigen Admin-Zugriff.

---

## Tab: System

Der Tab **System** bündelt Betrieb, Diagnose und Logs.

| Karte | Inhalt | API |
|-------|--------|-----|
| **Betrieb & Dienste** | Missionen, Invasion-Nester, File Indexer, MQTT, Notizen, Vault, Geräte, Kontext-Summary, Cheatsheets, Tunnel | `GET /api/dashboard/overview` |
| **LLM Guardian** | Guardian-Status und Metriken (nur wenn aktiv) | `GET /api/dashboard/guardian` |
| **Daemon Skills** | Laufende Daemon-Prozesse (wenn vorhanden) | `GET /api/daemons` |
| **Helper LLM** | Helper-Modell-Status, Metriken, letzte Operationen | `GET /api/dashboard/helper-llm` |
| **Aktivität** | Tool-Nutzung, Automationen, Cron-Zusammenfassung | `GET /api/dashboard/activity` |
| **Prompt-Analyse** | Build-Zahlen, Token-Durchschnitte, Tier-Verteilung, Budget-Shed-Bereiche, Einsparungen | `GET /api/dashboard/prompt-stats` |
| **Adaptive Tool-Filterung** | Filter-KPIs (bei vorhandener Telemetrie) | `GET /api/dashboard/tool-stats` |
| **Tooling-Diagnose** | Parse-Quellen, Recovery-Events, Policy-Signale, fehleranfällige Tools | `GET /api/dashboard/tool-stats` |
| **GitHub Repositories** | Verknüpfte Repos (bei GitHub-Integration) | `GET /api/dashboard/github-repos` |
| **Live-Log** | Server-Log-Tail mit Regex-Filter | `GET /api/dashboard/logs?lines=100` |

Prompt-Statistiken werden beim Serverneustart zurückgesetzt und erscheinen nach dem ersten Gespräch.

---

## Mood-Zustände

Die Personality Engine nutzt **benannte Mood-Zustände**, keine einfache Happy/Sad-Emoji-Skala. Gültige Moods:

| Mood | Typischer Charakter |
|------|---------------------|
| **curious** | Standard; explorativ, offen für neue Ansätze |
| **focused** | Aufgabenorientiert, präzise |
| **creative** | Fantasievoll, ungewöhnliche Vorschläge |
| **analytical** | Strukturiert, detailgetrieben |
| **cautious** | Vorsichtig, risikobewusst |
| **playful** | Locker, informeller Ton |
| **frustrated** | Direkt; oft nach wiederholten Fehlern |
| **concerned** | Aufmerksam bei Problemen oder Risiken |
| **relaxed** | Ruhig, entspannte Interaktion |

Mood-Wechsel werden mit Auslöser-Text protokolliert und im Stimmungsverlauf angezeigt. Mit aktivem Emotion Synthesizer kommt eine reichere emotionale Beschreibung und Timeline hinzu.

> 🔍 **Vertiefung:** Siehe [Kapitel 10: Persönlichkeit](10-personality.md) für Engine-Konfiguration und Trait-Verhalten.

---

## Echtzeit-Updates (SSE)

Das Dashboard nutzt **Server-Sent Events (SSE)** über die gemeinsame `AuraSSE`-Verbindung — **kein** separates Dashboard-WebSocket.

Beim Verbinden registriert das Dashboard Handler für:

| SSE-Event | Dashboard-Effekt |
|-----------|------------------|
| `system_metrics` | Aktualisiert CPU/RAM/Disk-Gauges und System-Stats (ca. alle 10 Sekunden) |
| `memory_update` | Aktualisiert Memory-Balkendiagramm und Zähler |
| `personality_update` | Aktualisiert Mood-Badge; Emotions-Timeline auf Agent-Tab |
| `daemon_update` | Lädt Daemon-Karte auf System-Tab neu |
| `audit_update` | Plant Audit-Tabellen-Refresh, wenn Audit-Tab aktiv |
| `budget_update` / `budget_warning` / `budget_blocked` | Aktualisiert Ausgaben-Anzeige; Toast-Warnungen |

Fällt die SSE-Verbindung länger als wenige Sekunden aus, erscheint oben ein Reconnect-Banner.

### Manuelles Aktualisieren

Es gibt **keinen** globalen **Refresh**-Button für das gesamte Dashboard. Stattdessen pro Bereich:

- **Audit**-Tab → **Aktualisieren**
- **Cronjobs**-Tab → **Aktualisieren**
- **Datei-Sync**-Tab → **Refresh** / **Rescan**
- **Live-Log**-Karte → **Aktualisieren**

Ein Browser-Reload lädt die Daten des aktiven Tabs neu. Es gibt **keinen** Chat-Befehl `/dashboard refresh`.

### Was die UI nicht bietet

Das Dashboard bietet **nicht**:

- CSV-, PDF- oder PNG-Export von Metriken
- einen generischen Endpunkt `/api/metrics`
- Browser-Push-Benachrichtigungs-Konfiguration
- YAML-gesteuertes Widget-Layout oder Retention-Einstellungen

Für programmatischen Zugriff nutze die Dashboard-REST-Endpunkte unten und [Kapitel 21: REST-API-Referenz](21-api-reference.md).

---

## Dashboard-API-Übersicht

| Endpunkt | Beschreibung |
|----------|--------------|
| `GET /api/dashboard/overview` | Agent-Banner und Quick-Status |
| `GET /api/dashboard/system` | CPU, RAM, Disk, Netzwerk, Uptime |
| `GET /api/budget` | Budget-Ausgaben und Limits |
| `GET /api/credits` | OpenRouter-Guthaben |
| `GET /api/dashboard/optimization` | Prompt-Optimierungs-Statistiken |
| `GET /api/dashboard/compression` | Output-Compression-Statistiken |
| `GET /api/dashboard/mission-history` | Mission-Lauf-Historie |
| `GET /api/personality/state` | Persönlichkeit und Mood |
| `GET /api/dashboard/mood-history` | Mood-Verlauf |
| `GET /api/dashboard/emotion-history` | Emotions-Historie |
| `GET /api/dashboard/memory` | Speicher-Statistiken und Health |
| `GET /api/dashboard/memory/curation*` | Memory-Curation Vorschau und Apply |
| `GET /api/dashboard/core-memory` | Kernfakten |
| `GET /api/dashboard/profile` | Nutzerprofil-Einträge |
| `GET /api/memory/activity-overview` | Rollierende Aktivitätsübersicht |
| `GET /api/dashboard/journal*` | Journal-Einträge und Summaries |
| `GET /api/dashboard/notes` | Offene/erledigte Notizen |
| `GET /api/knowledge-graph/*` | Knowledge Graph durchsuchen und Qualität |
| `GET /api/dashboard/audit` | Audit-Protokoll (Admin-Löschung) |
| `GET /api/dashboard/cronjobs` | Cronjob-Liste (Admin bearbeiten/löschen) |
| `GET /api/dashboard/guardian` | LLM-Guardian-Status |
| `GET /api/dashboard/helper-llm` | Helper-LLM-Metriken |
| `GET /api/dashboard/errors` | Fehler-Muster |
| `GET /api/dashboard/activity` | Aktivitäts- und Automations-Stats |
| `GET /api/dashboard/prompt-stats` | Prompt-Builder-Analyse |
| `GET /api/dashboard/tool-stats` | Tooling-Telemetrie |
| `GET /api/dashboard/github-repos` | GitHub-Repository-Liste |
| `GET /api/dashboard/logs` | Server-Log-Tail |

---

## Tipps

- **Tab direkt verlinken:** `http://localhost:8088/dashboard#agent` öffnet direkt den Agent-Tab.
- **Karten einklappen:** Mit ▼ am Kartenkopf selten genutzte Bereiche ausblenden.
- **Theme:** 🌙 in der Kopfzeile — gilt für alle Web-UI-Seiten.
- **Budget-Hinweise:** Bei Annäherung an Tageslimits kommen SSE-Toasts im Dashboard — Limits in [Kapitel 7: Konfiguration](07-configuration.md) setzen, nicht per Dashboard-YAML.

---

## Fehlerbehebung

| Problem | Wahrscheinliche Ursache | Lösung |
|---------|-------------------------|--------|
| Karten zeigen „—“ oder leer | Feature in Config deaktiviert oder noch keine Daten | Config prüfen; Chat starten für Prompt-Stats |
| Gauges aktualisieren nicht | SSE getrennt | Auf Reconnect-Banner warten oder Seite neu laden |
| Budget-Karte fehlt | Budget-Tracking aus | Budget in Config aktivieren |
| Guardian-Karte fehlt | LLM Guardian deaktiviert | Erwartet — Karte ist bei `enabled: false` ausgeblendet |
| Audit/Cron-Bearbeitung scheitert | Keine Admin-Session | Als Administrator anmelden |
| Datei-Sync ohne Daten | Indexer läuft nicht oder nie synchronisiert | Rescan auslösen; Indexing-Config prüfen |

---

## Nächste Schritte

- **[Kapitel 10: Persönlichkeit](10-personality.md)** — Mood-Engine und Traits konfigurieren
- **[Kapitel 9: Gedächtnis](09-memory.md)** — Speicherschichten auf dem Agent-Tab verstehen
- **[Kapitel 11: Mission Control](11-missions.md)** — Mission History und Cron-Zeitpläne
- **[Kapitel 12: Invasion Control](12-invasion.md)** — Remote-Nodes in den Betriebs-Stats
- **[Kapitel 14: Sicherheit](14-security.md)** — Audit-Protokoll und LLM Guardian
- **[Kapitel 21: REST-API-Referenz](21-api-reference.md)** — Vollständige Endpunkt-Details

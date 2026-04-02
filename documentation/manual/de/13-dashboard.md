# Kapitel 13: Dashboard

Das Dashboard ist dein zentrales Informationszentrum für alle AuraGo-Metriken. Hier behältst du Systemzustand, Kosten, Nutzung und Performance im Blick.

## Übersicht

```
┌─────────────────────────────────────────────────────────────┐
│  ⚡ AURAGO  DASHBOARD                        🌙         ≡   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │  CPU    │ │   RAM   │ │  Disk   │ │ Uptime  │           │
│  │   23%   │ │   4.2GB │ │  45%    │ │ 5d 3h   │           │
│  │  ▁▂▄▆█  │ │  ▃▅▇██  │ │  ▂▄▆▇█  │ │         │           │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘           │
│                                                             │
│  ┌─────────────────────────────┐ ┌───────────────────────┐ │
│  │     Mood History            │ │   Token Usage         │ │
│  │  😊 ═══════════════════════ │ │  ████████████ 12.4K   │ │
│  │  😐 ════════                │ │  ████████      8.2K   │ │
│  │  😠 ══                      │ │  ████          4.1K   │ │
│  │                             │ │  ─────────────────────│ │
│  │  [Letzte 7 Tage]            │ │  GPT-4  Claude  Local │ │
│  └─────────────────────────────┘ └───────────────────────┘ │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Budget Tracking        [=====>        ] $12.40/$50 │   │
│  │  Heute: $1.23  |  Diese Woche: $8.90  |  Monat...   │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## System-Metriken

### CPU-Auslastung

Die CPU-Anzeige zeigt die aktuelle Prozessorauslastung in Echtzeit.

| Wert | Bedeutung | Aktion |
|------|-----------|--------|
| 0-50% | Normal | Keine |
| 50-80% | Erhöht | Monitoring |
| 80-95% | Hoch | Ursache prüfen |
| 95-100% | Kritisch | Eingreifen |

**Komponenten:**

```
┌─────────────────────────────────────────┐
│ CPU Usage: 34%                          │
│                                         │
│ ┌─────────────────────────────────────┐ │
│ │████████████████░░░░░░░░░░░░░░░░░░░░│ │  (Echtzeit)
│ └─────────────────────────────────────┘ │
│                                         │
│ User:  25% ████████████████████░░░░░░  │
│ System: 7% ██████░░░░░░░░░░░░░░░░░░░░  │
│ IO Wait: 2% ██░░░░░░░░░░░░░░░░░░░░░░░  │
│                                         │
│ Kerne: 8 | Load: 2.4 | Temp: 45°C      │
└─────────────────────────────────────────┘
```

> 💡 Hohe "IO Wait" Werte deuten auf langsame Festplatten oder Netzwerk-Probleme hin.

### RAM-Verbrauch

Übersicht über den Arbeitsspeicher-Verbrauch.

```
┌─────────────────────────────────────────┐
│ Memory: 4.2 GB / 16 GB (26%)            │
│                                         │
│ ████████████████████░░░░░░░░░░░░░░░░░░  │
│                                         │
│ Used:      4.2 GB  ┌─────────────┐     │
│ Cached:    2.1 GB  │ App:  1.5GB │     │
│ Buffers:   0.5 GB  │ LLM:  1.8GB │     │
│ Free:      9.2 GB  │ Sys:  0.9GB │     │
│                    └─────────────┘     │
└─────────────────────────────────────────┘
```

**Hinweise zur Interpretation:**

| Situation | Bedeutung |
|-----------|-----------|
| Hoher "Used" + niedriger "Free" | Normal bei Linux (Caching) |
| Hoher "App" | AuraGo-Prozesse |
| Hoher "LLM" | Geladene KI-Modelle |
| Steigender Verbrauch | Memory Leak prüfen |

### Disk-Nutzung

Anzeige des belegten Speicherplatzes.

```
┌─────────────────────────────────────────┐
│ Disk: 45% (450 GB / 1 TB)               │
│                                         │
│ Gesamt: ████████████████████░░░░░░░░░░  │
│                                         │
│ /                     45%  ████░░░░░░  │
│ /data                 62%  ██████░░░░  │
│ /backups              23%  ██░░░░░░░░  │
│                                         │
│ [Größte Verzeichnisse anzeigen]        │
└─────────────────────────────────────────┘
```

> ⚠️ **Warnung:** Bei über 85% Auslastung werden automatische Bereinigungen empfohlen.

### Laufzeit (Uptime)

Zeigt, wie lange AuraGo ununterbrochen läuft.

| Anzeige | Bedeutung |
|---------|-----------|
| `5d 3h 24m` | Seit 5 Tagen ohne Neustart |
| `🔴 Restarted` | Kürzlich neu gestartet |
| `⚡ Just started` | Frisch gestartet |

## Mood History Visualisierung

### Emotions-Timeline

Die Mood-History zeigt die emotionale Entwicklung des Agents über Zeit.

```
┌─────────────────────────────────────────────────────────────┐
│ Mood History – Letzte 7 Tage                               │
│                                                             │
│  +1.0 ┤        ★ Peak: +0.8                                │
│  +0.5 ┤    ▲ ▲      ▲                                     │
│   0.0 ┼────┼─┼──────┼────┬────┬────┬────┬────┬────       │
│  -0.5 ┤    ▼ ▼      ▼    ▼    │    ▼    │    ▼           │
│  -1.0 ┤                       ★ Low: -0.6                  │
│                                                             │
│       Mo   Di   Mi   Do   Fr   Sa   So                      │
│                                                             │
│  Ø Durchschnitt: +0.2  |  Trend: ↗ Steigend               │
└─────────────────────────────────────────────────────────────┘
```

### Mood-Levels

| Bereich | Farbe | Bedeutung |
|---------|-------|-----------|
| +0.7 bis +1.0 | 🟢 | Sehr positiv, enthusiastisch |
| +0.3 bis +0.7 | 🟩 | Positiv, kooperativ |
| -0.3 bis +0.3 | ⬜ | Neutral, ausgewogen |
| -0.7 bis -0.3 | 🟧 | Negativ, gestresst |
| -1.0 bis -0.7 | 🔴 | Sehr negativ, frustrasiert |

### Einflussfaktoren

Das Dashboard zeigt Faktoren, die den Mood beeinflussen:

```
┌─────────────────────────────────────────┐
│ Mood-Faktoren (heute)                  │
│                                         │
│ Erfolgreiche Tasks    +0.3  ██████░░░░ │
│ Positive Interaktionen +0.2  ████░░░░░░ │
│ System-Auslastung     -0.1  ██░░░░░░░░ │
│ Fehlgeschlagene Tasks -0.2  ████░░░░░░ │
│─────────────────────────────────────────│
│ Gesamt                +0.2  ▲          │
└─────────────────────────────────────────┘
```

> 💡 Ein stabiler positiver Mood führt zu besseren Antworten und mehr Kreativität.

## Prompt Builder Analytics

### Token-Verbrauch

Übersicht über die Nutzung von Tokens pro Interaktion.

```
┌─────────────────────────────────────────────────────────────┐
│ Prompt Builder Analytics – Letzte 24 Stunden               │
│                                                             │
│  Input Tokens                                               │
│  ████████████████████████████████████  45,230  (Avg: 1.2K) │
│                                                             │
│  Output Tokens                                              │
│  ████████████████████                  23,891  (Avg: 0.6K) │
│                                                             │
│  ─────────────────────────────────────────────────────────  │
│  Kompression: 23% eingespart durch Context-Compression      │
│  Cache-Hit-Rate: 34%                                        │
└─────────────────────────────────────────────────────────────┘
```

### Modell-Nutzung

```
┌─────────────────────────────────────────────────────────────┐
│ Nach Modell                                                                │
│                                                             │
│  GPT-4 Turbo          ████████████████████  52%  $8.40     │
│  Claude 3 Sonnet      ██████████████        38%  $3.20     │
│  Local (Ollama)       ████                  10%  $0.00     │
│                                                             │
│  Durchschn. Latenz: 2.3s                                    │
│  Erfolgsrate: 98.5%                                         │
└─────────────────────────────────────────────────────────────┘
```

### Effizienz-Metriken

| Metrik | Beschreibung | Optimaler Wert |
|--------|--------------|----------------|
| Tokens/Anfrage | Durchschnittliche Token-Anzahl | < 2000 |
| Kompressionsrate | Einsparung durch Compression | > 20% |
| Cache-Hit-Rate | Wiederverwendete Kontexte | > 30% |
| Reuse-Rate | Identische Prompts | > 15% |

## Budget Tracking Display

### Kosten-Übersicht

```
┌─────────────────────────────────────────────────────────────┐
│ Budget Tracking                                             │
│                                                             │
│  Monatslimit: [████████████████░░░░░░░░]  $32.40 / $50.00  │
│                                                             │
│  Heute:      $1.23  ███░░░░░░░░░░░░░░░  8%                 │
│  Diese Woche: $8.90  ████████░░░░░░░░░  45%                │
│  Dieser Monat: $32.40  ████████████████████░░░░  65%       │
│                                                             │
│  Prognose: $48.50 (unter Limit ✓)                          │
└─────────────────────────────────────────────────────────────┘
```

### Warnstufen

| Auslastung | Indikator | Aktion |
|------------|-----------|--------|
| < 50% | 🟢 Grün | Keine |
| 50-75% | 🟡 Gelb | Beobachten |
| 75-90% | 🟠 Orange | Limit anpassen oder Kosten senken |
| > 90% | 🔴 Rot | Sofort eingreifen |

### Kosten nach Kategorie

```
┌─────────────────────────────────────────┐
│ Kosten-Aufschlüsselung (heute)         │
│                                         │
│ LLM API Calls    $0.85  ████████████░░ │
│ Embeddings       $0.12  ██░░░░░░░░░░░░ │
│ Web Search       $0.08  █░░░░░░░░░░░░░ │
│ Other Tools      $0.18  ██░░░░░░░░░░░░ │
│─────────────────────────────────────────│
│ Total            $1.23                 │
└─────────────────────────────────────────┘
```

> 💡 Aktiviere Budget-Warnungen unter Config → Budget, um Benachrichtigungen zu erhalten.

## Memory-Statistiken

### Vektordatenbank

```
┌─────────────────────────────────────────────────────────────┐
│ Memory – Vektordatenbank                                   │
│                                                             │
│  Gespeicherte Fakten:  12,847                              │
│  Vektoren:             12,847                              │
│  Durchschn. Vektor-Größe:  384 Dimensionen                 │
│  Speicherbedarf:       45.2 MB                             │
│                                                             │
│  Letzte Woche:  +234 Fakten  ████░░░░░░░░░░░░░░░░          │
│  Index-Typ: HNSW | Suche: ~5ms                             │
└─────────────────────────────────────────────────────────────┘
```

### Knowledge Graph

Der Knowledge Graph Tab bietet umfassende Einblicke in Auras strukturiertes Gedächtnissystem.

#### Knowledge Graph Zusammenfassung

```
Knowledge Graph Statistiken
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Knoten: 1,247    Typen: 89      Kanten: 3,892        │
│                                                         │
│  Top-Entitätstypen:                                    │
│  • Person: 234    • Ort: 189                          │
│  • Thema: 412     • Ereignis: 156                     │
│  • Tool: 89       • Sonstige: 167                     │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

#### Graph-Qualität

Der Graph-Qualität-Bereich hilft, ein gesundes Wissensnetz zu pflegen:

```
Graph-Qualitätsmetriken
┌─────────────────────────────────────────────────────────┐
│  🛡️ Geschützt: 12   📍 Isoliert: 5   🏷️ Ohne Typ: 1 │
└─────────────────────────────────────────────────────────┘

Isolierte Knoten (keine Verbindungen):
  • node_id_abc123 ... 2 direkte Verbindungen
  • node_id_def456 ... 1 direkte Verbindung

Knoten ohne Typ (fehlende Typ-Eigenschaft):
  • node_id_ghi789 ... Label: "Ein Thema"

Duplikat-Kandidaten (identische Labels):
  • "Besprechungsnotizen" ... 3 Vorkommen
  • "Projekt Alpha" ... 2 Vorkommen
```

**Qualitätsmetriken:**
| Metrik | Beschreibung | Idealwert |
|--------|--------------|------------|
| **Geschützte Knoten** | Knoten die vor Bereinigung geschützt sind | Beliebig |
| **Isolierte Knoten** | Knoten ohne Verbindungen | Möglichst 0 |
| **Knoten ohne Typ** | Knoten ohne Typ-Eigenschaft | Möglichst 0 |
| **Duplikat-Gruppen** | Gruppen mit identischen Labels | Möglichst 0 |

**Qualitätsprobleme:**
- **Isolierte Knoten**: Können verwaiste oder unvollständige Daten anzeigen
- **Knoten ohne Typ**: Sollten einen Typ für ordnungsgemäße Kategorisierung erhalten
- **Duplikate**: Erwäge das Zusammenführen von Knoten mit identischen Labels

#### KG-Explorer

Suchen und durchsuchen von Wissensgraph-Einträgen:

```
KG-Explorer
┌─────────────────────────────────────────────────────────┐
│  🔍 Suche: [________________________] [Suchen]        │
│                                                         │
│  Ergebnisse: "projekt" → 23 Knoten, 45 Kanten        │
│                                                         │
│  Letzte Knoten:                                        │
│  • Projekt Alpha          node_001                      │
│  • Projekt Beta           node_002                      │
│  • Besprechungsnotizen   node_003                      │
│                                                         │
│  Letzte Kanten:                                       │
│  • projekt_alpha --verwandet_mit--> besprechungsnotizen│
│  • benutzer_jan --erstellt--> projekt_beta             │
└─────────────────────────────────────────────────────────┘
```

#### Graph-Visualisierung

Interaktive visuelle Darstellung des Wissensgraphs:

```
Graph-Ansicht
┌─────────────────────────────────────────────────────────┐
│  [Übersicht]  [Fokus]           [Zurücksetzen]        │
│                                                         │
│       ┌─────────────────────────────────────┐          │
│       │           ○ ──── ○                 │          │
│       │          /│\     /│\                │          │
│       │         ○ ○ ○   ○ ○ ○               │          │
│       │          \│/     \│/                │          │
│       │           ○ ──── ○                  │          │
│       └─────────────────────────────────────┘          │
│                                                         │
│  Zeige 50 Knoten und 120 Kanten aus der Stichprobe.  │
└─────────────────────────────────────────────────────────┘
```

- **Übersicht**: Zeigt eine Stichprobe aller Knoten und Kanten
- **Fokus**: Zentriert auf einem ausgewählten Knoten mit seinen direkten Nachbarn
- Klicke auf einen Knoten um Details im Knoten-Inspektor zu sehen

#### Knoten-Inspektor

Detaillierte Knoteneigenschaften anzeigen und bearbeiten:

```
Knoten-Inspektor
┌─────────────────────────────────────────────────────────┐
│  🧭 Knoten-Inspektor                        [Schützen] │
├─────────────────────────────────────────────────────────┤
│  Label: "Projekt Alpha"                                │
│  Typ: Projekt                                           │
│  ID: node_001                                          │
│  Geschützt: Ja                                           │
│                                                         │
│  Eigenschaften:                                         │
│  • erstellt_am: 2024-01-15                            │
│  • status: aktiv                                       │
│  • priorität: hoch                                     │
│                                                         │
│  Kanten (12):                                          │
│  • --erstellt_von--> benutzer_jan                      │
│  • --verwandt_mit--> besprechungsnotizen (2x)         │
│  • --hat_schlagwort--> dringend                        │
│                                                         │
│  [Eigenschaften bearbeiten]  [Knoten löschen]          │
└─────────────────────────────────────────────────────────┘
```

### Memory-Nutzung nach Typ

| Typ | Anzahl | Speicher | Beschreibung |
|-----|--------|----------|--------------|
| Short-term | 50 | ~50 KB | Aktuelle Sitzung |
| Working | 500 | ~500 KB | Aktive Projekte |
| Long-term | 12,000 | ~40 MB | Permanente Fakten |
| Knowledge Graph | 3,400 | ~12 MB | Verknüpfte Entitäten |

## Diagramme verstehen

### Zeitliche Auflösung

| Zeitraum | Datenpunkte | Verwendung |
|----------|-------------|------------|
| Realtime | Jede Sekunde | Aktuelle Auslastung |
| 1h | Jede Minute | Kurzfristige Trends |
| 24h | Alle 15 Min | Tagesverlauf |
| 7d | Jede Stunde | Wochenvergleich |
| 30d | Jeder Tag | Langfristige Entwicklung |

### Interaktive Features

```
┌─────────────────────────────────────────┐
│ 📊 Diagramm-Interaktionen              │
│                                         │
│  Hover    → Tooltip mit Details        │
│  Click    → Drill-down zu Details      │
│  Zoom     │ Mausrad / Pinch           │
│  Pan      │ Drag & Drop               │
│  Reset    │ Doppelklick               │
│                                         │
│  [1h] [24h] [7d] [30d] [Custom]        │
└─────────────────────────────────────────┘
```

### Legende und Farbcodierung

| Farbe | Bedeutung |
|-------|-----------|
| 🟢 Grün | Normal, optimal |
| 🟡 Gelb | Erhöht, beachten |
| 🟠 Orange | Hoch, handeln |
| 🔴 Rot | Kritisch, sofort handeln |
| 🔵 Blau | Information, neutral |
| 🟣 Lila | Spezial, hervorgehoben |

## Dashboard-Anpassung

### Widgets hinzufügen/entfernen

1. **Klicke** auf das Zahnrad-Icon ⚙️ oben rechts
2. **Wähle** "Widgets konfigurieren"
3. **Ziehe** Widgets per Drag & Drop
4. **Entferne** Widgets mit dem ✕
5. **Speichere** die Anordnung

### Verfügbare Widgets

| Widget | Beschreibung |
|--------|--------------|
| System Metrics | CPU, RAM, Disk |
| Mood Chart | Emotions-Verlauf |
| Token Usage | API-Nutzung |
| Budget | Kosten-Tracking |
| Memory Stats | Speicher-Übersicht |
| Missions | Aktive Missionen |
| Invasion | Remote Nodes |
| Custom Graph | Eigene Metriken |

### Layout-Optionen

```yaml
# Dashboard-Konfiguration in config.yaml
dashboard:
  layout:
    columns: 3  # 1, 2, 3, oder 4 Spalten
    auto_refresh: 30  # Sekunden
    
  widgets:
    - type: "system_metrics"
      position: [0, 0]
      size: "large"
      
    - type: "mood_chart"
      position: [1, 0]
      size: "medium"
      timeframe: "7d"
      
    - type: "budget"
      position: [2, 0]
      size: "small"
      show_forecast: true
```

## Daten exportieren

### Export-Formate

| Format | Verwendung |
|--------|------------|
| CSV | Tabellenkalkulation |
| JSON | Programmatische Verarbeitung |
| PNG | Bild für Berichte |
| PDF | Dokumentation |

### Export durchführen

1. **Wähle** den gewünschten Zeitraum
2. **Klicke** auf 📥 "Export" oben rechts
3. **Wähle** das Format
4. **Optional:** Filter anwenden
5. **Download** startet automatisch

### API-Export

```bash
# CSV-Export
curl "http://localhost:8088/api/dashboard/export?format=csv&from=2024-01-01&to=2024-01-31" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o metrics_january.csv

# JSON-Export
curl "http://localhost:8088/api/dashboard/metrics?type=token_usage&hours=24" \
  -H "Authorization: Bearer YOUR_TOKEN" | jq
```

### Beispiel: Automatisierter Report

```bash
#!/bin/bash
# weekly_report.sh

DATE=$(date +%Y-%m-%d)
./aurago dashboard export \
  --format pdf \
  --from "7 days ago" \
  --to "today" \
  --output "report_${DATE}.pdf"

# Per Email versenden
mail -s "AuraGo Weekly Report" admin@example.com \
  -A "report_${DATE}.pdf"
```

## Echtzeit-Updates

### WebSocket-Verbindung

Das Dashboard nutzt WebSockets für Live-Updates:

```
┌─────────────┐      WebSocket       ┌─────────────┐
│  Browser    │◄────────────────────►│  AuraGo     │
│  (Dashboard)│   Echtzeit-Updates   │  (Server)   │
└─────────────┘                      └─────────────┘
```

### Update-Intervalle

| Metrik | Update-Intervall |
|--------|------------------|
| CPU/RAM/Disk | 5 Sekunden |
| Token Usage | 30 Sekunden |
| Mood | Bei Änderung |
| Budget | Bei API-Call |
| Mission Status | 10 Sekunden |

### Manuelles Aktualisieren

```
┌─────────────────────────────────────────┐
│  Letztes Update: Vor 5 Sekunden    🔄  │
└─────────────────────────────────────────┘
```

Klicke auf 🔄 für sofortige Aktualisierung.

### Offline-Modus

Wenn die Verbindung unterbrochen wird:

```
┌─────────────────────────────────────────┐
│  ⚠️  Verbindung unterbrochen            │
│  Letzte Daten: Vor 2 Minuten           │
│                                         │
│  [Wieder verbinden]                     │
└─────────────────────────────────────────┘
```

> 💡 Das Dashboard speichert bis zu 1 Stunde Daten lokal, um Unterbrechungen zu überbrücken.

## Benachrichtigungen

### Alert-Konfiguration

```yaml
dashboard:
  alerts:
    - metric: "cpu_usage"
      threshold: 80
      duration: "5m"
      action: "notify"
      
    - metric: "budget_daily"
      threshold: 5.00
      action: "warn"
      
    - metric: "disk_usage"
      threshold: 90
      action: "critical"
```

### Alert-Level

| Level | Farbe | Kanäle |
|-------|-------|--------|
| Info | 🔵 | Dashboard |
| Warn | 🟡 | Dashboard + Email |
| Critical | 🔴 | Dashboard + Email + Telegram |

## Tipps & Tricks

### Performance-Optimierung

- **Reduziere** die Anzahl der Widgets auf langsamen Geräten
- **Passe** das Aktualisierungsintervall an
- **Nutze** Zeitfilter statt lange Zeiträume

### Mobile Nutzung

Das Dashboard ist responsive und passt sich an:

```
Desktop:  [Widget 1] [Widget 2] [Widget 3]
Tablet:   [Widget 1] [Widget 2]
          [   Widget 3   ]
Mobile:   [Widget 1]
          [Widget 2]
          [Widget 3]
```

### Tastenkürzel

| Kürzel | Funktion |
|--------|----------|
| `R` | Aktualisieren |
| `F` | Vollbild-Modus |
| `E` | Export öffnen |
| `1-9` | Zu Widget wechseln |
| `Esc` | Einstellungen schließen |

## Fehlerbehebung

| Problem | Ursache | Lösung |
|---------|---------|--------|
| Diagramme laden nicht | Browser-Cache | Strg+F5 drücken |
| Daten veraltet | WebSocket getrennt | Seite neu laden |
| Falsche Werte | Zeitzonen-Problem | Zeitzone prüfen |
| Langsame Ladezeit | Zu viele Widgets | Widgets reduzieren |
| Export fehlschlägt | Zu große Datenmenge | Zeitraum verkleinern |

## Nächste Schritte

- **[Mission Control](11-missions.md)** – Aufgaben automatisieren
- **[Invasion Control](12-invasion.md)** – Remote-Nodes verwalten
- **API-Dokumentation** – Programmatischer Zugriff auf Metriken

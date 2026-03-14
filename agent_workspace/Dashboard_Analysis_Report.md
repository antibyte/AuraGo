# Dashboard-Analyse: Optimierungsvorschläge

## Executive Summary

Das AuraGo-Dashboard bietet bereits eine **umfassende Übersicht** mit 11 Karten über 4 Kategorien. Die Informationsdichte ist jedoch hoch und einige wichtige Metriken fehlen. Der Bericht identifiziert **12 Verbesserungsbereiche** mit Fokus auf "Schneller Überblick, Details auf Klick".

---

## Aktuelle Dashboard-Struktur

### Vorhandene Karten (11)

| # | Karte | Kategorie | Datenquelle | Bewertung |
|---|-------|-----------|-------------|-----------|
| 0 | **Agent Status Banner** | System | `/api/dashboard/overview` | ⭐⭐⭐⭐⭐ |
| 1 | **System Health** | System | `/api/dashboard/system` | ⭐⭐⭐⭐☆ |
| 2 | **Budget & Tokens** | Kosten | `/api/budget`, `/api/credits` | ⭐⭐⭐⭐⭐ |
| 3 | **Persönlichkeit** | AI | `/api/personality/state` | ⭐⭐⭐⭐☆ |
| 4 | **Gedächtnis** | AI | `/api/dashboard/memory` | ⭐⭐⭐⭐☆ |
| 5 | **Nutzerprofil** | AI | `/api/dashboard/profile` | ⭐⭐⭐☆☆ |
| 6 | **Betrieb & Dienste** | System | `/api/dashboard/overview` | ⭐⭐⭐⭐☆ |
| 7 | **Aktivität** | System | `/api/dashboard/activity` | ⭐⭐⭐⭐☆ |
| 8 | **Prompt-Analyse** | AI | `/api/dashboard/prompt-stats` | ⭐⭐⭐⭐⭐ |
| 9 | **GitHub Repos** | Integration | `/api/dashboard/github-repos` | ⭐⭐⭐☆☆ |
| 10 | **Live-Log** | System | `/api/dashboard/logs` | ⭐⭐⭐⭐☆ |

### Daten-Refresh-Verhalten
- **Keine Echtzeit-Updates**: Alle Daten werden einmalig beim Laden geholt
- **Keine Auto-Refresh-Logik**: Nutzer müssen manuell aktualisieren
- **Ausnahme**: Log-Viewer hat einen Refresh-Button

---

## 🔴 Kritisch: Fehlende Informationen

### 1. Echtzeit-Agent-Status
**Problem**: Der "Busy"-Status im Banner ist statisch und zeigt nicht:
- Aktuelle Aktion (welches Tool wird ausgeführt?)
- Fortschritt laufender Operationen (z.B. "Indiziere Dateien: 45%")
- Warteschlangen-Status

**Empfohlene Lösung**:
```javascript
// Neuer Endpunkt: /api/agent/status
{
  "state": "working",  // idle, working, error, maintenance
  "current_task": {
    "type": "file_indexing",
    "description": "Indiziere /docs",
    "progress": 45,
    "eta_seconds": 120
  },
  "queue_length": 3,
  "last_error": null
}
```

**UI**: 
- Banner zeigt animierten Indikator bei aktiver Arbeit
- Klick öffnet Detail-Modal mit Task-Liste

---

### 2. Kritische Fehler & Warnungen
**Problem**: Keine zentrale Übersicht über:
- Letzte Fehler (aus Logs extrahiert)
- Warnungen (z.B. "Speicher fast voll")
- Fehlgeschlagene Missionen/Tasks

**Empfohlene Lösung**:
```javascript
// Neuer Endpunkt: /api/alerts
{
  "critical": [],
  "warnings": [
    {
      "level": "warning",
      "source": "system",
      "message": "Speicher nutzung > 85%",
      "timestamp": "2026-03-14T10:30:00Z",
      "action_url": "/config"
    }
  ],
  "info": []
}
```

**UI**: 
- Kompakte Alert-Bar unter dem Header (nur Anzahl + Icon)
- Dropdown bei Klick mit Details
- Farbcodierung: Rot = Kritisch, Gelb = Warnung, Blau = Info

---

### 3. LLM-Performance-Metriken
**Problem**: Keine Einblicke in:
- Durchschnittliche Antwortzeit
- Fehlerrate der LLM-Calls
- Token-Durchsatz pro Minute
- Model-Switching-History (Fallback-LLM)

**Empfohlene Lösung**:
```javascript
// Erweiterung: /api/dashboard/overview.llm_performance
{
  "avg_response_time_ms": 1450,
  "error_rate_24h": 0.02,
  "tokens_per_minute": 4500,
  "fallback_activations_24h": 3,
  "model_distribution": {
    "gpt-4o": 85,
    "gpt-3.5-turbo": 15
  }
}
```

---

## 🟡 Wichtig: Verbesserungsmöglichkeiten

### 4. Speicher-Nutzung-Visualisierung
**Aktuell**: Einfache Zahlen (Core Facts, Messages, Embeddings)

**Verbesserung**:
- **Treemap** oder **Stacked Bar** zeigt relative Größe
- **Wachstums-Trend** ("+12% diese Woche")
- **Limit-Warnungen** ("VectorDB bei 85% Kapazität")

```javascript
// Erweiterung: /api/dashboard/memory
{
  "growth": {
    "core_memory_7d": "+5",
    "messages_7d": "+234",
    "vectordb_7d": "+12"
  },
  "limits": {
    "vectordb_max": 10000,
    "vectordb_warning": 8000
  }
}
```

---

### 5. Konversations-Übersicht
**Fehlt komplett**: Keine Metriken über:
- Gespräche heute/diese Woche
- Durchschnittliche Gesprächslänge
- Häufigste Themen (aus RAG-Queries)
- Nutzer-Aktivitäts-Heatmap

**Empfohlene Lösung**: Neue Karte "Konversationen"
```javascript
// Neuer Endpunkt: /api/conversations/stats
{
  "today": 12,
  "this_week": 89,
  "avg_length": 8.5,
  "avg_response_time": "2.3s",
  "top_topics": ["Coding", "System-Admin", "Fragen"]
}
```

---

### 6. Integration-Health-Check
**Aktuell**: Nur "Enabled/Disabled" Booleans

**Verbesserung**:
- **Verbindungsstatus** pro Integration (z.B. MQTT verbunden?)
- **Letzte erfolgreiche Verbindung**
- **Fehlerzähler** (z.B. "GitHub API: 3 Fehler heute")

```javascript
// Erweiterung: /api/dashboard/overview.integrations
{
  "mqtt": {
    "enabled": true,
    "connected": true,
    "last_ping": "2026-03-14T10:29:00Z"
  },
  "github": {
    "enabled": true,
    "api_reachable": true,
    "rate_limit_remaining": 4500
  }
}
```

---

### 7. Tool-Usage-Analytics
**Fehlt**: Welche Tools werden wie oft genutzt?

```javascript
// Neuer Endpunkt: /api/analytics/tools
{
  "most_used_24h": [
    {"tool": "execute_shell", "count": 45},
    {"tool": "manage_memory", "count": 23}
  ],
  "error_rate_by_tool": {
    "execute_shell": 0.05,
    "docker": 0.12
  },
  "avg_execution_time": {
    "execute_shell": "1.2s",
    "web_search": "3.5s"
  }
}
```

---

### 8. Mobile-Optimierung
**Aktuelle Probleme**:
- Grid bricht bei <900px um, aber viele Karten sind zu hoch
- Charts werden zu klein
- Horizontales Scrollen in Tabellen

**Empfohlene Lösung**:
- **Karten-Stack**: 1 Spalte auf Mobile
- **Zusammenfassungs-Modus**: Nur wichtigste KPIs sofort sichtbar
- **Swipe-Navigation** zwischen Karten

---

## 🟢 Nice-to-Have: Erweiterungen

### 9. Zeitbasierte Vergleiche
**Idee**: "Diese Woche vs. Letzte Woche" für:
- Token-Verbrauch
- Anzahl Gespräche
- System-Load

**UI**: Kleine Trend-Indikatoren (↑5%, ↓12%) neben den Werten

---

### 10. Anpassbares Layout
**Idee**: Nutzer können:
- Karten umsortieren (Drag & Drop)
- Karten ausblenden
- Eigenes "Executive Summary"-Dashboard erstellen

**Implementierung**: 
- `localStorage` für Layout-Einstellungen
- "Kompaktmodus"-Toggle (nur KPIs, keine Charts)

---

### 11. Export-Funktionen
**Fehlt**: Möglichkeit, Dashboard-Daten zu exportieren

**Lösung**: 
- "Bericht generieren"-Button
- PDF/JSON Export der aktuellen Ansicht
- Scheduled Reports (täglich/wöchentlich per E-Mail)

---

### 12. Keyboard Shortcuts
**Fehlt**: Keine Tastatur-Navigation

**Vorschläge**:
- `R` = Refresh
- `L` = Focus Log-Viewer
- `C` = Chat öffnen
- `1-9` = Karte N fokussieren
- `?` = Shortcut-Hilfe

---

## Konkrete Implementierungsvorschläge

### Phase 1: Kritische Fixes (Sofort)

```go
// internal/server/dashboard_handlers.go

// NEU: /api/agent/status
func handleAgentStatus(s *Server) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Aktuellen Zustand aus der Agent-Loop holen
        status := map[string]interface{}{
            "state": getAgentState(), // idle, working, error, maintenance
            "current_task": getCurrentTask(),
            "queue_length": getQueueLength(),
            "progress": getCurrentProgress(), // 0-100 oder nil
        }
        json.NewEncoder(w).Encode(status)
    }
}

// NEU: /api/alerts
func handleAlerts(s *Server) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        alerts := collectAlerts(s) // Aus Logs, System-Status, etc.
        json.NewEncoder(w).Encode(alerts)
    }
}
```

### Phase 2: UI-Verbesserungen (1-2 Wochen)

```javascript
// ui/js/dashboard/main.js - Verbesserungen

// 1. Auto-Refresh alle 30 Sekunden
setInterval(refreshDashboard, 30000);

// 2. Kollabierbare Karten (eingebaut, aber verbessern)
// Zustand in localStorage speichern

// 3. Alert-Banner
function renderAlerts(alerts) {
    if (alerts.critical.length === 0 && alerts.warnings.length === 0) return;
    
    const banner = document.createElement('div');
    banner.className = `alert-banner ${alerts.critical.length > 0 ? 'critical' : 'warning'}`;
    banner.innerHTML = `
        <span class="alert-icon">⚠️</span>
        <span class="alert-count">
            ${alerts.critical.length} kritisch, ${alerts.warnings.length} Warnungen
        </span>
        <button onclick="showAlertDetails()">Details</button>
    `;
    document.body.prepend(banner);
}
```

### Phase 3: Neue Karten (2-4 Wochen)

```html
<!-- dashboard.html - Neue Karten -->

<!-- 11. Konversationen (neu) -->
<div class="dash-card" id="card-conversations">
    <div class="dash-card-header">
        <span class="card-icon">💬</span> 
        <span data-i18n="dashboard.conversations_title">Konversationen</span>
    </div>
    <div class="dash-card-body">
        <div class="conv-stats" id="conv-stats"></div>
        <div class="conv-topics" id="conv-topics"></div>
    </div>
</div>

<!-- 12. Integration Health (neu) -->
<div class="dash-card" id="card-health">
    <div class="dash-card-header">
        <span class="card-icon">❤️</span> 
        <span data-i18n="dashboard.health_title">System Health</span>
    </div>
    <div class="dash-card-body">
        <div class="health-grid" id="health-grid"></div>
    </div>
</div>
```

---

## Zusammenfassung der Empfehlungen

| Priorität | Feature | Aufwand | Impact |
|-----------|---------|---------|--------|
| 🔴 Kritisch | Echtzeit-Agent-Status | Mittel | Hoch |
| 🔴 Kritisch | Alert-Banner | Niedrig | Hoch |
| 🔴 Kritisch | LLM-Performance-Metriken | Mittel | Mittel |
| 🟡 Wichtig | Speicher-Trends | Niedrig | Mittel |
| 🟡 Wichtig | Konversations-Übersicht | Mittel | Hoch |
| 🟡 Wichtig | Integration Health | Mittel | Mittel |
| 🟡 Wichtig | Auto-Refresh | Niedrig | Hoch |
| 🟢 Nice | Anpassbares Layout | Hoch | Niedrig |
| 🟢 Nice | Export-Funktionen | Mittel | Niedrig |

---

## Dashboard-Mockup (Text)

```
┌─────────────────────────────────────────────────────────────────┐
│  AURAGO DASHBOARD                    [🌙] [🔔 3 Alerts]         │
├─────────────────────────────────────────────────────────────────┤
│  🤖 GPT-4o  ● Bereit  🎭 neutral  📐 45%  🔌 12/25  💬 2h ago   │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │ 🖥️ SYSTEM   │ │ 💰 BUDGET   │ │ 🧠 MEMORY   │               │
│  │ CPU [////]  │ │ $12/$50     │ │ Core: 45    │               │
│  │ RAM [/// ]  │ │ ↻ 8h left   │ │ ↑ +5%       │               │
│  │ ▓▓▓▓░░ 70%  │ │ [Chart]     │ │ [Treemap]   │               │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ⚡ AKTIVITÄT (klickbar für Details)                     │   │
│  │  [🚀 5 Missionen] [🥚 3 Eggs] [📂 1.2k indiziert] ...    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 📝 PROMPT-ANALYSE                                       │   │
│  │  [Builds] [Tokens] [Einsparungen] [Tier-Verteilung]      │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

*Bericht erstellt: 2026-03-14*
*Empfohlene Priorisierung: Phase 1 sofort umsetzen, Phase 2 innerhalb 2 Wochen*

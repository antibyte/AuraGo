# KI-Gerechte Projektstruktur - Bewertung

**Datum:** 2026-03-13  
**Projekt:** AuraGo

---

## Gesamtbewertung

| Metrik | Wert | Bewertung |
|--------|------|-----------|
| **KI-Gerecht-Score** | **83.8%** | ✅ Gut |
| **Optimal (<300 Zeilen)** | 124 Dateien (67%) | ✅ Sehr gut |
| **Akzeptabel (<500 Zeilen)** | 152 Dateien (83%) | ✅ Gut |
| **Problematisch (>800 Zeilen)** | 17 Dateien (9%) | ⚠️ Zu verbessern |

**Fazit:** Das Projekt ist weitgehend KI-gerecht aufgeteilt. Die meisten Dateien haben eine angemessene Größe für KI-Verarbeitung.

---

## Was bereits gut ist ✅

### 1. Package-Struktur größtenteils ausbalanciert
- 11 von 15 Packages sind optimal strukturiert
- Kleine Packages wie `discord`, `budget`, `meshcentral` sind perfekt

### 2. Durchschnittliche Dateigröße
- **299 Zeilen/Datei** im Durchschnitt
- Liegt im optimalen Bereich für KI-Verarbeitung

### 3. Offensichtliche Aufteilung bereits erfolgt
Es gibt Hinweise, dass bereits Refactoring stattgefunden hat:
- `agent_loop.go` (1602 Zeilen) - vermutlich aus `agent.go` ausgelagert
- `agent_dispatch_infra.go` (1058 Zeilen) - separate Infrastruktur-Datei
- `agent_parse.go` (974 Zeilen) - Parsing-Logik separiert

---

## Was verbessert werden sollte ⚠️

### 1. Kritische Packages (zu viele Zeilen gesamt)

| Package | Zeilen | Dateien | Status |
|---------|--------|---------|--------|
| `tools` | 14.025 | 56 | 🔴 Zu groß |
| `server` | 10.933 | 32 | 🔴 Zu groß |
| `agent` | 8.760 | 15 | 🔴 Zu groß |
| `memory` | 4.175 | 10 | ⚠️ Grenzwertig |

**Problem:** Diese Packages haben zu viele Zeilen *insgesamt*, auch wenn einzelne Dateien OK sind. Das macht es schwierig, das gesamte Package im KI-Kontext zu verstehen.

### 2. Zu große Einzeldateien

#### 🔴 XXL (1200-2000 Zeilen)
| Datei | Zeilen | Empfohlene Aufteilung |
|-------|--------|----------------------|
| `internal/config/config.go` | 1.617 | In 3-4 Dateien aufteilen |
| `internal/agent/agent_loop.go` | 1.602 | Bereits separiert, aber noch zu groß |
| `internal/server/server.go` | 1.236 | In 2-3 Dateien aufteilen |
| `internal/tools/homepage.go` | 1.203 | In 2-3 Dateien aufteilen |

#### ⚠️ XLarge (800-1200 Zeilen)
| Datei | Zeilen | Handlungsempfohlung |
|-------|--------|---------------------|
| `internal/prompts/builder.go` | 1.199 | Aufteilen |
| `internal/server/handlers.go` | 1.100 | Aufteilen |
| `internal/agent/agent_dispatch_infra.go` | 1.058 | Aufteilen |
| `internal/memory/short_term.go` | 986 | Aufteilen |
| `internal/tools/netlify.go` | 982 | Aufteilen |
| `internal/server/config_handlers.go` | 978 | Aufteilen |
| `internal/agent/native_tools.go` | 977 | Aufteilen |
| `internal/agent/agent_parse.go` | 974 | Aufteilen |
| `internal/memory/personality.go` | 965 | Aufteilen |
| `internal/tools/docker.go` | 859 | Aufteilen |

---

## Konkrete Handlungsempfehlungen

### Priorität 1: Kritische Packages restrukturieren

#### Package `tools` (14.025 Zeilen)
Aufteilen in Sub-Packages:
```
internal/tools/
├── web/           # homepage, netlify (externe Dienste)
├── devops/        # docker, ansible, github
├── comms/         # email, webhooks, discord
├── cloud/         # netlify, tailscale
└── utils/         # sandbox, missions, etc.
```

#### Package `server` (10.933 Zeilen)
Aufteilen nach Handler-Typ:
```
internal/server/
├── core/          # server.go, middleware
├── handlers/
│   ├── chat/      # Chat-Handler
│   ├── config/    # Config-Handler
│   ├── dashboard/ # Dashboard-Handler
│   └── api/       # API-Handler
└── websocket/     # WebSocket-spezifisch
```

#### Package `agent` (8.760 Zeilen)
Die bereits begonnene Aufteilung fortsetzen:
```
internal/agent/
├── agent.go           # Core-Interface
├── agent_loop.go      # Event Loop (weiter verkleinern)
├── agent_dispatch_infra.go  # Dispatching
├── agent_parse.go     # Parsing
├── native_tools.go    # Native Tools
├── tools/             # Tool-Verzeichnis
│   ├── tool_*.go      # Einzelne Tools
│   └── registry.go    # Tool-Registrierung
└── workflow/          # Workflow-Logik
    ├── workflow.go
    └── steps.go
```

---

## Ziel-Struktur für KI-Optimierung

### Ideale Dateigrößen
| Kategorie | Zeilen | Anzahl | Verwendung |
|-----------|--------|--------|------------|
| Mikro | <100 | 30-40% | Utils, Helper |
| Klein | 100-300 | 40-50% | Hauptlogik |
| Mittel | 300-500 | 10-20% | Komplexe Module |
| Groß | 500-800 | <10% | Selten |

### Aktueller vs. Ziel-Status

| Metrik | Aktuell | Ziel | Status |
|--------|---------|------|--------|
| Dateien <300 Zeilen | 67% | >70% | ✅ Fast erreicht |
| Dateien >800 Zeilen | 9% | <5% | ⚠️ Noch zu hoch |
| Package-Größe max | 14.025 | <5.000 | 🔴 Zu groß |

---

## Zusammenfassung

### ✅ Starkpunkte
1. **67% der Dateien** sind optimal (<300 Zeilen)
2. **83% der Dateien** sind akzeptabel (<500 Zeilen)
3. **Durchschnitt** von 299 Zeilen/Datei ist sehr gut
4. **Bereits begonnene Aufteilung** der Agent-Logik

### ⚠️ Verbesserungsbedarf
1. **4 Packages** sind zu groß (>5.000 Zeilen)
2. **17 Dateien** sind problematisch (>800 Zeilen)
3. **Tools-Package** mit 14.025 Zeilen zu monolithisch

### 🎯 Empfohlene nächste Schritte
1. **Sofort:** Die 4 XXL-Dateien (>1200 Zeilen) aufteilen
2. **Kurzfristig:** Die 9 XLarge-Dateien (800-1200 Zeilen) aufteilen
3. **Mittelfristig:** Package `tools` in Sub-Packages aufteilen
4. **Langfristig:** Package `server` restrukturieren

---

## Fazit

**Das Projekt ist KI-gerecht aufgeteilt, aber es gibt noch Optimierungspotenzial.**

Der KI-Gerecht-Score von **83.8%** zeigt, dass die meisten Dateien eine gute Größe haben. Die Hauptprobleme liegen in wenigen, aber sehr großen Dateien und Packages. Eine konsequente Aufteilung der 17 problematischen Dateien würde den Score auf über 90% verbessern.

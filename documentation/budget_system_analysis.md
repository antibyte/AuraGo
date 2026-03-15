# Bericht: Budgeting-System Analyse

**Projekt:** AuraGo  
**Datum:** 15.03.2026  
**Autor:** Code-Analyse-Agent  
**Status:** Vollständige System-Review

---

## Zusammenfassung

Das Budgeting-System von AuraGo ist grundsätzlich gut konzipiert und funktioniert für den Standard-Anwendungsfall (OpenRouter). Es gibt jedoch **signifikante Lücken** bei der Preisermittlung für andere Provider und bei der Genauigkeit der Kostenerfassung.

**Gesamtbewertung:** ⚠️ Funktionsfähig, aber mit bekannten Ungenauigkeiten

---

## 1. Architektur-Überblick

### 1.1 Datenfluss

```
LLM-Aufruf → Token-Zählung (API oder Schätzung)
                ↓
         BudgetTracker.Record(model, input, output)
                ↓
         Preisermittlung (3-stufige Fallback-Logik)
                ↓
         Kostenberechnung → Speicherung in budget.json
                ↓
         UI-Update / Prompt-Hinweis
```

### 1.2 Kernkomponenten

| Komponente | Datei | Funktion |
|------------|-------|----------|
| **Budget Tracker** | `internal/budget/tracker.go` | Zentrale Tracking-Logik, Persistenz |
| **Preisermittlung** | `internal/llm/pricing.go` | Dynamische Preisabfrage (OpenRouter, etc.) |
| **Config-Typen** | `internal/config/config_types.go` | Strukturen für Budget, ModelCosts |
| **Integration** | `internal/agent/agent_loop.go` | Einbindung in den Agent-Loop |

---

## 2. Preisermittlung im Detail

### 2.1 Aktuelle Preisquellen

```go
// internal/budget/tracker.go:413-440
func (t *Tracker) findRatesLocked(model string) config.ModelCostRates {
    // 1. Per-Provider Model Costs (neu)
    for _, p := range t.cfg.Providers {
        for _, m := range p.Models { ... }
    }
    
    // 2. Legacy fallback: budget.models (veraltet)
    for _, m := range t.cfg.Budget.Models { ... }
    
    // 3. Global default (config.budget.default_cost)
    return t.cfg.Budget.DefaultCost
}
```

### 2.2 Dynamische Preisabfrage (nur bei Config-Generierung)

| Provider | Quelle | Qualität | Cache |
|----------|--------|----------|-------|
| **OpenRouter** | `https://openrouter.ai/api/v1/models` | ✅ Echt-Preise | 1 Stunde |
| **Ollama** | `/api/tags` | ✅ $0 (korrekt) | Nein |
| **Workers AI** | Hardcodiert + API-Fallback | ⚠️ Geschätzt | Nein |
| **OpenAI (direkt)** | OpenRouter-Proxy | ⚠️ Ungenau | 1 Stunde |
| **Anthropic (direkt)** | OpenRouter-Proxy | ⚠️ Ungenau | 1 Stunde |
| **Google (direkt)** | OpenRouter-Proxy | ⚠️ Ungenau | 1 Stunde |
| **Andere** | Global-Default | ❌ Schätzung | - |

---

## 3. Identifizierte Probleme

### 🔴 Kritisch: Falsche Preise bei direkten Provider-Verbindungen

**Problem:** Wenn ein Nutzer OpenAI oder Anthropic direkt (nicht über OpenRouter) verwendet, werden die Preise von OpenRouter als Proxy genutzt. Das ist oft ungenau weil:

1. OpenRouter fügt eigene Margen hinzu
2. Die Modell-Namen können abweichen (`gpt-4o` vs `openai/gpt-4o`)
3. Spezielle Preise (z.B. Batch-Processing) werden nicht berücksichtigt

**Code:**
```go
// internal/llm/pricing.go:39-56
case "openai":
    return fetchOpenRouterPricingFiltered("openai/")  // ← Proxy!
case "anthropic":
    return fetchOpenRouterPricingFiltered("anthropic/")  // ← Proxy!
```

**Empfohlene Lösung:**
- Direkte API-Abfrage für OpenAI: `https://api.openai.com/v1/models` mit Pricing
- Direkte API-Abfrage für Anthropic (oder aktuelle Dokumentation parsen)
- Nutzer können Preise manuell in `provider.models` überschreiben

---

### 🟠 Hoch: Token-Schätzung bei Streaming

**Problem:** Wenn die LLM-API keine `usage`-Daten zurückgibt (häufig bei Streaming), werden Tokens geschätzt:

```go
// internal/agent/agent_loop.go:733-738
completionTokens = estimateTokens(content)  // ← Schätzung!
for _, m := range req.Messages {
    promptTokens += estimateTokens(m.Content)  // ← Schätzung!
}
```

**Folge:** Das Budget kann um 10-30% davon abweichen.

**Empfohlene Lösung:**
- Warnung im UI anzeigen: "Token-Schätzung aktiv - Budget kann abweichen"
- Für bekannte Modelle verbesserte Heuristiken verwenden
- Nach dem Stream die tatsächlichen Kosten korrigieren (wenn verfügbar)

---

### 🟡 Mittel: Keine automatische Preis-Aktualisierung

**Problem:** Die dynamisch abgerufenen Preise (von OpenRouter) werden nicht automatisch in die Config übernommen. Sie müssen manuell via UI/Config eingetragen werden.

**Workflow aktuell:**
1. Admin ruft Preise ab (via Config-UI)
2. Preise werden angezeigt
3. Admin muss auf "Speichern" klicken

**Empfohlene Lösung:**
- Automatisches Update der Provider-Preise beim Start (optional)
- Oder: Transparenter Hinweis, dass Live-Preise verwendet werden

---

### 🟡 Mittel: Bildgenerierung nur geschätzt

**Problem:** Image Generation Kosten werden nur grob geschätzt:

```go
// internal/agent/agent_dispatch_exec.go:1075-1076
if budgetTracker != nil && result.CostEstimate > 0 {
    budgetTracker.RecordCost(result.CostEstimate)
}
```

Die `CostEstimate` ist eine grobe Annäherung basierend auf Modell-Heuristiken, nicht auf tatsächlicher API-Antwort.

---

### 🟢 Niedrig: Keine Unterstützung für:

- **Prompt-Caching-Kosten** (OpenAI, Anthropic)
- **Batch-Processing-Rabatte**
- **Fine-tuning-Modelle**
- **Einbettungs-Kosten** (separater Tracker nötig)

---

## 4. Was funktioniert gut ✅

| Feature | Bewertung | Kommentar |
|---------|-----------|-----------|
| **Persistenz** | ✅ Robust | JSON-basiert, atomare Schreibvorgänge |
| **Thread-Safety** | ✅ Korrekt | Mutex-Locks implementiert |
| **Tages-Reset** | ✅ Zuverlässig | Automatisch bei ResetHour |
| **Mehrere Modelle** | ✅ Gut | Separate Tracking pro Modell |
| **Enforcement** | ✅ Flexibel | warn/partial/full Modi |
| **OpenRouter-Preise** | ✅ Aktuell | Echtzeit-Abfrage mit Cache |

---

## 5. Konfigurationsempfehlungen

### 5.1 Für OpenRouter-Nutzer (Standard)

```yaml
budget:
  enabled: true
  daily_limit_usd: 5.0
  enforcement: partial  # Blockiert nur teure Features bei Überschreitung
  warning_threshold: 0.8
  default_cost:
    input_per_million: 1.0   # Fallback für unbekannte Modelle
    output_per_million: 3.0
  
providers:
  - id: openrouter
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-..."
    model: "google/gemini-2.0-flash-001"
    # Models-Liste wird automatisch vom System gefüllt
```

**Keine manuelle Konfiguration nötig - Preise werden automatisch abgerufen.**

### 5.2 Für OpenAI/Anthropic Direkt

```yaml
providers:
  - id: openai-direct
    type: openai
    base_url: https://api.openai.com/v1
    api_key: "sk-..."
    model: "gpt-4o"
    models:  # ← Preise MANUELL eintragen!
      - name: "gpt-4o"
        input_per_million: 2.50
        output_per_million: 10.00
      - name: "gpt-4o-mini"
        input_per_million: 0.15
        output_per_million: 0.60

budget:
  enabled: true
  daily_limit_usd: 10.0
  default_cost:
    input_per_million: 5.0   # Konservativer Fallback
    output_per_million: 15.0
```

⚠️ **Wichtig:** Ohne manuelle `models`-Konfiguration werden falsche Schätzwerte verwendet!

---

## 6. Verbesserungsvorschläge

### Priorität 1: Direkte Preis-APIs implementieren

```go
// Beispiel für OpenAI Direct Pricing
func fetchOpenAIDirectPricing(apiKey string) ([]ModelPricing, error) {
    // OpenAI bietet keine direkte Pricing-API, 
    // aber wir können die bekannten Preise hardcoden
    return []ModelPricing{
        {ModelID: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00},
        {ModelID: "gpt-4o-mini", InputPerMillion: 0.15, OutputPerMillion: 0.60},
        // ...
    }, nil
}
```

### Priorität 2: Warnung bei Token-Schätzung

```go
// In agent_loop.go
if GlobalTokenEstimated {
    broker.Send("budget_warning", "Token-Schätzung aktiv - Budget kann abweichen")
}
```

### Priorität 3: Preise automatisch aktualisieren

- Beim Start: Optionales `auto_fetch_pricing: true`
- Regelmäßig: Cron-Job für Preis-Updates (täglich)
- Im UI: "Preise aktualisieren"-Button mit Zeitstempel

### Priorität 4: Verbesserte Token-Schätzung

```go
// Statt simplem Längen-Verhältnis
func estimateTokensImproved(text string, model string) int {
    switch {
    case strings.Contains(model, "gpt-4"):
        // Tiktoken für genauere Schätzung
        return tiktokenEstimation(text, "cl100k_base")
    case strings.Contains(model, "claude"):
        // Claude verwendet andere Tokenisierung
        return int(float64(len(text)) / 3.5)
    default:
        return len(text) / 4  // Fallback
    }
}
```

---

## 7. Test-Empfehlungen

Um die Genauigkeit zu verifizieren:

1. **Vergleichstest:** 100 Requests mit bekanntem Modell
   - Tatsächliche Kosten (Provider-Invoice) vs. AuraGo-Budget
   - Akzeptanz: < 5% Abweichung

2. **Multi-Provider-Test:** Gleiche Anfrage an verschiedene Provider
   - Budget-Tracker sollte korrekte Preise für jeden verwenden

3. **Edge-Case-Test:**
   - Sehr lange Antworten (>50k Tokens)
   - Streaming mit Unterbrechung
   - Modell-Wechsel innerhalb einer Session

---

## 8. Fazit

| Aspekt | Bewertung |
|--------|-----------|
| **Tracking-Genauigkeit** | 🟡 Gut bei OpenRouter, ⚠️ Minderwertig bei Direkt-Providern |
| **System-Stabilität** | ✅ Hoch |
| **Konfigurations-Aufwand** | 🟡 Standard: Niedrig, Direkt-Providern: Hoch |
| **Preis-Aktualität** | 🟡 OpenRouter: Echtzeit, Andere: Ungenau |

**Empfehlung:**

1. **Kurzfristig:** Dokumentation ergänzen, dass Direkt-Providern manuelle Preis-Konfiguration erfordern
2. **Mittelfristig:** Hardcodierte Preislisten für OpenAI/Anthropic/Google implementieren
3. **Langfristig:** Automatische Preis-Aktualisierung und verbesserte Token-Schätzung

Das System ist produktionsreif für OpenRouter-Nutzer. Für Direkt-Provider-Verbindungen sollten die Preise manuell konfiguriert werden, um genaue Budgets zu gewährleisten.

---

*Ende des Berichts*

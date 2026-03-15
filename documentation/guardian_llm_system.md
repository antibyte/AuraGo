# AuraGo LLM Guardian System
## Optionale Sicherheitsebene für kritische Operationen

## Executive Summary

Das LLM Guardian System ist eine **optionale, zusätzliche Sicherheitsebene**, die ein dediziertes, kostengünstiges LLM als "Wächter" vor kritischen Operationen schaltet. Es fügt sich nahtlos in das bestehende regex-basierte Guardian-System ein und bietet tiefgehende semantische Analyse bei minimalen Token-Kosten.

**Kernprinzipien:**
- ⚡ **Token-Minimalismus**: Prompts <200 Tokens, Antworten <50 Tokens
- 💰 **Kosteneffizienz**: Optimiert für Gemini Flash, GPT-4o-mini, lokale 3B-Modelle
- 🔧 **Bereichs-spezifisch**: Konfigurierbar nach Schutz-Level pro Kategorie
- 🚀 **Performance**: Sub-500ms Entscheidungen mit aggressivem Caching

---

## Architektur-Übersicht

```
┌─────────────────────────────────────────────────────────────────┐
│                    USER REQUEST / TOOL CALL                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  STUFE 1: Regex Guardian (bestehend)                             │
│  • Schnelle Pattern-Erkennung                                     │
│  • Keine Kosten, <1ms                                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │   Suspicious?     │
                    └─────────┬─────────┘
                              │
              ┌───────────────┼───────────────┐
              │ YES           │ MEDIUM        │ NO
              ▼               ▼                ▼
    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
    │  AUTO-BLOCK     │ │  STUFE 2: LLM   │ │   ALLOW         │
    │  (keine Kosten) │ │  Guardian Check │ │   (keine Kosten)│
    └─────────────────┘ └────────┬────────┘ └─────────────────┘
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │  LLM Risk Assessment   │
                    │  • Kontext-Analyse     │
                    │  • Intent-Erkennung    │
                    │  • Data Classification │
                    └───────────┬────────────┘
                                │
                    ┌───────────┼───────────┐
                    │ SAFE      │ SUSPICIOUS│
                    ▼           ▼           ▼
            ┌──────────┐ ┌──────────┐ ┌──────────┐
            │  ALLOW   │ │ QUARANTINE│ │  BLOCK   │
            │ + Log    │ │ + Notify  │ │ + Alert  │
            └──────────┘ └──────────┘ └──────────┘
```

---

## Komponenten-Design

### 1. Guardian Controller

```go
// internal/security/llm_guardian.go
package security

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"
    
    "aurago/internal/config"
    "aurago/internal/llm"
)

// GuardianLevel definiert den Schutz-Level für verschiedene Bereiche
type GuardianLevel int

const (
    GuardianOff     GuardianLevel = iota // Deaktiviert
    GuardianLow                          // Nur High-Risk Tools
    GuardianMedium                       // Alle Tools + externe APIs
    GuardianHigh                         // Alle Operationen + Datenexport
    GuardianMaximum                      // Jede Operation
)

func (g GuardianLevel) String() string {
    switch g {
    case GuardianOff:     return "off"
    case GuardianLow:     return "low"
    case GuardianMedium:  return "medium"
    case GuardianHigh:    return "high"
    case GuardianMaximum: return "maximum"
    default:              return "unknown"
    }
}

// LLMGuardian ist der Haupt-Controller für LLM-basierte Sicherheitsprüfungen
type LLMGuardian struct {
    cfg       *config.GuardianConfig
    logger    *slog.Logger
    client    llm.ChatClient
    
    // Cache für wiederholte Prüfungen
    cache     *GuardianCache
    
    // Metriken für Monitoring
    metrics   *GuardianMetrics
    
    // Semaphore für Rate-Limiting
    sem       chan struct{}
}

// GuardianCheck repräsentiert eine zu prüfende Operation
type GuardianCheck struct {
    Type        CheckType       // Tool, DataExport, APIRequest, etc.
    Operation   string          // z.B. "execute_shell", "api_request"
    Parameters  map[string]string // Relevante Parameter
    Context     string          // Chat-Kontext (gekürzt)
    UserID      string          // Für User-spezifisches Profil
    Severity    SeverityLevel   // Vorgeschlagener Schweregrad
}

type CheckType string

const (
    CheckTool        CheckType = "tool"
    CheckDataExport  CheckType = "data_export"
    CheckAPIRequest  CheckType = "api_request"
    CheckFileAccess  CheckType = "file_access"
    CheckNetwork     CheckType = "network"
)

// GuardianResult enthält die Entscheidung des Wächters
type GuardianResult struct {
    Decision     Decision    // allow, block, quarantine
    RiskScore    float64     // 0.0 - 1.0
    Reason       string      // Kurze Begründung
    Confidence   float64     // Konfidenz der LLM-Entscheidung
    TokensUsed   int         // Für Kosten-Tracking
    Duration     time.Duration
    Cached       bool
}

type Decision string

const (
    DecisionAllow       Decision = "allow"
    DecisionBlock       Decision = "block"
    DecisionQuarantine  Decision = "quarantine" // Erlauben aber loggen & notify
)
```

### 2. Token-optimierte Prompts

```go
// Ultra-kompakte Prompts für günstige Modelle

const PromptGuardianToolCheck = `SECURITY AUDIT
Operation: {{.Operation}}
Params: {{.Params}}
Context: {{.Context}}

CLASSIFY: safe|suspicious|dangerous
Risk (0-100): 
Reason (5 words max):`

const PromptGuardianDataCheck = `DATA PROTECTION CHECK
Type: {{.DataType}}
Destination: {{.Destination}}
Content preview: {{.Preview}}

COMPLIANCE: compliant|risky|violation
Risk score: 
Action: allow|block|encrypt`

const PromptGuardianIntent = `INTENT ANALYSIS
User request: {{.Request}}
Tool chain: {{.Tools}}

INTENT: benign|unclear|malicious
Confidence (%): 
Red flags: `
```

### 3. Bereichs-spezifische Konfiguration

```yaml
# config.yaml - Guardian Konfiguration
guardian:
  enabled: true
  provider: "gemini-flash"  # Dedizierter Provider für Guardian
  
  # Globaler Level (Fallback)
  default_level: "medium"
  
  # Bereichs-spezifische Einstellungen
  domains:
    # System-Tools (höchste Gefahr)
    system_tools:
      level: "maximum"
      operations: ["execute_shell", "execute_sudo", "execute_python"]
      require_approval: true
      
    # Dateisystem
    filesystem:
      level: "high"
      operations: ["filesystem"]
      sensitive_paths: ["/etc", "/root", "/var/secrets"]
      max_file_size_mb: 10
      
    # Netzwerk/Externe APIs
    network:
      level: "medium"
      operations: ["api_request", "execute_remote_shell"]
      allowed_domains: ["api.github.com", "api.openai.com"]
      blocked_domains: ["localhost", "127.0.0.1", "169.254.*"]
      
    # Daten-Export
    data_export:
      level: "high"
      operations: ["webhooks", "api_request"]
      check_pii: true
      encryption_required: true
      
    # Container/Cloud
    infrastructure:
      level: "medium"
      operations: ["docker", "proxmox", "netlify"]
      production_protection: true
      
    # Allgemeine Tools (niedrigste Gefahr)
    safe_tools:
      level: "low"
      operations: ["query_memory", "manage_notes", "system_metrics"]
      fast_track: true  # Überspringe Guardian bei cleanem Regex-Check

  # Caching-Einstellungen
  cache:
    enabled: true
    ttl_seconds: 300
    max_entries: 1000
    
  # Rate Limiting
  rate_limit:
    max_checks_per_minute: 60
    burst_allowance: 10
    
  # Benachrichtigungen
  alerts:
    on_block: true
    on_quarantine: true
    webhook_url: ""  # Optional: externe Benachrichtigung
    
  # Token-Optimierung
  token_budget:
    max_context_length: 500  # Zeichen
    max_prompt_tokens: 200
    max_response_tokens: 50
```

### 4. Guardian Cache

```go
// Interner Cache für identische Prüfungen

type GuardianCache struct {
    mu      sync.RWMutex
    entries map[string]cacheEntry
    ttl     time.Duration
}

type cacheEntry struct {
    result    GuardianResult
    timestamp time.Time
}

func (c *GuardianCache) Get(key string) (GuardianResult, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, exists := c.entries[key]
    if !exists {
        return GuardianResult{}, false
    }
    
    if time.Since(entry.timestamp) > c.ttl {
        return GuardianResult{}, false // Expired
    }
    
    result := entry.result
    result.Cached = true
    return result, true
}

func (c *GuardianCache) Set(key string, result GuardianResult) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if len(c.entries) >= 1000 {
        // LRU eviction
        c.evictOldest()
    }
    
    c.entries[key] = cacheEntry{
        result:    result,
        timestamp: time.Now(),
    }
}

// Cache-Key aus Hash der Prüfparameter generieren
func generateCacheKey(check GuardianCheck) string {
    // Normalisiere und hashe
    data := fmt.Sprintf("%s:%s:%s", 
        check.Type, 
        check.Operation,
        hashParameters(check.Parameters))
    return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}
```

### 5. Risiko-Bewertungs-Engine

```go
// RiskScorer kombiniert mehrere Faktoren

type RiskScorer struct {
    weights RiskWeights
}

type RiskWeights struct {
    RegexThreat     float64 // 0.3
    LLMScore        float64 // 0.4
    OperationRisk   float64 // 0.2
    UserReputation  float64 // 0.1
}

func (rs *RiskScorer) Calculate(check GuardianCheck, llmResult GuardianResult) RiskAssessment {
    var score float64
    
    // 1. Regex-Bedrohung (bereits berechnet)
    regexScore := check.RegexThreatLevel * rs.weights.RegexThreat
    
    // 2. LLM Bewertung
    llmScore := llmResult.RiskScore * rs.weights.LLMScore
    
    // 3. Operations-Risiko (statische Tabelle)
    opRisk := getOperationRisk(check.Operation) * rs.weights.OperationRisk
    
    // 4. User-Reputation (aus History)
    userRisk := getUserRiskScore(check.UserID) * rs.weights.UserReputation
    
    totalScore := regexScore + llmScore + opRisk + userRisk
    
    return RiskAssessment{
        Score:       totalScore,
        Level:       scoreToLevel(totalScore),
        Components:  map[string]float64{
            "regex": regexScore,
            "llm": llmScore,
            "operation": opRisk,
            "user": userRisk,
        },
    }
}

func getOperationRisk(operation string) float64 {
    risks := map[string]float64{
        "execute_shell":      0.9,
        "execute_sudo":       1.0,
        "execute_python":     0.7,
        "filesystem":         0.6,
        "api_request":        0.5,
        "execute_remote_shell": 0.8,
        "docker":             0.6,
        "query_memory":       0.1,
        "manage_notes":       0.1,
    }
    if r, ok := risks[operation]; ok {
        return r
    }
    return 0.5 // Default
}
```

---

## Implementierungs-Workflow

### Integration in Agent Loop

```go
// internal/agent/agent_loop.go - Integration

func ExecuteAgentLoop(...) {
    // ... bestehender Code ...
    
    // Guardian Integration vor Tool-Ausführung
    for _, tc := range toolCalls {
        // 1. Regex-Check (schnell, kostenlos)
        scan := guardian.ScanForInjection(tc.RawJSON)
        
        // 2. LLM Guardian Check (nur wenn nötig und aktiviert)
        if shouldUseLLMGuardian(tc, scan, cfg) {
            check := GuardianCheck{
                Type:       CheckTool,
                Operation:  tc.Action,
                Parameters: extractRelevantParams(tc),
                Context:    truncateContext(lastUserMessage, 200),
                Severity:   mapThreatToSeverity(scan.Level),
            }
            
            result, err := llmGuardian.Evaluate(ctx, check)
            if err != nil {
                logger.Error("Guardian check failed", "error", err)
                // Fail-safe: Blockieren bei Guardian-Fehler (optional konfigurierbar)
                if cfg.Guardian.FailSafe {
                    broker.Send("guardian_block", "Sicherheitsprüfung fehlgeschlagen")
                    continue
                }
            }
            
            // Entscheidung verarbeiten
            switch result.Decision {
            case DecisionBlock:
                broker.Send("guardian_block", fmt.Sprintf("Operation blockiert: %s", result.Reason))
                logSecurityEvent(tc, result)
                continue // Tool nicht ausführen
                
            case DecisionQuarantine:
                broker.Send("guardian_warning", fmt.Sprintf("⚠️ Riskante Operation: %s", result.Reason))
                // Führe aus aber mit erhöhter Überwachung
                
            case DecisionAllow:
                // Normal fortfahren
            }
        }
        
        // Tool ausführen
        result := DispatchToolCall(ctx, tc, ...)
    }
}

func shouldUseLLMGuardian(tc ToolCall, scan ScanResult, cfg *config.Config) bool {
    // Guardian deaktiviert?
    if !cfg.Guardian.Enabled {
        return false
    }
    
    // Schneller Pfad: Bei niedrigem Regex-Risk und "fast_track" Domain
    domain := getDomainForTool(tc.Action, cfg.Guardian)
    if domain.FastTrack && scan.Level <= ThreatLow {
        return false
    }
    
    // Immer prüfen bei kritischen Operationen
    if domain.Level >= GuardianHigh {
        return true
    }
    
    // Bei verdächtigem Regex-Ergebnis
    if scan.Level >= ThreatMedium {
        return true
    }
    
    return domain.Level > GuardianOff
}
```

---

## Token-Optimierungs-Strategien

### 1. Kontext-Kompression

```go
func truncateContext(content string, maxLen int) string {
    if len(content) <= maxLen {
        return content
    }
    
    // Extrahiere Schlüsselwörter statt einfachem Truncating
    keywords := extractKeywords(content, 10)
    return strings.Join(keywords, " | ")
}

func extractKeywords(text string, count int) []string {
    // Einfache TF-IDF oder Keyword-Extraktion
    // Priorisiere: Nomen, Verben, technische Begriffe
    words := tokenize(text)
    freq := make(map[string]int)
    
    for _, w := range words {
        if len(w) > 3 && !isStopWord(w) {
            freq[w]++
        }
    }
    
    // Top N zurückgeben
    return topN(freq, count)
}
```

### 2. Prompt Templates

```go
var guardianPrompts = map[string]string{
    "tool_check": `AUDIT {{.Op}} {{.Params}} CTX:{{.Context}} RISK:?`,
    
    "data_check": `DATA {{.Type}} DST:{{.Dest}} SZ:{{.Size}} SEC:?`,
    
    "intent_check": `INTENT {{.Request}} T:{{.Tools}} MAL:?`,
}
```

### 3. Response Parsing

```go
// Parse minimalistische Antworten
func parseGuardianResponse(response string) GuardianResult {
    response = strings.ToLower(strings.TrimSpace(response))
    
    // Erwartetes Format: "risky 85 exposes system files"
    parts := strings.Fields(response)
    if len(parts) < 2 {
        return fallbackResult()
    }
    
    decision := parseDecision(parts[0])
    score, _ := strconv.ParseFloat(parts[1], 64)
    reason := strings.Join(parts[2:], " ")
    
    return GuardianResult{
        Decision:   decision,
        RiskScore:  score / 100.0,
        Reason:     reason,
        Confidence: 0.9,
    }
}
```

---

## Kosten-Optimierung

### Geschätzte Kosten pro Check

| Modell | Input Tokens | Output Tokens | Kosten/Check | Latenz |
|--------|-------------|---------------|--------------|--------|
| Gemini 1.5 Flash | 150 | 20 | $0.00005 | 200ms |
| GPT-4o-mini | 150 | 20 | $0.000075 | 300ms |
| Local (Phi-3 3B) | 150 | 20 | $0 | 500ms |

### Mit Caching

- Cache Hit Rate: ~60% (geschätzt)
- Effektive Kosten: 40% der Basis-Kosten
- Durchschnittskosten pro Session: <$0.01

### Budget-Limits

```yaml
guardian:
  budget_limits:
    max_checks_per_hour: 100
    max_cost_per_day_usd: 0.50
    emergency_stop_on_budget: true
```

---

## Konfigurations-Beispiele

### Szenario 1: Home Lab (Preiswert)

```yaml
guardian:
  enabled: true
  provider: "local-ollama"  # Oder Gemini Flash
  
  domains:
    system_tools:
      level: "high"         # Shell/Python prüfen
    filesystem:
      level: "medium"       # Nur Schreibzugriffe
    network:
      level: "off"          # Home LAN ist sicher
      
  cache:
    ttl_seconds: 600        # Länger cachen (weniger Kosten)
```

### Szenario 2: Enterprise (Streng)

```yaml
guardian:
  enabled: true
  provider: "gpt-4o-mini"
  default_level: "high"
  
  domains:
    system_tools:
      level: "maximum"
      require_approval: true  # Menschliche Bestätigung
      
    data_export:
      level: "maximum"
      check_pii: true
      encryption_required: true
      
    network:
      level: "high"
      blocked_domains: ["*internal*", "*.local"]
      
  alerts:
    on_block: true
    webhook_url: "https://security.company.com/alerts"
```

### Szenario 3: Performance-kritisch

```yaml
guardian:
  enabled: true
  
  domains:
    safe_tools:
      level: "low"
      fast_track: true      # Überspringe LLM bei cleanem Regex
      
    system_tools:
      level: "high"
      
  # Async Prüfung wo möglich
  async_mode: true
```

---

## Monitoring & Observability

```go
type GuardianMetrics struct {
    ChecksTotal       prometheus.Counter
    ChecksCached      prometheus.Counter
    ChecksBlocked     prometheus.Counter
    ChecksQuarantined prometheus.Counter
    LatencyHistogram  prometheus.Histogram
    TokenUsage        prometheus.Counter
    CostUSD           prometheus.Counter
}

func (m *GuardianMetrics) Record(check GuardianCheck, result GuardianResult) {
    m.ChecksTotal.Inc()
    
    if result.Cached {
        m.ChecksCached.Inc()
    }
    
    switch result.Decision {
    case DecisionBlock:
        m.ChecksBlocked.Inc()
    case DecisionQuarantine:
        m.ChecksQuarantined.Inc()
    }
    
    m.LatencyHistogram.Observe(result.Duration.Seconds())
    m.TokenUsage.Add(float64(result.TokensUsed))
    m.CostUSD.Add(calculateCost(result.TokensUsed))
}
```

---

## Fail-Safe Verhalten

```go
func (g *LLMGuardian) EvaluateWithFailSafe(ctx context.Context, check GuardianCheck) GuardianResult {
    // Timeout-Kontext
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    result, err := g.Evaluate(ctx, check)
    if err != nil {
        g.logger.Error("Guardian evaluation failed", "error", err, "check", check)
        
        // Konfigurierbares Fail-Safe
        switch g.cfg.FailSafeBehavior {
        case "block":
            return GuardianResult{
                Decision:   DecisionBlock,
                RiskScore:  1.0,
                Reason:     "Guardian unavailable - fail safe blocked",
                Confidence: 1.0,
            }
        case "allow":
            return GuardianResult{
                Decision:   DecisionAllow,
                RiskScore:  0.0,
                Reason:     "Guardian unavailable - fail safe allowed",
                Confidence: 0.0,
            }
        case "quarantine":
            return GuardianResult{
                Decision:   DecisionQuarantine,
                RiskScore:  0.5,
                Reason:     "Guardian unavailable - quarantined",
                Confidence: 0.5,
            }
        }
    }
    
    return result
}
```

---

## Zusammenfassung

Das LLM Guardian System bietet:

✅ **Erhöhte Sicherheit** durch semantische Analyse  
✅ **Kosteneffizienz** durch Token-Optimierung und Caching  
✅ **Flexibilität** durch bereichs-spezifische Konfiguration  
✅ **Performance** durch schnelle, preiswerte Modelle  
✅ **Transparenz** durch detailliertes Logging und Metriken  

**Empfohlener Einstieg:**
1. Starte mit `level: "medium"` für system_tools
2. Nutze Gemini Flash oder lokalen Ollama (3B)
3. Aktiviere Caching (TTL: 5 Minuten)
4. Überwache Metrics und passe an

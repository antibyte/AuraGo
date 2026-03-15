# LLM Guardian - Implementierungs-Guide

## Schnelle Integration (Step-by-Step)

### Step 1: Konfiguration erweitern

```go
// internal/config/config_types.go

// Am Ende der Config struct hinzufügen:
type Config struct {
    // ... bestehende Felder ...
    
    Guardian GuardianConfig `yaml:"guardian"`
}

type GuardianConfig struct {
    Enabled        bool                   `yaml:"enabled"`
    Provider       string                 `yaml:"provider"`        // Provider ID
    DefaultLevel   string                 `yaml:"default_level"`   // low, medium, high, maximum
    
    Domains        map[string]GuardianDomain `yaml:"domains"`
    Cache          GuardianCacheConfig    `yaml:"cache"`
    RateLimit      GuardianRateLimit      `yaml:"rate_limit"`
    Alerts         GuardianAlertConfig    `yaml:"alerts"`
    FailSafe       string                 `yaml:"fail_safe"`       // block, allow, quarantine
    BudgetLimits   GuardianBudget         `yaml:"budget_limits"`
}

type GuardianDomain struct {
    Level               string   `yaml:"level"`                // off, low, medium, high, maximum
    Operations          []string `yaml:"operations"`
    RequireApproval     bool     `yaml:"require_approval"`
    FastTrack           bool     `yaml:"fast_track"`
    SensitivePaths      []string `yaml:"sensitive_paths,omitempty"`
    AllowedDomains      []string `yaml:"allowed_domains,omitempty"`
    BlockedDomains      []string `yaml:"blocked_domains,omitempty"`
    CheckPII            bool     `yaml:"check_pii,omitempty"`
    EncryptionRequired  bool     `yaml:"encryption_required,omitempty"`
}

type GuardianCacheConfig struct {
    Enabled     bool          `yaml:"enabled"`
    TTLSeconds  int           `yaml:"ttl_seconds"`
    MaxEntries  int           `yaml:"max_entries"`
}

type GuardianRateLimit struct {
    MaxPerMinute   int `yaml:"max_checks_per_minute"`
    BurstAllowance int `yaml:"burst_allowance"`
}

type GuardianAlertConfig struct {
    OnBlock     bool   `yaml:"on_block"`
    OnQuarantine bool  `yaml:"on_quarantine"`
    WebhookURL  string `yaml:"webhook_url"`
}

type GuardianBudget struct {
    MaxPerHour      int     `yaml:"max_checks_per_hour"`
    MaxCostPerDay   float64 `yaml:"max_cost_per_day_usd"`
    EmergencyStop   bool    `yaml:"emergency_stop_on_budget"`
}
```

### Step 2: LLM Guardian Service erstellen

```go
// internal/security/llm_guardian.go

package security

import (
    "context"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "log/slog"
    "strings"
    "sync"
    "text/template"
    "time"
    
    "aurago/internal/config"
    "aurago/internal/llm"
    
    openai "github.com/sashabaranov/go-openai"
)

// LLMGuardian implementiert LLM-basierte Sicherheitsprüfungen
type LLMGuardian struct {
    cfg     *config.GuardianConfig
    logger  *slog.Logger
    client  llm.ChatClient
    cache   *GuardianCache
    metrics *GuardianMetrics
    sem     chan struct{}
    
    promptTemplates map[string]*template.Template
}

// NewLLMGuardian erstellt eine neue Guardian-Instanz
func NewLLMGuardian(cfg *config.GuardianConfig, logger *slog.Logger, client llm.ChatClient) (*LLMGuardian, error) {
    if !cfg.Enabled {
        return nil, nil
    }
    
    g := &LLMGuardian{
        cfg:     cfg,
        logger:  logger,
        client:  client,
        cache:   NewGuardianCache(time.Duration(cfg.Cache.TTLSeconds) * time.Second),
        metrics: NewGuardianMetrics(),
        sem:     make(chan struct{}, cfg.RateLimit.MaxPerMinute),
        promptTemplates: make(map[string]*template.Template),
    }
    
    // Templates kompilieren
    if err := g.compileTemplates(); err != nil {
        return nil, fmt.Errorf("failed to compile templates: %w", err)
    }
    
    return g, nil
}

// compileTemplates erstellt die token-optimierten Prompts
func (g *LLMGuardian) compileTemplates() error {
    templates := map[string]string{
        "tool_check": `SECURITY AUDIT
Operation: {{.Operation}}
Params: {{.Params}}
Risk: {{if .HighRisk}}HIGH{{else}}NORMAL{{end}}
Verdict:`,
        
        "data_check": `DATA CHECK
Type: {{.DataType}}
Dest: {{.Destination}}
PII: {{.HasPII}}
Allow:`,
        
        "intent_check": `INTENT
Req: {{.Request}}
Chain: {{.ToolChain}}
Malicious:`,
    }
    
    for name, tmpl := range templates {
        t, err := template.New(name).Parse(tmpl)
        if err != nil {
            return err
        }
        g.promptTemplates[name] = t
    }
    
    return nil
}

// Evaluate führt die Sicherheitsprüfung durch
func (g *LLMGuardian) Evaluate(ctx context.Context, check GuardianCheck) (GuardianResult, error) {
    start := time.Now()
    
    // 1. Rate Limiting
    select {
    case g.sem <- struct{}{}:
        defer func() { <-g.sem }()
    case <-ctx.Done():
        return GuardianResult{}, ctx.Err()
    case <-time.After(time.Second):
        return GuardianResult{}, fmt.Errorf("rate limit exceeded")
    }
    
    // 2. Cache-Check
    cacheKey := g.generateCacheKey(check)
    if cached, ok := g.cache.Get(cacheKey); ok {
        g.logger.Debug("Guardian cache hit", "operation", check.Operation)
        return cached, nil
    }
    
    // 3. LLM-Anfrage
    prompt, err := g.buildPrompt(check)
    if err != nil {
        return GuardianResult{}, err
    }
    
    // Kompakte LLM-Anfrage
    req := openai.ChatCompletionRequest{
        Model:       g.cfg.Provider, // z.B. "gemini-1.5-flash"
        Temperature: 0.1,            // Sehr deterministisch
        MaxTokens:   20,             // Kurze Antwort erzwingen
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleSystem, Content: "Security auditor. Respond with: SAFE|SUSPICIOUS|DANGEROUS score(0-100) brief_reason"},
            {Role: openai.ChatMessageRoleUser, Content: prompt},
        },
    }
    
    resp, err := g.client.CreateChatCompletion(ctx, req)
    if err != nil {
        return g.handleFailSafe(err)
    }
    
    if len(resp.Choices) == 0 {
        return g.handleFailSafe(fmt.Errorf("empty response"))
    }
    
    // 4. Antwort parsen
    result := g.parseResponse(resp.Choices[0].Message.Content)
    result.Duration = time.Since(start)
    result.TokensUsed = resp.Usage.TotalTokens
    
    // 5. Cachen
    g.cache.Set(cacheKey, result)
    
    // 6. Metriken
    g.metrics.Record(check, result)
    
    return result, nil
}

// buildPrompt erstellt den token-optimierten Prompt
func (g *LLMGuardian) buildPrompt(check GuardianCheck) (string, error) {
    var tmplName string
    var data interface{}
    
    switch check.Type {
    case CheckTool:
        tmplName = "tool_check"
        data = struct {
            Operation string
            Params    string
            HighRisk  bool
        }{
            Operation: check.Operation,
            Params:    g.truncateParams(check.Parameters),
            HighRisk:  check.Severity >= SeverityHigh,
        }
        
    case CheckDataExport:
        tmplName = "data_check"
        data = struct {
            DataType string
            Destination string
            HasPII   bool
        }{
            DataType:    check.DataType,
            Destination: check.Destination,
            HasPII:      check.ContainsPII,
        }
        
    default:
        tmplName = "intent_check"
        data = struct {
            Request  string
            ToolChain string
        }{
            Request:   g.truncate(check.Context, 100),
            ToolChain: strings.Join(check.ToolChain, ","),
        }
    }
    
    var buf strings.Builder
    if err := g.promptTemplates[tmplName].Execute(&buf, data); err != nil {
        return "", err
    }
    
    return buf.String(), nil
}

// parseResponse interpretiert die LLM-Antwort
func (g *LLMGuardian) parseResponse(response string) GuardianResult {
    response = strings.ToUpper(strings.TrimSpace(response))
    parts := strings.Fields(response)
    
    if len(parts) < 2 {
        // Fallback: Nicht erkannt, als verdächtig behandeln
        return GuardianResult{
            Decision:   DecisionQuarantine,
            RiskScore:  0.5,
            Reason:     "unclear response",
            Confidence: 0.3,
        }
    }
    
    // Parse: "SUSPICIOUS 85 exposes system files"
    decision := g.parseDecision(parts[0])
    score, _ := parseFloat(parts[1])
    reason := ""
    if len(parts) > 2 {
        reason = strings.Join(parts[2:], " ")
    }
    
    return GuardianResult{
        Decision:   decision,
        RiskScore:  score / 100.0,
        Reason:     reason,
        Confidence: 0.85,
    }
}

func (g *LLMGuardian) parseDecision(s string) Decision {
    switch {
    case strings.Contains(s, "SAFE"):
        return DecisionAllow
    case strings.Contains(s, "DANGEROUS"), strings.Contains(s, "BLOCK"):
        return DecisionBlock
    default:
        return DecisionQuarantine
    }
}

// truncateParams kürzt Parameter für den Prompt
func (g *LLMGuardian) truncateParams(params map[string]string) string {
    var parts []string
    for k, v := range params {
        if k == "password" || k == "api_key" || k == "token" {
            parts = append(parts, fmt.Sprintf("%s:***", k))
        } else {
            parts = append(parts, fmt.Sprintf("%s:%s", k, g.truncate(v, 30)))
        }
    }
    return strings.Join(parts, "|")
}

func (g *LLMGuardian) truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}

// generateCacheKey erstellt einen Cache-Schlüssel
func (g *LLMGuardian) generateCacheKey(check GuardianCheck) string {
    // Normalisiere: lowercase, nur relevante Felder
    data := fmt.Sprintf("%s|%s|%s|%v",
        check.Type,
        check.Operation,
        g.normalizeParams(check.Parameters),
        check.Severity,
    )
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%x", hash[:8]) // Nur erste 8 Bytes = 16 hex chars
}

func (g *LLMGuardian) normalizeParams(params map[string]string) string {
    // Sortiere Keys für konsistente Hashing
    var parts []string
    for k := range params {
        parts = append(parts, k)
    }
    // Einfache Normalisierung
    return strings.Join(parts, ",")
}

// handleFailSafe behandelt Fehler sicher
func (g *LLMGuardian) handleFailSafe(err error) (GuardianResult, error) {
    g.logger.Error("Guardian evaluation failed", "error", err)
    
    switch g.cfg.FailSafe {
    case "block":
        return GuardianResult{
            Decision:   DecisionBlock,
            RiskScore:  1.0,
            Reason:     "fail-safe block",
            Confidence: 1.0,
        }, nil
    case "allow":
        return GuardianResult{
            Decision:   DecisionAllow,
            RiskScore:  0.0,
            Reason:     "fail-safe allow",
            Confidence: 0.0,
        }, nil
    default: // quarantine
        return GuardianResult{
            Decision:   DecisionQuarantine,
            RiskScore:  0.5,
            Reason:     "fail-safe quarantine",
            Confidence: 0.5,
        }, nil
    }
}

// Helper-Funktionen
type GuardianCheck struct {
    Type        CheckType
    Operation   string
    Parameters  map[string]string
    Context     string
    Severity    SeverityLevel
    DataType    string
    Destination string
    ContainsPII bool
    ToolChain   []string
}

type GuardianResult struct {
    Decision     Decision
    RiskScore    float64
    Reason       string
    Confidence   float64
    TokensUsed   int
    Duration     time.Duration
    Cached       bool
}

type Decision string

const (
    DecisionAllow       Decision = "allow"
    DecisionBlock       Decision = "block"
    DecisionQuarantine Decision = "quarantine"
)

type CheckType string

const (
    CheckTool       CheckType = "tool"
    CheckDataExport CheckType = "data_export"
    CheckAPIRequest CheckType = "api_request"
)

type SeverityLevel int

const (
    SeverityLow SeverityLevel = iota
    SeverityMedium
    SeverityHigh
    SeverityCritical
)
```

### Step 3: Cache-Implementierung

```go
// internal/security/guardian_cache.go

package security

import (
    "sync"
    "time"
)

// GuardianCache implementiert einen TTL-basierten Cache
type GuardianCache struct {
    mu      sync.RWMutex
    entries map[string]cacheEntry
    ttl     time.Duration
    maxSize int
}

type cacheEntry struct {
    result    GuardianResult
    timestamp time.Time
    accessCount int
}

// NewGuardianCache erstellt einen neuen Cache
func NewGuardianCache(ttl time.Duration) *GuardianCache {
    cache := &GuardianCache{
        entries: make(map[string]cacheEntry),
        ttl:     ttl,
        maxSize: 1000,
    }
    
    // Cleanup Goroutine
    go cache.cleanupRoutine()
    
    return cache
}

// Get holt ein Ergebnis aus dem Cache
func (c *GuardianCache) Get(key string) (GuardianResult, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, exists := c.entries[key]
    if !exists {
        return GuardianResult{}, false
    }
    
    if time.Since(entry.timestamp) > c.ttl {
        return GuardianResult{}, false
    }
    
    // Update access count (async)
    entry.accessCount++
    
    result := entry.result
    result.Cached = true
    return result, true
}

// Set speichert ein Ergebnis im Cache
func (c *GuardianCache) Set(key string, result GuardianResult) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // LRU Eviction wenn nötig
    if len(c.entries) >= c.maxSize {
        c.evictLRU()
    }
    
    c.entries[key] = cacheEntry{
        result:      result,
        timestamp:   time.Now(),
        accessCount: 1,
    }
}

// evictLRU entfernt den am seltensten genutzten Eintrag
func (c *GuardianCache) evictLRU() {
    var oldestKey string
    var oldestCount int = int(^uint(0) >> 1) // MaxInt
    
    for key, entry := range c.entries {
        if entry.accessCount < oldestCount {
            oldestCount = entry.accessCount
            oldestKey = key
        }
    }
    
    if oldestKey != "" {
        delete(c.entries, oldestKey)
    }
}

// cleanupRoutine entfernt abgelaufene Einträge
func (c *GuardianCache) cleanupRoutine() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        c.mu.Lock()
        now := time.Now()
        for key, entry := range c.entries {
            if now.Sub(entry.timestamp) > c.ttl {
                delete(c.entries, key)
            }
        }
        c.mu.Unlock()
    }
}
```

### Step 4: Integration in Agent Loop

```go
// internal/agent/agent_loop.go

// Erweitere RunConfig
type RunConfig struct {
    // ... bestehende Felder ...
    LLMGuardian *security.LLMGuardian
}

// Im ExecuteAgentLoop:
func ExecuteAgentLoop(ctx context.Context, req openai.ChatCompletionRequest, runCfg RunConfig, ...) {
    // ... Setup ...
    
    for {
        // ... Loop-Header ...
        
        // Tool Calls verarbeiten
        if tc.IsTool {
            // 1. Regex Guardian (bestehend)
            scan := guardian.ScanForInjection(tc.RawJSON)
            
            // 2. LLM Guardian (neu, optional)
            if runCfg.LLMGuardian != nil && shouldCheckWithLLM(tc, scan, runCfg.Config) {
                check := security.GuardianCheck{
                    Type:       security.CheckTool,
                    Operation:  tc.Action,
                    Parameters: map[string]string{
                        "command": tc.Command,
                        "path": tc.FilePath,
                        "url": tc.URL,
                    },
                    Context:   truncate(lastUserMsg, 150),
                    Severity:  mapToSeverity(scan.Level),
                }
                
                result, err := runCfg.LLMGuardian.Evaluate(ctx, check)
                if err != nil {
                    logger.Error("Guardian check failed", "error", err)
                    // Fail-safe: Blockieren
                    broker.Send("guardian_error", "Sicherheitsprüfung fehlgeschlagen")
                    continue
                }
                
                // Entscheidung behandeln
                switch result.Decision {
                case security.DecisionBlock:
                    broker.Send("guardian_block", map[string]interface{}{
                        "operation": tc.Action,
                        "reason": result.Reason,
                        "risk_score": result.RiskScore,
                    })
                    
                    // Loggen
                    logger.Warn("Operation blocked by Guardian",
                        "operation", tc.Action,
                        "reason", result.Reason,
                        "risk", result.RiskScore,
                    )
                    
                    // Antwort an LLM: Operation blockiert
                    blockMsg := fmt.Sprintf("[SECURITY] Operation '%s' was blocked by security guardian: %s (risk: %.0f%%)",
                        tc.Action, result.Reason, result.RiskScore*100)
                    
                    req.Messages = append(req.Messages, openai.ChatCompletionMessage{
                        Role:    openai.ChatMessageRoleSystem,
                        Content: blockMsg,
                    })
                    continue // Nächster Tool Call
                    
                case security.DecisionQuarantine:
                    broker.Send("guardian_warning", map[string]interface{}{
                        "operation": tc.Action,
                        "reason": result.Reason,
                    })
                    // Fortfahren aber mit Warnung im Kontext
                    
                case security.DecisionAllow:
                    // Normal fortfahren
                }
            }
            
            // Tool ausführen
            result := DispatchToolCall(ctx, tc, ...)
            // ...
        }
    }
}

// Hilfsfunktionen
func shouldCheckWithLLM(tc ToolCall, scan security.ScanResult, cfg *config.Config) bool {
    if !cfg.Guardian.Enabled {
        return false
    }
    
    // Hole Domain-Konfiguration für dieses Tool
    domain := getDomainForTool(tc.Action, cfg.Guardian)
    
    // Bei kritischem Regex-Ergebnis immer prüfen
    if scan.Level >= security.ThreatHigh {
        return true
    }
    
    // Bei "Fast Track" und niedrigem Risiko überspringen
    if domain.FastTrack && scan.Level <= security.ThreatLow {
        return false
    }
    
    // Prüfe Level
    switch domain.Level {
    case "maximum":
        return true
    case "high":
        return scan.Level >= security.ThreatLow
    case "medium":
        return scan.Level >= security.ThreatMedium
    case "low":
        return scan.Level >= security.ThreatHigh
    default:
        return false
    }
}

func mapToSeverity(level security.ThreatLevel) security.SeverityLevel {
    switch level {
    case security.ThreatCritical:
        return security.SeverityCritical
    case security.ThreatHigh:
        return security.SeverityHigh
    case security.ThreatMedium:
        return security.SeverityMedium
    default:
        return security.SeverityLow
    }
}

func getDomainForTool(toolName string, guardian config.GuardianConfig) config.GuardianDomain {
    // Suche in Domains
    for _, domain := range guardian.Domains {
        for _, op := range domain.Operations {
            if op == toolName {
                return domain
            }
        }
    }
    
    // Default
    return config.GuardianDomain{
        Level: guardian.DefaultLevel,
    }
}
```

### Step 5: Initialisierung

```go
// internal/agent/agent.go

func InitializeAgent(cfg *config.Config, logger *slog.Logger) (*Agent, error) {
    // ... bestehende Initialisierung ...
    
    // LLM Guardian initialisieren (optional)
    var llmGuardian *security.LLMGuardian
    if cfg.Guardian.Enabled {
        // Erstelle dedizierten LLM-Client für Guardian
        // (kann anderer Provider sein als Haupt-LLM)
        guardianClient, err := llm.NewClient(llm.ClientConfig{
            Provider: cfg.Guardian.Provider,
            // ... weitere Config
        })
        if err != nil {
            logger.Error("Failed to create Guardian LLM client", "error", err)
            // Nicht-fatal: Agent funktioniert ohne Guardian
        } else {
            llmGuardian, err = security.NewLLMGuardian(&cfg.Guardian, logger, guardianClient)
            if err != nil {
                logger.Error("Failed to initialize Guardian", "error", err)
            } else {
                logger.Info("LLM Guardian initialized", 
                    "provider", cfg.Guardian.Provider,
                    "default_level", cfg.Guardian.DefaultLevel)
            }
        }
    }
    
    return &Agent{
        // ... bestehende Felder ...
        LLMGuardian: llmGuardian,
    }, nil
}
```

---

## Test-Beispiele

```go
// internal/security/llm_guardian_test.go

func TestLLMGuardian_Evaluate(t *testing.T) {
    cfg := &config.GuardianConfig{
        Enabled:      true,
        Provider:     "test",
        DefaultLevel: "medium",
        FailSafe:     "quarantine",
        Cache: config.GuardianCacheConfig{
            Enabled:    true,
            TTLSeconds: 300,
        },
    }
    
    mockClient := &mockLLMClient{
        response: "SUSPICIOUS 75 suspicious file access pattern",
    }
    
    guardian, err := NewLLMGuardian(cfg, slog.Default(), mockClient)
    if err != nil {
        t.Fatal(err)
    }
    
    check := GuardianCheck{
        Type:      CheckTool,
        Operation: "filesystem",
        Parameters: map[string]string{
            "operation": "read_file",
            "file_path": "/etc/passwd",
        },
        Severity: SeverityMedium,
    }
    
    result, err := guardian.Evaluate(context.Background(), check)
    if err != nil {
        t.Fatal(err)
    }
    
    if result.Decision != DecisionQuarantine {
        t.Errorf("Expected quarantine, got %s", result.Decision)
    }
    
    if result.RiskScore < 0.7 {
        t.Errorf("Expected high risk score, got %f", result.RiskScore)
    }
    
    // Test Cache
    result2, _ := guardian.Evaluate(context.Background(), check)
    if !result2.Cached {
        t.Error("Expected cached result")
    }
}
```

---

## Performance-Benchmarks

```bash
# Benchmark mit verschiedenen Modellen

go test -bench=BenchmarkGuardian -benchmem

# Erwartete Ergebnisse:
# Gemini Flash:    ~150ms,  ~170 tokens
# GPT-4o-mini:     ~250ms,  ~170 tokens  
# Local (Ollama):  ~600ms,  ~170 tokens (kostenlos)
```

---

## Zusammenfassung Integration

1. **Config erweitern** → Neue `GuardianConfig` Struktur
2. **Service erstellen** → `llm_guardian.go` mit Evaluate-Methode
3. **Cache hinzufügen** → `guardian_cache.go` für Performance
4. **Agent Loop** → Prüfung vor Tool-Ausführung
5. **Initialisierung** → Optionaler Start mit Fehler-Toleranz

**Wichtig:** Das System ist vollständig optional und deaktivierbar. Bei Fehlern fällt es graceful zurück auf den bestehenden Regex-Guardian.

package security

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"aurago/internal/config"
	"aurago/internal/llm"
)

// GuardianLevel defines the protection intensity.
type GuardianLevel int

const (
	GuardianOff     GuardianLevel = iota // No LLM checks
	GuardianLow                          // Only high-risk tools
	GuardianMedium                       // All tools + external APIs
	GuardianHigh                         // Every tool call checked
)

// Decision represents the guardian's verdict.
type Decision string

const (
	DecisionAllow      Decision = "allow"
	DecisionBlock      Decision = "block"
	DecisionQuarantine Decision = "quarantine"
)

// GuardianCheck represents an operation to evaluate.
type GuardianCheck struct {
	Operation  string            // tool name (e.g. "execute_shell")
	Parameters map[string]string // relevant parameters for evaluation
	Context    string            // truncated user message or chat context
	RegexLevel ThreatLevel       // pre-computed regex threat level
}

// GuardianResult contains the guardian's decision and metadata.
type GuardianResult struct {
	Decision   Decision      `json:"decision"`
	RiskScore  float64       `json:"risk_score"`
	Reason     string        `json:"reason"`
	TokensUsed int           `json:"tokens_used"`
	Duration   time.Duration `json:"duration"`
	Cached     bool          `json:"cached"`
}

// LLMGuardian evaluates tool calls using a dedicated LLM before execution.
type LLMGuardian struct {
	cfg     *config.Config
	logger  *slog.Logger
	client  *openai.Client
	model   string
	cache   *GuardianCache
	Metrics *GuardianMetrics
	sem     chan struct{} // rate-limiting semaphore
}

// NewLLMGuardian creates a new LLM Guardian from config.
// Returns nil if guardian is disabled.
func NewLLMGuardian(cfg *config.Config, logger *slog.Logger) *LLMGuardian {
	if !cfg.LLMGuardian.Enabled {
		return nil
	}

	client := llm.NewClientFromProvider(
		cfg.LLMGuardian.ProviderType,
		cfg.LLMGuardian.BaseURL,
		cfg.LLMGuardian.APIKey,
	)

	// Rate limiter: buffer = max checks per minute
	maxChecks := cfg.LLMGuardian.MaxChecksPerMin
	if maxChecks <= 0 {
		maxChecks = 60
	}

	return &LLMGuardian{
		cfg:     cfg,
		logger:  logger,
		client:  client,
		model:   cfg.LLMGuardian.ResolvedModel,
		cache:   NewGuardianCache(cfg.LLMGuardian.CacheTTL, 1000),
		Metrics: &GuardianMetrics{},
		sem:     make(chan struct{}, maxChecks),
	}
}

// ShouldCheck determines whether a tool call needs LLM guardian evaluation
// based on the configured level, tool overrides, and regex scan result.
func (g *LLMGuardian) ShouldCheck(toolName string, regexLevel ThreatLevel) bool {
	if g == nil {
		return false
	}

	level := g.resolveLevel(toolName)
	if level == GuardianOff {
		return false
	}

	// Always check if regex flagged something suspicious
	if regexLevel >= ThreatMedium {
		return true
	}

	switch level {
	case GuardianHigh:
		return true // check everything
	case GuardianMedium:
		return isRiskyTool(toolName)
	case GuardianLow:
		return isHighRiskTool(toolName)
	default:
		return false
	}
}

// Evaluate runs the LLM security check on a tool call.
// Returns the decision, which may be cached.
func (g *LLMGuardian) Evaluate(ctx context.Context, check GuardianCheck) GuardianResult {
	start := time.Now()

	// Check cache first
	cacheKey := GenerateCacheKey(check.Operation, check.Parameters)
	if result, hit := g.cache.Get(cacheKey); hit {
		result.Duration = time.Since(start)
		g.Metrics.Record(result)
		g.logger.Debug("[Guardian] Cache hit", "operation", check.Operation, "decision", result.Decision)
		return result
	}

	// Rate limiting: try to acquire semaphore
	select {
	case g.sem <- struct{}{}:
		defer func() { <-g.sem }()
	default:
		g.logger.Warn("[Guardian] Rate limit exceeded, applying fail-safe")
		g.Metrics.RecordError()
		return g.failSafeResult(start, "rate limit exceeded")
	}

	// Build prompt & call LLM
	result := g.callLLM(ctx, check, start)
	g.cache.Set(cacheKey, result)
	g.Metrics.Record(result)
	return result
}

// EvaluateWithFailSafe wraps Evaluate with timeout and error recovery.
func (g *LLMGuardian) EvaluateWithFailSafe(ctx context.Context, check GuardianCheck) GuardianResult {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return g.Evaluate(ctx, check)
}

func (g *LLMGuardian) callLLM(ctx context.Context, check GuardianCheck, start time.Time) GuardianResult {
	prompt := buildGuardianPrompt(check)

	req := openai.ChatCompletionRequest{
		Model: g.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: guardianSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   50,
		Temperature: 0,
	}

	resp, err := g.client.CreateChatCompletion(ctx, req)
	if err != nil {
		g.logger.Warn("[Guardian] LLM call failed", "error", err, "operation", check.Operation)
		g.Metrics.RecordError()
		return g.failSafeResult(start, fmt.Sprintf("LLM error: %v", err))
	}

	if len(resp.Choices) == 0 {
		g.Metrics.RecordError()
		return g.failSafeResult(start, "empty LLM response")
	}

	tokensUsed := resp.Usage.TotalTokens
	raw := resp.Choices[0].Message.Content
	result := parseGuardianResponse(raw)
	result.TokensUsed = tokensUsed
	result.Duration = time.Since(start)

	g.logger.Info("[Guardian] Evaluated",
		"operation", check.Operation,
		"decision", result.Decision,
		"risk", result.RiskScore,
		"reason", result.Reason,
		"tokens", tokensUsed,
		"latency_ms", result.Duration.Milliseconds())

	return result
}

func (g *LLMGuardian) failSafeResult(start time.Time, reason string) GuardianResult {
	fs := g.cfg.LLMGuardian.FailSafe
	var decision Decision
	var risk float64
	switch fs {
	case "block":
		decision = DecisionBlock
		risk = 1.0
	case "allow":
		decision = DecisionAllow
		risk = 0.0
	default: // "quarantine"
		decision = DecisionQuarantine
		risk = 0.5
	}
	return GuardianResult{
		Decision:  decision,
		RiskScore: risk,
		Reason:    "fail-safe: " + reason,
		Duration:  time.Since(start),
	}
}

func (g *LLMGuardian) resolveLevel(toolName string) GuardianLevel {
	// Check tool-specific override first
	if g.cfg.LLMGuardian.ToolOverrides != nil {
		if override, ok := g.cfg.LLMGuardian.ToolOverrides[toolName]; ok {
			return parseLevel(override)
		}
	}
	return parseLevel(g.cfg.LLMGuardian.DefaultLevel)
}

// ── Level / Tool classification ─────────────────────────────────────────────

func parseLevel(s string) GuardianLevel {
	switch strings.ToLower(s) {
	case "off":
		return GuardianOff
	case "low":
		return GuardianLow
	case "medium":
		return GuardianMedium
	case "high":
		return GuardianHigh
	default:
		return GuardianMedium
	}
}

// highRiskTools are always checked at GuardianLow level.
var highRiskTools = map[string]bool{
	"execute_shell":        true,
	"execute_sudo":         true,
	"execute_python":       true,
	"execute_remote_shell": true,
	"filesystem":           true,
}

// riskyTools are checked at GuardianMedium level (includes high-risk).
var riskyTools = map[string]bool{
	"execute_shell":        true,
	"execute_sudo":         true,
	"execute_python":       true,
	"execute_remote_shell": true,
	"filesystem":           true,
	"api_request":          true,
	"docker":               true,
	"proxmox":              true,
	"set_secret":           true,
	"save_tool":            true,
	"co_agent":             true,
	"manage_updates":       true,
	"netlify":              true,
	"home_assistant":       true,
}

func isHighRiskTool(name string) bool { return highRiskTools[name] }
func isRiskyTool(name string) bool    { return riskyTools[name] }

// ── Prompt & Response ───────────────────────────────────────────────────────

const guardianSystemPrompt = `You are a security auditor for an AI agent. Your ONLY job is to classify whether a tool call is safe. Respond in EXACTLY this format:
DECISION RISK_SCORE REASON
Where DECISION is safe/suspicious/dangerous, RISK_SCORE is 0-100, REASON is max 8 words.
Example: safe 10 routine file listing`

func buildGuardianPrompt(check GuardianCheck) string {
	var sb strings.Builder
	sb.WriteString("TOOL: ")
	sb.WriteString(check.Operation)
	sb.WriteString("\n")

	if len(check.Parameters) > 0 {
		sb.WriteString("PARAMS: ")
		for k, v := range check.Parameters {
			// Truncate long parameter values
			if len(v) > 200 {
				v = v[:200] + "..."
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(v)
			sb.WriteString(" ")
		}
		sb.WriteString("\n")
	}

	if check.Context != "" {
		ctx := check.Context
		if len(ctx) > 200 {
			ctx = ctx[:200] + "..."
		}
		sb.WriteString("CONTEXT: ")
		sb.WriteString(ctx)
		sb.WriteString("\n")
	}

	if check.RegexLevel > ThreatNone {
		sb.WriteString("REGEX_FLAG: ")
		sb.WriteString(check.RegexLevel.String())
		sb.WriteString("\n")
	}

	sb.WriteString("CLASSIFY:")
	return sb.String()
}

func parseGuardianResponse(raw string) GuardianResult {
	raw = strings.TrimSpace(raw)
	// Expected: "safe 10 routine file listing" or "dangerous 95 deletes system files"
	parts := strings.Fields(raw)
	if len(parts) < 2 {
		// Unparseable — treat as quarantine
		return GuardianResult{
			Decision:  DecisionQuarantine,
			RiskScore: 0.5,
			Reason:    "unparseable response: " + truncate(raw, 50),
		}
	}

	decision := mapDecision(parts[0])
	score, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		score = 50
	}
	score = score / 100.0 // normalize to 0.0-1.0
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	reason := ""
	if len(parts) > 2 {
		reason = strings.Join(parts[2:], " ")
	}

	return GuardianResult{
		Decision:  decision,
		RiskScore: score,
		Reason:    reason,
	}
}

func mapDecision(word string) Decision {
	switch strings.ToLower(word) {
	case "safe", "allow", "ok", "benign":
		return DecisionAllow
	case "dangerous", "block", "deny", "reject", "critical":
		return DecisionBlock
	case "suspicious", "risky", "quarantine", "warn", "caution":
		return DecisionQuarantine
	default:
		return DecisionQuarantine
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

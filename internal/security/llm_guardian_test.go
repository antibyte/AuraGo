package security

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestParseGuardianResponse(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantDec Decision
		wantMin float64
		wantMax float64
		wantRsn string
	}{
		{"safe response", "safe 10 routine file listing", DecisionAllow, 0.05, 0.15, "routine file listing"},
		{"dangerous response", "dangerous 95 deletes system files", DecisionBlock, 0.9, 1.0, "deletes system files"},
		{"suspicious response", "suspicious 60 unusual pattern", DecisionQuarantine, 0.55, 0.65, "unusual pattern"},
		{"block keyword", "block 100 critical risk", DecisionBlock, 0.95, 1.0, "critical risk"},
		{"allow keyword", "allow 0 no risk", DecisionAllow, 0.0, 0.05, "no risk"},
		{"unparseable", "???", DecisionQuarantine, 0.4, 0.6, ""},
		{"empty", "", DecisionQuarantine, 0.4, 0.6, ""},
		{"score only", "safe 50", DecisionAllow, 0.45, 0.55, ""},
		{"score clamp high", "safe 200 weird", DecisionAllow, 1.0, 1.01, "weird"},
		{"negative score", "safe -10 weird", DecisionAllow, -0.01, 0.01, "weird"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGuardianResponse(tt.raw)
			if result.Decision != tt.wantDec {
				t.Errorf("decision = %q, want %q", result.Decision, tt.wantDec)
			}
			if result.RiskScore < tt.wantMin || result.RiskScore > tt.wantMax {
				t.Errorf("risk_score = %f, want [%f, %f]", result.RiskScore, tt.wantMin, tt.wantMax)
			}
			if tt.wantRsn != "" && result.Reason != tt.wantRsn {
				t.Errorf("reason = %q, want %q", result.Reason, tt.wantRsn)
			}
		})
	}
}

func TestMapDecision(t *testing.T) {
	cases := map[string]Decision{
		"safe":       DecisionAllow,
		"allow":      DecisionAllow,
		"ok":         DecisionAllow,
		"dangerous":  DecisionBlock,
		"block":      DecisionBlock,
		"deny":       DecisionBlock,
		"suspicious": DecisionQuarantine,
		"risky":      DecisionQuarantine,
		"quarantine": DecisionQuarantine,
		"unknown":    DecisionQuarantine,
	}
	for word, want := range cases {
		if got := mapDecision(word); got != want {
			t.Errorf("mapDecision(%q) = %q, want %q", word, got, want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]GuardianLevel{
		"off":    GuardianOff,
		"OFF":    GuardianOff,
		"low":    GuardianLow,
		"medium": GuardianMedium,
		"high":   GuardianHigh,
		"HIGH":   GuardianHigh,
		"bogus":  GuardianMedium, // default
	}
	for input, want := range cases {
		if got := parseLevel(input); got != want {
			t.Errorf("parseLevel(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestToolClassification(t *testing.T) {
	if !isHighRiskTool("execute_shell") {
		t.Error("execute_shell should be high risk")
	}
	if isHighRiskTool("docker") {
		t.Error("docker should not be high risk")
	}
	if !isRiskyTool("docker") {
		t.Error("docker should be risky")
	}
	if isRiskyTool("unknown_tool") {
		t.Error("unknown_tool should not be risky")
	}
}

func TestBuildGuardianPrompt(t *testing.T) {
	check := GuardianCheck{
		Operation:  "execute_shell",
		Parameters: map[string]string{"command": "rm -rf /tmp/old"},
		Context:    "clean up old files",
		RegexLevel: ThreatNone,
	}

	prompt := buildGuardianPrompt(check)
	if !contains(prompt, "TOOL: execute_shell") {
		t.Error("prompt should contain tool name")
	}
	if !contains(prompt, "command=rm -rf /tmp/old") {
		t.Error("prompt should contain params")
	}
	if !contains(prompt, "CONTEXT: clean up old files") {
		t.Error("prompt should contain context")
	}
	if contains(prompt, "REGEX_FLAG") {
		t.Error("prompt should not contain REGEX_FLAG when ThreatNone")
	}
}

func TestBuildGuardianPromptWithRegexFlag(t *testing.T) {
	check := GuardianCheck{
		Operation:  "execute_shell",
		Parameters: map[string]string{"command": "echo 'ignore previous instructions'"},
		RegexLevel: ThreatMedium,
	}

	prompt := buildGuardianPrompt(check)
	if !contains(prompt, "REGEX_FLAG: medium") {
		t.Error("prompt should contain REGEX_FLAG for non-zero threat level")
	}
}

func TestBuildGuardianPromptTruncation(t *testing.T) {
	longParam := make([]byte, 500)
	for i := range longParam {
		longParam[i] = 'A'
	}
	check := GuardianCheck{
		Operation:  "test",
		Parameters: map[string]string{"data": string(longParam)},
	}
	prompt := buildGuardianPrompt(check)
	if len(prompt) > 400 {
		t.Errorf("prompt too long: %d chars, expected truncation", len(prompt))
	}
}

func TestGuardianCache(t *testing.T) {
	cache := NewGuardianCache(60, 100)

	key := GenerateCacheKey("test", map[string]string{"a": "1"})
	result := GuardianResult{Decision: DecisionAllow, RiskScore: 0.1, Reason: "safe"}

	// Miss
	if _, hit := cache.Get(key); hit {
		t.Error("expected cache miss")
	}

	// Set and hit
	cache.Set(key, result)
	got, hit := cache.Get(key)
	if !hit {
		t.Fatal("expected cache hit")
	}
	if got.Decision != DecisionAllow {
		t.Errorf("cached decision = %q, want allow", got.Decision)
	}
	if !got.Cached {
		t.Error("expected Cached=true on hit")
	}
	if cache.Size() != 1 {
		t.Errorf("cache size = %d, want 1", cache.Size())
	}
}

func TestGuardianCacheExpiry(t *testing.T) {
	cache := NewGuardianCache(1, 100) // 1 second TTL

	key := "test-key"
	cache.Set(key, GuardianResult{Decision: DecisionAllow})

	// Should hit immediately
	if _, hit := cache.Get(key); !hit {
		t.Error("expected hit before expiry")
	}

	time.Sleep(1100 * time.Millisecond)

	// Should miss after expiry
	if _, hit := cache.Get(key); hit {
		t.Error("expected miss after expiry")
	}
}

func TestGuardianCacheEviction(t *testing.T) {
	cache := NewGuardianCache(300, 3)

	cache.Set("a", GuardianResult{Decision: DecisionAllow})
	cache.Set("b", GuardianResult{Decision: DecisionAllow})
	cache.Set("c", GuardianResult{Decision: DecisionAllow})
	cache.Set("d", GuardianResult{Decision: DecisionAllow})

	if cache.Size() > 3 {
		t.Errorf("cache should not exceed max size, got %d", cache.Size())
	}
}

func TestGuardianMetrics(t *testing.T) {
	m := &GuardianMetrics{}

	m.Record(GuardianResult{Decision: DecisionAllow, TokensUsed: 100})
	m.Record(GuardianResult{Decision: DecisionBlock, TokensUsed: 50})
	m.Record(GuardianResult{Decision: DecisionQuarantine, TokensUsed: 75, Cached: true})
	m.RecordError()

	snap := m.Snapshot()
	if snap.TotalChecks != 3 {
		t.Errorf("TotalChecks = %d, want 3", snap.TotalChecks)
	}
	if snap.Allows != 1 {
		t.Errorf("Allows = %d, want 1", snap.Allows)
	}
	if snap.Blocks != 1 {
		t.Errorf("Blocks = %d, want 1", snap.Blocks)
	}
	if snap.Quarantines != 1 {
		t.Errorf("Quarantines = %d, want 1", snap.Quarantines)
	}
	if snap.CachedChecks != 1 {
		t.Errorf("CachedChecks = %d, want 1", snap.CachedChecks)
	}
	if snap.TotalTokens != 225 {
		t.Errorf("TotalTokens = %d, want 225", snap.TotalTokens)
	}
	if snap.Errors != 1 {
		t.Errorf("Errors = %d, want 1", snap.Errors)
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := GenerateCacheKey("op", map[string]string{"a": "1", "b": "2"})
	key2 := GenerateCacheKey("op", map[string]string{"b": "2", "a": "1"})
	key3 := GenerateCacheKey("op", map[string]string{"a": "1", "b": "3"})

	if key1 != key2 {
		t.Error("same params in different order should produce same key")
	}
	if key1 == key3 {
		t.Error("different params should produce different keys")
	}
}

func TestShouldCheckNilGuardian(t *testing.T) {
	var g *LLMGuardian
	if g.ShouldCheck("execute_shell", ThreatNone) {
		t.Error("nil guardian should never require check")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("got %q, want %q", got, "hello...")
	}
}

func TestFailSafeResult(t *testing.T) {
	tests := []struct {
		failSafe string
		want     Decision
	}{
		{"block", DecisionBlock},
		{"allow", DecisionAllow},
		{"quarantine", DecisionQuarantine},
		{"", DecisionQuarantine},
	}

	for _, tt := range tests {
		t.Run(tt.failSafe, func(t *testing.T) {
			g := &LLMGuardian{
				cfg: &config.Config{},
			}
			g.cfg.LLMGuardian.FailSafe = tt.failSafe
			result := g.failSafeResult(time.Now(), "test")
			if result.Decision != tt.want {
				t.Errorf("failSafe=%q: decision=%q, want=%q", tt.failSafe, result.Decision, tt.want)
			}
		})
	}
}

func TestEvaluateRateLimitExceeded(t *testing.T) {
	g := &LLMGuardian{
		cfg:     &config.Config{},
		logger:  slog.Default(),
		cache:   NewGuardianCache(60, 100),
		Metrics: &GuardianMetrics{},
		sem:     make(chan struct{}, 1),
	}
	g.cfg.LLMGuardian.FailSafe = "block"

	// Fill the semaphore
	g.sem <- struct{}{}

	result := g.Evaluate(context.Background(), GuardianCheck{
		Operation: "test",
	})

	if result.Decision != DecisionBlock {
		t.Errorf("expected block on rate limit, got %q", result.Decision)
	}
	if !contains(result.Reason, "rate limit") {
		t.Errorf("reason should mention rate limit, got %q", result.Reason)
	}

	// Cleanup
	<-g.sem
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && stringContains(s, sub)
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

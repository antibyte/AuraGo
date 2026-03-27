package agent

import (
	"testing"

	"aurago/internal/config"
	"aurago/internal/prompts"
)

func TestBuildToolingPolicyAutoEnablesNativeFunctionsForDeepSeek(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "deepseek-chat"

	policy := buildToolingPolicy(cfg, "")

	if !policy.UseNativeFunctions {
		t.Fatal("expected native function calling to be enabled for DeepSeek")
	}
	if !policy.AutoEnabledNativeFunctions {
		t.Fatal("expected DeepSeek native function calling to be marked as auto-enabled")
	}
}

func TestBuildToolingPolicyHonorsExplicitNativeFunctions(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.LLM.UseNativeFunctions = true

	policy := buildToolingPolicy(cfg, "")

	if !policy.UseNativeFunctions {
		t.Fatal("expected explicit native function calling setting to be preserved")
	}
	if policy.AutoEnabledNativeFunctions {
		t.Fatal("did not expect explicit native function calling to be treated as auto-enabled")
	}
}

func TestBuildToolingPolicyDisablesStructuredOutputsAndParallelForOllama(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.StructuredOutputs = true
	cfg.LLM.UseNativeFunctions = true

	policy := buildToolingPolicy(cfg, "")

	if !policy.StructuredOutputsRequested {
		t.Fatal("expected structured outputs request to be preserved")
	}
	if policy.StructuredOutputsEnabled {
		t.Fatal("expected structured outputs to be disabled for Ollama")
	}
	if policy.ParallelToolCallsEnabled {
		t.Fatal("expected parallel tool calls to be disabled for Ollama")
	}
}

func TestBuildPromptContextFlagsKeepsHomepageFallbackWhenDockerSocketUnavailable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Runtime.IsDocker = true
	cfg.Runtime.DockerSocketOK = false
	cfg.Docker.Enabled = true
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowLocalServer = true

	runCfg := RunConfig{
		Config:    cfg,
		SessionID: "default",
	}

	policy := buildToolingPolicy(cfg, "")
	flags := buildPromptContextFlags(runCfg, policy, promptContextOptions{})

	if flags.DockerEnabled {
		t.Fatal("expected docker-enabled prompt flag to be false without docker socket access")
	}
	if !flags.HomepageEnabled {
		t.Fatal("expected homepage prompt flag to stay enabled via local server fallback")
	}
	if !flags.HomepageAllowLocalServer {
		t.Fatal("expected homepage local server fallback flag to be exposed")
	}
}

func TestBuildToolingPolicyKeepsConfiguredGuideBudgetByDefault(t *testing.T) {
	resetAgentTelemetryForTest()

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.Agent.MaxToolGuides = 6

	policy := buildToolingPolicy(cfg, "")

	if policy.TelemetryProfile != "default" {
		t.Fatalf("unexpected telemetry profile: %s", policy.TelemetryProfile)
	}
	if policy.EffectiveMaxToolGuides != 6 {
		t.Fatalf("effective guide budget = %d, want 6", policy.EffectiveMaxToolGuides)
	}
}

func TestBuildToolingPolicyReducesGuideBudgetForWeakScope(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "deepseek-chat"}
	for i := 0; i < 8; i++ {
		RecordScopedToolResult(scope, i < 4)
	}

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "deepseek-chat"
	cfg.Agent.MaxToolGuides = 5

	policy := buildToolingPolicy(cfg, "")

	if policy.TelemetryProfile != "conservative" {
		t.Fatalf("telemetry profile = %s, want conservative", policy.TelemetryProfile)
	}
	if policy.EffectiveMaxToolGuides != 3 {
		t.Fatalf("effective guide budget = %d, want 3", policy.EffectiveMaxToolGuides)
	}
	if policy.TelemetrySnapshot.ToolCalls != 8 {
		t.Fatalf("telemetry tool calls = %d, want 8", policy.TelemetrySnapshot.ToolCalls)
	}
	if !policy.EffectiveGuideStrategy.PreferSemantics {
		t.Fatal("expected conservative profile to prefer semantic guides")
	}
	if policy.EffectiveGuideStrategy.DisableRecentHeuristics {
		t.Fatal("did not expect conservative profile to disable recent guide fallback")
	}
	if !policy.EffectiveGuideStrategy.DisableStatisticalHeuristics {
		t.Fatal("expected conservative profile to disable statistical heuristics")
	}
	if !policy.EffectiveGuideStrategy.DisableFrequencyHeuristics {
		t.Fatal("expected conservative profile to disable frequency heuristics")
	}
}

func TestApplyTelemetryAwarePromptTierDowngradesFullToCompactForWeakScope(t *testing.T) {
	policy := ToolingPolicy{
		TelemetryProfile: "conservative",
		TelemetrySnapshot: AgentTelemetryScopeSnapshot{
			ToolCalls:    10,
			FailureRate:  0.5,
			SuccessRate:  0.5,
			TotalEvents:  10,
			ProviderType: "openrouter",
			Model:        "deepseek-chat",
		},
	}
	flags := prompts.ContextFlags{
		MessageCount:    8,
		PredictedGuides: nil,
	}

	got := applyTelemetryAwarePromptTier(policy, flags, "full")

	if got != "compact" {
		t.Fatalf("tier = %s, want compact", got)
	}
}

func TestApplyTelemetryAwarePromptTierKeepsFullWhenGuidesOrCodingAreNeeded(t *testing.T) {
	policy := ToolingPolicy{TelemetryProfile: "conservative"}

	gotWithGuides := applyTelemetryAwarePromptTier(policy, prompts.ContextFlags{
		MessageCount:    8,
		PredictedGuides: []string{"guide"},
	}, "full")
	if gotWithGuides != "full" {
		t.Fatalf("tier with guides = %s, want full", gotWithGuides)
	}

	gotWithCoding := applyTelemetryAwarePromptTier(policy, prompts.ContextFlags{
		MessageCount:   8,
		RequiresCoding: true,
	}, "full")
	if gotWithCoding != "full" {
		t.Fatalf("tier with coding = %s, want full", gotWithCoding)
	}
}

func TestBuildToolingPolicyUsesFamilyGuardForProblematicIntentFamily(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "gpt-4o-mini"}
	for i := 0; i < 4; i++ {
		RecordScopedToolResultForTool(scope, "homepage", false)
	}

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.Agent.MaxToolGuides = 5

	policy := buildToolingPolicy(cfg, "please deploy the homepage to netlify")

	if policy.TelemetryProfile != "family_guarded" {
		t.Fatalf("telemetry profile = %s, want family_guarded", policy.TelemetryProfile)
	}
	if policy.IntentFamily != "deployment" {
		t.Fatalf("intent family = %s, want deployment", policy.IntentFamily)
	}
	if policy.FamilyTelemetry.ToolCalls != 4 || policy.FamilyTelemetry.ToolFailures != 4 {
		t.Fatalf("unexpected family telemetry: %+v", policy.FamilyTelemetry)
	}
	if policy.EffectiveMaxToolGuides != 4 {
		t.Fatalf("effective guide budget = %d, want 4", policy.EffectiveMaxToolGuides)
	}
	if !policy.EffectiveGuideStrategy.PreferSemantics {
		t.Fatal("expected family-guarded profile to prefer semantics")
	}
}

func TestCalculateEffectivePromptTokenBudgetScalesForHomepageFlow(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.Agent.ContextWindow = 64000
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.Homepage.Enabled = true
	cfg.Homepage.CircuitBreakerMaxCalls = 35
	cfg.Homepage.AllowTemporaryTokenBudgetOverflow = true

	got := calculateEffectivePromptTokenBudget(cfg, ToolCall{Action: "homepage"}, false, nil)

	if got != 42000 {
		t.Fatalf("effective prompt token budget = %d, want 42000", got)
	}
}

func TestCalculateEffectivePromptTokenBudgetKeepsBaseWhenHomepageOverflowDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.Homepage.Enabled = true
	cfg.Homepage.CircuitBreakerMaxCalls = 35
	cfg.Homepage.AllowTemporaryTokenBudgetOverflow = false

	got := calculateEffectivePromptTokenBudget(cfg, ToolCall{Action: "homepage"}, false, nil)

	if got != 12000 {
		t.Fatalf("effective prompt token budget = %d, want 12000", got)
	}
}

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/prompts"
	"aurago/internal/security"
)

func TestHandleDashboardPromptStatsContract(t *testing.T) {
	prompts.RecordBuild(prompts.PromptBuildRecord{
		Timestamp:     time.Now(),
		Tier:          "full",
		RawLen:        1000,
		OptimizedLen:  700,
		FormatSavings: 50,
		ShedSavings:   100,
		FilterSavings: 150,
		Tokens:        120,
		ModulesLoaded: 8,
		ModulesUsed:   6,
		GuidesCount:   2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/prompt-stats", nil)
	rec := httptest.NewRecorder()

	handleDashboardPromptStats().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"total_builds", "avg_raw_len", "avg_optimized_len", "avg_saved_chars", "tier_distribution", "recent"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("prompt stats missing key %q", key)
		}
	}
	if got := int(body["total_builds"].(float64)); got < 1 {
		t.Fatalf("total_builds = %d, want >= 1", got)
	}
}

func TestHandleDashboardToolStatsContract(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 17
	cfg.Agent.AdaptiveTools.DecayHalfLifeDays = 7
	cfg.Agent.AdaptiveTools.WeightSuccessRate = true

	prompts.RecordToolUsage("shell", "execute", true)
	prompts.RecordToolUsage("docker", "list", false)
	prompts.RecordAdaptiveToolUsage("shell", true)
	prompts.RecordAdaptiveToolUsage("docker", false)
	agent.RecordToolParseSource(agent.ToolCallParseSourceNative)
	agent.RecordToolRecoveryEvent("provider_422_recovered")
	agent.RecordToolPolicyEvent("conservative_profile_applied")
	scope := agent.AgentTelemetryScope{ProviderType: "openrouter", Model: "deepseek-chat"}
	agent.RecordToolParseSourceForScope(scope, agent.ToolCallParseSourceNative)
	agent.RecordScopedToolResultForTool(scope, "homepage", false)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/tool-stats", nil)
	rec := httptest.NewRecorder()

	handleDashboardToolStats(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"total_calls", "by_tool", "top_tools", "recent", "adaptive_enabled", "adaptive_scores", "max_tools", "agent_telemetry"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("tool stats missing key %q", key)
		}
	}
	if got := int(body["total_calls"].(float64)); got < 2 {
		t.Fatalf("total_calls = %d, want >= 2", got)
	}
	if enabled, ok := body["adaptive_enabled"].(bool); !ok || !enabled {
		t.Fatalf("adaptive_enabled = %#v, want true", body["adaptive_enabled"])
	}
	if got := int(body["max_tools"].(float64)); got != 17 {
		t.Fatalf("max_tools = %d, want 17", got)
	}
	telemetry, ok := body["agent_telemetry"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent_telemetry has unexpected type %T", body["agent_telemetry"])
	}
	for _, key := range []string{"parse_sources", "recovery_events", "policy_events", "scopes"} {
		if _, ok := telemetry[key]; !ok {
			t.Fatalf("agent_telemetry missing key %q", key)
		}
	}
	scopes, ok := telemetry["scopes"].([]interface{})
	if !ok || len(scopes) == 0 {
		t.Fatalf("agent_telemetry scopes = %#v, want non-empty scope list", telemetry["scopes"])
	}
	foundFamilyData := false
	for _, raw := range scopes {
		scopeMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := scopeMap["tool_families"]; ok {
			foundFamilyData = true
			break
		}
	}
	if !foundFamilyData {
		t.Fatal("expected at least one telemetry scope with tool_families")
	}
}

func TestHandleDashboardGuardianContract(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLMGuardian.Enabled = true
	cfg.LLMGuardian.DefaultLevel = "medium"
	cfg.LLMGuardian.FailSafe = "quarantine"

	guardian := &security.LLMGuardian{Metrics: &security.GuardianMetrics{}}
	guardian.Metrics.Record(security.GuardianResult{
		Decision:   security.DecisionAllow,
		TokensUsed: 42,
	})

	s := &Server{
		Cfg:         cfg,
		LLMGuardian: guardian,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/guardian", nil)
	rec := httptest.NewRecorder()

	handleDashboardGuardian(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"enabled", "level", "fail_safe", "metrics"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("guardian payload missing key %q", key)
		}
	}
	metrics, ok := body["metrics"].(map[string]interface{})
	if !ok {
		t.Fatalf("metrics has unexpected type %T", body["metrics"])
	}
	for _, key := range []string{"total_checks", "allows", "total_tokens", "cache_hit_rate"} {
		if _, ok := metrics[key]; !ok {
			t.Fatalf("guardian metrics missing key %q", key)
		}
	}
}

func TestHandleDashboardOverviewContract(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.LLM.ProviderType = "openrouter"
	cfg.Personality.CorePersonality = "helper"
	cfg.Agent.ContextWindow = 32000
	cfg.Maintenance.Enabled = true
	cfg.Docker.Enabled = true
	cfg.MemoryAnalysis.Enabled = true
	cfg.LLMGuardian.Enabled = true
	cfg.Homepage.Enabled = true
	cfg.Netlify.Enabled = true
	cfg.Tools.SkillManager.Enabled = true

	s := &Server{Cfg: cfg}

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/overview", nil)
	rec := httptest.NewRecorder()

	handleDashboardOverview(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"agent", "integrations", "missions", "context", "skills"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("overview payload missing key %q", key)
		}
	}

	agentInfo, ok := body["agent"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent info has unexpected type %T", body["agent"])
	}
	if agentInfo["model"] != "test-model" {
		t.Fatalf("agent model = %#v, want test-model", agentInfo["model"])
	}
	if agentInfo["provider"] != "openrouter" {
		t.Fatalf("agent provider = %#v, want openrouter", agentInfo["provider"])
	}

	integrations, ok := body["integrations"].(map[string]interface{})
	if !ok {
		t.Fatalf("integrations has unexpected type %T", body["integrations"])
	}
	for _, key := range []string{"docker", "memory_analysis", "llm_guardian", "homepage", "netlify", "skill_manager"} {
		if _, ok := integrations[key]; !ok {
			t.Fatalf("integrations missing key %q", key)
		}
	}
}

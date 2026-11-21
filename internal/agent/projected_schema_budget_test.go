package agent

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func TestProjectedStrictSchemaBudgetShedsOptionalOrBlocksRequired(t *testing.T) {
	original := []openai.Tool{budgetTestTool("required", "Required tool"), budgetTestTool("optional", strings.Repeat("optional description ", 35))}
	// Optional fields become required/nullable in the Strict wire projection.
	for i := range original {
		original[i].Function.Parameters = map[string]interface{}{"type": "object", "properties": map[string]interface{}{"value": map[string]interface{}{"type": "string"}}, "required": []string{}}
	}
	snapshot := newNativeToolSchemaSnapshot(original)
	projected := strictSchemasForActiveTools(snapshot, original)
	cap := estimateToolSchemaListTokens(original)
	if estimateToolSchemaListTokens(projected) <= cap {
		t.Fatal("fixture does not exercise projection growth")
	}
	b := &RequestBudget{MaxSchemaTokens: cap, CompletionReserve: 100, SafetyMargin: 32, MinimumSystem: 100, Routes: []RequestRouteBudget{{Limits: llm.ModelLimits{ContextWindow: 64000, Route: llm.ModelRoute{Model: "fixture"}}}}}
	kept, dropped, err := b.shedOptionalTools(nil, projected, map[string]bool{"required": true}, "fixture", newTokenCountCache(8))
	if err != nil || len(kept) != 1 || len(dropped) != 1 || dropped[0] != "optional" {
		t.Fatalf("projected selection kept=%d dropped=%v err=%v", len(kept), dropped, err)
	}
	_, _, err = b.shedOptionalTools(nil, projected, map[string]bool{"required": true, "optional": true}, "fixture", newTokenCountCache(8))
	var exceeded *ToolSchemaBudgetExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("oversized required projection sent: %v", err)
	}
	if _, err = b.validate(nil, projected, newTokenCountCache(8)); !errors.As(err, &exceeded) {
		t.Fatalf("final wire check missed projected growth: %v", err)
	}
	if original[0].Function.Strict {
		t.Fatal("budget check mutated original schemas")
	}
}

func TestAdaptiveStrictSchemaBudgetWithHomepageAndGameMaker(t *testing.T) {
	runCfg, client, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
	t.Cleanup(cleanup)
	runCfg.SuppressTurnSideEffects = true
	cfg := runCfg.Config
	cfg.LLM.Provider, cfg.LLM.ProviderType, cfg.LLM.Model = "main", "openai", "gpt-4o"
	cfg.LLM.UseNativeFunctions, cfg.LLM.StructuredOutputs = true, true
	cfg.Agent.ContextWindow = 128000
	cfg.Agent.AllowFilesystemWrite, cfg.Agent.AllowShell = true, true
	cfg.Tools.Memory.Enabled = true
	cfg.Homepage.Enabled, cfg.Homepage.AllowLocalServer = true, true
	cfg.GameMaker.Enabled = true
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.AlwaysInclude = []string{
		"filesystem", "query_memory", "manage_memory", "execute_shell",
		"homepage_deploy", "homepage_file", "homepage_git", "homepage_project", "homepage_quality",
		"game_maker_asset", "game_maker_file", "game_maker_preview", "game_maker_project",
	}
	hard := []string{"discover_tools", "invoke_tool", "execute_skill"}
	runCfg.AllowedTools = slices.Concat(hard, cfg.Agent.AdaptiveTools.AlwaysInclude)
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "List existing projects."}},
	}
	policy := buildToolingPolicy(cfg, "List existing projects.")
	snapshot := BuildNativeToolSchemaSnapshot(cfg.Directories.SkillsDir, nil, buildToolFeatureFlags(runCfg, policy), runCfg.Logger)
	raw := filterSchemasByAllowedTools(snapshot.FullSchemas(), runCfg.AllowedTools)
	rawTokens := estimateToolSchemaListTokens(raw)
	strictTokens := estimateToolSchemaListTokens(strictSchemasForActiveTools(snapshot, raw))
	limit := rawTokens
	cfg.Agent.AdaptiveTools.MaxSchemaTokens = limit
	if len(raw) != len(runCfg.AllowedTools) || strictTokens <= limit {
		t.Fatalf("fixture no longer reproduces the Strict overflow: %d tools, raw=%d strict=%d", len(raw), rawTokens, strictTokens)
	}
	if err := ValidateMinimumRequestBudget(t.Context(), req, runCfg); err != nil {
		t.Fatalf("preflight rejected a request with sheddable preferences: %v", err)
	}
	discovery := acquireDiscoveryRun()
	t.Cleanup(func() { releaseDiscoveryRun(discovery) })
	initRunCfg := runCfg
	initRunCfg.DiscoveryRunID = discovery
	state := initAgentLoopState(req, initRunCfg, nil, false)
	if tokens := estimateToolSchemaListTokens(state.req.Tools); tokens > limit {
		t.Fatalf("first selection exceeded the projected budget: %d > %d", tokens, limit)
	}
	if _, err := ExecuteAgentLoop(t.Context(), req, runCfg, false, &usageRegressionBroker{}); err != nil {
		t.Fatalf("agent loop should reach the model: %v", err)
	}
	wire := client.lastReq.Tools
	wireNames := toolSchemaNames(wire)
	if client.calls != 1 || len(wire) <= len(hard) || len(wire) >= len(raw) || estimateToolSchemaListTokens(wire) > limit {
		t.Fatalf("expected a budgeted model request with some preferred tools: calls=%d tools=%v", client.calls, wireNames)
	}
	for _, name := range hard {
		if !slices.Contains(wireNames, name) {
			t.Errorf("missing hard tool %s", name)
		}
	}
	for _, tool := range wire {
		if !tool.Function.Strict {
			t.Errorf("tool %s lost its Strict projection", tool.Function.Name)
		}
	}
}

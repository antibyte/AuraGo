package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

func TestCoAgentManagementSurvivesExhaustedBudget(t *testing.T) {
	for _, enforcement := range []string{"warn", "block"} {
		t.Run(enforcement, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.CoAgents.Enabled, cfg.CoAgents.MaxConcurrent, cfg.CoAgents.BudgetQuotaPercent = true, 1, 25
			cfg.Budget.Enabled, cfg.Budget.DailyLimitUSD, cfg.Budget.Enforcement = true, 100, enforcement
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			tracker := budget.NewTracker(cfg, logger, t.TempDir())
			defer tracker.Flush()
			tracker.RecordCostForCategory("coagent", 101)
			registry := NewCoAgentRegistry(1, logger)
			dc := &DispatchContext{Cfg: cfg, Logger: logger, BudgetTracker: tracker, CoAgentRegistry: registry, UserContext: "Stop all co-agents now"}
			id, _ := registry.Register("finished", func() {})
			registry.Complete(id, "already paid result", 35, 1)
			for _, op := range []string{"list", "get_result", "stop", "stop_all"} {
				if op == "stop" || op == "stop_all" {
					id, _ = registry.Register("running", func() {})
				}
				out, handled := dispatchPlatform(t.Context(), ToolCall{Action: "co_agent", Operation: op, CoAgentID: id}, dc)
				if !handled || !strings.Contains(out, `"status":"ok"`) && !strings.Contains(out, `"status": "ok"`) {
					t.Fatalf("%s: %s", op, out)
				}
				if op == "stop" || op == "stop_all" {
					status, _ := registry.GetStatus(id)
					if status["state"] != string(CoAgentCancelled) {
						t.Fatalf("stop failed: %v", status)
					}
				}
			}
			if _, _, err := SpawnCoAgent(newCoAgentSpawnContext(dc), CoAgentRequest{Task: "no spending"}, nil); err == nil {
				t.Fatal("spawn ignored exhausted budget")
			}
		})
	}
}

func TestCoAgentSelectedProviderAndFixedToolBudget(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider, cfg.LLM.ProviderType, cfg.LLM.Model = "main-local", "ollama", "local-model"
	p := config.ProviderEntry{ID: "co-openai", Type: "openai", Model: "gpt-4o-mini", AccountID: "child-account", GatewayID: "child-gateway"}
	cfg.Providers = []config.ProviderEntry{{ID: "main-local", Type: "ollama", Model: "local-model"}, p}
	cfg.CoAgents.LLM.Provider, cfg.CoAgents.LLM.ProviderType, cfg.CoAgents.LLM.Model = p.ID, p.Type, p.Model
	selection, _ := selectCoAgentLLMForRole(cfg, "")
	child := coAgentConfigForSelection(cfg, selection)
	if child.LLM.Provider != p.ID || child.LLM.AccountID != p.AccountID || child.LLM.GatewayID != p.GatewayID || child.FallbackLLM.Enabled {
		t.Fatal("child provider identity did not survive projection")
	}
	actual, expected := buildToolingPolicy(child, ""), buildToolingPolicy(llm.TaskProviderConfig(cfg, p), "")
	if actual.Capabilities.ProviderType != "openai" || actual.Capabilities.SupportsParallelToolCalls != expected.Capabilities.SupportsParallelToolCalls {
		t.Fatal("transport and loop provider capabilities disagree")
	}
	child.CircuitBreaker.MaxToolCalls, child.Homepage.CircuitBreakerMaxCalls = 2, 75
	child.Homepage.Enabled = true
	run := RunConfig{Config: child, IsCoAgent: true, ToolCallLimit: 2}
	if got := calculateEffectiveMaxCalls(run, ToolCall{Action: "homepage_file"}, false, false, nil, slog.Default()); got != 2 {
		t.Fatalf("limit=%d", got)
	}
	if cfg.LLM.Model != "local-model" || cfg.LLM.ProviderType != "ollama" {
		t.Fatal("main provider mutated")
	}
}

func TestCoAgentUnicodeHints(t *testing.T) {
	for _, value := range []string{"A界", "ÄÖÜ", "🙂🌍🚀", "e\u0301x"} {
		for limit := 1; limit <= 3; limit++ {
			cfg := &config.Config{}
			cfg.CoAgents.MaxContextHints, cfg.CoAgents.MaxContextHintChars = 2, limit
			req := normalizeCoAgentRequest(cfg, CoAgentRequest{ContextHints: []string{"\r\n", value, value, "other", "ignored"}})
			want := string([]rune(value)[:min(limit, utf8.RuneCountInString(value))])
			if len(req.ContextHints) != 2 || req.ContextHints[0] != want {
				t.Fatalf("%q limit %d: %q", value, limit, req.ContextHints)
			}
			for _, hint := range req.ContextHints {
				if !utf8.ValidString(hint) || utf8.RuneCountInString(hint) > limit {
					t.Fatal("invalid character cap")
				}
				for bytes := 1; bytes <= len(hint); bytes++ {
					block := truncatePromptBlock(hint, bytes)
					if !utf8.ValidString(block) || len(block) > bytes {
						t.Fatal("invalid prompt byte cap")
					}
				}
			}
		}
	}
}

func TestCoAgentPromptExamplesAndWriterContract(t *testing.T) {
	for _, dir := range []string{filepath.Join("..", "..", "prompts"), t.TempDir()} {
		cfg := &config.Config{}
		cfg.Directories.PromptsDir = dir
		cfg.CoAgents.Specialists.Writer.AdditionalPrompt = "Keep the requested voice."
		req := CoAgentRequest{Task: "Summarize the supplied material."}
		writer := buildSpecialistSystemPrompt(t.Context(), cfg, "writer", req, nil, nil, nil)
		if !strings.Contains(writer, "no runtime tools") || !strings.Contains(writer, "Keep the requested voice.") || strings.Contains(writer, "Use writing-oriented tools") {
			t.Fatal("writer default/fallback contradicts its runtime or loses custom style")
		}
		for _, prompt := range []string{buildCoAgentSystemPrompt(t.Context(), cfg, req, nil, nil), buildSpecialistSystemPrompt(t.Context(), cfg, "researcher", req, nil, nil, nil)} {
			if strings.Contains(prompt, "duckduckgo_search") || !strings.Contains(prompt, `"skill_name": "ddg_search"`) {
				t.Fatal("delegated prompt names a missing search skill")
			}
		}
	}
	found := false
	for _, tool := range buildCoreToolSchemas(ToolFeatureFlags{}, nil) {
		if tool.Function.Name == "ddg_search" {
			found = true
		}
	}
	if !found {
		t.Fatal("prompt search example has no registered native tool")
	}
}

func TestCoAgentUsageSurvivesTerminalStates(t *testing.T) {
	for _, terminal := range []string{"complete", "fail", "stop", "panic", "timeout"} {
		t.Run(terminal, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			r := NewCoAgentRegistry(1, logger)
			id, _ := r.Register("fixture", func() {})
			var wg sync.WaitGroup
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); r.RecordUsage(id, 2, 1); _, _ = r.GetStatus(id); _ = r.List() }()
			}
			wg.Wait()
			switch terminal {
			case "complete":
				r.Complete(id, "done", 2, 0)
			case "fail":
				r.Fail(id, "fixture error", 0, 0)
			case "stop":
				_ = r.Stop(id)
			case "timeout":
				r.FailIfActive(id, "deadline exceeded")
			case "panic":
				func() { defer recoverCoAgentPanic(r, id, logger); panic("fixture") }()
			}
			r.RecordUsage(id, 3, 0) // A response finishing cancellation is still consumption.
			status, _ := r.GetStatus(id)
			if status["tokens_used"] != 43 || status["tool_calls"] != 20 {
				t.Fatalf("lost usage: %v", status)
			}
			if r.AvailableSlots() != 1 {
				t.Fatal("terminal run held its slot")
			}
		})
	}
}

// Two physical responses, one real read-only tool, and a different main provider.
func TestCoAgentRunReportsCumulativeUsageAndInheritedScope(t *testing.T) {
	testCoAgentUsageRun(t, "success")
}

func TestCoAgentSpawnedHomepageLimitCannotExpand(t *testing.T) {
	testCoAgentUsageRun(t, "homepage_limit")
}

func TestCoAgentFailureKeepsUsageWithoutReplayingTools(t *testing.T) {
	testCoAgentUsageRun(t, "provider_failure")
}

func TestCoAgentRecoveryAccumulatesNormalizedUsage(t *testing.T) {
	testCoAgentUsageRun(t, "empty_retry")
}

func testCoAgentUsageRun(t *testing.T, outcome string) {
	t.Helper()
	homepageLimit := outcome == "homepage_limit"
	run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
	defer cleanup()
	var calls atomic.Int32
	var sawToolResult atomic.Bool
	var sawLimitSkip atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		for _, tool := range req.Tools {
			if tool.Function.Name != "system_metrics" && !(homepageLimit && tool.Function.Name == "read_tool_manual") {
				t.Errorf("out-of-scope schema: %s", tool.Function.Name)
			}
		}
		for _, message := range req.Messages {
			if message.Role == openai.ChatMessageRoleTool && message.ToolCallID == "metrics-call" {
				sawToolResult.Store(true)
			}
			if message.Role == openai.ChatMessageRoleTool && message.ToolCallID == "manual-call" && strings.Contains(message.Content, "tool_batch_stopped") {
				sawLimitSkip.Store(true)
			}
		}
		message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Fixture complete."}
		usage := openai.Usage{PromptTokens: 20, CompletionTokens: 3, TotalTokens: 23}
		finish := openai.FinishReasonStop
		callNumber := calls.Add(1)
		if outcome == "empty_retry" {
			callNumber-- // An empty first response triggers in-loop recovery.
		}
		if outcome == "provider_failure" && callNumber > 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"fixture failure after tool execution","type":"invalid_request_error"}}`)
			return
		}
		if callNumber == 1 {
			message.Content = ""
			message.ToolCalls = []openai.ToolCall{{ID: "metrics-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "system_metrics", Arguments: `{}`}}}
			if homepageLimit {
				// The forbidden Homepage call activates the workflow budget path
				// before role denial, without touching any real Homepage runtime.
				message.ToolCalls = append([]openai.ToolCall{{ID: "homepage-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "homepage_file", Arguments: `{"operation":"list_files","project_dir":"fixture"}`}}}, message.ToolCalls...)
				message.ToolCalls = append(message.ToolCalls, openai.ToolCall{ID: "manual-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "read_tool_manual", Arguments: `{"tool_name":"co_agent"}`}})
			}
			usage = openai.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12}
			finish = openai.FinishReasonToolCalls
		}
		if callNumber == 0 {
			message.Content = ""
			usage = openai.Usage{PromptTokens: 10, CompletionTokens: 2} // Normalize missing total.
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{ID: "fixture", Model: "gpt-4o-mini", Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: finish}}, Usage: usage})
	}))
	defer server.Close()
	cfg := run.Config
	cfg.LLM.Provider, cfg.LLM.ProviderType = "main", "ollama"
	cfg.LLM.UseNativeFunctions = true
	p := config.ProviderEntry{ID: "child", Type: "openai", BaseURL: server.URL + "/v1", APIKey: "fixture-key", Model: "gpt-4o-mini"}
	cfg.Providers = []config.ProviderEntry{p}
	cfg.CoAgents.LLM.Provider, cfg.CoAgents.LLM.ProviderType, cfg.CoAgents.LLM.BaseURL, cfg.CoAgents.LLM.APIKey, cfg.CoAgents.LLM.Model = p.ID, p.Type, p.BaseURL, p.APIKey, p.Model
	cfg.CoAgents.Enabled, cfg.CoAgents.QueueWhenBusy, cfg.CoAgents.MaxConcurrent = true, true, 1
	cfg.CoAgents.CircuitBreaker.MaxToolCalls, cfg.CoAgents.CircuitBreaker.TimeoutSeconds = 3, 20
	cfg.CoAgents.RetryPolicy.MaxRetries = 2
	cfg.CoAgents.RetryPolicy.RetryableErrorPatterns = []string{"fixture failure"}
	allowed := map[string]struct{}{"system_metrics": {}}
	role := ""
	if homepageLimit {
		cfg.CoAgents.CircuitBreaker.MaxToolCalls = 2
		cfg.Homepage.Enabled, cfg.Homepage.CircuitBreakerMaxCalls = true, 75
		cfg.CoAgents.Specialists.Designer.Enabled, role = true, "designer"
		allowed["homepage_file"], allowed["read_tool_manual"] = struct{}{}, struct{}{}
	}
	registry := NewCoAgentRegistry(1, run.Logger)
	parent := newCoAgentSpawnContext(&DispatchContext{Cfg: cfg, Logger: run.Logger, CoAgentRegistry: registry, ShortTermMem: run.ShortTermMem, Registry: run.Registry, SessionID: run.SessionID, UserContext: "Inspect system metrics", ToolScopeRestricted: true, AllowedTools: allowed})
	id, _, err := SpawnCoAgent(parent, CoAgentRequest{Task: "Call system_metrics once and report Fixture complete.", Specialist: role}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	status, err := registry.WaitForResult(ctx, id)
	wantState, wantTokens, wantCalls := string(CoAgentCompleted), 35, int32(2)
	if outcome == "provider_failure" {
		wantState, wantTokens = string(CoAgentFailed), 12
	} else if outcome == "empty_retry" {
		wantTokens, wantCalls = 47, 3
	}
	if err != nil || status["state"] != wantState {
		t.Fatalf("status=%v error=%v", status, err)
	}
	if calls.Load() != wantCalls || !sawToolResult.Load() {
		t.Fatalf("tool round missing: calls=%d", calls.Load())
	}
	if status["tokens_used"] != wantTokens || status["tool_calls"] != 1 {
		t.Fatalf("wrong usage: %v", status)
	}
	if homepageLimit && !sawLimitSkip.Load() {
		t.Fatal("third declared call escaped fixed child budget")
	}
	page, err := run.ShortTermMem.SearchAuditEvents(memory.AuditFilter{Source: memory.AuditSourceAgentTool, Type: agentActionAuditEventType, Limit: 10})
	if err != nil || page.Total < 1 {
		t.Fatalf("missing real tool ledger: page=%v err=%v", page, err)
	}
	for _, entry := range page.Entries {
		var metadata agentActionAuditMetadata
		if err := json.Unmarshal([]byte(entry.MetadataJSON), &metadata); err != nil || metadata.MessageSource != "co_agent" {
			t.Fatalf("delegated ledger attributed incorrectly: %v err=%v", metadata, err)
		}
	}
}

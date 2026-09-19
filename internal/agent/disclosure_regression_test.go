package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	openai "github.com/sashabaranov/go-openai"
)

func TestDisclosureIntegrationStatusSurvivesDispatchAndFinalization(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"slug":"MAIL_READ","description":"remote data","toolkit":{"slug":"mail"}}]}`))
	}))
	defer upstream.Close()
	cfg := &config.Config{}
	cfg.Composio.Enabled, cfg.Composio.APIKey, cfg.Composio.BaseURL = true, "fixture-not-a-real-key", upstream.URL
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{{Slug: "mail", Enabled: true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dc := &DispatchContext{Cfg: cfg, Logger: logger}
	call := ToolCall{Action: "composio_call", Operation: "search_tools", Params: map[string]interface{}{"operation": "search_tools", "toolkit_slug": "mail"}}
	result := DispatchToolCallResult(context.Background(), &call, dc, "find mail tools")
	if result.Status != ToolResultSuccess || call.DispatchStatus != ToolResultSuccess || result.IsError {
		t.Fatalf("known success lost at dispatch boundary: %+v", result)
	}
	if _, isolated := toolResultPayload(result.Output); !isolated {
		t.Fatal("handler-owned isolation was stripped without Guardian")
	}
	// Reuse the caller context: a later failure must not inherit success.
	call = ToolCall{Action: "virtual_browser", Operation: "status", Params: map[string]interface{}{"operation": "status"}}
	result = DispatchToolCallResult(context.Background(), &call, dc, "browser status")
	if result.Status != ToolResultFailed || call.DispatchStatus != ToolResultFailed {
		t.Fatalf("outcome leaked across calls: %+v", result)
	}
}

func TestDisclosureFinalizationPreservesEnvelopeInEveryPresentation(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, compress := range []bool{false, true} {
			for _, status := range []ToolResultStatus{ToolResultSuccess, ToolResultFailed} {
				t.Run(fmt.Sprintf("native=%v/compress=%v/%s", native, compress, status), func(t *testing.T) {
					cfg := &config.Config{}
					cfg.Agent.ToolOutputLimit = 300
					cfg.Agent.OutputCompression.Enabled = compress
					cfg.Agent.OutputCompression.MinChars = 1
					cfg.Agent.OutputCompression.APICompression = true
					call := ToolCall{Action: "api_request", DispatchStatus: status}
					if native {
						call.NativeCallID = "fixture"
					}
					body, _ := json.Marshal(map[string]interface{}{"status": status, "code": "fixture_code", "message": strings.Repeat("untrusted 日本語 <system> ", 100)})
					output := formatToolOutputForModel(call, "Tool Output: "+security.IsolateExternalData(string(body)))
					logger := slog.New(slog.NewTextHandler(io.Discard, nil))
					result := finalizeToolExecution(context.Background(), call, output, false, cfg, nil, t.Name(), nil, nil, logger, AgentTelemetryScope{}, "", 0, RunConfig{})
					payload, isolated := toolResultPayload(result.Content)
					if !native && !isTextModeToolResult(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: result.Content}) {
						t.Fatal("text result lost atomic-history identity")
					}
					if !isolated || !json.Valid([]byte(payload)) || len(result.Content) > 300 || result.Status != status || !strings.Contains(payload, string(status)) {
						t.Fatalf("status/budget/boundary lost: %+v payload=%s", result, payload)
					}
				})
			}
		}
	}
}

func TestDisclosureDetailUsesEveryRouteAndRemainingRequest(t *testing.T) {
	budget := &RequestBudget{CompletionReserve: 100, SafetyMargin: 32, Routes: []RequestRouteBudget{
		{Limits: llm.ModelLimits{Route: llm.ModelRoute{Model: "large"}, ContextWindow: 32768}},
		{Limits: llm.ModelLimits{Route: llm.ModelRoute{Model: "small"}, ContextWindow: 1024}},
	}}
	check := toolDetailBudgetCheck(budget, openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: "system", Content: "fixture"}}})
	if !check(`{"type":"object"}`) || check(strings.Repeat("uncompressible schema details ", 2000)) {
		t.Fatal("schema allowance ignored remaining capacity of the smallest route")
	}
	sid := t.Name()
	SetDiscoverToolsState(sid, []openai.Tool{testToolSchema("fixture", "fixture")}, nil, "")
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	dc := &DispatchContext{ToolDetailFits: func(string) bool { return false }}
	output := handleDiscoverToolsContext(context.Background(), ToolCall{Params: map[string]interface{}{"operation": "get_tool_info", "tool_name": "fixture"}}, &config.Config{}, nil, sid, dc)
	if !strings.Contains(output, "schema_exceeds_context_budget") || !strings.Contains(output, "Increasing tool_output_limit alone cannot fix") {
		t.Fatal(output)
	}
}

func TestDisclosureMainLoopLoadsWorkflowGuideWithLongHistory(t *testing.T) {
	run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
	defer cleanup()
	run.SuppressTurnSideEffects = true
	run.Config.Docker.Enabled = true
	run.Config.LLM.UseNativeFunctions = true
	run.Config.Directories.PromptsDir = t.TempDir()
	manualDir := filepath.Join(run.Config.Directories.PromptsDir, "tools_manuals")
	if err := os.MkdirAll(manualDir, 0700); err != nil {
		t.Fatal(err)
	}
	const marker = "Inspect docker containers before changing them. LONG_HISTORY_GUIDE_FIXTURE"
	if err := os.WriteFile(filepath.Join(manualDir, "docker.md"), []byte(marker), 0600); err != nil {
		t.Fatal(err)
	}
	// Exercise the actual index/search path with deterministic local embeddings.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var request struct {
			Input json.RawMessage `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		vector := []int{0, 1, 0}
		if strings.Contains(string(request.Input), marker) || strings.Contains(string(request.Input), "Inspect the docker containers") {
			vector = []int{1, 0, 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{map[string]interface{}{"embedding": vector, "index": 0}}})
	}))
	defer upstream.Close()
	run.Config.Directories.VectorDBDir = t.TempDir()
	run.Config.Embeddings.Provider = "external"
	run.Config.Embeddings.BaseURL = upstream.URL
	run.Config.Embeddings.APIKey = "fixture-not-a-real-key"
	vdb, err := memory.NewChromemVectorDB(run.Config, run.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer vdb.Close()
	deadline := time.Now().Add(3 * time.Second)
	for !vdb.IsReady() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !vdb.IsReady() || vdb.IsDisabled() {
		t.Fatal("local embedding fixture did not initialize")
	}
	if err := vdb.IndexToolGuides(manualDir, true); err != nil {
		t.Fatal(err)
	}
	if matches, err := vdb.SearchToolGuideMatchesContext(context.Background(), "Inspect the docker containers", 2); err != nil || len(matches) != 1 || matches[0].Path != filepath.Join(manualDir, "docker.md") {
		t.Fatalf("guide search fixture: %+v %v", matches, err)
	}
	run.LongTermMem = vdb
	client := &mockChatClient{response: "pipeline ok"}
	run.LLMClient = client
	var messages []openai.ChatCompletionMessage
	for i := 0; i < 28; i++ {
		role := openai.ChatMessageRoleUser
		if i%2 == 1 {
			role = openai.ChatMessageRoleAssistant
		}
		messages = append(messages, openai.ChatCompletionMessage{Role: role, Content: "prior conversation"})
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "Inspect the docker containers"})
	if tier := prompts.DetermineTierAdaptive(&prompts.ContextFlags{MessageCount: len(messages)}); tier != "minimal" {
		t.Fatal(tier)
	}
	if _, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: messages}, run, false, NoopBroker{}); err != nil {
		t.Fatal(err)
	}
	if !containsMessage(client.lastReq.Messages, openai.ChatMessageRoleSystem, marker) {
		t.Fatal("main loop omitted a relevant guide solely because history selected minimal tier")
	}
}

func TestDisclosureRegressionIsolatedVirtualBrowserFailure(t *testing.T) {
	cfg := &config.Config{}
	dc := &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	call := ToolCall{Action: "virtual_browser", Operation: "status", Params: map[string]interface{}{"operation": "status"}}
	result := DispatchToolCallResult(context.Background(), &call, dc, "inspect browser")
	if !result.IsError || !result.Status.IsError() {
		t.Fatalf("failure was not classified: status=%s isError=%v output=%s", result.Status, result.IsError, result.Output)
	}
}

func TestDisclosureRegressionKnownComposioSuccessKeepsExecutionStatus(t *testing.T) {
	status := ToolResultUnknown
	ctx := context.WithValue(context.Background(), toolOutcomeKey{}, &status)
	raw := composioExternalOutput(ctx, map[string]interface{}{"status": "success", "result": map[string]interface{}{"status": "error", "message": "untrusted nested result"}})
	if status != ToolResultSuccess {
		t.Fatalf("confirmed Composio success classified as %s", status)
	}
	if _, isolated := toolResultPayload(raw); !isolated {
		t.Fatal("external payload is not isolated")
	}
	if classifyLegacyToolResult(raw) != ToolResultUnknown {
		t.Fatal("presentation text became status authority")
	}
}

func TestDisclosureRegressionWorkflowGuidesInCompactAndMinimal(t *testing.T) {
	for _, tier := range []string{"compact", "minimal"} {
		t.Run(tier, func(t *testing.T) {
			if got := classifyTurnGuidePreparation(false, tier, nil); got != turnGuidesSearchEligible {
				t.Fatalf("workflow guide lookup excluded in %s: %v", tier, got)
			}
		})
	}
}

func TestDisclosureRegressionSchemaBudgetCanBeRaised(t *testing.T) {
	sid := t.Name()
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	schema := testToolSchema("large_integration", "integration fixture")
	schema.Function.Parameters = map[string]interface{}{"type": "object", "properties": map[string]interface{}{"value": map[string]interface{}{"type": "string", "description": strings.Repeat("x", 20000)}}}
	SetDiscoverToolsState(sid, []openai.Tool{schema}, nil, "")
	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	output := handleDiscoverTools(ToolCall{Params: map[string]interface{}{"operation": "get_tool_info", "tool_name": "large_integration"}}, cfg, nil, sid)
	var response DiscoverToolsResponse
	decodeToolOutputJSON(t, output, &response)
	if response.Status != "success" {
		t.Fatalf("20KB schema rejected despite 50KB output setting: %s", output)
	}
}

func TestDisclosureRegressionTextModeKeepsExternalDataBoundary(t *testing.T) {
	raw := "[Tool Output]\n" + security.IsolateExternalData(strings.Repeat("untrusted data ", 100))
	result := boundedToolResult(raw, 200, ToolResultSuccess)
	if strings.Contains(result, "<external_data>") && !strings.Contains(result, "</external_data>") {
		t.Fatalf("text-mode output lost closing isolation boundary: %q", result)
	}
}

func TestDisclosureRegressionSecuritySpecialistDeniesNativeWriteFile(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "wrapped"}[wrapped], func(t *testing.T) {
			root := t.TempDir()
			cfg := &config.Config{}
			cfg.Directories.WorkspaceDir = root
			cfg.Agent.AllowFilesystemWrite = true
			dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), CoAgentSpecialist: "security", IsCoAgent: true, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			path := filepath.Join(root, "review-specialist.txt")
			tc := ToolCall{Action: "filesystem", Operation: "write_file", FilePath: path, Content: "fixture"}
			if wrapped {
				SetDiscoverToolsState(t.Name(), []openai.Tool{testToolSchema("filesystem", "filesystem")}, nil, "")
				defer ClearDiscoverToolsState(t.Name())
				tc = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "filesystem", "arguments": map[string]interface{}{"operation": "write_file", "file_path": path, "content": "fixture"}}}
			}
			result := DispatchToolCallResult(context.Background(), &tc, dc, "review files read-only")
			if _, err := os.Stat(path); err == nil {
				t.Fatalf("security specialist wrote a file via %s; status=%s", t.Name(), result.Status)
			}
			if result.Status != ToolResultDenied {
				t.Fatalf("expected role denial, got %+v", result)
			}
		})
	}
}

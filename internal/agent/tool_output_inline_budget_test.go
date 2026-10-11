package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/localwiki"
	"aurago/internal/memory"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

func TestAutoToolResultInlineBudget(t *testing.T) {
	for _, tc := range []struct {
		name        string
		inputTokens int
		batch       int
		want        int
	}{
		{"unknown context", 0, 1, 6000},
		{"negative", -1, 1, 6000},
		{"conservative 32k default", 32768 - 4096 - 256, 1, 6000},
		{"64k", 65536 - 8192 - 256, 1, 6850},
		{"128k", 131072 - 8192 - 256, 1, 14714},
		{"256k", 256000 - 8192 - 256, 0, 29706},
		{"1M ceiling", 1_000_000, 1, 32000},
		// Parallel calls share the next request: budget / sqrt(calls).
		{"256k, three calls", 256000 - 8192 - 256, 3, 17150},
		{"1M, four calls", 1_000_000, 4, 16000},
		{"128k, sixteen calls keep the floor", 131072 - 8192 - 256, 16, 6000},
	} {
		basis := inlineBudgetBasis{InputTokens: tc.inputTokens, BatchCalls: tc.batch}
		if got := autoToolResultInlineBudget(basis); got != tc.want {
			t.Errorf("%s: autoToolResultInlineBudget(%+v) = %d, want %d", tc.name, basis, got, tc.want)
		}
	}
}

func TestRequestBudgetMinInputLimitUsesTightestRoute(t *testing.T) {
	var none *RequestBudget
	if got := none.minInputLimit(); got != 0 {
		t.Fatalf("nil budget = %d", got)
	}
	budget := &RequestBudget{CompletionReserve: 8192, SafetyMargin: 256, Routes: []RequestRouteBudget{
		{Limits: llm.ModelLimits{ContextWindow: 256000}},
		{Limits: llm.ModelLimits{ContextWindow: 131072}},
	}}
	if got := budget.minInputLimit(); got != 131072-8192-256 {
		t.Fatalf("minInputLimit = %d, want the failover route's capacity", got)
	}
}

// The agent loop hands the same basis to dispatch (paging tools) and to
// finalizeToolExecution (the vault threshold).
func TestAgentLoopDispatchCarriesTheInlineBudgetBasis(t *testing.T) {
	cfg := outputVaultTestConfig(0)
	s := &agentLoopState{runCfg: RunConfig{Config: cfg}, toolBatchCalls: 3, requestBudget: &RequestBudget{
		CompletionReserve: 8192, SafetyMargin: 256,
		Routes: []RequestRouteBudget{{Limits: llm.ModelLimits{ContextWindow: 256000}}},
	}}
	if got := toolResultInlineBudget(s.makeDispatchContext(testLogger)); got != 17150 {
		t.Fatalf("dispatch budget for three calls on a 256k route = %d", got)
	}
	if got := primaryOutputVaultMaxInline(cfg, s.inlineBudgetBasis()); got != 17150 {
		t.Fatalf("vault threshold = %d, want the dispatch budget", got)
	}
}

// The minimal loop sizes paging tools from its own request budget.
func TestMinimalLoopDispatchUsesTheRequestContext(t *testing.T) {
	lib := &agentWikiLibrary{}
	useAgentWikiSource(t, &agentWikiSource{lib: lib, open: true})
	cfg := outputVaultTestConfig(0)
	read := func(dc *DispatchContext) int {
		_, _ = dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Params: map[string]interface{}{"operation": "read", "path": "Berlin"}}, dc)
		return lib.read.PageRunes
	}
	floorPage := read(&DispatchContext{Cfg: cfg, Logger: testLogger})

	client := &minimalLoopRouteClient{routes: []llm.ModelRoute{{ProviderID: "big", ProviderType: "custom", Model: "big-model", Primary: true, ContextWindowOverride: 256000, MaxOutputTokensOverride: 4096}}}
	client.respond = func(req openai.ChatCompletionRequest, call int) (openai.ChatCompletionResponse, error) {
		msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "done"}
		if call == 1 {
			msg = openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
				ID: "call-wiki", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "local_wikipedia", Arguments: `{"operation":"read","path":"Berlin"}`},
			}}}
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg}}}, nil
	}
	tool := openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "local_wikipedia", Parameters: map[string]any{"type": "object"}}}
	lib.read = localwiki.ReadRequest{}
	if _, _, err := ExecuteMinimalLoop(context.Background(), client, "big-model", MinimalSystemPromptBuilder(nil), "read Berlin", []openai.Tool{tool}, &DispatchContext{Cfg: cfg, LLMClient: client, SessionID: t.Name()}, nil, testLogger, &MinimalLoopOptions{MaxToolRounds: 1}); err != nil {
		t.Fatal(err)
	}
	if lib.read.PageRunes <= floorPage {
		t.Fatalf("page of %d runes on a 256k route, %d without a request: the loop did not pass its context", lib.read.PageRunes, floorPage)
	}
}

// vaultTestRun archives raw through finalizeToolExecution with the vault on
// and an automatic inline limit for inputTokens.
func vaultTestRun(t *testing.T, action, raw string, inputTokens int) (toolExecutionResult, *DispatchContext) {
	t.Helper()
	stm, err := memory.NewSQLiteMemory(":memory:", testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stm.Close() })
	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	cfg.Agent.OutputCompression.Reversible.Enabled = true
	cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault = true
	tc := ToolCall{Action: action, NativeCallID: "call_" + action, DispatchStatus: ToolResultSuccess}
	result := finalizeToolExecution(context.Background(), tc, raw, false, cfg, stm, t.Name(), nil, nil, testLogger, AgentTelemetryScope{}, "", 0, RunConfig{}, inlineBudgetBasis{InputTokens: inputTokens})
	if result.OutputRef == "" {
		t.Fatalf("%d bytes were not archived", len(raw))
	}
	dc := &DispatchContext{Cfg: cfg, Logger: testLogger, ShortTermMem: stm, SessionID: t.Name(), inlineBasis: inlineBudgetBasis{InputTokens: inputTokens}, Guardian: security.NewGuardian(nil)}
	return result, dc
}

// readOutput dispatches read_tool_output like the agent does and decodes the
// reply (the Guardian isolates it as external output).
func readOutput(t *testing.T, dc *DispatchContext, params map[string]interface{}) (readToolOutputResponse, string) {
	t.Helper()
	tc := ToolCall{Action: "read_tool_output", NativeCallID: "read", Params: params}
	dispatched := DispatchToolCallResult(context.Background(), &tc, dc, "")
	payload, _ := vaultReadablePayload(dispatched.Output)
	var resp readToolOutputResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("read reply is not JSON: %v: %.300s", err, dispatched.Output)
	}
	return resp, dispatched.Output
}

func TestOutputVaultViewShowsHeadAndTailOfALog(t *testing.T) {
	var lines []string
	for i := 1; i <= 2000; i++ {
		lines = append(lines, fmt.Sprintf("2026-10-11T10:%02d:%02d INFO step %04d finished in 12ms", i/60%60, i%60, i))
	}
	lines[1999] = "2026-10-11T11:00:00 ERROR deploy failed: exit status 1"
	raw := strings.Join(lines, "\n")
	result, dc := vaultTestRun(t, "execute_shell", raw, 1_000_000)

	maxInline := primaryOutputVaultMaxInline(dc.Cfg, dc.inlineBasis)
	if len(result.Content) > maxInline || len(result.Content) < maxInline*9/10 {
		t.Fatalf("vault reply is %d bytes, want most of the %d-byte inline budget", len(result.Content), maxInline)
	}
	var reply outputVaultPayload
	if err := json.Unmarshal([]byte(strings.TrimPrefix(result.Content, "Tool Output: ")), &reply); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reply.View, lines[0]+"\n") || !strings.HasSuffix(reply.View, "\n"+lines[1999]) || !strings.Contains(reply.View, "lines omitted ...]") {
		t.Fatalf("view lacks head, tail or omission marker: %.200s ... %.200s", reply.View, reply.View[len(reply.View)-200:])
	}
	if !strings.Contains(reply.Message, `view="range", start_line=`) || !strings.Contains(reply.Message, result.OutputRef) {
		t.Fatalf("message does not say how to read the omitted lines: %s", reply.Message)
	}

	// The range the message names returns exactly the omitted lines.
	var start, end int
	if _, err := fmt.Sscanf(reply.Message[strings.Index(reply.Message, "start_line="):], "start_line=%d, end_line=%d", &start, &end); err != nil {
		t.Fatal(err)
	}
	resp, _ := readOutput(t, dc, map[string]interface{}{"ref": result.OutputRef, "view": "range", "start_line": float64(start), "end_line": float64(start + 9)})
	if resp.Status != "success" || resp.Content != strings.Join(lines[start-1:start+9], "\n") {
		t.Fatalf("range read = %+v", resp)
	}
}

func TestOutputVaultViewPagesThroughOneLineJSON(t *testing.T) {
	type container struct {
		ID, Name, Image, State, Status string
	}
	var containers []container
	for i := range 400 {
		containers = append(containers, container{fmt.Sprintf("%012x", i*7919), fmt.Sprintf("/svc-%d", i), "caddy:2.11.2-alpine", "running", "Up 14 minutes"})
	}
	body, _ := json.Marshal(map[string]interface{}{"containers": containers})
	payload := "Tool Output: " + string(body)
	// Like docker: the Guardian isolates the whole handler output.
	raw := security.NewGuardian(nil).SanitizeToolOutput("docker", payload)
	result, dc := vaultTestRun(t, "docker", raw, 247_552)

	maxInline := primaryOutputVaultMaxInline(dc.Cfg, dc.inlineBasis)
	if len(result.Content) > maxInline || len(result.Content) < maxInline*9/10 {
		t.Fatalf("vault reply is %d bytes, want most of the %d-byte inline budget", len(result.Content), maxInline)
	}
	// A twice-escaped quote shows up as a JSON-escaped entity or as &amp;#34;.
	if strings.Contains(result.Content, "\x5cu0026#34;") || strings.Contains(result.Content, "&amp;#34;") {
		t.Fatalf("vault view escapes the isolated output twice: %.300s", result.Content)
	}
	if strings.Count(result.Content, "</external_data>") != 1 {
		t.Fatalf("vault reply lost its isolation: %.300s", result.Content)
	}
	view, isolated := vaultReadablePayload(result.Content)
	var reply outputVaultPayload
	if err := json.Unmarshal([]byte(view), &reply); err != nil || !isolated {
		t.Fatalf("vault reply is not isolated JSON (%v): %.300s", err, result.Content)
	}
	if !strings.HasPrefix(string(body), reply.View) || len(reply.View) < len(body)/4 || reply.OriginalChars != len(body) {
		t.Fatalf("view is not a large prefix of the readable output: %d of %d bytes", len(reply.View), len(body))
	}
	var offset int
	if _, err := fmt.Sscanf(reply.Message[strings.Index(reply.Message, "offset="):], "offset=%d", &offset); err != nil || offset != len(reply.View) {
		t.Fatalf("message offset %d (%v), want %d: %s", offset, err, len(reply.View), reply.Message)
	}

	// Paging from the named offset (alias output_ref, no view) returns the rest.
	text := reply.View
	budget := readToolOutputBudget(dc)
	for pages := 0; offset > 0; pages++ {
		if pages > 10 {
			t.Fatal("paging does not end")
		}
		resp, model := readOutput(t, dc, map[string]interface{}{"output_ref": result.OutputRef, "offset": float64(offset)})
		if resp.Status != "success" || resp.Offset != offset || resp.TotalChars != len(body) {
			t.Fatalf("page at %d = %+v", offset, resp)
		}
		if len(model) > budget+readToolOutputReserve {
			t.Fatalf("page is %d bytes as the model sees it, budget %d", len(model), budget)
		}
		text += resp.Content
		offset = resp.NextOffset
	}
	if text != string(body) {
		t.Fatalf("pages do not reassemble the output: %d of %d bytes", len(text), len(body))
	}

	// jsonpath and grep work on the tool's own JSON, not its escaped form.
	resp, _ := readOutput(t, dc, map[string]interface{}{"ref": result.OutputRef, "view": "jsonpath", "query": "$.containers[399].Name"})
	if resp.Content != `"/svc-399"` {
		t.Fatalf("jsonpath = %+v", resp)
	}
	resp, _ = readOutput(t, dc, map[string]interface{}{"ref": result.OutputRef, "view": "grep", "query": `"Name":"/svc-250"`})
	if !strings.HasPrefix(resp.Content, "[line 1, byte ") || !strings.Contains(resp.Content, `"Name":"/svc-250"`) || len(resp.Content) > 2*grepContextBytes+200 {
		t.Fatalf("grep on a long line = %q", resp.Content)
	}
}

func TestReadToolOutputArgumentsAndViews(t *testing.T) {
	raw := strings.Repeat("ü", 4000) + " the end"
	result, dc := vaultTestRun(t, "api_request", raw, 0)

	if resp, _ := readOutput(t, dc, map[string]interface{}{"ref": result.OutputRef, "view": "tail", "max_chars": float64(10)}); resp.Content != "ü the end" || !resp.Truncated {
		t.Fatalf("tail of one long line = %+v", resp)
	}
	if resp, _ := readOutput(t, dc, map[string]interface{}{"output_ref": result.OutputRef, "limit": float64(9)}); resp.Content != "üüüü" || resp.NextOffset != 8 {
		t.Fatalf("limit alias / UTF-8 safe page = %+v", resp)
	}
	if resp, _ := readOutput(t, dc, map[string]interface{}{"view": "full"}); resp.Status != "error" || !strings.Contains(resp.Message, "output_ref") {
		t.Fatalf("missing ref = %+v", resp)
	}

	// Through invoke_tool a model may name the archive output_ref.
	SetDiscoverToolsState(dc.SessionID, []openai.Tool{testToolSchema("read_tool_output", "read archived output")}, nil, "")
	defer ClearDiscoverToolsState(dc.SessionID)
	tc := ToolCall{Action: "invoke_tool", NativeCallID: "wrapped", Params: map[string]interface{}{
		"tool_name": "read_tool_output",
		"arguments": map[string]interface{}{"output_ref": result.OutputRef, "offset": float64(0), "limit": float64(4)},
	}}
	dispatched := DispatchToolCallResult(context.Background(), &tc, dc, "")
	payload, _ := vaultReadablePayload(dispatched.Output)
	var resp readToolOutputResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil || resp.Status != "success" || resp.Content != "üü" {
		t.Fatalf("invoke_tool read (%v) = %s", err, dispatched.Output)
	}
}

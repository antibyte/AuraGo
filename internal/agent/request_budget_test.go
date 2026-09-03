package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/prompts"

	openai "github.com/sashabaranov/go-openai"
)

type budgetRouteClient struct {
	routes    []llm.ModelRoute
	routesFor func(openai.ChatCompletionRequest) []llm.ModelRoute
}

type budgetPlainClient struct{}

func (*budgetPlainClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{}, errors.New("not implemented")
}

func (*budgetPlainClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (c *budgetRouteClient) CandidateRoutes(req openai.ChatCompletionRequest) []llm.ModelRoute {
	if c.routesFor != nil {
		return append([]llm.ModelRoute(nil), c.routesFor(req)...)
	}
	return append([]llm.ModelRoute(nil), c.routes...)
}

func (c *budgetRouteClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{}, errors.New("not implemented")
}

func (c *budgetRouteClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func budgetTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewRequestBudgetReasoningAndSmallestFailoverLimits(t *testing.T) {
	tests := []struct {
		name        string
		routes      []llm.ModelRoute
		maxTokens   int
		wantReserve int
	}{
		{
			name:        "known reasoning default",
			routes:      []llm.ModelRoute{{ProviderType: "openai", Model: "o3", Primary: true}},
			wantReserve: llm.ReasoningOutputTokens,
		},
		{
			name: "smaller failover output wins",
			routes: []llm.ModelRoute{
				{ProviderType: "custom", Model: "primary", ContextWindowOverride: 64000, MaxOutputTokensOverride: 12000, Primary: true},
				{ProviderType: "custom", Model: "fallback", ContextWindowOverride: 16000, MaxOutputTokensOverride: 2048},
			},
			wantReserve: 2048,
		},
		{
			name:      "requested max tokens is capped",
			routes:    []llm.ModelRoute{{ProviderType: "custom", Model: "speech-route", ContextWindowOverride: 12000, MaxOutputTokensOverride: 3000, Primary: true}},
			maxTokens: 6000, wantReserve: 3000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &budgetRouteClient{routes: tt.routes}
			budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{MaxTokens: tt.maxTokens}, budgetTestLogger())
			if err != nil {
				t.Fatal(err)
			}
			if budget.CompletionReserve != tt.wantReserve {
				t.Fatalf("completion reserve = %d, want %d", budget.CompletionReserve, tt.wantReserve)
			}
		})
	}
}

func TestNewRequestBudgetUsesSpeechLabStyleSelectedSingleRoute(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "speech-selected"
	cfg.LLM.ProviderType = "custom"
	cfg.LLM.Model = "speech-small"
	cfg.Providers = []config.ProviderEntry{{
		ID: "speech-selected", Type: "custom", Model: "speech-small",
		ContextWindow: 6000, MaxOutputTokens: 900,
	}}
	budget, err := newRequestBudget(context.Background(), cfg, &budgetPlainClient{}, openai.ChatCompletionRequest{Model: "speech-small"}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if len(budget.Routes) != 1 || budget.Routes[0].Limits.Route.ProviderID != "speech-selected" {
		t.Fatalf("routes=%+v, want selected Speech Lab-style route only", budget.Routes)
	}
	if budget.CompletionReserve != 900 {
		t.Fatalf("completion reserve=%d, want selected route output limit 900", budget.CompletionReserve)
	}
}

func TestRequestBudgetShedsOnlyOptionalSchemasBeforePromptBuild(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "tiny", Primary: true,
		ContextWindowOverride: 5200, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "current request"}}
	tools := []openai.Tool{
		budgetTestTool("required", strings.Repeat("required schema ", 300)),
		budgetTestTool("optional_one", strings.Repeat("optional schema one ", 600)),
		budgetTestTool("optional_two", strings.Repeat("optional schema two ", 600)),
	}
	kept, dropped, err := budget.shedOptionalTools(messages, tools, map[string]bool{"required": true}, "tiny", newTokenCountCache(128))
	if err != nil {
		t.Fatal(err)
	}
	if len(dropped) == 0 {
		t.Fatal("expected optional schemas to be shed")
	}
	foundRequired := false
	for _, tool := range kept {
		if tool.Function != nil && tool.Function.Name == "required" {
			foundRequired = true
		}
	}
	if !foundRequired {
		t.Fatal("required schema was shed")
	}
	if _, err := budget.systemPromptBudget(messages, kept, "tiny", newTokenCountCache(128)); err != nil {
		t.Fatalf("shed schemas still leave no prompt budget: %v", err)
	}
}

func TestSystemPromptBudgetConvertsSmallestRouteToBuildModel(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{
		{ProviderType: "custom", Model: "plain-primary", Primary: true, ContextWindowOverride: 6000, MaxOutputTokensOverride: 1000},
		{ProviderType: "anthropic", Model: "claude-fallback", ContextWindowOverride: 6000, MaxOutputTokensOverride: 1000},
	}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{Model: "plain-primary"}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	got, err := budget.systemPromptBudget(nil, nil, "plain-primary", newTokenCountCache(64))
	if err != nil {
		t.Fatal(err)
	}
	routeAllowance := budget.inputLimit(budget.Routes[1].Limits) - 4
	if got >= routeAllowance {
		t.Fatalf("converted budget = %d, want below Claude route allowance %d", got, routeAllowance)
	}
	if translated := prompts.TranslateTokenBudget(routeAllowance, "claude-fallback", "plain-primary"); got != translated {
		t.Fatalf("converted budget = %d, want %d", got, translated)
	}
}

func TestPrepareRequestBudgetRefreshesRoutesAfterSchemaShedding(t *testing.T) {
	primary := llm.ModelRoute{
		ProviderType: "custom", Model: "tool-primary", Primary: true,
		ContextWindowOverride: 5200, MaxOutputTokensOverride: 1000,
	}
	fallback := llm.ModelRoute{
		ProviderType: "custom", Model: "small-no-tools",
		ContextWindowOverride: 3000, MaxOutputTokensOverride: 700,
	}
	client := &budgetRouteClient{routesFor: func(req openai.ChatCompletionRequest) []llm.ModelRoute {
		if len(req.Tools) > 0 {
			return []llm.ModelRoute{primary}
		}
		return []llm.ModelRoute{primary, fallback}
	}}
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "current request"}},
		Tools:    []openai.Tool{budgetTestTool("optional", strings.Repeat("large optional schema ", 1800))},
	}
	budget, tools, dropped, err := prepareRequestBudgetAndTools(context.Background(), &config.Config{}, client, req, nil, newTokenCountCache(128), budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 0 || len(dropped) != 1 || dropped[0] != "optional" {
		t.Fatalf("tools=%d dropped=%v, want optional schema removed", len(tools), dropped)
	}
	if len(budget.Routes) != 2 {
		t.Fatalf("routes=%d, want refreshed no-tool fallback", len(budget.Routes))
	}
	if budget.CompletionReserve != 700 {
		t.Fatalf("completion reserve=%d, want smaller fallback limit 700", budget.CompletionReserve)
	}
	if _, err := budget.validate(req.Messages, tools, newTokenCountCache(128)); err != nil {
		t.Fatalf("refreshed request does not fit every route: %v", err)
	}
}

func TestAllowedToolsAreScopeNotMandatorySchemas(t *testing.T) {
	required := requiredToolSchemasForState(&agentLoopState{
		runCfg: RunConfig{AllowedTools: []string{"optional_scoped_tool"}},
	})
	if required["optional_scoped_tool"] {
		t.Fatal("AllowedTools must not force every scoped schema into the request")
	}
}

func TestRequiredToolSchemasIncludeHardChannelAndExpandedConfigTools(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.AlwaysInclude = []string{"shell"}
	state := &agentLoopState{
		runCfg:        RunConfig{Config: cfg, MessageSource: "virtual_desktop_chat"},
		toolingPolicy: buildToolingPolicy(cfg, "run a command"),
	}
	required := requiredToolSchemasForState(state)
	for _, name := range []string{"discover_tools", "invoke_tool", "execute_skill", "run_tool", "question_user", "execute_shell"} {
		if !required[name] {
			t.Fatalf("required schema %q missing from %v", name, required)
		}
	}
	if required["shell"] {
		t.Fatal("unexpanded shell alias was marked required instead of execute_shell")
	}
	if required["activate_tools"] {
		t.Fatal("legacy activate_tools handler must not be reserved as a model-visible schema")
	}
}

func TestValidateMinimumRequestBudgetUsesOnlyMandatoryNativeSchemas(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "gpt-4o"
	cfg.Agent.ContextWindow = 8192
	cfg.LLM.UseNativeFunctions = true
	largeOptional := budgetTestTool("optional_large", strings.Repeat("optional schema ", 5000))
	requiredDiscover := budgetTestTool("discover_tools", "required discovery schema")
	runCfg := RunConfig{
		Config: cfg, Logger: budgetTestLogger(), LLMClient: &budgetPlainClient{},
		NativeToolSchemas: []openai.Tool{requiredDiscover, largeOptional},
		UserIntent:        "inspect available tools",
	}
	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "inspect available tools"}},
		Tools:    []openai.Tool{largeOptional},
	}
	if err := ValidateMinimumRequestBudget(context.Background(), req, runCfg); err != nil {
		t.Fatalf("optional native schema caused false preflight rejection: %v", err)
	}
	minimumTools := minimumBudgetToolSchemas(runCfg, req)
	if len(minimumTools) != 1 || minimumTools[0].Function == nil || minimumTools[0].Function.Name != "discover_tools" {
		t.Fatalf("minimum schemas = %#v, want only discover_tools", minimumTools)
	}
}

func TestRequestBudgetChronologicalTrimKeepsAtomicToolGroups(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "group-test", Primary: true,
		ContextWindowOverride: 5000, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	cache := newTokenCountCache(512)
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: strings.Repeat("system ", 550)}}
	for turn := 0; turn < 8; turn++ {
		callID := "call-" + string(rune('a'+turn))
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: strings.Repeat("user request ", 120)},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: callID, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool", Arguments: `{}`}}}},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: callID, Content: strings.Repeat("tool result ", 120)},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: strings.Repeat("turn answer ", 80)},
		)
	}

	trimmed, dropped, err := budget.trimHistory(messages, nil, false, budgetTestLogger(), cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(dropped) == 0 {
		t.Fatal("expected old turns to be dropped")
	}
	if _, droppedToolMessages := SanitizeToolMessages(trimmed); droppedToolMessages != 0 {
		t.Fatalf("trim split a tool group; sanitizer would drop %d messages", droppedToolMessages)
	}
	groups := buildConversationGroups(trimmed)
	if len(groups) == 0 || groups[len(groups)-1].end != len(trimmed) {
		t.Fatalf("latest complete group was not retained: %#v", groups)
	}
	if usage, err := budget.validate(trimmed, nil, cache); err != nil {
		t.Fatalf("trimmed request still exceeds route: usage=%+v err=%v", usage, err)
	}
}

func TestRequestBudgetImportanceAndRecapStayWithinEveryRoute(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{
		{ProviderType: "custom", Model: "primary", Primary: true, ContextWindowOverride: 9000, MaxOutputTokensOverride: 1800},
		{ProviderType: "custom", Model: "fallback", ContextWindowOverride: 6200, MaxOutputTokensOverride: 1400},
	}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	cache := newTokenCountCache(1024)
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: strings.Repeat("core ", 500)}}
	for i := 0; i < 100; i++ {
		role := openai.ChatMessageRoleAssistant
		if i%2 == 0 {
			role = openai.ChatMessageRoleUser
		}
		messages = append(messages, openai.ChatCompletionMessage{Role: role, Content: strings.Repeat("historical context ", 30)})
	}
	trimmed, dropped, err := budget.trimHistory(messages, nil, true, budgetTestLogger(), cache)
	if err != nil {
		t.Fatal(err)
	}
	trimmed = appendRecapWithinBudget(budget, trimmed, nil, dropped, cache)
	usage, err := budget.validate(trimmed, nil, cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range usage {
		if route.InputTokens+route.CompletionTokens+route.SafetyTokens > route.ContextWindow {
			t.Fatalf("route invariant failed: %+v", route)
		}
	}
}

func TestRequestBudgetImportanceTrimKeepsAtomicToolGroups(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "importance-groups", Primary: true,
		ContextWindowOverride: 5000, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	cache := newTokenCountCache(512)
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: strings.Repeat("system ", 550)}}
	for turn := 0; turn < 8; turn++ {
		callID := fmt.Sprintf("importance-call-%d", turn)
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: strings.Repeat("user request ", 120)},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
				ID: callID, Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "test_tool", Arguments: `{}`},
			}}},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: callID, Content: strings.Repeat("tool result ", 120)},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: strings.Repeat("turn answer ", 80)},
		)
	}

	trimmed, dropped, err := budget.trimHistory(messages, nil, true, budgetTestLogger(), cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(dropped) == 0 {
		t.Fatal("expected importance trimming to remove old groups")
	}
	if _, droppedToolMessages := SanitizeToolMessages(trimmed); droppedToolMessages != 0 {
		t.Fatalf("importance trim split a tool group; sanitizer would drop %d messages", droppedToolMessages)
	}
	groups := buildConversationGroups(trimmed)
	if len(groups) == 0 || groups[len(groups)-1].end != len(trimmed) {
		t.Fatalf("latest complete group was not retained: %#v", groups)
	}
	if usage, err := budget.validate(trimmed, nil, cache); err != nil {
		t.Fatalf("importance-trimmed request exceeds route: usage=%+v err=%v", usage, err)
	}
}

func TestRequestBudgetImpossibleMinimalRequest(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "impossible", Primary: true,
		ContextWindowOverride: 4096, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: strings.Repeat("latest mandatory request ", 4000)}}
	err = budget.validateMinimum(messages, nil, newTokenCountCache(128))
	if !IsContextBudgetExceeded(err) {
		t.Fatalf("error = %v, want context_budget_exceeded", err)
	}
	if strings.Contains(err.Error(), "latest mandatory request") {
		t.Fatal("typed error leaked prompt content")
	}
}

func TestRequestBudgetCountsStructuredToolCallArguments(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "structured-accounting", Primary: true,
		ContextWindowOverride: 4096, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "run the requested operation"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
			ID:   "call-large",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "test_tool",
				Arguments: `{"payload":"` + strings.Repeat("large structured argument ", 4000) + `"}`,
			},
		}}},
	}
	if err := budget.validateMinimum(messages, nil, newTokenCountCache(128)); !IsContextBudgetExceeded(err) {
		t.Fatalf("error = %v, want structured arguments to trigger context_budget_exceeded", err)
	}
}

func TestMinimumRequestMessagesPreservesTrustedSystemAddenda(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "trusted internal addendum"},
		{Role: openai.ChatMessageRoleUser, Content: "old request"},
		{Role: openai.ChatMessageRoleAssistant, Content: "old answer"},
		{Role: openai.ChatMessageRoleUser, Content: "current request"},
		{Role: openai.ChatMessageRoleAssistant, Content: "current answer"},
	}
	minimal := minimumRequestMessages(messages)
	if len(minimal) != 3 {
		t.Fatalf("minimal message count = %d, want system addendum plus latest group", len(minimal))
	}
	if minimal[0].Role != openai.ChatMessageRoleSystem || minimal[0].Content != "trusted internal addendum" {
		t.Fatalf("trusted system addendum was not preserved: %#v", minimal)
	}
	if minimal[1].Content != "current request" || minimal[2].Content != "current answer" {
		t.Fatalf("latest complete group was not preserved: %#v", minimal)
	}
}

func TestRequestBudgetFiveMessagesFitWithoutTrimming(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "normal", Primary: true,
		ContextWindowOverride: 16000, MaxOutputTokensOverride: 2000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "one"},
		{Role: openai.ChatMessageRoleAssistant, Content: "two"},
		{Role: openai.ChatMessageRoleUser, Content: "three"},
		{Role: openai.ChatMessageRoleAssistant, Content: "four"},
	}
	trimmed, dropped, err := budget.trimHistory(messages, nil, false, budgetTestLogger(), newTokenCountCache(64))
	if err != nil || len(dropped) != 0 || len(trimmed) != len(messages) {
		t.Fatalf("unexpected trim: len=%d dropped=%d err=%v", len(trimmed), len(dropped), err)
	}
}

func TestHistoryWorkingSetLimitUsesSmallestRouteAndClamp(t *testing.T) {
	tests := []struct {
		name   string
		routes []llm.ModelRoute
		want   int
	}{
		{"small model uses available capacity", []llm.ModelRoute{{Model: "small", ContextWindowOverride: 32768, MaxOutputTokensOverride: 4096}}, 32768 - 4096 - requestProtocolSafetyTokens},
		{"128k model uses seventy percent", []llm.ModelRoute{{Model: "medium", ContextWindowOverride: 128000, MaxOutputTokensOverride: 4096}}, 89600},
		{"Agnes sized model caps at 128k", []llm.ModelRoute{{Model: "agnes-2.5-flash", ContextWindowOverride: 524288, MaxOutputTokensOverride: 8192}}, 131072},
		{"smaller failover wins", []llm.ModelRoute{
			{Model: "large", ContextWindowOverride: 524288, MaxOutputTokensOverride: 8192, Primary: true},
			{Model: "fallback", ContextWindowOverride: 100000, MaxOutputTokensOverride: 4096},
		}, 70000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget, err := newRequestBudget(context.Background(), &config.Config{}, &budgetRouteClient{routes: tt.routes}, openai.ChatCompletionRequest{}, budgetTestLogger())
			if err != nil {
				t.Fatal(err)
			}
			if got := budget.historyWorkingSetLimit("", nil, newTokenCountCache(64)); got != tt.want {
				t.Fatalf("working history limit = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHistoryWorkingSetPreservesLargeCurrentRequestAndAtomicOldTurns(t *testing.T) {
	budget, err := newRequestBudget(context.Background(), &config.Config{}, &budgetRouteClient{routes: []llm.ModelRoute{{
		Model: "large", ContextWindowOverride: 524288, MaxOutputTokensOverride: 8192, Primary: true,
	}}}, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	current := strings.Repeat("current attachment content ", 45000)
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}}
	for i := 0; i < 20; i++ {
		callID := fmt.Sprintf("old-%d", i)
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: strings.Repeat("old request ", 1800)},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: callID, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool", Arguments: `{}`}}}},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: callID, Content: strings.Repeat("old result ", 1800)},
		)
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: current})

	trimmed, dropped, stats := budget.trimHistoryWorkingSet(messages, current, "system", nil, newTokenCountCache(2048))
	if len(dropped) == 0 {
		t.Fatal("expected older turns to be removed")
	}
	if got := messageText(trimmed[len(trimmed)-1]); got != strings.TrimSpace(current) {
		t.Fatal("current request was changed or removed")
	}
	if stats.KeptTokens > stats.LimitTokens {
		t.Fatalf("kept history = %d, limit = %d", stats.KeptTokens, stats.LimitTokens)
	}
	if _, droppedToolMessages := SanitizeToolMessages(trimmed); droppedToolMessages != 0 {
		t.Fatalf("working-set trim split a tool group; sanitizer would drop %d messages", droppedToolMessages)
	}
}

func TestHistoryWorkingSetNeverDropsTrustedSystemAddenda(t *testing.T) {
	budget, err := newRequestBudget(context.Background(), &config.Config{}, &budgetRouteClient{routes: []llm.ModelRoute{{
		Model: "route", ContextWindowOverride: 100000, MaxOutputTokensOverride: 4096,
	}}}, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	trusted := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: strings.Repeat("trusted execution contract ", 4000)}
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "generated system"},
		trusted,
		{Role: openai.ChatMessageRoleSystem, Content: "[CONTEXT_RECAP]: " + strings.Repeat("old recap ", 60000)},
		{Role: openai.ChatMessageRoleUser, Content: "current request"},
	}
	trimmed, dropped, _ := budget.trimHistoryWorkingSet(messages, "current request", "generated system", nil, newTokenCountCache(512))
	if len(dropped) != 1 || dropped[0].Content == trusted.Content {
		t.Fatalf("unexpected dropped messages: %#v", dropped)
	}
	found := false
	for _, message := range trimmed {
		found = found || message.Content == trusted.Content
	}
	if !found {
		t.Fatal("trusted system addendum was dropped")
	}
}

func TestCurrentUserMessageIndexDoesNotMistakeInternalFollowupForHumanIntent(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "human request"},
		{Role: openai.ChatMessageRoleAssistant, Content: "working"},
		{Role: openai.ChatMessageRoleUser, Content: "CIRCUIT BREAKER: summarize now"},
	}
	if got := currentUserMessageIndex(messages, "human request"); got != 0 {
		t.Fatalf("current user index=%d, want 0", got)
	}
}

func budgetTestTool(name, description string) openai.Tool {
	return openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: name, Description: description,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"payload": map[string]interface{}{"type": "string", "description": description},
			},
		},
	}}
}

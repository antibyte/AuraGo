package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"unicode/utf8"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/llm"
	"aurago/internal/security"
)

// c14ChatClient records requests and answers each through answer (a plain JSON-free
// "ok" with finish_reason stop when answer is nil).
type c14ChatClient struct {
	requests []openai.ChatCompletionRequest
	answer   func(n int, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

func (c *c14ChatClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	if c.answer == nil {
		return c14Answer("ok", openai.FinishReasonStop), nil
	}
	return c.answer(len(c.requests), req)
}

func (c *c14ChatClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("not used")
}

func c14Answer(content string, finish openai.FinishReason) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{Model: "served",
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: content}, FinishReason: finish}},
		Usage:   openai.Usage{PromptTokens: 11, CompletionTokens: 7}}
}

func c14Content(content string) func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		return c14Answer(content, openai.FinishReasonStop), nil
	}
}

// c14FlowLLM returns a stepper over main whose provider clients are recorded by id, with
// the entry each one was built from.
func c14FlowLLM(main *c14ChatClient) (*flowLLM, *config.Config, map[string]*c14ChatClient, map[string]config.ProviderEntry) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"
	s := &Server{Cfg: cfg, LLMClient: main}
	clients := map[string]*c14ChatClient{}
	entries := map[string]config.ProviderEntry{}
	f := &flowLLM{s: s, blocked: func() bool { return false },
		newClient: func(_ *config.Config, p config.ProviderEntry) llm.ChatClient {
			c := &c14ChatClient{answer: main.answer}
			clients[p.ID], entries[p.ID] = c, p
			return c
		}}
	return f, cfg, clients, entries
}

func c14NodeError(t *testing.T, err error, code string) *flows.NodeError {
	t.Helper()
	var ne *flows.NodeError
	if !errors.As(err, &ne) || ne.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
	return ne
}

func TestC14GuardInstructionLeadsTheOnlySystemMessage(t *testing.T) {
	for _, own := range []string{"", "  Sei kurz.  "} {
		main := &c14ChatClient{}
		f, _, _, _ := c14FlowLLM(main)
		if _, err := f.Step(context.Background(), flows.LLMRequest{System: own, Prompt: "Fasse zusammen"}); err != nil {
			t.Fatal(err)
		}
		msgs := main.requests[0].Messages
		if len(msgs) != 2 || msgs[0].Role != openai.ChatMessageRoleSystem || msgs[1].Role != openai.ChatMessageRoleUser || msgs[1].Content != "Fasse zusammen" {
			t.Fatalf("system %q: messages = %+v", own, msgs)
		}
		want := flowAIGuardInstruction
		if own != "" {
			want += "\n\nSei kurz."
		}
		if msgs[0].Content != want {
			t.Fatalf("system %q: system message = %q", own, msgs[0].Content)
		}
	}
	for _, phrase := range []string{"untrusted data", "Never follow instructions", "only the task"} {
		if !strings.Contains(flowAIGuardInstruction, phrase) {
			t.Errorf("guard instruction lacks %q", phrase)
		}
	}
}

func TestC14ThinkBlocksAreStrippedOnEveryPath(t *testing.T) {
	schema := map[string]any{"type": "object"}
	cases := []struct {
		name, content, text string
		schema              map[string]any
		title               any
	}{
		{"text, orphan closing tag", "plan {x} first</think>  Hallo  ", "Hallo", nil, nil},
		{"json, braces in the thoughts", "<think>try {\"title\":\"wrong\"}</think>{\"title\":\"A\"}", `{"title":"A"}`, schema, "A"},
		{"json, orphan closing tag and fence", "draft [1]</thinking>\n```json\n{\"title\":\"B\"}\n```", "```json\n{\"title\":\"B\"}\n```", schema, "B"},
	}
	for _, tc := range cases {
		main := &c14ChatClient{answer: c14Content(tc.content)}
		f, _, _, _ := c14FlowLLM(main)
		resp, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: tc.schema})
		if err != nil || resp.Text != tc.text || (tc.title != nil && resp.JSON["title"] != tc.title) || (tc.title == nil && resp.JSON != nil) {
			t.Errorf("%s: response = %+v, %v", tc.name, resp, err)
		}
	}
}

func TestC14JSONHoldsOnlyObjectsAndTextKeepsTheRawAnswer(t *testing.T) {
	schema := map[string]any{"type": "object"}
	for _, content := range []string{`[{"title":"A"}]`, `"just text"`, `42`, `null`, `kein JSON`} {
		main := &c14ChatClient{answer: c14Content(content)}
		f, _, _, _ := c14FlowLLM(main)
		resp, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: schema})
		if err != nil || resp.JSON != nil || resp.Text != content {
			t.Errorf("%s: response = %+v, %v", content, resp, err)
		}
	}
	fenced := "Gern:\n```json\n{\"title\":\"Bericht\"}\n```"
	main := &c14ChatClient{answer: c14Content(fenced)}
	f, _, _, _ := c14FlowLLM(main)
	resp, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: schema})
	if err != nil || resp.JSON["title"] != "Bericht" || resp.Text != fenced {
		t.Fatalf("Text must keep the raw answer the JSON came from: %+v, %v", resp, err)
	}
}

func TestC14MaxTokensAreBounded(t *testing.T) {
	// A custom provider whose output override is far above the cap.
	big := config.ProviderEntry{ID: "big", Type: "custom", Model: "c14-big-model", MaxOutputTokens: 1 << 20}
	for requested, want := range map[int]int{0: flowAIMaxTokens, -5: flowAIMaxTokens, 7: 7, flowAIMaxTokensCap: flowAIMaxTokensCap, 1 << 30: flowAIMaxTokensCap} {
		f, cfg, clients, _ := c14FlowLLM(&c14ChatClient{})
		cfg.Providers = []config.ProviderEntry{big}
		if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "big", MaxTokens: requested}); err != nil {
			t.Fatal(err)
		}
		if got := clients["big"].requests[0].MaxTokens; got != want {
			t.Errorf("MaxTokens %d: sent %d, want %d", requested, got, want)
		}
	}

	// Every budget is clamped to the route's max output: a provider override, and the
	// conservative 4096 of a main model with unknown limits.
	f, cfg, clients, _ := c14FlowLLM(&c14ChatClient{})
	cfg.Providers = []config.ProviderEntry{{ID: "small", Type: "custom", Model: "c14-small-model", MaxOutputTokens: 1000}}
	for _, requested := range []int{0, 10000} {
		if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "small", MaxTokens: requested}); err != nil {
			t.Fatal(err)
		}
		// Each step builds a new provider client.
		if got := clients["small"].requests[0].MaxTokens; got != 1000 {
			t.Errorf("small provider, MaxTokens %d: sent %d, want 1000", requested, got)
		}
	}
	main := &c14ChatClient{}
	f, _, _, _ = c14FlowLLM(main)
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", MaxTokens: 10000}); err != nil || main.requests[0].MaxTokens != llm.ConservativeOutputTokens {
		t.Fatalf("unknown main model: sent %d, %v", main.requests[0].MaxTokens, err)
	}

	if flowAIMaxTokensFor(0, llm.ModelLimits{Reasoning: true}) != llm.ReasoningOutputTokens ||
		flowAIMaxTokensFor(100, llm.ModelLimits{Reasoning: true}) != 100 ||
		flowAIMaxTokensFor(0, llm.ModelLimits{Reasoning: true, MaxOutputTokens: 4096}) != 4096 {
		t.Fatal("a reasoning route defaults to llm.ReasoningOutputTokens within its max output and keeps an explicit request")
	}
}

func TestC14OversizedAnswerIsRefused(t *testing.T) {
	atLimit := strings.Repeat("a", flowAIMaxAnswerBytes)
	main := &c14ChatClient{answer: c14Content(atLimit)}
	f, _, _, _ := c14FlowLLM(main)
	if resp, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"}); err != nil || len(resp.Text) != flowAIMaxAnswerBytes {
		t.Fatalf("an answer at the limit must pass: %d bytes, %v", len(resp.Text), err)
	}
	for _, schema := range []map[string]any{nil, {"type": "object"}} {
		main := &c14ChatClient{answer: c14Content(atLimit + "a")}
		f, _, _, _ := c14FlowLLM(main)
		_, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: schema})
		c14NodeError(t, err, "FLOW_OUTPUT_TOO_LARGE")
	}
}

func TestC14CutOffAnswerIsAnErrorInBothModes(t *testing.T) {
	for _, schema := range []map[string]any{nil, {"type": "object"}} {
		main := &c14ChatClient{answer: func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			return c14Answer(`{"title":"Ber`, openai.FinishReasonLength), nil
		}}
		f, _, _, _ := c14FlowLLM(main)
		_, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: schema})
		ne := c14NodeError(t, err, "FLOW_AI_OUTPUT_INVALID")
		if !strings.Contains(ne.Message, "cut off") || !strings.Contains(ne.Message, fmt.Sprint(flowAIMaxTokens)) {
			t.Fatalf("message = %q", ne.Message)
		}
	}
}

func TestC14StructuredOutputsFollowTheRoute(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}},
		"required": []any{"title"}, "additionalProperties": false}
	wantSchema, _ := json.Marshal(schema)
	isSchema := func(rf *openai.ChatCompletionResponseFormat) bool {
		if rf == nil || rf.Type != openai.ChatCompletionResponseFormatTypeJSONSchema || rf.JSONSchema == nil || !rf.JSONSchema.Strict {
			return false
		}
		got, err := json.Marshal(rf.JSONSchema.Schema)
		return err == nil && string(got) == string(wantSchema)
	}
	manual := func(structured bool) config.ProviderCapabilities {
		off := false
		return config.ProviderCapabilities{Auto: &off, StructuredOutputs: structured}
	}

	main := &c14ChatClient{answer: c14Content(`{"title":"A"}`)}
	f, cfg, clients, _ := c14FlowLLM(main)
	cfg.LLM.StructuredOutputs = true
	if resp, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: schema}); err != nil || resp.JSON["title"] != "A" {
		t.Fatalf("main route: %+v, %v", resp, err)
	}
	if !isSchema(main.requests[0].ResponseFormat) {
		t.Fatalf("main route with structured outputs must send the schema: %+v", main.requests[0].ResponseFormat)
	}
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"}); err != nil || main.requests[1].ResponseFormat != nil {
		t.Fatalf("a text step sends no format: %+v, %v", main.requests[1].ResponseFormat, err)
	}
	cfg.LLM.StructuredOutputs = false
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: schema}); err != nil || main.requests[2].ResponseFormat != nil {
		t.Fatalf("main route without structured outputs sends no format: %+v, %v", main.requests[2].ResponseFormat, err)
	}

	// A provider route uses that provider's capabilities, not the main configuration's.
	cfg.LLM.StructuredOutputs = true
	cfg.Providers = []config.ProviderEntry{
		{ID: "strict", Type: "custom", Model: "s-model", Capabilities: manual(true)},
		{ID: "plain", Type: "custom", Model: "p-model", Capabilities: manual(false)},
	}
	for id, want := range map[string]bool{"strict": true, "plain": false} {
		if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: id, JSONSchema: schema}); err != nil {
			t.Fatal(err)
		}
		rf := clients[id].requests[0].ResponseFormat
		if want != isSchema(rf) || (!want && rf != nil) {
			t.Errorf("provider %s: format = %+v", id, rf)
		}
	}
}

func TestC14RejectedJSONSchemaFallsBackToJSONObject(t *testing.T) {
	schema := map[string]any{"type": "object"}
	param := func(s string) *string { return &s }
	cases := []struct {
		name  string
		err   *openai.APIError
		retry bool
	}{
		{"format named in the message", &openai.APIError{HTTPStatusCode: 400, Message: "response_format json_schema is unavailable"}, true},
		{"format named as the parameter", &openai.APIError{HTTPStatusCode: 400, Message: "unsupported value", Param: param("response_format")}, true},
		{"schema refused, 422", &openai.APIError{HTTPStatusCode: 422, Message: "Invalid schema for function 'flow_step_answer'"}, true},
		{"context length", &openai.APIError{HTTPStatusCode: 400, Message: "This model's maximum context length is 8192 tokens; your schema and messages are too long",
			Param: param("messages"), Code: "context_length_exceeded"}, false},
		{"another bad request", &openai.APIError{HTTPStatusCode: 400, Message: "the model does not exist"}, false},
		{"not a bad request", &openai.APIError{HTTPStatusCode: 401, Message: "response_format needs a paid plan"}, false},
	}
	for _, tc := range cases {
		main := &c14ChatClient{answer: func(_ int, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			if req.ResponseFormat != nil && req.ResponseFormat.Type == openai.ChatCompletionResponseFormatTypeJSONSchema {
				return openai.ChatCompletionResponse{}, tc.err
			}
			return c14Answer(`{"title":"A"}`, openai.FinishReasonStop), nil
		}}
		f, cfg, _, _ := c14FlowLLM(main)
		cfg.LLM.StructuredOutputs = true
		resp, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: schema})
		if !tc.retry {
			if err == nil || len(main.requests) != 1 {
				t.Errorf("%s: must not be retried: %v, %d requests", tc.name, err, len(main.requests))
			}
			continue
		}
		if err != nil || resp.JSON["title"] != "A" || len(main.requests) != 2 ||
			main.requests[1].ResponseFormat == nil || main.requests[1].ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
			t.Errorf("%s: must be retried as json_object once: %+v, %v, %d requests", tc.name, resp, err, len(main.requests))
		}
	}
}

func TestC14ReasoningModelsGetMaxCompletionTokens(t *testing.T) {
	main := &c14ChatClient{}
	f, cfg, clients, _ := c14FlowLLM(main)
	key := "sk-c14-reasoning-key-0123456789"
	cfg.Providers = []config.ProviderEntry{
		{ID: "reason", Type: "openai", Model: "gpt-5-mini", APIKey: key},
		// Every provider saved in the UI has stored capabilities, which carry no reasoning
		// flag: the budget must come from the model limits.
		{ID: "stored", Type: "openai", Model: "gpt-5-mini", APIKey: key, Capabilities: config.ProviderCapabilities{Source: "auto", DetectedModel: "gpt-5-mini"}},
		{ID: "classic", Type: "openai", Model: "gpt-4o-mini", APIKey: key},
	}
	for _, id := range []string{"reason", "stored"} {
		if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: id}); err != nil {
			t.Fatal(err)
		}
		req := clients[id].requests[0]
		if req.MaxTokens != 0 || req.MaxCompletionTokens != llm.ReasoningOutputTokens || req.Temperature != 0 {
			t.Fatalf("%s: reasoning request = max_tokens %d, max_completion_tokens %d, temperature %v", id, req.MaxTokens, req.MaxCompletionTokens, req.Temperature)
		}
		if err := openai.NewReasoningValidator().Validate(req); err != nil {
			t.Fatalf("%s: go-openai would refuse the request: %v", id, err)
		}
	}
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "classic"}); err != nil {
		t.Fatal(err)
	}
	if req := clients["classic"].requests[0]; req.MaxTokens != flowAIMaxTokens || req.MaxCompletionTokens != 0 || req.Temperature != 0.2 {
		t.Fatalf("classic request = %+v", req)
	}
}

func TestC14ProvidersMustBeUsableChatProviders(t *testing.T) {
	f, cfg, clients, _ := c14FlowLLM(&c14ChatClient{})
	key := "sk-c14-eligibility-key-0123456789"
	cfg.Providers = []config.ProviderEntry{
		{ID: "images", Type: "stability", Model: "sd3-large", APIKey: key},
		{ID: "nomodel", Type: "openai", APIKey: key},
		{ID: "nokey", Type: "openai", Model: "gpt-4o-mini"},
		{ID: config.LocalLLMProviderID, Type: "openai", Model: "gpt-4o-mini", APIKey: key},
		{ID: "untyped-nomodel"},
		{ID: "keyless", Type: "custom", Model: "local-model"},
		{ID: "LM", Type: "LM-Studio", Model: "local-model"},
	}
	for id, reason := range map[string]string{"images": "media_provider", "nomodel": "missing_model", "nokey": "missing_credentials",
		config.LocalLLMProviderID: "not configured", "untyped-nomodel": "missing_model"} {
		_, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: id})
		if ne := c14NodeError(t, err, "FLOW_AI_UNAVAILABLE"); !strings.Contains(ne.Message, reason) {
			t.Errorf("%s: message = %q, want %q", id, ne.Message, reason)
		}
		if clients[id] != nil {
			t.Errorf("%s: a client was built", id)
		}
	}
	for _, id := range []string{"keyless", "LM"} {
		if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: id}); err != nil || clients[id] == nil {
			t.Errorf("%s: %v", id, err)
		}
	}
}

// A real provider that echoes the credential it was sent, in a JSON error and in plain text.
func TestC14ProviderCredentialEchoesAreRedacted(t *testing.T) {
	const key = "sk-c14-echoed-provider-key-0123456789abcdef"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if strings.Contains(r.URL.Path, "/text/") {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "upstream refused key %s", sent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":{"message":"Incorrect API key provided: %s","type":"invalid_request_error"}}`, sent)
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.LLM.Model, cfg.LLM.APIKey = "c14-main-model", key
	cfg.Providers = []config.ProviderEntry{
		{ID: "json", Type: "custom", BaseURL: srv.URL + "/json/v1", APIKey: key, Model: "c14-model"},
		{ID: "text", Type: "custom", BaseURL: srv.URL + "/text/v1", APIKey: "  " + key + " ", Model: "c14-model"},
	}
	s := &Server{Cfg: cfg, LLMClient: llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(cfg, "custom", srv.URL+"/json/v1", key, ""))}
	f := newFlowLLM(s)
	for _, model := range []string{"json", "text", ""} {
		_, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: model})
		if err == nil || strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "[redacted]") {
			t.Errorf("route %q: error = %v", model, err)
		}
	}
}

func TestC14RequestErrorsAreScrubbedBoundedAndKeepTheCause(t *testing.T) {
	secret := "c14-provider-body-secret-0123456789"
	security.RegisterSensitive(secret)
	cause := errors.New("upstream rejected the key " + secret + " " + strings.Repeat("x", 5000))
	main := &c14ChatClient{answer: func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		return openai.ChatCompletionResponse{}, cause
	}}
	f, _, _, _ := c14FlowLLM(main)
	_, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"})
	if err == nil || strings.Contains(err.Error(), secret) || !strings.HasPrefix(err.Error(), "the AI request failed: ") {
		t.Fatalf("error = %v", err)
	}
	if n := utf8.RuneCountInString(err.Error()); n > len("the AI request failed: ")+flowErrorRunes+1 {
		t.Fatalf("error has %d runes", n)
	}
	if !errors.Is(err, cause) {
		t.Fatal("the cause must stay reachable")
	}

	// A transport deadline while the run is still alive stays a timeout for the engine.
	main.answer = func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		return openai.ChatCompletionResponse{}, fmt.Errorf("post: %w", context.DeadlineExceeded)
	}
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline = %v", err)
	}
}

func TestC14CancelledContextComesBackAsTheContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	main := &c14ChatClient{answer: func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		cancel()
		return openai.ChatCompletionResponse{}, errors.New(`Post "https://provider.example/v1/chat/completions": context canceled`)
	}}
	f, _, _, _ := c14FlowLLM(main)
	if _, err := f.Step(ctx, flows.LLMRequest{Prompt: "x"}); err != context.Canceled {
		t.Fatalf("error = %v, want context.Canceled itself", err)
	}
}

func TestC14StepsAreChargedToTheFlowsBudget(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"
	cfg.Budget.Enabled, cfg.Budget.DailyLimitUSD, cfg.Budget.Enforcement = true, 1, "partial"
	cfg.Budget.DefaultCost = config.ModelCostRates{InputPerMillion: 1000, OutputPerMillion: 1000}
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	defer tracker.Flush()
	main := &c14ChatClient{}
	s := &Server{Cfg: cfg, LLMClient: main, BudgetTracker: tracker}
	f := newFlowLLM(s)
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
	// 11 input and 7 output tokens at 1000 USD per million each.
	if spent := tracker.CategorySpendUSD(flowAIBudgetCategory); spent < .0179 || spent > .0181 {
		t.Fatalf("flows spend = %f", spent)
	}
	main.answer = func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		return c14Answer("abgeschnit", openai.FinishReasonLength), nil
	}
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"}); err == nil {
		t.Fatal("a cut-off answer must fail")
	}
	if spent := tracker.CategorySpendUSD(flowAIBudgetCategory); spent < .0359 || spent > .0361 {
		t.Fatalf("a cut-off answer is paid for and must be charged: %f", spent)
	}
	// "partial" enforcement blocks every category but chat once the global limit is reached.
	tracker.RecordCostForCategory("chat", 1)
	calls := len(main.requests)
	_, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"})
	c14NodeError(t, err, "FLOW_BUDGET_EXCEEDED")
	if len(main.requests) != calls {
		t.Fatal("a blocked step must not call the model")
	}
}

func TestC14ProviderCredentialsComeFromTheVault(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("provider_vaulted_api_key", "sk-c14-vaulted-key-0123456789"); err != nil {
		t.Fatal(err)
	}
	token, _ := json.Marshal(config.OAuthToken{AccessToken: "ya29-c14-oauth-token-0123456789", Expiry: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	if err := vault.WriteSecret("oauth_gemini", string(token)); err != nil {
		t.Fatal(err)
	}
	expired, _ := json.Marshal(config.OAuthToken{AccessToken: "ya29-c14-expired-0123456789", Expiry: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)})
	if err := vault.WriteSecret("oauth_stale", string(expired)); err != nil {
		t.Fatal(err)
	}

	main := &c14ChatClient{}
	f, cfg, _, entries := c14FlowLLM(main)
	f.s.Vault = vault
	cfg.Providers = []config.ProviderEntry{
		{ID: "vaulted", Type: "openai", Model: "gpt-4o-mini"},
		{ID: "gemini", Type: "google", AuthType: "oauth2", Model: "gemini-2.5-flash"},
		{ID: "stale", Type: "google", AuthType: "oauth2", Model: "gemini-2.5-flash"},
	}
	// Loading the configuration copies provider_<id>_api_key into the entry.
	cfg.ApplyVaultSecrets(vault)
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "vaulted"}); err != nil || entries["vaulted"].APIKey != "sk-c14-vaulted-key-0123456789" {
		t.Fatalf("vaulted key: %q, %v", entries["vaulted"].APIKey, err)
	}
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "gemini"}); err != nil || entries["gemini"].APIKey != "ya29-c14-oauth-token-0123456789" {
		t.Fatalf("oauth token: %q, %v", entries["gemini"].APIKey, err)
	}
	_, err = f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "stale"})
	if ne := c14NodeError(t, err, "FLOW_AI_UNAVAILABLE"); !strings.Contains(ne.Message, "expired_oauth") {
		t.Fatalf("message = %q", ne.Message)
	}

	_, err = f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: strings.Repeat("m", 500)})
	if ne := c14NodeError(t, err, "FLOW_AI_UNAVAILABLE"); utf8.RuneCountInString(ne.Message) > 100 {
		t.Fatalf("an unknown provider id must be echoed bounded: %q", ne.Message)
	}
}

func TestC14FlowsAIProviderIsAProviderReference(t *testing.T) {
	cfg := &config.Config{}
	cfg.Flows.AIProvider = " fast "
	refs := providerReferences(cfg, "fast")
	if len(refs) != 1 || refs[0].Path != "flows.ai_provider" || refs[0].Role != "flows" {
		t.Fatalf("references = %+v", refs)
	}
	if refs := providerReferences(cfg, "other"); len(refs) != 0 {
		t.Fatalf("another provider must not be referenced: %+v", refs)
	}
}

// c14SecretVault returns a vault holding the flow secrets in values (names without the
// easydrag_ prefix) and its file path.
func c14SecretVault(t *testing.T, values map[string]string) (*security.Vault, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault(strings.Repeat("d", 64), path)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range values {
		if err := vault.WriteUserSecret(flowSecretPrefix+name, value, true); err != nil {
			t.Fatal(err)
		}
	}
	return vault, path
}

func TestC14SecretsRegisterTheStoredAndTheTrimmedValue(t *testing.T) {
	raw := "  c14-padded-flow-secret-value \n"
	vault, _ := c14SecretVault(t, map[string]string{"padded": raw, "short": " pin7 "})
	secrets := flowSecrets{s: &Server{Vault: vault}}
	if v, err := secrets.ReadSecret("padded"); err != nil || v != raw {
		t.Fatalf("ReadSecret = %q, %v (the value is returned as stored)", v, err)
	}
	// Nodes send the trimmed value; the scrubber derives its encoded forms (here the one of
	// a Basic authorization header) only from a registered value itself.
	trimmed := strings.TrimSpace(raw)
	for _, leak := range []string{trimmed, base64.StdEncoding.EncodeToString([]byte(trimmed))} {
		if got := security.Scrub("sent " + leak + " today"); strings.Contains(got, leak) {
			t.Fatalf("the trimmed value nodes send must be scrubbed: %q", got)
		}
	}
	// Documented limit: the global scrubber ignores values under 8 bytes.
	if v, err := secrets.ReadSecret("short"); err != nil || v != " pin7 " {
		t.Fatalf("ReadSecret = %q, %v", v, err)
	}
	if got := security.Scrub("code pin7"); !strings.Contains(got, "pin7") {
		t.Fatalf("short secrets are below the scrubber's bound: %q", got)
	}
}

func TestC14WhitespaceOnlySecretsAreNotRegistered(t *testing.T) {
	blanks := map[string]string{"tabs": "\t\t\t\t\t\t\t\t\t", "mixed": " \n \n \n \n \n "}
	vault, _ := c14SecretVault(t, blanks)
	secrets := flowSecrets{s: &Server{Vault: vault}}
	for name, value := range blanks {
		if v, err := secrets.ReadSecret(name); err != nil || v != value {
			t.Fatalf("%s: ReadSecret = %q, %v", name, v, err)
		}
	}
	for _, text := range []string{"func f() {\n\t\t\t\t\t\t\t\t\treturn\n}", "a \n \n \n \n \n b"} {
		if got := security.Scrub(text); got != text {
			t.Errorf("a whitespace-only secret must not redact indentation: %q", got)
		}
	}
}

func TestC14SecretErrorsAreBounded(t *testing.T) {
	vault, path := c14SecretVault(t, map[string]string{"token": "c14-token-value-0123456789"})
	secrets := flowSecrets{s: &Server{Vault: vault}}
	_, err := secrets.ReadSecret(strings.Repeat("Ä", 5000))
	if err == nil || utf8.RuneCountInString(err.Error()) > 100 {
		t.Fatalf("an invalid name must be echoed bounded: %v", err)
	}
	if _, err := secrets.ReadSecret("missing"); err == nil || !strings.Contains(err.Error(), `"missing" does not exist`) {
		t.Fatalf("missing secret = %v", err)
	}
	if err := os.WriteFile(path, []byte("not a vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = secrets.ReadSecret("token")
	if err == nil || !strings.HasPrefix(err.Error(), `the flow secret "token" cannot be read: `) ||
		utf8.RuneCountInString(err.Error()) > len(`the flow secret "token" cannot be read: `)+flowErrorRunes+1 {
		t.Fatalf("vault error = %v", err)
	}
	if _, err := (flowSecrets{s: &Server{}}).ReadSecret("token"); err == nil {
		t.Fatal("without a vault no secret can be read")
	}
}

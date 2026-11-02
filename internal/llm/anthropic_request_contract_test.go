package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestAnthropicOutputTokenLimits(t *testing.T) {
	for _, test := range []struct {
		maxTokens, completionTokens, want int
		invalid                           bool
	}{
		{0, 0, 8192, false}, {1, 0, 1, false}, {0, 17, 17, false},
		{17, 17, 17, false}, {1, 17, 0, true}, {-1, 0, 0, true}, {0, -1, 0, true},
	} {
		ant, err := translateOpenAIToAnthropic(openaiRequest{Model: "claude-sonnet-4-6", MaxTokens: test.maxTokens, MaxCompletionTokens: test.completionTokens}, anthropicThinkingConfig{})
		if (err != nil) != test.invalid || !test.invalid && ant.MaxTokens != test.want {
			t.Fatalf("caps %d/%d became %d: err=%v", test.maxTokens, test.completionTokens, ant.MaxTokens, err)
		}
	}
	var wire openaiRequest
	if err := json.Unmarshal([]byte(`{"model":"claude-sonnet-4-6","max_completion_tokens":17}`), &wire); err != nil {
		t.Fatal(err)
	}
	ant, err := translateOpenAIToAnthropic(wire, anthropicThinkingConfig{})
	if err != nil || ant.MaxTokens != 17 {
		t.Fatalf("max_completion_tokens lost on JSON boundary: cap=%d err=%v", ant.MaxTokens, err)
	}
}

func TestAnthropicParameterlessToolSchemas(t *testing.T) {
	for _, parameters := range []string{"", `,"parameters":null`, `,"parameters":{"type":"object","properties":{"value":{"type":"string"}},"required":["value"]}`} {
		tools, err := translateTools([]json.RawMessage{json.RawMessage(`{"type":"function","function":{"name":"fixture"` + parameters + `}}`)})
		if err != nil {
			t.Fatal(err)
		}
		var shape map[string]any
		if err := json.Unmarshal(tools[0].InputSchema, &shape); err != nil || shape["type"] != "object" || tools[0].CacheControl == nil {
			t.Fatalf("invalid schema/cache for parameterless function: %+v err=%v", tools, err)
		}
		if strings.Contains(parameters, "required") && !reflect.DeepEqual(shape["required"], []any{"value"}) {
			t.Fatal("supplied schema constraints were modified")
		}
	}
}

func TestAnthropicUnsupportedRequestsFailBeforeNetworkWithoutRetry(t *testing.T) {
	for _, fragment := range []string{
		`"max_tokens":-1`, `"max_tokens":1,"max_completion_tokens":17`,
		`"tool_choice":"unsupported"`, `"tool_choice":false`, `"tool_choice":{"type":"function","function":{"name":""}}`, `"tool_choice":{"type":"unsupported"}`,
		`"response_format":{"type":"json_object"}`, `"response_format":{"type":"json_schema","json_schema":{"schema":{"type":"object"}}}`,
		`"response_format":{}`, `"response_format":true`,
		`"tools":[{"type":"custom","function":{"name":"fixture"}}]`,
		`"tools":[{"type":"function","function":{}}]`,
		`"tools":[{"type":"function","function":{"name":"fixture","parameters":{"type":"array"}}}]`,
		`"tools":[{"type":"function","function":{"name":"fixture","parameters":true}}]`,
		`"messages":[{"role":"user","content":[{"type":"text","text":"keep"},{"type":"file","file":{"file_id":"private-fixture"}}]}]`,
		`"messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{}}]}]`,
		`"messages":[{"role":"user","content":[{"type":"text","text":4}]}]`,
		`"messages":[{"role":"user","content":[false]}]`,
		`"messages":[{"role":"user","content":{"private-fixture":true}}]`,
		`"messages":[{"role":"function","content":"private-fixture"}]`,
		`"messages":[{"role":"developer","content":[{"type":"file","file":{}}]}]`,
		`"messages":[{"role":"assistant","content":[{"type":"text","text":4}]}]`,
	} {
		t.Run(fragment, func(t *testing.T) {
			calls := 0
			transport := &anthropicTransport{base: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, nil })}
			req, err := http.NewRequest(http.MethodPost, "https://fixture.invalid/v1/chat/completions", strings.NewReader(`{"model":"claude-sonnet-4-6",`+fragment+`}`))
			if err != nil {
				t.Fatal(err)
			}
			_, err = transport.RoundTrip(req)
			if err == nil || calls != 0 || ClassifyError(err) != ErrCategoryNonRetryableConfig || strings.Contains(err.Error(), "private-fixture") {
				t.Fatalf("unsupported request escaped local rejection: calls=%d category=%s err=%v", calls, ClassifyError(err), err)
			}
		})
	}
}

func TestAnthropicDeveloperInstructionsRetainOrderAndCacheBoundary(t *testing.T) {
	ant, err := translateOpenAIToAnthropic(openaiRequest{Messages: []json.RawMessage{
		json.RawMessage(`{"role":"system","content":"first"}`),
		json.RawMessage(`{"role":"developer","content":[{"type":"text","text":"second"}]}`),
		json.RawMessage(`{"role":"user","content":"task"}`),
		json.RawMessage(`{"role":"system","content":"# TURN CONTEXT\nthird"}`),
		json.RawMessage(`{"role":"developer","content":"fourth"}`),
	}}, anthropicThinkingConfig{})
	if err != nil {
		t.Fatal(err)
	}
	blocks := ant.System.([]anthropicContentBlock)
	if len(blocks) != 2 || blocks[0].Text+blocks[1].Text != "first\n\nsecond\n\n# TURN CONTEXT\nthird\n\nfourth" || blocks[0].CacheControl == nil || blocks[1].CacheControl != nil || len(ant.Messages) != 1 || ant.Messages[0].Content != "task" {
		t.Fatalf("instruction order or cache boundary changed: %+v messages=%+v", blocks, ant.Messages)
	}
}

func TestAnthropicResponseFormatTextAndNullRemainCompatible(t *testing.T) {
	for _, raw := range []string{"", "null", `{"type":"text"}`} {
		_, err := translateOpenAIToAnthropic(openaiRequest{ResponseFormat: json.RawMessage(raw)}, anthropicThinkingConfig{})
		if err != nil {
			t.Fatalf("text format rejected: %v", err)
		}
	}
}

func TestAnthropicBridgeCapabilitiesBoundModelMetadata(t *testing.T) {
	model := "claude-sonnet-4-6"
	for _, caps := range []config.ProviderCapabilities{
		{},
		{Auto: boolPtr(false), ToolCalling: true, StructuredOutputs: true, Multimodal: true},
		{Source: "fixture-detection", DetectedModel: model, ToolCalling: true, StructuredOutputs: true, Multimodal: true},
	} {
		provider := config.ProviderEntry{Type: "anthropic", Model: model, Capabilities: caps}
		result := ResolveProviderCapabilities(provider, CapabilityFallback{StructuredOutputs: true})
		if result.StructuredOutputs || !result.ToolCalling || !result.Multimodal || !result.Image || !reflect.DeepEqual(caps, provider.Capabilities) {
			t.Fatalf("bridge capability bounds changed unrelated metadata: %+v", result)
		}
	}
	raw, ok := CapabilitiesFromRegistry("anthropic", model)
	if !ok || !raw.StructuredOutputs {
		t.Fatal("raw model support was overwritten with bridge limitations")
	}
	for _, provider := range []struct {
		kind, model string
		structured  bool
	}{
		{"opencode-go", "MiniMax-M3", false}, {"opencode-go", "glm-5", true},
		{"minimax", "MiniMax-M3", true}, {"openai", model, true},
	} {
		result := ResolveProviderCapabilities(config.ProviderEntry{Type: provider.kind, Model: provider.model, Capabilities: config.ProviderCapabilities{Auto: boolPtr(false), StructuredOutputs: true}}, CapabilityFallback{})
		if result.StructuredOutputs != provider.structured {
			t.Fatalf("%s/%s structured=%t, want %t", provider.kind, provider.model, result.StructuredOutputs, provider.structured)
		}
	}
}

func TestAnthropicSDKPositiveCompletionCapReachesNativeWire(t *testing.T) {
	seen := 0
	transport := &anthropicTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var native anthropicRequest
		if err := json.NewDecoder(req.Body).Decode(&native); err != nil {
			t.Fatal(err)
		}
		seen = native.MaxTokens
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"msg_fixture","content":[],"stop_reason":"end_turn"}`))}, nil
	})}
	cfg := openai.DefaultConfig("fixture")
	cfg.HTTPClient = &http.Client{Transport: transport}
	_, err := openai.NewClientWithConfig(cfg).CreateChatCompletion(t.Context(), openai.ChatCompletionRequest{Model: "claude-sonnet-4-6", MaxCompletionTokens: 17, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "task"}}})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 17 {
		t.Fatalf("cap expanded to %d", seen)
	}
}

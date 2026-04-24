package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestAIGatewayAuthTransportAddsHeader(t *testing.T) {
	transport := &aiGatewayAuthTransport{
		token: "test-token",
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("cf-aig-authorization"); got != "Bearer test-token" {
				t.Fatalf("cf-aig-authorization = %q, want %q", got, "Bearer test-token")
			}
			if got := req.Header.Get("Authorization"); got != "Bearer provider-key" {
				t.Fatalf("Authorization = %q, want %q", got, "Bearer provider-key")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.invalid/v1/chat/completions", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer provider-key")

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

func TestOpenAIPromptCacheRequestBodyAddsStableKeyFromPrefix(t *testing.T) {
	bodyA := []byte(`{
		"model": "gpt-4.1",
		"messages": [
			{"role": "system", "content": "Stable system prompt\n# TURN CONTEXT\nNow A"},
			{"role": "user", "content": "Hello A"}
		],
		"tools": [
			{"type":"function","function":{"name":"alpha","parameters":{"type":"object"}}}
		]
	}`)
	bodyB := []byte(`{
		"model": "gpt-4.1",
		"messages": [
			{"role": "system", "content": "Stable system prompt\n# TURN CONTEXT\nNow B"},
			{"role": "user", "content": "Hello B"}
		],
		"tools": [
			{"type":"function","function":{"name":"alpha","parameters":{"type":"object"}}}
		]
	}`)

	keyA := openAIPromptCacheKeyFromBodyForTest(t, openAIPromptCacheRequestBody(bodyA))
	keyB := openAIPromptCacheKeyFromBodyForTest(t, openAIPromptCacheRequestBody(bodyB))
	if keyA == "" {
		t.Fatal("expected prompt_cache_key")
	}
	if keyA != keyB {
		t.Fatalf("prompt_cache_key changed across volatile suffixes: %q vs %q", keyA, keyB)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(openAIPromptCacheRequestBody(bodyA), &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if _, exists := payload["prompt_cache_retention"]; exists {
		t.Fatal("did not expect prompt_cache_retention in minimal cache hint")
	}
}

func TestOpenAIPromptCacheTransportAddsRequestShapeHint(t *testing.T) {
	transport := &openAIPromptCacheTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body failed: %v", err)
			}
			if key := openAIPromptCacheKeyFromBodyForTest(t, raw); key == "" {
				t.Fatal("expected prompt_cache_key in outbound request")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(`{
		"model": "gpt-4.1",
		"messages": [{"role": "system", "content": "Stable\n# TURN CONTEXT\nTurn"}]
	}`))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

func openAIPromptCacheKeyFromBodyForTest(t *testing.T, body []byte) string {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	key, _ := payload["prompt_cache_key"].(string)
	return key
}

func TestMiniMaxConvertSystemMessages_StructuredContent(t *testing.T) {
	body := `{
		"model": "minimax",
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]}
		]
	}`
	result := miniMaxConvertSystemMessages([]byte(body))
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	msgs := payload["messages"].([]interface{})
	userMsg := msgs[0].(map[string]interface{})
	content := userMsg["content"]
	parts, ok := content.([]interface{})
	if !ok {
		t.Fatalf("content type = %T, want []interface{}", content)
	}
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	firstPart := parts[0].(map[string]interface{})
	if firstPart["type"] != "text" {
		t.Fatalf("first part type = %v, want text", firstPart["type"])
	}
	if !strings.Contains(firstPart["text"].(string), "You are a helpful assistant.") {
		t.Fatalf("first part text = %q, want it to contain system prompt", firstPart["text"])
	}
}

func TestMiniMaxConvertSystemMessages_NoUserMessage(t *testing.T) {
	body := `{
		"model": "minimax",
		"messages": [
			{"role": "system", "content": "System prompt"},
			{"role": "assistant", "content": "I can help."}
		]
	}`
	result := miniMaxConvertSystemMessages([]byte(body))
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	msgs := payload["messages"].([]interface{})
	asstMsg := msgs[0].(map[string]interface{})
	if asstMsg["content"] != "I can help." {
		t.Fatalf("assistant content = %q, want unchanged", asstMsg["content"])
	}
}

func TestMiniMaxConvertSystemMessages_MultipleSystems(t *testing.T) {
	body := `{
		"model": "minimax",
		"messages": [
			{"role": "system", "content": "First system"},
			{"role": "user", "content": "Hello"},
			{"role": "system", "content": "Second system"}
		]
	}`
	result := miniMaxConvertSystemMessages([]byte(body))
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	msgs := payload["messages"].([]interface{})
	userMsg := msgs[0].(map[string]interface{})
	if !strings.Contains(userMsg["content"].(string), "First system") ||
		!strings.Contains(userMsg["content"].(string), "Second system") {
		t.Fatalf("user content = %q, want both system prompts prepended", userMsg["content"])
	}
}

func TestMiniMaxConvertSystemMessages_StructuredContentNoTextPart(t *testing.T) {
	body := `{
		"model": "minimax",
		"messages": [
			{"role": "system", "content": "System prompt"},
			{"role": "user", "content": [{"type": "image_url", "image_url": {"url": "http://example.com/img.png"}}]}
		]
	}`
	result := miniMaxConvertSystemMessages([]byte(body))
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	msgs := payload["messages"].([]interface{})
	userMsg := msgs[0].(map[string]interface{})
	content := userMsg["content"]
	parts, ok := content.([]interface{})
	if !ok {
		t.Fatalf("content type = %T, want []interface{}", content)
	}
	if len(parts) != 2 {
		t.Fatalf("len(parts) = %d, want 2 (system text part prepended)", len(parts))
	}
	firstPart := parts[0].(map[string]interface{})
	if firstPart["type"] != "text" || !strings.Contains(firstPart["text"].(string), "System prompt") {
		t.Fatalf("first part = %v, want text part with system prompt", firstPart)
	}
}

func TestMiniMaxPrepareRequestBodyAddsReasoningSplit(t *testing.T) {
	body := `{
		"model": "MiniMax-M2.7",
		"stream": true,
		"messages": [
			{"role": "system", "content": "System prompt"},
			{"role": "user", "content": "Hello"}
		]
	}`
	result, isStream := miniMaxPrepareRequestBody([]byte(body))
	if !isStream {
		t.Fatal("isStream=false, want true")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	if got, ok := payload["reasoning_split"].(bool); !ok || !got {
		t.Fatalf("reasoning_split = %#v, want true", payload["reasoning_split"])
	}
	msgs := payload["messages"].([]interface{})
	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	userMsg := msgs[0].(map[string]interface{})
	if got := userMsg["role"]; got != "user" {
		t.Fatalf("message role = %q, want user", got)
	}
	if !strings.Contains(userMsg["content"].(string), "System prompt") {
		t.Fatalf("user content = %q, want system prompt prepended", userMsg["content"])
	}
}

func TestMiniMaxPrepareRequestBodyForcesReasoningSplit(t *testing.T) {
	body := `{
		"model": "MiniMax-M2.7",
		"reasoning_split": false,
		"messages": [{"role": "user", "content": "Hello"}]
	}`
	result, _ := miniMaxPrepareRequestBody([]byte(body))
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	if got, ok := payload["reasoning_split"].(bool); !ok || !got {
		t.Fatalf("reasoning_split = %#v, want true", payload["reasoning_split"])
	}
}

func TestMiniMaxPrepareRequestBodyMapsReasoningContentToDetails(t *testing.T) {
	body := `{
		"model": "MiniMax-M2.7",
		"messages": [{
			"role": "assistant",
			"content": "",
			"reasoning_content": "preserve this chain",
			"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "tool", "arguments": "{}"}}]
		}]
	}`
	result, _ := miniMaxPrepareRequestBody([]byte(body))
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	msgs := payload["messages"].([]interface{})
	msg := msgs[0].(map[string]interface{})
	if _, exists := msg["reasoning_content"]; exists {
		t.Fatalf("reasoning_content still present in MiniMax request: %#v", msg["reasoning_content"])
	}
	details, ok := msg["reasoning_details"].([]interface{})
	if !ok || len(details) != 1 {
		t.Fatalf("reasoning_details = %#v, want one reasoning detail", msg["reasoning_details"])
	}
	detail := details[0].(map[string]interface{})
	if got := detail["text"]; got != "preserve this chain" {
		t.Fatalf("reasoning_details[0].text = %q, want preserve this chain", got)
	}
	if got := detail["format"]; got != "MiniMax-response-v1" {
		t.Fatalf("reasoning_details[0].format = %q, want MiniMax-response-v1", got)
	}
}

func TestMiniMaxNormalizeResponseBodyMapsReasoningDetails(t *testing.T) {
	body := `{
		"id": "chatcmpl-test",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "",
				"reasoning_details": [
					{"type": "reasoning.text", "text": "first thought"},
					{"type": "reasoning.text", "text": "second thought"}
				],
				"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{}"}}]
			}
		}]
	}`
	result := miniMaxNormalizeResponseBody([]byte(body))
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal result failed: %v", err)
	}
	choices := payload["choices"].([]interface{})
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	if got := message["reasoning_content"]; got != "first thought\nsecond thought" {
		t.Fatalf("reasoning_content = %q, want joined reasoning details", got)
	}
	if _, ok := message["reasoning_details"]; !ok {
		t.Fatal("reasoning_details removed, want original field preserved")
	}
}

func TestMiniMaxNormalizeSSELineMapsDeltaReasoningDetails(t *testing.T) {
	line := `data: {"choices":[{"delta":{"reasoning_details":[{"text":"stream thought"}]}}]}` + "\n"
	result := miniMaxNormalizeSSELine(line)
	var payload map[string]interface{}
	raw := strings.TrimPrefix(strings.TrimSpace(result), "data: ")
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("json.Unmarshal normalized SSE data failed: %v", err)
	}
	choices := payload["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if got := delta["reasoning_content"]; got != "stream thought" {
		t.Fatalf("delta.reasoning_content = %q, want stream thought", got)
	}
}

func TestMiniMaxTransportNormalizesRequestAndResponse(t *testing.T) {
	transport := &miniMaxTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body failed: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("json.Unmarshal request failed: %v", err)
			}
			if got, ok := payload["reasoning_split"].(bool); !ok || !got {
				t.Fatalf("request reasoning_split = %#v, want true", payload["reasoning_split"])
			}
			msgs := payload["messages"].([]interface{})
			if len(msgs) != 1 {
				t.Fatalf("request messages = %d, want system collapsed into user", len(msgs))
			}
			body := `{"choices":[{"message":{"role":"assistant","reasoning_details":[{"text":"kept reasoning"}],"content":"ok"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.minimax.io/v1/chat/completions", strings.NewReader(`{
		"model": "MiniMax-M2.7",
		"messages": [
			{"role": "system", "content": "System"},
			{"role": "user", "content": "Hi"}
		]
	}`))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if !strings.Contains(string(raw), `"reasoning_content":"kept reasoning"`) {
		t.Fatalf("response body = %s, want reasoning_content", raw)
	}
}

func TestDetectProviderURLMismatchMiniMax(t *testing.T) {
	if got := detectProviderURLMismatch("minimax", "https://api.openai.com/v1"); !strings.Contains(got, "provider=openai") {
		t.Fatalf("minimax/openai mismatch = %q, want openai hint", got)
	}
	if got := detectProviderURLMismatch("openai", "https://api.minimax.io/v1"); !strings.Contains(got, "provider=minimax") {
		t.Fatalf("openai/minimax mismatch = %q, want minimax hint", got)
	}
	if got := detectProviderURLMismatch("minimax", "https://api.minimax.io/v1"); got != "" {
		t.Fatalf("valid minimax mismatch = %q, want empty", got)
	}
}

func TestNewClientFromProviderDetailsBuildsWorkersAIURLFromAccountID(t *testing.T) {
	client := NewClientFromProviderDetails("workers-ai", "", "test-key", "cf-account")
	if client == nil {
		t.Fatal("expected workers-ai client")
	}

	configValue := reflect.ValueOf(client).Elem().FieldByName("config")
	if !configValue.IsValid() {
		t.Fatal("expected openai client config field")
	}
	if got := configValue.FieldByName("BaseURL").String(); got != "https://api.cloudflare.com/client/v4/accounts/cf-account/ai/v1" {
		t.Fatalf("BaseURL = %q, want workers-ai account URL", got)
	}
}

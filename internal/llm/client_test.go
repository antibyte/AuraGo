package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
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

func TestShouldUseOpenAIPromptCacheKeyOnlyForOfficialOpenAI(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		baseURL      string
		want         bool
	}{
		{name: "default OpenAI URL", providerType: "openai", baseURL: "https://api.openai.com/v1", want: true},
		{name: "empty OpenAI URL uses SDK default", providerType: "openai", baseURL: "", want: true},
		{name: "custom OpenAI-compatible URL", providerType: "openai", baseURL: "https://proxy.example.test/v1", want: false},
		{name: "Cloudflare gateway URL", providerType: "openai", baseURL: "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai", want: false},
		{name: "OpenRouter provider", providerType: "openrouter", baseURL: "https://api.openai.com/v1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseOpenAIPromptCacheKey(tt.providerType, tt.baseURL); got != tt.want {
				t.Fatalf("shouldUseOpenAIPromptCacheKey(%q, %q) = %v, want %v", tt.providerType, tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestManifestProviderURLMismatchIsQuietForLocalGateway(t *testing.T) {
	if got := detectProviderURLMismatch("manifest", "http://127.0.0.1:2099/v1"); got != "" {
		t.Fatalf("detectProviderURLMismatch(manifest local) = %q, want empty", got)
	}
	if got := detectProviderURLMismatch("manifest", "http://manifest:2099/v1"); got != "" {
		t.Fatalf("detectProviderURLMismatch(manifest docker) = %q, want empty", got)
	}
}

func TestManifestRoutingTransportDisabledDoesNotSetHeaders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Routing.Enabled = false
	transport := &manifestRoutingTransport{
		routing: cfg.Manifest.Routing,
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-aurago-task"); got != "" {
				t.Fatalf("x-aurago-task = %q, want empty", got)
			}
			if got := req.Header.Get("x-manifest-specificity"); got != "" {
				t.Fatalf("x-manifest-specificity = %q, want empty", got)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://manifest.example.test/v1/chat/completions", strings.NewReader(`{"tools":[]}`))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

func TestManifestRoutingTransportFixedSpecificityAndSafeHeaders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Routing.Enabled = true
	cfg.Manifest.Routing.SpecificityMode = "fixed"
	cfg.Manifest.Routing.Specificity = "coding"
	cfg.Manifest.Routing.Headers = map[string]string{
		"x-aurago-task":          "coding",
		"authorization":          "Bearer leaked",
		"bad header":             "nope",
		"x-empty":                "",
		"x-manifest-specificity": "trading",
	}
	transport := &manifestRoutingTransport{
		routing: cfg.Manifest.Routing,
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-aurago-task"); got != "coding" {
				t.Fatalf("x-aurago-task = %q, want coding", got)
			}
			if got := req.Header.Get("x-manifest-specificity"); got != "coding" {
				t.Fatalf("x-manifest-specificity = %q, want fixed coding", got)
			}
			for _, blocked := range []string{"authorization", "bad header", "x-empty"} {
				if got := req.Header.Get(blocked); got != "" {
					t.Fatalf("%s = %q, want empty", blocked, got)
				}
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://manifest.example.test/v1/chat/completions", strings.NewReader(`{"tools":[]}`))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

func TestManifestRoutingTransportAutoSpecificityRequiresSingleCategory(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "single coding prefix",
			body: `{"tools":[{"type":"function","function":{"name":"code_review"}}]}`,
			want: "coding",
		},
		{
			name: "conflicting prefixes",
			body: `{"tools":[{"type":"function","function":{"name":"code_review"}},{"type":"function","function":{"name":"browser_open"}}]}`,
			want: "",
		},
		{
			name: "unknown prefix",
			body: `{"tools":[{"type":"function","function":{"name":"execute_shell"}}]}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Manifest.Routing.Enabled = true
			cfg.Manifest.Routing.SpecificityMode = "auto"
			transport := &manifestRoutingTransport{
				routing: cfg.Manifest.Routing,
				base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if got := req.Header.Get("x-manifest-specificity"); got != tt.want {
						t.Fatalf("x-manifest-specificity = %q, want %q", got, tt.want)
					}
					raw, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read forwarded body: %v", err)
					}
					if string(raw) != tt.body {
						t.Fatalf("forwarded body = %q, want original %q", string(raw), tt.body)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
				}),
			}
			req, err := http.NewRequest(http.MethodPost, "https://manifest.example.test/v1/chat/completions", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("http.NewRequest() error = %v", err)
			}
			if _, err := transport.RoundTrip(req); err != nil {
				t.Fatalf("RoundTrip() error = %v", err)
			}
		})
	}
}

func TestBuildLLMHTTPClientOnlyInstallsManifestRoutingForManifestProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Routing.Enabled = true
	cfg.Manifest.Routing.Headers = map[string]string{"x-aurago-task": "coding"}

	manifestClient := buildLLMHTTPClient(cfg, "manifest", "", "https://manifest.example.test/v1")
	if _, ok := unwrapLLMTransport(manifestClient.Transport).(*manifestRoutingTransport); !ok {
		t.Fatalf("manifest provider base transport = %T, want *manifestRoutingTransport", unwrapLLMTransport(manifestClient.Transport))
	}

	openAIClient := buildLLMHTTPClient(cfg, "openai", "", "https://api.openai.com/v1")
	if _, ok := unwrapLLMTransport(openAIClient.Transport).(*manifestRoutingTransport); ok {
		t.Fatal("openai provider should not install manifest routing transport")
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
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want synthetic user + assistant", len(msgs))
	}
	userMsg := msgs[0].(map[string]interface{})
	if userMsg["role"] != "user" || userMsg["content"] != "System prompt" {
		t.Fatalf("first message = %#v, want synthetic user with system prompt", userMsg)
	}
	asstMsg := msgs[1].(map[string]interface{})
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

func TestDetectProviderURLMismatchNewProviders(t *testing.T) {
	// DeepSeek
	if got := detectProviderURLMismatch("deepseek", "https://api.deepseek.com/v1"); got != "" {
		t.Fatalf("deepseek valid mismatch = %q, want empty", got)
	}
	if got := detectProviderURLMismatch("deepseek", "https://api.openai.com/v1"); got == "" {
		t.Fatal("deepseek/openai mismatch: expected hint, got empty")
	}

	// Groq
	if got := detectProviderURLMismatch("groq", "https://api.groq.com/openai/v1"); got != "" {
		t.Fatalf("groq valid mismatch = %q, want empty", got)
	}
	if got := detectProviderURLMismatch("groq", "https://api.openai.com/v1"); got == "" {
		t.Fatal("groq/openai mismatch: expected hint, got empty")
	}

	// Mistral
	if got := detectProviderURLMismatch("mistral", "https://api.mistral.ai/v1"); got != "" {
		t.Fatalf("mistral valid mismatch = %q, want empty", got)
	}
	if got := detectProviderURLMismatch("mistral", "https://api.openai.com/v1"); got == "" {
		t.Fatal("mistral/openai mismatch: expected hint, got empty")
	}

	// xAI
	if got := detectProviderURLMismatch("xai", "https://api.x.ai/v1"); got != "" {
		t.Fatalf("xai valid mismatch = %q, want empty", got)
	}
	if got := detectProviderURLMismatch("xai", "https://api.openai.com/v1"); got == "" {
		t.Fatal("xai/openai mismatch: expected hint, got empty")
	}

	// Moonshot
	if got := detectProviderURLMismatch("moonshot", "https://api.moonshot.ai/v1"); got != "" {
		t.Fatalf("moonshot valid mismatch = %q, want empty", got)
	}

	// Qwen
	if got := detectProviderURLMismatch("qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1"); got != "" {
		t.Fatalf("qwen valid mismatch = %q, want empty", got)
	}

	// Z.ai
	if got := detectProviderURLMismatch("zai", "https://open.bigmodel.cn/api/paas/v4"); got != "" {
		t.Fatalf("zai valid mismatch = %q, want empty", got)
	}

	// Local providers
	if got := detectProviderURLMismatch("llamacpp", "http://localhost:8080/v1"); got != "" {
		t.Fatalf("llamacpp valid mismatch = %q, want empty", got)
	}
	if got := detectProviderURLMismatch("lmstudio", "http://localhost:1234/v1"); got != "" {
		t.Fatalf("lmstudio valid mismatch = %q, want empty", got)
	}
}

func TestAIGatewaySegmentNewProviders(t *testing.T) {
	newProviders := []string{"deepseek", "groq", "mistral", "xai", "moonshot", "qwen", "zai", "llamacpp", "lmstudio"}
	for _, p := range newProviders {
		if got := aiGatewaySegment(p); got != "openai" {
			t.Fatalf("aiGatewaySegment(%q) = %q, want openai", p, got)
		}
	}
}

func TestNewClientFromProviderWithConfigAppliesAIGateway(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.BaseURL = "https://openrouter.ai/api/v1"
	cfg.AIGateway.Enabled = true
	cfg.AIGateway.AccountID = "acct"
	cfg.AIGateway.GatewayID = "gw"
	cfg.AIGateway.Token = "gw-token"

	client := NewClientFromProviderWithConfig(cfg, "openrouter", "https://openrouter.ai/api/v1", "provider-key", "")
	if client == nil {
		t.Fatal("expected client")
	}
	configValue := reflect.ValueOf(client).Elem().FieldByName("config")
	if !configValue.IsValid() {
		t.Fatal("expected openai client config field")
	}
	wantBase := "https://gateway.ai.cloudflare.com/v1/acct/gw/openai"
	if got := configValue.FieldByName("BaseURL").String(); got != wantBase {
		t.Fatalf("BaseURL = %q, want %q", got, wantBase)
	}

	mainClient := NewClient(cfg)
	if mainClient == nil {
		t.Fatal("expected main client")
	}
	mainConfig := reflect.ValueOf(mainClient).Elem().FieldByName("config")
	if got := mainConfig.FieldByName("BaseURL").String(); got != wantBase {
		t.Fatalf("main client BaseURL = %q, want %q", got, wantBase)
	}
}

func TestNewClientFromProviderDetailsLocalProviders(t *testing.T) {
	for _, provider := range []string{"llamacpp", "lmstudio"} {
		client := NewClientFromProviderDetails(provider, "http://localhost:8080", "", "")
		if client == nil {
			t.Fatalf("expected %s client", provider)
		}
		configValue := reflect.ValueOf(client).Elem().FieldByName("config")
		if !configValue.IsValid() {
			t.Fatal("expected openai client config field")
		}
		// BaseURL should have /v1 appended
		if got := configValue.FieldByName("BaseURL").String(); got != "http://localhost:8080/v1" {
			t.Fatalf("BaseURL = %q, want http://localhost:8080/v1", got)
		}
	}
}

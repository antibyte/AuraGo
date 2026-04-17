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

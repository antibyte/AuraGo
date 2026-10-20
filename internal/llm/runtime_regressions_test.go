package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMiniMaxPrepareRequestBodyAppliesAllMutationsTogether(t *testing.T) {
	body := []byte(`{
		"model":"MiniMax-M2.7",
		"stream":true,
		"reasoning_split":false,
		"messages":[
			{"role":"system","content":"System prompt"},
			{"role":"user","content":"User prompt"},
			{"role":"assistant","content":"","reasoning_content":"reasoning chain"}
		]
	}`)

	result, stream := miniMaxPrepareRequestBody(body)
	if !stream {
		t.Fatal("stream = false, want true")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("unmarshal prepared body: %v", err)
	}
	if got, ok := payload["reasoning_split"].(bool); !ok || !got {
		t.Fatalf("reasoning_split = %#v, want true", payload["reasoning_split"])
	}
	messages := payload["messages"].([]interface{})
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	user := messages[0].(map[string]interface{})
	if user["role"] != "user" || !strings.Contains(user["content"].(string), "System prompt") {
		t.Fatalf("user message = %#v, want converted system prompt", user)
	}
	assistant := messages[1].(map[string]interface{})
	if _, ok := assistant["reasoning_content"]; ok {
		t.Fatalf("reasoning_content remained in prepared body: %#v", assistant)
	}
	details, ok := assistant["reasoning_details"].([]interface{})
	if !ok || len(details) != 1 || details[0].(map[string]interface{})["text"] != "reasoning chain" {
		t.Fatalf("reasoning_details = %#v, want mapped reasoning chain", assistant["reasoning_details"])
	}
}

func TestMiniMaxPrepareRequestBodyLeavesInvalidJSONUnchanged(t *testing.T) {
	body := []byte(`{"stream":true`)
	result, stream := miniMaxPrepareRequestBody(body)
	if stream {
		t.Fatal("stream = true for invalid JSON, want false")
	}
	if !bytes.Equal(result, body) {
		t.Fatalf("invalid body changed: got %q want %q", result, body)
	}
}

func TestOpenAIPromptCacheTransportDoesNotStoreRequestBodies(t *testing.T) {
	typeOfTransport := reflect.TypeOf(openAIPromptCacheTransport{})
	if typeOfTransport.NumField() != 1 || typeOfTransport.Field(0).Name != "base" {
		t.Fatalf("openAIPromptCacheTransport fields = %v, want only base transport", transportFieldNames(typeOfTransport))
	}
}

func TestOpenAIPromptCacheTransportPreservesExistingKey(t *testing.T) {
	const body = `{"model":"gpt-4.1","prompt_cache_key":"caller-key","messages":[{"role":"user","content":"hi"}]}`
	transport := &openAIPromptCacheTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read outbound body: %v", err)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("unmarshal outbound body: %v", err)
		}
		if got := payload["prompt_cache_key"]; got != "caller-key" {
			t.Fatalf("prompt_cache_key = %#v, want caller-key", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
}

func TestFailoverManagerAppliesFinalRetryIntervalAtStartupAndReconfigure(t *testing.T) {
	original := FinalRetryInterval()
	cfg := &config.Config{}
	cfg.CircuitBreaker.FinalRetryInterval = "45s"
	fm := NewFailoverManager(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer fm.Stop()
	if got := fm.RetryPolicySnapshot().FinalInterval; got != 45*time.Second {
		t.Fatalf("startup interval=%s", got)
	}
	for _, invalid := range []string{"not-a-duration", "0s"} {
		cfg.CircuitBreaker.FinalRetryInterval = invalid
		fm.Reconfigure(cfg)
		if got := fm.RetryPolicySnapshot().FinalInterval; got != 30*time.Second {
			t.Fatalf("reconfigured interval=%s", got)
		}
		if cfg.CircuitBreaker.FinalRetryInterval != invalid {
			t.Fatal("manager mutated caller configuration")
		}
	}
	if FinalRetryInterval() != original {
		t.Fatal("manager changed standalone retry defaults")
	}
}

func transportFieldNames(typ reflect.Type) []string {
	names := make([]string, typ.NumField())
	for i := range names {
		names[i] = typ.Field(i).Name
	}
	return names
}

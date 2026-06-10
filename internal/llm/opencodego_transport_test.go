package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOpenCodeGoTransportDelegatesMiniMaxToAnthropic(t *testing.T) {
	var capturedPath string
	var capturedAPIKey string
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		capturedAPIKey = req.Header.Get("x-api-key")
		if req.Header.Get("Authorization") != "" {
			t.Fatal("expected Authorization header to be removed for MiniMax")
		}
		if req.Header.Get("anthropic-version") != anthropicAPIVersion {
			t.Fatalf("anthropic-version = %q, want %q", req.Header.Get("anthropic-version"), anthropicAPIVersion)
		}

		respBody := `{
			"id":"msg_test",
			"type":"message",
			"role":"assistant",
			"content":[{"type":"text","text":"hello"}],
			"model":"minimax-m2",
			"stop_reason":"end_turn",
			"usage":{"input_tokens":3,"output_tokens":2}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Header:     make(http.Header),
		}, nil
	})

	transport := &opencodeGoTransport{base: base}
	body := `{"model":"minimax-m2","messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/go/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer resp.Body.Close()

	if capturedPath != "/v1/messages" && !strings.HasSuffix(capturedPath, "/v1/messages") {
		t.Fatalf("captured path = %q, want /v1/messages", capturedPath)
	}
	if capturedAPIKey != "test-key" {
		t.Fatalf("x-api-key = %q, want test-key", capturedAPIKey)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	var oaiResp map[string]interface{}
	if err := json.Unmarshal(raw, &oaiResp); err != nil {
		t.Fatalf("unmarshal openai response: %v body=%s", err, raw)
	}
	choices, ok := oaiResp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("expected translated choices, got %v", oaiResp["choices"])
	}
}

func TestOpenCodeGoTransportPassesThroughNonMiniMax(t *testing.T) {
	var capturedPath string
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[]}`)),
			Header:     make(http.Header),
		}, nil
	})

	transport := &opencodeGoTransport{base: base}
	body := `{"model":"gpt-4.1","messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/go/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-key")

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if !strings.HasSuffix(capturedPath, "/chat/completions") {
		t.Fatalf("captured path = %q, want /chat/completions suffix", capturedPath)
	}
}
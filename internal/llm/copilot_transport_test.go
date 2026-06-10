package llm

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func codexResponsesSSEFixture() string {
	return `event: response.created
data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5-codex"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hi"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_1"}}

`
}

func TestCopilotTransportRoutesCodexNonStreamToBufferedJSON(t *testing.T) {
	var capturedPath string
	var capturedBody string
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		raw, _ := io.ReadAll(req.Body)
		capturedBody = string(raw)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(codexResponsesSSEFixture())),
			Header:     make(http.Header),
		}
		resp.Header.Set("Content-Type", "text/event-stream")
		return resp, nil
	})

	auth := NewCopilotAuth()
	auth.mu.Lock()
	auth.copilotToken = "copilot-token"
	auth.expiresAt = time.Now().Add(time.Hour)
	auth.mu.Unlock()

	transport := &copilotTransport{base: base, auth: auth}
	body := `{"model":"gpt-5-codex","messages":[{"role":"user","content":"hello"}],"stream":false}`
	req, err := http.NewRequest(http.MethodPost, CopilotBaseURL+"/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer resp.Body.Close()

	if !strings.HasSuffix(capturedPath, "/responses") {
		t.Fatalf("captured path = %q, want /responses suffix", capturedPath)
	}
	if !strings.Contains(capturedBody, `"input"`) {
		t.Fatalf("captured body = %s, want responses input field", capturedBody)
	}
	if !strings.Contains(capturedBody, `"stream":true`) {
		t.Fatalf("captured body = %s, want upstream stream:true for /responses", capturedBody)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, body = %s", err, raw)
	}
	if payload["object"] != "chat.completion" {
		t.Fatalf("object = %v, want chat.completion", payload["object"])
	}
	choices, ok := payload["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("choices = %v, want non-empty array", payload["choices"])
	}
	choice, _ := choices[0].(map[string]interface{})
	message, _ := choice["message"].(map[string]interface{})
	content, _ := message["content"].(string)
	if content != "Hi" {
		t.Fatalf("content = %q, want Hi", content)
	}
}

func TestCopilotTransportRoutesCodexStreamToSSEChunks(t *testing.T) {
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(codexResponsesSSEFixture())),
			Header:     make(http.Header),
		}
		resp.Header.Set("Content-Type", "text/event-stream")
		return resp, nil
	})

	auth := NewCopilotAuth()
	auth.mu.Lock()
	auth.copilotToken = "copilot-token"
	auth.expiresAt = time.Now().Add(time.Hour)
	auth.mu.Unlock()

	transport := &copilotTransport{base: base, auth: auth}
	body := `{"model":"gpt-5-codex","messages":[{"role":"user","content":"hello"}],"stream":true}`
	req, err := http.NewRequest(http.MethodPost, CopilotBaseURL+"/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer resp.Body.Close()

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)
	var chunks []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			chunks = append(chunks, strings.TrimPrefix(line, "data: "))
		}
	}
	if len(chunks) == 0 {
		t.Fatal("expected translated SSE chunks")
	}
	foundContent := false
	for _, chunk := range chunks {
		if chunk == "[DONE]" {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(chunk), &payload); err != nil {
			continue
		}
		choices, ok := payload["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]interface{})
		delta, _ := choice["delta"].(map[string]interface{})
		if content, _ := delta["content"].(string); strings.Contains(content, "Hi") {
			foundContent = true
		}
	}
	if !foundContent {
		t.Fatalf("chunks = %v, want content delta with Hi", chunks)
	}
}

func TestChatCompletionToResponsesBodyMapsMessages(t *testing.T) {
	in := []byte(`{"model":"gpt-5-codex","messages":[{"role":"user","content":"x"}],"max_tokens":16}`)
	out := chatCompletionToResponsesBody(in)
	if !strings.Contains(string(out), `"input"`) {
		t.Fatalf("output = %s, want input field", out)
	}
	if strings.Contains(string(out), `"messages"`) {
		t.Fatalf("output = %s, messages should be removed", out)
	}
	if !strings.Contains(string(out), `"max_output_tokens"`) {
		t.Fatalf("output = %s, want max_output_tokens", out)
	}
}
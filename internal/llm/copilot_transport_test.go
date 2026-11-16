package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func codexResponsesSSEFixture() string {
	return "data: " + `{"type":"response.created","response":{"id":"resp_1","model":"gpt-5-codex","status":"in_progress"}}` + "\n\n" +
		"data: " + `{"type":"response.output_text.delta","delta":"Hi"}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5-codex","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}` + "\n\n"
}

func responseFixtureClient(t *testing.T, provider, fixture string, inspect func(*http.Request, map[string]json.RawMessage)) ChatClient {
	t.Helper()
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if inspect != nil {
			inspect(req, body)
		}
		contentType := "application/json"
		if !strings.HasPrefix(fixture, "{") {
			contentType = "text/event-stream"
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(fixture))}, nil
	})
	var transport http.RoundTripper = base
	if provider == "copilot" {
		auth := NewCopilotAuth()
		auth.copilotToken, auth.expiresAt = "copilot-token", time.Now().Add(time.Hour)
		transport = &copilotTransport{base: transport, auth: auth}
	}
	transport = &usageObservationTransport{base: transport, provider: provider}
	cc := openai.DefaultConfig("fixture-key")
	cc.BaseURL, cc.HTTPClient = "https://fixture.invalid/v1", &http.Client{Transport: transport}
	return WrapOpenAIClient(openai.NewClientWithConfig(cc), ModelRoute{ProviderType: provider, BaseURL: cc.BaseURL})
}

func TestCopilotTransportRoutesCodexNonStreamToBufferedJSON(t *testing.T) {
	for _, model := range []string{"gpt-5-codex", "copilot/gpt-5.2-codex", "codex-fixture"} {
		t.Run(model, func(t *testing.T) {
			attempts := 0
			client := responseFixtureClient(t, "copilot", codexResponsesSSEFixture(), func(req *http.Request, body map[string]json.RawMessage) {
				attempts++
				if req.URL.Path != "/v1/responses" || req.Header.Get("Accept") != "text/event-stream" || req.Header.Get("Authorization") != "Bearer copilot-token" {
					t.Fatalf("wrong native request: %s %v", req.URL.Path, req.Header)
				}
				for _, key := range []string{"messages", "max_tokens", "max_completion_tokens"} {
					if body[key] != nil {
						t.Fatalf("Chat field %s leaked into Responses", key)
					}
				}
				if string(body["max_output_tokens"]) != "16" || string(body["stream"]) != "true" || strings.Contains(string(body["model"]), "copilot/") {
					t.Fatalf("incorrect Responses request: %s", body)
				}
			})
			response, err := client.CreateChatCompletion(t.Context(), openai.ChatCompletionRequest{Model: model, MaxTokens: 16, Temperature: 0.7, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}})
			if err != nil {
				t.Fatal(err)
			}
			if attempts != 1 || len(response.Choices) != 1 || response.Choices[0].Message.Content != "Hi" || response.Choices[0].FinishReason != "stop" || response.Usage.TotalTokens != 5 {
				t.Fatalf("incorrect buffered completion: %+v", response)
			}
		})
	}
}

func TestCopilotTransportRoutesCodexStreamToSSEChunks(t *testing.T) {
	client := responseFixtureClient(t, "copilot", codexResponsesSSEFixture(), nil)
	stream, err := client.CreateChatCompletionStream(t.Context(), openai.ChatCompletionRequest{Model: "gpt-5-codex", MaxTokens: 16, Temperature: 0.7})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	text, finished := "", false
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, choice := range chunk.Choices {
			text += choice.Delta.Content
			finished = finished || choice.FinishReason == "stop"
		}
	}
	if text != "Hi" || !finished {
		t.Fatalf("text=%q finished=%v", text, finished)
	}
}

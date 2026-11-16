package llm

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func TestInvalidProviderConfigurationDoesNotRetryOrFailover(t *testing.T) {
	for _, message := range []string{"Provider endpoint is missing", "Cloudflare model ID is misspelled", "Provider API key is missing"} {
		_, err := (invalidProviderTransport{message: message}).RoundTrip(&http.Request{})
		err = &url.Error{Op: "Post", URL: "https://fixture.invalid/v1/chat/completions", Err: err}
		if ClassifyError(err) != ErrCategoryNonRetryableConfig || IsRetryable(err) {
			t.Fatalf("local configuration error was retryable: %v", err)
		}
		fm := &FailoverManager{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), fallback: openai.NewClient("fixture"), errorThreshold: 3}
		for range 3 {
			fm.recordError(err)
		}
		if fm.isOnFallback || fm.errorCount != 0 {
			t.Fatal("local configuration failure changed provider health")
		}
	}
}

func TestAnthropicFullMessagesURLReachesOneNativeEndpoint(t *testing.T) {
	for _, base := range []string{
		"https://fixture.invalid", "https://fixture.invalid/v1", "https://fixture.invalid/v1/messages", "https://fixture.invalid/v1/messages/",
		"https://fixture.invalid/proxy/anthropic/v1/messages",
		"https://fixture.invalid/proxy/anthropic/v1/messages/",
	} {
		t.Run(base, func(t *testing.T) {
			cc := buildOpenAIClientConfig(&config.Config{}, resolvedProvider{ProviderType: "anthropic", BaseURL: base, APIKey: "fixture"})
			transport := &anthropicTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				want := "/v1/messages"
				if strings.Contains(base, "/proxy/") {
					want = "/proxy/anthropic/v1/messages"
				}
				if req.URL.Path != want {
					t.Fatalf("path=%q, want %q", req.URL.Path, want)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn"}`))}, nil
			})}
			cc.HTTPClient = &http.Client{Transport: transport}
			_, err := openai.NewClientWithConfig(cc).CreateChatCompletion(t.Context(), openai.ChatCompletionRequest{Model: "claude-sonnet-4-6", MaxTokens: 64, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	const other = "https://fixture.invalid/custom/messages"
	if normalizeProviderBaseURL("openai", other) != other {
		t.Fatal("normalization changed a different provider's custom endpoint")
	}
}

func TestGLMPreservesNativeSystemAndToolReasoning(t *testing.T) {
	client := buildLLMHTTPClient(&config.Config{}, "glm", "", "https://fixture.invalid/v1")
	// Inspect the selected chain by replacing its network leaf, retaining wrappers.
	var replace func(http.RoundTripper) http.RoundTripper
	leaf := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["reasoning_split"]; ok {
			t.Fatal("GLM received a MiniMax parameter")
		}
		var messages []openai.ChatCompletionMessage
		if err := json.Unmarshal(body["messages"], &messages); err != nil {
			t.Fatal(err)
		}
		if len(messages) != 3 || messages[0].Role != "system" || messages[2].ReasoningContent != "native reasoning" {
			t.Fatalf("native messages changed: %+v", messages)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))}, nil
	})
	replace = func(transport http.RoundTripper) http.RoundTripper {
		switch v := transport.(type) {
		case *miniMaxTransport:
			v.base = replace(v.base)
		case *usageObservationTransport:
			v.base = replace(v.base)
		case *leadingSystemTransport:
			v.base = replace(v.base)
		case *responseTimeoutTransport:
			v.base = replace(v.base)
		case *loggingTransport:
			v.base = replace(v.base)
		case *rateLimitAwareTransport:
			v.base = replace(v.base)
		case *http.Transport:
			return leaf
		default:
			t.Fatalf("unexpected transport %T", transport)
		}
		return transport
	}
	client.Transport = replace(client.Transport)
	cc := openai.DefaultConfig("fixture")
	cc.BaseURL, cc.HTTPClient = "https://fixture.invalid/v1", client
	_, err := openai.NewClientWithConfig(cc).CreateChatCompletion(t.Context(), openai.ChatCompletionRequest{Model: "glm-4.7", Messages: []openai.ChatCompletionMessage{
		{Role: "system", Content: "system instruction"}, {Role: "user", Content: "task"}, {Role: "assistant", Content: "", ReasoningContent: "native reasoning"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

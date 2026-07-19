package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

func TestEnableAgnesThinkingPreservesExistingTemplateOptions(t *testing.T) {
	body := []byte(`{
		"model": "agnes-2.0-flash",
		"messages": [{"role": "user", "content": "Hello"}],
		"stream": true,
		"chat_template_kwargs": {
			"enable_thinking": false,
			"custom_option": "keep-me"
		}
	}`)

	result, err := enableAgnesThinking(body)
	if err != nil {
		t.Fatalf("enableAgnesThinking() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	templateOptions, ok := payload["chat_template_kwargs"].(map[string]interface{})
	if !ok {
		t.Fatalf("chat_template_kwargs = %#v, want object", payload["chat_template_kwargs"])
	}
	if enabled, ok := templateOptions["enable_thinking"].(bool); !ok || !enabled {
		t.Fatalf("enable_thinking = %#v, want true", templateOptions["enable_thinking"])
	}
	if got := templateOptions["custom_option"]; got != "keep-me" {
		t.Fatalf("custom_option = %#v, want keep-me", got)
	}
	if stream, ok := payload["stream"].(bool); !ok || !stream {
		t.Fatalf("stream = %#v, want true", payload["stream"])
	}
}

func TestEnableAgnesThinkingAcceptsNullTemplateOptions(t *testing.T) {
	result, err := enableAgnesThinking([]byte(`{
		"model": "agnes-2.0-flash",
		"messages": [],
		"chat_template_kwargs": null
	}`))
	if err != nil {
		t.Fatalf("enableAgnesThinking() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	templateOptions, ok := payload["chat_template_kwargs"].(map[string]interface{})
	if !ok {
		t.Fatalf("chat_template_kwargs = %#v, want object", payload["chat_template_kwargs"])
	}
	if enabled, ok := templateOptions["enable_thinking"].(bool); !ok || !enabled {
		t.Fatalf("enable_thinking = %#v, want true", templateOptions["enable_thinking"])
	}
}

func TestAgnesClientsEnableThinkingByDefault(t *testing.T) {
	var (
		mu       sync.Mutex
		requests []map[string]interface{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("request = %s %s, want POST /v1/chat/completions", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			http.Error(w, "failed to read request", http.StatusBadRequest)
			return
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("unmarshal request body: %v", err)
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, payload)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "chatcmpl-agnes-test",
			"object": "chat.completion",
			"created": 1,
			"model": "agnes-2.0-flash",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "ok"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "agnes"
	cfg.LLM.BaseURL = server.URL + "/v1"
	cfg.LLM.APIKey = "test-key"

	clients := []struct {
		name         string
		client       ChatClient
		wantThinking bool
	}{
		{name: "main", client: NewClient(cfg), wantThinking: true},
		{name: "helper", client: NewClientFromProviderWithConfig(cfg, "agnes", server.URL+"/v1", "test-key", ""), wantThinking: true},
		{name: "other_provider", client: NewClientFromProviderWithConfig(cfg, "openai", server.URL+"/v1", "test-key", ""), wantThinking: false},
	}
	for _, testClient := range clients {
		t.Run(testClient.name, func(t *testing.T) {
			_, err := testClient.client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
				Model: "agnes-2.0-flash",
				Messages: []openai.ChatCompletionMessage{{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hello",
				}},
			})
			if err != nil {
				t.Fatalf("CreateChatCompletion() error = %v", err)
			}

			mu.Lock()
			defer mu.Unlock()
			payload := requests[len(requests)-1]
			templateOptions, exists := payload["chat_template_kwargs"].(map[string]interface{})
			if !testClient.wantThinking {
				if exists {
					t.Fatalf("chat_template_kwargs = %#v, want field omitted", templateOptions)
				}
				return
			}
			if !exists {
				t.Fatalf("chat_template_kwargs = %#v, want object", payload["chat_template_kwargs"])
			}
			if enabled, ok := templateOptions["enable_thinking"].(bool); !ok || !enabled {
				t.Fatalf("enable_thinking = %#v, want true", templateOptions["enable_thinking"])
			}
		})
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != len(clients) {
		t.Fatalf("request count = %d, want %d", len(requests), len(clients))
	}
}

func TestAgnesThinkingTransportPassesOtherRequestsThroughUnchanged(t *testing.T) {
	originalBody := `{"model":"agnes-2.0-flash"}`
	transport := &agnesThinkingTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if got := string(body); got != originalBody {
				t.Fatalf("request body = %q, want unchanged %q", got, originalBody)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://apihub.agnes-ai.com/v1/embeddings", strings.NewReader(originalBody))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

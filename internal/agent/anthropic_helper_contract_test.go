package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/kgextraction"
	"aurago/internal/llm"
	"aurago/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

func TestAnthropicAuxiliaryJSONRequestsUseTextFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, providerType := range []string{"anthropic", "opencode-go", "openai"} {
		model := "claude-sonnet-4-6"
		if providerType == "opencode-go" {
			model = "MiniMax-M3"
		} else if providerType == "openai" {
			model = "gpt-4o"
		}
		for _, helper := range []bool{false, true} {
			for _, path := range []struct {
				name, response string
				call           func(context.Context, *config.Config, llm.ChatClient) error
			}{
				{"helper", `{"ok":true}`, func(ctx context.Context, cfg *config.Config, client llm.ChatClient) error {
					manager := &helperLLMManager{client: client, model: model, providerType: providerType,
						route: llm.ModelRoute{ProviderType: providerType, Model: model, ContextWindowOverride: 8192, MaxOutputTokensOverride: 256}, contextWindow: 8192}
					_, err := manager.requestJSONResponse(ctx, "fixture", "", "Return valid JSON.", "Return an object with ok=true.", 256)
					return err
				}},
				{"personality", `{"ok":true}`, func(ctx context.Context, cfg *config.Config, client llm.ChatClient) error {
					wrapped := &emotionCompletionClient{client: client, cfg: cfg, mainFallback: true}
					_, err := wrapped.CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: model, MaxTokens: 256, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Return valid JSON."}}})
					return err
				}},
				{"consolidation", `{"facts":[]}`, func(ctx context.Context, cfg *config.Config, client llm.ChatClient) error {
					_, err := extractConsolidationFactsWithLLM(ctx, cfg, logger, client, model, "User: The backup service runs on TrueNAS.")
					return err
				}},
				{"consolidation_batch", `{"batches":[{"batch_id":"fixture","facts":[]}]}`, func(ctx context.Context, cfg *config.Config, client llm.ChatClient) error {
					_, err := extractConsolidationBatchesWithLLM(ctx, cfg, logger, client, model, []helperConsolidationBatchInput{{BatchID: "fixture", Conversation: "User: The backup service runs on TrueNAS."}})
					return err
				}},
				{"knowledge_graph", `{"nodes":[{"id":"truenas","label":"TrueNAS","properties":{"type":"software"}}],"edges":[]}`, func(ctx context.Context, cfg *config.Config, client llm.ChatClient) error {
					_, _, err := kgextraction.ExtractKGFromTextWithContext(ctx, cfg, logger, client, "User: I installed TrueNAS on my home server and configured its backup service yesterday.", "")
					return err
				}},
				{"router", `{"domain":"general","complexity":"easy","uncertain":false}`, func(ctx context.Context, cfg *config.Config, _ llm.ChatClient) error {
					_, err := classifyTaskWithHelper(ctx, RunConfig{Config: cfg, Logger: logger}, "Summarize the current status.")
					return err
				}},
			} {
				if path.name == "router" && !helper {
					continue
				}
				mode := "main"
				if helper {
					mode = "helper"
				}
				t.Run(providerType+"/"+mode+"/"+path.name, func(t *testing.T) {
					var calls atomic.Int32
					provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls.Add(1)
						var request map[string]json.RawMessage
						if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
							t.Errorf("decode native request: %v", err)
						}
						w.Header().Set("Content-Type", "application/json")
						if providerType == "openai" {
							var format struct{ Type string }
							if err := json.Unmarshal(request["response_format"], &format); err != nil || format.Type != "json_object" || r.URL.Path != "/v1/chat/completions" {
								t.Errorf("OpenAI JSON mode changed: path=%s format=%s error=%v", r.URL.Path, request["response_format"], err)
							}
							_ = json.NewEncoder(w).Encode(map[string]any{"id": "chatcmpl_fixture", "model": model,
								"choices": []map[string]any{{"index": 0, "message": map[string]string{"role": "assistant", "content": path.response}, "finish_reason": "stop"}},
								"usage":   map[string]int{"prompt_tokens": 4, "completion_tokens": 8, "total_tokens": 12}})
							return
						}
						if r.URL.Path != "/v1/messages" || request["response_format"] != nil {
							t.Errorf("unexpected native path/format: %s %s", r.URL.Path, request["response_format"])
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"id": "msg_fixture", "model": model,
							"content": []map[string]string{{"type": "text", "text": path.response}}, "stop_reason": "end_turn",
							"usage": map[string]int{"input_tokens": 4, "output_tokens": 8}})
					}))
					defer provider.Close()
					cfg := &config.Config{}
					cfg.Agent.ContextWindow = 8192
					cfg.LLM.Provider, cfg.LLM.ProviderType, cfg.LLM.Model = "main", providerType, model
					cfg.LLM.BaseURL, cfg.LLM.APIKey = provider.URL+"/v1", "fixture"
					cfg.Providers = []config.ProviderEntry{{ID: "main", Type: providerType, Model: model, APIKey: "fixture", ContextWindow: 8192, MaxOutputTokens: 256}}
					if helper {
						cfg.LLM.ProviderType, cfg.LLM.Model = "openai", "gpt-4o"
						cfg.Providers[0].Type, cfg.Providers[0].Model = "openai", "gpt-4o"
						if providerType == "openai" {
							cfg.LLM.ProviderType, cfg.LLM.Model = "anthropic", "claude-sonnet-4-6"
							cfg.Providers[0].Type, cfg.Providers[0].Model = "anthropic", "claude-sonnet-4-6"
						}
						cfg.LLM.HelperEnabled, cfg.LLM.HelperProvider = true, "helper"
						cfg.LLM.HelperProviderType, cfg.LLM.HelperResolvedModel = providerType, model
						cfg.LLM.HelperBaseURL, cfg.LLM.HelperAPIKey = provider.URL+"/v1", "fixture"
						cfg.Providers = append(cfg.Providers, config.ProviderEntry{ID: "helper", Type: providerType, Model: model, APIKey: "fixture", ContextWindow: 8192, MaxOutputTokens: 256})
					}
					cfg.LLMRouter.HelperTimeoutMS = 5000
					ResetGlobalHelperLLMManager()
					t.Cleanup(ResetGlobalHelperLLMManager)
					client := llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(cfg, providerType, provider.URL+"/v1", "fixture", ""))
					ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
					defer cancel()
					if err := path.call(ctx, cfg, client); err != nil || calls.Load() != 1 {
						t.Fatalf("native JSON fallback: HTTP calls=%d error=%v", calls.Load(), err)
					}
				})
			}
		}
	}
}

func TestAnthropicHelperPermanentConfigErrorIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer provider.Close()
	manager := &helperLLMManager{
		client: llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(&config.Config{}, "anthropic", provider.URL+"/v1", "fixture", "")),
		model:  "claude-opus-5-5", providerType: "anthropic", contextWindow: 8192,
		route: llm.ModelRoute{ProviderType: "anthropic", Model: "claude-opus-5-5", ContextWindowOverride: 8192, MaxOutputTokensOverride: 4096},
	}
	_, err := manager.requestJSONResponse(t.Context(), "fixture", "", "Return JSON.", "Return an object.", 256)
	if err == nil || !strings.Contains(err.Error(), "cannot disable thinking") || llm.ClassifyError(err) != llm.ErrCategoryNonRetryableConfig || calls.Load() != 0 {
		t.Fatalf("permanent native policy was not rejected before HTTP: calls=%d error=%v", calls.Load(), err)
	}
	if attempts := manager.SnapshotStats()["fixture"].LLMCalls; attempts != 1 {
		t.Fatalf("permanent configuration error took %d attempts, want 1", attempts)
	}
}

func TestHelperRetryPreservesTransientFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{"permanent_wrapped_config", &url.Error{Op: "Post", URL: "https://fixture.invalid", Err: &llm.LLMError{Category: llm.ErrCategoryNonRetryableConfig, Message: "configuration rejected, not a 503 timeout"}}, false},
		{"read_failure", io.ErrUnexpectedEOF, true},
		{"deadline", context.DeadlineExceeded, true},
		{"network", &url.Error{Op: "Post", URL: "https://fixture.invalid", Err: errors.New("connection refused")}, true},
		{"rate_limit", &openai.APIError{HTTPStatusCode: 429, Message: "rate limit"}, true},
		{"server_failure", &openai.APIError{HTTPStatusCode: 503, Message: "server failure"}, true},
		{"vector_disabled", memory.ErrVectorDBDisabled, true},
		{"vector_not_ready", memory.ErrVectorDBNotReady, true},
		{"vector_not_ready_with_config_error", errors.Join(memory.ErrVectorDBNotReady, &llm.LLMError{Category: llm.ErrCategoryNonRetryableConfig, Message: "configuration rejected"}), true},
		{"vector_closed", memory.ErrVectorDBClosed, true},
		{"cancelled", context.Canceled, false},
		{"nil", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isTransientHelperError(test.err); got != test.want {
				t.Fatalf("retry=%t, want %t", got, test.want)
			}
		})
	}
}

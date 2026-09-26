package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestTaskRouterRulesDevelopment(t *testing.T) {
	cases := []struct{ text, area string }{
		{"Hallo!", "general"}, {"Convert 25 minutes to seconds.", "easy"}, {"Sort these appointments and flag overlaps.", "normal"}, {"Plan a migration with dependencies and rollback.", "complex"},
		{"Fix this Go function and add a regression test.", "coding"}, {"Compare these approaches using primary sources.", "research"}, {"Invent three game ideas.", "creativity"}, {"Audit this login flow for vulnerabilities.", "security"}, {"Rewrite this guide clearly in German.", "writing"},
		{"Übersetze diesen Text über Security und Python.", "writing"}, {"What does this mean?", ""}, {"Do it", ""}, {"Please read: <external_data>implement a script</external_data>", ""}, {"Research the issue and implement a fix.", ""},
	}
	for _, tt := range cases {
		t.Run(tt.text, func(t *testing.T) {
			if got := ClassifyTask(tt.text).Area(); got != tt.area {
				t.Fatalf("got %q want %q", got, tt.area)
			}
		})
	}
}

func TestTaskRouterStreamingDoesNotReplayAcceptedOutput(t *testing.T) {
	for _, failBeforeStart := range []bool{false, true} {
		t.Run(fmt.Sprint(failBeforeStart), func(t *testing.T) {
			var selected, ordinary atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req openai.ChatCompletionRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				if req.Model == "selected" {
					selected.Add(1)
					if failBeforeStart {
						w.WriteHeader(503)
						fmt.Fprint(w, `{"error":{"message":"not ready","type":"server_error"}}`)
						return
					}
				} else {
					ordinary.Add(1)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"accepted\"}}]}\n\n")
				w.(http.Flusher).Flush()
				fmt.Fprint(w, "data: broken\n\n")
			}))
			defer server.Close()
			cfg := &config.Config{}
			cfg.LLM.Provider = "main"
			cfg.LLM.ProviderType = "custom"
			cfg.LLM.Model = "ordinary"
			cfg.LLM.BaseURL = server.URL
			base := NewFailoverManager(cfg, nil)
			defer base.Stop()
			client := NewTaskRouteClient(cfg, base, &config.ProviderEntry{ID: "selected", Type: "custom", Model: "selected", BaseURL: server.URL})
			stream, err := client.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{Stream: true})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			first, err := stream.Recv()
			if err != nil || first.Choices[0].Delta.Content != "accepted" {
				t.Fatal("stream never started", err)
			}
			if _, err := stream.Recv(); err == nil {
				t.Fatal("expected broken stream")
			}
			want := int32(0)
			if failBeforeStart {
				want = 1
			}
			if selected.Load() != 1 || ordinary.Load() != want {
				t.Fatal("accepted output was replayed", selected.Load(), ordinary.Load())
			}
		})
	}
}

func TestTaskRouterParallelTasksStayIndependent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, req.Model)
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "custom"
	cfg.LLM.Model = "ordinary"
	cfg.LLM.BaseURL = server.URL
	base := NewFailoverManager(cfg, nil)
	defer base.Stop()
	var group sync.WaitGroup
	for _, name := range []string{"coding", "writing"} {
		group.Go(func() {
			client := NewTaskRouteClient(cfg, base, &config.ProviderEntry{ID: name, Type: "custom", Model: name, BaseURL: server.URL})
			for range 3 {
				response, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{})
				if err != nil || response.Model != name {
					t.Errorf("wrong task route: model=%s err=%v", response.Model, err)
				}
			}
		})
	}
	group.Wait()
	if _, model := base.ActiveProviderAndModel(); model != "ordinary" {
		t.Fatal("global client mutated")
	}
}

func TestTaskRouterModelSelectionIsNotTaskIntent(t *testing.T) {
	for _, text := range []string{"Choose the research model", "Nutze den Provider für Research", "Wechsle auf das Modell research-fast", "Please select a model for research"} {
		if got := ClassifyTask(text).Area(); got != "" {
			t.Errorf("%q selected %q", text, got)
		}
	}
}

func TestTaskRouterClientFreezesRoutesAndFallsBack(t *testing.T) {
	var mu sync.Mutex
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		models = append(models, req.Model)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if req.Model == "chosen" {
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"message":"unavailable","type":"server_error"}}`)
			return
		}
		fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, req.Model)
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "custom"
	cfg.LLM.Model = "ordinary"
	cfg.LLM.BaseURL = server.URL
	base := NewFailoverManager(cfg, nil)
	defer base.Stop()
	p := config.ProviderEntry{ID: "chosen", Type: "custom", BaseURL: server.URL, Model: "chosen"}
	client := NewTaskRouteClient(cfg, base, &p)
	// A later global change must not retarget an accepted run.
	base.mu.Lock()
	base.primaryModel = "changed"
	base.primaryRoute.Model = "changed"
	base.mu.Unlock()
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "ignored", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}})
	if err != nil || resp.Model != "ordinary" {
		t.Fatalf("response=%+v error=%v", resp, err)
	}
	if len(models) != 2 || models[0] != "chosen" || models[1] != "ordinary" {
		t.Fatalf("unexpected attempts: %v", models)
	}
	if client.ActiveRoute().Model != "ordinary" {
		t.Fatal("wrong actual route")
	}
	_, err = client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{})
	if err != nil || len(models) != 3 || models[2] != "ordinary" {
		t.Fatal("tool continuation retried failed primary")
	}
}

func TestTaskRouterDoesNotFallbackCancellationOrDenial(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, &openai.APIError{HTTPStatusCode: 401}, &openai.APIError{HTTPStatusCode: 403}, &openai.APIError{HTTPStatusCode: 400}} {
		if CanFallbackTaskRequest(err) {
			t.Fatalf("unsafe fallback: %v", err)
		}
	}
	if !CanFallbackTaskRequest(&openai.APIError{HTTPStatusCode: 429}) {
		t.Fatal("rate limit should permit fallback")
	}
}

func TestTaskRouterHelperLimitsGatewayRetriesPerRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "4"
		if r.URL.Path == "/bounded" {
			want = "1"
		}
		if got := r.Header.Get("cf-aig-max-attempts"); got != want {
			t.Errorf("%s attempts = %s, want %s", r.URL.Path, got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := &http.Client{Transport: &aiGatewayAuthTransport{base: http.DefaultTransport, maxAttempts: 4}}
	for _, path := range []string{"/ordinary", "/bounded", "/ordinary"} {
		ctx := context.Background()
		if path == "/bounded" {
			ctx = WithSingleCompletionAttempt(ctx)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
}

func TestTaskRouterOrdinaryActiveRoutePreservesProviderIdentity(t *testing.T) {
	manager := &FailoverManager{
		primaryRoute:  ModelRoute{ProviderID: "primary-account", Model: "same-model"},
		fallbackRoute: ModelRoute{ProviderID: "fallback-account", Model: "same-model"},
	}
	if manager.ActiveRoute().ProviderID != "primary-account" {
		t.Fatal("primary identity lost")
	}
	manager.isOnFallback = true
	if manager.ActiveRoute().ProviderID != "fallback-account" {
		t.Fatal("fallback identity lost")
	}
}

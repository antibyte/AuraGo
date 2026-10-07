package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/httporigin"

	"github.com/sashabaranov/go-openai"
)

// Live fixtures for the same-origin redirect policy. Server A is
// the configured provider and answers with a redirect to server B, a second
// origin on another loopback port. B records whether it ever saw a request,
// the provider key or the prompt body.

const redirectFixturePrompt = "SECRET-PROMPT"

type redirectSink struct {
	hits    atomic.Int32
	sawAuth atomic.Bool
	sawBody atomic.Bool
}

func (s *redirectSink) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("cf-aig-authorization") != "" {
			s.sawAuth.Store(true)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), redirectFixturePrompt) {
			s.sawBody.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"leaked"}}]}`))
	}
}

func (s *redirectSink) assertUntouched(t *testing.T) {
	t.Helper()
	if hits := s.hits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s); auth=%v prompt=%v", hits, s.sawAuth.Load(), s.sawBody.Load())
	}
}

func redirectTo(target string, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target+r.URL.Path)
		w.WriteHeader(code)
	}
}

func redirectFixtureChatRequest() openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:    "m",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: redirectFixturePrompt}},
	}
}

func redirectFixtureConfig(baseURL string) *config.Config {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.BaseURL = baseURL
	cfg.LLM.APIKey = "sk-test"
	return cfg
}

// assertRedirectRejected checks that err is the policy rejection and that the
// retry loop treats it as final.
func assertRedirectRejected(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("cross-origin redirect was followed without error")
	}
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}
	if IsRetryable(err) {
		t.Fatalf("rejected redirect classified retryable (category %s)", ClassifyError(err))
	}
}

func TestChatClientDoesNotFollowCrossOriginRedirect(t *testing.T) {
	sink := &redirectSink{}
	second := httptest.NewServer(sink.handler())
	defer second.Close()
	provider := httptest.NewServer(redirectTo(second.URL, http.StatusTemporaryRedirect))
	defer provider.Close()

	client := NewClientFromProvider("openai", provider.URL+"/v1", "sk-test")
	_, err := client.CreateChatCompletion(context.Background(), redirectFixtureChatRequest())
	assertRedirectRejected(t, err)
	sink.assertUntouched(t)
}

func TestStreamingChatClientDoesNotFollowCrossOriginRedirect(t *testing.T) {
	sink := &redirectSink{}
	second := httptest.NewServer(sink.handler())
	defer second.Close()
	provider := httptest.NewServer(redirectTo(second.URL, http.StatusPermanentRedirect))
	defer provider.Close()

	client := NewClientFromProvider("openai", provider.URL+"/v1", "sk-test")
	req := redirectFixtureChatRequest()
	req.Stream = true
	_, err := client.CreateChatCompletionStream(context.Background(), req)
	assertRedirectRejected(t, err)
	sink.assertUntouched(t)
}

func TestLoopbackHTTPSClientDoesNotFollowCrossOriginRedirect(t *testing.T) {
	sink := &redirectSink{}
	second := httptest.NewTLSServer(sink.handler())
	defer second.Close()
	provider := httptest.NewTLSServer(redirectTo(second.URL, http.StatusTemporaryRedirect))
	defer provider.Close()
	if !isLoopbackHTTPS(provider.URL) {
		t.Fatalf("fixture is not loopback HTTPS: %s", provider.URL)
	}

	client := NewClientFromProvider("openai", provider.URL+"/v1", "sk-test")
	_, err := client.CreateChatCompletion(context.Background(), redirectFixtureChatRequest())
	assertRedirectRejected(t, err)
	sink.assertUntouched(t)
}

func TestClientWithTransportDoesNotFollowCrossOriginRedirect(t *testing.T) {
	sink := &redirectSink{}
	second := httptest.NewServer(sink.handler())
	defer second.Close()
	provider := httptest.NewServer(redirectTo(second.URL, http.StatusTemporaryRedirect))
	defer provider.Close()

	client := NewClientWithTransport(redirectFixtureConfig(provider.URL+"/v1"), http.DefaultTransport)
	_, err := client.CreateChatCompletion(context.Background(), redirectFixtureChatRequest())
	assertRedirectRejected(t, err)
	sink.assertUntouched(t)
}

func TestModelsProbeDoesNotFollowCrossOriginRedirect(t *testing.T) {
	sink := &redirectSink{}
	second := httptest.NewServer(sink.handler())
	defer second.Close()
	provider := httptest.NewServer(redirectTo(second.URL, http.StatusFound))
	defer provider.Close()

	if err := probeModelsEndpoints(context.Background(), provider.URL+"/v1", "sk-test"); err == nil {
		t.Fatal("models probe succeeded through a cross-origin redirect")
	}
	sink.assertUntouched(t)
}

func TestChatClientFollowsSameOriginRedirectWithBodyAndAuth(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/v1/chat/completions" {
			w.Header().Set("Location", "/v2/chat/completions")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), redirectFixturePrompt) || r.Header.Get("Authorization") == "" {
			t.Errorf("same-origin hop lost body or auth: body=%q auth=%q", body, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	client := NewClientFromProvider("openai", server.URL+"/v1", "sk-test")
	resp, err := client.CreateChatCompletion(context.Background(), redirectFixtureChatRequest())
	if err != nil || len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("same-origin redirect: err=%v resp=%+v hits=%d", err, resp, hits.Load())
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server hits = %d, want 2 (redirect + followed request)", got)
	}
}

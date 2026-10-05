package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/httporigin"

	"github.com/sashabaranov/go-openai"
)

// Audit H9 follow-up: the legacy Personality V2 client carries the V2 provider
// key and the conversation excerpt, so it must not follow a redirect off the
// configured origin. Server A is the configured endpoint and answers with a 307
// to server B on another loopback port; B must never be reached.
func TestPersonalityV2ClientDoesNotFollowCrossOriginRedirect(t *testing.T) {
	var secondHits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
	}))
	defer second.Close()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", second.URL+r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer provider.Close()

	cfg := &config.Config{}
	cfg.Personality.V2URL = provider.URL + "/v1"
	cfg.Personality.V2APIKey = "sk-personality"

	client := resolvePersonalityAnalyzerClient(cfg, &fakeActivityDigestClient{})
	_, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:    "m",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "SECRET-PROMPT"}},
	})
	if hits := secondHits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s)", hits)
	}
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}
}

// The legacy V2 client is shared with the emotion synthesizer, so it carries no
// client timeout of its own: a reply slower than v2_timeout_secs still arrives
// when the caller's context allows it.
func TestPersonalityV2ClientLeavesTimeoutToCallerContext(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer provider.Close()

	cfg := &config.Config{}
	cfg.Personality.V2URL = provider.URL + "/v1"
	cfg.Personality.V2APIKey = "sk-personality"
	cfg.Personality.V2TimeoutSecs = 1

	client := resolvePersonalityAnalyzerClient(cfg, &fakeActivityDigestClient{})
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:    "m",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hi"}},
	})
	if err != nil || len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("slow reply under a background context: err=%v resp=%+v", err, resp)
	}
}

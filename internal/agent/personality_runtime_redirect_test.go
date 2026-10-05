package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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

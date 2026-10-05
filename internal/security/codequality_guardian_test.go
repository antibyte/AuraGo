package security

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"github.com/danielthedm/promptsec"
	openai "github.com/sashabaranov/go-openai"
)

func TestGuardianProviderFailureDoesNotCacheFallback(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"dangerous 95 unsafe action"}}]}`)
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.LLMGuardian.FailSafe = "allow"
	cfg.LLMGuardian.TimeoutSecs = 5
	clientCfg := openai.DefaultConfig("synthetic-test-key")
	clientCfg.BaseURL = server.URL + "/v1"
	g := &LLMGuardian{cfg: cfg, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), client: openai.NewClientWithConfig(clientCfg), model: "test", cache: NewGuardianCache(60, 10), Metrics: &GuardianMetrics{}, sem: make(chan struct{}, 1)}
	request := promptsec.LLMJudgeRequest{Input: "same unsafe action", Policy: "test"}
	g.Judge(context.Background(), request)
	if g.cache.Size() != 0 {
		t.Fatal("provider failure cached as verdict")
	}
	verdict, err := g.Judge(context.Background(), request)
	if err != nil || verdict.Verdict != promptsec.LLMJudgeVerdictUnsafe || attempts.Load() != 2 {
		t.Fatalf("recovery verdict=%+v err=%v calls=%d", verdict, err, attempts.Load())
	}
}

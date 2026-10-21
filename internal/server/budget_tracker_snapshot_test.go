package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"github.com/sashabaranov/go-openai"
)

func TestBudgetTrackerSnapshotSeesTrackerEnabledAfterBotStartup(t *testing.T) {
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	server.CfgMu.Lock()
	server.reinitBudgetTracker(&config.Config{})
	server.CfgMu.Unlock()

	// Long-lived channel runtimes capture the getter, not its initial nil value.
	trackerSnapshot := server.budgetTrackerSnapshot
	if got := trackerSnapshot(); got != nil {
		t.Fatalf("tracker before budget enable = %p, want nil", got)
	}

	enabled := &config.Config{}
	enabled.Budget.Enabled = true
	enabled.Directories.DataDir = t.TempDir()
	server.CfgMu.Lock()
	server.reinitBudgetTracker(enabled)
	server.CfgMu.Unlock()

	tracker := trackerSnapshot()
	if tracker == nil {
		t.Fatal("captured tracker getter did not observe the hot-enabled tracker")
	}
	t.Cleanup(tracker.Flush)
}

func TestRecoveryProbeBooksTextWithoutUsageWithActualRequest(t *testing.T) {
	var generations atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"data":[{"id":"probe-budget-fixture"}]}`)
			return
		}
		if generations.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":{"message":"fixture primary unavailable"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}]}`)
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.Model = "openai", provider.URL+"/v1", "probe-budget-fixture"
	cfg.FallbackLLM.Enabled, cfg.FallbackLLM.ProviderType, cfg.FallbackLLM.BaseURL, cfg.FallbackLLM.Model = true, "openai", provider.URL+"/v1", "fallback-fixture"
	cfg.FallbackLLM.ErrorThreshold, cfg.FallbackLLM.ProbeIntervalSeconds = 1, 1
	cfg.Budget.Enabled, cfg.Budget.DailyLimitUSD = true, 100
	cfg.Budget.DefaultCost = config.ModelCostRates{InputPerMillion: 1, OutputPerMillion: 1}
	cfg.Directories.DataDir = t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := llm.NewFailoverManager(cfg, logger)
	defer manager.Stop()
	s := &Server{Logger: logger, LLMClient: manager}
	s.reinitBudgetTracker(cfg)
	defer s.BudgetTracker.Flush()
	if _, err := manager.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture failure"}}}); err == nil {
		t.Fatal("fixture must trigger primary failure before recovery")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		usage := s.BudgetTracker.GetStatus().Models[cfg.LLM.Model]
		if usage.Calls > 0 {
			if usage.Calls != 1 || usage.InputTokens <= 0 || usage.OutputTokens <= 0 || usage.EstimatedCalls != 1 || usage.CostUSD <= 0 || generations.Load() != 2 {
				t.Fatalf("probe was not booked exactly once: %+v generations=%d", usage, generations.Load())
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("recovery text without usage was never booked")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/planner"

	"github.com/sashabaranov/go-openai"
)

func TestPersistentCompressionReservesRecapAndReasoningOutput(t *testing.T) {
	for _, reasoning := range []bool{false, true} {
		t.Run(map[bool]string{false: "recap", true: "reasoning"}[reasoning], func(t *testing.T) {
			stm, history := seedPersistentCompressionHistory(t, t.Name())
			cfg := &config.Config{}
			cfg.LLM.Model = "gpt-4o-mini"
			model := cfg.LLM.Model
			want := historySummaryMaxTokens
			if reasoning {
				cfg.LLM.HelperProvider = "summary-helper"
				cfg.LLM.HelperEnabled = true
				cfg.LLM.HelperProviderType = "openrouter"
				cfg.LLM.HelperResolvedModel = "qwen/qwen3-32b"
				model = cfg.LLM.HelperResolvedModel
				cfg.Providers = []config.ProviderEntry{{
					ID: "summary-helper", Type: "openrouter", Model: model,
					ContextWindow: 32768, MaxOutputTokens: llm.ReasoningOutputTokens / 2,
				}}
				want = llm.ReasoningOutputTokens / 2
			}
			client := &mockChatClient{response: "complete bounded recap"}
			result := compressPersistentHistory(context.Background(), RunConfig{
				Config: cfg, ShortTermMem: stm, HistoryManager: history, SessionID: t.Name(),
			}, 0, 1, true, model, client, testLogger)
			if !result.Compressed || client.lastReq.MaxTokens != want {
				t.Fatalf("compressed=%v output budget=%d, want %d", result.Compressed, client.lastReq.MaxTokens, want)
			}
		})
	}
}

func TestPersistentCompressionFailureBackoffPreservesHistoryAndRecovers(t *testing.T) {
	stm, history := seedPersistentCompressionHistory(t, t.Name())
	db := newPlannerTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	runCfg := RunConfig{Config: &config.Config{}, ShortTermMem: stm, HistoryManager: history, SessionID: t.Name(), PlannerDB: db}
	client := &mockChatClient{response: "incomplete recap", finishReason: openai.FinishReasonLength}
	before := len(history.GetAll())
	compress := func() persistentCompressionResult {
		return compressPersistentHistory(context.Background(), runCfg, 0, 1, true, "test", client, testLogger)
	}
	allowRetry := func() {
		historyCompressionFailures.Lock()
		state := historyCompressionFailures.states[t.Name()]
		state.retryAfter = time.Now().Add(-time.Second)
		historyCompressionFailures.states[t.Name()] = state
		historyCompressionFailures.Unlock()
	}
	t.Cleanup(func() { resolveHistoryCompressionFailure(runCfg) })
	if compress().Compressed || compress().Compressed || client.calls != 1 {
		t.Fatalf("failed summary was retried during cooldown: calls=%d", client.calls)
	}
	stored, err := stm.GetSessionMessages(t.Name())
	if err != nil || len(stored) != before || len(history.GetAll()) != before || history.GetSummary() != "" {
		t.Fatalf("failed compression changed history: SQLite=%d history=%d before=%d err=%v", len(stored), len(history.GetAll()), before, err)
	}
	allowRetry()
	if compress().Compressed || client.calls != 2 {
		t.Fatalf("expired cooldown did not allow one retry: calls=%d", client.calls)
	}
	page, err := planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if err != nil || page.Total != 1 || !strings.Contains(page.Items[0].Detail, "incomplete_response") {
		t.Fatalf("missing sanitized failure diagnosis: page=%+v err=%v", page, err)
	}
	allowRetry()
	client.finishReason, client.response = openai.FinishReasonStop, "recovered recap"
	if !compress().Compressed || historyCompressionOnCooldown(t.Name()) {
		t.Fatal("successful summary did not clear failure cooldown")
	}
	page, err = planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if err != nil || page.Total != 0 {
		t.Fatalf("recovered summary left issue open: page=%+v err=%v", page, err)
	}
}

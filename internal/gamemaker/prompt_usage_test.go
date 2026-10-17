package gamemaker

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestPromptUsageCardIgnoresLegacyUsageAndPartialPrices(t *testing.T) {
	if summarizePromptUsage(nil) != nil {
		t.Fatal("no calls still produced a card")
	}
	partial := summarizePromptUsage([]map[string]any{
		{"input_tokens": 100, "output_tokens": 10, "cache_read_tokens": 60, "estimated_cost_usd": 0.01},
		{"input_tokens": 100, "output_tokens": 10, "cache_write_tokens": 9, "failed": true},
	})
	if partial == nil || partial.Calls != 2 || partial.Tokens == nil || *partial.Tokens != 220 || partial.CostUSD != nil {
		t.Fatalf("partial price: %+v", partial)
	}
	missingTokens := summarizePromptUsage([]map[string]any{
		{"input_tokens": 100, "estimated_cost_usd": 0.02},
		{"input_tokens": 5, "output_tokens": 5, "estimated_cost_usd": 0.03},
	})
	if missingTokens == nil || missingTokens.Calls != 2 || missingTokens.Tokens != nil || missingTokens.CostUSD == nil || math.Abs(*missingTokens.CostUSD-0.05) > 1e-9 {
		t.Fatalf("missing tokens: %+v", missingTokens)
	}
	priced := summarizePromptUsage([]map[string]any{
		{"input_tokens": float64(40), "output_tokens": float64(2), "estimated_cost_usd": 0.1},
		{"input_tokens": float64(1), "output_tokens": float64(0), "cache_read_tokens": float64(60), "estimated_cost_usd": 0.2},
	})
	if priced == nil || priced.Calls != 2 || priced.Tokens == nil || *priced.Tokens != 43 || priced.CostUSD == nil || math.Abs(*priced.CostUSD-0.3) > 1e-9 {
		t.Fatalf("priced: %+v", priced)
	}
	zero := summarizePromptUsage([]map[string]any{
		{"input_tokens": 0, "output_tokens": 0, "estimated_cost_usd": 0.0},
	})
	raw, err := json.Marshal(zero)
	if err != nil || zero.Tokens == nil || *zero.Tokens != 0 || zero.CostUSD == nil || *zero.CostUSD != 0 || !strings.Contains(string(raw), `"tokens":0`) || !strings.Contains(string(raw), `"cost_usd":0`) {
		t.Fatalf("zero measurements: %s", raw)
	}

	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	ctx := context.Background()
	jobID := "usage-job"
	if _, err := service.db.Exec(`INSERT INTO gm_jobs(id,project_id,kind,prompt,status,phase,created_at) VALUES(?,?,?,?,?,?,?)`,
		jobID, project.ID, "create", "usage", "failed", "failed", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := service.EmitAgentEvent(ctx, project.ID, jobID, "token_usage", map[string]any{"prompt_tokens": 900, "completion_tokens": 80}); err != nil {
		t.Fatal(err)
	}
	if err := service.EmitAgentEvent(ctx, project.ID, jobID, "prompt_usage", map[string]any{
		"input_tokens": 100, "output_tokens": 10, "cache_read_tokens": 60, "estimated_cost_usd": 0.01,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.EmitAgentEvent(ctx, project.ID, jobID, "prompt_usage", map[string]any{
		"input_tokens": 50, "output_tokens": 5, "failed": true,
	}); err != nil {
		t.Fatal(err)
	}
	card, err := service.promptUsageCard(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if card == nil || card.Calls != 2 || card.Tokens == nil || *card.Tokens != 165 || card.CostUSD != nil {
		t.Fatalf("stored card: %+v", card)
	}
	latest, ok, err := service.LatestJob(ctx, project.ID)
	if err != nil || !ok || latest.Usage == nil || latest.Usage.Calls != 2 || latest.Usage.Tokens == nil || *latest.Usage.Tokens != 165 || latest.Usage.CostUSD != nil {
		t.Fatalf("latest job usage: ok=%v err=%v usage=%+v", ok, err, latest.Usage)
	}
	live, err := service.GetJob(ctx, jobID)
	if err != nil || live.Usage != nil {
		t.Fatalf("GetJob scanned usage: err=%v usage=%+v", err, live.Usage)
	}
	none, err := service.promptUsageCard(ctx, "missing-job")
	if err != nil || none != nil {
		t.Fatalf("empty job: %+v %v", none, err)
	}
}

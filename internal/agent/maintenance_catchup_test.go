package agent

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

func TestRunConsolidationCatchupIsOptInAndHonorsMessageCap(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	for range 4 {
		if _, err := stm.InsertMessage("direct", "user", "Remember the NAS backup target.", false, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.DeleteOldMessages("direct", 1); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Consolidation.Enabled = true
	cfg.Consolidation.MaxBatchMessages = 2
	calls := 0
	client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		return skillQualityTestClient{response: `{"facts":[]}`}.CreateChatCompletion(ctx, req)
	}}
	runConsolidationCatchup(t.Context(), cfg, logger, client, stm, &hierarchyVectorDB{}, nil)
	if calls != 0 {
		t.Fatalf("disabled catch-up made %d LLM calls", calls)
	}
	cfg.Consolidation.CatchupMinutes = 15
	runConsolidationCatchup(t.Context(), cfg, logger, client, stm, &hierarchyVectorDB{}, nil)
	if calls != 1 {
		t.Fatalf("catch-up made %d LLM calls, want one capped batch", calls)
	}
	if backlog, err := stm.CountConsolidationCandidates(3); err != nil || backlog != 1 {
		t.Fatalf("catch-up backlog = %d, %v; want one remaining", backlog, err)
	}
}

func TestRunConsolidationCatchupDeadlineReleasesClaims(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stm, logger := maintenanceRegressionStores(t)
		for range 2 {
			if _, err := stm.InsertMessage("direct", "user", "Remember the NAS backup target.", false, false); err != nil {
				t.Fatal(err)
			}
		}
		if err := stm.DeleteOldMessages("direct", 1); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{}
		cfg.LLM.Model = "test-model"
		cfg.Consolidation.Enabled = true
		cfg.Consolidation.CatchupMinutes = 1
		client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			<-ctx.Done()
			return openai.ChatCompletionResponse{}, ctx.Err()
		}}
		started := time.Now()
		runConsolidationCatchup(t.Context(), cfg, logger, client, stm, &hierarchyVectorDB{}, nil)
		if elapsed := time.Since(started); elapsed != time.Minute {
			t.Fatalf("catch-up elapsed = %v, want one minute", elapsed)
		}
		candidates, err := stm.GetConsolidationCandidates(10, 3)
		if err != nil || len(candidates) != 1 || candidates[0].ConsolidationStatus != "pending" || candidates[0].ConsolidationRetries != 0 {
			t.Fatalf("canceled catch-up candidates = %+v, %v; want unspent pending claim", candidates, err)
		}
	})
}

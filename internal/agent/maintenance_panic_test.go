package agent

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMaintenanceControllerResetsAfterPanic(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	panicked := make(chan struct{})
	controller := newMaintenanceController(context.Background(), maintenanceControllerTestConfig(true), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now: func() time.Time { return clock.now }, timerFactory: clock.timer, claimDay: maintenanceControllerTestClaimDay,
		run: func(context.Context, *config.Config) { close(panicked); panic("synthetic failure") },
	})
	defer controller.Stop(context.Background())
	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	if !clock.fireLatest() {
		t.Fatal("timer missing")
	}
	select {
	case <-panicked:
	case <-time.After(time.Second):
		t.Fatal("run not invoked")
	}
	waitMaintenanceController(t, func() bool { return !controller.Status().Running })
}

func TestMaintenanceCatchupPanicPersistsFailedRunAndReleasesClaims(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	for range 5 {
		if _, err := stm.InsertMessage("panic-session", "user", "The user prefers complete backup reports.", false, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.DeleteOldMessages("panic-session", 1); err != nil {
		t.Fatal(err)
	}
	cfg := enabledMemoryTestConfig()
	cfg.LLM.Model = "test-model"
	cfg.Consolidation.Enabled = true
	cfg.Consolidation.MaxBatchMessages = 1
	cfg.Consolidation.CatchupMinutes = 1
	calls := 0
	client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		if calls > 1 {
			panic("synthetic catchup failure")
		}
		return skillQualityTestClient{response: `{"facts":[]}`}.CreateChatCompletion(ctx, req)
	}}
	runMaintenanceTask(t.Context(), cfg, logger, client, nil, nil, nil, nil, &hierarchyVectorDB{}, stm, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	run, err := stm.GetLatestMaintenanceRun()
	if err != nil || run == nil || run.Status != "failed" || !strings.Contains(strings.Join(run.PhaseResults.Errors, ","), "maintenance_panic") {
		t.Fatalf("failed run missing: %+v %v", run, err)
	}
	if calls != 2 {
		t.Fatalf("mutation continued after panic: calls=%d", calls)
	}
	pending, err := stm.GetConsolidationCandidates(20, 3)
	if err != nil || len(pending) != 3 {
		t.Fatalf("lost pending source: %+v %v", pending, err)
	}
	for _, message := range pending {
		if message.ConsolidationStatus != "pending" || message.ConsolidationRetries != 0 {
			t.Fatalf("claim leaked: %+v", message)
		}
	}
}

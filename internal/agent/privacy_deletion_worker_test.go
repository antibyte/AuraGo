package agent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestPrivacyDeletionWorkerRunsWhenMaintenanceIsDisabled(t *testing.T) {
	logger := slog.Default()
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("open STM: %v", err)
	}
	defer stm.Close()
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("open knowledge graph: %v", err)
	}
	defer kg.Close()

	session, err := stm.CreateChatSession()
	if err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	job, err := stm.PrepareChatPrivacyDeletion(context.Background(), session.ID, nil, nil, false)
	if err != nil {
		t.Fatalf("prepare privacy deletion: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	controller := StartMaintenanceController(ctx, &config.Config{}, MaintenanceControllerDependencies{ShortTermMem: stm, KG: kg, Logger: logger})
	defer func() {
		cancel()
		if err := controller.Stop(context.Background()); err != nil {
			t.Errorf("stop controller: %v", err)
		}
	}()
	if controller.Status().Enabled {
		t.Fatal("maintenance unexpectedly enabled")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, err := stm.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
		if err != nil {
			t.Fatalf("read privacy job: %v", err)
		}
		if current.DeletionStatus == memory.PrivacyDeletionCompleted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	current, err := stm.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil {
		t.Fatalf("read final privacy job: %v", err)
	}
	t.Fatalf("privacy job status = %q, want %q with maintenance disabled", current.DeletionStatus, memory.PrivacyDeletionCompleted)
}

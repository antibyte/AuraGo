package agent

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"

	chromem "github.com/philippgille/chromem-go"
)

func TestEpisodicHierarchyDeadlinePreservesSourceEpisodes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stm, logger := maintenanceRegressionStores(t)
		for _, session := range []string{"first", "second"} {
			for range 2 {
				if err := stm.InsertEpisodicMemoryWithDetails("2026-10-17", "Backup", "Backup configured", nil, 3, "consolidation", memory.EpisodicMemoryDetails{SessionID: session, HierarchyLevel: 1}); err != nil {
					t.Fatal(err)
				}
			}
		}
		vdb := &consolidationDeadlineVector{}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		started := time.Now()
		err := consolidateEpisodicHierarchy(ctx, logger, stm, vdb, nil)
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) != time.Second || vdb.contextCalls != 1 {
			t.Fatalf("elapsed=%v calls=%d err=%v", time.Since(started), vdb.contextCalls, err)
		}
		episodes, err := stm.GetEpisodicMemoriesByHierarchyLevel(1, 40)
		if err != nil || len(episodes) != 4 {
			t.Fatalf("unfinished episodes promoted: count=%d err=%v", len(episodes), err)
		}
		if err := consolidateEpisodicHierarchy(t.Context(), logger, stm, &hierarchyVectorDB{}, nil); err != nil {
			t.Fatal(err)
		}
		episodes, err = stm.GetEpisodicMemoriesByHierarchyLevel(1, 40)
		if err != nil || len(episodes) != 0 {
			t.Fatalf("next pass did not recover: count=%d err=%v", len(episodes), err)
		}
	})
}

func TestGraphMaintenanceStopsFollowingSyncsAfterDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stm, logger := maintenanceRegressionStores(t)
		if err := stm.UpsertMemoryMeta("original"); err != nil {
			t.Fatal(err)
		}
		vdb := &consolidationDeadlineVector{maintenanceOwnedVector: maintenanceOwnedVector{hierarchyVectorDB: hierarchyVectorDB{stored: map[string]string{"original": "AuroraGo backup"}}}}
		kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
		if err != nil {
			t.Fatal(err)
		}
		_ = kg.Close()
		closed, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		_ = closed.Close()
		ledger := newMaintenanceRunLedger()
		ledger.beginPhase("knowledge_graph")
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		runKnowledgeGraphMaintenance(ctx, &config.Config{}, logger, stm, vdb, kg, closed, closed, closed, ledger)
		if vdb.contextCalls != 1 || ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("deadline not reached: calls=%d err=%v", vdb.contextCalls, ctx.Err())
		}
		for _, code := range ledger.phaseResults.Errors {
			if code != "memory_hygiene" {
				t.Fatalf("started another step after deadline: %v", ledger.phaseResults.Errors)
			}
		}
	})
}

func TestPlannerSyncUsesMaintenanceContextForEmbeddings(t *testing.T) {
	_, logger := maintenanceRegressionStores(t)
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	type contextKey struct{}
	if err := kg.EnableSemanticSearchShared(chromem.NewDB(), func(ctx context.Context, _ string) ([]float32, error) {
		if ctx.Value(contextKey{}) != nil {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []float32{1, 0}, nil
	}); err != nil {
		t.Fatal(err)
	}
	db, err := planner.InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.WithValue(t.Context(), contextKey{}, true), 100*time.Millisecond)
	defer cancel()
	if err := SyncPlannerToKnowledgeGraph(ctx, db, kg, logger); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("planner embeddings ignored context: %v", err)
	}
}

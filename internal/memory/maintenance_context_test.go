package memory

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

func TestMaintenanceGraphDeadlineLeavesIndexWorkPending(t *testing.T) {
	for _, operation := range []string{"nodes", "edges", "edge write", "contact", "inventory"} {
		t.Run(operation, func(t *testing.T) {
			kg := newTestKG(t)
			var block atomic.Bool
			var blockedCalls atomic.Int32
			embed := func(ctx context.Context, text string) ([]float32, error) {
				if block.Load() {
					blockedCalls.Add(1)
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return []float32{float32(len(text)), 1}, nil
			}
			if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), embed, nil); err != nil {
				t.Fatal(err)
			}
			waitForSemanticReindexIdle(t, kg)
			if err := kg.AddEdge("nas", "backup", "uses", nil); err != nil {
				t.Fatal(err)
			}
			if operation == "edges" {
				if _, err := kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at=CURRENT_TIMESTAMP`); err != nil {
					t.Fatal(err)
				}
			}
			inventory, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = inventory.Close() })
			if _, err := inventory.Exec(`CREATE TABLE devices (id TEXT, name TEXT, type TEXT, ip_address TEXT, description TEXT, tags TEXT);
			INSERT INTO devices VALUES ('fixture','Fixture','server','','','')`); err != nil {
				t.Fatal(err)
			}
			block.Store(true)
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			started := time.Now()
			switch operation {
			case "nodes", "edges":
				err = kg.RunSemanticReindexContext(ctx)
			case "edge write":
				err = kg.AddEdgeContext(ctx, "nas", "backup", "depends_on", nil)
			case "contact":
				err = kg.SyncContact(ctx, "contact_fixture", "Fixture", map[string]string{"relationship": "Fixture Org"})
			case "inventory":
				err = kg.SyncExternalSourcesContext(ctx, inventory, nil)
			}
			if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
				t.Fatalf("deadline was not respected: elapsed=%v err=%v", time.Since(started), err)
			}
			if blockedCalls.Load() != 1 {
				t.Fatalf("embedding attempts=%d, want one without retries after deadline", blockedCalls.Load())
			}
			nodes, edges, err := kg.DirtySemanticCounts()
			if err != nil || nodes+edges == 0 {
				t.Fatalf("pending index work lost: nodes=%d edges=%d err=%v", nodes, edges, err)
			}
			block.Store(false)
			if err := kg.RunSemanticReindexContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			nodes, edges, err = kg.DirtySemanticCounts()
			if err != nil || nodes+edges != 0 {
				t.Fatalf("next pass did not recover: nodes=%d edges=%d err=%v", nodes, edges, err)
			}
		})
	}
}

func TestSemanticReindexDeadlineWhileWaitingForWriter(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), func(context.Context, string) ([]float32, error) {
		return []float32{1, 0}, nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	waitForSemanticReindexIdle(t, kg)
	if err := kg.AddNode("fixture", "Fixture", nil); err != nil {
		t.Fatal(err)
	}
	kg.semantic.MutationMu.Lock()
	defer kg.semantic.MutationMu.Unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := kg.RunSemanticReindexContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting for writer: %v", err)
	}
}

type deadlineRepairVector struct {
	fakeRepairVectorDB
	calls int
}

func (v *deadlineRepairVector) StoreDocumentOwnedContext(ctx context.Context, _, _ string, _ VectorStoreMode) (VectorStoreResult, error) {
	v.calls++
	<-ctx.Done()
	return VectorStoreResult{}, ctx.Err()
}

func TestCanonicalRepairDeadlinePreservesSourceAndCursor(t *testing.T) {
	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.UpsertMemoryMeta("original"); err != nil {
		t.Fatal(err)
	}
	vdb := &deadlineRepairVector{fakeRepairVectorDB: fakeRepairVectorDB{docs: map[string]string{"original": "AuroraGo uses the backup device."}}}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	report, err := stm.RepairCanonicalMemoryNamesContext(ctx, vdb, CanonicalRepairOptions{Limit: 1})
	if !errors.Is(err, context.DeadlineExceeded) || report.RepairedCount != 0 || vdb.calls != 1 {
		t.Fatalf("report=%+v err=%v calls=%d", report, err, vdb.calls)
	}
	meta, err := stm.GetMemoryMeta("original")
	if err != nil || IsMemoryArchived(meta) || vdb.docs["original"] == "" {
		t.Fatalf("source lost or archived: meta=%+v err=%v", meta, err)
	}
	cursor, err := stm.GetMemoryMaintenanceState(canonicalRepairCursorKey)
	if err != nil || cursor != "" {
		t.Fatalf("cursor skipped unfinished repair: %q err=%v", cursor, err)
	}
}

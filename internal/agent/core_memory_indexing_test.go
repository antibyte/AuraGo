package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/memory"
	"aurago/internal/memory/kgsemantic"
	chromem "github.com/philippgille/chromem-go"
)

func TestCoreMemorySyncRequiresIndexCompletion(t *testing.T) {
	type syncContextKey struct{}
	for _, mode := range []string{"failure", "cancel", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			stm, _ := newMemorySafetyStore(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			path := filepath.Join(t.TempDir(), "graph.db")
			kg, err := memory.NewKnowledgeGraph(path, "", logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = kg.Close() })
			vectors := chromem.NewDB()
			var fail atomic.Bool
			var initialized atomic.Bool
			syncCtx := context.WithValue(t.Context(), syncContextKey{}, true)
			ctx, cancel := context.WithCancel(syncCtx)
			defer cancel()
			if mode != "disabled" {
				if err := kg.EnableSemanticSearchShared(vectors, func(callCtx context.Context, text string) ([]float32, error) {
					// Exercise the requested sync independently of the startup backlog worker.
					if initialized.Load() && callCtx.Value(syncContextKey{}) != true {
						return nil, errors.New("synthetic background embedding unavailable")
					}
					if fail.Load() {
						if mode == "cancel" {
							cancel()
							return nil, callCtx.Err()
						}
						return nil, errors.New("synthetic embedding HTTP 401 Unauthorized")
					}
					if strings.Contains(text, "Foreign pending") {
						return nil, errors.New("synthetic embedding HTTP 401 Unauthorized")
					}
					return []float32{1, 0, 0}, nil
				}); err != nil {
					t.Fatal(err)
				}
				initialized.Store(true)
			}
			id, err := stm.AddCoreMemoryFact("User prefers German")
			if err != nil {
				t.Fatal(err)
			}
			if err := SyncCoreMemoryToKnowledgeGraph(syncCtx, stm, kg, logger); err != nil {
				t.Fatal(err)
			}
			nodeID := fmt.Sprintf("core_fact_%d", id)
			if _, err := kg.SetNodeProtected(nodeID, true); err != nil {
				t.Fatal(err)
			}
			if err := kg.AddNode("core_fact_999", "Stale fact", map[string]string{"source": "core_memory", "type": "concept"}); err != nil {
				t.Fatal(err)
			}
			if err := stm.UpdateCoreMemoryFact(id, "User prefers English"); err != nil {
				t.Fatal(err)
			}
			fail.Store(true)
			syncErr := SyncCoreMemoryToKnowledgeGraph(ctx, stm, kg, logger)
			stale, err := kg.GetNode("core_fact_999")
			if err != nil {
				t.Fatal(err)
			}
			if mode != "disabled" {
				if syncErr == nil || stale == nil {
					t.Fatalf("index failure lost: err=%v stale=%v", syncErr, stale)
				}
				if mode == "cancel" && !errors.Is(syncErr, context.Canceled) {
					t.Fatalf("cancellation lost: %v", syncErr)
				}
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				var pending bool
				err = db.QueryRow(`SELECT semantic_indexed_at IS NULL FROM kg_nodes WHERE id=?`, nodeID).Scan(&pending)
				db.Close()
				if err != nil || !pending {
					t.Fatalf("failed index not pending: %v %v", pending, err)
				}
			} else if syncErr != nil || stale != nil {
				t.Fatalf("disabled index blocks cleanup: %v %v", syncErr, stale)
			}
			fail.Store(false)
			// Legacy writers remain best effort; unrelated pending nodes cannot block Core.
			if err := kg.AddNode("foreign", "Foreign pending", map[string]string{"type": "concept"}); err != nil {
				t.Fatal(err)
			}
			if err := SyncCoreMemoryToKnowledgeGraph(syncCtx, stm, kg, logger); err != nil {
				t.Fatal(err)
			}
			node, err := kg.GetNode(nodeID)
			if err != nil || node == nil || !node.Protected || node.Label != "User prefers English" {
				t.Fatalf("retry/protection: %v %v", node, err)
			}
			if stale, _ := kg.GetNode("core_fact_999"); stale != nil {
				t.Fatal("successful retry did not clean stale node")
			}
			if mode != "disabled" {
				doc, err := vectors.GetCollection(kgsemantic.CollectionName, nil).GetByID(t.Context(), nodeID)
				if err != nil || !strings.Contains(doc.Content, "English") {
					t.Fatalf("index stale: %v %v", doc, err)
				}
			}
		})
	}
}

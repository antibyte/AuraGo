package memory

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestMemoryWritesEmbedEachFactOnce(t *testing.T) {
	for _, path := range []string{"owned", "domain", "batch"} {
		t.Run(path, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				vdb := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
					calls++
					time.Sleep(4 * time.Second)
					return []float32{1, 0, 0}, nil
				})
				started := time.Now()
				for i := range 8 {
					concept, content := fmt.Sprintf("Fact %d", i), fmt.Sprintf("Distinct durable detail %d.", i)
					var err error
					switch path {
					case "owned":
						_, err = vdb.StoreDocumentOwned(concept, content, VectorStoreDeduplicate)
					case "domain":
						_, err = vdb.StoreDocumentWithDomain(concept, content, "ops")
					case "batch":
						_, err = vdb.StoreBatch([]ArchiveItem{{Concept: concept, Content: content}})
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if calls != 8 || vdb.Count() != 8 || time.Since(started) != 32*time.Second {
					t.Fatalf("calls=%d documents=%d duration=%v; want 8/8/32s", calls, vdb.Count(), time.Since(started))
				}
			})
		})
	}
}

func TestMemoryDedupDoesNotAgeIdenticalFacts(t *testing.T) {
	vdb := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
	first, err := vdb.StoreDocumentOwned("Service", "Stable complete fact", VectorStoreDeduplicate)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := vdb.collection.GetByID(t.Context(), first.CreatedIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	doc.Metadata["timestamp"] = strconv.FormatInt(time.Now().AddDate(0, -3, 0).Unix(), 10)
	if err := vdb.collection.AddDocument(t.Context(), doc); err != nil {
		t.Fatal(err)
	}
	again, err := vdb.StoreDocumentOwned("Service", "Stable complete fact", VectorStoreDeduplicate)
	if err != nil || len(again.ReusedIDs) != 1 || again.ReusedIDs[0] != doc.ID || vdb.Count() != 1 {
		t.Fatalf("old identical fact duplicated: result=%+v count=%d error=%v", again, vdb.Count(), err)
	}
}

func TestAutomaticMemoryWriteStopsEmbeddingAtDeadline(t *testing.T) {
	for _, content := range []string{"A bounded fact.", strings.Repeat("A large fact. ", 500)} {
		t.Run(fmt.Sprintf("bytes_%d", len(content)), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				vdb := newTestChromemVectorDB(t, func(ctx context.Context, _ string) ([]float32, error) {
					calls++
					<-ctx.Done()
					return nil, ctx.Err()
				})
				stm, err := NewSQLiteMemory(":memory:", vdb.logger)
				if err != nil {
					t.Fatal(err)
				}
				defer stm.Close()
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				started := time.Now()
				_, err = StoreAutomaticMemoryDocumentWithContext(ctx, stm, vdb, "Deadline", content, MemoryMetaUpdate{SourceType: "consolidation"})
				if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) != time.Second || calls != 1 || vdb.Count() != 0 {
					t.Fatalf("error=%v elapsed=%v calls=%d documents=%d", err, time.Since(started), calls, vdb.Count())
				}
			})
		})
	}
}

func TestMemoryWriteRejectsEmbeddingReturnedAfterCancellation(t *testing.T) {
	for _, mode := range []VectorStoreMode{VectorStoreDeduplicate, VectorStoreForceCreate} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			vdb := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
				cancel()
				return []float32{1, 0, 0}, nil
			})
			_, err := vdb.StoreDocumentOwnedContext(ctx, "Late response", "Do not persist after cancellation.", mode)
			if !errors.Is(err, context.Canceled) || vdb.Count() != 0 {
				t.Fatalf("error=%v documents=%d", err, vdb.Count())
			}
		})
	}
}

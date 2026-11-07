package memory

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

func TestKGCloseCancelsSemanticStartup(t *testing.T) {
	for _, phase := range []string{"validation", "reindex"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "graph.db")
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			kg, err := NewKnowledgeGraph(path, "", logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = kg.Close() })
			if err := kg.AddNode("shutdown-fixture", "Shutdown fixture", map[string]string{"type": "software"}); err != nil {
				t.Fatal(err)
			}

			started := make(chan context.Context, 1)
			release := make(chan struct{})
			var releaseOnce, startedOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			embed := func(ctx context.Context, text string) ([]float32, error) {
				if phase == "reindex" && text == "knowledge graph semantic validation" {
					return []float32{1, 0}, nil
				}
				startedOnce.Do(func() { started <- ctx })
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-release:
					return []float32{1, 0}, nil
				}
			}
			enabled := make(chan error, 1)
			go func() { enabled <- kg.EnableSemanticSearchShared(chromem.NewDB(), embed) }()
			var embeddingCtx context.Context
			select {
			case embeddingCtx = <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("semantic startup did not reach the embedding provider")
			}

			closed := make(chan error, 1)
			go func() { closed <- kg.Close() }()
			select {
			case err := <-closed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Close did not cancel and drain semantic startup")
			}
			if !errors.Is(embeddingCtx.Err(), context.Canceled) {
				t.Fatal("Close left the embedding request running")
			}
			select {
			case err := <-enabled:
				if phase == "validation" && !errors.Is(err, context.Canceled) {
					t.Fatalf("startup error = %v, want cancellation", err)
				}
				if phase == "reindex" && err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("semantic initialization outlived Close")
			}
			if kg.semanticIndex() != nil {
				t.Fatal("semantic index was published after Close")
			}
			if err := kg.EnableSemanticSearchShared(chromem.NewDB(), embed); !errors.Is(err, context.Canceled) {
				t.Fatalf("startup after Close = %v, want cancellation", err)
			}

			reopened, err := NewKnowledgeGraph(path, "", logger)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			var indexedAt sql.NullString
			if err := reopened.db.QueryRow("SELECT semantic_indexed_at FROM kg_nodes WHERE id = ?", "shutdown-fixture").Scan(&indexedAt); err != nil {
				t.Fatalf("source node was not preserved: %v", err)
			}
			if indexedAt.Valid {
				t.Fatal("interrupted embedding was marked indexed instead of left for restart")
			}
		})
	}
}

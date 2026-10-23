package memory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

// seedPendingCorpusDocuments writes pending documents (as left behind by a model
// change) straight into the corpus.
func seedPendingCorpusDocuments(t *testing.T, cv *ChromemVectorDB, docs ...chromem.Document) {
	t.Helper()
	tx, err := cv.corpus.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		if doc.Metadata == nil {
			doc.Metadata = map[string]string{"embedding_fingerprint": "old-model"}
		}
		if doc.Embedding == nil {
			doc.Embedding = []float32{0, 1, 0}
		}
		if err := writeCorpusDocument(t.Context(), tx, "aurago_memories", doc); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func corpusDocumentState(t *testing.T, cv *ChromemVectorDB, id string) (pending int, blocked string) {
	t.Helper()
	if err := cv.corpus.db.QueryRow(`SELECT pending,blocked_reason FROM documents WHERE collection='aurago_memories' AND id=?`, id).Scan(&pending, &blocked); err != nil {
		t.Fatalf("read state of %s: %v", id, err)
	}
	return pending, blocked
}

// A document that startup reconcile blocked because the provider was down must be
// embedded and unblocked by a later pass once the provider works again, without
// touching documents blocked for other reasons.
func TestBlockedEmbeddingRetryPassReembedsAndUnblocks(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	var failing atomic.Bool
	var calls atomic.Int64
	failing.Store(true)
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
		calls.Add(1)
		if failing.Load() {
			return nil, errors.New("embedding provider unavailable")
		}
		return []float32{1, 0, 0}, nil
	}
	seedPendingCorpusDocuments(t, cv,
		chromem.Document{ID: "retry-me", Content: "needs a vector"},
		chromem.Document{ID: "picture", Content: "image description", Metadata: map[string]string{"multimodal": "true"}},
	)

	if err := cv.reconcileCorpus(t.Context()); err == nil {
		t.Fatal("reconcile succeeded although the provider was down")
	}
	if pending, blocked := corpusDocumentState(t, cv, "retry-me"); pending != 1 || blocked != "embedding_failed" {
		t.Fatalf("retry-me after failed reconcile: pending=%d blocked=%q", pending, blocked)
	}

	failing.Store(false)
	calls.Store(0)
	if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err != nil {
		t.Fatalf("retry pass: %v", err)
	}
	if pending, blocked := corpusDocumentState(t, cv, "retry-me"); pending != 0 || blocked != "" {
		t.Fatalf("retry-me after the pass: pending=%d blocked=%q, want published and unblocked", pending, blocked)
	}
	if doc, err := cv.collection.GetByID(t.Context(), "retry-me"); err != nil || len(doc.Embedding) != 3 || doc.Metadata["embedding_fingerprint"] != cv.EmbeddingFingerprint() {
		t.Fatalf("retry-me is not searchable with a current vector: %v %+v", err, doc)
	}
	if pending, blocked := corpusDocumentState(t, cv, "picture"); pending != 1 || blocked != "multimodal_source_reindex_required" {
		t.Fatalf("multimodal document was touched: pending=%d blocked=%q", pending, blocked)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("pass embedded %d documents, want only the one blocked for embedding", got)
	}

	// Nothing is blocked for embedding any more: the next pass does no work.
	calls.Store(0)
	if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err != nil || calls.Load() != 0 {
		t.Fatalf("idle pass: err=%v embeddings=%d", err, calls.Load())
	}
}

// With the provider unreachable a pass must give up after a few failures instead
// of burning a full retry cycle on every blocked document; permanent per-document
// failures must not stop it from reaching the documents behind them.
func TestBlockedEmbeddingRetryPassStopsWhenProviderIsDown(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failure   error
		wantCalls int64
	}{
		{"provider unavailable", &embeddingHTTPStatusError{statusCode: 503}, blockedRetryMaxConsecutiveFailures},
		{"documents rejected individually", &embeddingHTTPStatusError{statusCode: 400}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
			var calls atomic.Int64
			cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
				calls.Add(1)
				return nil, tc.failure
			}
			var ids []string
			var docs []chromem.Document
			for i := range 8 {
				id := fmt.Sprintf("blocked-%02d", i)
				ids = append(ids, id)
				docs = append(docs, chromem.Document{ID: id, Content: "content " + id})
			}
			seedPendingCorpusDocuments(t, cv, docs...)
			if err := cv.setCorpusBlocked(t.Context(), "aurago_memories", ids, "embedding_failed"); err != nil {
				t.Fatal(err)
			}

			if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err == nil {
				t.Fatal("pass reported success although every embedding failed")
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("pass made %d embedding calls, want %d", got, tc.wantCalls)
			}
			for _, id := range ids {
				if pending, blocked := corpusDocumentState(t, cv, id); pending != 1 || blocked != "embedding_failed" {
					t.Fatalf("%s lost its block: pending=%d blocked=%q", id, pending, blocked)
				}
			}
		})
	}
}

// The loop runs passes on its own schedule, ends with its context and is waited
// for by Close.
func TestBlockedEmbeddingRetryLoopRunsPassesAndStopsOnClose(t *testing.T) {
	old := blockedEmbeddingRetryInterval
	blockedEmbeddingRetryInterval = func() time.Duration { return 5 * time.Millisecond }
	t.Cleanup(func() { blockedEmbeddingRetryInterval = old })

	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	seedPendingCorpusDocuments(t, cv, chromem.Document{ID: "retry-me", Content: "needs a vector"})
	if err := cv.setCorpusBlocked(t.Context(), "aurago_memories", []string{"retry-me"}, "embedding_failed"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cv.validationCancel = cancel // what Close cancels, as in NewChromemVectorDB
	cv.startBlockedEmbeddingRetry(ctx)

	deadline := time.Now().Add(10 * time.Second)
	for {
		if pending, blocked := corpusDocumentState(t, cv, "retry-me"); pending == 0 && blocked == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the loop never recovered the blocked document")
		}
		time.Sleep(10 * time.Millisecond)
	}

	started := time.Now()
	if err := cv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("Close took %v, the loop did not stop with it", elapsed)
	}
	if ctx.Err() == nil {
		t.Fatal("Close did not cancel the loop's context")
	}
}

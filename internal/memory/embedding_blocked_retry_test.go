package memory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

// seedBlockedEmbeddingDocuments writes count pending documents that an earlier
// reconcile blocked because their embedding failed, and returns their IDs.
func seedBlockedEmbeddingDocuments(t *testing.T, cv *ChromemVectorDB, count int) []string {
	t.Helper()
	var ids []string
	var docs []chromem.Document
	for i := range count {
		id := fmt.Sprintf("blocked-%03d", i)
		ids = append(ids, id)
		docs = append(docs, chromem.Document{ID: id, Content: "content " + id})
	}
	seedPendingCorpusDocuments(t, cv, docs...)
	if err := cv.setCorpusBlocked(t.Context(), "aurago_memories", ids, "embedding_failed"); err != nil {
		t.Fatal(err)
	}
	return ids
}

// With the provider unreachable a pass must give up after a few failures instead
// of burning a full retry cycle on every blocked document. Credential refusals end
// it at once, and a provider that rejects documents one by one is bounded by a cap
// of failed attempts per pass.
func TestBlockedEmbeddingRetryPassStopsWhenProviderIsDown(t *testing.T) {
	for _, tc := range []struct {
		name         string
		failure      error
		documents    int
		wantCalls    int64
		wantRejected int
	}{
		{"provider unavailable", &embeddingHTTPStatusError{statusCode: 503}, 8, blockedRetryMaxConsecutiveFailures, 0},
		{"unknown failure", errors.New("embedding provider unavailable"), 8, blockedRetryMaxConsecutiveFailures, 0},
		{"credentials refused 401", &embeddingHTTPStatusError{statusCode: 401}, 8, 1, 0},
		{"payment required 402", &embeddingHTTPStatusError{statusCode: 402}, 8, 1, 0},
		{"forbidden 403", &embeddingHTTPStatusError{statusCode: 403}, 8, 1, 0},
		{"documents rejected individually", &embeddingHTTPStatusError{statusCode: 400}, 30, blockedRetryMaxFailedAttempts, blockedRetryMaxFailedAttempts},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
			var calls atomic.Int64
			cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
				calls.Add(1)
				return nil, tc.failure
			}
			ids := seedBlockedEmbeddingDocuments(t, cv, tc.documents)

			if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err == nil {
				t.Fatal("pass reported success although every embedding failed")
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("pass made %d embedding calls, want %d", got, tc.wantCalls)
			}
			rejected := 0
			for _, id := range ids {
				pending, blocked := corpusDocumentState(t, cv, id)
				if pending != 1 {
					t.Fatalf("%s was published: pending=%d", id, pending)
				}
				switch blocked {
				case "embedding_rejected":
					rejected++
				case "embedding_failed":
				default:
					t.Fatalf("%s has blocked reason %q", id, blocked)
				}
			}
			if rejected != tc.wantRejected {
				t.Fatalf("%d documents marked embedding_rejected, want %d", rejected, tc.wantRejected)
			}
		})
	}
}

// A document the provider rejected for good must not be sent again by every later
// pass; documents blocked only by a failure still are.
func TestBlockedEmbeddingRetryPassDoesNotResendRejectedDocuments(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	var calls atomic.Int64
	var status atomic.Int64
	status.Store(http.StatusBadRequest)
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
		calls.Add(1)
		return nil, &embeddingHTTPStatusError{statusCode: int(status.Load())}
	}
	ids := seedBlockedEmbeddingDocuments(t, cv, 3)

	if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err == nil {
		t.Fatal("first pass reported success although the provider rejected every document")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("first pass made %d calls, want 3", got)
	}
	for _, id := range ids {
		requireCorpusRowState(t, cv, "aurago_memories", id, 1, "embedding_rejected")
	}

	calls.Store(0)
	status.Store(http.StatusOK)
	if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("second pass sent %d rejected documents again", got)
	}
	for _, id := range ids {
		requireCorpusRowState(t, cv, "aurago_memories", id, 1, "embedding_rejected")
	}
}

// With the provider down the startup reconcile must not pay a full retry cycle for
// every pending document. After the breaker trips, the remaining documents are
// marked for the runtime retry loop without another provider call.
func TestReconcileStopsEmbeddingWhenTheProviderIsDown(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failure   error
		documents int
		wantCalls int64
	}{
		{"provider unavailable", &embeddingHTTPStatusError{statusCode: 503}, 10, blockedRetryMaxConsecutiveFailures},
		{"unknown failure across several pages", errors.New("embedding provider unavailable"), 300, blockedRetryMaxConsecutiveFailures},
		{"credentials refused", &embeddingHTTPStatusError{statusCode: 401}, 10, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
			var calls atomic.Int64
			var failing atomic.Bool
			failing.Store(true)
			cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
				calls.Add(1)
				if failing.Load() {
					return nil, tc.failure
				}
				return []float32{1, 0, 0}, nil
			}
			var ids []string
			var docs []chromem.Document
			for i := range tc.documents {
				id := fmt.Sprintf("pending-%03d", i)
				ids = append(ids, id)
				docs = append(docs, chromem.Document{ID: id, Content: "content " + id})
			}
			seedPendingCorpusDocuments(t, cv, docs...)

			if err := cv.reconcileCorpus(t.Context()); err == nil {
				t.Fatal("reconcile reported success although the provider was down")
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("reconcile made %d embedding calls, want %d", got, tc.wantCalls)
			}
			for _, id := range ids {
				requireCorpusRowState(t, cv, "aurago_memories", id, 1, "embedding_failed")
			}

			failing.Store(false)
			calls.Store(0)
			if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err != nil {
				t.Fatalf("retry pass with a working provider: %v", err)
			}
			if got := calls.Load(); got != int64(tc.documents) {
				t.Fatalf("retry pass embedded %d documents, want %d", got, tc.documents)
			}
			for _, id := range ids {
				requireCorpusRowState(t, cv, "aurago_memories", id, 0, "")
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

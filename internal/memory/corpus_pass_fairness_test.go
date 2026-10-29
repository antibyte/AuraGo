package memory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

// seedPendingCorpusDocumentsIn is seedPendingCorpusDocuments for another collection.
func seedPendingCorpusDocumentsIn(t *testing.T, cv *ChromemVectorDB, collection string, docs ...chromem.Document) {
	t.Helper()
	tx, err := cv.corpus.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		doc.Metadata = map[string]string{"embedding_fingerprint": "old-model"}
		doc.Embedding = []float32{0, 1, 0}
		if err := writeCorpusDocument(t.Context(), tx, collection, doc); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// blockCorpusDocuments writes documents as an earlier reconcile left them: pending,
// blocked because their embedding failed.
func blockCorpusDocuments(t *testing.T, cv *ChromemVectorDB, collection string, docs ...chromem.Document) {
	t.Helper()
	seedPendingCorpusDocumentsIn(t, cv, collection, docs...)
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	if err := cv.setCorpusBlocked(t.Context(), collection, ids, blockedEmbeddingReason); err != nil {
		t.Fatal(err)
	}
}

// failingBadDocuments makes the provider answer every text containing "BAD" with
// failure and embed all others. It returns the number of requests for bad texts.
func failingBadDocuments(cv *ChromemVectorDB, failure error) *atomic.Int64 {
	var badCalls atomic.Int64
	cv.embeddingFunc = func(_ context.Context, text string) ([]float32, error) {
		if strings.Contains(text, "BAD") {
			badCalls.Add(1)
			return nil, failure
		}
		return []float32{1, 0, 0}, nil
	}
	return &badCalls
}

// freshGoodDocument returns the good document that is blocked before pass number
// pass (1-based). Its ID sorts after the bad documents ("m-10", "m-11") and below
// every earlier fresh one. The pass resumes behind the previous pass's last document,
// which is the previous fresh one, so it reaches the new one only after wrapping
// around, that is after the bad documents: in every pass the provider embeds a
// document after the bad ones failed.
func freshGoodDocument(pass int) chromem.Document {
	id := fmt.Sprintf("m-%d", 100-pass)
	return chromem.Document{ID: id, Content: "good " + id}
}

// Documents that keep failing with provider-side errors (llama-server answering 500
// for a chunk larger than its batch) sort first and used to open the breaker at the
// same place on every pass, so nothing behind them was ever tried, in their
// collection or in later ones. The retry loop must resume after the last document
// the previous pass attempted.
func TestRetryPassMovesPastDocumentsThatKeepFailing(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	badCalls := failingBadDocuments(cv, &embeddingHTTPStatusError{statusCode: 500})
	seedPendingCorpusDocuments(t, cv,
		chromem.Document{ID: "a-1", Content: "BAD chunk 1"},
		chromem.Document{ID: "a-2", Content: "BAD chunk 2"},
		chromem.Document{ID: "a-3", Content: "BAD chunk 3"},
		chromem.Document{ID: "b-1", Content: "good one"},
		chromem.Document{ID: "b-2", Content: "good two"},
	)
	seedPendingCorpusDocumentsIn(t, cv, "zz_later", chromem.Document{ID: "c-1", Content: "later collection"})

	if err := cv.reconcileCorpus(t.Context()); err == nil {
		t.Fatal("reconcile reported success although three documents failed")
	}
	// The first retry pass meets the same three documents and opens the breaker again.
	if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err == nil {
		t.Fatal("first retry pass reported success although three documents failed")
	}
	// The second pass resumes behind them.
	_ = cv.runBlockedEmbeddingRetryPass(t.Context())
	requireCorpusRowState(t, cv, "aurago_memories", "b-1", 0, "")
	requireCorpusRowState(t, cv, "aurago_memories", "b-2", 0, "")
	requireCorpusRowState(t, cv, "zz_later", "c-1", 0, "")

	// Three adjacent failures are the last thing a pass tries and nothing is embedded
	// after them, so nothing tells them from a provider that went down meanwhile:
	// they stay blocked for failure, and each pass spends only the breaker's three
	// requests on them.
	for pass := 3; pass <= 4; pass++ {
		before := badCalls.Load()
		if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err == nil {
			t.Fatalf("retry pass %d reported success although three documents failed", pass)
		}
		if got := badCalls.Load() - before; got != blockedRetryMaxConsecutiveFailures {
			t.Fatalf("retry pass %d sent %d requests for the failing documents, want %d", pass, got, blockedRetryMaxConsecutiveFailures)
		}
	}
	for _, id := range []string{"a-1", "a-2", "a-3"} {
		requireCorpusRowState(t, cv, "aurago_memories", id, 1, "embedding_failed")
	}
}

// A document the provider answers with a server error pass after pass, while the
// provider embeds another document after it in the same pass each time, is the cause
// of the failures and leaves the runtime loop.
func TestRetryPassRejectsDocumentsThatFailWhileOthersAreEmbeddedAfterThem(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		badCollection  string
		goodCollection string
	}{
		{"500 in one collection", 500, "aurago_memories", "aurago_memories"},
		{"500, good documents in a later collection", 500, "aurago_memories", "zz_later"},
		{"502", 502, "aurago_memories", "aurago_memories"},
		{"504", 504, "aurago_memories", "aurago_memories"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
			badCalls := failingBadDocuments(cv, &embeddingHTTPStatusError{statusCode: tc.status})
			blockCorpusDocuments(t, cv, tc.badCollection,
				chromem.Document{ID: "m-10", Content: "BAD one"},
				chromem.Document{ID: "m-11", Content: "BAD two"})

			for pass := 1; pass <= corpusDocumentMaxFailedPasses; pass++ {
				good := freshGoodDocument(pass)
				blockCorpusDocuments(t, cv, tc.goodCollection, good)
				_ = cv.runBlockedEmbeddingRetryPass(t.Context())
				requireCorpusRowState(t, cv, tc.goodCollection, good.ID, 0, "")
				want := "embedding_failed"
				if pass == corpusDocumentMaxFailedPasses {
					want = "embedding_rejected"
				}
				for _, id := range []string{"m-10", "m-11"} {
					requireCorpusRowState(t, cv, tc.badCollection, id, 1, want)
				}
			}

			// A rejected document is not sent again.
			settled := badCalls.Load()
			for pass := corpusDocumentMaxFailedPasses + 1; pass <= corpusDocumentMaxFailedPasses+2; pass++ {
				blockCorpusDocuments(t, cv, tc.goodCollection, freshGoodDocument(pass))
				if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err != nil {
					t.Fatalf("pass %d: %v", pass, err)
				}
			}
			if got := badCalls.Load(); got != settled {
				t.Fatalf("rejected documents were sent again: %d requests before the verdict, %d after", settled, got)
			}
		})
	}
}

// Only a definite answer about the input blames a document. Deadlines, transport
// errors, errors of the local embeddings runtime and overload say something about the
// provider: with the provider embedding other documents after the failing ones in
// every pass, and foreground stores succeeding between the passes, such documents
// are never rejected, however many passes fail.
func TestRetryPassNeverRejectsDocumentsForFailuresThatAreNotAboutThem(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
	}{
		{"deadline", context.DeadlineExceeded},
		{"request deadline", &embeddingTransportError{cause: context.DeadlineExceeded}},
		{"transport", &embeddingTransportError{cause: errors.New("connection reset by peer")}},
		{"local runtime", errors.New("local embeddings runtime: restart limit reached")},
		{"local runtime unresponsive", errors.New("local embeddings backend unresponsive")},
		{"rate limited", &embeddingHTTPStatusError{statusCode: 429}},
		{"overloaded", &embeddingHTTPStatusError{statusCode: 503}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
			badCalls := failingBadDocuments(cv, tc.failure)
			blockCorpusDocuments(t, cv, "aurago_memories",
				chromem.Document{ID: "m-10", Content: "BAD one"},
				chromem.Document{ID: "m-11", Content: "BAD two"})

			for pass := 1; pass <= 2*corpusDocumentMaxFailedPasses; pass++ {
				good := freshGoodDocument(pass)
				blockCorpusDocuments(t, cv, "aurago_memories", good)
				// A foreground store succeeds between the passes.
				if err := cv.embedDocumentsOutsideLock(t.Context(), []chromem.Document{{ID: fmt.Sprintf("fg-%d", pass), Content: "foreground store"}}); err != nil {
					t.Fatal(err)
				}
				before := badCalls.Load()
				_ = cv.runBlockedEmbeddingRetryPass(t.Context())
				if badCalls.Load() == before {
					t.Fatalf("pass %d never tried the failing documents; the test would prove nothing", pass)
				}
				requireCorpusRowState(t, cv, "aurago_memories", good.ID, 0, "")
			}
			for _, id := range []string{"m-10", "m-11"} {
				requireCorpusRowState(t, cv, "aurago_memories", id, 1, "embedding_failed")
			}
		})
	}
}

// A provider that is merely down must never get documents rejected: the failures
// say nothing about them unless the provider embedded other documents meanwhile.
func TestRetryPassKeepsDocumentsWhileNothingElseIsEmbedded(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
		return nil, &embeddingHTTPStatusError{statusCode: 500}
	}
	ids := seedBlockedEmbeddingDocuments(t, cv, 6)
	for range corpusDocumentMaxFailedPasses + 1 {
		_ = cv.runBlockedEmbeddingRetryPass(t.Context())
	}
	for _, id := range ids {
		requireCorpusRowState(t, cv, "aurago_memories", id, 1, "embedding_failed")
	}
}

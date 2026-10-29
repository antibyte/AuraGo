package memory

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

// wantPassDocumentTimeout pins the deadline that background passes give each document.
const wantPassDocumentTimeout = 60 * time.Second

// Background passes retry on their own schedule, so they must not sit through the
// in-call retry waits of the embedding wrapper while holding the store slot. A 429
// with Retry-After: 30 costs one request and no sleep there, and every document has
// a deadline of its own.
func TestBackgroundPassMakesOneAttemptPerDocument(t *testing.T) {
	waits := stubEmbeddingRetrySleep(t)
	var mu sync.Mutex
	var requests int
	var lastLeft time.Duration
	inner := func(ctx context.Context, _ string) ([]float32, error) {
		mu.Lock()
		defer mu.Unlock()
		requests++
		lastLeft = -1
		if deadline, ok := ctx.Deadline(); ok {
			lastLeft = time.Until(deadline)
		}
		return nil, &embeddingHTTPStatusError{statusCode: 429, retryAfter: 30 * time.Second}
	}
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	cv.embeddingFunc = withEmbeddingRetry(inner, nil)
	seedBlockedEmbeddingDocuments(t, cv, 8)

	if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err == nil {
		t.Fatal("pass reported success although the provider answered 429")
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != blockedRetryMaxConsecutiveFailures {
		t.Fatalf("the pass sent %d requests, want one per document until the breaker opened (%d)", requests, blockedRetryMaxConsecutiveFailures)
	}
	if got := waits.all(); len(got) != 0 {
		t.Fatalf("the pass waited %v inside the store slot, want no retry wait", got)
	}
	if lastLeft <= 0 || lastLeft > wantPassDocumentTimeout {
		t.Fatalf("request ran with %v left, want a per-document deadline of at most %v", lastLeft, wantPassDocumentTimeout)
	}
}

// holdEmbeddingSlot makes cv's embedding provider block for text "holder" until the
// returned release is called, and returns once that call is inside the provider. Any
// other text embeds at once and is reported on seen.
func holdEmbeddingSlot(t *testing.T, cv *ChromemVectorDB) (release func(), seen <-chan time.Duration) {
	t.Helper()
	gate := make(chan struct{})
	inside := make(chan struct{})
	observed := make(chan time.Duration, 8)
	cv.embeddingFunc = func(ctx context.Context, text string) ([]float32, error) {
		if text == "holder" {
			close(inside)
			select {
			case <-gate:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return []float32{1, 0, 0}, nil
		}
		left := time.Duration(-1)
		if deadline, ok := ctx.Deadline(); ok {
			left = time.Until(deadline)
		}
		observed <- left
		return []float32{1, 0, 0}, nil
	}
	done := make(chan error, 1)
	go func() {
		done <- cv.embedDocumentsOutsideLock(context.Background(), []chromem.Document{{ID: "held", Content: "holder"}})
	}()
	select {
	case <-inside:
	case <-time.After(5 * time.Second):
		t.Fatal("the holder never reached the provider")
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			close(gate)
			if err := <-done; err != nil {
				t.Errorf("holder: %v", err)
			}
		})
	}
	t.Cleanup(release)
	return release, observed
}

// A foreground store must give up when its own context ends, not when the slot
// holder is done: behind a slow background call it used to wait for minutes.
func TestEmbeddingSlotWaiterReturnsWhenItsContextEnds(t *testing.T) {
	cv := newTestChromemVectorDB(t, stubEmbedding)
	release, _ := holdEmbeddingSlot(t, cv)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- cv.embedDocumentsOutsideLock(ctx, []chromem.Document{{ID: "waiting", Content: "foreground"}})
	}()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiter error = %v, want its context deadline", err)
		}
	case <-time.After(3 * time.Second):
		release()
		t.Fatal("the waiter stayed blocked behind the slot holder past its own deadline")
	}
	release()

	// The slot is usable again, and still hands out one call at a time.
	if err := cv.embedDocumentsOutsideLock(t.Context(), []chromem.Document{{ID: "after", Content: "foreground"}}); err != nil {
		t.Fatalf("embedding after the slot was released: %v", err)
	}
}

// A tool-guide batch has a deadline for its provider calls and publication. Waiting
// for the store slot behind another caller must not eat into it.
func TestIndexBatchDeadlineStartsAfterTheSlotWait(t *testing.T) {
	cv := newTestChromemVectorDB(t, stubEmbedding)
	cv.dataDir = t.TempDir()
	release, seen := holdEmbeddingSlot(t, cv)
	col, err := cv.db.GetOrCreateCollection("batch_wait", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}

	const wait = 400 * time.Millisecond
	time.AfterFunc(wait, release)
	docs := []chromem.Document{{ID: "guide-1", Content: "guide text " + strings.Repeat("x", 8)}}
	if _, err := cv.addDocumentsInBatches(t.Context(), col, docs, false); err != nil {
		t.Fatalf("addDocumentsInBatches: %v", err)
	}
	select {
	case left := <-seen:
		full := calculateBatchTimeout(1)
		if left < full-wait/2 {
			t.Fatalf("the provider call had %v left of a %v batch deadline, the slot wait was charged to it", left, full)
		}
	default:
		t.Fatal("the batch never reached the provider")
	}
}

func stubEmbedding(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil }

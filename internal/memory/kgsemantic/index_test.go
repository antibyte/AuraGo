package kgsemantic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestContentCacheRefreshesRecencyAndRemovesAllKeyEntries(t *testing.T) {
	idx := &Index{}

	idx.SetContentCacheEntry("old", "old content")
	idx.SetContentCacheEntry("middle", "middle content")
	idx.SetContentCacheEntry("old", "fresh content")

	if got := idx.ContentKeys; len(got) != 2 || got[0] != "middle" || got[1] != "old" {
		t.Fatalf("ContentKeys after refresh = %#v, want middle then old", got)
	}
	if idx.ContentCache["old"] != "fresh content" {
		t.Fatalf("ContentCache[old] = %q, want refreshed content", idx.ContentCache["old"])
	}

	idx.ContentKeys = append(idx.ContentKeys, "old")
	idx.RemoveContentCacheEntry("old")
	if _, ok := idx.ContentCache["old"]; ok {
		t.Fatal("expected old cache entry removed")
	}
	for _, key := range idx.ContentKeys {
		if key == "old" {
			t.Fatalf("expected all old keys removed, got %#v", idx.ContentKeys)
		}
	}
}

func TestQueryEmbeddingOneWaiterCancelDoesNotStopSharedCall(t *testing.T) {
	idx := &Index{QueryCache: make(map[string]QueryCacheEntry), QueryCacheTTL: QueryCacheTTL}
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	var calls atomic.Int32
	embed := func(ctx context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		started <- ctx
		select {
		case <-release:
			return []float32{1, 0}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := idx.QueryEmbedding(firstCtx, "same", embed)
		firstDone <- err
	}()
	providerCtx := <-started
	secondDone := make(chan error, 1)
	go func() {
		embedding, err := idx.QueryEmbedding(context.Background(), "same", embed)
		if err == nil && (len(embedding) != 2 || embedding[0] != 1) {
			err = errors.New("second waiter received the wrong embedding")
		}
		secondDone <- err
	}()
	waitForQueryWaiters(t, idx, "same", 2)

	cancelFirst()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first waiter error = %v, want context canceled", err)
	}
	select {
	case <-providerCtx.Done():
		t.Fatal("shared provider call canceled while a second waiter remained")
	default:
	}
	close(release)
	if err := <-secondDone; err != nil {
		t.Fatalf("second waiter error: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func TestQueryEmbeddingLastCancelDetachesAndIgnoresLateProviderReturn(t *testing.T) {
	idx := &Index{QueryCache: make(map[string]QueryCacheEntry), QueryCacheTTL: QueryCacheTTL}
	started := make(chan context.Context, 2)
	releaseOld := make(chan struct{})
	releaseNew := make(chan struct{})
	var calls atomic.Int32
	embed := func(ctx context.Context, _ string) ([]float32, error) {
		call := calls.Add(1)
		started <- ctx
		if call == 1 {
			<-releaseOld // Deliberately ignore cancellation to exercise a late return.
			return []float32{1}, nil
		}
		select {
		case <-releaseNew:
			return []float32{2}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := idx.QueryEmbedding(firstCtx, "same", embed)
		firstDone <- err
	}()
	providerCtxOld := <-started
	oldFlight := waitForQueryWaiters(t, idx, "same", 1)
	cancelFirst()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("abandoned waiter error = %v, want context canceled", err)
	}
	select {
	case <-providerCtxOld.Done():
	case <-time.After(time.Second):
		t.Fatal("last waiter did not cancel the old provider call")
	}

	secondDone := make(chan []float32, 1)
	secondErr := make(chan error, 1)
	go func() {
		embedding, err := idx.QueryEmbedding(context.Background(), "same", embed)
		secondDone <- embedding
		secondErr <- err
	}()
	providerCtxNew := <-started
	newFlight := waitForQueryWaiters(t, idx, "same", 1)
	if oldFlight == newFlight {
		t.Fatal("new caller joined the abandoned flight")
	}

	close(releaseOld)
	select {
	case <-oldFlight.returned:
	case <-time.After(time.Second):
		t.Fatal("old provider call did not return")
	}
	idx.Mu.Lock()
	_, cached := idx.QueryCache["same"]
	idx.Mu.Unlock()
	if cached {
		t.Fatal("late abandoned provider result was cached")
	}
	idx.queryMu.Lock()
	stillCurrent := idx.queryFlights["same"] == newFlight
	idx.queryMu.Unlock()
	if !stillCurrent {
		t.Fatal("late abandoned provider return removed the newer flight")
	}

	thirdDone := make(chan error, 1)
	go func() {
		_, err := idx.QueryEmbedding(context.Background(), "same", embed)
		thirdDone <- err
	}()
	waitForQueryWaiters(t, idx, "same", 2)
	if got := calls.Load(); got != 2 {
		t.Fatalf("provider calls before release = %d, want 2", got)
	}
	close(releaseNew)
	for name, result := range map[string]chan error{"second": secondErr, "third": thirdDone} {
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("%s waiter error: %v", name, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s waiter did not complete", name)
		}
	}
	if embedding := <-secondDone; len(embedding) != 1 || embedding[0] != 2 {
		t.Fatalf("new flight embedding = %v, want [2]", embedding)
	}
	idx.Mu.Lock()
	cachedEmbedding := cloneEmbedding(idx.QueryCache["same"].Embedding)
	idx.Mu.Unlock()
	if len(cachedEmbedding) != 1 || cachedEmbedding[0] != 2 {
		t.Fatalf("cached embedding = %v, want the newer [2] result", cachedEmbedding)
	}
	select {
	case <-providerCtxNew.Done():
	default:
		t.Fatal("completed provider context was not canceled")
	}
}

func TestQueryEmbeddingCallerTimeoutCancelsLastWaiter(t *testing.T) {
	idx := &Index{QueryCache: make(map[string]QueryCacheEntry), QueryCacheTTL: QueryCacheTTL}
	started := make(chan context.Context, 1)
	providerCanceled := make(chan struct{})
	embed := func(ctx context.Context, _ string) ([]float32, error) {
		started <- ctx
		<-ctx.Done()
		close(providerCanceled)
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := idx.QueryEmbedding(ctx, "timeout", embed)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("query error = %v, want deadline exceeded", err)
	}
	providerCtx := <-started
	select {
	case <-providerCanceled:
	case <-time.After(time.Second):
		t.Fatal("last waiter timeout did not cancel provider context")
	}
	if providerCtx.Err() == nil {
		t.Fatal("provider context remains active after the only waiter timed out")
	}
}

func TestQueryEmbeddingDoesNotCacheProviderErrors(t *testing.T) {
	idx := &Index{QueryCache: make(map[string]QueryCacheEntry), QueryCacheTTL: QueryCacheTTL}
	wantErr := errors.New("temporary embedding failure")
	var calls atomic.Int32
	embed := func(context.Context, string) ([]float32, error) {
		if calls.Add(1) == 1 {
			return nil, wantErr
		}
		return []float32{3}, nil
	}

	if _, err := idx.QueryEmbedding(context.Background(), "retry", embed); !errors.Is(err, wantErr) {
		t.Fatalf("first query error = %v, want %v", err, wantErr)
	}
	embedding, err := idx.QueryEmbedding(context.Background(), "retry", embed)
	if err != nil {
		t.Fatalf("second query error: %v", err)
	}
	if len(embedding) != 1 || embedding[0] != 3 {
		t.Fatalf("second query embedding = %v, want [3]", embedding)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
}

func TestQueryEmbeddingCloseWakesWaiters(t *testing.T) {
	idx := &Index{QueryCache: make(map[string]QueryCacheEntry), QueryCacheTTL: QueryCacheTTL}
	started := make(chan struct{})
	embed := func(ctx context.Context, _ string) ([]float32, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		_, err := idx.QueryEmbedding(context.Background(), "close", embed)
		done <- err
	}()
	<-started
	idx.Close()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiter error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not wake the query waiter")
	}
}

func waitForQueryWaiters(t *testing.T, idx *Index, query string, want int) *queryFlight {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		idx.queryMu.Lock()
		flight := idx.queryFlights[query]
		waiters := 0
		if flight != nil {
			waiters = flight.waiters
		}
		idx.queryMu.Unlock()
		if waiters == want {
			return flight
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("query %q did not reach %d waiters", query, want)
	return nil
}

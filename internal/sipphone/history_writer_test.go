package sipphone

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type orderedTestCallStore struct {
	mu             sync.Mutex
	records        map[string]CallRecord
	blockNextWrite chan struct{}
	writeStarted   chan struct{}
	closed         bool
}

func newOrderedTestCallStore() *orderedTestCallStore {
	return &orderedTestCallStore{records: make(map[string]CallRecord)}
}

func (s *orderedTestCallStore) Upsert(ctx context.Context, record CallRecord) error {
	s.mu.Lock()
	block := s.blockNextWrite
	started := s.writeStarted
	s.blockNextWrite = nil
	s.writeStarted = nil
	s.mu.Unlock()
	if started != nil {
		close(started)
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.mu.Lock()
	s.records[record.ID] = record
	s.mu.Unlock()
	return nil
}

func (s *orderedTestCallStore) List(context.Context, int) ([]CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]CallRecord, 0, len(s.records))
	for _, record := range s.records {
		result = append(result, record)
	}
	return result, nil
}

func (s *orderedTestCallStore) DeleteOlderThan(_ context.Context, cutoff time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, record := range s.records {
		if record.StartedAt.Before(cutoff) {
			delete(s.records, id)
		}
	}
	return nil
}

func (s *orderedTestCallStore) DeleteAll(context.Context) error {
	s.mu.Lock()
	s.records = make(map[string]CallRecord)
	s.mu.Unlock()
	return nil
}

func (s *orderedTestCallStore) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *orderedTestCallStore) blockWrite() (<-chan struct{}, chan<- struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	started := make(chan struct{})
	release := make(chan struct{})
	s.writeStarted = started
	s.blockNextWrite = release
	return started, release
}

func newHistoryTestManager(store callStore) *Manager {
	return &Manager{store: store}
}

func waitForHistoryControl(t *testing.T, manager *Manager) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		manager.persistMu.Lock()
		controls := manager.persistControls
		manager.persistMu.Unlock()
		if controls > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("history control was not enqueued")
}

func TestHistoryDeleteIsOrderedAfterInflightUpsert(t *testing.T) {
	store := newOrderedTestCallStore()
	manager := newHistoryTestManager(store)
	started, release := store.blockWrite()
	manager.persistCall(CallRecord{ID: "call-1", State: "ended", StartedAt: time.Now().UTC()}, "finished")
	<-started

	deleted := make(chan error, 1)
	go func() { deleted <- manager.DeleteHistory(context.Background()) }()
	select {
	case err := <-deleted:
		t.Fatalf("delete passed an in-flight upsert: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	calls, err := manager.ListCalls(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("deleted call was resurrected: %+v", calls)
	}
	if err := manager.closePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryBarrierPreventsCrossBarrierCoalescing(t *testing.T) {
	store := newOrderedTestCallStore()
	manager := newHistoryTestManager(store)
	started, release := store.blockWrite()
	now := time.Now().UTC()
	manager.persistCall(CallRecord{ID: "call-1", State: "ringing", StartedAt: now}, "ringing")
	<-started

	listed := make(chan []CallRecord, 1)
	go func() {
		calls, _ := manager.ListCalls(context.Background(), 10)
		listed <- calls
	}()
	waitForHistoryControl(t, manager)
	manager.persistCall(CallRecord{ID: "call-1", State: "ended", StartedAt: now}, "finished")
	close(release)

	first := <-listed
	if len(first) != 1 || first[0].State != "ringing" {
		t.Fatalf("barrier observed wrong state: %+v", first)
	}
	latest, err := manager.ListCalls(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 1 || latest[0].State != "ended" {
		t.Fatalf("latest state was not persisted: %+v", latest)
	}
	if err := manager.closePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryPruneIsOrderedAfterInflightUpsert(t *testing.T) {
	store := newOrderedTestCallStore()
	manager := newHistoryTestManager(store)
	started, release := store.blockWrite()
	manager.persistCall(CallRecord{ID: "old-call", State: "ended", StartedAt: time.Now().Add(-time.Hour)}, "finished")
	<-started

	pruned := make(chan error, 1)
	go func() { pruned <- manager.PruneHistory(context.Background(), time.Now()) }()
	waitForHistoryControl(t, manager)
	close(release)
	if err := <-pruned; err != nil {
		t.Fatal(err)
	}
	calls, err := manager.ListCalls(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("pruned call was resurrected: %+v", calls)
	}
	if err := manager.closePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryUpsertQueueIsBounded(t *testing.T) {
	store := newOrderedTestCallStore()
	manager := newHistoryTestManager(store)
	started, release := store.blockWrite()
	manager.persistCall(CallRecord{ID: "blocked", StartedAt: time.Now()}, "created")
	<-started
	for index := 0; index < 256; index++ {
		manager.persistCall(CallRecord{ID: fmt.Sprintf("call-%d", index), StartedAt: time.Now()}, "created")
	}
	manager.persistMu.Lock()
	queued := manager.persistUpserts
	manager.persistMu.Unlock()
	if queued != 128 {
		t.Fatalf("queued upserts = %d, want 128", queued)
	}
	close(release)
	if err := manager.closePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryCloseFlushesAcceptedWrites(t *testing.T) {
	store := newOrderedTestCallStore()
	manager := newHistoryTestManager(store)
	manager.persistCall(CallRecord{ID: "call-1", State: "ended", StartedAt: time.Now().UTC()}, "finished")
	if err := manager.closePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.records["call-1"]; !ok || !store.closed {
		t.Fatalf("close did not flush and close store: records=%+v closed=%v", store.records, store.closed)
	}
}

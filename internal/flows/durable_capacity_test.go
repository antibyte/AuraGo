package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type concurrentCheckpointKeys struct {
	mu     sync.Mutex
	create []bool
}

func (k *concurrentCheckpointKeys) CheckpointKey(_ context.Context, create bool) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.create = append(k.create, create)
	return bytes.Repeat([]byte{7}, 32), nil
}

func TestDurableAdmissionRejectsFullCheckpointCapacity(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now().UTC()
	occupier := sampleFlow("flow_capacity_existing")
	if _, err := store.CreateFlow(ctx, occupier, "mission_capacity", now); err != nil {
		t.Fatal(err)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxDurableCheckpoints; i++ {
		id := fmt.Sprintf("run_capacity_%d", i)
		if _, err := tx.Exec(`INSERT INTO flow_runs(id,flow_id,revision,mode,status,started_at) VALUES(?,?,1,'live','waiting',?)`, id, occupier.ID, formatTime(now)); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at) VALUES(?,1,'waiting',?,?,?,?)`, id, formatTime(now.Add(time.Minute)), formatTime(now.Add(time.Hour)), []byte("legacy"), formatTime(now)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	req := durableReviewPublished(t, store, "flow_capacity_target", 60)
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
	r := NewRunner(NewEngine(newTestRegistry(t), services, discardLogger(), 1), store, RunnerHooks{}, RunnerConfig{DurableEnabled: true, DurableAuthority: durableTestAuthority{}, DurableHistory: &durableTestProjection{}}, discardLogger())
	t.Cleanup(func() { _ = r.Shutdown(ctx) })
	r.mu.Lock()
	r.durableStarted = true
	r.mu.Unlock()
	if _, err := r.StartContext(ctx, req); !errors.Is(err, ErrDurableCapacity) {
		t.Fatalf("capacity admission: %v", err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_runs`).Scan(&count); err != nil || count != maxDurableCheckpoints {
		t.Fatalf("runs=%d err=%v", count, err)
	}
}

func TestDurableOrphanCheckpointFailsClosed(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	store.db.SetMaxOpenConns(1)
	doc := sampleFlow("flow_orphan_checkpoint")
	if _, err := store.CreateFlow(ctx, doc, "mission_orphan", time.Now()); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_orphan_checkpoint", FlowID: doc.ID, Mode: ModeLive, Status: RunRunning, StartedAt: time.Now()}
	if err := store.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cp := durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: now.Add(time.Minute), Deadline: now.Add(time.Hour), Ciphertext: []byte("legacy")}
	if err := saveDurableCheckpoint(ctx, store.db, cp, 0, "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`PRAGMA foreign_keys=OFF; DELETE FROM flows WHERE id='flow_orphan_checkpoint'; PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.durableReservations(ctx); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("orphan accepted: %v", err)
	}
}

func TestDurableConcurrentFirstKeyAdmissionIsSerialized(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store := openTestStore(t)
	doc := sampleFlow("flow_first_key_race")
	now := time.Now().UTC()
	if _, err := store.CreateFlow(ctx, doc, "mission_first_key", now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"run_key_one", "run_key_two"} {
		if err := store.CreateRun(ctx, RunRecord{ID: id, FlowID: doc.ID, Mode: ModeLive, Status: RunRunning, StartedAt: now}, nil); err != nil {
			t.Fatal(err)
		}
	}
	keys := &concurrentCheckpointKeys{}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []string{"run_key_one", "run_key_two"} {
		go func(id string) {
			<-start
			results <- withCheckpointKey(ctx, store.db, keys, func(tx *sql.Tx, key []byte) error {
				sealed, err := sealCheckpoint(key, id, 1, []byte("sample"))
				if err != nil {
					return err
				}
				return saveDurableCheckpointTx(ctx, tx, durableCheckpointRecord{RunID: id, Generation: 1, WakeAt: now.Add(time.Minute), Deadline: now.Add(time.Hour), Ciphertext: sealed}, 0, "", now)
			})
		}(id)
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	keys.mu.Lock()
	defer keys.mu.Unlock()
	if len(keys.create) != 2 || !keys.create[0] || keys.create[1] {
		t.Fatalf("key creation proofs=%v", keys.create)
	}
}

func TestCheckpointByteCapacityBoundary(t *testing.T) {
	for _, tc := range []struct {
		used int64
		size int
		want bool
	}{
		{0, maxCheckpointBytes, true},
		{maxDurableBytes - 1, 1, true},
		{maxDurableBytes, 0, true},
		{maxDurableBytes, 1, false},
		{maxDurableBytes - 1, 2, false},
		{-1, 1, false},
		{maxDurableBytes + 1, 0, false},
		{0, -1, false},
	} {
		if got := checkpointBytesFit(tc.used, tc.size); got != tc.want {
			t.Fatalf("used=%d size=%d: got %v want %v", tc.used, tc.size, got, tc.want)
		}
	}
}

func TestDurableConcurrentSchedulersCannotClaimSameGeneration(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	doc, err := req.Flow.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	guard := durableClaimGuard{flowID: req.Flow.ID, missionID: req.missionID, revision: req.Revision, document: doc}
	type result struct {
		ok  bool
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	now := s.now()
	for _, owner := range []string{"scheduler_a", "scheduler_b"} {
		go func(owner string) {
			<-start
			ok, err := claimDurableCheckpoint(ctx, s.store.db, runID, 1, owner, now, now.Add(durableLease), guard)
			results <- result{ok, err}
		}(owner)
	}
	close(start)
	success := 0
	for i := 0; i < 2; i++ {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.ok {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("claimed %d times", success)
	}
}

package flows

import (
	"context"
	"testing"
	"time"
)

func TestDurableLegacyTerminalCheckpointSurvivesRunRetention(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now().UTC()
	doc := sampleFlow("flow_legacy_retention")
	if _, err := store.CreateFlow(ctx, doc, "mission_legacy", now); err != nil {
		t.Fatal(err)
	}
	old := RunRecord{ID: "run_legacy_retention", FlowID: doc.ID, Mode: ModeLive, Status: RunRunning, StartedAt: now.AddDate(0, 0, -30)}
	if err := store.CreateRun(ctx, old, nil); err != nil {
		t.Fatal(err)
	}
	rec := durableCheckpointRecord{RunID: old.ID, Generation: 1, WakeAt: now.Add(time.Minute), Deadline: now.Add(time.Hour), Ciphertext: []byte("legacy bytes")}
	if err := saveDurableCheckpoint(ctx, store.db, rec, 0, "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE flow_runs SET status='cancelled' WHERE id=?`, old.ID); err != nil {
		t.Fatal(err)
	}
	newer := RunRecord{ID: "run_newer_retention", FlowID: doc.ID, Mode: ModeLive, Status: RunSuccess, StartedAt: now}
	if err := store.CreateRun(ctx, newer, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PruneRunsByMode(ctx, 1, 1, 1, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, old.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy checkpoint=%d err=%v", count, err)
	}
	s := NewService(store, catalogRegistry(t, fullEnv()), &Services{}, newFakeBridge(), ServiceConfig{}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	if ok, err := s.CancelDurableRun(ctx, old.ID, 1); !ok || err != nil {
		t.Fatalf("legacy cancel %v %v", ok, err)
	}
	if _, err := store.PruneRunsByMode(ctx, 1, 1, 1, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetRunHeader(ctx, old.ID); err != ErrRunNotFound {
		t.Fatalf("cancelled legacy row retained: %v", err)
	}
}

package systemworld

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "world.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := NewStore(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	return s, db
}
func TestHistoryRetentionReplayGapsAndRestart(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	start := time.Now().Truncate(time.Minute).Add(-25 * time.Hour).UnixMilli()
	for minute := 0; minute <= 1500; minute++ {
		at := start + int64(minute)*60000
		e := Entity{ID: "agent", Kind: "district", District: "agent", State: "idle", At: at, Actions: []string{"start"}}
		if minute%7 == 0 {
			e.State = "running"
		}
		snap := Snapshot{At: at, Metrics: map[string]float64{"cpu": float64(minute % 100), "uptime": float64(minute * 60)}, Entities: []Entity{e}}
		if err := s.Save(ctx, snap, []Entity{e}); err != nil {
			t.Fatal(err)
		}
	}
	cutoff := start + 60*60000
	history, err := s.History(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2882 || history[0].At < cutoff {
		t.Fatalf("retention: %d samples, first %d", len(history), history[0].At)
	}
	reopened, err := NewStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := reopened.At(ctx, start+1499*60000)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Entities) != 1 || len(snap.Entities[0].Actions) != 0 || snap.Metrics["cpu"] != 99 {
		t.Fatalf("invalid replay: %+v", snap)
	}
	if _, err = reopened.At(ctx, start+1520*60000); err != sql.ErrNoRows {
		t.Fatalf("downtime fabricated: %v", err)
	}
	if _, err = reopened.At(ctx, start); err != sql.ErrNoRows {
		t.Fatalf("expired replay: %v", err)
	}
}

func TestHistoryUnchangedInventoryRetainsObservedFreshness(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	at := time.Now().Truncate(time.Minute).Add(-10 * time.Minute).UnixMilli()
	e := Entity{ID: "container:a", Kind: "container", Source: "system-world/container", State: "running", At: at}
	for minute := 0; minute < 5; minute++ {
		now := at + int64(minute)*60000
		observed := now
		if minute == 4 {
			observed = at + 3*60000 // Failed poll does not refresh the source.
		}
		if err := s.Save(ctx, Snapshot{At: now, Entities: []Entity{e}, Metrics: map[string]float64{"cpu": 0, "observed:" + e.Source: float64(observed)}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, minute := range []int64{3, 4} {
		snap, err := s.At(ctx, at+minute*60000)
		if err != nil || snap.Entities[0].At != at+3*60000 {
			t.Fatalf("freshness lost or failed poll invented: %+v, %v", snap, err)
		}
		if _, ok := snap.Metrics["observed:"+e.Source]; ok {
			t.Fatal("internal receipt displayed as a metric")
		}
	}
}
func TestHistoryAggregatesStateRemovalAndStormBound(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	at := time.Now().Truncate(time.Minute).UnixMilli()
	first := Snapshot{At: at, Metrics: map[string]float64{"cpu": 0}, Entities: []Entity{{ID: "container:a", State: "running", At: at}}}
	if err := s.Save(ctx, first, nil); err != nil {
		t.Fatal(err)
	}
	first.At += 10000
	first.Metrics["cpu"] = 50
	if err := s.Save(ctx, first, []Entity{{ID: "container:a", State: "removed", At: first.At}}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.At(ctx, first.At)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Entities) != 0 {
		t.Fatal("removed entity reappeared")
	}
	hist, err := s.History(ctx, at)
	if err != nil {
		t.Fatal(err)
	}
	if hist[0].Count != 2 || hist[0].Average != 25 || hist[0].Min != 0 || hist[0].Max != 50 {
		t.Fatal(hist)
	}
	for i := 0; i < 52; i++ {
		first.At = at + int64(i+1)*10000
		changes := make([]Entity, 1000)
		for j := range changes {
			changes[j] = Entity{ID: fmt.Sprintf("node:%d", j), State: "running", At: first.At}
		}
		if err = s.Save(ctx, first, changes); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err = db.QueryRow("SELECT COUNT(*) FROM system_world_events").Scan(&n); err != nil || n > 50000 {
		t.Fatalf("unbounded storm: %d %v", n, err)
	}
	if _, err = s.At(ctx, at); err != sql.ErrNoRows {
		t.Fatal("truncated deltas must expose a replay gap")
	}
	events, err := s.Events(ctx, at, 0, 999)
	if err != nil || len(events) > 200 {
		t.Fatalf("event page: %d %v", len(events), err)
	}
}

func TestHistoryDroppedProducerDeltasInvalidateOnlyRecentReplay(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	at := time.Now().Truncate(time.Minute).UnixMilli()
	snap := Snapshot{At: at, Metrics: map[string]float64{"cpu": 12}}
	for _, offset := range []int64{0, 300000} {
		snap.At = at + offset
		if err := s.Save(ctx, snap, nil); err != nil {
			t.Fatal(err)
		}
	}
	snap.At = at + 360000
	snap.IncompleteBefore = true
	snap.Entities = []Entity{{ID: "agent", State: "running", At: snap.At}}
	if err := s.Save(ctx, snap, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.At(ctx, at); err != nil {
		t.Fatal("unaffected history lost", err)
	}
	if _, err := s.At(ctx, at+330000); err != sql.ErrNoRows {
		t.Fatal("dropped deltas must be a gap", err)
	}
	if v, err := s.At(ctx, snap.At); err != nil || len(v.Entities) != 1 {
		t.Fatal("fresh checkpoint unavailable", err)
	}
}

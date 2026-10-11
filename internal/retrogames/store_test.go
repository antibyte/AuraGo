package retrogames

import (
	"context"
	"testing"
	"time"
)

func TestReplaceSourceSwapsRowsAndRecordsState(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{testGame("a", "Alpha", PlatformMSDOS, 1990, 10), testGame("b", "Beta", PlatformMSDOS, 1991, 20)}, testNow); err != nil {
		t.Fatalf("ReplaceSource: %v", err)
	}
	if err := st.ReplaceSource(ctx, PlatformArcade, []Game{testGame("z", "Zeta", PlatformArcade, 1982, 5)}, testNow); err != nil {
		t.Fatalf("ReplaceSource arcade: %v", err)
	}
	later := testNow.Add(time.Hour)
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{testGame("b", "Beta Deluxe", PlatformMSDOS, 1991, 30), testGame("c", "Gamma", PlatformMSDOS, 1992, 1)}, later); err != nil {
		t.Fatalf("ReplaceSource again: %v", err)
	}
	if _, ok, _ := st.Game(ctx, "a"); ok {
		t.Fatal("game a should be gone")
	}
	b, ok, err := st.Game(ctx, "b")
	if err != nil || !ok || b.Title != "Beta Deluxe" || b.Downloads != 30 {
		t.Fatalf("game b = %+v ok=%v err=%v", b, ok, err)
	}
	if countGames(t, st, PlatformArcade) != 1 {
		t.Fatal("arcade rows must stay untouched")
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		t.Fatalf("SyncStates: %v", err)
	}
	if s := states[PlatformMSDOS]; s.ItemCount != 2 || !s.LastSuccessAt.Equal(later) || s.LastError != "" {
		t.Fatalf("msdos state = %+v", s)
	}
}

func TestReplaceSourceKeepsFTSInSync(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	matches := func() int {
		var n int
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM games_fts WHERE games_fts MATCH '"persia"*'`).Scan(&n); err != nil {
			t.Fatalf("match: %v", err)
		}
		return n
	}
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{testGame("pop", "Prince of Persia", PlatformMSDOS, 1990, 1)}, testNow); err != nil {
		t.Fatal(err)
	}
	if matches() != 1 {
		t.Fatal("FTS row missing after insert")
	}
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{testGame("other", "Other", PlatformMSDOS, 1990, 1)}, testNow); err != nil {
		t.Fatal(err)
	}
	if matches() != 0 {
		t.Fatal("FTS row survived delete")
	}
}

func TestRecordSyncFailureKeepsLastSuccess(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.ReplaceSource(ctx, PlatformZX, []Game{testGame("z1", "Jetpac", PlatformZX, 1983, 1)}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordSyncFailure(ctx, PlatformZX, testNow.Add(time.Hour), "boom"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordSyncFailure(ctx, PlatformC64, testNow, "never worked"); err != nil {
		t.Fatal(err)
	}
	states, _ := st.SyncStates(ctx)
	if s := states[PlatformZX]; !s.LastSuccessAt.Equal(testNow) || s.LastError != "boom" || s.ItemCount != 1 {
		t.Fatalf("zx state = %+v", s)
	}
	if s := states[PlatformC64]; !s.LastSuccessAt.IsZero() || s.LastError != "never worked" {
		t.Fatalf("c64 state = %+v", s)
	}
}

func TestNeedsSync(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	keys := []string{PlatformMSDOS, PlatformZX}
	if need, err := st.NeedsSync(ctx, testNow, 7*24*time.Hour, keys); err != nil || !need {
		t.Fatalf("empty index: need=%v err=%v", need, err)
	}
	for _, k := range keys {
		if err := st.ReplaceSource(ctx, k, []Game{testGame(k+"_1", "X", k, 1990, 1)}, testNow); err != nil {
			t.Fatal(err)
		}
	}
	if need, _ := st.NeedsSync(ctx, testNow.Add(24*time.Hour), 7*24*time.Hour, keys); need {
		t.Fatal("fresh index should not need a sync")
	}
	if need, _ := st.NeedsSync(ctx, testNow.Add(8*24*time.Hour), 7*24*time.Hour, keys); !need {
		t.Fatal("stale index should need a sync")
	}
}

func TestOpenStoreReopenKeepsData(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceSource(context.Background(), PlatformWin3, []Game{testGame("w", "JezzBall", PlatformWin3, 1992, 1)}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = OpenStore(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, ok, _ := st.Game(context.Background(), "w"); !ok {
		t.Fatal("data lost after reopen")
	}
}

func TestOpenStoreRejectsEmptyDir(t *testing.T) {
	if _, err := OpenStore("  ", discardLogger()); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

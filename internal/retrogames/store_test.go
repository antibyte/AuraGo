package retrogames

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
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
	if _, ok, err := st.Game(ctx, "a"); err != nil || ok {
		t.Fatalf("game a should be gone: ok=%v err=%v", ok, err)
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
	states, err := st.SyncStates(ctx)
	if err != nil {
		t.Fatalf("SyncStates: %v", err)
	}
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
	if need, err := st.NeedsSync(ctx, testNow.Add(24*time.Hour), 7*24*time.Hour, keys); err != nil || need {
		t.Fatalf("fresh index should not need a sync: need=%v err=%v", need, err)
	}
	if need, err := st.NeedsSync(ctx, testNow.Add(8*24*time.Hour), 7*24*time.Hour, keys); err != nil || !need {
		t.Fatalf("stale index should need a sync: need=%v err=%v", need, err)
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
	if _, ok, err := st.Game(context.Background(), "w"); err != nil || !ok {
		t.Fatalf("data lost after reopen: ok=%v err=%v", ok, err)
	}
}

func TestOpenStoreRejectsEmptyDir(t *testing.T) {
	if _, err := OpenStore("  ", discardLogger()); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestReplaceSourceKeepsFirstOwnerAndCountsStoredRows(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	mustReplace := func(source string, games ...Game) {
		t.Helper()
		if err := st.ReplaceSource(ctx, source, games, testNow); err != nil {
			t.Fatalf("ReplaceSource %s: %v", source, err)
		}
	}
	itemCount := func(source string) int {
		t.Helper()
		states, err := st.SyncStates(ctx)
		if err != nil {
			t.Fatalf("SyncStates: %v", err)
		}
		return states[source].ItemCount
	}
	mustReplace(PlatformMSDOS, testGame("x", "X", PlatformMSDOS, 1990, 1), testGame("y", "Y", PlatformMSDOS, 1990, 1))
	mustReplace(PlatformArcade, testGame("x", "X", PlatformArcade, 1982, 1), testGame("z", "Z", PlatformArcade, 1982, 1))
	x, ok, err := st.Game(ctx, "x")
	if err != nil || !ok || x.Platform != PlatformMSDOS {
		t.Fatalf("x = %+v ok=%v err=%v, want it to stay with msdos", x, ok, err)
	}
	var source string
	if err := st.db.QueryRow(`SELECT source FROM games WHERE identifier = 'x'`).Scan(&source); err != nil || source != PlatformMSDOS {
		t.Fatalf("x source = %q err=%v", source, err)
	}
	if got := itemCount(PlatformArcade); got != 1 {
		t.Fatalf("arcade item_count = %d, want 1", got)
	}
	if got := itemCount(PlatformMSDOS); got != 2 {
		t.Fatalf("msdos item_count = %d, want 2", got)
	}

	mustReplace(PlatformMSDOS, testGame("y", "Y", PlatformMSDOS, 1990, 1))
	if _, ok, err := st.Game(ctx, "x"); err != nil || ok {
		t.Fatalf("x should be gone after msdos dropped it: ok=%v err=%v", ok, err)
	}
	mustReplace(PlatformArcade, testGame("x", "X", PlatformArcade, 1982, 1), testGame("z", "Z", PlatformArcade, 1982, 1))
	x, ok, err = st.Game(ctx, "x")
	if err != nil || !ok || x.Platform != PlatformArcade {
		t.Fatalf("x = %+v ok=%v err=%v, want it owned by arcade now", x, ok, err)
	}
	if got := itemCount(PlatformArcade); got != 2 {
		t.Fatalf("arcade item_count = %d, want 2", got)
	}
}

func TestReplaceSourceRejectsBadInputAndKeepsRows(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{testGame("a", "Alpha", PlatformMSDOS, 1990, 1)}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceSource(ctx, PlatformMSDOS, nil, testNow.Add(time.Hour)); err == nil {
		t.Fatal("empty replace must be refused")
	}
	if err := st.ReplaceSource(ctx, "nope", []Game{testGame("n", "Nope", PlatformMSDOS, 1990, 1)}, testNow); err == nil {
		t.Fatal("unknown source must be refused")
	}
	if countGames(t, st, PlatformMSDOS) != 1 {
		t.Fatal("rejected replaces must leave rows untouched")
	}
	if _, ok, err := st.Game(ctx, "n"); err != nil || ok {
		t.Fatalf("game n must not exist: ok=%v err=%v", ok, err)
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s := states[PlatformMSDOS]; s.ItemCount != 1 || !s.LastSuccessAt.Equal(testNow) {
		t.Fatalf("msdos state = %+v", s)
	}
	if _, ok := states["nope"]; ok {
		t.Fatal("unknown source must not get a sync state")
	}
}

func TestOpenStoreMovesCorruptDatabaseAside(t *testing.T) {
	dir := t.TempDir()
	garbage := bytes.Repeat([]byte("this is not an sqlite database. "), 4096/32)
	if err := os.WriteFile(filepath.Join(dir, dbFileName), garbage, 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := OpenStore(dir, discardLogger())
	if err != nil {
		t.Fatalf("OpenStore on corrupt file: %v", err)
	}
	defer st.Close()
	moved, err := filepath.Glob(filepath.Join(dir, dbFileName+".corrupt-*"))
	if err != nil || len(moved) != 1 {
		t.Fatalf("corrupt copies = %v err=%v", moved, err)
	}
	if err := st.ReplaceSource(context.Background(), PlatformMSDOS, []Game{testGame("a", "Alpha", PlatformMSDOS, 1990, 1)}, testNow); err != nil {
		t.Fatalf("fresh store unusable: %v", err)
	}
	if _, ok, err := st.Game(context.Background(), "a"); err != nil || !ok {
		t.Fatalf("game a: ok=%v err=%v", ok, err)
	}
}

func TestOpenStoreRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = OpenStore(dir, discardLogger())
	if err == nil {
		_ = st.Close()
		t.Fatal("expected an error for a newer schema")
	}
	if !strings.Contains(err.Error(), "newer AuraGo") {
		t.Fatalf("error = %v", err)
	}
	if moved, _ := filepath.Glob(filepath.Join(dir, dbFileName+".corrupt-*")); len(moved) != 0 {
		t.Fatalf("a newer schema must never move the file aside: %v", moved)
	}
}

func TestFTSFollowsTitleUpdates(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{testGame("pop", "Prince of Persia", PlatformMSDOS, 1990, 1)}, testNow); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE games SET title = 'Something Else', tags = 'Something Else' WHERE identifier = 'pop'`); err != nil {
		t.Fatal(err)
	}
	count := func(term string) int {
		t.Helper()
		var n int
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM games_fts WHERE games_fts MATCH ?`, `"`+term+`"*`).Scan(&n); err != nil {
			t.Fatalf("match %s: %v", term, err)
		}
		return n
	}
	if got := count("persia"); got != 0 {
		t.Fatalf("old title still matches: %d", got)
	}
	if got := count("something"); got != 1 {
		t.Fatalf("new title matches = %d, want 1", got)
	}
}

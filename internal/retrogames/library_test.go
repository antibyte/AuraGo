package retrogames

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func mustLibrary(t *testing.T, st *Store) Library {
	t.Helper()
	lib, err := st.Library(context.Background())
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	return lib
}

func countRows(t *testing.T, st *Store, table string) int {
	t.Helper()
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestFavoritesToggleOrderAndSnapshot(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	a := testGame("a", "Alpha", PlatformMSDOS, 1990, 1)
	b := testGame("b", "Beta", PlatformMSDOS, 1991, 1)
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{a, b}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFavorite(ctx, a, true, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFavorite(ctx, b, true, testNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	lib := mustLibrary(t, st)
	if len(lib.Favorites) != 2 || lib.Favorites[0].Identifier != "b" || !lib.Favorites[0].Available || lib.Favorites[0].At != testNow.Add(time.Minute).Unix() {
		t.Fatalf("favorites = %+v", lib.Favorites)
	}
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{b}, testNow); err != nil {
		t.Fatal(err)
	}
	lib = mustLibrary(t, st)
	if len(lib.Favorites) != 2 {
		t.Fatalf("favorites after source change = %+v", lib.Favorites)
	}
	if lib.Favorites[1].Identifier != "a" || lib.Favorites[1].Title != "Alpha" || lib.Favorites[1].Available {
		t.Fatalf("snapshot of removed game = %+v", lib.Favorites[1])
	}
	if err := st.SetFavorite(ctx, Game{Identifier: "a"}, false, testNow); err != nil {
		t.Fatal(err)
	}
	lib = mustLibrary(t, st)
	if len(lib.Favorites) != 1 || lib.Favorites[0].Identifier != "b" {
		t.Fatalf("favorite not removed: %+v", lib.Favorites)
	}
}

func TestRefavoriteKeepsAddedAtAndRefreshesTitle(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.SetFavorite(ctx, testGame("a", "Alpha", PlatformMSDOS, 1990, 1), true, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFavorite(ctx, testGame("a", "Alpha Deluxe", PlatformMSDOS, 1990, 1), true, testNow.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	lib := mustLibrary(t, st)
	if len(lib.Favorites) != 1 {
		t.Fatalf("favorites = %+v", lib.Favorites)
	}
	if got := lib.Favorites[0]; got.Title != "Alpha Deluxe" || got.At != testNow.Unix() {
		t.Fatalf("after re-favorite: title=%q at=%d, want %q at=%d", got.Title, got.At, "Alpha Deluxe", testNow.Unix())
	}
	if err := st.SetFavorite(ctx, Game{Identifier: "a", Platform: PlatformMSDOS, System: "DOS"}, true, testNow.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	lib = mustLibrary(t, st)
	if len(lib.Favorites) != 1 || lib.Favorites[0].Title != "Alpha Deluxe" || lib.Favorites[0].At != testNow.Unix() {
		t.Fatalf("an empty title must not overwrite the snapshot: %+v", lib.Favorites)
	}
}

func TestRecordPlayCountsAndTrims(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	g := testGame("g", "Game", PlatformZX, 1984, 1)
	for i := 0; i < 3; i++ {
		if err := st.RecordPlay(ctx, g, testNow.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	lib := mustLibrary(t, st)
	if len(lib.Recent) != 1 || lib.Recent[0].PlayCount != 3 || lib.Recent[0].At != testNow.Add(2*time.Minute).Unix() {
		t.Fatalf("recent = %+v", lib.Recent)
	}
	for i := 0; i < maxStoredPlays+5; i++ {
		other := testGame(fmt.Sprintf("p%03d", i), "P", PlatformZX, 1984, 1)
		if err := st.RecordPlay(ctx, other, testNow.Add(time.Hour+time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if n := countRows(t, st, "plays"); n != maxStoredPlays {
		t.Fatalf("stored plays = %d", n)
	}
	lib = mustLibrary(t, st)
	if len(lib.Recent) != maxRecent || lib.Recent[0].Identifier != fmt.Sprintf("p%03d", maxStoredPlays+4) {
		first := ""
		if len(lib.Recent) > 0 {
			first = lib.Recent[0].Identifier
		}
		t.Fatalf("recent len=%d first=%s", len(lib.Recent), first)
	}
}

func TestRecordPlayKeepsTitleWhenEmpty(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.RecordPlay(ctx, testGame("g", "Game", PlatformZX, 1984, 1), testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordPlay(ctx, Game{Identifier: "g", Platform: PlatformZX, System: "ZX Spectrum"}, testNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	lib := mustLibrary(t, st)
	if len(lib.Recent) != 1 || lib.Recent[0].Title != "Game" || lib.Recent[0].PlayCount != 2 {
		t.Fatalf("recent = %+v", lib.Recent)
	}
}

func TestBrokenHidesFromSearch(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	g := testGame("x", "Xenon", PlatformAmiga, 1988, 1)
	if err := st.ReplaceSource(ctx, PlatformAmiga, []Game{g}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBroken(ctx, g, true, testNow); err != nil {
		t.Fatal(err)
	}
	res, err := st.Search(ctx, Query{Text: "xenon"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Fatal("broken game still listed")
	}
	res, err = st.Search(ctx, Query{Text: "xenon", IncludeBroken: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatal("IncludeBroken should list it")
	}
	lib := mustLibrary(t, st)
	if len(lib.Broken) != 1 || lib.Broken[0].Identifier != "x" || !lib.Broken[0].Available {
		t.Fatalf("broken = %+v", lib.Broken)
	}
	if err := st.SetBroken(ctx, Game{Identifier: "x"}, false, testNow); err != nil {
		t.Fatal(err)
	}
	if lib = mustLibrary(t, st); len(lib.Broken) != 0 {
		t.Fatalf("broken marker not removed: %+v", lib.Broken)
	}
	res, err = st.Search(ctx, Query{Text: "xenon"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatal("unmarked game should be listed again")
	}
}

func TestFavoritesCap(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < maxFavorites; i++ {
		if err := st.SetFavorite(ctx, testGame(fmt.Sprintf("f%03d", i), "F", PlatformMSDOS, 1990, 1), true, testNow); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetFavorite(ctx, testGame("f000", "F again", PlatformMSDOS, 1990, 1), true, testNow); err != nil {
		t.Fatalf("re-adding an existing favorite must work: %v", err)
	}
	if err := st.SetFavorite(ctx, testGame("over", "Over", PlatformMSDOS, 1990, 1), true, testNow); !errors.Is(err, ErrLibraryFull) {
		t.Fatalf("err = %v, want ErrLibraryFull", err)
	}
	if n := countRows(t, st, "favorites"); n != maxFavorites {
		t.Fatalf("favorites = %d, want %d", n, maxFavorites)
	}
	if err := st.SetFavorite(ctx, Game{Identifier: "f001"}, false, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFavorite(ctx, testGame("over", "Over", PlatformMSDOS, 1990, 1), true, testNow); err != nil {
		t.Fatalf("a freed slot must accept a new favorite: %v", err)
	}
}

func TestBrokenCap(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < maxBroken; i++ {
		if err := st.SetBroken(ctx, testGame(fmt.Sprintf("b%03d", i), "B", PlatformAmiga, 1990, 1), true, testNow); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetBroken(ctx, testGame("b000", "B again", PlatformAmiga, 1990, 1), true, testNow); err != nil {
		t.Fatalf("re-marking an existing game must work: %v", err)
	}
	if err := st.SetBroken(ctx, testGame("over", "Over", PlatformAmiga, 1990, 1), true, testNow); !errors.Is(err, ErrLibraryFull) {
		t.Fatalf("err = %v, want ErrLibraryFull", err)
	}
	if err := st.SetBroken(ctx, Game{Identifier: "b001"}, false, testNow); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBroken(ctx, testGame("over", "Over", PlatformAmiga, 1990, 1), true, testNow); err != nil {
		t.Fatalf("a freed slot must accept a new marker: %v", err)
	}
	if n := countRows(t, st, "broken"); n != maxBroken {
		t.Fatalf("broken = %d, want %d", n, maxBroken)
	}
}

func TestFavoritesCapHoldsUnderConcurrency(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < maxFavorites-1; i++ {
		if err := st.SetFavorite(ctx, testGame(fmt.Sprintf("f%03d", i), "F", PlatformMSDOS, 1990, 1), true, testNow); err != nil {
			t.Fatal(err)
		}
	}
	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = st.SetFavorite(ctx, testGame(fmt.Sprintf("new%d", i), "N", PlatformMSDOS, 1990, 1), true, testNow)
		}(i)
	}
	wg.Wait()
	added, full := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			added++
		case errors.Is(err, ErrLibraryFull):
			full++
		default:
			t.Errorf("worker %d: unexpected error %v", i, err)
		}
	}
	if added != 1 || full != workers-1 {
		t.Fatalf("added=%d full=%d, want 1 and %d", added, full, workers-1)
	}
	if n := countRows(t, st, "favorites"); n != maxFavorites {
		t.Fatalf("favorites = %d, want exactly %d", n, maxFavorites)
	}
}

func TestSnapshotReadsLibraryRows(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.RecordPlay(ctx, testGame("s", "Snap", PlatformC64, 1985, 1), testNow); err != nil {
		t.Fatal(err)
	}
	g, ok, err := st.Snapshot(ctx, "s")
	if err != nil || !ok || g.Title != "Snap" || g.Platform != PlatformC64 || g.Year != 1985 {
		t.Fatalf("snapshot = %+v ok=%v err=%v", g, ok, err)
	}
	if _, ok, _ := st.Snapshot(ctx, "missing"); ok {
		t.Fatal("missing snapshot reported")
	}
}

func TestSnapshotPrefersPlaysThenFavoritesThenBroken(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.SetBroken(ctx, testGame("s", "Broken Title", PlatformC64, 1985, 1), true, testNow); err != nil {
		t.Fatal(err)
	}
	g, ok, err := st.Snapshot(ctx, "s")
	if err != nil || !ok || g.Title != "Broken Title" {
		t.Fatalf("broken only: %+v ok=%v err=%v", g, ok, err)
	}
	if err := st.SetFavorite(ctx, testGame("s", "Favorite Title", PlatformC64, 1985, 1), true, testNow); err != nil {
		t.Fatal(err)
	}
	if g, ok, err = st.Snapshot(ctx, "s"); err != nil || !ok || g.Title != "Favorite Title" {
		t.Fatalf("favorite over broken: %+v ok=%v err=%v", g, ok, err)
	}
	if err := st.RecordPlay(ctx, testGame("s", "Play Title", PlatformC64, 1985, 1), testNow); err != nil {
		t.Fatal(err)
	}
	if g, ok, err = st.Snapshot(ctx, "s"); err != nil || !ok || g.Title != "Play Title" {
		t.Fatalf("play over favorite: %+v ok=%v err=%v", g, ok, err)
	}
}

func TestEmptyIdentifierIsRejected(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"", "   "} {
		g := testGame(id, "No ID", PlatformMSDOS, 1990, 1)
		if err := st.SetFavorite(ctx, g, true, testNow); err == nil {
			t.Errorf("SetFavorite(%q) accepted an empty identifier", id)
		}
		if err := st.SetBroken(ctx, g, true, testNow); err == nil {
			t.Errorf("SetBroken(%q) accepted an empty identifier", id)
		}
		if err := st.RecordPlay(ctx, g, testNow); err == nil {
			t.Errorf("RecordPlay(%q) accepted an empty identifier", id)
		}
	}
	for _, table := range []string{"favorites", "broken", "plays"} {
		if n := countRows(t, st, table); n != 0 {
			t.Errorf("%s has %d rows after rejected writes", table, n)
		}
	}
}

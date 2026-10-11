package retrogames

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

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
	lib, err := st.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Favorites) != 2 || lib.Favorites[0].Identifier != "b" || !lib.Favorites[0].Available || lib.Favorites[0].At != testNow.Add(time.Minute).Unix() {
		t.Fatalf("favorites = %+v", lib.Favorites)
	}
	if err := st.ReplaceSource(ctx, PlatformMSDOS, []Game{b}, testNow); err != nil {
		t.Fatal(err)
	}
	lib, _ = st.Library(ctx)
	if lib.Favorites[1].Identifier != "a" || lib.Favorites[1].Title != "Alpha" || lib.Favorites[1].Available {
		t.Fatalf("snapshot of removed game = %+v", lib.Favorites[1])
	}
	if err := st.SetFavorite(ctx, Game{Identifier: "a"}, false, testNow); err != nil {
		t.Fatal(err)
	}
	lib, _ = st.Library(ctx)
	if len(lib.Favorites) != 1 {
		t.Fatalf("favorite not removed: %+v", lib.Favorites)
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
	lib, _ := st.Library(ctx)
	if len(lib.Recent) != 1 || lib.Recent[0].PlayCount != 3 || lib.Recent[0].At != testNow.Add(2*time.Minute).Unix() {
		t.Fatalf("recent = %+v", lib.Recent)
	}
	for i := 0; i < maxStoredPlays+5; i++ {
		other := testGame(fmt.Sprintf("p%03d", i), "P", PlatformZX, 1984, 1)
		if err := st.RecordPlay(ctx, other, testNow.Add(time.Hour+time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM plays`).Scan(&n); err != nil || n != maxStoredPlays {
		t.Fatalf("stored plays = %d err=%v", n, err)
	}
	lib, _ = st.Library(ctx)
	if len(lib.Recent) != maxRecent || lib.Recent[0].Identifier != fmt.Sprintf("p%03d", maxStoredPlays+4) {
		t.Fatalf("recent len=%d first=%s", len(lib.Recent), lib.Recent[0].Identifier)
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
	res, _ := st.Search(ctx, Query{Text: "xenon"})
	if res.Total != 0 {
		t.Fatal("broken game still listed")
	}
	res, _ = st.Search(ctx, Query{Text: "xenon", IncludeBroken: true})
	if res.Total != 1 {
		t.Fatal("IncludeBroken should list it")
	}
	lib, _ := st.Library(ctx)
	if len(lib.Broken) != 1 || lib.Broken[0].Identifier != "x" {
		t.Fatalf("broken = %+v", lib.Broken)
	}
	if err := st.SetBroken(ctx, Game{Identifier: "x"}, false, testNow); err != nil {
		t.Fatal(err)
	}
	res, _ = st.Search(ctx, Query{Text: "xenon"})
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

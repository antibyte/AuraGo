package retrogames

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func seedSearchStore(t *testing.T) *Store {
	t.Helper()
	st := newTestStore(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(st.ReplaceSource(ctx, PlatformMSDOS, []Game{
		testGame("pop1", "Prince of Persia", PlatformMSDOS, 1990, 2000000),
		testGame("pop2", "Prince of Persia 2", PlatformMSDOS, 1993, 70000),
		testGame("little", "Little Prince", PlatformMSDOS, 1995, 10),
		testGame("prince", "Prince", PlatformMSDOS, 1988, 5),
		testGame("wolf", "Wolfenstein 3D", PlatformMSDOS, 1992, 1400000),
		testGame("cafe", "Café Racer", PlatformMSDOS, 0, 50),
	}, testNow))
	must(st.ReplaceSource(ctx, PlatformArcade, []Game{
		testGame("outrun", "Out Run", PlatformArcade, 1986, 37000),
		testGame("qbert", "Q*bert", PlatformArcade, 1982, 400000),
	}, testNow))
	must(st.ReplaceSource(ctx, PlatformConsole, []Game{
		testGame("pitfall2", "Pitfall II - Lost Caverns", PlatformConsole, 1984, 9216),
	}, testNow))
	return st
}

func ids(games []Game) []string {
	out := make([]string, len(games))
	for i, g := range games {
		out[i] = g.Identifier
	}
	return out
}

func TestBuildMatch(t *testing.T) {
	cases := map[string]string{
		"Prince of Persia": `"prince"* "of"* "persia"*`,
		`a"b OR c*`:        `"a"* "b"* "or"* "c"*`,
		"  ":               "",
		"Q*bert":           `"q"* "bert"*`,
		"one two three four five six seven eight nine ten": `"one"* "two"* "three"* "four"* "five"* "six"* "seven"* "eight"*`,
	}
	for in, want := range cases {
		if got := BuildMatch(in); got != want {
			t.Errorf("BuildMatch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSearchRanksExactThenPopular(t *testing.T) {
	st := seedSearchStore(t)
	res, err := st.Search(context.Background(), Query{Text: "prince"})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(ids(res.Games), ",")
	if got != "prince,pop1,pop2,little" || res.Total != 4 {
		t.Fatalf("order = %s total=%d", got, res.Total)
	}
	res, _ = st.Search(context.Background(), Query{Text: "little prince"})
	if len(res.Games) == 0 || res.Games[0].Identifier != "little" {
		t.Fatalf("little prince = %v", ids(res.Games))
	}
}

func TestSearchPrefixDiacriticsAndPunctuation(t *testing.T) {
	st := seedSearchStore(t)
	for query, want := range map[string]string{"wolf": "wolf", "cafe": "cafe", "q bert": "qbert", "Q*bert": "qbert"} {
		res, err := st.Search(context.Background(), Query{Text: query})
		if err != nil || len(res.Games) != 1 || res.Games[0].Identifier != want {
			t.Errorf("search %q = %v err=%v", query, ids(res.Games), err)
		}
	}
}

func TestSearchIgnoresFTSSyntax(t *testing.T) {
	st := seedSearchStore(t)
	for _, q := range []string{`"`, `NEAR(`, `prince OR`, `*`, `-`, `title:prince`, `(a) AND`} {
		if _, err := st.Search(context.Background(), Query{Text: q}); err != nil {
			t.Errorf("search %q: %v", q, err)
		}
	}
}

func TestSearchFilters(t *testing.T) {
	st := seedSearchStore(t)
	ctx := context.Background()
	res, _ := st.Search(ctx, Query{Platform: PlatformArcade})
	if res.Total != 2 || strings.Join(ids(res.Games), ",") != "qbert,outrun" {
		t.Fatalf("arcade = %v total=%d", ids(res.Games), res.Total)
	}
	res, _ = st.Search(ctx, Query{System: "Atari 2600"})
	if res.Total != 1 || res.Games[0].Identifier != "pitfall2" {
		t.Fatalf("system = %v", ids(res.Games))
	}
	res, _ = st.Search(ctx, Query{Decade: 1980, Sort: SortYear})
	if strings.Join(ids(res.Games), ",") != "qbert,pitfall2,outrun,prince" {
		t.Fatalf("1980s = %v", ids(res.Games))
	}
}

func TestSearchSortsAndPages(t *testing.T) {
	st := seedSearchStore(t)
	res, err := st.Search(context.Background(), Query{Platform: PlatformMSDOS, Sort: SortTitle, Limit: 2, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 6 || strings.Join(ids(res.Games), ",") != "little,prince" || res.Limit != 2 || res.Offset != 1 {
		t.Fatalf("page = %v total=%d", ids(res.Games), res.Total)
	}
	res, _ = st.Search(context.Background(), Query{Platform: PlatformMSDOS, Sort: SortYear})
	if res.Games[len(res.Games)-1].Identifier != "cafe" {
		t.Fatalf("unknown year must sort last: %v", ids(res.Games))
	}
}

func TestQueryNormalize(t *testing.T) {
	bad := []Query{
		{Platform: "nes"},
		{Decade: 1985},
		{Decade: 1940},
		{Sort: "random"},
		{Offset: -1},
		{Text: strings.Repeat("x", 201)},
		{System: strings.Repeat("s", 61)},
	}
	for _, q := range bad {
		if _, err := q.Normalize(); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("Normalize(%+v) err = %v", q, err)
		}
	}
	q, err := Query{Limit: 500}.Normalize()
	if err != nil || q.Limit != MaxSearchLimit || q.Sort != SortRelevance {
		t.Fatalf("defaults = %+v err=%v", q, err)
	}
	q, _ = Query{}.Normalize()
	if q.Limit != MaxSearchLimit {
		t.Fatalf("default limit = %d", q.Limit)
	}
}

func TestPlatformAndSystemCounts(t *testing.T) {
	st := seedSearchStore(t)
	counts, err := st.PlatformCounts(context.Background())
	if err != nil || counts[PlatformMSDOS] != 6 || counts[PlatformArcade] != 2 || counts[PlatformConsole] != 1 {
		t.Fatalf("counts = %v err=%v", counts, err)
	}
	systems, err := st.SystemCounts(context.Background(), PlatformConsole)
	if err != nil || len(systems) != 1 || systems[0].Name != "Atari 2600" || systems[0].Count != 1 {
		t.Fatalf("systems = %+v err=%v", systems, err)
	}
}

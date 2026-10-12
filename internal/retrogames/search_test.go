package retrogames

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
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
	// Operators are plain words: "OR" is an ANDed prefix term (no title word
	// starts with "or" here), so it narrows the hits instead of widening them.
	// A real FTS5 OR would return the union and "prince OR wolf" five games.
	for _, q := range []string{"prince OR", "prince or", "prince OR wolf"} {
		res, err := st.Search(context.Background(), Query{Text: q})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if res.Total != 0 || len(res.Games) != 0 {
			t.Errorf("search %q = %v total=%d, want no hits", q, ids(res.Games), res.Total)
		}
	}
	// Column filters are neutralised too: "title:prince" is the two words
	// "title" and "prince", not a filter on the title column.
	res, err := st.Search(context.Background(), Query{Text: "title:prince"})
	if err != nil || res.Total != 0 {
		t.Errorf(`search "title:prince" = %v total=%d err=%v, want no hits`, ids(res.Games), res.Total, err)
	}
}

func TestSearchWithoutSearchableWordFindsNothing(t *testing.T) {
	st := seedSearchStore(t)
	for _, q := range []string{"!!!", `"`, "*", "-", " ** -- "} {
		res, err := st.Search(context.Background(), Query{Text: q, Offset: 3, Limit: 5})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if res.Total != 0 || len(res.Games) != 0 || res.Games == nil || res.Offset != 3 || res.Limit != 5 {
			t.Errorf("search %q = %+v, want an empty page with offset 3 limit 5", q, res)
		}
	}
	// Blank text still browses: it is "no text", not "no searchable word".
	res, err := st.Search(context.Background(), Query{Text: "   "})
	if err != nil || res.Total != 9 {
		t.Errorf("blank text total = %d err=%v, want 9", res.Total, err)
	}
}

func TestSearchHidesBrokenRows(t *testing.T) {
	st := seedSearchStore(t)
	ctx := context.Background()
	total := func(q Query) (int, []string) {
		t.Helper()
		res, err := st.Search(ctx, q)
		if err != nil {
			t.Fatalf("search %+v: %v", q, err)
		}
		return res.Total, ids(res.Games)
	}
	if n, _ := total(Query{}); n != 9 {
		t.Fatalf("baseline total = %d, want 9", n)
	}
	if _, err := st.db.Exec(`INSERT INTO broken(identifier, marked_at, title, platform, system, year) VALUES ('pop1', ?, 'Prince of Persia', 'msdos', 'MS-DOS', 1990)`, testNow.Unix()); err != nil {
		t.Fatal(err)
	}
	if n, got := total(Query{}); n != 8 || contains(got, "pop1") {
		t.Errorf("browse without broken = %d %v, want 8 without pop1", n, got)
	}
	if n, got := total(Query{Text: "prince"}); n != 3 || contains(got, "pop1") {
		t.Errorf("search without broken = %d %v, want 3 without pop1", n, got)
	}
	if n, got := total(Query{IncludeBroken: true}); n != 9 || !contains(got, "pop1") {
		t.Errorf("browse with broken = %d %v, want 9 with pop1", n, got)
	}
	if n, got := total(Query{Text: "prince", IncludeBroken: true}); n != 4 || !contains(got, "pop1") {
		t.Errorf("search with broken = %d %v, want 4 with pop1", n, got)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestSearchCombinedFilters(t *testing.T) {
	st := seedSearchStore(t)
	ctx := context.Background()
	// Prince hits in MS-DOS from the 1990s, by relevance: pop1, pop2, little
	// ("prince" is from 1988 and filtered out by the decade).
	for offset, want := range []string{"pop1", "pop2", "little"} {
		res, err := st.Search(ctx, Query{Text: "prince", Platform: PlatformMSDOS, Decade: 1990, Limit: 1, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		if res.Total != 3 || len(res.Games) != 1 || res.Games[0].Identifier != want || res.Offset != offset || res.Limit != 1 {
			t.Errorf("offset %d = %v total=%d, want %s total=3", offset, ids(res.Games), res.Total, want)
		}
	}
	res, err := st.Search(ctx, Query{Text: "prince", Platform: PlatformMSDOS, Decade: 1990, Limit: 1, Offset: 3})
	if err != nil || res.Total != 3 || len(res.Games) != 0 {
		t.Errorf("past the end = %v total=%d err=%v", ids(res.Games), res.Total, err)
	}
	res, err = st.Search(ctx, Query{Text: "prince", Platform: PlatformArcade})
	if err != nil || res.Total != 0 {
		t.Errorf("prince on arcade = %v total=%d err=%v, want none", ids(res.Games), res.Total, err)
	}
}

func TestSearchStaysFastOnLargeIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("large index seeding is skipped in -short mode")
	}
	st := newTestStore(t)
	ctx := context.Background()
	seed := func(platform, prefix string, n int) {
		games := make([]Game, n)
		for i := range games {
			games[i] = testGame(fmt.Sprintf("%s%05d", prefix, i), fmt.Sprintf("Game %05d", i), platform, 1980+i%20, int64(i))
		}
		if err := st.ReplaceSource(ctx, platform, games, testNow); err != nil {
			t.Fatal(err)
		}
	}
	seed(PlatformMSDOS, "m", 40000)
	seed(PlatformArcade, "a", 30000)
	for _, id := range []string{"m00001", "m00002", "a00003"} {
		if _, err := st.db.Exec(`INSERT INTO broken(identifier, marked_at, title, platform, system, year) VALUES (?, ?, 'Broken', 'msdos', 'MS-DOS', 1980)`, id, testNow.Unix()); err != nil {
			t.Fatal(err)
		}
	}

	const budget = 250 * time.Millisecond
	cases := []struct {
		name  string
		query Query
		total int
	}{
		{"platform by title", Query{Platform: PlatformMSDOS, Sort: SortTitle}, 40000 - 2},
		{"popular", Query{Sort: SortPopular}, 70000 - 3},
		{"text game 1", Query{Text: "game 1"}, -1},
		{"arcade by year offset 1000", Query{Platform: PlatformArcade, Sort: SortYear, Offset: 1000}, 30000 - 1},
	}
	for _, c := range cases {
		start := time.Now()
		res, err := st.Search(ctx, c.query)
		elapsed := time.Since(start)
		t.Logf("%s: %v (total=%d)", c.name, elapsed, res.Total)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if c.total >= 0 && res.Total != c.total {
			t.Errorf("%s: total = %d, want %d", c.name, res.Total, c.total)
		}
		if len(res.Games) == 0 {
			t.Errorf("%s: no games returned", c.name)
		}
		if elapsed > budget {
			t.Errorf("%s took %v, want < %v", c.name, elapsed, budget)
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

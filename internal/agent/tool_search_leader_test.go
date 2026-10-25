package agent

import (
	"context"
	"slices"
	"testing"
)

func TestHoldClearLexicalLeader(t *testing.T) {
	hit := func(name string, score float64, tier int) toolSearchHit {
		return toolSearchHit{Entry: &ToolCatalogEntry{Name: name}, Score: score, Tier: tier}
	}
	lex := func(name string, score float64) toolSearchHit { return hit(name, score, toolSearchTierLexical) }

	for _, tc := range []struct {
		name   string
		fused  []string
		pinned int
		hits   []toolSearchHit
		want   []string
	}{
		{
			name:  "a clear leader displaced by semantic tools returns to the front",
			fused: []string{"sem1", "l2", "sem2", "lead", "l3"},
			hits:  []toolSearchHit{lex("lead", 6), lex("l2", 3), lex("l3", 2)},
			want:  []string{"lead", "sem1", "l2", "sem2", "l3"},
		},
		{
			name:  "a lead of exactly one margin is clear",
			fused: []string{"l2", "lead"},
			hits:  []toolSearchHit{lex("lead", 3), lex("l2", 2)},
			want:  []string{"lead", "l2"},
		},
		{
			name:  "a lead below the margin is a near-tie and stays with the fusion",
			fused: []string{"l2", "lead"},
			hits:  []toolSearchHit{lex("lead", 2.9), lex("l2", 2)},
			want:  []string{"l2", "lead"},
		},
		{
			name:  "the runner-up is the best other hit, not the next in order",
			fused: []string{"l3", "lead", "l2"},
			hits:  []toolSearchHit{lex("lead", 6), lex("l2", 2), lex("l3", 5)},
			want:  []string{"l3", "lead", "l2"},
		},
		{
			name:  "a runner-up of a worse status class still counts",
			fused: []string{"other", "lead"},
			hits:  []toolSearchHit{{Entry: &ToolCatalogEntry{Name: "lead"}, Score: 3, Class: 0}, {Entry: &ToolCatalogEntry{Name: "other"}, Score: 8, Class: 2}},
			want:  []string{"other", "lead"},
		},
		{
			name:  "a leader that is already first changes nothing",
			fused: []string{"lead", "sem1", "l2"},
			hits:  []toolSearchHit{lex("lead", 6), lex("l2", 1.5)},
			want:  []string{"lead", "sem1", "l2"},
		},
		{
			name:   "pinned names stay in front and the guard acts on the first free slot",
			fused:  []string{"p1", "p2", "sem1", "lead", "l2"},
			pinned: 2,
			hits:   []toolSearchHit{hit("p1", 100, toolSearchTierName), hit("p2", 90, toolSearchTierAlias), lex("lead", 6), lex("l2", 3)},
			want:   []string{"p1", "p2", "lead", "sem1", "l2"},
		},
		{
			name:  "a single lexical hit counts as clear",
			fused: []string{"sem1", "sem2", "lead"},
			hits:  []toolSearchHit{lex("lead", 1.5)},
			want:  []string{"lead", "sem1", "sem2"},
		},
		{
			name:  "a leader the fusion dropped is not added",
			fused: []string{"sem1", "l2"},
			hits:  []toolSearchHit{lex("lead", 6), lex("l2", 3)},
			want:  []string{"sem1", "l2"},
		},
		{
			name:  "without lexical hits the fusion stands",
			fused: []string{"p1", "sem1", "sem2"},
			// Exact tiers are no lexical leader whatever their score.
			pinned: 1,
			hits:   []toolSearchHit{hit("p1", 100, toolSearchTierName)},
			want:   []string{"p1", "sem1", "sem2"},
		},
		{
			name:  "no hits and no fused names",
			fused: nil,
			want:  nil,
		},
		{
			name:   "only pinned names leave no slot to guard",
			fused:  []string{"p1"},
			pinned: 1,
			hits:   []toolSearchHit{lex("lead", 6)},
			want:   []string{"p1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := slices.Clone(tc.fused)
			got := holdClearLexicalLeader(tc.fused, tc.pinned, tc.hits)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("holdClearLexicalLeader = %v, want %v", got, tc.want)
			}
			if !slices.Equal(tc.fused, before) {
				t.Fatalf("the input changed to %v from %v", tc.fused, before)
			}
		})
	}
}

// TestSearchContextKeepsAClearLexicalLeader shows the guard end to end: the
// manual search ranks the second lexical hit first, which the fusion alone
// would let overtake a leader that clearly outscores it.
func TestSearchContextKeepsAClearLexicalLeader(t *testing.T) {
	catalog := newToolCatalog(nil, nil, "", false)
	for _, entry := range []*ToolCatalogEntry{
		testSearchEntry("a_leader", "Frobnicate widgets", ToolStatusHidden),
		testSearchEntry("b_second", "Widgets", ToolStatusHidden),
		testSearchEntry("c_semantic", "Unrelated text", ToolStatusHidden),
	} {
		catalog.add(entry)
	}
	const query = "frobnicate widgets"
	hits := catalog.rankedSearch(query)
	if len(hits) != 2 || hits[0].Entry.Name != "a_leader" || hits[1].Entry.Name != "b_second" {
		t.Fatalf("fixture: lexical ranking = %v", catalogNames(hitEntries(hits)))
	}
	if hits[0].Score < toolSearchLeaderMargin*hits[1].Score {
		t.Fatalf("fixture: leader %.3f is not %.1f times ahead of %.3f", hits[0].Score, toolSearchLeaderMargin, hits[1].Score)
	}

	source := manualGuideSearcher{manuals: []string{"b_second", "c_semantic"}}
	got := catalogNames(catalog.SearchContext(context.Background(), query, source, nil))
	if want := []string{"a_leader", "b_second", "c_semantic"}; !slices.Equal(got, want) {
		t.Fatalf("SearchContext = %v, want %v", got, want)
	}

	// Without the guard the manual ranking decides.
	var lexical, manuals []string
	for _, hit := range hits {
		lexical = append(lexical, hit.Entry.Name)
	}
	manuals = source.manuals
	members := map[string][]string{"b_second": {"b_second"}, "c_semantic": {"c_semantic"}}
	if fused := fuseToolRanking(nil, lexical, manuals, members, nil, discoveryToolFusion); fused[0] != "b_second" {
		t.Fatalf("fixture: the fusion alone should put b_second first, got %v", fused)
	}
}

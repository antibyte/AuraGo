package agent

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestFuseToolRanking(t *testing.T) {
	small := toolFusionConfig{ManualWindow: 3, MaxSemanticTools: 2, FamilyLimit: 2, LexicalWeight: 1, SemanticWeight: 0.8, K: 20}
	// Reciprocal weights with K=0 make ties exact.
	tie := toolFusionConfig{ManualWindow: 4, MaxSemanticTools: 5, FamilyLimit: 3, LexicalWeight: 1, SemanticWeight: 1}
	noSkip := func(string) bool { return false }
	for _, tc := range []struct {
		name     string
		cfg      toolFusionConfig
		pinned   []string
		lexical  []string
		manuals  []string
		members  map[string][]string
		skip     func(string) bool
		expected []string
	}{
		{
			name:     "pinned first, no duplicates",
			cfg:      small,
			pinned:   []string{"p1", "p2", "p1"},
			lexical:  []string{"l1", "p2", "l2"},
			expected: []string{"p1", "p2", "l1", "l2"},
		},
		{
			name:    "budget, family limit and window bound the semantic tools",
			cfg:     small,
			lexical: []string{"l1", "l2"},
			manuals: []string{"m1", "m2", "m3", "m4"},
			members: map[string][]string{
				"m1": {"s1"},
				"m2": {"b1", "b2", "b3"}, // wider than the family limit and no lexical member
				"m3": {"s2", "s3"},       // only s2 fits the budget of two
				"m4": {"s4"},             // beyond the window of three
			},
			expected: []string{"l1", "l2", "s1", "s2"},
		},
		{
			name:    "a lexical member gains the manual rank without using the budget",
			cfg:     small,
			lexical: []string{"l1", "l2", "l3"},
			manuals: []string{"m1", "m2"},
			members: map[string][]string{"m1": {"l3", "s1"}, "m2": {"s2"}},
			// l3 is found twice and moves up; s1 and s2 both fit the budget.
			expected: []string{"l3", "l1", "l2", "s1", "s2"},
		},
		{
			name:    "a wide family still boosts its lexical members",
			cfg:     small,
			lexical: []string{"l1", "l2"},
			manuals: []string{"m1"},
			members: map[string][]string{"m1": {"l2", "x1", "x2"}},
			// x1 and x2 stay out; l2 gains the semantic score and passes l1.
			expected: []string{"l2", "l1"},
		},
		{
			name:    "a skipped manual takes no window position and no rank",
			cfg:     small,
			manuals: []string{"hidden", "m1", "m2", "m3", "m4"},
			members: map[string][]string{"hidden": {"h"}, "m1": {"a"}, "m2": {"b"}, "m3": {"c"}, "m4": {"d"}},
			skip:    func(manual string) bool { return manual == "hidden" },
			// The window of three counts m1..m3; the budget of two keeps a and b.
			expected: []string{"a", "b"},
		},
		{
			name:     "a manual without tools still uses a window position",
			cfg:      toolFusionConfig{ManualWindow: 2, MaxSemanticTools: 5, FamilyLimit: 3, LexicalWeight: 1, SemanticWeight: 1},
			manuals:  []string{"empty", "m1", "m2"},
			members:  map[string][]string{"m1": {"a"}, "m2": {"b"}},
			expected: []string{"a"},
		},
		{
			name:    "ties go to the better rank, then to the tool seen first",
			cfg:     tie,
			lexical: []string{"l1", "l2", "l3", "x"},
			manuals: []string{"m1", "m2", "m3", "m4"},
			// l2 (lexical rank 2), y (manual rank 2) and x (lexical 4 plus
			// manual 4) all score 0.5.
			members:  map[string][]string{"m2": {"y"}, "m4": {"x"}},
			expected: []string{"l1", "l2", "y", "x", "l3"},
		},
		{
			name:     "a manual listed twice counts once",
			cfg:      small,
			manuals:  []string{"m1", "m1", "m2"},
			members:  map[string][]string{"m1": {"a"}, "m2": {"b"}},
			expected: []string{"a", "b"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			skip := tc.skip
			if skip == nil {
				skip = noSkip
			}
			if got := fuseToolRanking(tc.pinned, tc.lexical, tc.manuals, tc.members, skip, tc.cfg); !slices.Equal(got, tc.expected) {
				t.Fatalf("fuseToolRanking = %v, want %v", got, tc.expected)
			}
		})
	}
}

// blockingGuideSearcher answers nothing until the caller gives up.
type blockingGuideSearcher struct{ memory.VectorDB }

func (blockingGuideSearcher) SearchToolGuideMatchesContext(ctx context.Context, _ string, _ int) ([]memory.ToolGuideMatch, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// manualGuideSearcher returns the given manuals for any query.
type manualGuideSearcher struct {
	memory.VectorDB
	manuals []string
}

func (s manualGuideSearcher) SearchToolGuideMatchesContext(context.Context, string, int) ([]memory.ToolGuideMatch, error) {
	var matches []memory.ToolGuideMatch
	for _, manual := range s.manuals {
		matches = append(matches, memory.ToolGuideMatch{Path: filepath.Join("tools_manuals", manual+".md")})
	}
	return matches, nil
}

func TestSearchContextFallsBackToLexicalWhenTheManualSearchBlocks(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil), nil, "")
	const query = "send an email to the team"
	want := catalogNames(catalog.Search(query))
	start := time.Now()
	got := catalogNames(catalog.SearchContext(context.Background(), query, blockingGuideSearcher{}))
	if elapsed := time.Since(start); elapsed > 700*time.Millisecond {
		t.Fatalf("a blocked manual search held the result for %s", elapsed)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lexical fallback = %v, want %v", got, want)
	}
}

func TestSearchContextFusesManualsAndKeepsStatusOrder(t *testing.T) {
	catalog := newToolCatalog(nil, nil, "", false)
	for _, entry := range []*ToolCatalogEntry{
		testSearchEntry("x_lexical", "Frobnicate widgets", ToolStatusHidden),
		testSearchEntry("y_enabled", "Unrelated text one", ToolStatusHidden),
		testSearchEntry("z_disabled", "Unrelated text two", ToolStatusDisabled),
		testSearchEntry("exact_tool", "Exact fixture", ToolStatusHidden),
	} {
		catalog.add(entry)
	}
	source := manualGuideSearcher{manuals: []string{"z_disabled", "y_enabled"}}
	// The disabled tool ranks first by manual but stays behind the enabled one.
	if got, want := catalogNames(catalog.SearchContext(context.Background(), "frobnicate", source)), []string{"x_lexical", "y_enabled", "z_disabled"}; !slices.Equal(got, want) {
		t.Fatalf("fused result = %v, want %v", got, want)
	}
	// An exact name stays first whatever the manuals say.
	if got := catalogNames(catalog.SearchContext(context.Background(), "exact_tool", source)); len(got) == 0 || got[0] != "exact_tool" {
		t.Fatalf("exact name lost its place: %v", got)
	}
}

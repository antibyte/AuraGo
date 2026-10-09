package agent

import (
	"context"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
	"testing"

	"aurago/internal/memory"
	promptsembed "aurago/prompts"
)

// standInGuideIndex ranks the embedded tool manuals like the tool_guides
// vector index with a stand-in embedder: the similarity of a manual is the
// number of distinct query words it contains, ties keep the manual name
// order, and a search returns the top-k unique paths with a similarity above
// zero. skip leaves one manual out, which gives the corpus before it existed.
type standInGuideIndex struct {
	memory.VectorDB
	manuals map[string]string // manual name -> lower-cased text
}

func newStandInGuideIndex(t *testing.T, skip string) *standInGuideIndex {
	t.Helper()
	ix := &standInGuideIndex{manuals: map[string]string{}}
	entries, err := fs.ReadDir(promptsembed.FS, "tools_manuals")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".md")
		if !ok || name == skip {
			continue
		}
		raw, err := fs.ReadFile(promptsembed.FS, "tools_manuals/"+entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		ix.manuals[name] = strings.ToLower(string(raw))
	}
	return ix
}

func (ix *standInGuideIndex) rank(query string, k int) []string {
	words := strings.Fields(strings.ToLower(query))
	type hit struct {
		name  string
		score int
	}
	var hits []hit
	for name, text := range ix.manuals {
		score := 0
		for _, w := range words {
			if strings.Contains(text, w) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, hit{name, score})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].name < hits[j].name
	})
	var paths []string
	for _, h := range hits[:min(k, len(hits))] {
		paths = append(paths, path.Join("tools_manuals", h.name+".md"))
	}
	return paths
}

func (ix *standInGuideIndex) SearchToolGuides(query string, topK int) ([]string, error) {
	return ix.rank(query, topK), nil
}

func (ix *standInGuideIndex) SearchToolGuidesContext(_ context.Context, query string, topK int) ([]string, error) {
	return ix.rank(query, topK), nil
}

func (ix *standInGuideIndex) SearchToolGuideMatchesContext(_ context.Context, query string, topK int) ([]memory.ToolGuideMatch, error) {
	var matches []memory.ToolGuideMatch
	for _, p := range ix.rank(query, topK) {
		matches = append(matches, memory.ToolGuideMatch{Path: p})
	}
	return matches, nil
}

// The local_wikipedia manual is in the tool_guides index whether or not the
// tool is enabled. It must not take a semantic slot from another tool: the
// adaptive priority and discover_tools give exactly the results of an index
// without the manual while the tool is off, and the adaptive priority also
// while it is on (the ranking never picks local_wikipedia).
func TestLocalWikipediaManualTakesNoSemanticSlot(t *testing.T) {
	const query = "kiwix infobox redirect snippet next_offset truncated"
	withManual, withoutManual := newStandInGuideIndex(t, ""), newStandInGuideIndex(t, "local_wikipedia")
	if top := withManual.rank(query, 4); !slices.Contains(top, "tools_manuals/local_wikipedia.md") {
		t.Fatalf("the stand-in ranking must put the manual in the top four, got %v", top)
	}
	if got := withoutManual.rank(query, 6); len(got) < 6 {
		t.Fatalf("the stand-in corpus needs at least six other matches, got %v", got)
	}
	off := allBuiltinToolFeatureFlags()
	off.LocalWikipediaEnabled = false
	on := off
	on.LocalWikipediaEnabled = true
	for name, ff := range map[string]ToolFeatureFlags{"off": off, "on": on} {
		schemas := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
		got := buildAdaptiveToolPriority(schemas, nil, query, withManual, nil)
		want := buildAdaptiveToolPriority(schemas, nil, query, withoutManual, nil)
		got = slices.DeleteFunc(got, func(n string) bool { return n == "local_wikipedia" })
		want = slices.DeleteFunc(want, func(n string) bool { return n == "local_wikipedia" })
		if !slices.Equal(got, want) {
			t.Errorf("tool %s: adaptive priority %v, without the manual %v", name, got, want)
		}
	}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, off, nil)
	catalog := BuildToolCatalog(schemas, schemas, "")
	if lexical := catalog.Search(query); len(lexical) >= 5 {
		t.Fatalf("query must leave room for semantic discovery, lexical = %d", len(lexical))
	}
	got := catalogNames(catalog.SearchContext(context.Background(), query, withManual))
	want := catalogNames(catalog.SearchContext(context.Background(), query, withoutManual))
	if !slices.Equal(got, want) {
		t.Errorf("discover_tools with the tool off: %v, without the manual %v", got, want)
	}
	onSchemas := BuildNativeToolSchemas(t.TempDir(), nil, on, nil)
	onCatalog := BuildToolCatalog(onSchemas, onSchemas, "")
	if found := catalogNames(onCatalog.SearchContext(context.Background(), query, withManual)); !slices.Contains(found, "local_wikipedia") {
		t.Errorf("discover_tools with the tool on misses local_wikipedia: %v", found)
	}
}

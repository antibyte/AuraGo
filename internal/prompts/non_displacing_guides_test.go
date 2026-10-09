package prompts

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"aurago/internal/memory"
	promptsembed "aurago/prompts"
)

// standInGuideSearch writes the embedded manuals to a tools directory and
// ranks them like the tool_guides index with a stand-in embedder: the
// similarity of a manual is the number of distinct query words it contains,
// ties keep the name order. skip leaves one manual out of the index (the
// corpus before it existed); the file stays on disk like an indexed manual.
func standInGuideSearch(t *testing.T, dir, skip string) func(context.Context, memory.VectorDB, string, int) ([]memory.ToolGuideMatch, error) {
	t.Helper()
	texts := map[string]string{}
	entries, err := fs.ReadDir(promptsembed.FS, "tools_manuals")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".md")
		if !ok {
			continue
		}
		raw, err := fs.ReadFile(promptsembed.FS, "tools_manuals/"+entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if name != skip {
			texts[name] = strings.ToLower(string(raw))
		}
	}
	return func(_ context.Context, _ memory.VectorDB, query string, topK int) ([]memory.ToolGuideMatch, error) {
		type hit struct {
			name  string
			score int
		}
		var hits []hit
		for name, text := range texts {
			score := 0
			for _, w := range strings.Fields(strings.ToLower(query)) {
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
		var matches []memory.ToolGuideMatch
		for _, h := range hits[:min(topK, len(hits))] {
			matches = append(matches, memory.ToolGuideMatch{Path: filepath.Join(dir, h.name+".md")})
		}
		return matches, nil
	}
}

// The local_wikipedia manual is indexed whether or not the tool is enabled.
// While the tool is off it must not take one of the two semantic guide
// slots: the prompt guides are exactly those of an index without it.
func TestDisabledLocalWikipediaManualTakesNoGuideSlot(t *testing.T) {
	original := searchDynamicToolGuides
	t.Cleanup(func() { searchDynamicToolGuides = original })
	const query = "kiwix infobox redirect snippet next_offset truncated"
	dir := t.TempDir()
	withManual := standInGuideSearch(t, dir, "")
	withoutManual := standInGuideSearch(t, dir, "local_wikipedia")
	if top, _ := withManual(context.Background(), nil, query, 2); len(top) == 0 || filepath.Base(top[0].Path) != "local_wikipedia.md" {
		t.Fatalf("the stand-in ranking must put the manual first, got %v", top)
	}
	guides := func(search func(context.Context, memory.VectorDB, string, int) ([]memory.ToolGuideMatch, error), strategy DynamicGuideStrategy) []string {
		searchDynamicToolGuides = search
		return PrepareDynamicGuidesWithStrategyContext(context.Background(), &memory.ChromemVectorDB{}, nil, query, "", dir, nil, nil, 3, strategy, nil)
	}
	allowed := []string{"wikipedia_search", "web_scraper", "api_request", "filesystem", "execute_shell", "send_email", "webhooks", "homepage"}
	for name, strategy := range map[string]DynamicGuideStrategy{
		"text mode, tool off":  {PreferSemantics: true, DisableRecentHeuristics: true, Flags: &ContextFlags{}},
		"native, not allowed":  {PreferSemantics: true, DisableRecentHeuristics: true, AllowedTools: allowed, Flags: &ContextFlags{}},
		"native, no flags set": {PreferSemantics: true, DisableRecentHeuristics: true, AllowedTools: allowed},
	} {
		got, want := guides(withManual, strategy), guides(withoutManual, strategy)
		if !slices.Equal(got, want) {
			t.Errorf("%s: %d guides with the manual, %d without; differ", name, len(got), len(want))
		}
	}
	enabled := DynamicGuideStrategy{PreferSemantics: true, DisableRecentHeuristics: true, AllowedTools: append(allowed, "local_wikipedia"), Flags: &ContextFlags{LocalWikipediaEnabled: true}}
	if got := guides(withManual, enabled); len(got) == 0 || !strings.Contains(got[0], "Kiwix") {
		t.Errorf("enabled local_wikipedia lost its guide: %d guides", len(got))
	}
}

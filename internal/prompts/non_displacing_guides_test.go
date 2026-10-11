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

// The manual of a non-displacing tool is indexed whether or not the tool is
// enabled. While the tool is off it must not take one of the two semantic
// guide slots: the prompt guides are exactly those of an index without it.
func TestDisabledLocalWikipediaManualTakesNoGuideSlot(t *testing.T) {
	assertDisabledManualTakesNoGuideSlot(t, "local_wikipedia", "kiwix infobox redirect snippet next_offset truncated", "Kiwix", &ContextFlags{LocalWikipediaEnabled: true})
}

func TestDisabledRetroGamesManualTakesNoGuideSlot(t *testing.T) {
	assertDisabledManualTakesNoGuideSlot(t, "retro_games", "retro games hall fame index_building index_incomplete power button msdos", "archive.org", &ContextFlags{RetroGamesEnabled: true})
}

// assertDisabledManualTakesNoGuideSlot checks the guide selection for the
// manual of tool: without it in the index, and with it while the tool is off
// (text mode, native mode with the tool not allowed, native mode without
// flags), the guides are identical; with the tool on (enabledFlags) its guide
// is the first one and mentions guideWord.
func assertDisabledManualTakesNoGuideSlot(t *testing.T, tool, query, guideWord string, enabledFlags *ContextFlags) {
	t.Helper()
	original := searchDynamicToolGuides
	t.Cleanup(func() { searchDynamicToolGuides = original })
	dir := t.TempDir()
	withManual := standInGuideSearch(t, dir, "")
	withoutManual := standInGuideSearch(t, dir, tool)
	if top, _ := withManual(context.Background(), nil, query, 2); len(top) == 0 || filepath.Base(top[0].Path) != tool+".md" {
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
	enabled := DynamicGuideStrategy{PreferSemantics: true, DisableRecentHeuristics: true, AllowedTools: append(slices.Clone(allowed), tool), Flags: enabledFlags}
	if got := guides(withManual, enabled); len(got) == 0 || !strings.Contains(got[0], guideWord) {
		t.Errorf("enabled %s lost its guide: %d guides", tool, len(got))
	}
}

package agent

import (
	"strings"
	"testing"

	"aurago/internal/prompts"
)

func TestLocalWikipediaSchemaIsGatedAndReadOnly(t *testing.T) {
	if containsName(toolNames(BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{}, nil)), "local_wikipedia") {
		t.Fatal("local_wikipedia schema present while disabled")
	}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{LocalWikipediaEnabled: true}, nil)
	props := nativeToolProperties(t, schemas, "local_wikipedia")
	allowed := map[string]bool{"operation": true, "query": true, "limit": true, "title": true, "path": true, "section": true, "offset": true, "_todo": true}
	for name := range allowed {
		if _, ok := props[name]; !ok && name != "_todo" {
			t.Fatalf("missing property %q", name)
		}
	}
	for name := range props {
		if !allowed[name] {
			t.Fatalf("unexpected property %q: the tool is read-only", name)
		}
	}
	operation := props["operation"].(map[string]interface{})
	if enum, _ := operation["enum"].([]string); strings.Join(enum, ",") != "search,read" {
		t.Fatalf("operation enum = %v", operation["enum"])
	}
	if (ToolFeatureFlags{}).Key() == (ToolFeatureFlags{LocalWikipediaEnabled: true}).Key() {
		t.Fatal("feature key ignores LocalWikipediaEnabled")
	}
}

// The live acceptance found the encyclopedia-intent swap skipped at default
// settings: the knapsack fills the schema token budget to within a few
// tokens, and local_wikipedia (358) was far larger than wikipedia_search
// (181). The schema now stays close to wikipedia_search's size.
func TestLocalWikipediaSchemaStaysCompact(t *testing.T) {
	got := estimateSingleToolSchemaTokens(localWikipediaSchema())
	var online int
	for _, s := range builtinToolSchemas(ToolFeatureFlags{}) {
		if s.Function.Name == "wikipedia_search" {
			online = estimateSingleToolSchemaTokens(s)
		}
	}
	if online == 0 {
		t.Fatal("wikipedia_search schema not found")
	}
	if got > 220 || got-online > 40 {
		t.Fatalf("local_wikipedia schema is %d tokens (wikipedia_search %d); keep it at most 220 and within 40 of wikipedia_search", got, online)
	}
	desc := localWikipediaSchema().Function.Description
	for _, want := range []string{"wikipedia_search", "web search", "edition date"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("description lost %q: %s", want, desc)
		}
	}
}

func TestLocalWikipediaFeatureNeedsAnOpenEdition(t *testing.T) {
	cfg := localWikipediaTestConfig()
	useAgentWikiSource(t, nil)
	if buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, "")).LocalWikipediaEnabled {
		t.Fatal("tool enabled without an open edition")
	}
	useAgentWikiSource(t, &agentWikiSource{lib: &agentWikiLibrary{}, open: true})
	if !buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, "")).LocalWikipediaEnabled {
		t.Fatal("tool disabled with an open edition")
	}
	if !buildPromptContextFlags(RunConfig{Config: cfg}, ToolingPolicy{}, promptContextOptions{}).LocalWikipediaEnabled {
		t.Fatal("prompt flags miss the open edition")
	}
	if !containsName(collectEnabledTools(&prompts.ContextFlags{LocalWikipediaEnabled: true}), "local_wikipedia") {
		t.Fatal("prompt cache key ignores local_wikipedia")
	}
	cfg.LocalWikipedia.AgentAccess = false
	if buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, "")).LocalWikipediaEnabled {
		t.Fatal("tool enabled without agent access")
	}
}

func TestLocalWikipediaCatalogSearch(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil), nil, "")
	for _, query := range []string{"local_wikipedia", "Wikipedia offline", "lokale Wikipedia Artikel lesen", "offline encyclopedia"} {
		matches := catalog.Search(query)
		found := false
		for _, entry := range matches[:min(5, len(matches))] {
			found = found || entry.Name == "local_wikipedia"
		}
		if !found {
			t.Fatalf("%q does not find local_wikipedia in the top five", query)
		}
	}
}

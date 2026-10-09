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

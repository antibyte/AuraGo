package agent

import "testing"

func TestLocalWikipediaDiscoverySurfaces(t *testing.T) {
	if ToolCategoryForName("local_wikipedia") != "network" {
		t.Fatalf("category = %q", ToolCategoryForName("local_wikipedia"))
	}
	for _, alias := range []string{"offline wikipedia", "Lokale Wikipedia", "kiwix"} {
		if got := resolveDiscoverToolName(alias); got != "local_wikipedia" {
			t.Fatalf("alias %q resolves to %q", alias, got)
		}
	}
	if resolveDiscoverToolName("wikipedia") != "wikipedia_search" {
		t.Fatal("the online wikipedia alias must stay unchanged")
	}
	if classifyToolFamily("local_wikipedia") != "web" || inferToolFamilyFromQuery("lies den wikipedia artikel") != "web" {
		t.Fatal("local_wikipedia must belong to the web family")
	}
	if seeds := adaptiveFamilySeedsForQuery("Was steht im Lexikon über die Enzyklopädie?"); !containsName(seeds, "local_wikipedia") {
		t.Fatalf("seeds = %v", seeds)
	}
	if seeds := adaptiveFamilySeedsForQuery("starte den docker container neu"); containsName(seeds, "local_wikipedia") {
		t.Fatalf("unrelated query seeds local_wikipedia: %v", seeds)
	}
}

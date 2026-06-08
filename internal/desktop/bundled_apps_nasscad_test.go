package desktop

import "testing"

func TestBuildMonolithicNasscadHTMLInlinesSiblingScripts(t *testing.T) {
	indexHTML, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/index.html")
	if err != nil {
		t.Fatalf("ReadFile index: %v", err)
	}
	monolithic, err := buildMonolithicNasscadHTML(indexHTML, bundledAppAssets, "bundled_apps/nasscad")
	if err != nil {
		t.Fatalf("buildMonolithicNasscadHTML: %v", err)
	}
	if !bytesContainsNasscadMonolithMarkers(monolithic) {
		t.Fatal("monolithic nasscad html is missing required runtime markers")
	}
	if nasscadExternalScriptPattern.Match(monolithic) {
		t.Fatal("monolithic nasscad html still contains external script tags")
	}
	if len(monolithic) <= len(indexHTML) {
		t.Fatalf("monolithic html should be larger than shell index: %d <= %d", len(monolithic), len(indexHTML))
	}
}
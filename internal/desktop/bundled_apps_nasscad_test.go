package desktop

import (
	"bytes"
	"testing"
)

func TestBuildMonolithicNasscadHTMLInlinesSiblingScripts(t *testing.T) {
	indexHTML, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/index.html")
	if err != nil {
		t.Fatalf("ReadFile index: %v", err)
	}
	if !bytes.Contains(indexHTML, []byte("NASSCAD V4.3.0")) {
		t.Fatal("bundled nasscad shell should contain NASSCAD V4.3.0")
	}
	if bytes.Contains(indexHTML, []byte(`margin-left:4px">V4.2.7</span>`)) {
		t.Fatal("bundled nasscad shell should not show the previous version in the visible app logo")
	}
	if !bytes.Contains(indexHTML, []byte(`margin-left:4px">V4.3.0</span>`)) {
		t.Fatal("bundled nasscad shell should show NASSCAD V4.3.0 in the visible app logo")
	}
	if !bytes.Contains(indexHTML, []byte(`helvetiker_bold.typeface.js`)) {
		t.Fatal("bundled nasscad shell should reference current typeface font assets")
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

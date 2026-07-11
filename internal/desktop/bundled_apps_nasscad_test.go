package desktop

import (
	"bytes"
	"testing"
)

func TestBuildMonolithicNasscadHTMLAcceptsBundledAIO(t *testing.T) {
	indexHTML, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/index.html")
	if err != nil {
		t.Fatalf("ReadFile index: %v", err)
	}
	if !bytes.Contains(indexHTML, []byte("NASSCAD V4.3.0")) {
		t.Fatal("bundled nasscad AIO should contain NASSCAD V4.3.0")
	}
	if bytes.Contains(indexHTML, []byte(`margin-left:4px">V4.2.7</span>`)) {
		t.Fatal("bundled nasscad AIO should not show the previous version in the visible app logo")
	}
	if !bytes.Contains(indexHTML, []byte(`margin-left:4px">V4.3.0</span>`)) {
		t.Fatal("bundled nasscad AIO should show NASSCAD V4.3.0 in the visible app logo")
	}
	if len(indexHTML) < 20*1024*1024 {
		t.Fatalf("bundled nasscad AIO looks too small: %d bytes", len(indexHTML))
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
	if len(monolithic) != len(indexHTML) {
		t.Fatalf("pre-inlined AIO should pass through unchanged: %d != %d", len(monolithic), len(indexHTML))
	}
}

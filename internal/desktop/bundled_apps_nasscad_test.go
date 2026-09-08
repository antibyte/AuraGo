package desktop

import (
	"bytes"
	"testing"
)

func TestBuildMonolithicNasscadHTMLInlinesBundledRuntime(t *testing.T) {
	indexHTML, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/index.html")
	if err != nil {
		t.Fatalf("ReadFile index: %v", err)
	}
	if !bytes.Contains(indexHTML, []byte("NASSCAD V4.7.0")) {
		t.Fatal("bundled nasscad source should contain NASSCAD V4.7.0")
	}
	if !bytes.Contains(indexHTML, []byte(`margin-left:4px">V4.7.0</span>`)) {
		t.Fatal("bundled nasscad source should show NASSCAD V4.7.0 in the visible app logo")
	}
	if bytes.Contains(indexHTML, []byte("googletagmanager.com")) {
		t.Fatal("bundled nasscad source must not load web analytics")
	}
	if !nasscadExternalScriptPattern.Match(indexHTML) {
		t.Fatal("bundled nasscad source should reference the vendored runtime modules")
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
	if len(monolithic) < len(indexHTML)+4*1024*1024 {
		t.Fatalf("monolithic nasscad html looks too small: %d bytes", len(monolithic))
	}
	for asset, marker := range map[string]string{
		"step-import.js":  "nasscad_occt_wasm.js' + location.search",
		"quick-fillet.js": "opencascade.wasm.wasm'+location.search",
	} {
		data, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/" + asset)
		if err != nil {
			t.Fatalf("read bundled nasscad asset %s: %v", asset, err)
		}
		if !bytes.Contains(data, []byte(marker)) {
			t.Fatalf("bundled nasscad asset %s does not preserve the desktop embed token", asset)
		}
	}
}

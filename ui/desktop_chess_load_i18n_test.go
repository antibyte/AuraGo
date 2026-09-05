package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopChessLoadI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/chess.js")
	if !strings.Contains(source, "setStatus(state, ctx.t('desktop.chess_load_failed'))") {
		t.Fatal("chess vendor-load status must use desktop.chess_load_failed")
	}
	if strings.Contains(source, "(err && err.message) || ctx.t('desktop.chess_load_failed')") {
		t.Fatal("chess vendor-load status still shows raw err.message")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.chess_load_failed"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.chess_load_failed", path)
		}
		if lang == "de" && got == "Chess failed to load." {
			t.Fatalf("%s must not copy the English chess load-failed string", path)
		}
		if lang == "fr" && got == "Chess failed to load." {
			t.Fatalf("%s must not copy the English chess load-failed string", path)
		}
	}
}

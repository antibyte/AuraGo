package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopChessTemplateI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/chess.js")
	if !strings.Contains(source, "ctx.esc(ctx.t('desktop.chess_load_failed'))") {
		t.Fatal("chess template fallback must localize desktop.chess_load_failed")
	}
	if strings.Contains(source, "Chess UI failed to load.") {
		t.Fatal("chess template fallback still hardcodes English load text")
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
		if lang == "de" && got == "Chess UI failed to load." {
			t.Fatalf("%s must not copy the English chess template fallback", path)
		}
		if lang == "zh" && got == "Chess failed to load." {
			t.Fatalf("%s must not copy the English chess load-failed string", path)
		}
	}
}

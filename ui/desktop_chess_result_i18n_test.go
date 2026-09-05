package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopChessResultI18n(t *testing.T) {
	t.Parallel()

	fx := readDesktopAssetText(t, "js/desktop/apps/chess-fx.js")
	for _, want := range []string{
		"typeof options.t === 'function' ? options.t : (key => key)",
		"opts.primaryLabel || t('desktop.chess_new_game')",
		"opts.secondaryLabel || t('desktop.ok')",
	} {
		if !strings.Contains(fx, want) {
			t.Fatalf("chess result i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"|| 'New game'",
		"|| 'OK'",
	} {
		if strings.Contains(fx, forbidden) {
			t.Fatalf("chess result still hardcodes %q", forbidden)
		}
	}

	app := readDesktopAssetText(t, "js/desktop/apps/chess.js")
	if !strings.Contains(app, "t: ctx.t") {
		t.Fatal("chess app must pass t into createChessFx")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.chess_new_game", "desktop.ok"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.chess_new_game"] == "New game" {
			t.Fatalf("%s must not copy the English chess new-game label", path)
		}
	}
}

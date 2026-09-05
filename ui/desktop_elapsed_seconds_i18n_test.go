package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopElapsedSecondsI18n(t *testing.T) {
	t.Parallel()

	openscad := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"state.ctx.t('desktop.noisemaker_progress_elapsed', { seconds })",
	} {
		if !strings.Contains(openscad, want) {
			t.Fatalf("openscad elapsed i18n missing marker %q", want)
		}
	}
	if strings.Contains(openscad, "toFixed(0) + 's'") {
		t.Fatal("openscad still hardcodes elapsed seconds with + 's'")
	}

	homepage := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"t('desktop.noisemaker_progress_elapsed', { seconds })",
		"elapsedText(0)",
	} {
		if !strings.Contains(homepage, want) {
			t.Fatalf("homepage-studio elapsed i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"textContent = '0s'",
		"+ 's'",
	} {
		if strings.Contains(homepage, forbidden) {
			t.Fatalf("homepage-studio still hardcodes %q", forbidden)
		}
	}

	noisemaker := readDesktopAssetText(t, "js/desktop/apps/noisemaker.js")
	if !strings.Contains(noisemaker, "text(ctx, 'progress_elapsed', { seconds: 0 }") {
		t.Fatal("noisemaker seed elapsed i18n missing progress_elapsed")
	}
	if strings.Contains(noisemaker, "data-nm-elapsed>0 s<") {
		t.Fatal("noisemaker still hardcodes the 0 s seed")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.noisemaker_progress_elapsed"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.noisemaker_progress_elapsed", path)
		}
		if !strings.Contains(got, "{{seconds}}") {
			t.Fatalf("%s desktop.noisemaker_progress_elapsed must keep {{seconds}}", path)
		}
		if lang == "ja" && got == "{{seconds}} s" {
			t.Fatalf("%s must not copy the English elapsed-seconds unit", path)
		}
	}
}

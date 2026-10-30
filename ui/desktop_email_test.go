package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDesktopEmailExporterCoverage(t *testing.T) {
	// Each first-party producer/viewer in the product contract owns an adapter.
	for _, file := range []string{
		"apps/writer.js", "apps/sheets.js", "apps/notes.js", "apps/editor-filemenu.js", "apps/code-studio/core.js", "apps/cheater.js",
		"apps/pixel-email.js", "apps/camera.js", "apps/gallery.js", "apps/network-cameras.js", "apps/sysworld.js", "shared/qr-dialog.js",
		"apps/synth-studio.js", "apps/noisemaker.js", "apps/rtl-sdr.js", "apps/video-studio-jobs.js",
		"apps/openscad.js", "apps/nasscad.js", "apps/layerling.js", "apps/detective.js", "apps/newspaper.js",
		"apps/easydrag-editor.js", "apps/game-maker-studio.js", "apps/cyd-creator-app.js", "file-manager/core-render.js",
		"apps/viewer.js", "apps/viewer-3d.js", "apps/zipper.js", "apps/bookshelf.js",
	} {
		t.Run(file, func(t *testing.T) {
			text := string(mustReadUIFile(t, "js/desktop/"+file))
			if !strings.Contains(text, ".attach(") && !strings.Contains(text, "attachEmailExport(") {
				t.Fatal("missing Desktop email export adapter")
			}
		})
	}
	for _, file := range []string{"apps/tresor.js", "apps/ziro-designer.js", "sdk-runtime.js"} {
		if strings.Contains(string(mustReadUIFile(t, "js/desktop/"+file)), "AuraDesktopEmail") {
			t.Errorf("excluded app/runtime %s must not acquire an email handoff", file)
		}
	}
}

func TestDesktopEmailLocaleCoverage(t *testing.T) {
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]string
		if err := json.Unmarshal([]byte(mustReadUIFile(t, "lang/desktop/"+locale+".json")), &words); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"export", "format", "disabled", "readonly", "no_account", "settings", "empty", "failed", "too_large", "file_too_large", "preparing"} {
			if strings.TrimSpace(words["desktop.email_"+key]) == "" {
				t.Errorf("%s: missing desktop.email_%s", locale, key)
			}
		}
	}
}

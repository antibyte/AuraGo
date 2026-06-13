package ui

import (
	"os"
	"strings"
	"testing"
)

func TestConfigLayoutCSSDefinesGridAndActionRows(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	for _, marker := range []string{
		".field-grid",
		"display:grid",
		".field-grid.two-cols",
		"grid-template-columns:repeat(2, minmax(0, 1fr))",
		".cfg-actions-row",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config.css missing %q", marker)
		}
	}
}

func TestConfigPhase1HelpMarkersPresent(t *testing.T) {
	t.Parallel()

	heartbeatJS := normalizeAssetText(mustReadUIFile(t, "cfg/heartbeat.js"))
	for _, marker := range []string{
		"t('help.heartbeat.enabled')",
		"t('help.heartbeat.check_tasks')",
		"t('help.heartbeat.day_time_window')",
		"t('help.heartbeat.night_time_window')",
	} {
		if !strings.Contains(heartbeatJS, marker) {
			t.Fatalf("heartbeat.js missing help marker %q", marker)
		}
	}

	webhooksJS := normalizeAssetText(mustReadUIFile(t, "cfg/webhooks.js"))
	for _, marker := range []string{
		"t('help.webhooks.enabled')",
		"t('help.webhooks.max_payload_size')",
		"t('help.webhooks.rate_limit')",
	} {
		if !strings.Contains(webhooksJS, marker) {
			t.Fatalf("webhooks.js missing help marker %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"cfgFieldOptionLabel",
		"config.field.disabled_option",
		"config.field.other_custom_option",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing marker %q", marker)
		}
	}

	videoJS := normalizeAssetText(mustReadUIFile(t, "cfg/video_download.js"))
	if strings.Contains(videoJS, "field-grid two-col\"") || strings.Contains(videoJS, "field-grid two-col'") {
		t.Fatal("video_download.js still uses two-col typo")
	}
	if !strings.Contains(videoJS, "field-grid two-cols") {
		t.Fatal("video_download.js should use field-grid two-cols")
	}
}

func TestConfigVirtualDesktopSectionLabelsInSectionsBundle(t *testing.T) {
	t.Parallel()

	langs := []string{"en", "de", "fr", "ja", "zh"}
	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("lang/config/sections/" + lang + ".json")
			if err != nil {
				t.Fatalf("read sections %s: %v", lang, err)
			}
			content := string(data)
			for _, key := range []string{
				"config.section.virtual_desktop.label",
				"config.section.virtual_desktop.desc",
			} {
				if !strings.Contains(content, key) {
					t.Fatalf("sections/%s.json missing %q", lang, key)
				}
			}
		})
	}
}
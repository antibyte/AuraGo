package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTranslationsP3FeatureBlocks(t *testing.T) {
	t.Parallel()
	// These sentences are used by the feature renderers and require translation
	// in every target language. Brand names and shared technical terms are excluded.
	for section, keys := range map[string][]string{
		"config": {"config.sip.max_duration", "config.sip.answer_inbound"},
		"config/virtual_computers": {
			"help.virtual_computers.agent_control_browsers",
			"config.virtual_computers.allow_persistent_label",
		},
		"desktop": {
			"game_maker.no_projects", "game_maker.choose_project",
			"cheater.empty_subtitle", "pixel.remove_bg", "galaxa.relic_hint2",
		},
		"dashboard": {"dashboard.opt_reason_awaiting_comparable_exposures"},
	} {
		en, err := readJSONFileMap(filepath.Join("lang", section, "en.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
			t.Run(section+"/"+lang, func(t *testing.T) {
				values, err := readJSONFileMap(filepath.Join("lang", section, lang+".json"))
				if err != nil {
					t.Fatal(err)
				}
				for _, key := range keys {
					source, _ := en[key].(string)
					value, _ := values[key].(string)
					if source == "" {
						t.Fatalf("missing English source for %s", key)
					}
					if strings.TrimSpace(value) == "" || strings.TrimSpace(value) == source {
						t.Errorf("%s must have a translated %s value", key, lang)
					}
				}
			})
		}
	}
}

func TestTranslationsP3ConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(lang, func(t *testing.T) {
			page := browser.MustPage(configRefreshFixtureOrigin(t, lang, false) + "/config#overview")
			defer page.MustClose()
			waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
			for _, section := range []string{"sip", "telephone_agent", "virtual_computers"} {
				page.MustEval(`async section => {
					await selectSection(section, {scrollBehavior:'auto'});
					document.querySelectorAll('#content details').forEach(el => el.open = true);
					resetDirtySnapshot();
				}`, section)
				for _, width := range []int{390, 1440} {
					page.MustSetViewport(width, 900, 1, width == 390)
					page.MustEval(`() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`)
					result := page.MustEval(`() => {
						const content = document.getElementById('content');
						return {text: content.innerText.trim(), error: !!content.querySelector('.cfg-error-state'),
							overflow: document.documentElement.scrollWidth > innerWidth + 1 || content.scrollWidth > content.clientWidth + 1};
					}`).Map()
					if result["text"].Str() == "" || result["error"].Bool() || result["overflow"].Bool() {
						t.Errorf("%s at %d px: empty=%t error=%t overflow=%t", section, width,
							result["text"].Str() == "", result["error"].Bool(), result["overflow"].Bool())
					}
				}
			}
		})
	}
}

package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The Storymaker block of the Virtual Desktop configuration page: storymaker.enabled and storymaker.render.{timeout_seconds,
// max_duration_seconds, browser_path, transport}, the export-readiness check against /api/desktop/storymaker/status, and the
// sixteen-language texts in lang/config/common.

var storymakerConfigKeys = []string{
	"title", "help", "enabled", "timeout", "timeout_help", "max_duration", "max_duration_help", "browser_path", "browser_path_help",
	"transport", "transport_help", "transport_auto", "check", "checking", "unavailable", "save_first", "ready", "missing", "disabled",
	"part_ffmpeg", "part_browser",
}

func TestConfigStorymakerBlockIsWired(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("cfg", "virtual_desktop.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, marker := range []string{
		"html += vdCfgStorymaker();",
		"function vdCfgStorymaker()",
		`data-path="storymaker.render.timeout_seconds"`,
		`min="60" max="14400"`,
		`data-path="storymaker.render.max_duration_seconds"`,
		`min="10" max="3600"`,
		`data-path="storymaker.render.browser_path"`,
		`data-path="storymaker.render.transport"`,
		"'storymaker.enabled'",
		"fetch('/api/desktop/storymaker/status'",
		"hasUnsavedConfigChanges()",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("virtual_desktop.js lacks %q", marker)
		}
	}
	for _, key := range storymakerConfigKeys {
		if !strings.Contains(src, "'"+key+"'") && !strings.Contains(src, "'config.storymaker."+key+"'") {
			t.Errorf("virtual_desktop.js never uses config.storymaker.%s", key)
		}
	}
	// The limits of the form are the limits of NormalizeStorymakerConfig.
	normalize, err := os.ReadFile(filepath.Join("..", "internal", "config", "storymaker.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []string{"60", "14400", "10", "3600"} {
		if !strings.Contains(string(normalize), limit) {
			t.Errorf("internal/config/storymaker.go no longer mentions the limit %s the form promises", limit)
		}
	}
}

func TestConfigStorymakerTranslations(t *testing.T) {
	t.Parallel()
	english := map[string]string{}
	for _, lang := range storymakerLanguages {
		values := storymakerConfigLocale(t, lang)
		if lang == "en" {
			for key, value := range values {
				english[key] = value
			}
		}
		for _, key := range storymakerConfigKeys {
			full := "config.storymaker." + key
			text := strings.TrimSpace(values[full])
			if text == "" || strings.ContainsRune(text, '�') {
				t.Errorf("%s: %s is missing or broken", lang, full)
			}
			if strings.Contains(text, "{missing}") != (key == "missing") {
				t.Errorf("%s: %s has the wrong placeholders", lang, full)
			}
			if lang == "de" && (strings.Contains(" "+text+" ", " Sie ") || strings.Contains(" "+text+" ", " Ihre ") || strings.Contains(" "+text+" ", " Ihnen ")) {
				t.Errorf("de: %s uses formal address", full)
			}
		}
	}
	// Names that stay as they are in several languages.
	same := map[string]bool{"title": true, "part_ffmpeg": true, "part_browser": true}
	for _, lang := range storymakerLanguages {
		if lang == "en" {
			continue
		}
		values := storymakerConfigLocale(t, lang)
		for _, key := range storymakerConfigKeys {
			full := "config.storymaker." + key
			if values[full] == english[full] && !same[key] {
				t.Errorf("%s: %s is untranslated", lang, full)
			}
		}
	}
}

func storymakerConfigLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	return storymakerLocale(t, "config/common", lang)
}

func TestConfigStorymakerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(30 * time.Second)
	page.MustWaitLoad()
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
        const originalFetch = window.fetch;
        window.storymakerStatusCalls = 0;
        window.storymakerStatus = {enabled: true, ffmpeg: {ready: false}, browser: {ready: true}};
        window.fetch = (url, options = {}) => {
            if (String(url) === '/api/desktop/storymaker/status') {
                storymakerStatusCalls++;
                return Promise.resolve(new Response(JSON.stringify(storymakerStatus), {headers: {'Content-Type': 'application/json'}}));
            }
            if (String(url) === '/api/config' && options.method === 'PUT') window.storymakerSavedConfig = JSON.parse(options.body);
            return originalFetch(url, options);
        };
        await selectSection('virtual_desktop'); resetDirtySnapshot();
    }`)
	// Defaults of a configuration that has no storymaker section: on, 3600 s, 900 s, no browser path, automatic transfer.
	if !page.MustEval(`() => document.querySelector('[data-path="storymaker.enabled"]').classList.contains('on') && document.querySelector('[data-path="storymaker.render.timeout_seconds"]').value==='3600' && document.querySelector('[data-path="storymaker.render.max_duration_seconds"]').value==='900' && document.querySelector('[data-path="storymaker.render.browser_path"]').value==='' && document.querySelector('[data-path="storymaker.render.transport"]').value===''`).Bool() {
		t.Fatal("the Storymaker defaults are not what the server applies")
	}
	page.MustEval(`() => {
        const set = (path, value) => { const input = document.querySelector('[data-path="' + path + '"]'); input.value = value; input.dispatchEvent(new Event('change', {bubbles: true})); };
        set('storymaker.render.timeout_seconds', '1800'); set('storymaker.render.max_duration_seconds', '300');
        set('storymaker.render.browser_path', '/usr/bin/chromium'); set('storymaker.render.transport', 'png');
        toggleBool(document.querySelector('[data-path="storymaker.enabled"]'));
    }`)
	page.MustEval(`async () => vdCfgTestStorymaker(document.getElementById('vd-storymaker-btn'))`)
	if !page.MustEval(`() => storymakerStatusCalls===0 && document.getElementById('vd-storymaker-result').textContent===t('config.storymaker.save_first')`).Bool() {
		t.Fatal("the check must not run against unsaved settings")
	}
	if !page.MustEval(`async () => await saveConfig()`).Bool() {
		t.Fatal("the Storymaker settings could not be saved")
	}
	if !page.MustEval(`() => { const s = storymakerSavedConfig.storymaker; return s.enabled===false && s.render.timeout_seconds===1800 && s.render.max_duration_seconds===300 && s.render.browser_path==='/usr/bin/chromium' && s.render.transport==='png'; }`).Bool() {
		t.Fatalf("saving the Desktop section omitted the Storymaker settings: %s", page.MustEval(`() => JSON.stringify((window.storymakerSavedConfig||{}).storymaker)`).String())
	}
	// The check names what is missing, then reports ready.
	page.MustEval(`async () => vdCfgTestStorymaker(document.getElementById('vd-storymaker-btn'))`)
	waitForJSBool(t, page, `() => storymakerStatusCalls===1 && document.getElementById('vd-storymaker-result').textContent===t('config.storymaker.missing').replace('{missing}', t('config.storymaker.part_ffmpeg'))`)
	page.MustEval(`() => { storymakerStatus = {enabled: true, ffmpeg: {ready: true}, browser: {ready: true}}; }`)
	page.MustEval(`async () => vdCfgTestStorymaker(document.getElementById('vd-storymaker-btn'))`)
	waitForJSBool(t, page, `() => storymakerStatusCalls===2 && document.getElementById('vd-storymaker-result').textContent===t('config.storymaker.ready')`)
	page.MustSetViewport(390, 844, 1, true)
	if page.MustEval(`() => document.documentElement.scrollWidth>innerWidth+1`).Bool() {
		t.Fatal("the Storymaker configuration overflows the narrow viewport")
	}
}

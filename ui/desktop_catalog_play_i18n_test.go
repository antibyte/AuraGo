package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCatalogPlayI18n(t *testing.T) {
	t.Parallel()

	teevee := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	if !strings.Contains(teevee, "throw new Error('iptv-org HTTP')") {
		t.Fatal("teevee fetchJSON must throw the iptv-org HTTP sentinel without a status")
	}
	if strings.Contains(teevee, "'iptv-org HTTP ' +") {
		t.Fatal("teevee still concatenates iptv-org HTTP with a status")
	}
	if strings.Contains(teevee, "iptv-org HTTP '") {
		t.Fatal("teevee still builds iptv-org HTTP with a trailing status fragment")
	}
	if !strings.Contains(teevee, "state.error = t('desktop.teevee_catalog_error')") {
		t.Fatal("teevee loadCatalog must localize desktop.teevee_catalog_error")
	}
	if strings.Contains(teevee, "state.error = err.message || t('desktop.teevee_catalog_error')") {
		t.Fatal("teevee catalog catch still dumps err.message")
	}

	fetchJSON := teeveeFetchJSONSource(teevee)
	if strings.Contains(fetchJSON, "t('") || strings.Contains(fetchJSON, "t(\"") {
		t.Fatal("teevee fetchJSON must not call t(); it is module-level")
	}

	radio := readDesktopAssetText(t, "js/desktop/apps/radio.js")
	if !strings.Contains(radio, "showToast(t('desktop.radio_error'))") {
		t.Fatal("radio play() must localize desktop.radio_error")
	}
	if strings.Contains(radio, "err.message || t('desktop.radio_error')") {
		t.Fatal("radio play() still dumps err.message")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.teevee_catalog_error", "desktop.radio_error"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
	}
}

func teeveeFetchJSONSource(source string) string {
	const marker = "async function fetchJSON(url, cacheMode)"
	start := strings.Index(source, marker)
	if start < 0 {
		return ""
	}
	rest := source[start:]
	next := strings.Index(rest, "\n    function joinStreamsWithChannels")
	if next < 0 {
		return rest
	}
	return rest[:next]
}

package ui

import (
	"strings"
	"testing"
)

// Every read of a plural key goes through ED.format.plural, so the forms of the locale are used.
func TestEasyDragPluralKeysAreReadThroughFormatPlural(t *testing.T) {
	for _, name := range easyDragModules {
		for i, line := range strings.Split(readDesktopAssetText(t, "js/desktop/apps/"+name), "\n") {
			for _, k := range easyDragPluralKeys {
				if strings.Contains(line, "'easydrag.ui."+k+"'") && !strings.Contains(line, "format.plural(") {
					t.Errorf("%s:%d reads easydrag.ui.%s without ED.format.plural", name, i+1, k)
				}
			}
			if name != "easydrag-core.js" && strings.Contains(line, "pluralKey(") {
				t.Errorf("%s:%d still calls pluralKey", name, i+1)
			}
		}
	}
	for _, key := range []string{"versions_stats", "versions_stats_one", "versions_stats_edge_one"} {
		for _, lang := range []string{"de", "en"} {
			if _, ok := readEasyDragLang(t, lang)["easydrag.ui."+key]; ok {
				t.Errorf("%s still has easydrag.ui.%s", lang, key)
			}
		}
	}
}

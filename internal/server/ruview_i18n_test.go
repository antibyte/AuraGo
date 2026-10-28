package server

import (
	"encoding/json"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"aurago/internal/ruview"
	"aurago/ui"
)

// readRuViewLocale loads ui/lang/ruview/<lang>.json as a flat key -> text map.
func readRuViewLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	raw, err := fs.ReadFile(ui.Content, "lang/ruview/"+lang+".json")
	if err != nil {
		t.Fatalf("read ruview locale %s: %v", lang, err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("parse ruview locale %s: %v", lang, err)
	}
	return values
}

// TestRuViewErrorCodesTranslated pins the contract between the Go enumerations
// (error codes, runtimes, phases, stop reasons) and the desktop strings: every
// value the backend can send has a text in every language, and all 16 language
// files carry exactly the keys of en.json.
func TestRuViewErrorCodesTranslated(t *testing.T) {
	families := map[string][]string{
		"ruview.err_":         ruview.ErrorCodes(),
		"ruview.state_":       ruview.Runtimes(),
		"ruview.phase_":       ruview.Phases(),
		"ruview.stop_reason_": ruview.StopReasons(),
	}
	prefixes := make([]string, 0, len(families))
	for prefix := range families {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	en := readRuViewLocale(t, "en")
	enKeys := make([]string, 0, len(en))
	for key := range en {
		enKeys = append(enKeys, key)
	}
	sort.Strings(enKeys)

	for _, lang := range strings.Fields("cs da de el en es fr hi it ja nl no pl pt sv zh") {
		values := readRuViewLocale(t, lang)
		for _, prefix := range prefixes {
			for _, value := range families[prefix] {
				if strings.TrimSpace(values[prefix+value]) == "" {
					t.Errorf("%s missing %s%s", lang, prefix, value)
				}
			}
		}
		for _, key := range enKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s missing %s", lang, key)
			}
		}
		extra := make([]string, 0)
		for key := range values {
			if _, ok := en[key]; !ok {
				extra = append(extra, key)
			}
		}
		sort.Strings(extra)
		for _, key := range extra {
			t.Errorf("%s has extra %s", lang, key)
		}
	}
}

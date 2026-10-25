package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSelfTestDashboard(t *testing.T) { runDashboardNodeCheck(t, "testdata/selftests.cjs") }

func TestSelfTestDashboardLocales(t *testing.T) {
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		b, err := os.ReadFile(filepath.Join("lang", "dashboard", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var data map[string]string
		if err := json.Unmarshal(b, &data); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"title", "desc", "read", "fixtures", "targeted", "start", "stop", "report", "budget", "bindings_hint", "case", "tool", "connections"} {
			if data["dashboard.selftest_"+key] == "" {
				t.Errorf("missing %s in %s", key, lang)
			}
		}
	}
}

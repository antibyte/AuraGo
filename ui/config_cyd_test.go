package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCYDConfigI18nKeys(t *testing.T) {
	t.Parallel()
	langs := []string{"en", "de", "fr", "es", "it", "ja", "zh", "nl", "pl", "cs", "da", "el", "hi", "no", "pt", "sv"}
	en := mustReadJSONMap(t, "lang/config/cyd/en.json")
	if len(en) == 0 {
		t.Fatal("lang/config/cyd/en.json is empty")
	}
	helpKeys := []string{
		"help.cyd.enabled",
		"help.cyd.poll_seconds",
		"help.cyd.overlay_ttl",
		"help.cyd.allow_agent_control",
		"help.cyd.mqtt_mirror",
	}
	for _, lang := range langs {
		values := mustReadJSONMap(t, "lang/config/cyd/"+lang+".json")
		for key := range en {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("lang/config/cyd/%s.json missing %s", lang, key)
			}
		}
		sec := string(mustReadUIFile(t, "lang/config/sections/"+lang+".json"))
		if !strings.Contains(sec, `"config.section.cyd.label"`) || !strings.Contains(sec, `"config.section.cyd.desc"`) {
			t.Fatalf("sections/%s.json missing CYD section keys", lang)
		}
		help := mustReadJSONMap(t, "lang/help/"+lang+".json")
		for _, key := range helpKeys {
			if strings.TrimSpace(help[key]) == "" {
				t.Fatalf("lang/help/%s.json missing %s", lang, key)
			}
		}
	}
}

func TestCYDConfigModuleWiresContentRoot(t *testing.T) {
	t.Parallel()
	src := string(mustReadUIFile(t, "cfg/cyd.js"))
	if strings.Contains(src, "getElementById('cfg-content')") || strings.Contains(src, `getElementById("cfg-content")`) {
		t.Fatal("cfg/cyd.js must render into #content, not #cfg-content")
	}
	for _, marker := range []string{
		"getElementById('content')",
		"function renderCYDSection",
		"data-path=\"cyd.enabled\"",
		"data-path=\"cyd.poll_seconds\"",
		"data-path=\"cyd.overlay_ttl_seconds\"",
		"data-path=\"cyd.allow_agent_control\"",
		"data-path=\"cyd.mqtt_mirror\"",
		"/api/cyd/status",
		"/api/cyd/test",
		"/api/tokens",
		"scopes: ['cyd']",
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("cfg/cyd.js missing %q", marker)
		}
	}
}

func mustReadJSONMap(t *testing.T, path string) map[string]string {
	t.Helper()
	raw := mustReadUIFile(t, path)
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return values
}

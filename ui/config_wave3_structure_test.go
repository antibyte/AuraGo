package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// The config sidebar follows the setup path: fixed group ids in a fixed order, every section key exactly once,
// no leftover emoji icon fields, and a translated label for every group in all sixteen languages.
func TestConfigWave3SectionGroupStructure(t *testing.T) {
	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	start := strings.Index(mainJS, "const SECTIONS = [")
	end := strings.Index(mainJS, "const lang =")
	if start < 0 || end <= start {
		t.Fatal("config main.js missing parseable SECTIONS block")
	}
	block := mainJS[start:end]

	wantIDs := []string{"models", "behavior", "speech", "tools", "tool_sources", "apps", "media", "communication",
		"productivity", "storage", "smart_home", "infrastructure", "security", "system", "danger"}
	var gotIDs []string
	for _, m := range regexp.MustCompile(`\bid:\s*'([a-z_]+)',\s*group:\s*t\('config\.group\.([a-z_]+)'\)`).FindAllStringSubmatch(block, -1) {
		if m[1] != m[2] {
			t.Fatalf("group id %q uses label key config.group.%s", m[1], m[2])
		}
		gotIDs = append(gotIDs, m[1])
	}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("SECTIONS group ids = %v, want %v", gotIDs, wantIDs)
	}
	if strings.Count(block, "group: t(") != len(wantIDs) {
		t.Fatalf("SECTIONS has %d groups, want %d", strings.Count(block, "group: t("), len(wantIDs))
	}
	if !strings.Contains(block, "id: 'danger',\n        group: t('config.group.danger'),\n        dangerGroup: true,") {
		t.Fatal("the danger group must stay last and keep dangerGroup: true")
	}
	for _, forbidden := range []string{"icon:", "customRender"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("SECTIONS must not carry %q any more", forbidden)
		}
	}

	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`key:\s*'([a-z0-9_]+)'`).FindAllStringSubmatch(block, -1) {
		if seen[m[1]] {
			t.Fatalf("section key %q appears more than once in SECTIONS", m[1])
		}
		seen[m[1]] = true
	}
	// Every key that has a lazy module or a sidebar icon slot must also be listed.
	for _, m := range regexp.MustCompile(`(?m)^    ([a-z0-9_]+): \{ m: '`).FindAllStringSubmatch(mainJS, -1) {
		if !seen[m[1]] {
			t.Errorf("SECTION_MODULES key %q is not listed in SECTIONS", m[1])
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var bundle map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/sections/"+lang+".json"), &bundle); err != nil {
			t.Fatalf("%s sections bundle: %v", lang, err)
		}
		for _, id := range wantIDs {
			if strings.TrimSpace(bundle["config.group."+id]) == "" {
				t.Errorf("%s misses config.group.%s", lang, id)
			}
		}
		if strings.TrimSpace(bundle["config.section.ruview.label"]) == "" {
			t.Errorf("%s misses config.section.ruview.label", lang)
		}
	}
}

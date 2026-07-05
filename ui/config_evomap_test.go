package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvomapConfigModuleIsWired(t *testing.T) {
	t.Parallel()

	main := readEmbeddedText(t, "js/config/main.js")
	if !strings.Contains(main, "key: 'evomap'") || !strings.Contains(main, "renderEvomapSection") {
		t.Fatalf("config main.js must register the evomap section and lazy module")
	}

	js := readEmbeddedText(t, "cfg/evomap.js")
	for _, want := range []string{
		"/api/evomap/status",
		"/api/evomap/test",
		"/api/evomap/register",
		"config.evomap.node_secret_existing",
		"config.evomap.kg_enabled",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("evomap config UI missing marker %q", want)
		}
	}
}

func TestEvomapSectionTranslationsExist(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("lang", "config", "sections", "*.json"))
	if err != nil {
		t.Fatalf("glob config section languages: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("config section language files = %d, want at least 15", len(files))
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var entries map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, key := range []string{
			"config.section.evomap.label",
			"config.section.evomap.desc",
			"config.evomap.enabled_label",
			"config.evomap.test_ok",
			"config.evomap.register_ok",
		} {
			if strings.TrimSpace(entries[key]) == "" {
				t.Fatalf("%s missing non-empty %s", file, key)
			}
		}
	}
}

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPerformanceControlHelpTranslationsCoverEveryLocale(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("lang", "help", "*.json"))
	if err != nil {
		t.Fatalf("glob help translations: %v", err)
	}
	if len(files) != 16 {
		t.Fatalf("help translations = %d files, want 16", len(files))
	}
	keys := []string{
		"help.agent.max_concurrent_loops",
		"help.circuit_breaker.final_retry_interval",
		"help.server.debug_pprof",
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}

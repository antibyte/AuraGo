package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadReadOnlyPreservesResolutionWithoutDiagnosticWrites(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	for _, content := range []string{"sqlite:\n  game_maker_path: ./custom/game.db\n", "sqlite: ["} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadReadOnly(file)
		entries, readErr := os.ReadDir(dir)
		if readErr != nil || len(entries) != 1 {
			t.Fatalf("read-only load wrote diagnostics: %v, %v", entries, readErr)
		}
		if content == "sqlite: [" {
			if err == nil {
				t.Fatal("invalid YAML accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		normal, err := Load(file)
		if err != nil || !reflect.DeepEqual(cfg, normal) {
			t.Fatalf("read-only resolution differs: %v", err)
		}
	}
}

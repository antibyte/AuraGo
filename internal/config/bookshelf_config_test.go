package config

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestBookshelfDefaultsOnAndPreservesExplicitToggle(t *testing.T) {
	t.Parallel()
	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if node := yamlLookup(template, "virtual_desktop", "bookshelf_enabled"); node == nil || node.Value != "true" {
		t.Fatal("config template must explicitly enable Bookshelf")
	}
	for _, tc := range []struct {
		name, yaml string
		want       bool
	}{
		{"template", string(template), true},
		{"section missing", "server:\n  ui_language: en\n", true},
		{"key missing", "virtual_desktop:\n  enabled: true\n", true},
		{"explicit true", "virtual_desktop:\n  bookshelf_enabled: true\n", true},
		{"explicit false", "virtual_desktop:\n  bookshelf_enabled: false\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.VirtualDesktop.BookshelfEnabled != tc.want {
				t.Fatalf("BookshelfEnabled = %v, want %v", cfg.VirtualDesktop.BookshelfEnabled, tc.want)
			}
			for _, enabled := range []bool{false, true} {
				cfg.VirtualDesktop.BookshelfEnabled = enabled
				if err := cfg.Save(path); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if node := yamlLookup(raw, "virtual_desktop", "bookshelf_enabled"); node == nil || node.Value != strconv.FormatBool(enabled) {
					t.Fatalf("Save must explicitly persist bookshelf_enabled=%v", enabled)
				}
				cfg, err = Load(path)
				if err != nil {
					t.Fatal(err)
				}
				if cfg.VirtualDesktop.BookshelfEnabled != enabled {
					t.Fatalf("saved BookshelfEnabled = %v reloaded as %v", enabled, cfg.VirtualDesktop.BookshelfEnabled)
				}
			}
		})
	}
}

package config

import (
	"path/filepath"
	"testing"
)

func TestLoadModelCatalogDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := WriteFileAtomic(configPath, []byte(`
providers:
  - id: main
    type: openai
    name: Main
    base_url: https://api.openai.com/v1
    model: gpt-4o
llm:
  provider: main
`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.ModelCatalog.Enabled {
		t.Fatal("model catalog should default to enabled")
	}
	if !cfg.ModelCatalog.CatalogOnlyVisible {
		t.Fatal("catalog-only providers should default to visible")
	}
	if len(cfg.ModelCatalog.DisabledProviders) != 0 {
		t.Fatalf("disabled providers = %v", cfg.ModelCatalog.DisabledProviders)
	}
}

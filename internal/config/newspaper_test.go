package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewspaperSearchBudgetDefaultsAndBounds(t *testing.T) {
	for _, tc := range []struct{ value, want int }{{0, 32}, {-1, 1}, {1, 1}, {32, 32}, {64, 64}, {1000, 64}} {
		if got := (NewspaperConfig{MaxSearches: tc.value}).EffectiveMaxSearches(); got != tc.want {
			t.Fatalf("%d => %d, want %d", tc.value, got, tc.want)
		}
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("newspaper:\n  enabled: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Newspaper.MaxSearches != 32 {
		t.Fatalf("legacy default=%d", cfg.Newspaper.MaxSearches)
	}
}

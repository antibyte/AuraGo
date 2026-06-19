package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGalaxaAdaptiveMusicModuleExists(t *testing.T) {
	t.Parallel()

	expectedFiles := []string{
		"galaxa-supers.js",
		"galaxa-biome-transitions.js",
		"galaxa-combo-ladder.js",
		"galaxa-adaptive-music.js",
		"galaxa-entities-core.js",
		"galaxa-entities-spawning.js",
		"galaxa-entities-behaviors.js",
		"galaxa-render-effects.js",
		"galaxa-render-stage.js",
		"galaxa-render-hud.js",
	}

	base := filepath.Join("js", "desktop", "apps")
	for _, name := range expectedFiles {
		path := filepath.Join(base, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected module file %s to exist", name)
		}
	}
}

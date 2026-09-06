package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameMakerAssetPackUIAndTranslations(t *testing.T) {
	app := readGameMakerAsset(t, "js", "desktop", "apps", "game-maker-studio.js")
	if strings.Count(app, "asset_pack_ids: state.selectedAssetPackIDs || []") != 2 {
		t.Fatal("both create and edit must send pack selection")
	}
	loader := readGameMakerAsset(t, "js", "desktop", "core", "module-loader.js")
	if !strings.Contains(loader, "/js/desktop/apps/game-maker-studio-assets.js") {
		t.Fatal("asset browser missing from lazy loader")
	}
	en, err := os.ReadFile(filepath.Join("lang", "desktop", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source map[string]string
	if err := json.Unmarshal(en, &source); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join("lang", "desktop", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 16 {
		t.Fatalf("locale count %d", len(files))
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var entries map[string]string
		if err := json.Unmarshal(data, &entries); err != nil {
			t.Fatal(err)
		}
		for key := range source {
			if strings.HasPrefix(key, "game_maker.assets") || strings.HasPrefix(key, "game_maker.pack_") {
				if strings.TrimSpace(entries[key]) == "" {
					t.Errorf("%s missing %s", file, key)
				}
			}
		}
	}
}

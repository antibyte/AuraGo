package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopMediaGalleryAssets(t *testing.T) {
	t.Parallel()

	text := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"appId === 'gallery'",
		"function renderGallery(",
		"data-gallery-tab=\"Photos\"",
		"data-gallery-tab=\"Videos\"",
		"data-gallery-rename",
		"data-gallery-delete",
		"data-gallery-download",
		"data-gallery-more",
		"GALLERY_PAGE_SIZE",
		"desktop.gallery_load_more",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("desktop shell missing media gallery marker %q", want)
		}
	}

	var hasGallery bool
	for _, app := range desktop.BuiltinApps() {
		if app.ID == "gallery" && app.Icon == "image" {
			hasGallery = true
		}
	}
	if !hasGallery {
		t.Fatalf("backend builtin apps missing gallery app: %+v", desktop.BuiltinApps())
	}
}

func TestDesktopTranslationsIncludeMediaGalleryKeys(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_gallery",
		"desktop.gallery_title",
		"desktop.gallery_photos",
		"desktop.gallery_videos",
		"desktop.gallery_refresh",
		"desktop.gallery_open",
		"desktop.gallery_download",
		"desktop.gallery_rename",
		"desktop.gallery_delete",
		"desktop.gallery_empty",
		"desktop.gallery_load_more",
		"desktop.media_open",
		"desktop.media_download",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}

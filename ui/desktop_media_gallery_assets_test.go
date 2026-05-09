package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
		if app.ID == "gallery" && app.Icon == "gallery" {
			hasGallery = true
		}
	}
	if !hasGallery {
		t.Fatalf("backend builtin apps missing gallery app: %+v", desktop.BuiltinApps())
	}
}

func TestDesktopMediaGalleryCardUsesSemanticActionsAndReadableNames(t *testing.T) {
	t.Parallel()

	text := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		`data-gallery-name`,
		`iconMarkup('gallery-action-preview', 'O', 'vd-gallery-action-icon', 16)`,
		`iconMarkup('gallery-action-download', 'D', 'vd-gallery-action-icon', 16)`,
		`iconMarkup('gallery-action-edit', 'E', 'vd-gallery-action-icon', 16)`,
		`iconMarkup('gallery-action-delete', 'X', 'vd-gallery-action-icon', 16)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("desktop gallery card missing semantic action/name marker %q", want)
		}
	}
	for _, wrong := range []string{
		`iconMarkup('folder-open', 'O', 'vd-gallery-action-icon'`,
		`iconMarkup('download', 'D', 'vd-gallery-action-icon'`,
		`iconMarkup('edit', 'E', 'vd-gallery-action-icon'`,
		`iconMarkup('trash', 'X', 'vd-gallery-action-icon'`,
	} {
		if strings.Contains(text, wrong) {
			t.Fatalf("desktop gallery action must use compact gallery action icon key, not %q", wrong)
		}
	}

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		".vd-gallery-card-meta {\n    display: grid;",
		"grid-template-columns: minmax(0, 1fr);",
		".vd-gallery-card-meta > [data-gallery-name]",
		".vd-gallery-actions {\n    display: inline-flex;",
		"justify-self: end;",
		".vd-gallery-action-icon.vd-papirus-icon",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop gallery CSS missing readable filename marker %q", want)
		}
	}
	galleryGridRule := cssRuleBlock(t, css, ".vd-gallery-grid")
	gridRowMin := cssPixelValue(t, galleryGridRule, `grid-auto-rows:\s*minmax\((\d+)px,\s*auto\);`)
	if gridRowMin < 190 || gridRowMin > 196 {
		t.Fatalf("desktop gallery grid rows must fit preview, filename, and actions without clipping: got %dpx", gridRowMin)
	}
	galleryCardRule := cssRuleBlock(t, css, ".vd-gallery-card")
	cardMin := cssPixelValue(t, galleryCardRule, `min-height:\s*(\d+)px;`)
	if cardMin < 190 || cardMin > 196 {
		t.Fatalf("desktop gallery cards must be tall enough for action row without clipping: got %dpx", cardMin)
	}
	actionIconRule := css[strings.Index(css, ".vd-gallery-action-icon.vd-papirus-icon"):]
	if idx := strings.Index(actionIconRule, "}"); idx >= 0 {
		actionIconRule = actionIconRule[:idx]
	}
	if strings.Contains(actionIconRule, "drop-shadow") {
		t.Fatalf("desktop gallery action icons must not use drop-shadow in compact buttons")
	}
}

func cssRuleBlock(t *testing.T, css, selector string) string {
	t.Helper()
	start := strings.Index(css, selector+" {")
	if start < 0 {
		t.Fatalf("desktop gallery CSS missing rule %q", selector)
	}
	end := strings.Index(css[start:], "}")
	if end < 0 {
		t.Fatalf("desktop gallery CSS rule %q is not closed", selector)
	}
	return css[start : start+end+1]
}

func cssPixelValue(t *testing.T, css, pattern string) int {
	t.Helper()
	matches := regexp.MustCompile(pattern).FindStringSubmatch(css)
	if len(matches) != 2 {
		t.Fatalf("desktop gallery CSS missing pixel rule %q", pattern)
	}
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		t.Fatalf("parse CSS pixel value %q: %v", matches[1], err)
	}
	return value
}

func TestDesktopMediaGalleryActionIconsExistInBothThemes(t *testing.T) {
	t.Parallel()

	for _, theme := range []string{"papirus", "whitesur"} {
		manifest := rawDesktopAssetText(t, "img/"+theme+"/manifest.json")
		for _, key := range []string{"gallery-action-preview", "gallery-action-download", "gallery-action-edit", "gallery-action-delete"} {
			if !strings.Contains(manifest, `"`+key+`"`) {
				t.Fatalf("%s theme manifest missing gallery action icon key %q", theme, key)
			}
		}
	}
}

func TestDesktopMediaGalleryActionIconsAreCompactSVGs(t *testing.T) {
	t.Parallel()

	for _, theme := range []string{"papirus", "whitesur"} {
		for _, name := range []string{"gallery-action-preview", "gallery-action-download", "gallery-action-edit", "gallery-action-delete"} {
			svg := rawDesktopAssetText(t, "img/"+theme+"/icons/"+name+".svg")
			if strings.Contains(svg, "<image") || strings.Contains(svg, "base64,") {
				t.Fatalf("%s %s must be a compact action SVG, not an embedded bitmap app icon", theme, name)
			}
			if !strings.Contains(svg, `viewBox="0 0 16 16"`) && !strings.Contains(svg, `width="16"`) {
				t.Fatalf("%s %s must be a 16px action icon, not a large app or file-type icon", theme, name)
			}
		}
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

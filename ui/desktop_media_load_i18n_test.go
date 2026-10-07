package ui

import (
	"strings"
	"testing"
)

func TestDesktopMediaLoadI18n(t *testing.T) {
	t.Parallel()

	gallery := readDesktopAssetText(t, "js/desktop/apps/gallery-view.js")
	if !strings.Contains(gallery, "<p>${esc(t('desktop.load_failed'))}</p>") || strings.Contains(gallery, "err.message") {
		t.Fatal("Gallery errors must use translated user-facing copy")
	}

	pixel := readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js")
	if !strings.Contains(pixel, "this.t('pixel.error_load')") {
		t.Fatal("pixel image decode must localize pixel.error_load")
	}
	if strings.Contains(pixel, "Failed to load image") {
		t.Fatal("pixel still hardcodes Failed to load image")
	}

	teevee := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	if !strings.Contains(teevee, "t('desktop.teevee_catalog_error')") {
		t.Fatal("teevee catalog timeout must localize desktop.teevee_catalog_error")
	}
	if strings.Contains(teevee, "Catalog request timed out") {
		t.Fatal("teevee still hardcodes Catalog request timed out")
	}

	viewer := readDesktopAssetText(t, "js/desktop/apps/viewer-3d.js")
	if !strings.Contains(viewer, "throw new Error(t('viewer.error'))") {
		t.Fatal("viewer 3d missing STLLoader must throw viewer.error")
	}
	if strings.Contains(viewer, "throw new Error('Three.js STLLoader is unavailable')") {
		t.Fatal("viewer 3d still throws hardcoded STLLoader English")
	}
	if !strings.Contains(viewer, "Three.js STLLoader is unavailable") {
		t.Fatal("viewer 3d must still map the English STLLoader sentinel")
	}
}

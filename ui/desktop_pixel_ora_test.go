package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The OpenRaster project format (pixel-ora.js) is wired into the existing Pixel open/save entry points by extension.
// The format itself is covered by scripts/test-pixel-ora.mjs and internal/server/pixel_ora_test.go.

func TestPixelOraIsAnImageExtensionAndOpensThroughTheProjectLoader(t *testing.T) {
	state := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-state.js"))
	list := regexp.MustCompile(`const IMAGE_EXTS = \[([^\]]*)\]`).FindStringSubmatch(state)
	if list == nil || !strings.Contains(list[1], "'ora'") {
		t.Fatalf("IMAGE_EXTS must include 'ora' so the open dialog and drag & drop accept projects: %v", list)
	}

	actions := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-actions.js"))
	load := jsFunctionBodyInWindowMenuTest(t, actions, "loadDesktopImagePath: Pixel.bindRuntime(runtime, async function loadDesktopImagePath(")
	dispatch := strings.Index(load, "this.loadOraProject(nextPath, options)")
	if dispatch < 0 || !strings.Contains(load[:dispatch], `/\.ora$/i.test(nextPath)`) {
		t.Fatalf("loadDesktopImagePath must hand .ora paths to loadOraProject: %s", load)
	}
	if preview := strings.Index(load, "/api/desktop/preview"); preview < dispatch {
		t.Fatalf("the .ora dispatch must come before the image preview request (preview at %d, dispatch at %d)", preview, dispatch)
	}

	open := jsFunctionBodyInWindowMenuTest(t, actions, "openFile: Pixel.bindRuntime(runtime, async function openFile()")
	if !strings.Contains(open, "extensions: ['ora']") || !strings.Contains(open, "desktop.file_dialog_ora") {
		t.Fatalf("the open dialog should offer an OpenRaster file type: %s", open)
	}

	photos := jsFunctionBodyInWindowMenuTest(t, actions, "loadPhotos: Pixel.bindRuntime(runtime, async function loadPhotos()")
	if !strings.Contains(photos, "ext !== 'ora'") {
		t.Fatalf("the photo thumbnails request raster previews, which .ora projects do not have: %s", photos)
	}
}

func TestPixelSaveDispatchesOraProjectsAndFlattensOtherFormats(t *testing.T) {
	actions := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-actions.js"))
	save := jsFunctionBodyInWindowMenuTest(t, actions, "saveFile: Pixel.bindRuntime(runtime, async function saveFile(destination)")
	for _, want := range []string{
		"ext === 'ora'",
		"await this.oraDataURL()",
		"format: isORA ? 'ora' : (isJPEG ? 'jpeg' : 'png')",
		"this.flattenToDataURL('image/png')",
		"this.flattenToDataURL('image/jpeg', 0.92)",
		"this.noteFlattenedLayers()",
		"this.oraErrorMessage(err, 'save')",
		"/api/pixel/save",
		"'If-None-Match': '*'",
	} {
		if !strings.Contains(save, want) {
			t.Fatalf("saveFile is missing %q", want)
		}
	}
	if !strings.Contains(save, "/^(Photos|Music|Videos|AuraGo Documents)\\//i.test(path)") {
		t.Fatal("saveFile must keep sending media-root paths to Save As")
	}

	saveAs := jsFunctionBodyInWindowMenuTest(t, actions, "saveFileAs: Pixel.bindRuntime(runtime, async function saveFileAs()")
	for _, want := range []string{"initialPath: 'Pictures'", "desktop.file_dialog_png", "desktop.file_dialog_jpeg", "desktop.file_dialog_ora", "extensions: ['ora']", "defaultExtension: filters[0].extensions[0]"} {
		if !strings.Contains(saveAs, want) {
			t.Fatalf("saveFileAs is missing %q", want)
		}
	}
}

func TestPixelOraModuleSurface(t *testing.T) {
	source := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-ora.js"))
	for _, want := range []string{
		"Pixel.registerModule('ora'",
		"Pixel.ora = {",
		"'image/openraster'",
		"version=\"0.0.5\"",
		"aurago:locked-pixels", "aurago:locked-alpha", "aurago:locked-position", "aurago:clip", "aurago:mask=", "aurago:mask-enabled",
		"/api/desktop/download?path=",
		"data:application/zip;base64,",
		"Thumbnails/thumbnail.png", "mergedimage.png",
		"DecompressionStream('deflate-raw')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("pixel-ora.js is missing %q", want)
		}
	}
	for _, name := range []string{"write", "read"} {
		if !regexp.MustCompile(`\b` + name + `,?\n`).MatchString(source[strings.Index(source, "Pixel.ora = {"):]) {
			t.Fatalf("Pixel.ora must expose %s()", name)
		}
	}
}

// The server rejects archives it cannot trust; the client must not offer what the server would refuse.
func TestPixelOraClientLimitsMatchTheServer(t *testing.T) {
	source := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-ora.js"))
	serverBytes, err := os.ReadFile(filepath.Join("..", "internal", "server", "pixel_ora.go"))
	if err != nil {
		t.Fatal(err)
	}
	server := string(serverBytes)
	for client, serverConst := range map[string]string{
		"entries: 128":                  `pixelORAMaxEntries\s*=\s*128\b`,
		"xmlBytes: 1024 * 1024":         `pixelORAMaxXMLBytes\s*=\s*1 << 20\b`,
		"totalBytes: 512 * 1024 * 1024": `pixelORAMaxTotalBytes\s*=\s*512 << 20\b`,
		"side: 16384":                   `pixelORAMaxSide\s*=\s*16384\b`,
	} {
		if !strings.Contains(source, client) {
			t.Fatalf("pixel-ora.js lost limit %q", client)
		}
		if !regexp.MustCompile(serverConst).MatchString(server) {
			t.Fatalf("internal/server/pixel_ora.go no longer matches the client limit %q", client)
		}
	}
	if !regexp.MustCompile(`pixelORAMimetype\s*=\s*"image/openraster"`).MatchString(server) || !strings.Contains(server, "urn:aurago:openraster") || !strings.Contains(source, "urn:aurago:openraster") {
		t.Fatal("client and server must agree on the mimetype and the AuraGo XML namespace")
	}
}

// desktop.file_dialog_ora is added to the language files by the final i18n pass. Until then this is a no-op; once any
// language has the key, every language must have it.
func TestDesktopPixelOraFilterLabelIsCompleteOnceTranslated(t *testing.T) {
	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	with := 0
	missing := []string{}
	for _, lang := range languages {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.TrimSpace(values["desktop.file_dialog_ora"]) != "" {
			with++
		} else {
			missing = append(missing, lang)
		}
	}
	if with > 0 && len(missing) > 0 {
		t.Fatalf("desktop.file_dialog_ora is missing in %v", missing)
	}
}

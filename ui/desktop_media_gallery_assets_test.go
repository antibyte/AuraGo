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

var desktopGalleryLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func TestDesktopMediaGalleryAssets(t *testing.T) {
	t.Parallel()

	// The main bundle only keeps a thin delegator; the app lives in lazy modules.
	text := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"appId === 'gallery'",
		"renderGallery(id, context)",
		"async function renderGallery(id, context)",
		"function galleryAppContext(context)",
		"window.GalleryApp",
		"app.render(host, id, ctx)",
		"afterFileChange: refreshDesktopAfterFileChange",
		"pageSize: GALLERY_PAGE_SIZE",
		"readonly: desktopReadonly()",
		"gallery: 'GalleryApp'",
		"function openMediaLightbox(file)",
		"openMediaLightbox(file).then(opened =>",
		"function openLegacyMediaPreview(file, kind, previewURL)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("desktop shell missing media gallery marker %q", want)
		}
	}
	if strings.Contains(text, "data-gallery-tab=\"Photos\"") {
		t.Fatal("legacy inline gallery markup must not remain in the main bundle")
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	order := []string{
		"'/js/desktop/apps/gallery-library.js'",
		"'/js/desktop/apps/gallery-view.js'",
		"'/js/desktop/apps/gallery-menus.js'",
		"'/js/desktop/apps/gallery-lightbox.js'",
		"'/js/desktop/apps/gallery.js'",
	}
	last := -1
	for _, want := range order {
		idx := strings.Index(loader, want)
		if idx < 0 {
			t.Fatalf("module loader missing gallery script %q", want)
		}
		if idx < last {
			t.Fatalf("gallery scripts must load helpers before gallery.js; %q is out of order", want)
		}
		last = idx
	}

	app := readDesktopAssetText(t, "js/desktop/apps/gallery.js")
	for _, want := range []string{
		"window.GalleryApp = {",
		"function render(host, windowId, context)",
		"function dispose(windowId)",
		"registerWindowCleanup(windowId, g.dispose)",
		"recursive=true&limit=",
		"data-gallery-item",
		"data-gallery-rename",
		"data-gallery-delete",
		"data-gallery-download",
		"data-gallery-more",
		"desktop.gallery_load_more",
		"AuraSSE.on('virtual_desktop_event'",
		"AuraSSE.off('virtual_desktop_event'",
		"visibilitychange",
		"files.confirm_delete",
		"desktopSound('file.delete')",
		"lightbox.open({",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("gallery app module missing marker %q", want)
		}
	}
	for _, path := range []string{
		"js/desktop/apps/gallery.js",
		"js/desktop/apps/gallery-view.js",
		"js/desktop/apps/gallery-menus.js",
		"js/desktop/apps/gallery-library.js",
		"js/desktop/apps/gallery-lightbox.js",
	} {
		source := readDesktopAssetText(t, path)
		for _, forbidden := range []string{"alert(", "window.confirm(", "window.prompt("} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s must use desktop dialogs, not %q", path, forbidden)
			}
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

	view := readDesktopAssetText(t, "js/desktop/apps/gallery-view.js")
	tile := jsFunctionBodyInWindowMenuTest(t, view, "function tileHTML(v, item, index, kind, selected, url)")
	for _, want := range []string{
		`data-gallery-name`,
		`class="vd-gallery-card-name"`,
		`role="option"`,
		`aria-label="${esc(item.name || '')}"`,
		`iconMarkup('gallery-action-download', 'D', ICON, 16)`,
		`iconMarkup('gallery-action-edit', 'E', ICON, 16)`,
		`iconMarkup('gallery-action-delete', 'X', ICON, 16)`,
		`data-gallery-toggle`,
		`role="checkbox"`,
		`data-gallery-duration`,
		`loading="lazy"`,
		`preload="none"`,
	} {
		if !strings.Contains(tile, want) {
			t.Fatalf("desktop gallery tile missing semantic action/name marker %q", want)
		}
	}
	if !strings.Contains(view, "const ICON = 'vd-gallery-action-icon';") {
		t.Fatal("gallery view must render action icons with the compact vd-gallery-action-icon role")
	}
	for _, wrong := range []string{
		`iconMarkup('folder-open', 'O', ICON`,
		`iconMarkup('download', 'D', ICON`,
		`iconMarkup('edit', 'E', ICON`,
		`iconMarkup('trash', 'X', ICON`,
	} {
		if strings.Contains(view, wrong) {
			t.Fatalf("desktop gallery action must use compact gallery action icon key, not %q", wrong)
		}
	}

	css := strings.ReplaceAll(readAllDesktopCSS(t), "\r\n", "\n")
	for _, want := range []string{
		".vd-gallery-card {",
		"aspect-ratio: 1 / 1;",
		"grid-template-columns: repeat(auto-fill, minmax(var(--vd-gallery-tile), 1fr));",
		".vd-gallery-card-meta {",
		".vd-gallery-card-name {",
		"text-overflow: ellipsis;",
		".vd-gallery-actions {\n    display: inline-flex;",
		".vd-gallery-action-icon.vd-papirus-icon",
		".vd-gallery-card.is-selected",
		".vd-gallery-card:focus-visible",
		".vd-gallery-section-header {\n    position: sticky;",
		"@media (hover: hover)",
		"@media (hover: none)",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop gallery CSS missing readable filename/tile marker %q", want)
		}
	}
	infoRule := cssRuleBlock(t, css, ".vd-gallery-info")
	infoWidth := cssPixelValue(t, infoRule, `width:\s*(\d+)px;`)
	if infoWidth < 240 || infoWidth > 340 {
		t.Fatalf("desktop gallery details panel must stay readable but compact: got %dpx", infoWidth)
	}
	checkRule := cssRuleBlock(t, css, ".vd-gallery-check")
	checkSize := cssPixelValue(t, checkRule, `width:\s*(\d+)px;`)
	if checkSize < 20 || checkSize > 28 {
		t.Fatalf("desktop gallery selection checkbox must be a compact touch target: got %dpx", checkSize)
	}
	actionIconRule := css[strings.Index(css, ".vd-gallery-action-icon.vd-papirus-icon"):]
	if idx := strings.Index(actionIconRule, "}"); idx >= 0 {
		actionIconRule = actionIconRule[:idx]
	}
	if strings.Contains(actionIconRule, "drop-shadow") {
		t.Fatalf("desktop gallery action icons must not use drop-shadow in compact buttons")
	}
}

func TestDesktopMediaGalleryLightboxContract(t *testing.T) {
	t.Parallel()

	lightbox := readDesktopAssetText(t, "js/desktop/apps/gallery-lightbox.js")
	for _, want := range []string{
		"window.GalleryLightbox = {",
		"root.setAttribute('role', 'dialog');",
		"root.setAttribute('aria-modal', 'true');",
		"data-lb-prev",
		"data-lb-next",
		"data-lb-close",
		"data-lb-info",
		"data-lb-slideshow",
		"data-lb-zoom-in",
		"data-lb-zoom-out",
		"data-lb-strip",
		"event.stopPropagation();",
		"key === 'Escape'",
		"key === 'ArrowRight'",
		"key === 'ArrowLeft'",
		"vd-lightbox-open",
		"setItems",
	} {
		if !strings.Contains(lightbox, want) {
			t.Fatalf("gallery lightbox missing marker %q", want)
		}
	}

	css := strings.ReplaceAll(readAllDesktopCSS(t), "\r\n", "\n")
	for _, want := range []string{
		".vd-lightbox {",
		"z-index: var(--vd-z-media-preview, 950);",
		".vd-lightbox-backdrop {",
		".vd-lightbox-canvas.is-zoomed {",
		".vd-lightbox-strip {",
		".vd-lightbox-thumb.is-current {",
		".vd-lightbox.is-idle .vd-lightbox-bar,",
		".vd-lightbox.no-animations",
		".vd-media-preview-backdrop {",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop gallery lightbox CSS missing marker %q", want)
		}
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
		// Library UI
		"desktop.gallery_library",
		"desktop.gallery_search_placeholder",
		"desktop.gallery_search_clear",
		"desktop.gallery_sort",
		"desktop.gallery_sort_newest",
		"desktop.gallery_sort_oldest",
		"desktop.gallery_sort_name",
		"desktop.gallery_sort_size",
		"desktop.gallery_group_by_date",
		"desktop.gallery_group_today",
		"desktop.gallery_group_yesterday",
		"desktop.gallery_group_this_week",
		"desktop.gallery_group_unknown",
		"desktop.gallery_tile_size",
		"desktop.gallery_tile_smaller",
		"desktop.gallery_tile_larger",
		"desktop.gallery_select",
		"desktop.gallery_select_done",
		"desktop.gallery_select_all",
		"desktop.gallery_select_none",
		"desktop.gallery_selected_count",
		"desktop.gallery_toggle_select",
		"desktop.gallery_info",
		"desktop.gallery_details",
		"desktop.gallery_field_name",
		"desktop.gallery_field_type",
		"desktop.gallery_field_size",
		"desktop.gallery_field_dimensions",
		"desktop.gallery_field_duration",
		"desktop.gallery_field_modified",
		"desktop.gallery_field_created",
		"desktop.gallery_field_path",
		"desktop.gallery_kind_image",
		"desktop.gallery_kind_video",
		"desktop.gallery_kind_audio",
		"desktop.gallery_edit_pixel",
		"desktop.gallery_show_in_files",
		"desktop.gallery_copy_link",
		"desktop.gallery_link_copied",
		"desktop.gallery_status_summary",
		"desktop.gallery_status_summary_one",
		"desktop.gallery_status_results",
		"desktop.gallery_status_results_one",
		"desktop.gallery_item_count",
		"desktop.gallery_item_count_one",
		"desktop.gallery_delete_failed_one",
		"desktop.gallery_loading_library",
		"desktop.gallery_no_results",
		"desktop.gallery_no_results_hint",
		"desktop.gallery_empty_photos",
		"desktop.gallery_empty_videos",
		"desktop.gallery_empty_hint",
		"desktop.gallery_error_title",
		"desktop.gallery_delete_one_title",
		"desktop.gallery_delete_many_title",
		"desktop.gallery_delete_msg",
		"desktop.gallery_delete_progress",
		"desktop.gallery_delete_failed",
		"desktop.gallery_deleted",
		"desktop.gallery_download_progress",
		"desktop.gallery_readonly_hint",
		"desktop.gallery_new_items",
		"desktop.gallery_show_new",
		"desktop.gallery_thumbnail_failed",
		// Lightbox
		"desktop.gallery_zoom_fit",
		"desktop.gallery_zoom_actual",
		"desktop.gallery_zoom_in",
		"desktop.gallery_zoom_out",
		"desktop.gallery_slideshow_start",
		"desktop.gallery_slideshow_stop",
		"desktop.gallery_prev",
		"desktop.gallery_next",
		"desktop.gallery_counter",
		"desktop.gallery_lightbox_label",
		"desktop.gallery_lightbox_failed",
	}
	placeholders := map[string][]string{
		"desktop.gallery_selected_count":    {"{{count}}"},
		"desktop.gallery_status_summary":     {"{{count}}", "{{size}}"},
		"desktop.gallery_status_summary_one": {"{{size}}"},
		"desktop.gallery_status_results":     {"{{count}}"},
		"desktop.gallery_item_count":         {"{{count}}"},
		"desktop.gallery_loading_library":   {"{{count}}"},
		"desktop.gallery_no_results":        {"{{query}}"},
		"desktop.gallery_delete_one_title":  {"{{name}}"},
		"desktop.gallery_delete_many_title": {"{{count}}"},
		"desktop.gallery_delete_progress":   {"{{done}}", "{{total}}"},
		"desktop.gallery_download_progress": {"{{done}}", "{{total}}"},
		"desktop.gallery_delete_failed":     {"{{count}}"},
		"desktop.gallery_deleted":           {"{{count}}"},
		"desktop.gallery_new_items":         {"{{count}}"},
		"desktop.gallery_counter":           {"{{index}}", "{{total}}"},
	}
	english := map[string]string{}
	for _, lang := range desktopGalleryLocales {
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
			value := strings.TrimSpace(values[key])
			if value == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
			for _, placeholder := range placeholders[key] {
				if !strings.Contains(value, placeholder) {
					t.Fatalf("%s translation for %s must keep placeholder %s: %q", path, key, placeholder, value)
				}
			}
			if lang == "en" {
				english[key] = value
			}
		}
	}
	// Spot-check that the German locale is not an English copy and uses real umlauts / the personal form.
	de := map[string]string{}
	data, err := os.ReadFile(filepath.Join("lang", "desktop", "de.json"))
	if err != nil {
		t.Fatalf("read de.json: %v", err)
	}
	if err := json.Unmarshal(data, &de); err != nil {
		t.Fatalf("parse de.json: %v", err)
	}
	for _, key := range []string{"desktop.gallery_delete", "desktop.gallery_search_placeholder", "desktop.gallery_group_this_week", "desktop.gallery_delete_msg"} {
		if de[key] == english[key] {
			t.Fatalf("de.json copies the English string for %s: %q", key, de[key])
		}
	}
	if strings.Contains(de["desktop.gallery_delete"], "oe") || strings.Contains(de["desktop.gallery_delete"], "Loeschen") {
		t.Fatalf("de.json must use real umlauts: %q", de["desktop.gallery_delete"])
	}
	for _, key := range keys {
		value := de[key]
		if strings.Contains(value, " Sie ") || strings.HasPrefix(value, "Sie ") {
			t.Fatalf("de.json must use the personal form (Du), not Sie: %s = %q", key, value)
		}
	}
}

package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWidgetPinContract(t *testing.T) {
	t.Parallel()

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"function widgetIsPinned(widget)",
		"widget.config.pinned",
		"function updateWidgetPinBadge(card, pinned)",
		"vd-widget-pin",
		"vd-widget-pinned",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop widget pin contract missing marker %q", want)
		}
	}

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"async function toggleWidgetPinned(widget)",
		"if (widgetIsPinned(card._widgetData || widget)) return;",
		"if (widgetIsPinned(widget)) return;",
		"pinned: nextPinned",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("desktop widget pin persistence missing marker %q", want)
		}
	}
	drag := jsFunctionBodyInWindowMenuTest(t, shell, "function wireDraggableWidget(card, widget)")
	if !strings.Contains(drag, "widgetIsPinned") {
		t.Fatal("a pinned widget must never start a drag")
	}
	bounds := jsFunctionBodyInWindowMenuTest(t, shell, "async function persistWidgetBounds(widget, card)")
	if !strings.Contains(bounds, "widgetIsPinned") {
		t.Fatal("a pinned widget must never store a new position")
	}

	menus := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	menu := jsFunctionBodyInWindowMenuTest(t, menus, "function showWidgetContextMenu(event, widget)")
	for _, want := range []string{
		"desktop.widget_pin",
		"desktop.widget_unpin",
		"icon: pinned ? 'check-square' : 'square'",
		"disabled: desktopReadonly()",
		"action: () => toggleWidgetPinned(widget)",
	} {
		if !strings.Contains(menu, want) {
			t.Fatalf("widget context menu pin toggle missing marker %q", want)
		}
	}

	sticky := readDesktopAssetText(t, "js/desktop/core/sticky-notes-runtime.js")
	stickyMenu := jsFunctionBodyInWindowMenuTest(t, sticky, "function showStickyNoteMenu(x, y, widget)")
	for _, want := range []string{
		"desktop.widget_pin",
		"desktop.widget_unpin",
		"action: () => toggleWidgetPinned(widget)",
	} {
		if !strings.Contains(stickyMenu, want) {
			t.Fatalf("sticky note menu pin toggle missing marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-widgets.css")
	for _, want := range []string{".vd-widget.vd-widget-pinned", ".vd-widget-pin", ".vd-widget-pin-icon"} {
		if !strings.Contains(css, want) {
			t.Fatalf("widget pin styles missing marker %q", want)
		}
	}
}

func TestDesktopWidgetPinTranslations(t *testing.T) {
	t.Parallel()

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{"desktop.widget_pin", "desktop.widget_unpin", "desktop.widget_pinned"}
	bundles := map[string]map[string]string{}
	for _, lang := range langs {
		raw := rawDesktopAssetText(t, filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
		values := map[string]string{}
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s is missing %q", lang, key)
			}
		}
		bundles[lang] = values
	}
	english := bundles["en"]
	for _, lang := range langs {
		if lang == "en" {
			continue
		}
		for _, key := range keys {
			if bundles[lang][key] == english[key] {
				t.Errorf("%s copies the English text for %q", lang, key)
			}
		}
	}
}

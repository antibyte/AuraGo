package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopLoadFailedI18n(t *testing.T) {
	t.Parallel()

	calendar := readDesktopAssetText(t, "js/desktop/apps/calendar.js")
	if !strings.Contains(calendar, "t('desktop.load_failed')") {
		t.Fatal("calendar empty state must localize desktop.load_failed")
	}
	if strings.Contains(calendar, ".vd-calendar-body').innerHTML = `<div class=\"vd-empty\">${esc(err.message)}</div>`") {
		t.Fatal("calendar empty state still dumps err.message")
	}

	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	if strings.Count(source, "t('desktop.load_failed')") < 3 {
		t.Fatal("todo, gallery, and quick connect device list must localize desktop.load_failed")
	}
	for _, forbidden := range []string{
		".vd-todo-list').innerHTML = `<div class=\"vd-empty\">${esc(err.message)}</div>`",
		"grid.innerHTML = `<div class=\"vd-empty\">${esc(err.message)}</div>`",
		"deviceList.innerHTML = `<div class=\"vd-empty\">${esc(err.message)}</div>`",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("planning gallery still dumps err.message via %q", forbidden)
		}
	}

	generated := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	if !strings.Contains(generated, "t('desktop.load_failed')") {
		t.Fatal("generated-app host empty state must localize desktop.load_failed")
	}
	if strings.Contains(generated, "host.innerHTML = `<div class=\"vd-empty\">${esc(err.message)}</div>`") {
		t.Fatal("generated-app host still dumps err.message")
	}

	people := readDesktopAssetText(t, "js/desktop/apps/people.js")
	if !strings.Contains(people, "t(inst.context, 'desktop.load_failed')") {
		t.Fatal("people empty state must localize desktop.load_failed")
	}
	if strings.Contains(people, "vd-people-empty-title\">${esc(err.message)}") {
		t.Fatal("people empty state still dumps err.message")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.load_failed"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.load_failed", path)
		}
		if lang == "de" && got == "Could not load this view." {
			t.Fatalf("%s must not copy the English load-failed string", path)
		}
		if lang == "fr" && got == "Could not load this view." {
			t.Fatalf("%s must not copy the English load-failed string", path)
		}
	}
}

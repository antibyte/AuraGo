package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPeopleDaysI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/people.js")
	for _, want := range []string{
		"t(inst.context, 'desktop.people_days_until_birthday', { days })",
		"t(inst.context, 'desktop.people_today')",
		"t(inst.context, 'desktop.people_tomorrow')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("people days i18n missing marker %q", want)
		}
	}
	if strings.Count(source, "t(inst.context, 'desktop.people_days_until_birthday', { days })") < 4 {
		t.Fatal("people must use desktop.people_days_until_birthday in sidebar, cards, list, and detail")
	}
	for _, forbidden := range []string{
		"days + 'd'",
		"+ ' days)'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("people still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.people_days_until_birthday"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.people_days_until_birthday", path)
		}
		if !strings.Contains(got, "{{days}}") {
			t.Fatalf("%s desktop.people_days_until_birthday must keep {{days}}", path)
		}
		if lang == "de" && got == "{{days}} days" {
			t.Fatalf("%s must not copy the English birthday-days string", path)
		}
	}
}

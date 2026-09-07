package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopNotifyRequestI18n(t *testing.T) {
	t.Parallel()

	people := readDesktopAssetText(t, "js/desktop/apps/people.js")
	if strings.Count(people, "t(inst.context, 'desktop.request_failed')") < 2 {
		t.Fatal("people save/delete must localize desktop.request_failed")
	}
	if strings.Contains(people, "message: err.message") {
		t.Fatal("people notifies still dump err.message")
	}

	notes := readDesktopAssetText(t, "js/desktop/apps/notes.js")
	if !strings.Contains(notes, "state.t('desktop.request_failed')") {
		t.Fatal("notes notifyError must localize desktop.request_failed")
	}
	if !strings.Contains(notes, "existsErr.notesCode = 'rename_exists'") {
		t.Fatal("notes rename conflict must set notesCode")
	}
	if !strings.Contains(notes, "err.notesCode === 'rename_exists'") {
		t.Fatal("notes notifyError must keep the rename conflict message")
	}
	if strings.Contains(notes, "message: (err && err.message) || String(err)") {
		t.Fatal("notes notifyError still dumps err.message")
	}

	radio := readDesktopAssetText(t, "js/desktop/apps/radio.js")
	if !strings.Contains(radio, "function resumePlayback()") || !strings.Contains(radio, "showToast(t('desktop.radio_error'))") {
		t.Fatal("radio toggle play must localize desktop.radio_error")
	}
	if strings.Contains(radio, "err.message || String(err)") {
		t.Fatal("radio toggle play still dumps err.message")
	}

	english := "The request failed."
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.request_failed"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.request_failed", path)
		}
		if (lang == "de" || lang == "fr") && got == english {
			t.Fatalf("%s must not copy the English request failed string", path)
		}
	}
}

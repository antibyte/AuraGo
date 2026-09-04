package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFileManagerNewFolderI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager/actions-operations.js")
	if !strings.Contains(source, "promptDialog(t('desktop.fm.new_folder_prompt'), t('desktop.fm.new_folder'))") {
		t.Fatal("new folder prompt must use desktop.fm.new_folder as the default name")
	}
	if strings.Contains(source, "'New Folder'") {
		t.Fatal("file manager still hardcodes English New Folder")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.fm.new_folder"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.fm.new_folder", path)
		}
		if lang == "de" && got == "New Folder" {
			t.Fatalf("%s must not copy the English new folder name", path)
		}
	}
}

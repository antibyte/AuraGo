package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopShellPromptDefaultsI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"promptDialog(t('desktop.new_file'), t('desktop.new_file_default'))",
		"promptDialog(t('desktop.new_folder'), t('desktop.new_folder'))",
		"workspaceJoinPath(state.filesPath, 'untitled.txt')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("shell prompt default i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"promptDialog(t('desktop.new_file'), 'untitled.txt')",
		"promptDialog(t('desktop.new_folder'), 'New Folder')",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("shell still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.new_file_default"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.new_file_default", path)
		}
		if !strings.HasSuffix(got, ".txt") {
			t.Fatalf("%s desktop.new_file_default must stay a .txt filename", path)
		}
		if strings.ContainsAny(got, `/\`) {
			t.Fatalf("%s desktop.new_file_default must not contain path separators", path)
		}
		if lang == "de" && got == "untitled.txt" {
			t.Fatalf("%s must not copy the English untitled filename", path)
		}
		folder := values["desktop.new_folder"]
		if strings.TrimSpace(folder) == "" {
			t.Fatalf("%s missing non-empty desktop.new_folder", path)
		}
		if lang == "de" && folder == "New folder" {
			t.Fatalf("%s must not copy the English new-folder string", path)
		}
	}
}

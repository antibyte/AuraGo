package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFileManagerNewFileDefaultI18n(t *testing.T) {
	t.Parallel()

	operations := readDesktopAssetText(t, "js/desktop/file-manager/actions-operations.js")
	if !strings.Contains(operations, "promptDialog(t('desktop.fm.new_file_prompt'), t('desktop.new_file_default'))") {
		t.Fatal("file manager new-file prompt must use desktop.new_file_default")
	}
	if strings.Contains(operations, "'new-file.txt'") {
		t.Fatal("file manager operations still hardcode new-file.txt")
	}

	templates := readDesktopAssetText(t, "js/desktop/file-manager/advanced-actions.js")
	if !strings.Contains(templates, `value="${esc(t('desktop.new_file_default'))}"`) {
		t.Fatal("file manager template dialog must use desktop.new_file_default")
	}
	if strings.Contains(templates, `value="new-file.txt"`) {
		t.Fatal("file manager template dialog still hardcodes new-file.txt")
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
		if lang == "de" && got == "untitled.txt" {
			t.Fatalf("%s must not copy the English untitled filename", path)
		}
	}
}

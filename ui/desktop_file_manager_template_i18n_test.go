package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFileManagerTemplateI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager/advanced-actions.js")
	for _, want := range []string{
		"t('desktop.fm.zip_created')",
		"t('desktop.fm.zip_extracted')",
		"t('desktop.fm.batch_rename_success')",
		"t('desktop.fm.new_file_template_label')",
		"desktop.fm.new_file_kind_txt",
		"desktop.fm.new_file_kind_md",
		"desktop.fm.new_file_kind_py",
		"desktop.fm.new_file_kind_go",
		"desktop.fm.new_file_kind_js",
		"desktop.fm.new_file_kind_json",
		"desktop.fm.new_file_kind_yaml",
		"desktop.fm.new_file_kind_sh",
		"desktop.fm.new_file_kind_html",
		"desktop.fm.new_file_kind_css",
		"esc(t(item.labelKey))",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("file manager template i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"label: 'Plain Text'",
		">Template</label>",
		"ZIP created successfully",
		"ZIP extracted successfully",
		"Files renamed successfully",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("file manager still hardcodes %q", forbidden)
		}
	}

	required := []string{
		"desktop.fm.batch_rename_success",
		"desktop.fm.new_file_template_label",
		"desktop.fm.new_file_kind_css",
		"desktop.fm.new_file_kind_go",
		"desktop.fm.new_file_kind_html",
		"desktop.fm.new_file_kind_js",
		"desktop.fm.new_file_kind_json",
		"desktop.fm.new_file_kind_md",
		"desktop.fm.new_file_kind_py",
		"desktop.fm.new_file_kind_sh",
		"desktop.fm.new_file_kind_txt",
		"desktop.fm.new_file_kind_yaml",
		"desktop.fm.zip_created",
		"desktop.fm.zip_extracted",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if lang == "de" {
			if values["desktop.fm.new_file_kind_txt"] == "Plain Text" {
				t.Fatalf("%s must not copy the English plain text label", path)
			}
			if values["desktop.fm.new_file_template_label"] == "Template" {
				t.Fatalf("%s must not copy the English template label", path)
			}
		}
	}
}

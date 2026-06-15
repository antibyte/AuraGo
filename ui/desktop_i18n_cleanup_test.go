package ui

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopAuditedI18nKeysAndPlaceholders(t *testing.T) {
	t.Parallel()

	required := []string{
		"desktop.app_error_title",
		"desktop.sheets_clear_range",
		"desktop.sheets_insert_row_above",
		"desktop.sheets_insert_row_below",
		"desktop.sheets_insert_col_left",
		"desktop.sheets_insert_col_right",
		"desktop.sheets_delete_rows",
		"desktop.sheets_delete_columns",
		"desktop.launchpad_icon_search_placeholder",
		"desktop.launchpad_icon_url_placeholder",
		"desktop.looper_title",
		"desktop.looper_iteration",
		"desktop.search",
		"desktop.back",
		"desktop.forward",
		"desktop.settings_agent_provider",
		"desktop.settings_agent_provider_desc",
		"desktop.settings_agent_provider_default",
		"desktop.fm.upload_too_large",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
		iteration := values["desktop.looper_iteration"]
		if !strings.Contains(iteration, "{{n}}") || !strings.Contains(iteration, "{{max}}") {
			t.Fatalf("%s uses inconsistent looper iteration placeholders: %q", path, iteration)
		}
	}
}

func TestDesktopAuditedI18nUsageHasNoEnglishInlineFallbacks(t *testing.T) {
	t.Parallel()

	sheets := readDesktopAssetText(t, "js/desktop/apps/sheets.js")
	for _, forbidden := range []string{
		"t('desktop.sheets_clear_range', 'Clear contents')",
		"t('desktop.sheets_insert_row_above', 'Insert row above')",
		"t('desktop.sheets_insert_row_below', 'Insert row below')",
		"t('desktop.sheets_insert_col_left', 'Insert column left')",
		"t('desktop.sheets_insert_col_right', 'Insert column right')",
		"t('desktop.sheets_delete_rows', 'Delete selected rows')",
		"t('desktop.sheets_delete_columns', 'Delete selected columns')",
	} {
		if strings.Contains(sheets, forbidden) {
			t.Fatalf("Sheets still uses audited inline fallback %q", forbidden)
		}
	}

	looper := readDesktopAssetText(t, "js/desktop/apps/looper.js")
	for _, marker := range []string{
		"t('desktop.looper_title')",
		".replace('{{n}}', data.iteration)",
		".replace('{{max}}', data.max_iterations)",
	} {
		if !strings.Contains(looper, marker) {
			t.Fatalf("Looper missing audited i18n usage marker %q", marker)
		}
	}
	if strings.Contains(looper, "title: 'Looper'") {
		t.Fatal("Looper notifications should use a translated title")
	}

	launchpad := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"placeholder=\"${esc(t('desktop.launchpad_search'))}\"",
		"${esc(t('desktop.launchpad_all_categories'))}",
		"desktop.launchpad_icon_search_placeholder",
		"desktop.launchpad_icon_url_placeholder",
	} {
		if !strings.Contains(launchpad, marker) {
			t.Fatalf("Launchpad missing audited i18n usage marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		`placeholder="Search links..."`,
		`<option value="">All categories</option>`,
		`placeholder="plex, nginx..."`,
		`placeholder="https://..."`,
	} {
		if strings.Contains(launchpad, forbidden) {
			t.Fatalf("Launchpad still has hardcoded placeholder %q", forbidden)
		}
	}

	fileManager := readDesktopAssetText(t, "js/desktop/file-manager/core-render.js")
	for _, forbidden := range []string{
		"t('desktop.back', 'Back')",
		"t('desktop.forward', 'Forward')",
		"t('desktop.search', 'Search')",
	} {
		if strings.Contains(fileManager, forbidden) {
			t.Fatalf("File Manager still uses audited inline fallback %q", forbidden)
		}
	}

	fileInput := readDesktopAssetText(t, "js/desktop/file-manager/actions-input.js")
	if strings.Contains(fileInput, "t('desktop.fm.upload_too_large', '{{name}} exceeds the maximum upload size.'") {
		t.Fatal("File Manager upload size error still uses an audited inline English fallback")
	}
}

func TestDesktopUsedI18nKeysExistInAllLanguages(t *testing.T) {
	t.Parallel()

	keyPattern := regexp.MustCompile(`(?s)(?:\b(?:dt|desktopT|t)\s*\(\s*['"]((?:desktop|common|chat|config|help|codeStudio|viewer)\.[^'"]+)['"]|data-i18n(?:-[a-z-]+)?\s*=\s*['"]((?:desktop|common|chat|config|help|codeStudio|viewer)\.[^'"]+)['"]|(?:labelKey|titleKey|messageKey|placeholderKey)\s*:\s*['"]((?:desktop|common|chat|config|help|codeStudio|viewer)\.[^'"]+)['"])`)
	used := map[string]bool{}
	for _, asset := range desktopI18nAssets(t) {
		content := readDesktopAssetText(t, asset)
		for _, match := range keyPattern.FindAllStringSubmatch(content, -1) {
			key := ""
			for i := 1; i < len(match); i++ {
				if match[i] != "" {
					key = match[i]
					break
				}
			}
			if key == "" || strings.HasSuffix(key, "_") {
				continue
			}
			used[key] = true
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readMergedLangMap(t, lang)
		for key := range used {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing desktop-used translation key %s", lang, key)
			}
		}
	}
}

func TestDesktopBuiltinAppNamesExistInAllLanguages(t *testing.T) {
	t.Parallel()

	var keys []string
	for _, app := range desktop.BuiltinApps() {
		keys = append(keys, "desktop.app_"+strings.ReplaceAll(app.ID, "-", "_"))
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
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
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty builtin app translation for %s", path, key)
			}
		}
	}
}

func desktopI18nAssets(t *testing.T) []string {
	t.Helper()
	assets := []string{
		"desktop.html",
		"cfg/virtual_desktop.js",
	}
	if err := fs.WalkDir(Content, "js/desktop", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}
		assets = append(assets, path)
		return nil
	}); err != nil {
		t.Fatalf("walk desktop assets: %v", err)
	}
	return assets
}

func readMergedLangMap(t *testing.T, lang string) map[string]string {
	t.Helper()
	values := map[string]string{}
	if err := fs.WalkDir(Content, "lang", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != lang+".json" {
			return nil
		}
		data, err := Content.ReadFile(filepath.ToSlash(path))
		if err != nil {
			return err
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		for key, value := range raw {
			if text, ok := value.(string); ok {
				values[key] = text
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("read merged %s translations: %v", lang, err)
	}
	return values
}

func TestDesktopSettingsDescriptionsAreLocalized(t *testing.T) {
	t.Parallel()

	enPath := filepath.Join("lang", "desktop", "en.json")
	enData, err := os.ReadFile(enPath)
	if err != nil {
		t.Fatalf("read %s: %v", enPath, err)
	}
	var en map[string]string
	if err := json.Unmarshal(enData, &en); err != nil {
		t.Fatalf("parse %s: %v", enPath, err)
	}

	var keys []string
	for key := range en {
		if strings.HasPrefix(key, "desktop.settings_") && strings.HasSuffix(key, "_desc") {
			keys = append(keys, key)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
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
			if values[key] == en[key] {
				t.Fatalf("%s keeps English settings description for %s", path, key)
			}
		}
	}
}

package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var findItLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func readFindItLocale(t *testing.T, locale string) map[string]string {
	t.Helper()
	var bundle map[string]string
	if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "lang/findit/"+locale+".json")), &bundle); err != nil {
		t.Fatalf("parse FindIt locale %s: %v", locale, err)
	}
	values := make(map[string]string, len(bundle))
	for key, value := range bundle {
		if !strings.HasPrefix(key, "findit.") {
			t.Errorf("FindIt locale %s has non-namespaced or nested key %s", locale, key)
			continue
		}
		values[strings.TrimPrefix(key, "findit.")] = value
	}
	return values
}

func TestDesktopFindItLocalesAreComplete(t *testing.T) {
	english := readFindItLocale(t, "en")
	if len(english) < 800 {
		t.Fatalf("FindIt English catalog has only %d keys", len(english))
	}
	prose := 0
	for key, value := range english {
		if strings.HasSuffix(key, "_help") || strings.HasSuffix(key, "_intro") || strings.HasSuffix(key, "_note") || strings.HasSuffix(key, "_title") || strings.HasPrefix(key, "error_") {
			prose++
			if strings.Contains(value, "\uFFFD") {
				t.Errorf("English FindIt copy %s contains a replacement character", key)
			}
			words := strings.ReplaceAll(key, "_", " ")
			if strings.EqualFold(strings.TrimSpace(value), words) {
				t.Errorf("English FindIt copy %s is still a key-name placeholder", key)
			}
		}
	}
	if prose < 25 {
		t.Fatalf("FindIt English catalog exposes only %d explanatory strings", prose)
	}

	for _, locale := range findItLocales {
		if locale == "en" {
			continue
		}
		words := readFindItLocale(t, locale)
		if len(words) != len(english) {
			t.Errorf("%s has %d FindIt keys; English has %d", locale, len(words), len(english))
		}
		translatedProse := 0
		for key, source := range english {
			value, ok := words[key]
			if !ok || strings.TrimSpace(value) == "" {
				t.Errorf("%s is missing FindIt key %s", locale, key)
				continue
			}
			if strings.Contains(value, "\uFFFD") {
				t.Errorf("%s FindIt copy %s contains a replacement character", locale, key)
			}
			if strings.HasSuffix(key, "_help") || strings.HasSuffix(key, "_intro") || strings.HasSuffix(key, "_note") || strings.HasSuffix(key, "_title") || strings.HasPrefix(key, "error_") {
				if value == source {
					t.Errorf("%s leaves explanatory FindIt key %s in English", locale, key)
				} else {
					translatedProse++
				}
			}
		}
		if translatedProse < prose-2 {
			t.Errorf("%s translates only %d of %d FindIt explanatory strings", locale, translatedProse, prose)
		}
	}
}

func TestDesktopFindItCatalogAndUIContracts(t *testing.T) {
	english := readFindItLocale(t, "en")
	checks := map[string][]string{
		"js/desktop/core/module-loader.js":        {"findit:", "desktop-app-findit.css", "findit-model.js", "findit-workflows.js"},
		"js/desktop/core/menus-and-routing.js":    {"appId === 'findit'", "loadAppScript('findit')"},
		"js/desktop/core/desktop-foundation.js":   {"findit: 'FindItApp'", "params.get('item')", "params.get('location')"},
		"js/desktop/core/window-shell-runtime.js": {"findit: { width: 1320, height: 820 }", "appId === 'findit'", "location_id"},
		"js/desktop/shared/qr-dialog.js":          {"initialSpec", "onScan"},
		"js/desktop/apps/findit-model.js":         {"request_id", "quantityUnits", "packageFamilies", "atlasURL", "package_size"},
		"js/desktop/apps/findit-dialogs.js":       {"secondarySubmitLabel", "contentHTML", "AbortController"},
		"js/desktop/apps/findit-items.js":         {"stock_id", "movement_history", "icon_id", "save_and_next", "package_size", "image_id", "datasheet_urls.join('\\n')"},
		"js/desktop/apps/findit-projects.js":      {"groupActiveReservations", "location_path", "printPickList", "data-reservation-action=\"consume\""},
		"js/desktop/apps/findit-workflows.js":     {"findFindItLink", "deepLink", "location", "request_id", "TextDecoder('utf-8', { fatal: true })"},
		"js/desktop/apps/findit.js":               {"searchAll", "can_print", "ResizeObserver", "pendingInitialStock", "findit_settings", "preserveViewState", "replaceWith(nextApp)"},
		"css/desktop-app-findit.css":              {"@container", "pointer: coarse", "min-height: 44px", ".fi-pick-group"},
	}
	for path, markers := range checks {
		text := rawDesktopAssetText(t, path)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Errorf("%s is missing FindIt contract %q", path, marker)
			}
		}
	}
	findItModulePaths := []string{
		"js/desktop/apps/findit.js",
		"js/desktop/apps/findit-model.js",
		"js/desktop/apps/findit-dialogs.js",
		"js/desktop/apps/findit-items.js",
		"js/desktop/apps/findit-projects.js",
		"js/desktop/apps/findit-templates.js",
		"js/desktop/apps/findit-csv.js",
		"js/desktop/apps/findit-workflows.js",
	}
	literalTranslationKey := regexp.MustCompile(`\btr\(\s*[\x27\"]([a-z][a-z0-9_]*)[\x27\"]`)
	dynamicTranslationPrefix := regexp.MustCompile(`\btr\(\s*[\x27\"][a-z][a-z0-9_]*_[\x27\"]\s*\+`)
	formTranslationKey := regexp.MustCompile(`(?:label|title|description|help|placeholder|emptyLabel|submitLabel|confirmLabel|kicker)\s*:\s*[\x27\"]([a-z][a-z0-9_]*)[\x27\"]`)
	for _, path := range findItModulePaths {
		source := dynamicTranslationPrefix.ReplaceAllString(rawDesktopAssetText(t, path), "tr(")
		matches := append(literalTranslationKey.FindAllStringSubmatch(source, -1), formTranslationKey.FindAllStringSubmatch(source, -1)...)
		for _, match := range matches {
			if _, ok := english[match[1]]; !ok {
				t.Errorf("%s refers to missing FindIt locale key %s", path, match[1])
			}
		}
	}
	var manifest struct {
		Version int `json:"version"`
		Columns int `json:"columns"`
		Rows    int `json:"rows"`
		Icons   []struct {
			ID    string `json:"id"`
			Index int    `json:"index"`
		} `json:"icons"`
	}
	if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "img/findit/manifest.json")), &manifest); err != nil {
		t.Fatalf("parse FindIt sprite manifest: %v", err)
	}
	if manifest.Version != 1 || manifest.Columns*manifest.Rows < 64 || len(manifest.Icons) != 64 {
		t.Fatalf("FindIt sprite catalog has unexpected dimensions or icon count: %+v", manifest)
	}
	seen := map[string]bool{}
	for _, icon := range manifest.Icons {
		if icon.ID == "" || seen[icon.ID] || icon.Index < 0 || icon.Index >= manifest.Columns*manifest.Rows {
			t.Errorf("invalid FindIt sprite icon: %+v", icon)
		}
		seen[icon.ID] = true
		if _, ok := english["category_"+icon.ID]; !ok {
			t.Errorf("FindIt sprite %s has no localized label", icon.ID)
		}
	}
	fixture := string(mustReadUIFile(t, "testdata/findit-functional.js"))
	for _, marker := range []string{"initial stock", "project reservation", "CSV preview", "deep link", "photo upload"} {
		if !strings.Contains(fixture, marker) {
			t.Errorf("FindIt real-handler browser fixture is missing %q coverage", marker)
		}
	}
}

package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// i18nSafeHTMLPattern matches HTML elements that already use data-i18n attribute
// and therefore do not contain hardcoded user-visible text.
var i18nSafeHTMLPattern = regexp.MustCompile(`data-i18n\s*=`)

// highConfidenceHardcodedJS lists patterns that, when found in JS files, almost
// always represent hardcoded user-visible strings without proper i18n fallback.
var highConfidenceHardcodedJS = []struct {
	desc    string
	pattern *regexp.Regexp
}{
	// Toast/showMessage calls with hardcoded first string arg (not from t())
	{"toast_hardcoded", regexp.MustCompile(`showToast\(\s*'[^']+'\s*,`)},
	{"toast_dblquote", regexp.MustCompile(`showToast\(\s*"[^"]+"\s*,`)},
	{"alert_hardcoded", regexp.MustCompile(`alert\(\s*'[^']+'\s*\)`)},
	{"alert_dblquote", regexp.MustCompile(`alert\(\s*"[^"]+"\s*\)`)},
	{"confirm_hardcoded", regexp.MustCompile(`confirm\(\s*'[^']+'\s*\)`)},
	{"confirm_dblquote", regexp.MustCompile(`confirm\(\s*"[^"]+"\s*\)`)},
}

// isJSLineSafeForI18nCheck returns true if the line contains technical JS patterns
// that are not user-visible strings (CSS classes, API checks, etc.).
func isJSLineSafeForI18nCheck(line string) bool {
	// Skip lines that are clearly technical
	if strings.Contains(line, "classList.add") ||
		strings.Contains(line, "classList.remove") ||
		strings.Contains(line, "classList.toggle") ||
		strings.Contains(line, ".className") ||
		strings.Contains(line, "data.status") ||
		strings.Contains(line, "resp.ok") ||
		strings.Contains(line, ".dataset.") ||
		strings.Contains(line, "element.id") {
		return true
	}
	// Skip if it's just checking equality to a string
	if strings.Contains(line, "==='ok'") ||
		strings.Contains(line, "!=='ok'") ||
		strings.Contains(line, "==='error'") ||
		strings.Contains(line, "!=='error'") {
		return true
	}
	return false
}

// TestFrontend_JS_HardcodedUIToasts scans .js files in ui/js/ for high-confidence
// hardcoded user-visible strings in toast/alert/confirm calls.
// This test focuses on the most likely real problems while minimizing false positives.
func TestFrontend_JS_HardcodedUIToasts(t *testing.T) {
	t.Parallel()

	jsDir := filepath.Join("js")
	entries, err := os.ReadDir(jsDir)
	if err != nil {
		t.Skipf("ui/js/ directory not found, skipping test: %v", err)
	}

	var failures []string
	checkedFiles := 0

	// Skip directories that are known to have many technical strings
	skipDirs := map[string]bool{
		"vendor":     true,
		"codemirror": true,
		"monaco":     true,
	}

	var scanFile func(path string) error
	scanFile = func(path string) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if skipDirs[e.Name()] {
					continue
				}
				if err := scanFile(filepath.Join(path, e.Name())); err != nil {
					// skip errors for individual files
				}
			}
			return nil
		}

		if !strings.HasSuffix(path, ".js") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		checkedFiles++
		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			if isJSLineSafeForI18nCheck(line) {
				continue
			}

			for _, check := range highConfidenceHardcodedJS {
				if check.pattern.MatchString(line) {
					relPath, _ := filepath.Rel("ui", path)
					failures = append(failures, fmt.Sprintf("%s:%d: %s (line: %s)", relPath, lineNum+1, check.desc, strings.TrimSpace(line)))
				}
			}
		}
		return nil
	}

	for _, e := range entries {
		if skipDirs[e.Name()] {
			continue
		}
		scanFile(filepath.Join(jsDir, e.Name()))
	}

	if len(failures) > 0 {
		t.Errorf("Found %d high-confidence hardcoded UI strings in %d JS files:\n%s", len(failures), checkedFiles, strings.Join(failures, "\n"))
	}
}

// germanUITextPatterns German UI text patterns that should have data-i18n attribute
var germanUITextPatterns = []struct {
	desc    string
	pattern *regexp.Regexp
}{
	{"german_laden", regexp.MustCompile(`(?i)>\s*Laden`)},
	{"german_speichern", regexp.MustCompile(`(?i)>\s*Speichern`)},
	{"german_abbrechen", regexp.MustCompile(`(?i)>\s*Abbrechen`)},
	{"german_schliessen", regexp.MustCompile(`(?i)>\s*Schließen`)},
	{"german_zuruecksetzen", regexp.MustCompile(`(?i)>\s*Zurücksetzen`)},
	{"german_loeschen", regexp.MustCompile(`(?i)>\s*Löschen`)},
	{"german_bearbeiten", regexp.MustCompile(`(?i)>\s*Bearbeiten`)},
	{"german_erstellen", regexp.MustCompile(`(?i)>\s*Erstellen`)},
	{"german_konfiguration", regexp.MustCompile(`(?i)>\s*Konfiguration`)},
	{"german_einstellungen", regexp.MustCompile(`(?i)>\s*Einstellungen`)},
	{"german_passwort", regexp.MustCompile(`(?i)>\s*Passwort`)},
	{"german_benutzer", regexp.MustCompile(`(?i)>\s*Benutzer`)},
	{"german_anmeldung", regexp.MustCompile(`(?i)>\s*Anmeldung`)},
	{"german_fehler", regexp.MustCompile(`(?i)>\s*Fehler`)},
	{"german_erfolg", regexp.MustCompile(`(?i)>\s*Erfolg`)},
	{"german_aktualisieren", regexp.MustCompile(`(?i)>\s*Aktualisieren`)},
	{"german_zurueck", regexp.MustCompile(`(?i)>\s*Zurück`)},
	{"german_weiter", regexp.MustCompile(`(?i)>\s*Weiter`)},
	{"german_fertig", regexp.MustCompile(`(?i)>\s*Fertig`)},
	{"german_starten", regexp.MustCompile(`(?i)>\s*Starten`)},
	{"german_stoppen", regexp.MustCompile(`(?i)>\s*Stoppen`)},
	{"german_neu", regexp.MustCompile(`(?i)>\s*Neu`)},
	{"german_konto", regexp.MustCompile(`(?i)>\s*Konto`)},
	{"german_profil", regexp.MustCompile(`(?i)>\s*Profil`)},
}

// TestFrontend_HTML_HardcodedGermanUIText scans .html files in ui/ (excluding lang/)
// for German UI text that appears in HTML without data-i18n attribute.
// German text is a strong indicator of hardcoded content since most UI
// should use i18n keys even if only English is present in the file.
func TestFrontend_HTML_HardcodedGermanUIText(t *testing.T) {
	t.Parallel()

	htmlFiles := []string{
		"index.html",
		"config.html",
		"containers.html",
		"dashboard.html",
		"gallery.html",
		"cheatsheets.html",
		"invasion_control.html",
		"knowledge.html",
		"login.html",
		"media.html",
		"missions_v2.html",
		"setup.html",
		"skills.html",
		"plans.html",
		"truenas.html",
	}

	var failures []string
	checkedFiles := 0

	for _, htmlFile := range htmlFiles {
		path := htmlFile
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		checkedFiles++
		lines := strings.Split(string(content), "\n")

		for lineNum, line := range lines {
			// Skip lines that already use data-i18n - these are properly internationalized
			if i18nSafeHTMLPattern.MatchString(line) {
				continue
			}

			for _, check := range germanUITextPatterns {
				if check.pattern.MatchString(line) {
					failures = append(failures, fmt.Sprintf("%s:%d: %s (line: %s)", path, lineNum+1, check.desc, strings.TrimSpace(line)))
				}
			}
		}
	}

	if len(failures) > 0 {
		t.Errorf("Found %d hardcoded German UI strings in %d HTML files:\n%s", len(failures), checkedFiles, strings.Join(failures, "\n"))
	}
}

// TestTranslations_MultimodalKeysAreTranslated verifies that specific multimodal
// related translation keys are present and translated in all language files.
func TestTranslations_MultimodalKeysAreTranslated(t *testing.T) {
	t.Parallel()

	type check struct {
		relPath string
		key     string
	}
	checks := []check{
		{relPath: filepath.Join("lang", "help"), key: "help.llm.multimodal"},
		{relPath: filepath.Join("lang", "help"), key: "help.llm.multimodal_provider_types_extra"},
		{relPath: filepath.Join("lang", "config", "misc"), key: "config.llm.multimodal_banner"},
	}

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

	for _, c := range checks {
		enFile := filepath.Join(c.relPath, "en.json")
		en, err := readJSONFileMap(enFile)
		if err != nil {
			t.Fatalf("read %s: %v", enFile, err)
		}
		enVal, ok := en[c.key].(string)
		if !ok || enVal == "" {
			t.Fatalf("missing or non-string key %q in %s", c.key, enFile)
		}

		for _, lang := range langs {
			f := filepath.Join(c.relPath, lang+".json")
			m, err := readJSONFileMap(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			vAny, ok := m[c.key]
			if !ok {
				t.Fatalf("missing key %q in %s", c.key, f)
			}
			v, ok := vAny.(string)
			if !ok || v == "" {
				t.Fatalf("key %q is not a non-empty string in %s", c.key, f)
			}
			if lang != "en" && v == enVal {
				t.Fatalf("key %q in %s is identical to English; needs translation", c.key, f)
			}
		}
	}
}

// TestTranslations_AllKeysPresentInAllLanguages verifies that all translation keys
// present in en.json are also present in all other language files.
func TestTranslations_AllKeysPresentInAllLanguages(t *testing.T) {
	t.Parallel()

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	langDir := filepath.Join("lang")

	// Find all subdirectories in lang/
	entries, err := os.ReadDir(langDir)
	if err != nil {
		t.Skipf("ui/lang/ directory not found, skipping test: %v", err)
	}

	// Collect all translation directories (including nested ones like config/*/)
	var transDirs []string

	var walkDir func(path string)
	walkDir = func(path string) {
		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			subPath := filepath.Join(path, e.Name())
			// Check if this directory has an en.json file
			enFile := filepath.Join(subPath, "en.json")
			if _, err := os.Stat(enFile); err == nil {
				transDirs = append(transDirs, subPath)
			}
			// Recursively check subdirectories
			walkDir(subPath)
		}
	}

	// Start with top-level directories
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirPath := filepath.Join(langDir, e.Name())
		enFile := filepath.Join(dirPath, "en.json")
		if _, err := os.Stat(enFile); err == nil {
			transDirs = append(transDirs, dirPath)
		}
		// Also walk subdirectories
		walkDir(dirPath)
	}

	if len(transDirs) == 0 {
		t.Skip("No translation directories with en.json found")
	}

	var failures []string

	for _, transDir := range transDirs {
		relDir, _ := filepath.Rel("lang", transDir)
		if relDir == "." {
			relDir = "lang"
		}

		// Read en.json as reference
		enFile := filepath.Join(transDir, "en.json")
		enMap, err := readJSONFileMap(enFile)
		if err != nil {
			failures = append(failures, fmt.Sprintf("Failed to read %s: %v", enFile, err))
			continue
		}

		// Collect all keys from en.json (including nested ones)
		var collectKeys func(prefix string, m map[string]any, keys *[]string)
		collectKeys = func(prefix string, m map[string]any, keys *[]string) {
			for k, v := range m {
				fullKey := k
				if prefix != "" {
					fullKey = prefix + "." + k
				}
				if nested, ok := v.(map[string]any); ok {
					collectKeys(fullKey, nested, keys)
				} else {
					*keys = append(*keys, fullKey)
				}
			}
		}

		var enKeys []string
		collectKeys("", enMap, &enKeys)

		// Check each non-English language
		for _, lang := range langs {
			if lang == "en" {
				continue
			}

			langFile := filepath.Join(transDir, lang+".json")
			langMap, err := readJSONFileMap(langFile)
			if err != nil {
				failures = append(failures, fmt.Sprintf("Missing file %s: %v", langFile, err))
				continue
			}

			// Collect all keys from the language file
			var langKeys []string
			collectKeys("", langMap, &langKeys)

			// Build a set for quick lookup
			langKeySet := make(map[string]bool)
			for _, k := range langKeys {
				langKeySet[k] = true
			}

			// Find missing keys
			var missingKeys []string
			for _, key := range enKeys {
				if !langKeySet[key] {
					missingKeys = append(missingKeys, key)
				}
			}

			if len(missingKeys) > 0 {
				failures = append(failures, fmt.Sprintf("Directory %s, language %s: missing %d keys: %v", relDir, lang, len(missingKeys), missingKeys))
			}
		}
	}

	if len(failures) > 0 {
		t.Errorf("Found %d translation completeness issues:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

// TestTranslations_NoNestedStructures verifies that all translation JSON files
// contain flat string values (no nested objects) except for meta.json.
func TestTranslations_NoNestedStructures(t *testing.T) {
	t.Parallel()

	langDir := filepath.Join("lang")
	metaFile := filepath.Join(langDir, "meta.json")

	var failures []string

	var checkFile func(path string)
	checkFile = func(path string) {
		content, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("Failed to read %s: %v", path, err))
			return
		}

		var m map[string]any
		if err := json.Unmarshal(content, &m); err != nil {
			failures = append(failures, fmt.Sprintf("Invalid JSON in %s: %v", path, err))
			return
		}

		// Check for nested structures
		var checkNested func(prefix string, v any)
		checkNested = func(prefix string, v any) {
			switch val := v.(type) {
			case map[string]any:
				for k, nestedV := range val {
					newKey := k
					if prefix != "" {
						newKey = prefix + "." + k
					}
					if _, ok := nestedV.(map[string]any); ok {
						failures = append(failures, fmt.Sprintf("Nested structure found at %s in %s", newKey, path))
					} else {
						checkNested(newKey, nestedV)
					}
				}
			case []any:
				// Arrays are also considered nested structures
				failures = append(failures, fmt.Sprintf("Array found at %s in %s", prefix, path))
			}
		}

		checkNested("", m)
	}

	var walkDir func(path string)
	walkDir = func(path string) {
		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}
		for _, e := range entries {
			fullPath := filepath.Join(path, e.Name())
			if e.IsDir() {
				walkDir(fullPath)
			} else if strings.HasSuffix(e.Name(), ".json") {
				// Skip meta.json
				absMeta, _ := filepath.Abs(metaFile)
				absPath, _ := filepath.Abs(fullPath)
				if absPath == absMeta {
					continue
				}
				checkFile(fullPath)
			}
		}
	}

	walkDir(langDir)

	if len(failures) > 0 {
		t.Errorf("Found %d files with nested structures:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

// TestTranslations_AllLanguageFilesExist verifies that all translation directories
// containing en.json also contain all 16 language files.
func TestTranslations_AllLanguageFilesExist(t *testing.T) {
	t.Parallel()

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	langDir := filepath.Join("lang")

	// Find all subdirectories in lang/
	entries, err := os.ReadDir(langDir)
	if err != nil {
		t.Skipf("ui/lang/ directory not found, skipping test: %v", err)
	}

	// Collect all translation directories (including nested ones like config/*/)
	var transDirs []string

	var walkDir func(path string)
	walkDir = func(path string) {
		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			subPath := filepath.Join(path, e.Name())
			// Check if this directory has an en.json file
			enFile := filepath.Join(subPath, "en.json")
			if _, err := os.Stat(enFile); err == nil {
				transDirs = append(transDirs, subPath)
			}
			// Recursively check subdirectories
			walkDir(subPath)
		}
	}

	// Start with top-level directories
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirPath := filepath.Join(langDir, e.Name())
		enFile := filepath.Join(dirPath, "en.json")
		if _, err := os.Stat(enFile); err == nil {
			transDirs = append(transDirs, dirPath)
		}
		// Also walk subdirectories
		walkDir(dirPath)
	}

	if len(transDirs) == 0 {
		t.Skip("No translation directories with en.json found")
	}

	var failures []string

	for _, transDir := range transDirs {
		relDir, _ := filepath.Rel("lang", transDir)
		if relDir == "." {
			relDir = "lang"
		}

		for _, lang := range langs {
			langFile := filepath.Join(transDir, lang+".json")
			if _, err := os.Stat(langFile); os.IsNotExist(err) {
				failures = append(failures, fmt.Sprintf("Directory %s: missing file %s.json", relDir, lang))
			}
		}
	}

	if len(failures) > 0 {
		t.Errorf("Found %d missing language files:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

func TestTranslations_AllJSONFilesParse(t *testing.T) {
	t.Parallel()

	langDir := filepath.Join("lang")
	if _, err := os.Stat(langDir); err != nil {
		t.Skipf("ui/lang/ directory not found, skipping test: %v", err)
	}

	var failures []string

	var walkDir func(path string)
	walkDir = func(path string) {
		entries, err := os.ReadDir(path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("Failed to read directory %s: %v", path, err))
			return
		}
		for _, e := range entries {
			fullPath := filepath.Join(path, e.Name())
			if e.IsDir() {
				walkDir(fullPath)
				continue
			}
			if !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
				continue
			}

			content, err := os.ReadFile(fullPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("Failed to read %s: %v", fullPath, err))
				continue
			}

			var parsed any
			if err := json.Unmarshal(content, &parsed); err != nil {
				failures = append(failures, fmt.Sprintf("Invalid JSON in %s: %v", fullPath, err))
			}
		}
	}

	walkDir(langDir)

	if len(failures) > 0 {
		t.Errorf("Found %d invalid translation JSON files:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

func readJSONFileMap(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

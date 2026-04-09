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
		"setup.html",
		"skills.html",
		"plans.html",
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

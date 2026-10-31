package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

var configWave2Locales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

// configBaselineCheck compares the current gap lines with a committed baseline file. Only gaps that are
// not in the baseline fail; baseline lines that are fixed are logged. With the update variable set the
// file is rewritten from the current gaps.
func configBaselineCheck(t *testing.T, file, updateEnv string, current []string, header string) {
	t.Helper()
	set := map[string]bool{}
	for _, line := range current {
		set[line] = true
	}
	current = current[:0]
	for line := range set {
		current = append(current, line)
	}
	sort.Strings(current)
	if os.Getenv(updateEnv) == "1" {
		body := header + strings.Join(current, "\n")
		if len(current) > 0 {
			body += "\n"
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s rewritten with %d lines", file, len(current))
		return
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("baseline %s missing (run with %s=1): %v", file, updateEnv, err)
	}
	baseline := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			baseline[line] = true
		}
	}
	fixed := 0
	for line := range baseline {
		if !set[line] {
			fixed++
			t.Logf("fixed (remove from %s): %s", file, line)
		}
	}
	var added []string
	for _, line := range current {
		if !baseline[line] {
			added = append(added, line)
		}
	}
	if len(added) > 0 {
		shown := added
		if len(shown) > 60 {
			shown = shown[:60]
		}
		t.Errorf("%d new gaps not in %s (fix them or refresh with %s=1):\n%s", len(added), file, updateEnv, strings.Join(shown, "\n"))
	}
	t.Logf("%d gaps, %d in baseline, %d fixed", len(current), len(baseline), fixed)
}

// configFlattenLang flattens nested translation objects like internal/i18n does.
func configFlattenLang(t *testing.T, file string) map[string]string {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(bytes.TrimPrefix(mustReadUIFile(t, file), []byte("\xef\xbb\xbf")), &raw); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	out := map[string]string{}
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case string:
			if prefix != "" {
				out[prefix] = v
			}
		case map[string]any:
			for key, child := range v {
				next := key
				if prefix != "" {
					next = prefix + "." + key
				}
				walk(next, child)
			}
		}
	}
	for key, value := range raw {
		walk(key, value)
	}
	return out
}

// configLangFilesInLoadOrder lists lang files per language in the order internal/i18n.load merges them
// (depth-first over sorted directory entries, later files override earlier ones).
func configLangFilesInLoadOrder(t *testing.T) map[string][]string {
	t.Helper()
	result := map[string][]string{}
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			p := dir + "/" + entry.Name()
			if entry.IsDir() {
				walk(p)
			} else if strings.HasSuffix(entry.Name(), ".json") && dir != "lang" {
				lang := strings.TrimSuffix(entry.Name(), ".json")
				result[lang] = append(result[lang], p)
			}
		}
	}
	entries, err := os.ReadDir("lang")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			walk("lang/" + entry.Name())
		}
	}
	return result
}

func TestConfigNoTranslationFallbackChains(t *testing.T) {
	// t() returns the key (or '') for missing translations, so `t(a) || x` never falls back. Use tOr(key, fallback).
	pattern := regexp.MustCompile(`\bt\((?:[^()]|\([^()]*\))*\)\s*(\|\||\?\?)`)
	var files []string
	for _, glob := range []string{"cfg/*.js", "js/config/*.js"} {
		matches, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) < 50 {
		t.Fatalf("expected config modules, found %d", len(files))
	}
	for _, file := range files {
		for number, line := range strings.Split(string(mustReadUIFile(t, filepath.ToSlash(file))), "\n") {
			if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
				continue
			}
			if pattern.MatchString(line) {
				t.Errorf("%s:%d: t(...) || fallback never falls back, use tOr(key, fallback): %s", file, number+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestConfigI18nParityAcrossLanguages(t *testing.T) {
	var current []string
	folders := map[string]bool{}
	err := filepath.WalkDir("lang/config", func(p string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == "en.json" {
			folders[filepath.ToSlash(filepath.Dir(p))] = true
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, extra := range []string{"lang/help", "lang/ruview", "lang/common", "lang/action_policy"} {
		folders[extra] = true
	}
	perLang := map[string]int{}
	for folder := range folders {
		enKeys := configFlattenLang(t, folder+"/en.json")
		name := strings.TrimPrefix(folder, "lang/")
		for _, locale := range configWave2Locales {
			if locale == "en" {
				continue
			}
			file := folder + "/" + locale + ".json"
			var keys map[string]string
			if _, err := os.Stat(file); err == nil {
				keys = configFlattenLang(t, file)
			}
			for key := range enKeys {
				if _, ok := keys[key]; !ok {
					current = append(current, locale+" "+name+" "+key)
					perLang[locale]++
				}
			}
		}
	}
	t.Logf("missing keys per language: %v", perLang)
	configBaselineCheck(t, "testdata/config_i18n_missing.txt", "AURAGO_UPDATE_I18N_MISSING", current,
		"# <lang> <folder under ui/lang> <key>: config/help keys present in en.json but missing in <lang>.json (Welle 2 package T empties this).\n")
}

func TestConfigI18nNoConflictingDuplicateKeys(t *testing.T) {
	// internal/i18n merges every ui/lang file of a language into one map; a later file silently wins.
	// Duplicates with different values hide one text, so they are listed in this baseline-free allowlist only.
	allowed := map[string]string{}
	var conflicts []string
	for locale, files := range configLangFilesInLoadOrder(t) {
		seen := map[string]string{} // key -> value
		origin := map[string]string{}
		for _, file := range files {
			for key, value := range configFlattenLang(t, file) {
				if previous, ok := seen[key]; ok && previous != value {
					if _, ok := allowed[locale+" "+key]; !ok {
						conflicts = append(conflicts, fmt.Sprintf("%s %s: %s (overridden) vs %s (wins)", locale, key, origin[key], file))
					}
				}
				seen[key] = value
				origin[key] = file
			}
		}
	}
	sort.Strings(conflicts)
	if len(conflicts) > 0 {
		t.Errorf("%d keys defined with different values in several lang files (delete the dead copy):\n%s", len(conflicts), strings.Join(conflicts, "\n"))
	}
}

func TestConfigSectionDescriptionContract(t *testing.T) {
	// Sidebar descriptions: one sentence, ends with a period, 20-110 characters, no keyword lists.
	var current []string
	sectionDesc := regexp.MustCompile(`^config\.section\.([a-z0-9_]+)\.desc$`)
	files := configLangFilesInLoadOrder(t)
	for _, locale := range []string{"de", "en"} {
		merged := map[string]string{}
		for _, file := range files[locale] {
			for key, value := range configFlattenLang(t, file) {
				merged[key] = value
			}
		}
		for key, value := range merged {
			match := sectionDesc.FindStringSubmatch(key)
			if match == nil {
				continue
			}
			var problems []string
			text := strings.TrimSpace(value)
			if n := utf8.RuneCountInString(text); n < 20 || n > 110 {
				problems = append(problems, "length")
			}
			if !strings.HasSuffix(text, ".") {
				problems = append(problems, "period")
			}
			if strings.Contains(text, ";") {
				problems = append(problems, "semicolon")
			}
			sentenceText := strings.ReplaceAll(text, "Fritz!Box", "FritzBox") // "!" inside a product name is no sentence end
			if strings.Contains(strings.TrimSuffix(sentenceText, "."), ". ") || strings.ContainsAny(sentenceText, "!?") {
				problems = append(problems, "sentences")
			}
			for _, problem := range problems {
				current = append(current, locale+" "+match[1]+" "+problem)
			}
		}
	}
	if len(current) == 0 {
		t.Log("all sidebar descriptions satisfy the contract")
	}
	configBaselineCheck(t, "testdata/config_section_desc_gaps.txt", "AURAGO_UPDATE_SECTION_DESC_GAPS", current,
		"# <lang> <section key> <length|period|semicolon|sentences>: sidebar description contract violations (Welle 2 package 4 empties this).\n")
}

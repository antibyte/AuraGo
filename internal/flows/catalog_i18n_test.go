package flows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

var catalogLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func loadEasyDragLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "ui", "lang", "easydrag", lang+".json"))
	if err != nil {
		t.Fatalf("read %s: %v", lang, err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse %s: %v", lang, err)
	}
	return m
}

var localePlaceholder = regexp.MustCompile(`\{\{[^}]*\}\}|\{[a-z_]+\}`)

func localePlaceholders(s string) string {
	found := localePlaceholder.FindAllString(s, -1)
	sort.Strings(found)
	return strings.Join(found, " ")
}

func TestCatalogTranslationsAreComplete(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	en := loadEasyDragLocale(t, "en")
	for _, key := range CatalogI18nKeys(reg) {
		if strings.TrimSpace(en[key]) == "" {
			t.Errorf("en.json misses %s", key)
		}
	}
	for _, lang := range catalogLocales {
		m := loadEasyDragLocale(t, lang)
		for key, value := range en {
			got, ok := m[key]
			if !ok || strings.TrimSpace(got) == "" {
				t.Errorf("%s.json misses %s", lang, key)
				continue
			}
			if localePlaceholders(got) != localePlaceholders(value) {
				t.Errorf("%s.json %s: placeholders %q, want %q", lang, key, localePlaceholders(got), localePlaceholders(value))
			}
		}
		for key := range m {
			if _, ok := en[key]; !ok {
				t.Errorf("%s.json has %s, which en.json lacks", lang, key)
			}
			if !strings.HasPrefix(key, "easydrag.") {
				t.Errorf("%s.json has a key outside easydrag.*: %s", lang, key)
			}
		}
	}
}

func TestTemplatesAreValidInEveryLocale(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	vc := ValidateContext{Mode: ModeDraft, Now: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), Location: time.UTC}
	for _, lang := range catalogLocales {
		m := loadEasyDragLocale(t, lang)
		tr := func(key string) string {
			if v, ok := m[key]; ok {
				return v
			}
			return key
		}
		for _, info := range Templates() {
			f, err := TemplateFlow(info.ID, tr)
			if err != nil {
				t.Fatalf("%s/%s: %v", lang, info.ID, err)
			}
			for _, is := range Validate(f, reg, vc) {
				if is.Severity == SeverityError {
					t.Errorf("%s/%s: %+v", lang, info.ID, is)
				}
			}
		}
	}
}

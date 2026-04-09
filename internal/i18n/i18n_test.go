package i18n

import (
	"io/fs"
	"log/slog"
	"testing"

	"aurago/ui"
)

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// German variants
		{"German", "de"},
		{"german", "de"},
		{"Deutsch", "de"},
		{"deutsch", "de"},
		{"de", "de"},
		{"DE", "de"},
		{"De", "de"},

		// English variants
		{"English", "en"},
		{"english", "en"},
		{"en", "en"},
		{"EN", "en"},

		// Spanish
		{"Spanish", "es"},
		{"español", "es"},
		{"es", "es"},

		// French
		{"French", "fr"},
		{"français", "fr"},
		{"fr", "fr"},

		// Polish
		{"Polish", "pl"},
		{"polski", "pl"},
		{"pl", "pl"},

		// Chinese
		{"Chinese", "zh"},
		{"chinese", "zh"},
		{"zh", "zh"},

		// Japanese
		{"Japanese", "ja"},
		{"japanese", "ja"},
		{"日本語", "ja"},
		{"ja", "ja"},

		// Hindi
		{"Hindi", "hi"},
		{"hindi", "hi"},
		{"hi", "hi"},

		// Dutch
		{"Dutch", "nl"},
		{"dutch", "nl"},
		{"nederlands", "nl"},
		{"nl", "nl"},

		// Italian
		{"Italian", "it"},
		{"italiano", "it"},
		{"it", "it"},

		// Portuguese
		{"Portuguese", "pt"},
		{"português", "pt"},
		{"pt", "pt"},

		// Danish
		{"Danish", "da"},
		{"dansk", "da"},
		{"da", "da"},

		// Swedish
		{"Swedish", "sv"},
		{"svenska", "sv"},
		{"sv", "sv"},

		// Norwegian
		{"Norwegian", "no"},
		{"norsk", "no"},
		{"no", "no"},

		// Czech
		{"Czech", "cs"},
		{"čeština", "cs"},
		{"cs", "cs"},

		// Greek
		{"Greek", "el"},
		{"ελληνικά", "el"},
		{"el", "el"},

		// Unknown - fallback to English
		{"unknown", "en"},
		{"invalid", "en"},
		{"", "en"},
		{"   ", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeLang(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeLang(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadTranslations(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal("failed to create UI sub-FS:", err)
	}

	Load(uiFS, slog.Default())

	langs := GetSupportedLanguages()
	if len(langs) == 0 {
		t.Fatal("no languages loaded at all")
	}
	t.Logf("Loaded %d languages: %v", len(langs), langs)

	// Check German has setup keys
	deJSON := string(GetJSON("de"))
	if deJSON == "{}" || deJSON == "" {
		t.Fatal("German translations not loaded")
	}

	// Check English as fallback
	enJSON := string(GetJSON("en"))
	if enJSON == "{}" || enJSON == "" {
		t.Fatal("English translations not loaded")
	}

	// Check meta JSON
	metaJSON := string(GetMetaJSON())
	if metaJSON == "" {
		t.Error("Meta JSON is empty")
	}
	t.Logf("Meta JSON length: %d chars", len(metaJSON))
}

func TestGetJSONFallback(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal("failed to create UI sub-FS:", err)
	}

	Load(uiFS, slog.Default())

	// Existing language should work
	de := GetJSON("de")
	if string(de) == "{}" {
		t.Error("German JSON should not be empty")
	}

	// Non-existent language should fall back to English
	xyz := GetJSON("xyz")
	en := GetJSON("en")
	if string(xyz) != string(en) {
		t.Error("Non-existent language should fall back to English")
	}
}

func TestHasKey(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal("failed to create UI sub-FS:", err)
	}

	Load(uiFS, slog.Default())

	// Check existing keys
	if !HasKey("en", "setup.nav_next") {
		t.Error("setup.nav_next should exist in English")
	}

	if !HasKey("de", "setup.nav_next") {
		t.Error("setup.nav_next should exist in German")
	}

	// Check non-existing key
	if HasKey("en", "nonexistent.key") {
		t.Error("nonexistent.key should not exist")
	}
}

func TestT(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal("failed to create UI sub-FS:", err)
	}

	Load(uiFS, slog.Default())

	// Test basic lookup
	result := T("en", "setup.nav_next")
	if result == "" {
		t.Error("setup.nav_next should return a non-empty string in English")
	}
	t.Logf("T(en, setup.nav_next) = %q", result)

	// Test German lookup
	resultDe := T("de", "setup.nav_next")
	if resultDe == "" {
		t.Error("setup.nav_next should return a non-empty string in German")
	}
	t.Logf("T(de, setup.nav_next) = %q", resultDe)

	// Test non-existing key falls back to English
	resultFallback := T("de", "nonexistent.key")
	if resultFallback != "nonexistent.key" {
		t.Errorf("T(de, nonexistent.key) = %q, want fallback to key itself", resultFallback)
	}
}

func TestInterpolate(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal("failed to create UI sub-FS:", err)
	}

	Load(uiFS, slog.Default())

	// Test with no parameters
	result := T("en", "setup.nav_next")
	if result == "" {
		t.Error("T should return non-empty string")
	}
}

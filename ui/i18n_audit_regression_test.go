package ui

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTranslationsAuditCorrections(t *testing.T) {
	t.Parallel()
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(lang+"/file-manager-errors", func(t *testing.T) {
			values, err := readJSONFileMap(filepath.Join("lang", "desktop", lang+".json"))
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"desktop.fm.undo_error", "desktop.fm.redo_error"} {
				value, _ := values[key].(string)
				if !strings.Contains(value, "{{error}}") {
					t.Errorf("%s must show the error passed by the file manager", key)
				}
			}
		})
	}

	// These reviewed labels require native text; URLs, names and numeric
	// formats deliberately stay outside this check.
	for _, locale := range []struct {
		name   string
		script string
	}{
		{"el", `\p{Greek}`},
		{"hi", `\p{Devanagari}`},
		{"ja", `[\p{Han}\p{Hiragana}\p{Katakana}]`},
		{"zh", `\p{Han}`},
	} {
		for section, keys := range map[string][]string{
			"chat": {"chat.sse_tool_unknown", "chat.sse_tool_deferred", "chat.sse_tool_needs_setup"},
			"config/guardian": {
				"config.guardian.preset_strict", "config.guardian.preset_moderate",
				"config.guardian.preset_lenient", "config.guardian.sanitizer_title",
				"config.guardian.taint_title",
			},
		} {
			t.Run(locale.name+"/"+section, func(t *testing.T) {
				values, err := readJSONFileMap(filepath.Join("lang", section, locale.name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				native := regexp.MustCompile(locale.script)
				for _, key := range keys {
					value, _ := values[key].(string)
					if !native.MatchString(value) {
						t.Errorf("%s must contain translated %s text", key, locale.name)
					}
				}
			})
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(lang+"/composio-policy", func(t *testing.T) {
			values, err := readJSONFileMap(filepath.Join("lang", "help", lang+".json"))
			if err != nil {
				t.Fatal(err)
			}
			for key, raw := range values {
				if !strings.HasPrefix(key, "help.composio.") {
					continue
				}
				value, _ := raw.(string)
				for _, untranslated := range []string{
					"Natural-language tool input", "Enable this after storing",
					"Unknown or mutating tool slugs", "Keep disabled unless",
					"Maximum wait time per Composio", "Default: https://backend.composio.dev",
				} {
					if value == "" || strings.Contains(value, untranslated) {
						t.Errorf("%s must contain a complete translated explanation", key)
					}
				}
			}
		})
	}

	for section, pattern := range map[string]string{
		"chat":          `fluestern|hoert|pruefe|losstuermt|Autoritaet`,
		"config":        `Fuegt Dograh`,
		"config/telnyx": `Ihr Telnyx`,
		"help":          `Kompatibilitaetsfeld|geprueft|Fuegt relevanten|Natuerliche|Entitaeten`,
		"desktop":       `Oberflaeche|Gluecksfund|Orbitalkoenig|Hinzufuegen`,
	} {
		t.Run("de/"+section, func(t *testing.T) {
			values, err := readJSONFileMap(filepath.Join("lang", section, "de.json"))
			if err != nil {
				t.Fatal(err)
			}
			bad := regexp.MustCompile(pattern)
			for key, raw := range values {
				if value, ok := raw.(string); ok && bad.MatchString(value) {
					t.Errorf("%s contains a reviewed German spelling or address regression", key)
				}
			}
		})
	}
}

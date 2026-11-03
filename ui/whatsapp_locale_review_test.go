package ui

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	whatsAppLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	// German UI text addresses the reader with "du". The formal forms are wrong here.
	germanFormalForm = regexp.MustCompile(`\b(Sie|Ihnen|Ihre?[nmrs]?)\b`)
	// Typical stand-ins for umlauts in words these strings use. Real umlauts are required.
	germanAsciiUmlaut = regexp.MustCompile(`(?i)\b(fuer|ueber|koenn\w*|pruef\w*|naechst\w*|bestaetig\w*|aender\w*|verknuepf\w*|loes\w*|waehl\w*|zurueck\w*|geraet\w*)`)
	placeholderToken  = regexp.MustCompile(`\{[a-z_]+\}`)
)

// whatsAppLocaleKeys returns every WhatsApp string of stages 1-4 by locale directory.
// All config/whatsapp keys come from en.json, so a key added later is covered at once.
func whatsAppLocaleKeys(t *testing.T) map[string][]string {
	t.Helper()
	en, err := readJSONFileMap(filepath.Join("lang", "config", "whatsapp", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	own := make([]string, 0, len(en))
	for key := range en {
		own = append(own, key)
	}
	sort.Strings(own)
	return map[string][]string{
		"config/whatsapp": own,
		"config/sections": {"config.section.whatsapp.label", "config.section.whatsapp.desc"},
		"dashboard":       {"dashboard.integration_whatsapp"},
		"easydrag": {
			"easydrag.node.notify_whatsapp.description",
			"easydrag.node.notify_whatsapp.label",
			"easydrag.node.notify_whatsapp.summary",
			"easydrag.ui.notify_whatsapp",
		},
		"config/newspaper": {
			"config.newspaper.allow_whatsapp",
			"config.newspaper.allow_whatsapp_help",
			"config.newspaper.whatsapp_ready",
			"config.newspaper.whatsapp_pending",
		},
		"desktop": {
			"newspaper.send_whatsapp",
			"newspaper.send_whatsapp_test",
			"newspaper.whatsapp_daily",
			"newspaper.whatsapp_ready",
			"newspaper.whatsapp_unavailable",
		},
		"config/security": {
			"security.hint.whatsapp_self_chat_personal_account.title",
			"security.hint.whatsapp_self_chat_personal_account.description",
			"security.hint.whatsapp_dedicated_no_owner.title",
			"security.hint.whatsapp_dedicated_no_owner.description",
		},
		"config/danger": {"config.danger.unsafe_host_execution.desc"},
	}
}

// Every WhatsApp string exists in all 16 locales, is a real translation (not the English text),
// keeps its placeholders and, in German, uses "du" with real umlauts.
func TestWhatsAppLocaleStringsAreCompleteAndTranslated(t *testing.T) {
	keysByDir := whatsAppLocaleKeys(t)
	dirs := make([]string, 0, len(keysByDir))
	for dir := range keysByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		parts := strings.Split(dir, "/")
		en, err := readJSONFileMap(filepath.Join(append(append([]string{"lang"}, parts...), "en.json")...))
		if err != nil {
			t.Fatal(err)
		}
		for _, lang := range whatsAppLocales {
			values, err := readJSONFileMap(filepath.Join(append(append([]string{"lang"}, parts...), lang+".json")...))
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range keysByDir[dir] {
				english, _ := en[key].(string)
				if strings.TrimSpace(english) == "" {
					t.Errorf("en: %s/%s is missing", dir, key)
					continue
				}
				value, _ := values[key].(string)
				if strings.TrimSpace(value) == "" {
					t.Errorf("%s: %s/%s is missing or empty", lang, dir, key)
					continue
				}
				if lang != "en" && value == english && english != "WhatsApp" && !strings.HasPrefix(english, "{") {
					t.Errorf("%s: %s/%s is still the English text", lang, dir, key)
				}
				if got, want := placeholderToken.FindAllString(value, -1), placeholderToken.FindAllString(english, -1); strings.Join(sortedCopy(got), ",") != strings.Join(sortedCopy(want), ",") {
					t.Errorf("%s: %s/%s has placeholders %v, want %v", lang, dir, key, got, want)
				}
				if lang == "de" {
					if germanFormalForm.MatchString(value) {
						t.Errorf("de: %s/%s uses the formal address: %q", dir, key, value)
					}
					if germanAsciiUmlaut.MatchString(value) {
						t.Errorf("de: %s/%s spells umlauts as ASCII: %q", dir, key, value)
					}
				}
			}
		}
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

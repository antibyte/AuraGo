package ui

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var localWikipediaLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func readLocalWikipediaBundles(t *testing.T) (map[string]map[string]interface{}, []string) {
	t.Helper()
	byLocale := make(map[string]map[string]interface{}, len(localWikipediaLocales))
	for _, locale := range localWikipediaLocales {
		byLocale[locale] = readLocaleMap(t, filepath.Join("lang", "config", "local_wikipedia", locale+".json"))
	}
	keys := make([]string, 0, len(byLocale["en"]))
	for key := range byLocale["en"] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return byLocale, keys
}

func TestConfigLocalWikipediaTranslationsCoverAllLocales(t *testing.T) {
	byLocale, keys := readLocalWikipediaBundles(t)
	english := byLocale["en"]
	if len(keys) < 80 {
		t.Fatalf("English Local Wikipedia bundle is unexpectedly small: %d keys", len(keys))
	}
	placeholder := regexp.MustCompile(`\{[a-z]+\}`)
	for _, locale := range localWikipediaLocales {
		values := byLocale[locale]
		got := make([]string, 0, len(values))
		for key := range values {
			got = append(got, key)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, keys) {
			t.Fatalf("%s keys differ from English", locale)
		}
		identical := 0
		for _, key := range keys {
			value, _ := values[key].(string)
			if strings.TrimSpace(value) == "" {
				t.Fatalf("%s: %s is empty", locale, key)
			}
			wantPlaceholders := placeholder.FindAllString(english[key].(string), -1)
			gotPlaceholders := placeholder.FindAllString(value, -1)
			sort.Strings(wantPlaceholders)
			sort.Strings(gotPlaceholders)
			if !reflect.DeepEqual(gotPlaceholders, wantPlaceholders) {
				t.Fatalf("%s: %s placeholders %v, want %v", locale, key, gotPlaceholders, wantPlaceholders)
			}
			if value == english[key] {
				identical++
			}
			if strings.Contains(value, "9–130") || strings.Contains(value, "1–130") {
				t.Fatalf("%s: %s must state the size as up to 130 GB", locale, key)
			}
		}
		if locale != "en" && identical > 8 {
			t.Fatalf("%s copies %d English strings", locale, identical)
		}
	}
	german, _ := json.Marshal(byLocale["de"])
	if text := string(german); strings.Contains(text, " Sie ") || strings.Contains(text, `"Sie `) ||
		!strings.ContainsAny(text, "äöüß") || strings.Contains(text, "Loeschen") {
		t.Fatal("German Local Wikipedia strings must use Du and real umlauts")
	}
	for _, suffix := range []string{
		"state_not_installed", "state_downloading", "state_verifying", "state_ready", "state_interrupted", "state_error",
		"error_insufficient_disk_space", "error_free_space_unknown", "error_checksum_mismatch", "error_download_failed",
		"error_catalog_unreachable", "error_zim_unreadable", "error_fulltext_unsupported", "error_busy", "error_disabled",
		"error_data_dir_invalid", "error_already_installed", "error_no_operation", "error_unknown_language", "error_unknown",
		"variant_nopic", "variant_maxi", "enabled", "agent_access", "language", "variant", "update_check", "data_dir",
	} {
		if _, ok := english["config.local_wikipedia."+suffix]; !ok {
			t.Fatalf("English bundle lacks the dynamic key config.local_wikipedia.%s", suffix)
		}
	}
	for _, field := range []string{"enabled", "agent_access", "language", "variant", "data_dir", "update_check"} {
		if _, ok := english["help.local_wikipedia."+field]; !ok {
			t.Fatalf("English bundle lacks help.local_wikipedia.%s", field)
		}
	}
	for _, key := range []string{"config.section.local_wikipedia.label", "config.section.local_wikipedia.desc"} {
		if _, ok := english[key]; !ok {
			t.Fatalf("English bundle lacks %s", key)
		}
	}
}

func TestConfigLocalWikipediaModuleUsesAdminAPIAndKnownKeys(t *testing.T) {
	module := string(mustReadUIFile(t, "cfg/local_wikipedia.js"))
	for _, wanted := range []string{
		"function renderLocalWikipediaSection(",
		"'/api/local-wikipedia/status'",
		"'/api/local-wikipedia/catalog?lang='",
		"'/api/local-wikipedia/install'",
		"'/api/local-wikipedia/cancel'",
		"'/api/local-wikipedia/delete'",
		"'/api/local-wikipedia/check-update'",
		"hasUnsavedConfigChanges()",
		"isDockerRuntime()",
		"status.data_dir_locked",
		"showConfirm(",
		"setTimeout(localWikiRefreshStatus, 2000)",
		"confirm_unknown_space: true",
		"replace_mode: 'delete_old_first'",
		"data.can_delete_old === true",
		`<progress id="lw-progress"`,
		"aurago:config-saved",
		"cfg:section-leave",
		"['no', 'Norsk']",
		// The status error text prefers the server's operation-specific
		// recommendation, and "edition available" follows the readable flag.
		"function localWikiStatusErrorText(",
		"status.recommendation",
		"status.readable === true",
	} {
		if !strings.Contains(module, wanted) {
			t.Fatalf("Local Wikipedia config module missing %q", wanted)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt(", `style="`, "130 GB"} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("Local Wikipedia config module contains forbidden %q", forbidden)
		}
	}
	byLocale, _ := readLocalWikipediaBundles(t)
	for _, match := range regexp.MustCompile(`'((?:config|help)\.local_wikipedia\.[a-z_]+)'`).FindAllStringSubmatch(module, -1) {
		key := match[1]
		if strings.HasSuffix(key, ".") || strings.HasSuffix(key, "_") {
			continue
		}
		if _, ok := byLocale["en"][key]; !ok {
			t.Fatalf("module uses %s, which the English bundle lacks", key)
		}
	}
}

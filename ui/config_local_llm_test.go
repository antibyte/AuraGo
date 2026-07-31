package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestLocalLLMConfigModuleUsesSavedConfigurationAndAdminSurface(t *testing.T) {
	module := string(mustReadUIFile(t, "cfg/local_llm.js"))
	mainJS := string(mustReadUIFile(t, "js/config/main.js"))
	for _, required := range []string{
		"/api/local-llm/status",
		"/api/local-llm/probe",
		"/api/local-llm/install",
		"/api/local-llm/action",
		"/api/local-llm/role",
		"/api/local-llm/acknowledgement",
		"isDirty",
		"aurago:config-saved",
		"cfg:section-leave",
		"setTimeout(localLLMRefreshStatus, 2000)",
		"_localLLMInstallPending",
		"['16384', '16K']",
		"['32768', '32K']",
		"localLLMLocalizedRuntimeValue",
		"cache.qualified",
		"cache.decision_persisted",
		"localLLMCacheErrorText",
		"], 'number');",
		`data-type="`,
	} {
		if !strings.Contains(module, required) {
			t.Fatalf("Local LLM config module missing %q", required)
		}
	}
	for _, obsolete := range []string{"['2048', '2K']", "['8192', '8K']"} {
		if strings.Contains(module, obsolete) {
			t.Fatalf("Local LLM config module still offers obsolete context option %q", obsolete)
		}
	}
	stateJS := string(mustReadUIFile(t, "js/config/state.js"))
	if !strings.Contains(stateJS, `element.dataset.type === 'number'`) {
		t.Fatal("config state does not preserve numeric select values as numbers")
	}
	if !strings.Contains(mainJS, `el.dataset.type === 'number'`) {
		t.Fatal("legacy config form collector does not preserve numeric select values as numbers")
	}
	if !strings.Contains(mainJS, `local_llm: { m: 'local_llm', fn: 'renderLocalLLMSection' }`) {
		t.Fatal("config main module registry is missing Local LLM lazy loading")
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt("} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("Local LLM module uses forbidden browser dialog %q", forbidden)
		}
	}
}

func TestLocalLLMSetupIsOptionalAndInstallRunsAfterSetupSave(t *testing.T) {
	html := string(mustReadUIFile(t, "setup.html"))
	setupJS := string(mustReadUIFile(t, "js/setup/main.js"))
	for _, required := range []string{
		`id="step-local-llm"`,
		`data-i18n="config.local_llm.purpose"`,
		`data-i18n="config.local_llm.hardware"`,
		`data-i18n="config.local_llm.quality"`,
		`id="setup-local-llm-ack"`,
	} {
		if !strings.Contains(html, required) {
			t.Fatalf("setup Local LLM step missing %q", required)
		}
	}
	for _, required := range []string{
		`body: JSON.stringify({ backend:`,
		`acknowledgement_fingerprint`,
		`if (!setupLocalLLMProbeFingerprint)`,
		`config.local_llm.compat_`,
	} {
		if !strings.Contains(setupJS, required) {
			t.Fatalf("setup Local LLM logic missing %q", required)
		}
	}
	saveAt := strings.Index(setupJS, "const result = await resp.json()")
	jobAt := strings.Index(setupJS, "pollSetupLocalLLMJob(result.local_llm_job_id")
	if saveAt < 0 || jobAt < saveAt {
		t.Fatal("setup job polling is not ordered after the regular setup save response")
	}
}

func TestLocalLLMTranslationsCoverAllSixteenLocalesWithoutEnglishPlaceholders(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	configByLocale := make(map[string]map[string]interface{}, len(locales))
	setupByLocale := make(map[string]map[string]interface{}, len(locales))
	for _, locale := range locales {
		configByLocale[locale] = readLocaleMap(t, filepath.Join("lang", "config", locale+".json"))
		setupByLocale[locale] = readLocaleMap(t, filepath.Join("lang", "setup", locale+".json"))
	}

	expectedConfig := prefixedLocaleKeys(configByLocale["en"], "config.local_llm.")
	expectedSetup := append(
		prefixedLocaleKeys(setupByLocale["en"], "setup.local_llm_"),
		"setup.step_label_local_llm",
	)
	sort.Strings(expectedSetup)
	if len(expectedConfig) < 35 || len(expectedSetup) < 10 {
		t.Fatalf("English Local LLM translation surface is unexpectedly incomplete: config=%d setup=%d", len(expectedConfig), len(expectedSetup))
	}
	englishConfig := localeSubset(configByLocale["en"], expectedConfig)
	englishSetup := localeSubset(setupByLocale["en"], expectedSetup)
	for _, locale := range locales {
		if got := prefixedLocaleKeys(configByLocale[locale], "config.local_llm."); !reflect.DeepEqual(got, expectedConfig) {
			t.Fatalf("%s Local LLM config keys differ from English\n got: %v\nwant: %v", locale, got, expectedConfig)
		}
		gotSetup := append(prefixedLocaleKeys(setupByLocale[locale], "setup.local_llm_"), "setup.step_label_local_llm")
		sort.Strings(gotSetup)
		if !reflect.DeepEqual(gotSetup, expectedSetup) {
			t.Fatalf("%s Local LLM setup keys differ from English\n got: %v\nwant: %v", locale, gotSetup, expectedSetup)
		}
		if locale != "en" &&
			reflect.DeepEqual(localeSubset(configByLocale[locale], expectedConfig), englishConfig) &&
			reflect.DeepEqual(localeSubset(setupByLocale[locale], expectedSetup), englishSetup) {
			t.Fatalf("%s uses only English Local LLM placeholders", locale)
		}
	}
}

func readLocaleMap(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return decoded
}

func prefixedLocaleKeys(value map[string]interface{}, prefix string) []string {
	keys := flattenLocaleKeys("", value)
	filtered := make([]string, 0)
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			filtered = append(filtered, key)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func localeSubset(value map[string]interface{}, keys []string) map[string]string {
	subset := make(map[string]string, len(keys))
	for _, key := range keys {
		if direct, ok := value[key].(string); ok {
			subset[key] = direct
			continue
		}
		current := interface{}(value)
		for _, part := range strings.Split(key, ".") {
			object, ok := current.(map[string]interface{})
			if !ok {
				current = nil
				break
			}
			current = object[part]
		}
		subset[key], _ = current.(string)
	}
	return subset
}

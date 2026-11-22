package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

// The Chibi Sensei config section is registered with its lazy module, sidebar slot and inline icon, binds every P1 config field,
// uses the shared provider dropdown for the generator model and has its strings in all sixteen config locales.
func TestConfigChibiSenseiSection(t *testing.T) {
	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"{ key: 'chibi_sensei', label: t('config.section.chibi_sensei.label'), desc: t('config.section.chibi_sensei.desc') }",
		"chibi_sensei: 129,",
		"chibi_sensei: '<circle",
		"chibi_sensei: { m: 'chibi_sensei', fn: 'renderChibiSenseiSection' }",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js misses %q", marker)
		}
	}
	module := normalizeAssetText(mustReadUIFile(t, "cfg/chibi_sensei.js"))
	for _, marker := range []string{
		"function renderChibiSenseiSection(section)",
		"'chibi_sensei.enabled'", "'chibi_sensei.storage.max_gb'", "'chibi_sensei.dataset.max_rows'",
		"'chibi_sensei.import.max_rows'", "'chibi_sensei.import.max_file_mb'", "'chibi_sensei.generator.model'",
		"'chibi_sensei.generator.max_rows_per_job'", "'chibi_sensei.generator.max_daily_rows'",
		`data-path="chibi_sensei.generator.provider" data-config-choice="providers"`,
		"cfgChoiceOptionsHTML('providers', helpTexts['chibi_sensei.generator.provider'] || {}, data.generator.provider || '')",
		"window.AuraConfigState.set('chibi_sensei.enabled', next)",
		"/desktop?app=chibi-sensei",
	} {
		if !strings.Contains(module, marker) {
			t.Fatalf("cfg/chibi_sensei.js misses %q", marker)
		}
	}
	// The ranges equal internal/config/chibi_sensei.go.
	for _, marker := range []string{
		"'chibi_sensei.storage.max_gb', data.storage.max_gb, 1, 500",
		"'chibi_sensei.dataset.max_rows', data.dataset.max_rows, 1000, 50000",
		"'chibi_sensei.import.max_rows', data.import.max_rows, 100, 50000",
		"'chibi_sensei.import.max_file_mb', data.import.max_file_mb, 1, 500",
		"'chibi_sensei.generator.max_rows_per_job', data.generator.max_rows_per_job, 10, 5000",
		"'chibi_sensei.generator.max_daily_rows', data.generator.max_daily_rows, 100, 100000",
	} {
		if !strings.Contains(module, marker) {
			t.Fatalf("cfg/chibi_sensei.js misses the range %q", marker)
		}
	}
	catalog := normalizeAssetText(mustReadUIFile(t, "js/config/catalog.js"))
	for _, marker := range []string{
		"'chibi_sensei.storage.max_gb': { type: 'number', min: 1, max: 500, integer: true, changedOnly: true }",
		"'chibi_sensei.dataset.max_rows': { type: 'number', min: 1000, max: 50000, integer: true, changedOnly: true }",
		"'chibi_sensei.import.max_rows': { type: 'number', min: 100, max: 50000, integer: true, changedOnly: true }",
		"'chibi_sensei.import.max_file_mb': { type: 'number', min: 1, max: 500, integer: true, changedOnly: true }",
		"'chibi_sensei.generator.max_rows_per_job': { type: 'number', min: 10, max: 5000, integer: true, changedOnly: true }",
		"'chibi_sensei.generator.max_daily_rows': { type: 'number', min: 100, max: 100000, integer: true, changedOnly: true }",
	} {
		if !strings.Contains(catalog, marker) {
			t.Fatalf("config catalog.js misses %q", marker)
		}
	}
	var meta map[string]map[string]any
	if err := json.Unmarshal(mustReadUIFile(t, "lang/meta.json"), &meta); err != nil {
		t.Fatal(err)
	}
	if entry := meta["help.chibi_sensei.generator.provider"]; entry["provider_ref"] != true || entry["empty_label_key"] != "config.chibi_sensei.generator_provider_main" {
		t.Fatalf("lang/meta.json help.chibi_sensei.generator.provider = %v", entry)
	}

	var en map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/config/chibi_sensei/en.json"), &en); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"config.section.chibi_sensei.label", "config.section.chibi_sensei.desc", "config.chibi_sensei.generator_provider_main", "config.chibi_sensei.later_note"} {
		if en[key] == "" {
			t.Fatalf("lang/config/chibi_sensei/en.json misses %s", key)
		}
	}
	for _, lang := range storymakerLanguages {
		var words map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/chibi_sensei/"+lang+".json"), &words); err != nil {
			t.Fatalf("lang/config/chibi_sensei/%s.json: %v", lang, err)
		}
		for key := range en {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("lang/config/chibi_sensei/%s.json misses %s", lang, key)
			}
		}
		if len(words) != len(en) {
			t.Errorf("lang/config/chibi_sensei/%s.json has %d keys, en has %d", lang, len(words), len(en))
		}
		if lang == "de" {
			for key, text := range words {
				padded := " " + text + " "
				if strings.Contains(padded, " Sie ") || strings.Contains(padded, " Ihre ") || strings.Contains(padded, " Ihnen ") {
					t.Errorf("de: %s uses formal address", key)
				}
			}
		}
	}
}

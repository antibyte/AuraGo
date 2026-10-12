package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

// The CYD Creator config section is registered with its lazy module, sidebar slot and inline icon,
// binds every config field of the overview, uses the shared provider dropdown for the agent model
// and has its strings in all sixteen config locales.
func TestConfigCYDCreatorSection(t *testing.T) {
	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"{ key: 'cyd_creator', icon: '◧', label: t('config.section.cyd_creator.label'), desc: t('config.section.cyd_creator.desc') }",
		"cyd_creator: 123,",
		"cyd_creator: '<rect",
		"cyd_creator: { m: 'cyd_creator', fn: 'renderCYDCreatorSection' }",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js misses %q", marker)
		}
	}
	module := normalizeAssetText(mustReadUIFile(t, "cfg/cyd_creator.js"))
	for _, marker := range []string{
		"function renderCYDCreatorSection(section)",
		"'cyd_creator.enabled'", "'cyd_creator.builder_image'", "'cyd_creator.build_timeout_seconds'",
		"'cyd_creator.cold_build_timeout_seconds'", "'cyd_creator.memory_mb'", "'cyd_creator.cpu_cores'",
		"'cyd_creator.idle_stop_minutes'", "'cyd_creator.max_repair_attempts'", "'cyd_creator.job_timeout_minutes'",
		"'cyd_creator.keep_builds_per_project'", "'cyd_creator.ota.enabled'", "'cyd_creator.ota.heartbeat_seconds'",
		"'cyd_creator.ota.public_url'",
		`data-path="cyd_creator.model" data-config-choice="providers"`,
		"cfgChoiceOptionsHTML('providers', helpTexts['cyd_creator.model'] || {}, data.model || '')",
		"window.AuraConfigState.set('cyd_creator.enabled', next)",
		"/desktop?app=cyd-creator",
	} {
		if !strings.Contains(module, marker) {
			t.Fatalf("cfg/cyd_creator.js misses %q", marker)
		}
	}
	var meta map[string]map[string]any
	if err := json.Unmarshal(mustReadUIFile(t, "lang/meta.json"), &meta); err != nil {
		t.Fatal(err)
	}
	if entry := meta["help.cyd_creator.model"]; entry["provider_ref"] != true || entry["empty_label_key"] != "config.cyd_creator.model_main" {
		t.Fatalf("lang/meta.json help.cyd_creator.model = %v", entry)
	}

	var en map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/config/cyd_creator/en.json"), &en); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"config.section.cyd_creator.label", "config.section.cyd_creator.desc", "config.cyd_creator.model_main", "config.cyd_creator.restart_note"} {
		if en[key] == "" {
			t.Fatalf("lang/config/cyd_creator/en.json misses %s", key)
		}
	}
	for _, lang := range cydCreatorLocales {
		var words map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/cyd_creator/"+lang+".json"), &words); err != nil {
			t.Fatalf("lang/config/cyd_creator/%s.json: %v", lang, err)
		}
		for key := range en {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("lang/config/cyd_creator/%s.json misses %s", lang, key)
			}
		}
		if len(words) != len(en) {
			t.Errorf("lang/config/cyd_creator/%s.json has %d keys, en has %d", lang, len(words), len(en))
		}
	}
}

package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// The plan mode block of the Video Studio settings on the Virtual Desktop configuration page:
// video_studio.plan.{enabled, agent_provider, agent_model, max_daily_jobs, job_timeout_seconds} and its sixteen-language
// texts in lang/config/common.
var videoStudioPlanConfigKeys = []string{
	"plan_title", "plan_help", "plan_enabled", "plan_agent_provider", "plan_agent_provider_help", "plan_agent_provider_main",
	"plan_agent_model", "plan_agent_model_help", "plan_max_daily_jobs", "plan_max_daily_jobs_help", "plan_job_timeout",
	"plan_job_timeout_help",
}

func TestConfigVideoStudioPlanBlockIsWired(t *testing.T) {
	t.Parallel()
	src := strings.ReplaceAll(string(mustReadUIFile(t, filepath.Join("cfg", "virtual_desktop.js"))), "\r\n", "\n")
	for _, marker := range []string{
		"if (typeof plan.enabled !== 'boolean') plan.enabled = true;",
		"'video_studio.plan.enabled'",
		`data-path="video_studio.plan.agent_provider" data-config-choice="providers"`,
		"cfgChoiceOptionsHTML('providers', helpTexts['video_studio.plan.agent_provider'] || {}, plan.agent_provider || '')",
		`data-path="video_studio.plan.agent_model"`,
		`min="1" max="1000" value="' + escapeAttr(String(plan.max_daily_jobs || 20)) + '" data-path="video_studio.plan.max_daily_jobs"`,
		`min="120" max="3600" value="' + escapeAttr(String(plan.job_timeout_seconds || 900)) + '" data-path="video_studio.plan.job_timeout_seconds"`,
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("virtual_desktop.js lacks %q", marker)
		}
	}
	for _, key := range videoStudioPlanConfigKeys {
		if key != "plan_agent_provider_main" && !strings.Contains(src, "'"+key+"'") {
			t.Errorf("virtual_desktop.js never uses config.video_studio.%s", key)
		}
	}
	var meta map[string]map[string]any
	if err := json.Unmarshal(mustReadUIFile(t, filepath.Join("lang", "meta.json")), &meta); err != nil {
		t.Fatal(err)
	}
	if entry := meta["help.video_studio.plan.agent_provider"]; entry["provider_ref"] != true || entry["empty_label_key"] != "config.video_studio.plan_agent_provider_main" {
		t.Fatalf("lang/meta.json help.video_studio.plan.agent_provider = %v", entry)
	}
	// The form limits and defaults are the ones of normalizeVideoStudioPlan.
	normalize := string(mustReadUIFile(t, filepath.Join("..", "internal", "config", "video_studio.go")))
	for _, fragment := range []string{"min(max(p.MaxDailyJobs, 1), 1000)", "min(max(p.JobTimeoutSeconds, 120), 3600)", "= 20", "= 900"} {
		if !strings.Contains(normalize, fragment) {
			t.Errorf("internal/config/video_studio.go no longer holds %q, which the form promises", fragment)
		}
	}
}

func TestConfigVideoStudioPlanTranslations(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "config/common", "en")
	for _, lang := range storymakerLanguages {
		values := storymakerLocale(t, "config/common", lang)
		for _, key := range videoStudioPlanConfigKeys {
			full := "config.video_studio." + key
			text := strings.TrimSpace(values[full])
			if text == "" || strings.ContainsRune(text, utf8.RuneError) {
				t.Errorf("%s: %s is missing or broken", lang, full)
				continue
			}
			if lang != "en" && text == strings.TrimSpace(english[full]) {
				t.Errorf("%s: %s is untranslated", lang, full)
			}
			if lang == "de" && (strings.Contains(" "+text+" ", " Sie ") || strings.Contains(" "+text+" ", " Ihre ") || strings.Contains(" "+text+" ", " Ihnen ")) {
				t.Errorf("de: %s uses formal address", full)
			}
		}
	}
}

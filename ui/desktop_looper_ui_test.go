package ui

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

func TestDesktopLooperUIContract(t *testing.T) {
	t.Parallel()

	looper := readDesktopAssetText(t, "js/desktop/apps/looper.js")
	monitor := readDesktopAssetText(t, "js/desktop/apps/looper-monitor.js")
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")

	if !strings.Contains(loader, "'/js/desktop/apps/looper-monitor.js'") || !strings.Contains(loader, "'/js/desktop/apps/looper.js'") {
		t.Fatal("module loader must load looper-monitor.js before looper.js")
	}
	monitorIdx := strings.Index(loader, "'/js/desktop/apps/looper-monitor.js'")
	appIdx := strings.Index(loader, "'/js/desktop/apps/looper.js'")
	if monitorIdx < 0 || appIdx < 0 || monitorIdx > appIdx {
		t.Fatal("looper-monitor.js must load before looper.js")
	}

	for _, source := range []struct {
		name string
		body string
	}{
		{"looper.js", looper},
		{"looper-monitor.js", monitor},
	} {
		if got := desktopLooperEmoji(source.body); got != "" {
			t.Fatalf("%s must not contain emoji %q", source.name, got)
		}
	}

	for _, id := range []string{
		"looper-start-",
		"looper-stop-",
		"looper-pause-",
		"looper-resume-",
	} {
		needle := id + "' + windowId"
		if !strings.Contains(looper, needle) {
			t.Fatalf("looper control id %s must stay window-scoped via %q", id, needle)
		}
	}
	if strings.Contains(monitor, `id="looper-history-clear"`) {
		t.Fatal("history clear must not use a static duplicate id")
	}

	required := desktopLooperRequiredKeys()
	sources := looper + "\n" + monitor
	keyPattern := regexp.MustCompile(`['"]desktop\.looper_[a-z0-9_]+['"]`)
	used := map[string]bool{}
	for _, match := range keyPattern.FindAllString(sources, -1) {
		used[strings.Trim(match, `'"`)] = true
	}
	for _, key := range required {
		if used[key] || desktopLooperDerivedKey(key) {
			continue
		}
		t.Fatalf("looper UI no longer references required key %s", key)
	}

	for _, fragment := range []string{
		"desktop.looper_example_",
		"desktop.looper_status_",
		"desktop.looper_step_",
		"vd-looper-error",
		"vd-looper-log--pending",
	} {
		if !strings.Contains(looper+"\n"+monitor, fragment) {
			t.Fatalf("looper UI missing dynamic i18n family %q", fragment)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if !strings.Contains(values["desktop.looper_error_detail"], "{{message}}") {
			t.Fatalf("%s looper error detail must keep {{message}}: %q", path, values["desktop.looper_error_detail"])
		}
	}
}

func desktopLooperDerivedKey(key string) bool {
	rest := strings.TrimPrefix(key, "desktop.looper_")
	if strings.HasPrefix(rest, "example_") || strings.HasPrefix(rest, "status_") || strings.HasPrefix(rest, "step_") {
		return true
	}
	for _, field := range []string{"goal", "work", "evaluate", "finish"} {
		if rest == field || rest == field+"_help" || rest == field+"_placeholder" {
			return true
		}
	}
	return false
}

func desktopLooperRequiredKeys() []string {
	return []string{
		"desktop.looper_already_running",
		"desktop.looper_best_score",
		"desktop.looper_cost",
		"desktop.looper_cost_under",
		"desktop.looper_default_provider",
		"desktop.looper_delete",
		"desktop.looper_delete_confirm",
		"desktop.looper_delete_error",
		"desktop.looper_deleted",
		"desktop.looper_duplicate",
		"desktop.looper_duration_ms",
		"desktop.looper_duration_s",
		"desktop.looper_error",
		"desktop.looper_error_detail",
		"desktop.looper_evaluate",
		"desktop.looper_evaluate_help",
		"desktop.looper_evaluate_placeholder",
		"desktop.looper_example_flashcards",
		"desktop.looper_example_flashcards_desc",
		"desktop.looper_example_project_readme",
		"desktop.looper_example_project_readme_desc",
		"desktop.looper_example_python_tool",
		"desktop.looper_example_python_tool_desc",
		"desktop.looper_example_research_briefing",
		"desktop.looper_example_research_briefing_desc",
		"desktop.looper_example_short_story",
		"desktop.looper_example_short_story_desc",
		"desktop.looper_examples",
		"desktop.looper_field_required",
		"desktop.looper_finish",
		"desktop.looper_finish_help",
		"desktop.looper_finish_placeholder",
		"desktop.looper_finish_toggle",
		"desktop.looper_goal",
		"desktop.looper_goal_help",
		"desktop.looper_goal_placeholder",
		"desktop.looper_history_back",
		"desktop.looper_history_clear",
		"desktop.looper_history_clear_confirm",
		"desktop.looper_history_delete",
		"desktop.looper_history_delete_confirm",
		"desktop.looper_history_empty",
		"desktop.looper_history_load_error",
		"desktop.looper_jump_bottom",
		"desktop.looper_max_rounds",
		"desktop.looper_model",
		"desktop.looper_model_default",
		"desktop.looper_more_options",
		"desktop.looper_my_loops",
		"desktop.looper_my_loops_empty",
		"desktop.looper_name",
		"desktop.looper_new",
		"desktop.looper_no_logs",
		"desktop.looper_no_run",
		"desktop.looper_pause",
		"desktop.looper_pause_error",
		"desktop.looper_pause_requested",
		"desktop.looper_provider",
		"desktop.looper_resume",
		"desktop.looper_resume_error",
		"desktop.looper_round",
		"desktop.looper_round_of",
		"desktop.looper_save",
		"desktop.looper_save_error",
		"desktop.looper_save_prompt",
		"desktop.looper_save_update_confirm",
		"desktop.looper_saved",
		"desktop.looper_stall_rounds",
		"desktop.looper_stall_rounds_help",
		"desktop.looper_start",
		"desktop.looper_start_error",
		"desktop.looper_status_completed",
		"desktop.looper_status_failed",
		"desktop.looper_status_idle",
		"desktop.looper_status_max_rounds",
		"desktop.looper_status_paused",
		"desktop.looper_status_running",
		"desktop.looper_status_stalled",
		"desktop.looper_status_stopped",
		"desktop.looper_step_evaluate",
		"desktop.looper_step_finish",
		"desktop.looper_step_work",
		"desktop.looper_stop",
		"desktop.looper_stop_error",
		"desktop.looper_tab_history",
		"desktop.looper_tab_run",
		"desktop.looper_tab_setup",
		"desktop.looper_target_score",
		"desktop.looper_title",
		"desktop.looper_tokens",
		"desktop.looper_untitled",
		"desktop.looper_work",
		"desktop.looper_work_help",
		"desktop.looper_work_placeholder",
	}
}

func desktopLooperEmoji(source string) string {
	for _, r := range source {
		switch {
		case r >= 0x1F300 && r <= 0x1FAFF:
			return string(r)
		case r >= 0x2600 && r <= 0x27BF:
			return string(r)
		case r >= 0x1F900 && r <= 0x1F9FF:
			return string(r)
		case unicode.Is(unicode.So, r) && r > 0x2000:
			return string(r)
		}
	}
	return ""
}

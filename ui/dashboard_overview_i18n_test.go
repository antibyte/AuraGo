package ui

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// dashboardAttentionCodes are the attention codes of internal/server/dashboard_attention.go. The client renders
// each one as dashboard.attention.<code> with the item's params plus {{count}}.
var dashboardAttentionCodes = []string{
	"operational_issue", "cron_errors", "missions_failed", "daemon_failed", "budget_blocked", "budget_warning",
	"disk_high", "memory_high", "indexer_error", "memory_conflicts", "localwiki_update",
}

// dashboardOverviewI18nKeys are the P3 keys: overview tiles, sidebar status block, badges and attention texts.
func dashboardOverviewI18nKeys() []string {
	keys := []string{
		"dashboard.overview.status_title",
		"dashboard.overview.pill_ok",
		"dashboard.overview.pill_critical",
		"dashboard.overview.pill_warning",
		"dashboard.overview.pill_loading",
		"dashboard.overview.pill_unavailable",
		"dashboard.overview.label_agent",
		"dashboard.overview.label_model",
		"dashboard.overview.label_persona",
		"dashboard.overview.label_mood",
		"dashboard.overview.label_context",
		"dashboard.overview.label_last_activity",
		"dashboard.overview.attention_title",
		"dashboard.overview.attention_empty",
		"dashboard.overview.attention_more",
		"dashboard.overview.attention_problems_unchecked",
		"dashboard.overview.severity_critical",
		"dashboard.overview.severity_warning",
		"dashboard.overview.severity_info",
		"dashboard.overview.tile_system",
		"dashboard.overview.tile_budget",
		"dashboard.overview.tile_automations",
		"dashboard.overview.tile_knowledge",
		"dashboard.overview.tile_activity",
		"dashboard.overview.tile_agent_state",
		"dashboard.overview.waiting_for_sample",
		"dashboard.overview.not_available",
		"dashboard.overview.budget_spent",
		"dashboard.overview.budget_ok",
		"dashboard.overview.kpi_running",
		"dashboard.overview.kpi_queued",
		"dashboard.overview.kpi_cron_errors",
		"dashboard.overview.kpi_failed_missions",
		"dashboard.overview.kpi_kg_nodes",
		"dashboard.overview.activity_empty",
		"dashboard.overview.trend_label",
		"dashboard.overview.trend_steady",
		"dashboard.overview.trend_recovering",
		"dashboard.overview.trend_strained",
		"dashboard.overview.affect_at_rest",
		"dashboard.overview.customize",
		"dashboard.overview.show_all",
		"dashboard.overview.tile_toggle",
		"dashboard.overview.sidebar_status",
		"dashboard.overview.live_on",
		"dashboard.overview.live_off",
		"dashboard.overview.live_connecting",
		"dashboard.overview.badge_overview",
		"dashboard.overview.badge_problems",
		"dashboard.overview.badge_automations",
	}
	for _, code := range dashboardAttentionCodes {
		keys = append(keys, "dashboard.attention."+code)
	}
	return keys
}

// TestDashboardOverviewKeysExistInAllLocales: every P3 key exists in all 16 locales and keeps exactly the
// {{placeholders}} of its English text. The client fills those placeholders, so a translated placeholder
// would show up raw.
func TestDashboardOverviewKeysExistInAllLocales(t *testing.T) {
	t.Parallel()

	keys := dashboardOverviewI18nKeys()
	assertLocaleKeys(t, filepath.Join("lang", "dashboard"), keys)

	placeholder := regexp.MustCompile(`\{\{([a-z_]+)\}\}`)
	placeholders := func(text string) []string {
		var names []string
		for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
			names = append(names, m[1])
		}
		slices.Sort(names)
		return slices.Compact(names)
	}
	var english map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/dashboard/en.json"), &english); err != nil {
		t.Fatalf("lang/dashboard/en.json: %v", err)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/dashboard/"+lang+".json"), &values); err != nil {
			t.Fatalf("lang/dashboard/%s.json: %v", lang, err)
		}
		for _, key := range keys {
			if got, want := placeholders(values[key]), placeholders(english[key]); !slices.Equal(got, want) {
				t.Errorf("lang/dashboard/%s.json %s placeholders = %v, want %v", lang, key, got, want)
			}
		}
	}
}

package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// easyDragModules is the load order of the EasyDrag scripts (dependencies first).
var easyDragModules = []string{
	"easydrag-core.js", "easydrag-template.js", "easydrag-model.js", "easydrag-geometry.js", "easydrag-saver.js",
	"easydrag-canvas.js", "easydrag-wires.js", "easydrag-interact.js", "easydrag-palette.js", "easydrag-fields.js",
	"easydrag-forms.js", "easydrag-mapping.js", "easydrag-detail.js", "easydrag-runs.js", "easydrag-publish.js",
	"easydrag-home.js", "easydrag-dialogs.js", "easydrag-editor.js", "easydrag.js",
}

func TestEasyDragAssetsLoadInDependencyOrder(t *testing.T) {
	loader := strings.ReplaceAll(readDesktopAssetText(t, "js/desktop/core/module-loader.js"), "\r\n", "\n")
	start := strings.Index(loader, "'easydrag': {")
	if start < 0 {
		t.Fatal("module-loader.js has no easydrag entry")
	}
	end := strings.Index(loader[start:], "]\n        }")
	if end < 0 {
		t.Fatal("easydrag entry is not closed")
	}
	block := loader[start : start+end]
	for _, marker := range []string{"appStyles('/css/desktop-app-easydrag.css')", "modules: ['/js/vendor/pdf.min.js']"} {
		if !strings.Contains(block, marker) {
			t.Fatalf("easydrag assets miss %s:\n%s", marker, block)
		}
	}
	last := -1
	for _, name := range easyDragModules {
		idx := strings.Index(block, "'/js/desktop/apps/"+name+"'")
		if idx < 0 {
			t.Fatalf("easydrag assets miss %s", name)
		}
		if idx < last {
			t.Fatalf("%s loads before a module it depends on", name)
		}
		last = idx
	}
	if !strings.Contains(loader, "'easydrag': ['easydrag']") {
		t.Fatal("APP_I18N_SECTIONS must load the easydrag section")
	}
}

func TestEasyDragShellIntegration(t *testing.T) {
	shell := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"easydrag: 'EasyDragApp'",
		"appId === 'easydrag' && window.EasyDragApp",
		"window.EasyDragApp.open(existing.id, context)",
		"new CustomEvent('aurago:flows-changed'",
		"payload.appId === 'easydrag'",
		"const SESSION_CONTEXT_KEYS = ['path', 'category', 'flowId'];",
		"'easydrag': { width: 1320, height: 840 }",
	} {
		if !strings.Contains(shell, marker) {
			t.Errorf("desktop shell misses %q", marker)
		}
	}
	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	start := strings.Index(routing, "if (appId === 'mission-control' && window.MissionControlApp")
	if start < 0 {
		t.Fatal("menus-and-routing.js has no Mission Control render branch")
	}
	mc := routing[start:]
	end := strings.Index(mc, "}));")
	if end < 0 {
		t.Fatal("the Mission Control render branch has no end (\"}));\")")
	}
	if !strings.Contains(mc[:end], "openApp") {
		t.Error("Mission Control needs openApp in its render context to open flows in EasyDrag")
	}
}

func TestEasyDragAppLifecycle(t *testing.T) {
	app := readDesktopAssetText(t, "js/desktop/apps/easydrag.js")
	for _, marker := range []string{
		"window.EasyDragApp = { render, open, dispose, _instances: instances };",
		"function dispose(windowId)",
		"instances.delete(windowId)",
		"inst.ctx.clearWindowMenus(windowId)",
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("easydrag.js misses %q", marker)
		}
	}
	editor := readDesktopAssetText(t, "js/desktop/apps/easydrag-editor.js")
	for _, marker := range []string{"ctx.setWindowBeforeClose(ed.windowId, leave)", "typeof ctx.isActive === 'function' && !ctx.isActive()", "bag.listen(document, 'aurago:flows-changed'"} {
		if !strings.Contains(editor, marker) {
			t.Errorf("easydrag-editor.js misses %q", marker)
		}
	}
	for _, name := range easyDragModules {
		text := readDesktopAssetText(t, "js/desktop/apps/"+name)
		if lines := strings.Count(text, "\n"); lines >= 1100 {
			t.Errorf("%s has %d lines, the desktop budget is 1100", name, lines)
		}
		if !strings.HasPrefix(strings.TrimSpace(text), "//") || !strings.Contains(text, "'use strict';") {
			t.Errorf("%s must start with a purpose comment and use strict mode", name)
		}
	}
}

var easyDragKeyLiteral = regexp.MustCompile(`'(easydrag\.ui\.[a-z0-9_]+)'(\s*\+)?`)

func readEasyDragLang(t *testing.T, lang string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/easydrag/"+lang+".json")), &out); err != nil {
		t.Fatalf("lang/easydrag/%s.json: %v", lang, err)
	}
	return out
}

func TestEasyDragUIKeysExistInAllLocales(t *testing.T) {
	en := readEasyDragLang(t, "en")
	used := map[string]bool{}
	prefixes := map[string]bool{}
	for _, name := range easyDragModules {
		for _, m := range easyDragKeyLiteral.FindAllStringSubmatch(readDesktopAssetText(t, "js/desktop/apps/"+name), -1) {
			if m[2] != "" {
				prefixes[m[1]] = true
			} else {
				used[m[1]] = true
			}
		}
	}
	if len(used) < 250 {
		t.Fatalf("found only %d literal easydrag.ui keys; is the scan broken?", len(used))
	}
	families := map[string][]string{
		"easydrag.ui.status_":      {"pending", "queued", "running", "waiting", "success", "error", "skipped", "cancelled"},
		"easydrag.ui.effect_":      {"sends_message", "writes_files", "controls_devices", "runs_code", "deletes", "system_change"},
		"easydrag.ui.port_":        {"in", "out", "error", "true", "false", "default"},
		"easydrag.ui.connect_":     {"missing", "self", "port", "duplicate", "cycle"},
		"easydrag.ui.key_":         {"invalid", "reserved", "taken", "missing"},
		"easydrag.ui.save_":        {"saved", "dirty", "saving", "invalid", "offline", "failed", "conflict"},
		"easydrag.ui.state_":       {"draft", "published", "changes", "inactive", "run_view", "publish_incomplete"},
		"easydrag.ui.view_":        {"tree", "table", "json"},
		"easydrag.ui.type_":        {"text", "number", "bool", "list", "object", "file", "null"},
		"easydrag.ui.root_":        {"trigger", "run", "flow"},
		"easydrag.ui.cond_type_":   {"auto", "text", "number", "date", "bool"},
		"easydrag.ui.concurrency_": {"queue", "parallel", "skip"},
		"easydrag.ui.notify_":      {"desktop", "push", "telegram", "off"},
		"easydrag.ui.home_filter_": {"all", "active", "inactive", "errors"},
		"easydrag.ui.runs_filter_": {"all", "errors", "tests", "live"},
		"easydrag.ui.detail_":      {"input", "parameters", "output"},
		"easydrag.ui.op_":          {"eq", "ne", "contains", "not_contains", "starts_with", "ends_with", "matches", "gt", "gte", "lt", "lte", "before", "after", "empty", "not_empty", "is_true", "is_false"},
		"easydrag.ui.issue_": {"edge_duplicate", "edge_id_duplicate", "edge_node_missing", "edge_port_invalid", "edge_self", "flow_cycle", "flow_name_required",
			"flow_no_trigger", "flow_schema", "flow_too_many_nodes", "node_id_duplicate", "node_id_invalid", "node_key_duplicate", "node_key_invalid",
			"node_key_reserved", "node_type_unknown", "node_unavailable", "node_unreachable", "param_invalid", "param_required", "template_not_upstream",
			"template_root_unavailable", "template_syntax", "template_unknown_field", "template_unknown_root", "untrusted_data_to_sink",
			"flow_too_many_issues", "flow_node_not_found"},
		// error_ lists every code the flows API and the engine send (internal/flows, internal/server/flows*).
		"easydrag.ui.error_": {"flows_disabled", "flow_bad_request", "flow_cancelled", "flow_internal", "flow_invalid", "flow_locked", "flow_not_found",
			"flow_not_published", "flow_no_trigger", "flow_options_unavailable", "flow_permission_denied", "flow_revision_conflict", "flow_run_limit",
			"flow_run_not_found", "flow_run_timeout", "flow_node_timeout", "flow_tool_error", "flow_tool_denied", "flow_ai_unavailable", "flow_budget_exceeded",
			"flow_secret_unavailable", "flow_restarted", "generic", "network",
			"flow_exists", "flow_mission_ambiguous", "flow_mission_missing", "flow_too_large", "flow_rate_limited", "flow_run_finished", "flow_publish_incomplete",
			"flow_too_many_issues", "flow_port_invalid", "flow_run_output_too_large", "flow_node_abandoned", "flow_runner_panic", "flow_tools_unavailable",
			"node_type_unknown", "flow_ai_output_invalid", "flow_condition_failed", "flow_condition_invalid", "flow_cycle", "flow_file_exists",
			"flow_http_status", "flow_name_required", "flow_node_failed", "flow_node_not_found", "flow_node_panic", "flow_node_unavailable",
			"flow_notify_failed", "flow_output_invalid", "flow_output_too_large", "flow_param_invalid", "flow_schema", "flow_shutdown", "flow_stopped",
			"flow_template_error", "flow_too_many_nodes", "flow_trigger_disabled", "flow_trigger_invalid", "flow_value_type", "flow_wait_too_long",
			// The 503 causes other than flows switched off (FLOWS_DISABLED), audit 2026-10-08 finding 1.2.
			"flow_mission_control_unavailable", "flow_runner_stopped", "flow_vault_unavailable", "flow_request_cancelled"},
	}
	for prefix := range prefixes {
		if _, ok := families[prefix]; !ok {
			t.Errorf("dynamic key family %s has no member list in this test", prefix)
		}
	}
	for prefix, members := range families {
		for _, member := range members {
			used[prefix+member] = true
		}
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := en
		if lang != "en" {
			words = readEasyDragLang(t, lang)
		}
		for key := range used {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("lang/easydrag/%s.json misses %s", lang, key)
			}
		}
	}
}

func TestEasyDragStylesheetUsesThemeTokens(t *testing.T) {
	css := readDesktopAssetText(t, "css/desktop-app-easydrag.css")
	split := strings.Index(css, ".ed-app [data-cat] {")
	if split < 0 {
		t.Fatal("category alias rules missing")
	}
	if hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(css[split:]); hex != "" {
		t.Fatalf("literal colour %s outside the .ed-app alias block", hex)
	}
	for _, marker := range []string{
		"background: var(--ed-bg);", "--ed-bg: var(--vd-theme-app-bg);", "--ed-panel: var(--vd-theme-panel-bg);",
		".ed-app [hidden] { display: none !important; }", "@media (prefers-reduced-motion: reduce)", "@container (max-width: 900px)",
	} {
		if !strings.Contains(css, marker) {
			t.Errorf("desktop-app-easydrag.css misses %q", marker)
		}
	}
}

func TestMissionControlShowsFlowMissions(t *testing.T) {
	for file, markers := range map[string][]string{
		"js/desktop/apps/mission-control.js":          {"const FILTERS = ['all', 'manual', 'scheduled', 'triggered', 'flow', 'errors'];", "openApp('easydrag'", "if (mission && mission.execution_type === 'flow') { openFlow(mission); return; }", "openFlow: 'openFlow'"},
		"js/desktop/apps/mission-control-list.js":     {"case 'flow':", "vd-mc-row-badge--flow", "JSON.stringify(m.flow_triggers || null)"},
		"js/desktop/apps/mission-control-detail.js":   {"data-mc-action=\"openFlow\"", "desktop.mc_flow_card_desc", "mission.execution_type === 'flow' ? '' : prepMarkup()"},
		"js/desktop/apps/mission-control-triggers.js": {"function flowSummary(mission, t, ctx)", "if (mission.execution_type === 'flow') return flowSummary(mission, t, ctx);", "m.execution_type === 'flow'"},
		"js/desktop/apps/mission-control-menus.js":    {"desktop.mc_new_flow", "function isFlow(mission)", "desktop.mc_filter_flow"},
	} {
		text := readDesktopAssetText(t, file)
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Errorf("%s misses %q", file, marker)
			}
		}
	}
}

func TestStandaloneMissionsPageShowsFlowsReadOnly(t *testing.T) {
	text := readDesktopAssetText(t, "js/missions/main.js")
	for _, marker := range []string{"function isFlowMission(id)", "missions.flow_managed", "icons.flow", "m.execution_type === 'flow'"} {
		if !strings.Contains(text, marker) {
			t.Errorf("js/missions/main.js misses %q", marker)
		}
	}
}

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProviderActionsUseReadableAccentContrast(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/config.css")

	pill := configProviderCSSRuleBody(t, css, ".prov-provider-pill")
	for _, want := range []string{
		"background: var(--pw-surface-elevated);",
		"color: var(--pw-text);",
		"border: 1px solid rgba(20, 184, 166, 0.42);",
		"text-shadow: none;",
	} {
		if !strings.Contains(pill, want) {
			t.Fatalf("provider pill contrast style missing %q in block:\n%s", want, pill)
		}
	}
	if strings.Contains(pill, "color: #fff") || strings.Contains(pill, "background: var(--accent)") {
		t.Fatalf("provider pill must not use white text on the bright accent background:\n%s", pill)
	}

	addButton := configProviderCSSRuleBody(t, css, ".prov-section-actions .btn-save.prov-btn-sm")
	for _, want := range []string{
		"background: var(--pw-accent);",
		"color: var(--cfg-on-accent);",
		"border: 1px solid transparent;",
		"text-shadow: none;",
	} {
		if !strings.Contains(addButton, want) {
			t.Fatalf("provider add button contrast style missing %q in block:\n%s", want, addButton)
		}
	}
}

func TestConfigCSSCacheBustForProviderContrast(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "config.html")
	if !strings.Contains(html, `/css/config.css?v={{.BuildVersion}}`) {
		t.Fatal("config.html must version config.css with the running build")
	}
}

func TestConfigProvidersGuardLoadFailureBeforeMutation(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopAssetText(t, "js/config/main.js")
	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"let providersLoaded = false;",
		"let providersLoadError = '';",
		"async function loadProviders()",
		"providersLoaded = true;",
		"providersLoadError =",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing provider load guard marker %q", marker)
		}
	}
	for _, marker := range []string{
		"config.providers.load_failed_title",
		"config.providers.retry_load",
		"async function providerRetryLoad()",
		"function providerEnsureLoaded()",
		"if (!providerEnsureLoaded()) return;",
		"if (!providerEnsureLoaded()) return false;",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing provider mutation guard marker %q", marker)
		}
	}
}

func TestConfigProvidersPricingRequiresSavedNonOpenRouterProvider(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"const providerIsSaved =",
		"provType === 'openrouter'",
		"config.providers.pricing_save_first",
		"url = '/api/providers/pricing?id=' + encodeURIComponent(provId);",
		"url = '/api/openrouter/models';",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing safe pricing marker %q", marker)
		}
	}
	if strings.Contains(providersJS, "} else {\r\n                    url = '/api/openrouter/models';") ||
		strings.Contains(providersJS, "} else {\n                    url = '/api/openrouter/models';") {
		t.Fatal("providers.js still falls back to OpenRouter pricing for every unsaved provider")
	}
}

func TestConfigProvidersModelPricingActionsStayInsidePanel(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/config.css")
	head := configProviderCSSRuleBody(t, css, ".prov-model-pricing-head")
	for _, want := range []string{
		"gap: 0.5rem 0.75rem;",
		"flex-wrap: wrap;",
	} {
		if !strings.Contains(head, want) {
			t.Fatalf("provider model pricing header must allow wrapping; missing %q in block:\n%s", want, head)
		}
	}

	title := configProviderCSSRuleBody(t, css, ".prov-model-pricing-title")
	for _, want := range []string{
		"flex: 1 1 12rem;",
		"min-width: 0;",
	} {
		if !strings.Contains(title, want) {
			t.Fatalf("provider model pricing title must yield space to actions; missing %q in block:\n%s", want, title)
		}
	}

	actions := configProviderCSSRuleBody(t, css, ".prov-model-pricing-actions")
	for _, want := range []string{
		"flex-wrap: wrap;",
		"max-width: 100%;",
		"margin-left: auto;",
	} {
		if !strings.Contains(actions, want) {
			t.Fatalf("provider model pricing actions must wrap inside the panel; missing %q in block:\n%s", want, actions)
		}
	}
}

func TestConfigProvidersWarnBeforeRiskyModalAndDeleteActions(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"function providerModalDirty()",
		"config.providers.discard_changes_confirm_title",
		"config.providers.discard_changes_confirm",
		"function providerReferenceLabel(ref)",
		"p.references",
		"config.providers.delete_references_warning",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing safer action marker %q", marker)
		}
	}
}

func TestConfigProvidersCopilotAndGuardTranslationsExistInAllLocales(t *testing.T) {
	t.Parallel()

	required := []string{
		"config.providers.load_failed_title",
		"config.providers.load_failed_body",
		"config.providers.retry_load",
		"config.providers.load_required",
		"config.providers.pricing_save_first",
		"config.providers.discard_changes_confirm_title",
		"config.providers.discard_changes_confirm",
		"config.providers.delete_references_warning",
		"config.providers.delete_references_item",
		"config.providers.copilot_authorized",
		"config.providers.copilot_not_authorized",
		"config.providers.copilot_requesting",
		"config.providers.copilot_start_auth",
		"config.providers.copilot_visit_code",
		"config.providers.copilot_check_auth",
		"config.providers.copilot_waiting",
		"config.providers.copilot_auth_success",
		"config.providers.copilot_auth_failed",
		"config.providers.copilot_error",
		"config.providers.copilot_unknown_error",
	}
	files, err := filepath.Glob(filepath.Join("lang", "config", "providers", "*.json"))
	if err != nil {
		t.Fatalf("glob provider translations: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("expected all provider language files, got %d", len(files))
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", path, key)
			}
		}
	}
}

func TestConfigProvidersCopilotFlowUsesI18n(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, forbidden := range []string{
		"Requesting...",
		"Start GitHub Authorization",
		"Waiting for authorization...",
		"Visit the URL below and enter the code:",
		"Check Authorization",
		"GitHub Copilot authorized successfully",
		"Copilot auth failed:",
	} {
		if strings.Contains(providersJS, forbidden) {
			t.Fatalf("providers.js contains hardcoded Copilot copy %q", forbidden)
		}
	}
}

func TestConfigProvidersModelLimitsUseOverridesEffectiveValuesAndAllLocales(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"id=\"prov-context-window\"",
		"id=\"prov-max-output-tokens\"",
		"effective_context_window",
		"effective_max_output_tokens",
		"context_window_source",
		"max_output_tokens_source",
		"model_limits_probe_status",
		"providerScheduleLimitRefresh",
		"unknown_model_conservative_limits",
		"Number.isInteger(context_window)",
		"Number.isInteger(max_output_tokens)",
		"providerCompactTokenCount",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing model-limit contract marker %q", marker)
		}
	}

	required := []string{
		"config.providers.card_context_window",
		"config.providers.card_max_output_tokens",
		"config.providers.field_context_window_label",
		"config.providers.field_max_output_tokens_label",
		"config.providers.model_limit_override_help",
		"config.providers.limit_automatic",
		"config.providers.limit_effective",
		"config.providers.limit_source_provider_override",
		"config.providers.limit_source_model_registry",
		"config.providers.limit_source_provider_probe",
		"config.providers.limit_source_global_unknown_primary",
		"config.providers.limit_source_global_cap",
		"config.providers.limit_source_conservative_default",
		"config.providers.unknown_model_limits_warning",
		"config.providers.model_limits_nonnegative_error",
		"config.providers.limit_compact",
		"config.providers.limits_uncertain",
	}
	files, err := filepath.Glob(filepath.Join("lang", "config", "providers", "*.json"))
	if err != nil {
		t.Fatalf("glob provider translations: %v", err)
	}
	if len(files) != 16 {
		t.Fatalf("provider locale count = %d, want 16", len(files))
	}
	for _, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		var values map[string]string
		if unmarshalErr := json.Unmarshal(data, &values); unmarshalErr != nil {
			t.Fatalf("unmarshal %s: %v", path, unmarshalErr)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", path, key)
			}
		}
	}

	helpFiles, err := filepath.Glob(filepath.Join("lang", "help", "*.json"))
	if err != nil {
		t.Fatalf("glob help translations: %v", err)
	}
	if len(helpFiles) != 16 {
		t.Fatalf("help locale count = %d, want 16", len(helpFiles))
	}
	for _, path := range helpFiles {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		var values map[string]string
		if unmarshalErr := json.Unmarshal(data, &values); unmarshalErr != nil {
			t.Fatalf("unmarshal %s: %v", path, unmarshalErr)
		}
		if strings.TrimSpace(values["help.agent.context_window"]) == "" {
			t.Fatalf("%s missing help.agent.context_window", path)
		}
	}
}

func TestConfigProvidersListUsesRolesCompactLimitsAndEditorTabs(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"id=\"prov-list-search\"",
		"id=\"prov-list-type\"",
		"id=\"prov-list-role\"",
		"function providerApplyListFilter()",
		"function providerBindModalTabs()",
		"data-prov-tab=\"identity\"",
		"data-prov-tab=\"model\"",
		"data-prov-tab=\"capabilities\"",
		"data-prov-tab=\"pricing\"",
		"data-prov-tab=\"auth\"",
		"prov-role-chip",
		"providerCompactTokenCount",
		"config.providers.limit_compact",
		"config.providers.limits_uncertain",
		"p.references",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing list/editor UX marker %q", marker)
		}
	}
	if strings.Contains(providersJS, "capPills") {
		t.Fatal("provider cards must not render Tools/JSON/Vision capability pills")
	}

	css := readDesktopAssetText(t, "css/config.css")
	for _, marker := range []string{
		"[data-workspace-page=\"config\"] .prov-toolbar",
		"[data-workspace-page=\"config\"] .prov-provider-card",
		".prov-role-chip",
		".prov-modal-tabs",
		".prov-modal-body",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config.css missing provider list style %q", marker)
		}
	}
	card := configProviderCSSRuleBody(t, css, "[data-workspace-page=\"config\"] .prov-provider-card")
	if strings.Contains(card, "backdrop-filter") || strings.Contains(card, "translateY") {
		t.Fatalf("provider cards must not use glass or hover lift:\n%s", card)
	}

	required := []string{
		"config.providers.search_placeholder",
		"config.providers.filter_type_all",
		"config.providers.filter_role_all",
		"config.providers.filter_no_results",
		"config.providers.auth_key_set",
		"config.providers.auth_key_missing",
		"config.providers.auth_key_optional",
		"config.providers.auth_oauth_needed",
		"config.providers.tab_identity",
		"config.providers.tab_model",
		"config.providers.tab_capabilities",
		"config.providers.tab_pricing",
		"config.providers.tab_auth",
		"config.providers.unused",
		"config.providers.used_as",
		"config.providers.role_primary_llm",
		"config.providers.role_helper_llm",
		"config.providers.role_vision",
		"config.providers.role_speech_to_text",
		"config.providers.role_telephone_agent_llm",
		"config.providers.role_telephone_agent_asr",
		"config.providers.role_embeddings",
		"config.providers.role_llm_guardian",
		"config.providers.role_mission_preparation",
		"config.providers.role_image_generation",
		"config.providers.role_music_generation",
		"config.providers.role_video_generation",
		"config.providers.role_a2a_llm",
	}
	files, err := filepath.Glob(filepath.Join("lang", "config", "providers", "*.json"))
	if err != nil {
		t.Fatalf("glob provider translations: %v", err)
	}
	if len(files) != 16 {
		t.Fatalf("provider locale count = %d, want 16", len(files))
	}
	for _, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		var values map[string]string
		if unmarshalErr := json.Unmarshal(data, &values); unmarshalErr != nil {
			t.Fatalf("unmarshal %s: %v", path, unmarshalErr)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", path, key)
			}
		}
	}
}

func configProviderCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()

	selector = strings.TrimPrefix(selector, `[data-workspace-page="config"] `)
	needle := "\n" + `.pw-page[data-workspace-page="config"] ` + selector + " {"
	start := strings.Index(source, needle)
	if start < 0 {
		t.Fatalf("config CSS missing selector %q", selector)
	}
	start++
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("config CSS selector %q is missing opening brace", selector)
	}
	bodyStart := start + open + 1
	close := strings.Index(source[bodyStart:], "}")
	if close < 0 {
		t.Fatalf("config CSS selector %q is missing closing brace", selector)
	}
	return source[bodyStart : bodyStart+close]
}

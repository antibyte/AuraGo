package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func normalizeSIPContractSource(data []byte) string {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.ReplaceAll(source, "\r", "\n")
}

func TestNormalizeSIPContractSourceHandlesWindowsLineEndings(t *testing.T) {
	if got, want := normalizeSIPContractSource([]byte("first\r\nsecond\rthird\n")), "first\nsecond\nthird\n"; got != want {
		t.Fatalf("normalized source = %q, want %q", got, want)
	}
}

func TestSIPConfigUIUsesSavedStateAndMaskedSecret(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("cfg", "sip.js"))
	if err != nil {
		t.Fatal(err)
	}
	source := normalizeSIPContractSource(data)
	for _, marker := range []string{
		"/api/sip/config", "/api/sip/providers", "/api/sip/setup", "/api/sip/status", "/api/sip/test", "password_set",
		"sipSavedState", "sipComparable(current) !== sipSavedState",
		"config.telephone_agent.sip_summary", `class="sip-settings-grid"`,
		`class="field-group sip-toggle-field"`, `class="toggle"`, `class="slider"`,
		`class="sip-wizard-shell cfg-topic"`, `class="sip-provider-grid"`, `class="sip-advanced"`,
		"confirm_replace", "sipWizardPassword", "sipAdvancedDirty",
		"const canReusePassword = sipConfigState.password_set && sipConfigState.preset_id === provider.id",
		"async function renderSIPSection() {\n    sipWizardStep = 1;",
		"sipWizardPassword = '';\n    sipWizardQuery = '';\n    sipWizardMessage = '';",
		"function sipParseGuidedValues(", "function sipReviewCalling()",
		"function sipMarkClean(", "function sipDeleteProfile()",
		"function sipNormalizeOutboundPayload(",
		"outbound_scope: sipWizardOutboundScope", "inbound_scope: sipWizardInboundEnabled",
		"data-sip-guided-scope", "data-sip-guided-inbound",
		"data-sip-profile=\"delete\"", "method: 'DELETE'",
		"error.code = body.code", "sipTestErrorMessage(",
		"aria-current=\"step\"", "aria-invalid=\"true\"",
		"Array.isArray(state.outbound.allowed_users)",
		"Array.isArray(state.outbound.denied_users)",
		"inbound.denied_callers", "outbound.denied_domains", "outbound.denied_users", "outbound.denied_e164_prefixes",
		"config.sip.policy_precedence", "config.sip.allowed_callers_help", "config.sip.denied_callers_help",
		"function sipHasUnsavedChanges()", "async function sipSaveUnsaved()",
		"function sipDiscardUnsaved()",
		"window.sipHasUnsavedChanges = sipHasUnsavedChanges",
		"window.sipSaveUnsaved = sipSaveUnsaved",
		"window.sipDiscardUnsaved = sipDiscardUnsaved",
		"function sipNotifyDirty()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("SIP config UI missing contract marker %q", marker)
		}
	}
	if strings.Contains(source, "localStorage") || strings.Contains(source, "sessionStorage") {
		t.Fatal("SIP config UI must not persist credentials in browser storage")
	}
}

func TestSIPConfigConnectionTestHasClientTimeout(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("cfg", "sip.js"))
	if err != nil {
		t.Fatal(err)
	}
	source := normalizeSIPContractSource(data)
	for _, marker := range []string{
		"const timeoutMs = Number(requestOptions.timeoutMs || 0);",
		"new AbortController()",
		"timeoutID = setTimeout(() => controller.abort(), timeoutMs);",
		"timeoutError.code = 'timeout';",
		"const currentMessage = document.getElementById('sip-wizard-status');",
		"if (currentMessage) currentMessage.textContent = sipWizardMessage;",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("SIP config connection test missing timeout marker %q", marker)
		}
	}
	if strings.Count(source, "timeoutMs: 20000") != 2 {
		t.Fatalf("SIP registration test timeout must be applied to setup and saved-profile tests")
	}
	if strings.Count(source, "timeoutMs: 5000") != 2 {
		t.Fatalf("SIP status refresh timeout must be applied after setup and saved-profile tests")
	}
}

func TestSIPConfigWiresGlobalSaveBar(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("js", "config", "main.js"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, marker := range []string{
		`typeof window.sipHasUnsavedChanges === 'function' && window.sipHasUnsavedChanges()`,
		`window.sipSaveUnsaved`,
		`window.sipDiscardUnsaved`,
		`if (sipDirty)`,
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("config main.js missing SIP global-save integration marker %q", marker)
		}
	}
}

func TestSIPConfigHasResponsiveLayoutStyles(t *testing.T) {
	page, err := os.ReadFile("config.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `/css/config-sip.css?v={{.BuildVersion}}`) {
		t.Fatal("config page does not load the SIP layout stylesheet")
	}

	data, err := os.ReadFile(filepath.Join("css", "config-sip.css"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, marker := range []string{
		`[data-workspace-page="config"] .sip-settings-grid`, "display: grid",
		"repeat(2, minmax(0, 1fr))", "@media (max-width: 820px)",
		"grid-template-columns: minmax(0, 1fr)", ".sip-toggle-field",
		".sip-wizard-shell", ".sip-provider-grid", ".sip-advanced",
		".sip-mode-card", ".sip-choice-group", ".sip-profile-card",
		".sip-readiness-list", ".sip-delete-confirm", ".sip-password-toggle",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("SIP layout stylesheet missing contract marker %q", marker)
		}
	}
}

func TestSIPConfigTranslationsComplete(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	required := []string{
		"config.section.sip.label", "config.section.sip.desc", "config.sip.title", "config.sip.description",
		"config.sip.password_stored", "config.sip.password_missing", "config.sip.save_first", "config.sip.status_value",
		"config.sip.wizard.advanced", "config.sip.wizard.advanced_hint", "config.sip.wizard.applied",
		"config.sip.wizard.apply", "config.sip.wizard.automatic", "config.sip.wizard.back",
		"config.sip.wizard.category_europe", "config.sip.wizard.category_germany", "config.sip.wizard.category_global",
		"config.sip.wizard.category_local", "config.sip.wizard.category_north_america", "config.sip.wizard.category_pbx",
		"config.sip.wizard.change", "config.sip.wizard.choose", "config.sip.wizard.configured",
		"config.sip.wizard.phone_enable", "config.sip.wizard.phone_enabled", "config.sip.wizard.phone_intro",
		"config.sip.wizard.phone_invalid", "config.sip.wizard.phone_required", "config.sip.wizard.phone_targets",
		"config.sip.wizard.phone_targets_hint", "config.sip.wizard.phone_targets_placeholder", "config.sip.wizard.phone_title",
		"config.sip.wizard.phone_warning",
		"config.sip.wizard.phone_full_title", "config.sip.wizard.phone_full_intro", "config.sip.wizard.phone_full_enable",
		"config.sip.wizard.mode_title", "config.sip.wizard.mode_registration", "config.sip.wizard.mode_registration_desc",
		"config.sip.wizard.mode_desktop", "config.sip.wizard.mode_desktop_desc", "config.sip.wizard.apply_desktop",
		"config.sip.wizard.trusted_peers", "config.sip.wizard.trusted_peers_hint", "config.sip.wizard.trusted_peers_placeholder",
		"config.sip.wizard.trusted_peers_required", "config.sip.wizard.allowed_callers", "config.sip.wizard.allowed_callers_hint",
		"config.sip.wizard.allowed_callers_placeholder", "config.sip.wizard.allowed_callers_required",
		"config.sip.restart_modal_title", "config.sip.restart_modal_message", "config.sip.restart_modal_confirm",
		"config.sip.restart_modal_later",
		"config.sip.wizard.continue", "config.sip.wizard.documentation", "config.sip.wizard.eyebrow",
		"config.sip.wizard.intro", "config.sip.wizard.no_results", "config.sip.wizard.notice_account_server",
		"config.sip.wizard.notice_device_password", "config.sip.wizard.notice_fritzbox_phone",
		"config.sip.wizard.notice_pbx_credentials", "config.sip.wizard.notice_router_recommended",
		"config.sip.wizard.phone_number", "config.sip.wizard.prefer_srv", "config.sip.wizard.progress",
		"config.sip.wizard.replace_confirm", "config.sip.wizard.replace_required", "config.sip.wizard.required",
		"config.sip.wizard.review", "config.sip.wizard.safe_registration", "config.sip.wizard.safe_title",
		"config.sip.wizard.search", "config.sip.wizard.server", "config.sip.wizard.title",
		"config.sip.blocked_targets", "config.sip.blocked_targets_help",
		"config.sip.denied_callers", "config.sip.denied_domains", "config.sip.denied_e164", "config.sip.denied_users",
		"config.sip.policy_intro", "config.sip.policy_precedence", "config.sip.trusted_peers_help",
		"config.sip.allowed_callers_help", "config.sip.denied_callers_help",
		"config.sip.allowed_domains_help", "config.sip.denied_domains_help",
		"config.sip.allowed_users_help", "config.sip.denied_users_help",
		"config.sip.allowed_e164_help", "config.sip.denied_e164_help",
		"config.sip.password_show", "config.sip.password_hide", "config.sip.status_summary",
		"config.sip.status.disabled", "config.sip.status.registered", "config.sip.status.failed",
		"config.sip.profile.ready", "config.sip.profile.account_ready", "config.sip.profile.outbound_ready",
		"config.sip.profile.inbound_off", "config.sip.profile.audio_restart", "config.sip.profile.delete",
		"config.sip.profile.permissions", "config.sip.profile.credentials",
		"config.sip.delete.title", "config.sip.delete.history", "config.sip.delete.confirm",
		"config.sip.diagnostic.dns_failed", "config.sip.diagnostic.unreachable",
		"config.sip.diagnostic.authentication_failed", "config.sip.diagnostic.timeout",
		"config.sip.wizard.calling", "config.sip.wizard.verify", "config.sip.wizard.calling_title",
		"config.sip.wizard.scope_all", "config.sip.wizard.scope_all_desc",
		"config.sip.wizard.scope_domestic", "config.sip.wizard.scope_custom",
		"config.sip.wizard.custom_targets", "config.sip.wizard.custom_targets_required",
		"config.sip.wizard.inbound_enable", "config.sip.wizard.callers_all", "config.sip.wizard.callers_custom",
		"config.sip.wizard.verify_title", "config.sip.wizard.save_test",
	}
	for _, locale := range locales {
		data, err := os.ReadFile(filepath.Join("lang", "config", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Fatalf("%s config locale: %v", locale, err)
		}
		moduleData, err := os.ReadFile(filepath.Join("lang", "config", "sip", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var module map[string]any
		if err := json.Unmarshal(moduleData, &module); err != nil {
			t.Fatalf("%s SIP config module: %v", locale, err)
		}
		flattenSIPTranslations("", module, messages)
		for _, key := range required {
			if strings.TrimSpace(messages[key]) == "" {
				t.Fatalf("%s missing %s", locale, key)
			}
		}
	}
}

func flattenSIPTranslations(prefix string, values map[string]any, target map[string]string) {
	for key, value := range values {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch typed := value.(type) {
		case string:
			target[fullKey] = typed
		case map[string]any:
			flattenSIPTranslations(fullKey, typed, target)
		}
	}
}

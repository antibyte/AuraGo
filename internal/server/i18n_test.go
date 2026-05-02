package server

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/i18n"
	"aurago/ui"
)

func TestLoadI18NSetupKeys(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal("failed to create UI sub-FS:", err)
	}

	i18n.Load(uiFS, slog.Default())

	langs := i18n.GetSupportedLanguages()
	if len(langs) == 0 {
		t.Fatal("no languages loaded at all")
	}

	t.Logf("Loaded %d languages", len(langs))

	// Check that German (de) translations exist and have setup keys
	deJSON := string(i18n.GetJSON("de"))
	if deJSON == "{}" || deJSON == "" {
		t.Fatal("German translations not loaded; loaded languages:", langs)
	}

	var de map[string]string
	if err := json.Unmarshal([]byte(deJSON), &de); err != nil {
		t.Fatal("German translations JSON is invalid:", err)
	}

	t.Logf("German translations: %d keys", len(de))

	mustHave := []string{
		"setup.nav_next",
		"setup.nav_back",
		"setup.plan_title",
		"setup.plan_description",
		"setup.step_label_plan",
		"setup.step_label_0",
		"setup.step_label_quick",
		"setup.step_label_3",
		"setup.header_subtitle",
		"setup.profile_openrouter_name",
		"setup.profile_custom_name",
	}

	for _, key := range mustHave {
		if de[key] == "" {
			t.Errorf("key %q missing or empty in German translations", key)
		} else {
			t.Logf("  %s = %q", key, de[key])
		}
	}

	// Also check English as fallback
	enJSON := string(i18n.GetJSON("en"))
	if enJSON == "{}" || enJSON == "" {
		t.Fatal("English translations not loaded")
	}
	var en map[string]string
	if err := json.Unmarshal([]byte(enJSON), &en); err != nil {
		t.Fatal("English translations JSON is invalid:", err)
	}
	t.Logf("English translations: %d keys", len(en))
	if en["setup.nav_next"] == "" {
		t.Error("setup.nav_next missing from English translations")
	}
}

func TestLoadI18NBackendKeys(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal("failed to create UI sub-FS:", err)
	}

	i18n.Load(uiFS, slog.Default())

	// Check that German (de) translations have backend keys
	deJSON := string(i18n.GetJSON("de"))
	var de map[string]string
	if err := json.Unmarshal([]byte(deJSON), &de); err != nil {
		t.Fatal("German translations JSON is invalid:", err)
	}

	backendMustHave := []string{
		"backend.cmd_unknown",
		"backend.cmd_reset_success",
		"backend.cmd_help_header",
		"backend.budget_disabled",
		"backend.credits_unavailable",
		"backend.addssh_host_user_required",
		"backend.sudopwd_usage",
		"backend.warnings_none",
		"backend.http_method_not_allowed",
		"backend.auth_too_many_login_attempts",
		"backend.auth_invalid_credentials",
		"backend.auth_bad_request",
		"backend.auth_invalid_json",
		"backend.auth_not_configured",
		"backend.auth_unauthorized",
		"backend.auth_password_min_length",
		"backend.auth_internal_error",
		"backend.auth_failed_generate_secret",
		"backend.auth_failed_save_config",
		"backend.auth_password_set",
		"backend.auth_failed_generate_totp_secret",
		"backend.auth_invalid_request",
		"backend.auth_invalid_code",
		"backend.auth_failed_save_totp_config",
		"backend.authenticator_activated",
		"backend.authenticator_deactivated",
		// New handler keys
		"backend.handler_bad_request",
		"backend.handler_command_failed",
		"backend.handler_internal_error",
		"backend.handler_streaming_unsupported",
		"backend.handler_no_messages",
		"backend.handler_followup_circuit_breaker",
		"backend.handler_timeout_error",
		"backend.handler_sync_error",
		"backend.handler_llm_quota_error",
		"backend.handler_llm_auth_error",
		"backend.handler_llm_config_error",
		"backend.handler_empty_batch",
		"backend.handler_concept_content_required",
		"backend.handler_agent_interrupted",
		"backend.handler_failed_parse_form",
		"backend.handler_missing_file",
		"backend.handler_failed_create_dir",
		"backend.handler_failed_write_file",
		"backend.handler_failed_save_file",
		"backend.handler_provider_not_openrouter",
		"backend.handler_credits_fetch_error",
		"backend.handler_image_not_supported",
		"backend.voice_no_audio_file",
		"backend.voice_invalid_format",
		"backend.voice_server_error",
		"backend.voice_save_error",
		"backend.voice_transcription_failed",
		"backend.knowledge_not_configured",
		"backend.knowledge_unavailable",
		"backend.knowledge_invalid_upload",
		"backend.knowledge_missing_file",
		"backend.knowledge_invalid_filename",
		"backend.knowledge_type_not_allowed",
		"backend.knowledge_invalid_destination",
		"backend.knowledge_file_exists",
		"backend.knowledge_cannot_store",
		"backend.knowledge_missing_filename",
		"backend.knowledge_file_not_found",
		"backend.knowledge_delete_failed",
		"backend.personality_id_required",
		"backend.personality_not_found",
		"backend.personality_persist_failed",
		"backend.personality_engine_disabled",
		"backend.personality_invalid_feedback_type",
		"backend.personality_invalid_name",
		"backend.personality_invalid_name_detail",
		"backend.personality_read_only",
		"backend.personality_cannot_delete_active",
		"backend.personality_save_failed",
		"backend.personality_delete_failed",
		"backend.contacts_not_initialized",
		"backend.contacts_list_failed",
		"backend.contacts_create_failed",
		"backend.contacts_missing_id",
		"backend.contacts_not_found",
		"backend.contacts_update_failed",
		"backend.contacts_delete_failed",
		"backend.planner_not_initialized",
		"backend.planner_list_failed",
		"backend.planner_create_failed",
		"backend.planner_missing_id",
		"backend.planner_not_found",
		"backend.planner_update_failed",
		"backend.planner_delete_failed",
		"backend.preferences_invalid_body",
	}

	for _, key := range backendMustHave {
		if de[key] == "" {
			t.Errorf("key %q missing or empty in German backend translations", key)
		} else {
			t.Logf("  DE: %s = %q", key, de[key])
		}
	}

	// Check English as fallback
	enJSON := string(i18n.GetJSON("en"))
	var en map[string]string
	if err := json.Unmarshal([]byte(enJSON), &en); err != nil {
		t.Fatal("English translations JSON is invalid:", err)
	}

	for _, key := range backendMustHave {
		if en[key] == "" {
			t.Errorf("key %q missing or empty in English backend translations", key)
		} else {
			t.Logf("  EN: %s = %q", key, en[key])
		}
	}
}

func TestSetupTemplateI18NInsertion(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal(err)
	}

	i18n.Load(uiFS, slog.Default())

	tmpl, err := template.ParseFS(uiFS, "setup.html")
	if err != nil {
		t.Fatal("failed to parse setup.html template:", err)
	}

	lang := i18n.NormalizeLang("de")
	data := map[string]interface{}{
		"Lang": lang,
		"I18N": i18n.GetJSON(lang),
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatal("failed to execute setup template:", err)
	}

	html := buf.String()

	// Check that the inline script block has a proper I18N assignment
	if !strings.Contains(html, `let I18N = {`) {
		// Find what I18N is set to
		idx := strings.Index(html, "let I18N = ")
		if idx == -1 {
			t.Fatal("I18N assignment not found in rendered HTML")
		}
		snippet := html[idx:]
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("I18N assignment looks wrong: %s", snippet)
	}

	// Verify a known German key appears in the rendered HTML
	if !strings.Contains(html, `Weiter →`) {
		t.Error("German translation 'Weiter →' not found in rendered HTML")
	}

	if !strings.Contains(html, `setup.nav_next`) {
		t.Log("Note: setup.nav_next key not in HTML (expected - only value should appear)")
	}

	t.Logf("Rendered HTML size: %d bytes", len(html))

	// Extract the inline I18N object size
	startIdx := strings.Index(html, "let I18N = ")
	if startIdx != -1 {
		endIdx := strings.Index(html[startIdx:], ";\n")
		if endIdx != -1 {
			i18nLine := html[startIdx : startIdx+endIdx]
			t.Logf("I18N assignment length: %d chars", len(i18nLine))
		}
	}
}

func langKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

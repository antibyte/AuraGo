package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProviderOAuthWizardContract(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"async function providerStartOAuthConnect(providerID, preparedPopup)",
		"function providerLaunchOAuthWindow(providerID, preparedPopup)",
		"'/api/oauth/start?provider=' + encodeURIComponent(providerID) + '&launch=1'",
		"popup.opener = null;",
		"providerPollOAuthUntilConnected(providerID);",
		"function providerSubmitOAuthPaste(providerID)",
		`id="prov-oauth-paste-url"`,
		`id="prov-oauth-paste-submit"`,
		"body: JSON.stringify({ url: pastedURL })",
		"function providerHandleOAuthMessage(event)",
		"function providerHandleOAuthBroadcast(event)",
		"new BroadcastChannel('aurago-oauth')",
		"aurago:oauth-provider-connected",
		"window.addEventListener('message', providerHandleOAuthMessage);",
		"providerOAuthMissingFieldsText(st)",
		"config.providers.oauth_missing_fields",
		"providerRefreshOAuthStatus(data.id)",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers OAuth wizard missing marker %q", marker)
		}
	}
	for _, marker := range []string{
		"const PROVIDER_OAUTH_PRESETS",
		"const PROVIDER_OAUTH_SETUP_LINKS",
		"function providerRefreshOAuthSetupGuide(type)",
		`id="prov-oauth-provider-setup-link"`,
		"config.providers.oauth_setup_redirect_step",
		"config.providers.oauth_setup_credentials_step",
		"config.providers.oauth_setup_save_connect_step",
		"function providerCopyOAuthLoginLink()",
		`id="prov-oauth-copy-login-link"`,
		"auth_url: 'https://accounts.google.com/o/oauth2/v2/auth'",
		"token_url: 'https://oauth2.googleapis.com/token'",
		"https://www.googleapis.com/auth/generative-language.retriever",
		"legacy_scopes: ['openid email https://www.googleapis.com/auth/cloud-platform']",
		"preset.legacy_scopes.includes(value)",
		"function providerOAuthConfigFromInputs(type)",
		"function providerApplyOAuthPresetForType(type, options)",
		`id="prov-oauth-advanced"`,
		"config.providers.oauth_advanced_toggle",
		"config.providers.oauth_browser_hint",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers OAuth browser setup missing marker %q", marker)
		}
	}
	if strings.Contains(providersJS, "!entry.oauth_auth_url || !entry.oauth_token_url || !entry.oauth_client_id") {
		t.Fatal("known OAuth providers must not require users to type auth/token endpoint paths before browser login")
	}
	if strings.Contains(providersJS, "alert(") {
		t.Fatal("providers config module must use modals/toasts instead of alert()")
	}
}

func TestConfigProviderOAuthWizardStyles(t *testing.T) {
	t.Parallel()

	configCSS := strings.ReplaceAll(readDesktopAssetText(t, "css/config.css"), "\r\n", "\n")
	for _, marker := range []string{
		".prov-oauth-connect-panel",
		".prov-oauth-connect-head",
		".prov-oauth-setup-head",
		".prov-oauth-steps",
		".prov-oauth-status.is-ok",
		".prov-oauth-status.is-warn",
		".prov-oauth-status.is-error",
		".prov-oauth-direct-actions",
		".prov-oauth-paste-box",
		".prov-oauth-paste-row",
	} {
		if !strings.Contains(configCSS, marker) {
			t.Fatalf("config CSS missing OAuth wizard marker %q", marker)
		}
	}
}

func TestConfigProviderOAuthTranslationsExistInAllLocales(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("lang", "config", "providers", "*.json"))
	if err != nil {
		t.Fatalf("glob provider translations: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("expected all provider language files, got %d", len(files))
	}
	required := []string{
		"config.providers.oauth_connect_title",
		"config.providers.oauth_connect_hint",
		"config.providers.oauth_connect",
		"config.providers.oauth_missing_fields",
		"config.providers.oauth_field_auth_url",
		"config.providers.oauth_field_token_url",
		"config.providers.oauth_field_client_id",
		"config.providers.oauth_field_auth_type",
		"config.providers.oauth_field_provider",
		"config.providers.oauth_waiting",
		"config.providers.oauth_started",
		"config.providers.oauth_paste_label",
		"config.providers.oauth_paste_placeholder",
		"config.providers.oauth_paste_help",
		"config.providers.oauth_paste_submit",
		"config.providers.oauth_paste_success",
		"config.providers.oauth_save_first",
		"config.providers.oauth_browser_hint",
		"config.providers.oauth_advanced_toggle",
		"config.providers.oauth_advanced_help",
		"config.providers.oauth_preset_google",
		"config.providers.oauth_preset_custom",
		"config.providers.oauth_setup_title",
		"config.providers.oauth_setup_open_provider_settings",
		"config.providers.oauth_setup_provider_settings_hint",
		"config.providers.oauth_setup_custom_provider_hint",
		"config.providers.oauth_setup_redirect_step",
		"config.providers.oauth_setup_credentials_step",
		"config.providers.oauth_setup_save_connect_step",
		"config.providers.oauth_setup_connect_step",
		"config.providers.oauth_setup_callback_step",
		"config.providers.oauth_direct_retry_help",
		"config.providers.oauth_copy_login_link",
		"config.providers.oauth_login_link_copied",
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

func TestGoogleWorkspaceOAuthRevokeUsesDelete(t *testing.T) {
	t.Parallel()

	googleWorkspaceJS := readDesktopAssetText(t, "cfg/google_workspace.js")
	if !strings.Contains(googleWorkspaceJS, "fetch('/api/oauth/revoke?provider=google_workspace', { method: 'DELETE' })") {
		t.Fatal("Google Workspace OAuth disconnect must use DELETE to match /api/oauth/revoke")
	}
}

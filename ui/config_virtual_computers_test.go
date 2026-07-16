package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestVirtualComputersEnabledToggleSyncsPrecisionDraftState(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `'vcCfgToggleEnabled(this)'`) {
		t.Fatal("virtual computers enabled toggle must pass the element to avoid nested quote syntax errors in onclick")
	}
	if strings.Contains(vcJS, `classList.contains("on")`) {
		t.Fatal("virtual computers enabled toggle must not render nested double quotes inside onclick")
	}
	if !strings.Contains(vcJS, `window.AuraConfigState.set('virtual_computers.enabled', nextEnabled)`) {
		t.Fatal("virtual computers enabled toggle must update AuraConfigState so saveConfig includes virtual_computers.enabled")
	}
	if strings.Contains(vcJS, "setDirty(true);\n    renderVirtualComputersSection(null);") {
		t.Fatal("virtual computers enabled toggle must not mark dirty before re-rendering; the old DOM state overwrites the draft")
	}
}

func TestVirtualComputersModeSelectOffersLocalHostAndHidesSSHFields(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `value="local_host"`) {
		t.Fatal("virtual computers setup mode select must offer local_host")
	}
	if !strings.Contains(vcJS, `config.virtual_computers.mode_local_host`) {
		t.Fatal("virtual computers setup mode select must use translated local_host label")
	}
	if !strings.Contains(vcJS, `config.virtual_computers.local_host_note`) {
		t.Fatal("virtual computers UI must show local host requirements")
	}
	if !strings.Contains(vcJS, `vcCfgOnModeChange(this)`) {
		t.Fatal("virtual computers mode select must re-render when switching between local and SSH modes")
	}
}

func TestVirtualComputersBoringdURLDefaultAvoidsCommonCaddyPort(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `http://127.0.0.1:18082`) {
		t.Fatal("virtual computers UI must default boringd_url to the private boringd port 18082")
	}
	if strings.Contains(vcJS, `http://127.0.0.1:8080`) {
		t.Fatal("virtual computers UI must not default boringd_url to 8080 because local Caddy/AuraGo sites commonly use it")
	}
	if strings.Contains(vcJS, `http://127.0.0.1:18080`) {
		t.Fatal("virtual computers UI must not default boringd_url to AuraGo's internal loopback port 18080")
	}
}

func TestVirtualComputersInstallShowsElapsedProgressAndFailures(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	for _, want := range []string{
		`vcCfgPostSetup('/api/virtual-computers/setup/install', 'vc-install-btn', true)`,
		`window.setInterval(updateElapsed, 1000)`,
		`window.clearInterval(elapsedTimer)`,
		`body.status === 'unhealthy'`,
		`body.setup && body.setup.message`,
	} {
		if !strings.Contains(vcJS, want) {
			t.Fatalf("virtual computers setup progress handling missing %q", want)
		}
	}
}

func TestVirtualComputersLocalSetupUsesCentralSudoPasswordVaultField(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	localBranch := strings.Index(vcJS, "if (isLocalHost) {")
	sudoField := strings.Index(vcJS, `id="vc-sudo-password-input"`)
	remoteBranch := -1
	if localBranch >= 0 {
		if offset := strings.Index(vcJS[localBranch:], "} else {"); offset >= 0 {
			remoteBranch = localBranch + offset
		}
	}
	if localBranch < 0 || sudoField < localBranch || remoteBranch < sudoField {
		t.Fatal("sudo password field must be rendered only in the local_host branch")
	}
	if !strings.Contains(vcJS, `'sudo_password'`) {
		t.Fatal("local setup must store the existing central sudo_password Vault key")
	}
	if strings.Contains(vcJS, `data-path="virtual_computers.sudo_password"`) {
		t.Fatal("sudo password must never be serialized into configData")
	}
	if !strings.Contains(vcJS, `body.sudo_password_stored`) {
		t.Fatal("local setup UI must use the safe stored-state boolean from setup status")
	}
}

func TestVirtualComputersDoesNotDeleteSharedSudoPassword(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if strings.Contains(vcJS, `vcCfgClearSudoPassword`) || strings.Contains(vcJS, `method: 'DELETE'`) {
		t.Fatal("Virtual Computers must not delete the globally shared sudo_password")
	}
	if strings.Contains(vcJS, "confirm(") || strings.Contains(vcJS, "alert(") {
		t.Fatal("virtual computers config must not use native confirm or alert dialogs")
	}
}

func TestVirtualComputersSudoStatusRefreshCannotOverwriteMutation(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `const requestGeneration = ++_vcSudoPasswordStatusGeneration`) {
		t.Fatal("sudo status refresh must capture a request generation")
	}
	if !strings.Contains(vcJS, `requestGeneration !== _vcSudoPasswordStatusGeneration`) {
		t.Fatal("stale sudo status responses must be ignored")
	}
	if !strings.Contains(vcJS, `++_vcSudoPasswordStatusGeneration;`) {
		t.Fatal("sudo password mutations must invalidate in-flight status requests")
	}
}

func TestVirtualComputersSudoPasswordDisablesLoginAutofillAndWrapsActions(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `id="vc-sudo-password-input" value="" autocomplete="off"`) {
		t.Fatal("sudo password field must disable login-password autofill")
	}
	if !strings.Contains(vcJS, `<div class="cfg-password-row">`) {
		t.Fatal("sudo password actions must use the responsive wrapping row")
	}
	if strings.Contains(vcJS, `autocomplete="current-password"`) {
		t.Fatal("sudo password field must not request the AuraGo login password")
	}
}

func TestVirtualComputersEmptySudoPasswordDoesNotForgetStoredState(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `_vcSudoPasswordStored ? 'config.virtual_computers.sudo_password_stored'`) {
		t.Fatal("an empty password field must preserve an already confirmed Vault state")
	}
}

func TestVirtualComputersSudoPasswordTranslationsExist(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"config.virtual_computers.sudo_password_label",
		"config.virtual_computers.sudo_password_stored",
		"config.virtual_computers.sudo_password_missing",
		"config.virtual_computers.sudo_password_saved",
		"config.virtual_computers.sudo_password_cleared",
		"config.virtual_computers.sudo_password_clear",
		"config.virtual_computers.sudo_password_clear_confirm",
		"config.virtual_computers.sudo_password_save_failed",
		"help.virtual_computers.sudo_password",
	}
	for _, language := range languages {
		language := language
		t.Run(language, func(t *testing.T) {
			var translations map[string]string
			path := fmt.Sprintf("lang/config/virtual_computers/%s.json", language)
			if err := json.Unmarshal([]byte(mustReadUIFile(t, path)), &translations); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			for _, key := range keys {
				if strings.TrimSpace(translations[key]) == "" {
					t.Errorf("%s is missing translation %s", path, key)
				}
			}
			if !strings.Contains(translations["help.virtual_computers.sudo_password"], "execute_sudo") {
				t.Errorf("%s must explain that sudo_password is shared with execute_sudo", path)
			}
		})
	}
}

func TestVirtualComputersStorageUsesVaultFieldsAndReadOnlyConnectionTest(t *testing.T) {
	t.Parallel()
	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	for _, want := range []string{
		`data-path="virtual_computers.storage.endpoint"`,
		`data-path="virtual_computers.storage.bucket"`,
		`data-path="virtual_computers.storage.region"`,
		`'virtual_computers.storage.use_ssl'`,
		`virtual_computers_s3_access_key_id`,
		`virtual_computers_s3_secret_key`,
		`/api/virtual-computers/storage/test`,
	} {
		if !strings.Contains(vcJS, want) {
			t.Fatalf("virtual computer storage UI missing %q", want)
		}
	}
	if strings.Contains(vcJS, `data-path="virtual_computers.s3_secret_key"`) || strings.Contains(vcJS, `data-path="virtual_computers.s3_access_key_id"`) {
		t.Fatal("S3 credentials must never be serialized into config data")
	}
}

func TestVirtualComputersAgentProviderUsesVaultFields(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	for _, want := range []string{
		`id="vc-anthropic-key-input"`,
		`virtual_computers_anthropic_key`,
		`id="vc-openrouter-key-input"`,
		`virtual_computers_openrouter_key`,
	} {
		if !strings.Contains(vcJS, want) {
			t.Fatalf("virtual computer agent provider UI missing %q", want)
		}
	}
	if strings.Contains(vcJS, `data-path="virtual_computers.boring_anthropic_key"`) ||
		strings.Contains(vcJS, `data-path="virtual_computers.boring_openrouter_key"`) {
		t.Fatal("agent provider credentials must never be serialized into config data")
	}

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"config.virtual_computers.agent_provider_title",
		"config.virtual_computers.anthropic_key_label",
		"config.virtual_computers.openrouter_key_label",
		"help.virtual_computers.anthropic_key",
		"help.virtual_computers.openrouter_key",
	}
	for _, language := range languages {
		var translations map[string]string
		path := fmt.Sprintf("lang/config/virtual_computers/%s.json", language)
		if err := json.Unmarshal([]byte(mustReadUIFile(t, path)), &translations); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(translations[key]) == "" {
				t.Errorf("%s is missing translation %s", path, key)
			}
		}
	}
}

func TestVirtualComputersDesktopUsesCapabilitiesTasksVolumesAndModal(t *testing.T) {
	t.Parallel()
	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, want := range []string{
		`machine.display === true`, `/api/virtual-computers/tasks`, `/api/virtual-computers/volumes`,
		`volume_id`, `ttl_seconds`, `data-role="modal"`, `cancel_agent_task`,
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("virtual computer desktop app missing %q", want)
		}
	}
	if strings.Contains(app, "alert(") || strings.Contains(app, "confirm(") {
		t.Fatal("virtual computer desktop app must use its modal instead of native alert/confirm")
	}
}

func TestVirtualComputersNewTranslationsExist(t *testing.T) {
	t.Parallel()
	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	configKeys := []string{
		"config.virtual_computers.storage_title", "config.virtual_computers.storage_endpoint_label",
		"config.virtual_computers.storage_bucket_label", "config.virtual_computers.storage_region_label",
		"config.virtual_computers.storage_ssl_label", "config.virtual_computers.storage_test_button",
		"config.virtual_computers.s3_access_key_label", "config.virtual_computers.s3_secret_key_label",
	}
	desktopKeys := []string{
		"desktop.virtual_computers_headless", "desktop.virtual_computers_task_start",
		"desktop.virtual_computers_task_cancel", "desktop.virtual_computers_tasks",
		"desktop.virtual_computers_volumes", "desktop.virtual_computers_volume_create",
		"desktop.virtual_computers_volume_import", "desktop.virtual_computers_modal_cancel_task",
	}
	for _, language := range languages {
		for _, item := range []struct {
			path string
			keys []string
		}{
			{fmt.Sprintf("lang/config/virtual_computers/%s.json", language), configKeys},
			{fmt.Sprintf("lang/desktop/%s.json", language), desktopKeys},
		} {
			var translations map[string]string
			if err := json.Unmarshal([]byte(mustReadUIFile(t, item.path)), &translations); err != nil {
				t.Fatalf("decode %s: %v", item.path, err)
			}
			for _, key := range item.keys {
				if strings.TrimSpace(translations[key]) == "" {
					t.Errorf("%s is missing translation %s", item.path, key)
				}
			}
		}
	}
}

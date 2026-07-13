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
	if !strings.Contains(vcJS, `http://127.0.0.1:18080`) {
		t.Fatal("virtual computers UI must default boringd_url to the private boringd port 18080")
	}
	if strings.Contains(vcJS, `http://127.0.0.1:8080`) {
		t.Fatal("virtual computers UI must not default boringd_url to 8080 because local Caddy/AuraGo sites commonly use it")
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

func TestVirtualComputersSudoPasswordCanBeClearedSafely(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `await showConfirm(`) {
		t.Fatal("clearing the sudo password must use the shared confirmation modal")
	}
	if !strings.Contains(vcJS, `method: 'DELETE'`) || !strings.Contains(vcJS, `encodeURIComponent('sudo_password')`) {
		t.Fatal("clear action must delete only the central sudo_password Vault key")
	}
	if strings.Contains(vcJS, "confirm(") || strings.Contains(vcJS, "alert(") {
		t.Fatal("virtual computers config must not use native confirm or alert dialogs")
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
		})
	}
}

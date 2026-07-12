package ui

import (
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

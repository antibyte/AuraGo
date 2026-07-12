package ui

import (
	"strings"
	"testing"
)

func TestVirtualComputersEnabledToggleSyncsPrecisionDraftState(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	if !strings.Contains(vcJS, `window.AuraConfigState.set('virtual_computers.enabled', nextEnabled)`) {
		t.Fatal("virtual computers enabled toggle must update AuraConfigState so saveConfig includes virtual_computers.enabled")
	}
	if strings.Contains(vcJS, "setDirty(true);\n    renderVirtualComputersSection(null);") {
		t.Fatal("virtual computers enabled toggle must not mark dirty before re-rendering; the old DOM state overwrites the draft")
	}
}

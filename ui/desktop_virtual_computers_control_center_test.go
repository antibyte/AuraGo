package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestVirtualComputersDesktopRendersControlCenter(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, marker := range []string{
		`activeSection: 'machines'`,
		`templates: []`,
		`resourceErrors: {`,
		`pendingActions: new Set()`,
		`role="tablist"`,
		`role="tab"`,
		`role="tabpanel"`,
		`id: 'machines'`,
		`id: 'tasks'`,
		`id: 'volumes'`,
		`/api/virtual-computers/templates`,
		`function fallbackTemplates()`,
		`role="dialog" aria-modal="true"`,
		`'confirm-destroy'`,
		`'confirm-delete-volume'`,
		`aria-live="polite"`,
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("virtual computers control center missing %q", marker)
		}
	}
	if strings.Contains(app, "alert(") || strings.Contains(app, "confirm(") {
		t.Fatal("virtual computers control center must use accessible app dialogs")
	}
}

func TestVirtualComputersMachineListPollingContract(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, marker := range []string{
		`const machinePollIntervalMs = 5000;`,
		`machineSnapshot: JSON.stringify([])`,
		`machinePollTimer: null`,
		`machinePollInFlight: false`,
		`function normalizeMachineList(body)`,
		`function storeMachines(state, machines)`,
		`function isMachinePollingVisible(state)`,
		`async function pollMachines(state)`,
		`function scheduleMachineRefresh(state)`,
		`request('/api/virtual-computers/machines')`,
		`document.visibilityState`,
		`closest('.vd-window')`,
		`scheduleMachineRefresh(state);`,
		`clearTimeout(state.machinePollTimer)`,
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("virtual computers machine polling missing %q", marker)
		}
	}

	applyStart := strings.Index(app, `function applyResourceResult`)
	applyEnd := -1
	if applyStart >= 0 {
		if offset := strings.Index(app[applyStart:], `async function refresh`); offset >= 0 {
			applyEnd = applyStart + offset
		}
	}
	if applyStart < 0 || applyEnd <= applyStart || !strings.Contains(app[applyStart:applyEnd], `storeMachines(state, normalizeMachineList(body));`) {
		t.Fatal("normal machine refresh must update the polling snapshot")
	}

	disposeStart := strings.Index(app, `function dispose(windowId)`)
	disposeEnd := -1
	if disposeStart >= 0 {
		if offset := strings.Index(app[disposeStart:], `window.VirtualComputersApp`); offset >= 0 {
			disposeEnd = disposeStart + offset
		}
	}
	if disposeStart < 0 || disposeEnd <= disposeStart || !strings.Contains(app[disposeStart:disposeEnd], `clearTimeout(state.machinePollTimer)`) {
		t.Fatal("disposing a Virtual Computers window must clear machine polling")
	}
}

func TestVirtualComputersExpiryCountdownContract(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, marker := range []string{
		`const expiryCountdownIntervalMs = 1000;`,
		`expiryCountdownTimer: null`,
		`function formatExpiryCountdown(expiresAt, nowMs)`,
		`function expiryCountdownMarkup(state, machine, className)`,
		`function updateExpiryCountdowns(state)`,
		`function scheduleExpiryCountdown(state)`,
		`data-expiry-countdown`,
		`scheduleExpiryCountdown(state);`,
		`clearTimeout(state.expiryCountdownTimer)`,
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("virtual computers expiry countdown missing %q", marker)
		}
	}
}

func TestVirtualComputersDesktopUsesThemeAndContainerContracts(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/desktop-app-virtual-computers.css"))
	for _, marker := range []string{
		"container-type: inline-size;",
		"@container (max-width: 760px)",
		".vc-app.is-vnc-expanded",
		"var(--vd-theme-app-bg)",
		"var(--vd-theme-panel-bg)",
		"var(--vd-theme-control-bg)",
		"var(--vd-theme-border)",
		"white-space: nowrap;",
		"overflow-x: auto;",
		"min-height: 44px;",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, marker) {
			t.Errorf("virtual computers stylesheet missing %q", marker)
		}
	}
}

func TestVirtualComputersControlCenterTranslations(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"desktop.virtual_computers_machines",
		"desktop.virtual_computers_new",
		"desktop.virtual_computers_health_operational",
		"desktop.virtual_computers_health_degraded",
		"desktop.virtual_computers_readonly",
		"desktop.virtual_computers_select_machine",
		"desktop.virtual_computers_template",
		"desktop.virtual_computers_runtime",
		"desktop.virtual_computers_expires",
		"desktop.virtual_computers_display",
		"desktop.virtual_computers_web_ports",
		"desktop.virtual_computers_templates_fallback",
		"desktop.virtual_computers_section_error",
		"desktop.virtual_computers_confirm_destroy",
		"desktop.virtual_computers_confirm_destroy_desc",
		"desktop.virtual_computers_confirm_delete_volume",
		"desktop.virtual_computers_confirm_delete_volume_desc",
		"desktop.virtual_computers_active_jobs",
		"desktop.virtual_computers_completed_jobs",
	}
	for _, language := range languages {
		language := language
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			var messages map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/desktop/%s.json", language)), &messages); err != nil {
				t.Fatalf("parse desktop locale: %v", err)
			}
			for _, key := range keys {
				if strings.TrimSpace(messages[key]) == "" {
					t.Errorf("desktop locale missing non-empty %q", key)
				}
			}
		})
	}
}

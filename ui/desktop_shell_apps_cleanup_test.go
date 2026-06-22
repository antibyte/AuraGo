package ui

import (
	"strings"
	"testing"
)

func TestDesktopCalculatorRegistersWindowCleanup(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/settings-calculator.js")
	if !strings.Contains(source, "function renderCalculator(id)") {
		t.Fatal("renderCalculator missing")
	}
	if !strings.Contains(source, "registerWindowCleanup(id, () =>") {
		t.Fatal("calculator must register window cleanup on close")
	}
}

func TestDesktopQuickConnectCleanupRemovesModals(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	if !strings.Contains(source, "function renderQuickConnect(id)") {
		t.Fatal("renderQuickConnect missing in planning-gallery-music fragment")
	}
	for _, want := range []string{
		"registerWindowCleanup(id, () =>",
		"vd-qc-modal-overlay",
		"activeWS.close()",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("quick connect missing cleanup marker %q", want)
		}
	}
}

func TestDesktopLaunchpadCleanupRemovesModals(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	body := jsFunctionBodyInWindowMenuTest(t, source, "function renderLaunchpad(id)")
	for _, want := range []string{
		"registerWindowCleanup(id, () =>",
		"vd-modal-backdrop",
		"clearTimeout(iconSearchDebounce)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("launchpad missing cleanup marker %q", want)
		}
	}
}

func TestDesktopQuickConnectSetupBadgeUsesI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	if !strings.Contains(source, "desktop.qc_badge_setup") {
		t.Fatal("quick connect must use desktop.qc_badge_setup for template badge")
	}
	if strings.Contains(source, `vd-qc-badge-info">Setup</span>`) {
		t.Fatal("quick connect must not hardcode Setup badge text")
	}
}
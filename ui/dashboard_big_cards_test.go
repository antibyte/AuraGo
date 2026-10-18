package ui

import (
	"strings"
	"testing"
)

// TestDashboardDisclosuresBootAndRestore: initDashboard wires DashDisclosures before the router shows the first
// section, and loadSectionContent re-opens the stored disclosures of its section after a successful load (after the
// Updated stamp, inside the try block, so a failed loader skips both).
func TestDashboardDisclosuresBootAndRestore(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/dashboard/main.js"))
	boot := strings.Index(mainJS, "DashDisclosures.init(document);")
	router := strings.Index(mainJS, "DashRouter.init({ onShow: id => loadSectionContent(id) });")
	if boot < 0 || router < 0 || boot > router {
		t.Errorf("initDashboard must call DashDisclosures.init(document) before DashRouter.init (init at %d, router at %d)", boot, router)
	}
	stamp := strings.Index(mainJS, "t('dashboard.section.updated').replace('{{time}}', new Date().toLocaleTimeString())")
	restore := strings.Index(mainJS, "DashDisclosures.restore(document.getElementById('sec-' + id));")
	warn := strings.Index(mainJS, "console.warn('loadSectionContent failed', id, err);")
	if stamp < 0 || restore < stamp || warn < restore {
		t.Errorf("loadSectionContent must call DashDisclosures.restore after the Updated stamp, inside its try block (stamp %d, restore %d, catch %d)", stamp, restore, warn)
	}
}

package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

var virtualComputersRepairConfigKeys = []string{
	"setup_running", "setup_running_hint", "setup_finished_installed", "setup_finished_failed", "setup_finished_unhealthy",
}

func TestVirtualComputersRepairConfigTranslations(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, language := range languages {
		language := language
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			var messages map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/config/virtual_computers/%s.json", language)), &messages); err != nil {
				t.Fatalf("parse config locale: %v", err)
			}
			for _, key := range virtualComputersRepairConfigKeys {
				if value, ok := messages["config.virtual_computers."+key]; !ok || strings.TrimSpace(value) == "" {
					t.Errorf("config locale %s missing non-empty %q", language, key)
				}
			}
			if !strings.Contains(messages["config.virtual_computers.setup_running"], "{seconds}") {
				t.Errorf("config locale %s setup_running lost the {seconds} placeholder", language)
			}
		})
	}
}

// The config page has no browser harness, so its resume and polling paths are pinned statically.
func TestVirtualComputersConfigResumesRunningSetup(t *testing.T) {
	t.Parallel()

	vcJS := normalizeAssetText(mustReadUIFile(t, "cfg/virtual_computers.js"))
	for _, want := range []string{
		`void vcCfgResumeSetupWatch();`,
		`body.setup.running !== true`,
		`outcome && outcome.network`,
		`window.setInterval(() => { void vcCfgPollSetup(); }, 5000)`,
		`Date.parse(setup.started_at)`,
		`id="vc-setup-hint" hidden`,
		`config.virtual_computers.setup_running`,
		`setup.last_state`,
		`setup.last_message`,
		`btn.disabled = disabled`,
	} {
		if !strings.Contains(vcJS, want) {
			t.Errorf("virtual computers config setup watch missing %q", want)
		}
	}
	// Both timers are cleared when the run ends or the section is gone.
	if strings.Count(vcJS, "window.clearInterval(_vcSetupWatch.") < 2 {
		t.Error("the setup watch must clear its tick and poll timers")
	}
}

func TestVirtualComputersRepairUXStaticContract(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, want := range []string{
		`role="status" data-role="setup-banner"`,
		`role="alert" data-role="repair-banner"`,
		`data-action="run-setup"`,
		`kind: 'run-setup'`,
		`/api/virtual-computers/setup/install', jsonRequest('POST', {})`,
		`if (state.setupPollTimer) clearTimeout(state.setupPollTimer);`,
		`if (state.setupAcceptTimer) clearTimeout(state.setupAcceptTimer);`,
		`data-setup-elapsed`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("virtual computers app missing %q", want)
		}
	}
	workspaces := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers-workspaces.js"))
	if !strings.Contains(workspaces, `${blocked ? ' disabled' : ''}>${machineOptions}</optgroup>`) {
		t.Error("the picker's running computers group must be disabled while Install / Repair runs or is pending")
	}
}

const virtualComputersRepairFixture = `() => {
 window.fixtureStatus.capabilities.agent_control = true;
 window.fixtureMachines = [{id:'vm-1', name:'box', template:'python', status:'running', display:false}];
 window.repairDict = (key, vars) => {
   if (key === 'desktop.virtual_computers_duration_minutes') return vars.count + ' min';
   if (key === 'desktop.virtual_computers_duration_seconds') return vars.count + ' sec';
   if (key === 'desktop.virtual_computers_setup_running') return 'RUNNING ' + vars.elapsed;
   return key;
 };
}`

func loadRepairUXApp(t *testing.T, windowID string, status string) *rod.Page {
	t.Helper()
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(60 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersOverhaulFixture)
	page.MustEval(virtualComputersRepairFixture)
	page.MustEval(`s => { Object.assign(window.fixtureStatus, JSON.parse(s)); }`, status)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/apps/virtual-computers-workspaces.js")))
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js")))
	page.MustEval(`id => window.VirtualComputersApp.render(document.querySelector('#app'), id, {t: window.repairDict})`, windowID)
	return page
}

func TestVirtualComputersRunningSetupBannerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	started := time.Now().Add(-125 * time.Second).UTC().Format(time.RFC3339)
	page := loadRepairUXApp(t, "setup-running", `{"setup":{"running":true,"action":"install","started_at":"`+started+`","finished_at":"","last_state":"","last_message":""}}`)
	banner := page.MustElement(`[data-role="setup-banner"][role="status"]`)
	if text := banner.MustText(); !strings.Contains(text, "RUNNING 2 min") {
		t.Fatalf("running banner does not show the elapsed time: %q", text)
	}
	page.MustElement(`[data-action="select-machine"][data-id="vm-1"]`)
	page.MustElement(`[data-action="assign-agent"][disabled]`)
	if title := page.MustEval(`() => document.querySelector('[data-action="assign-agent"]').title`).Str(); title != "desktop.virtual_computers_assign_blocked" {
		t.Fatalf("assign button lacks the explanation, title = %q", title)
	}
	if page.MustEval(`() => !!document.querySelector('[data-role="repair-banner"]')`).Bool() {
		t.Fatal("the repair banner must not show while Install / Repair runs")
	}
	// The elapsed time moves on with the one-second countdown tick.
	page.MustEval(`() => { const node = document.querySelector('[data-setup-elapsed]'); node.dataset.setupElapsed = String(Date.now() - 305000); }`)
	page.MustWait(`() => document.querySelector('[data-setup-elapsed]').textContent === '5 min'`)
	// The status is re-read while the run is active, and the end of the run clears the banner.
	page.MustEval(`() => { window.fixtureStatus.setup = {running:false, last_state:'installed'}; window.statusReads = window.requests.filter(r => r.endsWith('/setup/status')).length; }`)
	page.MustWait(`() => window.requests.filter(r => r.endsWith('/setup/status')).length > window.statusReads && !document.querySelector('[data-role="setup-banner"]')`)
	page.MustEval(`() => window.VirtualComputersApp.dispose('setup-running')`)
}

func TestVirtualComputersRepairRequiredBannerStartsSetupBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := loadRepairUXApp(t, "repair-required", `{"repair_required":true,"setup":{"running":false}}`)
	page.MustElement(`[data-role="repair-banner"][role="alert"]`)
	page.MustElement(`[data-action="select-machine"][data-id="vm-1"]`)
	page.MustElement(`[data-action="assign-agent"][disabled]`)
	// Install / Repair answers only after the whole run: keep the request open.
	page.MustEval(`() => {
 const inner = window.fetch;
 window.setupPosts = [];
 window.fetch = (path, options = {}) => {
   if ((options.method || 'GET') === 'POST' && path === '/api/virtual-computers/setup/install') {
     window.setupPosts.push(options.body);
     return new Promise(resolve => { window.finishSetup = () => resolve(new Response(JSON.stringify({status:'installed'}), {status:200, headers:{'Content-Type':'application/json'}})); });
   }
   return inner(path, options);
 };
 }`)
	page.MustElement(`[data-role="repair-banner"] [data-action="run-setup"]`).MustClick()
	dialog := page.MustElement(`[role="dialog"]`)
	if !strings.Contains(dialog.MustText(), "desktop.virtual_computers_confirm_run_setup_desc") {
		t.Fatalf("the confirmation does not explain the consequences: %q", dialog.MustText())
	}
	if !page.MustEval(`() => document.querySelector('[data-role="modal-submit"]').classList.contains('danger')`).Bool() {
		t.Fatal("the confirmation is not styled as dangerous")
	}
	page.MustElement(`[data-role="modal-submit"]`).MustClick()
	// The dialog closes after the short wait and the running banner takes over while the POST is still open.
	page.MustElement(`[data-role="setup-banner"][role="status"]`)
	page.MustWait(`() => !document.querySelector('[role="dialog"]')`)
	if got := page.MustEval(`() => JSON.stringify(window.setupPosts)`).Str(); got != `["{}"]` {
		t.Fatalf("install POST bodies = %s, want one empty object", got)
	}
	if page.MustEval(`() => !!document.querySelector('[data-role="repair-banner"]')`).Bool() {
		t.Fatal("the repair banner stayed next to the running banner")
	}
	// Finishing the request refreshes the status.
	page.MustEval(`() => { window.fixtureStatus.repair_required = false; window.fixtureStatus.setup = {running:false, last_state:'installed'}; window.finishSetup(); }`)
	page.MustWait(`() => !document.querySelector('[data-role="setup-banner"]') && !document.querySelector('[data-role="repair-banner"]')`)
	page.MustWait(`() => !document.querySelector('[data-action="assign-agent"][disabled]')`)
	page.MustEval(`() => window.VirtualComputersApp.dispose('repair-required')`)
}

func TestVirtualComputersOlderBackendShowsNoBannerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := loadRepairUXApp(t, "old-backend", `{}`)
	page.MustElement(`[data-action="select-machine"][data-id="vm-1"]`)
	page.MustElement(`[data-action="assign-agent"]:not([disabled])`)
	if page.MustEval(`() => !!document.querySelector('[data-role="setup-banner"], [data-role="repair-banner"]')`).Bool() {
		t.Fatal("a status without setup fields must not show an Install / Repair banner")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('old-backend')`)
}

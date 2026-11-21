package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Desktop language keys the workspace, picker, checkpoint and VNC overhaul added (without the
// "desktop." prefix). Every language file has to carry all of them.
var virtualComputersWorkspaceOverhaulKeys = []string{
	"virtual_computers_ws_state_opening", "virtual_computers_ws_state_ready", "virtual_computers_ws_state_closing",
	"virtual_computers_ws_state_closed", "virtual_computers_ws_state_failed", "virtual_computers_ws_state_lost",
	"virtual_computers_job_state_queued", "virtual_computers_job_state_running", "virtual_computers_job_state_completed",
	"virtual_computers_job_state_canceled", "virtual_computers_job_state_interrupted", "virtual_computers_job_state_failed",
	"virtual_computers_grant_state_pending_approval", "virtual_computers_grant_state_active", "virtual_computers_grant_state_revoked",
	"virtual_computers_grant_state_expired", "virtual_computers_grant_state_consumed",
	"virtual_computers_usage_shell", "virtual_computers_usage_browser",
	"virtual_computers_event_workspace_open_failed", "virtual_computers_event_workspace_opened", "virtual_computers_event_workspace_closed",
	"virtual_computers_event_workspace_released", "virtual_computers_event_workspace_adopted", "virtual_computers_event_shell_exec",
	"virtual_computers_event_job_started", "virtual_computers_event_job_canceled", "virtual_computers_event_file_written",
	"virtual_computers_event_checkpoint_created", "virtual_computers_event_desktop_action", "virtual_computers_event_browser_action",
	"virtual_computers_event_control_changed", "virtual_computers_event_credential_grant_requested", "virtual_computers_event_credential_grant_approved",
	"virtual_computers_event_credential_grant_revoked", "virtual_computers_event_credential_grant_consumed",
	"virtual_computers_event_workspace_lost", "virtual_computers_event_workspace_instance_reset",
	"virtual_computers_network_machine", "virtual_computers_badge_assigned_vm", "virtual_computers_picker_running_computers",
	"virtual_computers_request_placeholder", "virtual_computers_request_count", "virtual_computers_checkpoint_hint",
	"virtual_computers_checkpoint_permanent", "virtual_computers_vnc_view_only_locked",
	"virtual_computers_err_forbidden", "virtual_computers_err_not_found", "virtual_computers_err_owner_mismatch",
	"virtual_computers_err_workspace_manager_unavailable", "virtual_computers_err_job_limit_reached",
	"virtual_computers_err_browser_session_limit_reached", "virtual_computers_err_browser_human_control",
	"virtual_computers_err_agent_control_disabled", "virtual_computers_err_invalid_control_owner",
	"virtual_computers_err_legacy_import_in_progress", "virtual_computers_err_storage_unavailable",
	"virtual_computers_err_desktop_control_unconfirmed", "virtual_computers_err_workspace_timeout",
	"virtual_computers_err_workspace_error",
	"virtual_computers_setup_running", "virtual_computers_repair_required", "virtual_computers_repair_run",
	"virtual_computers_confirm_run_setup", "virtual_computers_confirm_run_setup_desc", "virtual_computers_toast_setup_started",
	"virtual_computers_toast_setup_done", "virtual_computers_toast_setup_failed", "virtual_computers_assign_blocked",
	"virtual_computers_err_control_plane_repair_required",
}

func TestVirtualComputersOverhaulTranslations(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, language := range languages {
		language := language
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			var messages map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/desktop/%s.json", language)), &messages); err != nil {
				t.Fatalf("parse desktop locale: %v", err)
			}
			for _, key := range virtualComputersWorkspaceOverhaulKeys {
				if value, ok := messages["desktop."+key]; !ok || strings.TrimSpace(value) == "" {
					t.Errorf("desktop locale %s missing non-empty %q", language, key)
				}
			}
		})
	}
}

func TestVirtualComputersWorkspaceLabelsAreDedicatedAndAccessible(t *testing.T) {
	t.Parallel()

	workspaces := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers-workspaces.js"))
	for _, forbidden := range []string{"terminal_connecting", "terminal_disconnected", "health_operational", "section_error", "completed_jobs"} {
		if strings.Contains(workspaces, forbidden) {
			t.Errorf("workspace labels still borrow the unrelated key %q", forbidden)
		}
	}
	for _, marker := range []string{"ws_state: 'ws_state_'", "job_state: 'job_state_'", "grant_state: 'grant_state_'", "usage: 'usage_'", "event: 'event_'", "function humanize(", "optgroup", "picker_running_computers", "data-role=\"agent-request-count\"", "badge_assigned_vm", "release_workspace"} {
		if !strings.Contains(workspaces, marker) {
			t.Errorf("workspace pane missing %q", marker)
		}
	}

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/bundles/virtual-computers.bundle.js"))
	for _, marker := range []string{
		`aria-controls="${esc(state.panelId)}"`,
		`id="${esc(state.panelId)}" data-role="content" role="tabpanel"`,
		`class="vc-banner is-error" role="alert"`,
		`role="status"`,
		`state.host.addEventListener('input', state.inputHandler)`,
		`if (state.inputHandler) state.host.removeEventListener('input', state.inputHandler)`,
		`/assign-agent`,
		`data-role="checkpoint-target"`,
		`checkpoint-permanent`,
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("virtual computers app missing %q", marker)
		}
	}
	for _, label := range []string{"refresh", "open_config", "task_start", "new", "screenshot", "vnc_live", "terminal"} {
		if !strings.Contains(app, `aria-label="${esc(tx(c, 'desktop.virtual_computers_`+label+`'))}"`) {
			t.Errorf("icon label button %q has no aria-label", label)
		}
	}

	vnc := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers-vnc.js"))
	for _, marker := range []string{"vnc_maximize", "vnc_restore", "vnc_view_only_locked", "aria-describedby", `data-vnc-action="retry"`} {
		if !strings.Contains(vnc, marker) {
			t.Errorf("VNC controller missing %q", marker)
		}
	}
	if strings.Contains(vnc, "vnc_collapse") {
		t.Error("VNC expand toggle must use the maximize/restore labels")
	}
	terminal := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers-terminal.js"))
	if !strings.Contains(terminal, `data-terminal-action="retry"`) || !strings.Contains(terminal, `data-role="terminal-message"`) {
		t.Error("terminal error overlay needs a retry button")
	}
}

func TestVirtualComputersStylesheetIsScopedAndTokenised(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/desktop-app-virtual-computers.css"))
	if regexp.MustCompile(`(?m)^button:disabled`).MatchString(css) {
		t.Error("button:disabled must be scoped to .vc-app")
	}
	for _, want := range []string{".vc-app button:disabled { opacity: .52; cursor: not-allowed; }", `.vc-app button[aria-busy="true"] { cursor: progress; }`, "--vc-canvas-bg", "--vc-console-bg", "--vc-overlay-bg", "--vc-overlay-fg"} {
		if !strings.Contains(css, want) {
			t.Errorf("stylesheet missing %q", want)
		}
	}
	if strings.Contains(css, "cursor: wait") {
		t.Error("disabled controls must not show the wait cursor")
	}
	for _, dead := range []string{".vc-volume-tools", ".vc-import-field", ".vc-ledger", ".vc-filter", ".vc-detail-skeleton"} {
		if strings.Contains(css, dead) {
			t.Errorf("stylesheet still carries the unused selector %q", dead)
		}
	}
	// Dark colours are defined once, as variables, on .vc-app.
	hex := regexp.MustCompile(`(?:background|color|border[a-z-]*)\s*:[^;{}]*#[0-9a-fA-F]{3,8}\b`)
	for _, line := range strings.Split(css, "\n") {
		if hex.MatchString(line) && !strings.Contains(line, "color-mix") && !strings.Contains(line, "--vc-") {
			t.Errorf("hard-coded colour outside the variables: %s", strings.TrimSpace(line))
		}
	}
	for _, rule := range []string{".vc-vnc-toolbar {", ".vc-terminal-body"} {
		if !strings.Contains(css, rule) {
			t.Errorf("stylesheet missing %q", rule)
		}
	}
}

const virtualComputersWorkspaceFixture = `() => {
 document.body.innerHTML = '<div id="app"></div>';
 window.dict = {
   'desktop.virtual_computers_ws_state_closed': 'Closed!',
   'desktop.virtual_computers_ws_state_ready': 'Ready!',
   'desktop.virtual_computers_release_workspace': 'Release!',
   'desktop.virtual_computers_badge_assigned_vm': 'Assigned VM!'
 };
 window.selectedLaunch = null;
 window.requests = [];
 window.bodies = [];
 window.fixtureMachines = [
   {id:'vm-run', label:'Spare box', template:'python', status:'running', display:false},
   {id:'vm-owned', label:'Build box', template:'desktop', status:'running', display:true, workspace_id:'ws-adopted', workspace_origin:'adopted'}
 ];
 window.fixtureWorkspaces = [
   {id:'ws-adopted', state:'ready', origin:'adopted', template:'desktop', network_profile:'machine', machine_id:'vm-owned', owner_session_id:'virtual-desktop'},
   {id:'ws-closed', state:'closed', origin:'launched', template:'desktop', network_profile:'offline', machine_id:'vm-gone', owner_session_id:'virtual-desktop'},
   {id:'ws-odd', state:'weird_state', origin:'launched', template:'desktop', machine_id:'vm-gone', owner_session_id:'virtual-desktop'}
 ];
 window.fixtureVolumes = [
   {id:'vol-ws', name:'Data', format:'workspace_v2', availability:'available'},
   {id:'vol-legacy', name:'Old', format:'legacy_root', availability:'available'}
 ];
 const json = (body, status) => new Response(JSON.stringify(body), {status:status||200, headers:{'Content-Type':'application/json'}});
 window.fetch = async (path, options = {}) => {
   const method = options.method || 'GET';
   window.requests.push(method + ' ' + path);
   if (options.body) window.bodies.push({path, body:JSON.parse(options.body)});
   if (path.endsWith('/setup/status')) return json({enabled:true, capabilities:{agent_control:true, volumes:true, volume_retention:false}});
   if (path.endsWith('/templates')) return json({templates:[{id:'python',name:'Python',display:false},{id:'desktop',name:'Desktop',display:true}]});
   if (path.endsWith('/volumes')) return json({volumes:window.fixtureVolumes});
   if (path.endsWith('/assign-agent') && method === 'POST') {
     const workspace = {id:'ws-new', state:'ready', origin:'adopted', template:'python', machine_id:'vm-run', owner_session_id:'virtual-desktop'};
     window.fixtureWorkspaces.push(workspace);
     window.fixtureMachines[0].workspace_id = 'ws-new';
     return json({workspace, reused:false});
   }
   if (path.endsWith('/checkpoint') && method === 'POST') return json({status:'ok'});
   if (path.endsWith('/machines')) return json({machines:window.fixtureMachines});
   if (path.endsWith('/workspaces')) return json({workspaces:window.fixtureWorkspaces});
   return json({});
 };
}`

func TestVirtualComputersWorkspaceCardsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersWorkspaceFixture)
	for _, file := range []string{"js/desktop/apps/virtual-computers-workspaces.js", "js/desktop/bundles/virtual-computers.bundle.js"} {
		page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, file)))
	}
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'ws-fixture',{t:k=>window.dict[k]||k,openApp:(app,context)=>window.selectedLaunch={app,context}})`)
	page.MustElement(`[data-action="section"][data-section="workspaces"]`).MustClick()
	page.MustElement(`.vc-workspace-card[data-workspace-id="ws-adopted"]`)

	// Tab and panel are linked.
	if !page.MustEval(`() => { const tab = document.querySelector('[role="tab"][aria-selected="true"]'); const panel = document.querySelector('[role="tabpanel"]'); return !!tab && !!panel && tab.getAttribute('aria-controls') === panel.id && panel.id.length > 0; }`).Bool() {
		t.Fatal("tab does not reference the tab panel")
	}
	// Adopted workspace: machine name, Assigned VM badge, Release instead of Close.
	if !page.MustEval(`() => {
 const card = document.querySelector('.vc-workspace-card[data-workspace-id="ws-adopted"]');
 const close = card.querySelector('[data-action="close-workspace"]');
 return card.querySelector('h3').textContent === 'Build box'
   && card.querySelector('.vc-badge.is-assigned').textContent === 'Assigned VM!'
   && close.textContent === 'Release!' && !close.classList.contains('danger')
   && !!card.querySelector('[data-action="checkpoint-workspace"]')
   && card.querySelector('.vc-state-chip').textContent === 'Ready!';
 }`).Bool() {
		t.Fatalf("adopted card wrong: %s", page.MustEval(`() => document.querySelector('.vc-workspace-card[data-workspace-id="ws-adopted"]').outerHTML`).Str())
	}
	// Closed workspace: translated state, no actions; unknown state is humanized.
	if !page.MustEval(`() => {
 const card = document.querySelector('.vc-workspace-card[data-workspace-id="ws-closed"]');
 const odd = document.querySelector('.vc-workspace-card[data-workspace-id="ws-odd"]');
 return card.querySelector('.vc-state-chip').textContent === 'Closed!'
   && !card.querySelector('[data-action="checkpoint-workspace"], [data-action="close-workspace"], [data-action="observe-workspace"], [data-action="takeover-workspace"]')
   && !card.querySelector('.vc-badge.is-assigned')
   && odd.querySelector('.vc-state-chip').textContent === 'Weird state'
   && !odd.querySelector('[data-action="close-workspace"]')
   && card.querySelector('.vc-meta-grid dd').textContent === 'vm-gone';
 }`).Bool() {
		t.Fatalf("closed card offers actions: %s", page.MustEval(`() => document.querySelector('.vc-workspace-card[data-workspace-id="ws-closed"]').outerHTML`).Str())
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('ws-fixture')`)
}

func TestVirtualComputersRunningMachinePickerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersWorkspaceFixture)
	for _, file := range []string{"js/desktop/apps/virtual-computers-workspaces.js", "js/desktop/bundles/virtual-computers.bundle.js"} {
		page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, file)))
	}
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'picker-fixture',{t:(k,v)=>window.dict[k]||k,openApp:(app,context)=>window.selectedLaunch={app,context}})`)
	page.MustElement(`[data-action="new-agent-job"]:not([disabled])`).MustClick()
	page.MustElement(`[data-role="agent-workspace"] optgroup option[value="machine:vm-run"]`)
	if !page.MustEval(`() => {
 const select = document.querySelector('[data-role="agent-workspace"]');
 const group = select.querySelector('optgroup[label="desktop.virtual_computers_picker_running_computers"]');
 return !!group && group.querySelectorAll('option').length === 1 && group.textContent.includes('Spare box')
   && !select.querySelector('option[value="machine:vm-owned"]') && !!select.querySelector('option[value="ws-adopted"]')
   && !select.querySelector('option[value="ws-closed"]');
 }`).Bool() {
		t.Fatalf("picker groups wrong: %s", page.MustEval(`() => document.querySelector('[data-role="agent-workspace"]').outerHTML`).Str())
	}
	page.MustEval(`() => { document.querySelector('[data-role="agent-workspace"]').value = 'machine:vm-run'; }`)
	page.MustElement(`[data-role="agent-request"]`).MustInput("Check the logs")
	if got := page.MustEval(`() => document.querySelector('[data-role="agent-request-count"]').textContent`).Str(); got != "14 / 4000" {
		t.Fatalf("character counter = %q", got)
	}
	page.MustElement(`[data-action="ask-agent"]`).MustClick()
	page.MustWait(`() => window.selectedLaunch !== null`)
	if got := page.MustEval(`() => window.selectedLaunch.context.window_context.workspace_id`).Str(); got != "ws-new" {
		t.Fatalf("chat opened for %q, want the adopted workspace", got)
	}
	if !page.MustEval(`() => window.requests.includes('POST /api/virtual-computers/machines/vm-run/assign-agent') && window.selectedLaunch.context.chat_prefill === 'Check the logs' && window.selectedLaunch.context.window_context.guide.includes('releases it without destroying')`).Bool() {
		t.Fatal("assign request or adopted guide text missing")
	}
	// The request box is cleared and the new workspace is preselected; the machine left the group.
	page.MustWait(`() => document.querySelector('[data-role="agent-request"]').value === '' && document.querySelector('[data-role="agent-request-count"]').textContent === '0 / 4000' && !document.querySelector('option[value="machine:vm-run"]')`)
	if got := page.MustEval(`() => document.querySelector('[data-role="agent-workspace"]').value`).Str(); got != "ws-new" {
		t.Fatalf("picker selection after assign = %q", got)
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('picker-fixture')`)
	// A disposed window must not keep reacting to input.
	page.MustEval(`() => document.querySelector('#app').dispatchEvent(new Event('input', {bubbles:true}))`)
}

func TestVirtualComputersCheckpointDialogBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersWorkspaceFixture)
	for _, file := range []string{"js/desktop/apps/virtual-computers-workspaces.js", "js/desktop/bundles/virtual-computers.bundle.js"} {
		page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, file)))
	}
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'cp-fixture',{t:(k,v)=>window.dict[k]||k,notify:n=>window.lastToast=n})`)
	page.MustElement(`[data-action="section"][data-section="workspaces"]`).MustClick()
	page.MustElement(`.vc-workspace-card[data-workspace-id="ws-adopted"] [data-action="checkpoint-workspace"]`).MustClick()
	page.MustElement(`[data-role="checkpoint-target"]`)
	if !page.MustEval(`() => {
 const select = document.querySelector('[data-role="checkpoint-target"]');
 const permanent = document.querySelector('[data-role="checkpoint-permanent"]');
 return !!select.querySelector('option[value="vol-ws"]') && !select.querySelector('option[value="vol-legacy"]') && permanent.disabled
   && document.querySelector('[data-role="checkpoint-new-fields"]').hidden === false;
 }`).Bool() {
		t.Fatalf("checkpoint dialog wrong: %s", page.MustEval(`() => document.querySelector('[data-role="modal"]').outerHTML`).Str())
	}
	page.MustElement(`[data-role="checkpoint-name"]`).MustInput("Snapshot one")
	page.MustElement(`[data-action="confirm-checkpoint"]`).MustClick()
	page.MustWait(`() => !document.querySelector('[data-role="modal"]')`)
	if !page.MustEval(`() => { const entry = window.bodies.find(item => item.path.endsWith('/workspaces/ws-adopted/checkpoint')); return !!entry && entry.body.name === 'Snapshot one' && entry.body.persistent === undefined && entry.body.volume_id === undefined; }`).Bool() {
		t.Fatalf("new-volume checkpoint body wrong: %s", page.MustEval(`() => JSON.stringify(window.bodies)`).Str())
	}
	if got := page.MustEval(`() => window.lastToast && window.lastToast.type`).Str(); got != "success" {
		t.Fatalf("checkpoint toast type = %q", got)
	}

	// An existing workspace volume hides the new-volume fields and is sent as volume_id.
	page.MustEval(`() => { window.bodies = []; }`)
	page.MustElement(`.vc-workspace-card[data-workspace-id="ws-adopted"] [data-action="checkpoint-workspace"]`).MustClick()
	page.MustElement(`[data-role="checkpoint-target"]`)
	page.MustEval(`() => { const select = document.querySelector('[data-role="checkpoint-target"]'); select.value = 'vol-ws'; select.dispatchEvent(new Event('change', {bubbles:true})); }`)
	if !page.MustEval(`() => document.querySelector('[data-role="checkpoint-new-fields"]').hidden`).Bool() {
		t.Fatal("new-volume fields stay visible for an existing volume")
	}
	page.MustElement(`[data-action="confirm-checkpoint"]`).MustClick()
	page.MustWait(`() => !document.querySelector('[data-role="modal"]')`)
	if !page.MustEval(`() => { const entry = window.bodies.find(item => item.path.endsWith('/checkpoint')); return !!entry && entry.body.volume_id === 'vol-ws' && entry.body.name === undefined; }`).Bool() {
		t.Fatalf("existing-volume checkpoint body wrong: %s", page.MustEval(`() => JSON.stringify(window.bodies)`).Str())
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('cp-fixture')`)
}

func TestVirtualComputersVNCOverlayAndLabelsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(`() => { document.body.innerHTML = '<div id="mount"></div>'; window.constructed = 0; window.expandedStates = []; }`)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/apps/virtual-computers-vnc.js")))
	page.MustEval(`() => window.VirtualComputersVNC.mount(document.querySelector('#mount'), {url:'ws://x/vnc', machineId:'vm-1', viewOnly:true, lockViewOnly:true, t:k=>k, onExpandedChange:v=>window.expandedStates.push(v)})`)
	// Without the RFB library the overlay offers a retry that reconnects.
	page.MustElement(`[data-role="vnc-message"]:not([hidden]) [data-vnc-action="retry"]`)
	page.MustEval(`() => { window.RFB = function () { window.constructed++; this.addEventListener = () => {}; this.disconnect = () => {}; }; }`)
	page.MustElement(`[data-vnc-action="retry"]`).MustClick()
	page.MustWait(`() => window.constructed === 1 && document.querySelector('[data-role="vnc-message"]').hidden`)

	if !page.MustEval(`() => {
 const button = document.querySelector('[data-vnc-action="view-only"]');
 const note = document.getElementById(button.getAttribute('aria-describedby'));
 return button.disabled && button.title === 'desktop.virtual_computers_vnc_view_only_locked' && !!note && note.textContent === 'desktop.virtual_computers_vnc_view_only_locked';
 }`).Bool() {
		t.Fatal("locked view-only button lacks its explanation")
	}
	expand := page.MustElement(`[data-vnc-action="expand"]`)
	if got := expand.MustText(); got != "desktop.virtual_computers_vnc_maximize" {
		t.Fatalf("collapsed expand label = %q", got)
	}
	expand.MustClick()
	if got := expand.MustText(); got != "desktop.virtual_computers_vnc_restore" {
		t.Fatalf("expanded expand label = %q", got)
	}
}

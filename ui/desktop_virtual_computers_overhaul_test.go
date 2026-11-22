package ui

import (
	"strings"
	"testing"
	"time"
)

func TestVirtualComputersModalAndConfirmContract(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/bundles/virtual-computers.bundle.js"))
	for _, marker := range []string{
		`function openConfirmModal(state`,
		`data-autofocus`,
		`aria-busy`,
		`returnSelector`,
		`renderKey`,
		`role="progressbar"`,
		`/assign-agent`,
		`function sendToAgent(state, workspaceId, requestText, adopted)`,
		`function errorText(state, error)`,
		`function runModalAction(state`,
		`data-role="modal-error"`,
		`data-role="modal-submit"`,
		`state.changeHandler`,
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("virtual computers overhaul missing %q", marker)
		}
	}
	for _, forbidden := range []string{"confirm(", "alert(", "prompt("} {
		if strings.Contains(app, forbidden) {
			t.Errorf("virtual computers must not use native dialog %q", forbidden)
		}
	}
	if !strings.Contains(app, "kind: adopted ? 'release-workspace' : 'close-workspace'") {
		t.Error("closing a workspace must confirm, with release wording for adopted workspaces")
	}
	// Every destructive or security relevant workspace action goes through the confirm dialog.
	for _, kind := range []string{"'destroy'", "'delete-volume'", "'cancel-workspace-job'", "'approve-workspace-grant'", "'revoke-workspace-grant'", "'takeover-workspace'"} {
		if !strings.Contains(app, "kind: "+kind) {
			t.Errorf("virtual computers confirm dialog missing kind %s", kind)
		}
	}
}

const virtualComputersOverhaulFixture = `() => {
 document.body.innerHTML = '<div id="app"></div>';
 window.fixtureStatus = {enabled:true, capabilities:{volumes:true, agent_control:true, persistent:true, internet:true, volume_retention:false}};
 window.fixtureMachines = [];
 window.fixtureVolumes = [];
 window.requests = [];
 const json = (body, status) => new Response(JSON.stringify(body), {status:status||200, headers:{'Content-Type':'application/json'}});
 window.fetch = async (path, options = {}) => {
   window.requests.push((options.method || 'GET') + ' ' + path);
   if (path.endsWith('/setup/status')) return json(window.fixtureStatus);
   if (path.endsWith('/templates')) return json({templates:[{id:'python',name:'Python',display:false},{id:'desktop',name:'Desktop',display:true}]});
   if (path.endsWith('/machines')) return json({machines:window.fixtureMachines});
   if (path.endsWith('/volumes')) return json({volumes:window.fixtureVolumes});
   if (path.endsWith('/workspaces')) return json({workspaces:[]});
   return json({});
 };
}`

func TestVirtualComputersModalSurvivesPollBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersOverhaulFixture)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/bundles/virtual-computers.bundle.js")))
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'poll-fixture',{t:k=>k})`)
	page.MustElement(`[data-action="new-machine"]:not([disabled])`).MustClick()
	input := page.MustElement(`[data-role="launch-name"]`)
	input.MustInput("Build box")
	page.MustEval(`() => { window.launchNameInput = document.querySelector('[data-role="launch-name"]'); window.launchNameInput.setSelectionRange(2, 5); }`)
	// An explicit refresh and a changed machine list (found by the background poll) must keep the dialog.
	page.MustEval(`() => { window.fixtureMachines = [{id:'vm-poll', name:'poll-machine', template:'python', status:'running', display:false}]; document.querySelector('[data-action="refresh"]').click(); }`)
	page.MustElement(`.vc-machine[data-id="vm-poll"]`)
	if !page.MustEval(`() => {
   const field = document.querySelector('[data-role="launch-name"]');
   return field === window.launchNameInput && field.value === 'Build box' && document.activeElement === field && field.selectionStart === 2 && field.selectionEnd === 5;
 }`).Bool() {
		t.Fatal("a refresh rebuilt the open launch dialog or reset its input")
	}
	page.MustEval(`() => { window.fixtureMachines = [{id:'vm-poll', name:'poll-machine', template:'python', status:'running', display:false}, {id:'vm-late', name:'late-machine', template:'python', status:'running', display:false}]; }`)
	page.MustElement(`.vc-machine[data-id="vm-late"]`)
	if !page.MustEval(`() => {
   const field = document.querySelector('[data-role="launch-name"]');
   return field === window.launchNameInput && field.value === 'Build box' && document.querySelectorAll('[data-role="modal"]').length === 1;
 }`).Bool() {
		t.Fatal("the machine poll rebuilt the open launch dialog")
	}
	// Escape closes the dialog and returns focus to the control that opened it.
	page.MustEval(`() => { document.querySelector('[data-role="launch-name"]').dispatchEvent(new KeyboardEvent('keydown', {key:'Escape', bubbles:true})); }`)
	if !page.MustEval(`() => !document.querySelector('[data-role="modal"]') && document.activeElement && document.activeElement.dataset.action === 'new-machine'`).Bool() {
		t.Fatal("closing the dialog did not return focus to the launch button")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('poll-fixture')`)
}

func TestVirtualComputersVolumesPageBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersOverhaulFixture)
	page.MustEval(`() => {
 window.fixtureStatus.limits = {max_ttl_seconds:900, default_ttl_seconds:600, volume_max_ttl_seconds:604800, volume_quota_mb:100};
 const soon = new Date(Date.now() + 3 * 86400000).toISOString();
 window.fixtureVolumes = [
   {id:'vol-locked', name:'Archive', locked:true, persistent:true, availability:'available', format:'workspace_v2', used_bytes:50*1048576, quota_mb:100, files:12, attached_to:[]},
   {id:'vol-busy', name:'', locked:false, persistent:false, expires_at:soon, availability:'available', format:'legacy_root', used_bytes:1024, quota_mb:100, attached_to:[{kind:'workspace', id:'ws-1'}]},
   {id:'vol-old', name:'Old', locked:false, persistent:false, expires_at:soon, availability:'previous_store', format:'workspace_v2', used_bytes:0, attached_to:[]}];
 }`)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/bundles/virtual-computers.bundle.js")))
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'volumes-fixture',{t:k=>k})`)
	page.MustElement(`[data-action="refresh"]:not([disabled])`)
	page.MustEval(`() => document.querySelector('[data-action="section"][data-section="volumes"]').click()`)
	page.MustElement(`.vc-volume-card[data-volume-id="vol-locked"]`)
	if !page.MustEval(`() => !!document.querySelector('.vc-volume-banner[role="status"]')`).Bool() {
		t.Fatal("missing retention capability must show the Install / Repair banner")
	}
	if !page.MustEval(`() => {
   const locked = document.querySelector('[data-action="delete-volume"][data-id="vol-locked"]');
   const busy = document.querySelector('[data-action="delete-volume"][data-id="vol-busy"]');
   const free = document.querySelector('[data-action="delete-volume"][data-id="vol-old"]');
   return locked.disabled && locked.title && busy.disabled && busy.title && !free.disabled;
 }`).Bool() {
		t.Fatal("delete must be disabled, with a reason, for locked and attached volumes only")
	}
	if !page.MustEval(`() => {
   const bar = document.querySelector('[data-volume-id="vol-locked"] [role="progressbar"]');
   const lock = document.querySelector('[data-action="toggle-volume-lock"][data-id="vol-locked"]');
   return bar && bar.getAttribute('aria-valuenow') === '50' && lock.disabled;
 }`).Bool() {
		t.Fatal("usage bar value or retention gating wrong without the capability")
	}
	page.MustElement(`[data-action="create-volume"]`)
	page.MustEval(`() => document.querySelector('[data-action="create-volume"]').click()`)
	page.MustElement(`[data-role="volume-retention"]`)
	if !page.MustEval(`() => {
   const select = document.querySelector('[data-role="volume-retention"]');
   const values = Array.from(select.options).map(option => option.value);
   const permanent = select.querySelector('option[value="permanent"]');
   return values.includes('86400') && values.includes('604800') && !values.includes('2592000') && permanent.disabled && document.querySelector('[data-role="volume-lock"]').disabled;
 }`).Bool() {
		t.Fatal("create dialog must hide the 30 day option above the configured maximum and disable permanent and lock without the capability")
	}
	page.MustEval(`() => document.querySelector('[data-action="close-modal"]').click()`)
	page.MustEval(`() => { window.fixtureStatus.capabilities.volume_retention = true; window.fixtureStatus.limits.volume_max_ttl_seconds = 2592000; document.querySelector('[data-action="refresh"]').click(); }`)
	page.MustWait(`() => !document.querySelector('.vc-volume-banner')`)
	page.MustElement(`[data-action="create-volume"]`)
	page.MustEval(`() => document.querySelector('[data-action="create-volume"]').click()`)
	if !page.MustEval(`() => {
   const select = document.querySelector('[data-role="volume-retention"]');
   const values = Array.from(select.options).map(option => option.value);
   return values.includes('2592000') && !select.querySelector('option[value="permanent"]').disabled && !document.querySelector('[data-role="volume-lock"]').disabled;
 }`).Bool() {
		t.Fatal("create dialog must offer 30 days, permanent and lock once retention is available")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('volumes-fixture')`)
}

func TestVirtualComputersOwnedMachineDestroyDisabledBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersOverhaulFixture)
	page.MustEval(`() => {
 window.fixtureMachines = [
   {id:'vm-owned', label:'Build box', name:'vm-owned', template:'desktop', status:'running', display:true, workspace_id:'ws-9', workspace_origin:'adopted'},
   {id:'vm-free', name:'free-machine', template:'custom_tpl', status:'starting', display:false}];
 }`)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/bundles/virtual-computers.bundle.js")))
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'owned-fixture',{t:k=>k})`)
	page.MustElement(`.vc-machine[data-id="vm-owned"]`)
	if !page.MustEval(`() => {
   const detail = document.querySelector('.vc-machine-detail');
   const destroy = detail.querySelector('[data-action="destroy"]');
   return detail.querySelector('h3').textContent === 'Build box' && destroy.disabled && destroy.title && !!detail.querySelector('[data-action="open-workspace"][data-id="ws-9"]') && !!detail.querySelector('.vc-badge.is-assigned');
 }`).Bool() {
		t.Fatal("an owned machine must show its name and badge, disable destroy with a reason and offer the workspace")
	}
	if !page.MustEval(`() => document.querySelector('.vc-machine[data-id="vm-free"]').textContent.includes('Custom tpl') && document.querySelector('.vc-machine[data-id="vm-free"]').textContent.includes('Starting')`).Bool() {
		t.Fatal("unknown template and status values need a humanized fallback")
	}
	page.MustElement(`.vc-machine[data-id="vm-free"]`).MustClick()
	page.MustElement(`.vc-machine-detail [data-action="destroy"]:not([disabled])`).MustClick()
	page.MustElement(`[data-role="modal"] [role="dialog"], [data-role="modal"] .vc-modal`)
	if !page.MustEval(`() => document.activeElement.dataset.action === 'close-modal' && document.activeElement.hasAttribute('data-autofocus')`).Bool() {
		t.Fatal("the destroy confirmation must focus Cancel")
	}
	page.MustEval(`() => document.querySelector('[data-action="close-modal"]').click()`)
	if !page.MustEval(`() => !document.querySelector('[data-role="modal"]') && document.activeElement.dataset.action === 'destroy'`).Bool() {
		t.Fatal("cancelling must return focus to the destroy button")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('owned-fixture')`)
}

func TestVirtualComputersMachineActionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(40 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersOverhaulFixture)
	page.MustEval(`() => {
 window.fixtureStatus.limits = {max_ttl_seconds:600, default_ttl_seconds:600};
 window.posts = [];
 window.launched = null;
 const base = window.fetch;
 window.fetch = async (path, options = {}) => {
   const method = options.method || 'GET';
   const json = (body, status) => new Response(JSON.stringify(body), {status:status||200, headers:{'Content-Type':'application/json'}});
   if (method === 'POST' && path.endsWith('/machines')) {
     window.posts.push(JSON.parse(options.body));
     const machine = {id:'vm-new', label:JSON.parse(options.body).name, template:'python', status:'running', display:false};
     window.fixtureMachines.push(machine);
     return json({status:'ok', machine});
   }
   if (method === 'POST' && path.endsWith('/assign-agent')) return json({status:'ok', workspace:{id:'ws-77'}, reused:false});
   if (method === 'DELETE' && path.includes('/machines/')) return json({status:'error', code:'machine_owned_by_workspace', error:'owned by a workspace'}, 409);
   return base(path, options);
 };
 }`)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/bundles/virtual-computers.bundle.js")))
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'actions-fixture',{t:k=>k,openApp:(app,context)=>{window.launched={app,context};}})`)
	page.MustElement(`[data-action="refresh"]:not([disabled])`)
	page.MustEval(`() => document.querySelector('[data-action="new-machine"]').click()`)
	if !page.MustEval(`() => Array.from(document.querySelector('[data-role="ttl"]').options).map(option => option.value).join(',') === '300,600'`).Bool() {
		t.Fatal("the runtime options must be limited to the configured maximum")
	}
	page.MustElement(`[data-role="launch-name"]`).MustInput("Alpha")
	page.MustEval(`() => document.querySelector('[data-action="confirm-launch"]').click()`)
	page.MustWait(`() => window.posts.length === 1 && !document.querySelector('[data-role="modal"]')`)
	if !page.MustEval(`() => window.posts[0].name === 'Alpha' && window.posts[0].ttl_seconds === 600 && document.querySelector('.vc-machine.is-active').dataset.id === 'vm-new'`).Bool() {
		t.Fatalf("launch body or selection wrong: %s", page.MustEval(`() => JSON.stringify(window.posts[0])`).Str())
	}
	// Assigning a computer posts the adoption and opens the chat with the returned workspace.
	page.MustEval(`() => document.querySelector('[data-action="assign-agent"][data-id="vm-new"]').click()`)
	page.MustEval(`() => document.querySelector('[data-action="confirm-assign-agent"]').click()`)
	if !page.MustEval(`() => !!document.querySelector('[data-role="modal-error"]:not([hidden])') && !!document.querySelector('[data-role="modal"]')`).Bool() {
		t.Fatal("an empty task must keep the dialog open with an inline error")
	}
	page.MustElement(`[data-role="assign-text"]`).MustInput("Install the tools")
	page.MustEval(`() => document.querySelector('[data-action="confirm-assign-agent"]').click()`)
	page.MustWait(`() => window.launched !== null && !document.querySelector('[data-role="modal"]')`)
	if !page.MustEval(`() => window.launched.context.window_context.workspace_id === 'ws-77' && window.launched.context.window_context.guide.includes('handed over') && window.launched.context.chat_prefill === 'Install the tools'`).Bool() {
		t.Fatal("assigning did not open the agent chat with the adopted workspace")
	}
	// A failed destroy stays inline in the dialog and leaves it usable.
	page.MustEval(`() => document.querySelector('[data-action="destroy"][data-id="vm-new"]').click()`)
	page.MustEval(`() => document.querySelector('[data-action="modal-confirm"]').click()`)
	page.MustWait(`() => { const error = document.querySelector('[data-role="modal-error"]'); return error && !error.hidden && error.textContent.includes('owned by a workspace'); }`)
	if !page.MustEval(`() => { const submit = document.querySelector('[data-role="modal-submit"]'); return !submit.disabled && !submit.hasAttribute('aria-busy'); }`).Bool() {
		t.Fatal("a failed action must leave the confirmation button usable")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('actions-fixture')`)
}

package ui

import (
	"testing"
	"time"
)

func TestVirtualComputersWorkspaceSelectionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(30 * time.Second)
	page.MustWaitLoad()
	page.MustEval(`() => {
 document.body.innerHTML = '<div id="app"></div>';
 window.selectedLaunch = null;
 window.fixtureWorkspaces = [
 {id:'ws-desktop', state:'ready', template:'desktop', owner_session_id:'virtual-desktop'},
 {id:'ws-other', state:'ready', template:'desktop', owner_session_id:'other-chat'},
 {id:'ws-closed', state:'closed', template:'desktop', owner_session_id:'virtual-desktop'}];
 window.fetch = async path => new Response(JSON.stringify(path.endsWith('/status') ? {enabled:true, capabilities:{agent_control:true}} : path.endsWith('/workspaces') ? {workspaces:window.fixtureWorkspaces} : {}), {headers:{'Content-Type':'application/json'}});
 }`)
	for _, file := range []string{"js/desktop/apps/virtual-computers-workspaces.js", "js/desktop/apps/virtual-computers.js"} {
		page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, file)))
	}
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'fixture',{t:k=>k,openApp:(app,context)=>window.selectedLaunch={app,context}})`)
	page.MustElement(`[data-action="new-agent-job"]:not([disabled])`).MustClick()
	page.MustElement(`[data-role="agent-workspace"] option[value="ws-desktop"]`)
	if page.MustEval(`() => !!document.querySelector('[data-role="agent-workspace"] option[value="ws-other"], [data-role="agent-workspace"] option[value="ws-closed"]')`).Bool() {
		t.Fatal("inaccessible workspace offered")
	}
	page.MustEval(`() => { document.querySelector('[data-role="agent-workspace"]').value='ws-desktop'; }`)
	page.MustElement(`[data-role="agent-request"]`).MustInput("Open the visible browser")
	page.MustElement(`[data-action="ask-agent"]`).MustClick()
	if got := page.MustEval(`() => window.selectedLaunch.context.window_context.workspace_id`).Str(); got != "ws-desktop" {
		t.Fatalf("wrong workspace: %q", got)
	}
	// A refresh must retain the selection, including when it becomes unavailable.
	page.MustEval(`() => { window.fixtureWorkspaces[0].state='closed'; }`)
	page.MustElement(`[data-action="refresh"]`).MustClick()
	page.MustElement(`[data-role="agent-workspace"] option[value="ws-desktop"][disabled]`)
	page.MustEval(`() => { window.selectedLaunch=null; }`)
	page.MustElement(`[data-role="agent-request"]`).MustInput("Do not substitute another workspace")
	page.MustElement(`[data-action="ask-agent"]`).MustClick()
	if !page.MustEval(`() => window.selectedLaunch === null`).Bool() {
		t.Fatal("unavailable selection was silently substituted")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('fixture')`)
}

func TestVirtualComputersManualNetworkChoiceBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(30 * time.Second)
	page.MustWaitLoad()
	page.MustEval(`() => {
 document.body.innerHTML = '<div id="app"></div>';
 window.fixtureInternet = true;
 window.fixtureMachines = [];
 window.launchBodies = [];
 const json = body => new Response(JSON.stringify(body), {headers:{'Content-Type':'application/json'}});
 window.fetch = async (path, options = {}) => {
   if (path.endsWith('/setup/status')) return json({enabled:true, capabilities:{internet:window.fixtureInternet}});
   if (path.endsWith('/templates')) return json({templates:[{id:'python',name:'Python',display:false},{id:'desktop',name:'Desktop',display:true}]});
   if (path.endsWith('/machines') && options.method === 'POST') {
     const body = JSON.parse(options.body);
     window.launchBodies.push(body);
     const machine = {id:'vm-' + window.launchBodies.length, template:body.template, status:'running', display:body.template==='desktop'};
     window.fixtureMachines.push(machine);
     return json({machine});
   }
   if (path.endsWith('/machines')) return json({machines:window.fixtureMachines});
   return json({});
 };
 }`)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js")))
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'network-fixture',{t:k=>k})`)
	page.MustElement(`[data-action="new-machine"]:not([disabled])`).MustClick()
	if got := page.MustEval(`() => document.querySelector('[data-role="network"]').value`).Str(); got != "internet" {
		t.Fatalf("enabled internet gate should default new computer to internet, got %q", got)
	}
	page.MustElement(`[data-action="confirm-launch"]`).MustClick()
	page.MustWait(`() => window.launchBodies.length === 1 && !document.querySelector('[data-role="modal"]')`)
	if !page.MustEval(`() => window.launchBodies[0].allow_internet === true && window.launchBodies[0].template === 'python'`).Bool() {
		t.Fatalf("headless launch did not request internet: %s", page.MustEval(`() => JSON.stringify(window.launchBodies[0])`).Str())
	}
	page.MustElement(`[data-action="new-machine"]:not([disabled])`).MustClick()
	page.MustEval(`() => { document.querySelector('[data-role="network"]').value = 'offline'; }`)
	page.MustElement(`[data-action="confirm-launch"]`).MustClick()
	page.MustWait(`() => window.launchBodies.length === 2 && !document.querySelector('[data-role="modal"]')`)
	if !page.MustEval(`() => window.launchBodies[1].allow_internet === false`).Bool() {
		t.Fatalf("offline launch requested internet: %s", page.MustEval(`() => JSON.stringify(window.launchBodies[1])`).Str())
	}
	page.MustEval(`() => {
 window.VirtualComputersApp.dispose('network-fixture');
 window.fixtureInternet = false;
 window.VirtualComputersApp.render(document.querySelector('#app'),'network-fixture',{t:k=>k});
 }`)
	page.MustElement(`[data-action="new-machine"]:not([disabled])`).MustClick()
	if !page.MustEval(`() => {
   const select = document.querySelector('[data-role="network"]');
   return select.disabled && select.value === 'offline' && !select.querySelector('[value="internet"]');
 }`).Bool() {
		t.Fatal("disabled internet gate offered an online manual launch")
	}
	page.MustElement(`[data-action="confirm-launch"]`).MustClick()
	page.MustWait(`() => window.launchBodies.length === 3`)
	if !page.MustEval(`() => window.launchBodies[2].allow_internet === false`).Bool() {
		t.Fatal("launch bypassed disabled internet gate")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('network-fixture')`)
}

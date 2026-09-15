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

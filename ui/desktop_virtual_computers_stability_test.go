package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func loadWorkspaceFixtureApp(t *testing.T, windowID string) *rod.Page {
	t.Helper()
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(60 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersWorkspaceFixture)
	for _, file := range []string{"js/desktop/apps/virtual-computers-workspaces.js", "js/desktop/bundles/virtual-computers.bundle.js"} {
		page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, file)))
	}
	page.MustEval(`id => window.VirtualComputersApp.render(document.querySelector('#app'),id,{t:k=>window.dict[k]||k})`, windowID)
	page.MustElement(`[data-action="section"][data-section="workspaces"]`).MustClick()
	page.MustElement(`.vc-workspace-card[data-workspace-id="ws-adopted"]`)
	return page
}

func TestVirtualComputersWorkspacePickerSurvivesRefreshBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := loadWorkspaceFixtureApp(t, "picker-stable")
	page.MustElement(`[data-role="agent-workspace"] option[value="machine:vm-run"]`)
	page.MustEval(`() => { const select = document.querySelector('[data-role="agent-workspace"]'); select.value = 'machine:vm-run'; select.focus(); window.pickerSelect = select; window.workspaceFetches = window.requests.filter(r => r === 'GET /api/virtual-computers/workspaces').length; }`)
	// An unchanged refresh keeps the very same select, its value and its focus.
	page.MustEval(`() => document.querySelector('[data-action="refresh"]').click()`)
	page.MustWait(`() => window.requests.filter(r => r === 'GET /api/virtual-computers/workspaces').length > window.workspaceFetches && !document.querySelector('[data-action="refresh"][aria-busy="true"]')`)
	if !page.MustEval(`() => { const select = document.querySelector('[data-role="agent-workspace"]'); return select === window.pickerSelect && select.value === 'machine:vm-run' && document.activeElement === select; }`).Bool() {
		t.Fatal("an unchanged workspace refresh rebuilt the picker or dropped its value or focus")
	}
	// A real change rebuilds the cards but still keeps the choice and the focus.
	page.MustEval(`() => { window.fixtureWorkspaces.push({id:'ws-extra', state:'ready', origin:'launched', template:'python', machine_id:'vm-gone', owner_session_id:'virtual-desktop'}); document.querySelector('[data-action="refresh"]').click(); }`)
	page.MustElement(`.vc-workspace-card[data-workspace-id="ws-extra"]`)
	page.MustWait(`() => !document.querySelector('[data-action="refresh"][aria-busy="true"]')`)
	if !page.MustEval(`() => { const select = document.querySelector('[data-role="agent-workspace"]'); return select.value === 'machine:vm-run' && document.activeElement === select && !!select.querySelector('option[value="ws-extra"]'); }`).Bool() {
		t.Fatal("a changed workspace list lost the picker value or focus")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('picker-stable')`)
}

func TestVirtualComputersWorkspaceVNCCloseReleasesListenersBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(60 * time.Second)
	page.MustWaitLoad()
	page.MustEval(`() => {
 window.listenerNet = {};
 const key = (type, options) => type + (options === true || (options && options.capture) ? ':capture' : '');
 const add = EventTarget.prototype.addEventListener;
 const remove = EventTarget.prototype.removeEventListener;
 EventTarget.prototype.addEventListener = function (type, listener, options) { if (type === 'fullscreenchange' || type === 'pointerdown') window.listenerNet[key(type, options)] = (window.listenerNet[key(type, options)] || 0) + 1; return add.call(this, type, listener, options); };
 EventTarget.prototype.removeEventListener = function (type, listener, options) { if (type === 'fullscreenchange' || type === 'pointerdown') window.listenerNet[key(type, options)] = (window.listenerNet[key(type, options)] || 0) - 1; return remove.call(this, type, listener, options); };
 window.rfbDisconnects = 0;
 window.RFB = function () { this.addEventListener = () => {}; this.disconnect = () => { window.rfbDisconnects++; }; };
 }`)
	page.MustEval(virtualComputersWorkspaceFixture)
	for _, file := range []string{"js/desktop/apps/virtual-computers-workspaces.js", "js/desktop/apps/virtual-computers-vnc.js", "js/desktop/bundles/virtual-computers.bundle.js"} {
		page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, file)))
	}
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'vnc-close',{t:k=>window.dict[k]||k})`)
	page.MustElement(`[data-action="section"][data-section="workspaces"]`).MustClick()
	page.MustElement(`.vc-workspace-card[data-workspace-id="ws-adopted"]`)
	page.MustElement(`[data-action="observe-workspace"][data-id="ws-adopted"]`).MustClick()
	page.MustElement(`[data-role="workspace-vnc-mount"] [data-vnc-action="disconnect"]`)
	if page.MustEval(`() => (window.listenerNet.fullscreenchange || 0) < 1 || (window.listenerNet['pointerdown:capture'] || 0) < 1`).Bool() {
		t.Fatal("the mounted session registered no listeners to remove")
	}
	page.MustElement(`[data-vnc-action="disconnect"]`).MustClick()
	page.MustWait(`() => !document.querySelector('[data-role="workspace-vnc-mount"]')`)
	if page.MustEval(`() => Object.values(window.listenerNet).some(count => count !== 0)`).Bool() {
		t.Fatalf("listeners leaked after closing the workspace VNC: %s", page.MustEval(`() => JSON.stringify(window.listenerNet)`).Str())
	}
	if got := page.MustEval(`() => window.rfbDisconnects`).Int(); got != 1 {
		t.Fatalf("RFB disconnect ran %d times, want exactly once", got)
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('vnc-close')`)
}

func TestVirtualComputersTakeoverWithoutSessionReleasesControlBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	// The VNC library is not loaded, so opening the session fails right after control was taken.
	page := loadWorkspaceFixtureApp(t, "takeover-release")
	page.MustElement(`[data-action="takeover-workspace"][data-id="ws-adopted"]`).MustClick()
	page.MustElement(`[data-role="modal-submit"]`).MustClick()
	page.MustWait(`() => window.bodies.filter(b => b.path.endsWith('/ws-adopted/control')).length >= 2`)
	if !page.MustEval(`() => { const owners = window.bodies.filter(b => b.path.endsWith('/ws-adopted/control')).map(b => b.body.owner); return owners[0] === 'human' && owners[owners.length - 1] === 'agent'; }`).Bool() {
		t.Fatalf("control was not handed back: %s", page.MustEval(`() => JSON.stringify(window.bodies)`).Str())
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('takeover-release')`)
}

func TestVirtualComputersWindowsKeepUniqueIdsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(60 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersWorkspaceFixture)
	page.MustEval(`() => { document.body.innerHTML = '<div id="app"></div><div id="app2"></div>'; }`)
	for _, file := range []string{"js/desktop/apps/virtual-computers-workspaces.js", "js/desktop/bundles/virtual-computers.bundle.js"} {
		page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, file)))
	}
	page.MustEval(`() => { window.VirtualComputersApp.render(document.querySelector('#app'),'win-a',{t:k=>k}); window.VirtualComputersApp.render(document.querySelector('#app2'),'win-b',{t:k=>k}); }`)
	page.MustElement(`#app [data-action="new-machine"]:not([disabled])`).MustClick()
	page.MustElement(`#app [role="dialog"]`)
	page.MustEval(`() => document.querySelector('#app [data-action="close-modal"]').click()`)
	page.MustElement(`#app2 [data-action="new-machine"]:not([disabled])`).MustClick()
	page.MustElement(`#app2 [role="dialog"]`)
	// Open both dialogs at once: the second window's dialog stays, the first is reopened.
	page.MustEval(`() => { document.querySelector('#app [data-action="new-machine"]').click(); }`)
	page.MustElement(`#app [role="dialog"]`)
	if !page.MustEval(`() => {
 const ids = Array.from(document.querySelectorAll('[id]')).map(node => node.id);
 const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
 return new Set(ids).size === ids.length && dialogs.length === 2
   && dialogs.every(dialog => { const label = document.getElementById(dialog.getAttribute('aria-labelledby')); return !!label && dialog.contains(label); })
   && Array.from(document.querySelectorAll('[role="tabpanel"]')).every(panel => !!document.getElementById(panel.getAttribute('aria-labelledby')));
 }`).Bool() {
		t.Fatal("two Virtual Computers windows share DOM ids or have dangling aria-labelledby references")
	}
	page.MustEval(`() => { window.VirtualComputersApp.dispose('win-a'); window.VirtualComputersApp.dispose('win-b'); }`)
}

func TestVirtualComputersModalKeepsFocusOnPendingSubmitBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(60 * time.Second)
	page.MustWaitLoad()
	page.MustEval(virtualComputersOverhaulFixture)
	page.MustEval(`() => {
 const inner = window.fetch;
 window.launchPosts = 0;
 window.fetch = (path, options = {}) => {
   if ((options.method || 'GET') === 'POST' && path === '/api/virtual-computers/machines') {
     window.launchPosts++;
     return new Promise(resolve => { window.failLaunch = () => resolve(new Response(JSON.stringify({error:'Runtime too long', code:'ttl_too_long'}), {status:400, headers:{'Content-Type':'application/json'}})); });
   }
   return inner(path, options);
 };
 }`)
	page.MustEval(`source => { (0,eval)(source); }`, string(mustReadUIFile(t, "js/desktop/bundles/virtual-computers.bundle.js")))
	page.MustEval(`() => window.VirtualComputersApp.render(document.querySelector('#app'),'pending-fixture',{t:k=>k})`)
	page.MustElement(`[data-action="new-machine"]:not([disabled])`).MustClick()
	page.MustElement(`[data-role="launch-name"]`).MustInput("Box")
	page.MustEval(`() => { const submit = document.querySelector('[data-action="confirm-launch"]'); submit.focus(); submit.click(); }`)
	page.MustWait(`() => document.querySelector('[data-action="confirm-launch"]').getAttribute('aria-busy') === 'true'`)
	if !page.MustEval(`() => { const submit = document.querySelector('[data-action="confirm-launch"]'); return document.activeElement === submit && submit.getAttribute('aria-disabled') === 'true' && !submit.disabled; }`).Bool() {
		t.Fatal("the pending submit lost focus or was natively disabled")
	}
	// Clicks are ignored while pending, Escape is held back, and the Tab trap still wraps.
	page.MustEval(`() => { const submit = document.querySelector('[data-action="confirm-launch"]'); submit.click(); submit.click(); submit.dispatchEvent(new KeyboardEvent('keydown', {key:'Escape', bubbles:true})); submit.dispatchEvent(new KeyboardEvent('keydown', {key:'Tab', bubbles:true})); }`)
	if !page.MustEval(`() => window.launchPosts === 1 && !!document.querySelector('[data-role="modal"]') && document.activeElement !== document.body && document.activeElement !== document.querySelector('[data-action="confirm-launch"]')`).Bool() {
		t.Fatal("pending submit accepted another click, closed on Escape or did not wrap the Tab key")
	}
	page.MustEval(`() => { document.querySelector('[data-action="confirm-launch"]').focus(); window.failLaunch(); }`)
	page.MustWait(`() => { const error = document.querySelector('[data-role="modal-error"]'); return !!error && !error.hidden && error.textContent.length > 0; }`)
	if !page.MustEval(`() => { const submit = document.querySelector('[data-action="confirm-launch"]'); return document.activeElement === submit && !submit.hasAttribute('aria-disabled') && !submit.hasAttribute('aria-busy'); }`).Bool() {
		t.Fatal("after the error the submit does not hold the focus or still looks busy")
	}
	page.MustEval(`() => window.VirtualComputersApp.dispose('pending-fixture')`)
}

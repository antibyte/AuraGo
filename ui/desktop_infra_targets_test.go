package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type infraQuickConnectTargetFixture struct{}

func (infraQuickConnectTargetFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/desktop/integrations/devices":
		_ = json.NewEncoder(w).Encode(map[string]any{"devices": []any{
			map[string]any{"id": "ssh-target", "name": "SSH target", "type": "server", "protocol": "ssh", "quick_connect": true, "ip_address": "192.0.2.10", "port": 22},
			map[string]any{"id": "vnc-target", "name": "VNC target", "type": "server", "protocol": "vnc", "quick_connect": true, "ip_address": "192.0.2.11", "port": 5900},
			map[string]any{"id": "none-target", "name": "Registry only", "type": "server", "protocol": "none", "ip_address": "192.0.2.12", "port": 22},
		}})
	case "/api/desktop/integrations/credentials":
		_ = json.NewEncoder(w).Encode([]any{})
	default:
		http.NotFound(w, r)
	}
}

func TestDesktopInfraQuickConnectTargetBrowser(t *testing.T) {
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/integrations/devices":     infraQuickConnectTargetFixture{},
		"/api/desktop/integrations/credentials": infraQuickConnectTargetFixture{},
	})
	defer page.Close()

	page.MustWait(`()=>!!window.QuickConnectSerial?.create`)
	page.MustEval(`()=>{
		window.targetSessionSockets=[];
		window.WebSocket=class extends EventTarget {
			static CONNECTING=0;static OPEN=1;static CLOSING=2;static CLOSED=3;
			constructor(url){super();this.url=String(url);this.readyState=WebSocket.OPEN;this.binaryType='blob';this.bufferedAmount=0;this.protocol='';this.extensions='';this.onopen=null;this.onmessage=null;this.onerror=null;this.onclose=null;this.closeCalls=0;this.sent=[];if(/\/api\/desktop\/(ssh|vnc)(\?|$)/.test(this.url))targetSessionSockets.push(this);}
			send(value){this.sent.push(value);}
			close(){this.closeCalls++;this.readyState=WebSocket.CLOSED;if(this.onclose)this.onclose();}
		};
		window.Terminal=class {constructor(){this.cols=80;this.rows=24;this.handlers={};this.disposed=0;}loadAddon(){}open(){}onData(fn){this.handlers.data=fn;}onResize(fn){this.handlers.resize=fn;}write(){}dispose(){this.disposed++;}};
		window.FitAddon={FitAddon:class {fit(){}}};
		window.serialHarnesses=[];
		const original=window.QuickConnectSerial;
		window.QuickConnectSerial={...original,create(options){const harness={options,disconnectReasons:[],disposeCalls:0};serialHarnesses.push(harness);return {render(){},async disconnect(reason){harness.disconnectReasons.push(reason);return true;},dispose(){harness.disposeCalls++;}};}};
	}`)

	// Untargeted launches continue to create independent Quick Connect windows.
	page.MustEval(`()=>{auditTest.openApp('quick-connect');auditTest.openApp('quick-connect')}`)
	page.MustWait(`()=>document.querySelectorAll('.vd-window[data-app-id="quick-connect"]').length===2`)
	page.MustEval(`()=>auditTest.openApp('quick-connect',{device_id:'ssh-target'})`)
	page.MustWait(`()=>document.querySelectorAll('.vd-window[data-app-id="quick-connect"]').length===2&&document.querySelector('.vd-qc-target:not([hidden]) [role="status"]')?.textContent.includes('SSH target')`)
	page.MustWait(`()=>document.querySelector('[data-device-id="ssh-target"]')?.getAttribute('aria-current')==='true'`)
	if count := page.MustEval(`()=>targetSessionSockets.length`).Int(); count != 0 {
		t.Fatalf("target launch auto-connected %d session sockets", count)
	}

	// Start SSH explicitly, then target a different device. The current terminal
	// and its socket must remain active when the shell reuses this window.
	if !page.MustEval(`name=>{const panel=[...document.querySelectorAll('.vd-qc-target:not([hidden])')].find(item=>item.querySelector('[role="status"]')?.textContent.includes(name));const button=panel?.querySelector('button:not(:disabled)');if(!button)return false;button.click();return true}`, "SSH target").Bool() {
		t.Fatal("could not find the SSH target Connect button")
	}
	page.MustWait(`()=>targetSessionSockets.length===1&&!!document.querySelector('.vd-qc-term-container')`)
	page.MustEval(`()=>auditTest.openApp('quick-connect',{device_id:'vnc-target'})`)
	page.MustWait(`()=>document.querySelector('[data-device-id="vnc-target"]')?.getAttribute('aria-current')==='true'`)
	if result := page.MustEval(`()=>({count:targetSessionSockets.length,closed:targetSessionSockets[0].closeCalls,terminal:!!document.querySelector('.vd-qc-term-container'),windows:document.querySelectorAll('.vd-window[data-app-id="quick-connect"]').length})`); result.Get("count").Int() != 1 || result.Get("closed").Int() != 0 || !result.Get("terminal").Bool() || result.Get("windows").Int() != 2 {
		t.Fatalf("targeting another device disturbed SSH or created a window: %s", result.JSON("", ""))
	}

	// The test connector is the same one the serial UI receives. Opening a
	// target context must not call its switch/disconnect hook.
	page.MustEval(`()=>serialHarnesses[0].options.onSessionStart()`)
	page.MustWait(`()=>!document.querySelector('.vd-qc-serial-content').hidden`)
	baselineDisconnects := page.MustEval(`()=>serialHarnesses[0].disconnectReasons.length`).Int()
	page.MustEval(`()=>auditTest.openApp('quick-connect',{device_id:'ssh-target'})`)
	if result := page.MustEval(`()=>({disconnects:serialHarnesses[0].disconnectReasons.length,disposed:serialHarnesses[0].disposeCalls,visible:!document.querySelector('.vd-qc-serial-content').hidden,windows:document.querySelectorAll('.vd-window[data-app-id="quick-connect"]').length})`); result.Get("disconnects").Int() != baselineDisconnects || result.Get("disposed").Int() != 0 || !result.Get("visible").Bool() || result.Get("windows").Int() != 2 {
		t.Fatalf("targeting a device interrupted the serial session: %s", result.JSON("", ""))
	}

	// Restore VNC explicitly and verify a targeted protocol:none registry row is
	// denied without closing the session or opening a new transport.
	page.MustEval(`()=>auditTest.openApp('quick-connect',{device_id:'vnc-target'})`)
	page.MustWait(`()=>document.querySelector('[data-device-id="vnc-target"]')?.getAttribute('aria-current')==='true'`)
	if !page.MustEval(`name=>{const panel=[...document.querySelectorAll('.vd-qc-target:not([hidden])')].find(item=>item.querySelector('[role="status"]')?.textContent.includes(name));const button=panel?.querySelector('button:not(:disabled)');if(!button)return false;button.click();return true}`, "VNC target").Bool() {
		t.Fatal("could not find the VNC target Connect button")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		state := page.MustEval(`()=>({sessions:document.querySelectorAll('.vd-qc-vnc-session').length,sockets:targetSessionSockets.map(socket=>({url:socket.url,closed:socket.closeCalls})),panels:[...document.querySelectorAll('.vd-qc-target')].map(panel=>({hidden:panel.hidden,label:panel.querySelector('[role="status"]')?.textContent||'',disabled:panel.querySelector('button')?.disabled??null,window:panel.closest('.vd-window')?.dataset.windowId||''})),serial:[...document.querySelectorAll('.vd-qc-serial-content')].map(node=>({hidden:node.hidden,connected:node.isConnected})),errors:fixtureErrors.slice()})`)
		if state.Get("sessions").Int() == 1 && page.MustEval(`()=>targetSessionSockets.some(socket=>socket.url.includes('/api/desktop/vnc?'))`).Bool() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("VNC target did not open (state=%s)", state.JSON("", ""))
		}
		time.Sleep(50 * time.Millisecond)
	}
	page.MustEval(`()=>auditTest.openApp('quick-connect',{device_id:'none-target'})`)
	page.MustWait(`()=>document.querySelector('[data-device-id="none-target"]')?.getAttribute('aria-current')==='true'`)
	if !page.MustEval(`()=>[...document.querySelectorAll('.vd-qc-target:not([hidden])')].some(panel=>panel.querySelector('[role="status"]')?.textContent.includes('Registry only')&&panel.querySelector('button')?.disabled)`).Bool() {
		t.Fatal("protocol:none target exposed an enabled Connect button")
	}
	page.MustEval(`()=>{const panel=[...document.querySelectorAll('.vd-qc-target:not([hidden])')].find(item=>item.querySelector('[role="status"]')?.textContent.includes('Registry only'));panel?.closest('.vd-window')?.querySelector('[data-device-id="none-target"]')?.click()}`)
	page.MustWait(`()=>document.querySelectorAll('.vd-qc-notify').length>0`)
	if result := page.MustEval(`()=>{const vnc=targetSessionSockets.filter(socket=>socket.url.includes('/api/desktop/vnc?'));return {vncSockets:vnc.length,vncClosed:vnc[0]?.closeCalls??-1,sshClosed:targetSessionSockets.find(socket=>socket.url.includes('/api/desktop/ssh?'))?.closeCalls??-1,session:!!document.querySelector('.vd-qc-vnc-session')}}`); result.Get("vncSockets").Int() != 1 || result.Get("vncClosed").Int() != 0 || !result.Get("session").Bool() {
		t.Fatalf("protocol:none denied action disrupted existing VNC: %s", result.JSON("", ""))
	}

	// Closing the targeted window disposes its serial surface and a fresh target
	// launch gets a new window/context rather than stale per-window state.
	page.MustEval(`()=>{
		const ids=[...auditTest.state.windows.values()].filter(win=>win.appId==='quick-connect').map(win=>win.id);
		ids.forEach(id=>auditTest.closeWindow(id));
	}`)
	page.MustWait(`()=>document.querySelectorAll('.vd-window[data-app-id="quick-connect"]').length===0`)
	if closed := page.MustEval(`()=>targetSessionSockets.find(socket=>socket.url.includes('/api/desktop/vnc?'))?.closeCalls??-1`).Int(); closed != 1 {
		t.Fatalf("closing targeted window closed VNC %d times, want once", closed)
	}
	page.MustEval(`()=>auditTest.openApp('quick-connect',{device_id:'vnc-target'})`)
	page.MustWait(`()=>document.querySelectorAll('.vd-window[data-app-id="quick-connect"]').length===1&&document.querySelector('[data-device-id="vnc-target"]')?.getAttribute('aria-current')==='true'`)
	if errs := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}

func TestDesktopInfraCameraTargetBrowser(t *testing.T) {
	fixture := newNetworkCamerasDetectionFixture(t, true)
	page := networkCamerasDetectionPage(t, fixture)
	defer page.Close()

	page.MustEval(`()=>{localStorage.removeItem('aurago.desktop.network-cameras');auditTest.openApp('network-cameras')}`)
	page.MustWait(`()=>!!document.querySelector('.nc-card[data-stream-card="front-door"]')`)
	page.MustEval(`()=>auditTest.openApp('network-cameras',{camera_id:'garage'})`)
	page.MustWait(`()=>document.querySelector('.nc-segmented [data-mode="snapshots"]')?.getAttribute('aria-pressed')==='true'&&!!document.querySelector('.nc-card.is-selected[data-stream-card="garage"]')&&document.querySelector('.nc-detail-header strong')?.textContent==='Garage'`)
	if count := page.MustEval(`()=>document.querySelectorAll('.vd-window[data-app-id="network-cameras"]').length`).Int(); count != 1 {
		t.Fatalf("camera target opened %d windows, want existing window reuse", count)
	}

	// An unknown but syntactically valid ID must not fall back to the first
	// stream or create a viewer for a source that was not enumerated.
	page.MustEval(`()=>auditTest.openApp('network-cameras',{camera_id:'unconfigured-camera'})`)
	page.MustWait(`()=>document.querySelectorAll('.nc-card.is-selected').length===0&&!!document.querySelector('.nc-detail-empty')`)
	if result := page.MustEval(`()=>({windows:document.querySelectorAll('.vd-window[data-app-id="network-cameras"]').length,arbitrary:!!document.querySelector('iframe[src*="unconfigured-camera"]'),selected:document.querySelectorAll('.nc-card.is-selected').length})`); result.Get("windows").Int() != 1 || result.Get("arbitrary").Bool() || result.Get("selected").Int() != 0 {
		t.Fatalf("invalid camera target selected an arbitrary source: %s", result.JSON("", ""))
	}

	// Notification-center event contexts keep the existing event/detail flow
	// after camera-only launch support was added.
	page.MustEval(`id=>{
		const a=auditTest;
		a.state.notificationHistory.unshift({id:'n-infra-target-compat',title:'Camera event',message:'Garage',appId:'network-cameras',context:{event_id:id,camera_id:'garage'},ts:Date.now(),read:false});
		document.getElementById('vd-notification-button').click();
	}`, ncdEventGarage)
	page.MustWait(`()=>!!document.querySelector('[data-notification-id="n-infra-target-compat"]')`)
	page.MustEval(`()=>document.querySelector('[data-notification-id="n-infra-target-compat"]').click()`)
	page.MustWait(`cover=>document.querySelector('.nc-segmented [data-mode="detections"]')?.getAttribute('aria-pressed')==='true'&&document.querySelector('[data-ncd-filter="camera"]')?.value==='garage'&&(document.querySelector('.ncd-frame img')?.getAttribute('src')||'').endsWith('/detection/images/'+cover)`, fmt.Sprint(ncdFrameID(ncdEventGarage, 1)))
	networkCamerasWaitFor(t, "notification event camera filter", 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any {
			for _, query := range f.eventQueries {
				if strings.Contains(query, "stream_id=garage") {
					return true
				}
			}
			return false
		}).(bool)
	})
	if errs := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}

type infraSoftwareStoreTargetFixture struct {
	started  chan struct{}
	release  chan struct{}
	once     sync.Once
	requests atomic.Int32
}

func (f *infraSoftwareStoreTargetFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/desktop/store/catalog" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	f.requests.Add(1)
	f.once.Do(func() { close(f.started) })
	<-f.release
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"catalog": []any{map[string]any{
			"id": "infra-associated-app", "name": "Infra associated app", "image": "fixture", "description": "Fixture app",
			"metadata": map[string]string{},
		}},
		"installed": []any{}, "docker_available": true, "mutations_allowed": true,
	})
}

func TestDesktopInfraSoftwareStoreTargetBrowser(t *testing.T) {
	fixture := &infraSoftwareStoreTargetFixture{started: make(chan struct{}), release: make(chan struct{})}
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/store/catalog": fixture,
	})
	defer page.Close()

	page.MustEval(`()=>auditTest.openApp('software-store')`)
	select {
	case <-fixture.started:
	case <-time.After(8 * time.Second):
		t.Fatal("software store did not begin its initial catalog load")
	}
	// The context arrives while the first catalog request is pending. Selection
	// must be remembered and applied when the asynchronous response is rendered.
	page.MustEval(`()=>auditTest.openApp('software-store',{app_id:'infra-associated-app'})`)
	if count := page.MustEval(`()=>document.querySelectorAll('.vd-window[data-app-id="software-store"]').length`).Int(); count != 1 {
		t.Fatalf("targeted software-store launch opened %d windows, want reuse", count)
	}
	page.MustWait(`()=>!document.querySelector('[data-app-id="infra-associated-app"]')`)
	close(fixture.release)
	page.MustWait(`()=>!!document.querySelector('.vd-store-card[data-app-id="infra-associated-app"]')||!!document.querySelector('.vd-store-empty')||fixtureErrors.length>0`)
	page.MustWait(`()=>{const card=document.querySelector('.vd-store-card[data-app-id="infra-associated-app"]');return card?.getAttribute('aria-current')==='true'&&document.activeElement===card}`)
	if result := page.MustEval(`()=>{const card=document.querySelector('.vd-store-card[data-app-id="infra-associated-app"]');const active=document.activeElement;const rect=card?.getBoundingClientRect();const style=card?getComputedStyle(card):null;const info={tag:active?.tagName||'',className:active?.className||'',id:active?.id||'',html:(active?.outerHTML||'').slice(0,240)};return {selected:card?.getAttribute('aria-current')==='true',focused:active===card,tabIndex:card?.tabIndex??null,connected:!!card?.isConnected,rect:rect?{x:rect.x,y:rect.y,width:rect.width,height:rect.height}:null,display:style?.display||'',visibility:style?.visibility||'',inert:!!card?.closest('[inert]'),active:info,install:!!card?.querySelector('[data-action="install"]'),modal:!!document.querySelector('.vd-modal'),windows:[...document.querySelectorAll('.vd-window')].map(w=>w.dataset.appId),errors:fixtureErrors.slice()}}`); !result.Get("selected").Bool() || !result.Get("focused").Bool() || !result.Get("install").Bool() || result.Get("modal").Bool() || len(result.Get("windows").Arr()) != 1 {
		t.Fatalf("target selection/focus behavior failed (catalog requests=%d): %s", fixture.requests.Load(), result.JSON("", ""))
	}
	if count := fixture.requests.Load(); count != 1 {
		t.Fatalf("targeted open started %d catalog requests, want the original request only", count)
	}
	if errs := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}

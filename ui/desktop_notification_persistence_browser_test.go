package ui

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestDesktopNotificationPolicyAndHistoryPersistenceBrowser(t *testing.T) {
	var scopeMu sync.RWMutex
	sessionScope := "a1b2c3d4" + "a1b2c3d4" + "a1b2c3d4" + "a1b2c3d4" + "a1b2c3d4" + "a1b2c3d4" + "a1b2c3d4" + "a1b2c3d4"
	var settingsMu sync.Mutex
	settingsPuts := 0
	handlers := map[string]http.Handler{
		"/api/auth/status": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			scopeMu.RLock()
			scope := sessionScope
			scopeMu.RUnlock()
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"enabled":true,"authenticated":true,"session_scope":%q}`, scope)
		}),
		"/api/desktop/settings": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				settingsMu.Lock()
				settingsPuts++
				settingsMu.Unlock()
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true}`)
		}),
	}
	page := desktopAuditBrowserWithAPI(t, handlers)
	page.MustWait(`()=>window.AuraDesktopNotificationCenter?.status().ready&&window.AuraDesktopNotificationCenter.status().authMode==='authenticated'`)
	fixtureURL := page.MustEval(`()=>location.origin+'/fixture'`).Str()

	second := page.Browser().MustPage("about:blank").Timeout(30 * time.Second)
	t.Cleanup(func() { _ = second.Close() })
	second.MustSetViewport(1280, 900, 1, false)
	second.MustNavigate(fixtureURL).MustWaitLoad()
	second.MustWait(`()=>window.AuraDesktopNotificationCenter?.status().ready&&window.AuraDesktopNotificationCenter.status().authMode==='authenticated'`)
	second.MustEval(`()=>{
		window.policyStorageEvents=[];
		window.addEventListener('storage',event=>{
			if(event.key==='aurago.desktop.notification-policy.v1')window.policyStorageEvents.push(event.newValue);
		});
	}`)

	// The notification policy is browser-local, so readonly workspaces can still
	// manage it without writing a backend settings value.
	page.MustEval(`()=>{
		auditTest.state.bootstrap.workspace.readonly=true;
		auditTest.toggleNotificationCenter(document.getElementById('vd-notification-button'));
		document.querySelector('#vd-notification-dnd').checked=true;
		document.querySelector('#vd-notification-dnd').dispatchEvent(new Event('change',{bubbles:true}));
		document.querySelector('#vd-notification-center .vd-notification-policy details summary').click();
	}`)
	page.MustWait(`()=>document.querySelector('#vd-notification-center')?.dataset.policyOpen==='true'`)
	page.MustEval(`()=>{
		const select=document.getElementById('vd-notification-source');
		if(![...select.options].some(option=>option.value==='calculator'))throw Error('calculator policy option is missing');
		select.value='calculator';select.dispatchEvent(new Event('change',{bubbles:true}));
		const toasts=document.getElementById('vd-notification-toasts');toasts.checked=false;toasts.dispatchEvent(new Event('change',{bubbles:true}));
	}`)
	second.MustWait(`()=>window.policyStorageEvents.length>=2&&window.AuraDesktopNotificationCenter.policy().dnd&&window.AuraDesktopNotificationCenter.policy().apps.calculator?.toast===false`)
	if !page.MustEval(`()=>{
		const stored=JSON.parse(localStorage.getItem('aurago.desktop.notification-policy.v1'));
		return auditTest.state.bootstrap.workspace.readonly===true&&stored.version===1&&stored.dnd===true&&stored.apps.calculator.toast===false;
	}`).Bool() {
		t.Fatal("readonly shell policy controls must save DND and per-app toast settings in localStorage")
	}
	if !second.MustEval(`()=>{
		const event=JSON.parse(policyStorageEvents.at(-1));
		return event.version===1&&event.dnd===true&&event.apps.calculator.toast===false&&event.apps.calculator.sound===true;
	}`).Bool() {
		t.Fatal("a second same-origin tab must receive the real localStorage policy change")
	}

	page.MustEval(`()=>{
		const center=window.AuraDesktopNotificationCenter;
		window.tabOneSaved=center.record({appId:'desktop',title:'Tab one saved'});
		center.markRead(tabOneSaved.id);
		window.tabOneEphemeral=center.record({appId:'desktop',title:'Tab one ephemeral',persist:false});
		window.tabOneHistory= center.history().map(item=>item.title);
	}`)
	second.MustEval(`()=>{
		window.tabTwoSaved=window.AuraDesktopNotificationCenter.record({appId:'desktop',title:'Tab two saved'});
		window.tabTwoHistory=window.AuraDesktopNotificationCenter.history().map(item=>item.title);
	}`)
	if !page.MustEval(`()=>tabOneHistory.includes('Tab one saved')&&tabOneHistory.includes('Tab one ephemeral')&&!tabOneHistory.includes('Tab two saved')`).Bool() {
		t.Fatal("each tab must retain its own live history, including persist:false entries")
	}
	if !second.MustEval(`()=>tabTwoHistory.includes('Tab two saved')&&!tabTwoHistory.includes('Tab one saved')&&!tabTwoHistory.includes('Tab one ephemeral')`).Bool() {
		t.Fatal("notification history must remain isolated to its originating tab")
	}

	page.MustReload().MustWaitLoad()
	page.MustWait(`()=>window.AuraDesktopNotificationCenter?.status().ready&&window.AuraDesktopNotificationCenter.status().authMode==='authenticated'`)
	if !page.MustEval(`()=>{
		const history=window.AuraDesktopNotificationCenter.history();
		const saved=history.find(item=>item.title==='Tab one saved');
		return saved?.read===true&&!history.some(item=>item.title==='Tab one ephemeral')&&
			JSON.parse(localStorage.getItem('aurago.desktop.notification-policy.v1')).dnd===true;
	}`).Bool() {
		t.Fatal("same-session reload must retain persisted read flags and omit persist:false entries")
	}

	scopeMu.Lock()
	sessionScope = "b2c3d4e5" + "b2c3d4e5" + "b2c3d4e5" + "b2c3d4e5" + "b2c3d4e5" + "b2c3d4e5" + "b2c3d4e5" + "b2c3d4e5"
	scopeMu.Unlock()
	page.MustReload().MustWaitLoad()
	page.MustWait(`()=>window.AuraDesktopNotificationCenter?.status().ready&&window.AuraDesktopNotificationCenter.status().authMode==='authenticated'`)
	if !page.MustEval(`()=>window.AuraDesktopNotificationCenter.history().length===0&&sessionStorage.getItem('aurago.desktop.notifications.v1')===null`).Bool() {
		t.Fatal("a different validated auth session must clear the previous session history before rendering")
	}

	page.MustEval(`()=>{
		sessionStorage.setItem('aurago.desktop.notifications.v1','{broken');
		localStorage.setItem('aurago.desktop.notification-policy.v1','{broken');
	}`)
	page.MustReload().MustWaitLoad()
	page.MustWait(`()=>window.AuraDesktopNotificationCenter?.status().ready&&window.AuraDesktopNotificationCenter.status().authMode==='authenticated'`)
	if !page.MustEval(`()=>{
		const center=window.AuraDesktopNotificationCenter;
		return center.history().length===0&&center.policy().dnd===false&&
			sessionStorage.getItem('aurago.desktop.notifications.v1')===null&&
			localStorage.getItem('aurago.desktop.notification-policy.v1')===null;
	}`).Bool() {
		t.Fatal("damaged session and local storage must recover to empty history and default policy")
	}
	page.MustEval(`()=>window.AuraDesktopNotificationCenter.setPolicy({dnd:true})`)
	if !page.MustEval(`()=>JSON.parse(localStorage.getItem('aurago.desktop.notification-policy.v1')).dnd===true`).Bool() {
		t.Fatal("notification policy writes must remain usable after damaged storage recovery")
	}

	settingsMu.Lock()
	puts := settingsPuts
	settingsMu.Unlock()
	if puts != 0 {
		t.Fatalf("readonly notification policy controls made %d backend settings PUT requests", puts)
	}
}

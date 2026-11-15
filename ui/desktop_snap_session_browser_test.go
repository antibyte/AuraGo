package ui

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopSnapSessionReloadPreservesFreeBoundsBrowser(t *testing.T) {
	var mu sync.RWMutex
	snapshot := ""
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/bootstrap": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.RLock()
			saved := snapshot
			mu.RUnlock()
			settings := desktop.DesktopSettingDefaults()
			for key, value := range map[string]string{"session.windows": saved, "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false"} {
				settings[key] = value
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{}, "widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{}, "workspace": map[string]interface{}{"readonly": false}, "settings": settings})
		}),
	})
	page.MustEval(`()=>{
        auditTest.openApp('files');const item=auditTest.state.windows.get(auditTest.state.activeWindowId);
        window.freeBounds={left:parseInt(item.element.style.left),top:parseInt(item.element.style.top),width:parseInt(item.element.style.width),height:parseInt(item.element.style.height)};
        if(!auditTest.applyWindowSnap(item.element,'left-half'))throw Error('snap failed');
    }`)
	freeBounds := page.MustEval(`()=>JSON.stringify(freeBounds)`).Str()
	captured := page.MustEval(`()=>JSON.stringify(auditTest.captureSessionSnapshot())`).Str()
	mu.Lock()
	snapshot = captured
	mu.Unlock()
	if !page.MustEval(`()=>{
        const saved=auditTest.captureSessionSnapshot().windows.find(item=>item.appId==='files');
        return saved.snapped==='left-half'&&['left','top','width','height'].every(key=>saved[key]===freeBounds[key]);
    }`).Bool() {
		t.Fatal("snapshot must store the snap zone with the original free geometry")
	}
	page.MustReload().MustWaitLoad()
	page.MustWait(`()=>!!auditTest.state.ws&&!auditTest.state.sessionRestoring&&[...auditTest.state.windows.values()].some(item=>item.appId==='files'&&item.snapped==='left-half')`)
	if !page.MustEval(`expected=>{
        const item=[...auditTest.state.windows.values()].find(item=>item.appId==='files');
        auditTest.restoreWindowFromSnap(item.id);const bounds=JSON.parse(expected);
        return !item.snapped&&['left','top','width','height'].every(key=>parseInt(item.element.style[key])===bounds[key]);
    }`, freeBounds).Bool() {
		t.Fatal("Restore after reload must return to the original free window bounds")
	}
	page.MustSetViewport(400, 700, 1, false)
	page.MustReload().MustWaitLoad()
	page.MustWait(`()=>!!auditTest.state.ws&&!auditTest.state.sessionRestoring&&[...auditTest.state.windows.values()].some(item=>item.appId==='files')`)
	if !page.MustEval(`()=>{
        const item=[...auditTest.state.windows.values()].find(item=>item.appId==='files'),r=item.element.getBoundingClientRect();
        return !item.snapped&&r.left>=0&&r.top>=0&&r.right<=innerWidth+1&&r.bottom<=innerHeight+1;
    }`).Bool() {
		t.Fatal("an unavailable snap on a smaller display must fall back to bounded free geometry")
	}
	invalid := page.MustEval(`()=>{
        const saved=auditTest.captureSessionSnapshot(),entry=saved.windows.find(item=>item.appId==='files');
        Object.assign(entry,{snapped:'unknown-layout',left:1e12,top:-1e12,width:-10,height:'invalid'});return JSON.stringify(saved);
    }`).Str()
	mu.Lock()
	snapshot = invalid
	mu.Unlock()
	page.MustReload().MustWaitLoad()
	page.MustWait(`()=>!!auditTest.state.ws&&!auditTest.state.sessionRestoring&&[...auditTest.state.windows.values()].some(item=>item.appId==='files')`)
	if !page.MustEval(`()=>{
        const item=[...auditTest.state.windows.values()].find(item=>item.appId==='files'),r=item.element.getBoundingClientRect();
        return !item.snapped&&r.width>0&&r.height>0&&r.left>=0&&r.top>=0&&r.right<=innerWidth+1&&r.bottom<=innerHeight+1;
    }`).Bool() {
		t.Fatal("invalid persisted layouts and bounds must not escape the available desktop")
	}
}

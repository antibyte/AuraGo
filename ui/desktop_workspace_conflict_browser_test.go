package ui

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/desktop"
)

func TestDesktopWorkspaceConflictKeepsLiveDraftBrowser(t *testing.T) {
	fixture := &desktopWorkspaceBrowserFixture{state: desktop.WorkspaceState{
		Layout:   desktop.DefaultWorkspaceLayout(),
		Settings: map[string]string{"session.windows": "", "appearance.wallpaper": "groupshoot", "appearance.wallpaper_by_space": "{}"},
	}}
	started, release := make(chan struct{}), make(chan struct{})
	var startOnce, releaseOnce sync.Once
	releaseSave := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseSave()
	var rejected atomic.Int32
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/workspaces": http.HandlerFunc(fixture.serveHTTP),
		"/api/desktop/bootstrap": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			settings := desktop.DesktopSettingDefaults()
			for key, value := range fixture.state.Settings {
				settings[key] = value
			}
			for _, key := range []string{"windows.restore_session", "windows.animations", "pet.enabled", "phone_gadget.enabled", "desktop.show_widgets"} {
				settings[key] = "false"
			}
			encoded, _ := json.Marshal(fixture.state.Layout)
			settings[desktop.WorkspaceLayoutSettingKey] = string(encoded)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "settings": settings,
				"installed_apps": []interface{}{}, "widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{}})
		}),
		"/api/desktop/settings": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				fixture.mu.Lock()
				defer fixture.mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"settings": fixture.state.Settings, "workspace": fixture.state})
				return
			}
			var body struct {
				Revision int64             `json:"revision"`
				Settings map[string]string `json:"settings"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode autosave: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			startOnce.Do(func() { close(started) })
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			if body.Revision != fixture.state.Layout.Revision {
				rejected.Add(1)
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": "workspace_revision_conflict", "state": fixture.state})
				return
			}
			t.Error("stale autosave unexpectedly retained the workspace revision")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"settings": fixture.state.Settings, "workspace": fixture.state})
		}),
		"/api/desktop/file": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `"draft-v1"`)
			_ = json.NewEncoder(w).Encode(map[string]string{"content": "saved content"})
		}),
	})
	second := page.Browser().MustPage("about:blank").Timeout(30 * time.Second)
	t.Cleanup(func() { _ = second.Close() })
	second.MustSetViewport(1280, 900, 1, false)
	second.MustNavigate(page.MustInfo().URL).MustWaitLoad()
	second.MustWait(`()=>!!window.auditTest?.state.ws`)
	page.MustEval(`()=>{auditTest.switchSpace('2');auditTest.openApp('editor',{path:'Documents/draft.txt'});}`)
	page.MustWait(`()=>!!document.querySelector('.vd-editor textarea')&&!document.querySelector('.vd-editor textarea').readOnly`)
	page.MustEval(`()=>{
        window.originalWindow=auditTest.state.windows.get(auditTest.state.activeWindowId);
        window.originalTextarea=document.querySelector('.vd-editor textarea');
        originalTextarea.value='unsaved content must survive';originalTextarea.dispatchEvent(new Event('input',{bubbles:true}));
        auditTest.state.bootstrap.settings['windows.restore_session']='true';
        window.autosaveDone=false;auditTest.persistSessionSnapshot().then(()=>autosaveDone=true);
    }`)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("autosave did not reach the held request")
	}
	// Isolate this in-flight save from later focus/geometry autosave timers.
	page.MustEval(`()=>{auditTest.state.bootstrap.settings['windows.restore_session']='false';}`)
	second.MustEval(`async()=>{await AuraDesktopWorkspaceManager.remove('2','3');}`)
	releaseSave()
	page.MustWait(`()=>window.autosaveDone&&AuraDesktopWorkspaceManager.resolveId('2')==='3'`)
	if rejected.Load() != 1 {
		t.Fatalf("conflicted autosave attempts=%d; want one with no replay", rejected.Load())
	}
	if !page.MustEval(`()=>auditTest.state.windows.get(originalWindow.id)===originalWindow&&originalWindow.spaceId==='3'
        &&auditTest.state.activeSpaceId==='3'&&originalTextarea.isConnected&&originalTextarea.value==='unsaved content must survive'`).Bool() {
		t.Fatal("reconciliation replaced a live editor, lost its draft or ignored the chosen destination")
	}
	fixture.mu.Lock()
	saved := fixture.state.Settings["session.windows"]
	fixture.mu.Unlock()
	if saved != "" {
		t.Fatal("rejected autosave overwrote the persisted session")
	}
	for _, tab := range []struct{ name, errors string }{{"original", page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str()}, {"second", second.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str()}} {
		if tab.errors != "[]" {
			t.Fatalf("%s tab errors: %s", tab.name, tab.errors)
		}
	}
}

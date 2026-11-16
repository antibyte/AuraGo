package ui

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopPersonalizationSettingsBrowser(t *testing.T) {
	settings := desktop.DesktopSettingDefaults()
	for _, key := range []string{"windows.restore_session", "windows.animations", "pet.enabled", "phone_gadget.enabled", "desktop.show_widgets"} {
		settings[key] = "false"
	}
	layout := desktop.DefaultWorkspaceLayout()
	var mu sync.Mutex
	var missing atomic.Bool
	imageBytes := customWallpaperBrowserPNG(t)
	protected := func() map[string]string {
		return map[string]string{"session.windows": settings["session.windows"], "appearance.wallpaper": settings["appearance.wallpaper"], "appearance.wallpaper_by_space": settings["appearance.wallpaper_by_space"]}
	}
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/settings": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings, "workspace": desktop.WorkspaceState{Layout: layout, Settings: protected()}})
				return
			}
			var body struct {
				Revision *int64            `json:"revision"`
				Settings map[string]string `json:"settings"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode settings: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if _, sensitive := body.Settings["appearance.wallpaper_by_space"]; sensitive {
				if body.Revision == nil || *body.Revision != layout.Revision {
					t.Errorf("wallpaper selection omitted current revision: %+v, want %d", body.Revision, layout.Revision)
					w.WriteHeader(http.StatusConflict)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": "workspace_revision_conflict", "state": desktop.WorkspaceState{Layout: layout, Settings: protected()}})
					return
				}
				layout.Revision++
			}
			for key, value := range body.Settings {
				settings[key] = value
			}
			encoded, _ := json.Marshal(layout)
			settings[desktop.WorkspaceLayoutSettingKey] = string(encoded)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings, "workspace": desktop.WorkspaceState{Layout: layout, Settings: protected()}})
		}),
		"/api/desktop/files": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"files": []interface{}{map[string]interface{}{"name": "sample.png", "path": "Pictures/sample.png", "type": "file", "size": len(imageBytes)}}})
		}),
		"/api/desktop/wallpaper": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("path") != "Pictures/sample.png" || missing.Load() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": "wallpaper_not_found"})
				return
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		}),
	})
	page.MustEval(`()=>auditTest.openApp('settings',{category:'appearance',settingKey:'appearance.wallpaper_by_space'})`)
	page.MustElement(`[data-settings-action="wallpaper-choose"]`).MustClick()
	page.MustElement(`.vd-file-dialog-row[data-path="Pictures/sample.png"]`).MustClick()
	page.MustElement(`[data-file-dialog-confirm]`).MustClick()
	page.MustWait(`()=>document.body.dataset.wallpaper==='file:Pictures/sample.png'&&document.body.dataset.customWallpaperState==='ready'`)
	if !page.MustEval(`()=>document.querySelector('[data-setting-key="appearance.wallpaper"]').value==='file:Pictures/sample.png'&&document.querySelector('.vd-wallpaper-path').textContent==='Pictures/sample.png'`).Bool() {
		t.Fatal("custom selection must remain visible in both the background selector and its source path")
	}
	page.MustEval(`()=>{const fit=document.querySelector('[data-setting-key="appearance.wallpaper_fit"]');fit.value='contain';fit.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>document.body.style.getPropertyValue('--vd-custom-wallpaper-fit')==='contain'&&document.body.dataset.customWallpaperState==='ready'`)
	captureEverydayPanel(t, page, "custom-wallpaper-settings")

	missing.Store(true)
	page.MustEval(`()=>{auditTest.switchSpace('2');auditTest.switchSpace('1');}`)
	page.MustWait(`()=>document.body.dataset.customWallpaperState==='missing'&&!document.getElementById('vd-wallpaper-status').hidden`)
	if !page.MustEval(`()=>document.getElementById('vd-wallpaper-status').textContent.includes('Pictures/sample.png')&&JSON.parse(auditTest.state.bootstrap.settings['appearance.wallpaper_by_space'])['1']==='file:Pictures/sample.png'`).Bool() {
		t.Fatal("a missing wallpaper needs visible feedback while retaining the stored path")
	}
	missing.Store(false)
	page.MustElement(`[data-settings-action="wallpaper-choose"]`).MustClick()
	page.MustElement(`.vd-file-dialog-row[data-path="Pictures/sample.png"]`).MustClick()
	page.MustElement(`[data-file-dialog-confirm]`).MustClick()
	page.MustWait(`()=>document.body.dataset.customWallpaperState==='ready'&&document.getElementById('vd-wallpaper-status').hidden`)

	page.MustEval(`()=>{auditTest.state.bootstrap.readonly=true;window.dispatchEvent(new CustomEvent('aurago:desktop-settings'));}`)
	if !page.MustEval(`()=>document.querySelector('[data-settings-action="wallpaper-choose"]').disabled&&document.querySelector('[data-setting-key="appearance.wallpaper_fit"]').disabled`).Bool() {
		t.Fatal("server-owned wallpaper preferences must be disabled in a readonly desktop")
	}
	page.MustSetViewport(640, 450, 2, false)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "forced-colors", Value: "active"}, {Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if !page.MustEval(`()=>{const r=document.querySelector('.vd-settings-app').getBoundingClientRect();return r.left>=0&&r.right<=innerWidth+1&&r.width>0&&matchMedia('(forced-colors: active)').matches;}`).Bool() {
		t.Fatal("personalization settings must remain within the constrained viewport")
	}
	captureEverydayPanel(t, page, "custom-wallpaper-forced-colors")
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}

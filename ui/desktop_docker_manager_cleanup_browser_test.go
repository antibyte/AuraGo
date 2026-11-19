package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDesktopDockerManagerCleanupBrowser(t *testing.T) {
	fixture := newDockerManagerFixture()
	var deleted atomic.Int32
	var previewError atomic.Bool
	preview := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var in struct{ Kind string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		if previewError.Load() {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"status":"error","code":"docker_error","message":"Engine unavailable"}`))
			return
		}
		items := []map[string]any{}
		if in.Kind == "images" {
			items = append(items, map[string]any{"id": "img", "revision": "r", "name": "unused:1", "size": 2048})
		} else if in.Kind == "volumes" && deleted.Load() == 0 {
			items = append(items, map[string]any{"id": "archive", "revision": "v", "name": "archive", "size": -1})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "items": items, "limit": 200})
	})
	apply := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var in struct {
			Kind  string
			Items []map[string]string
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || r.Method != http.MethodPost ||
			in.Kind != "volumes" || len(in.Items) != 1 || in.Items[0]["id"] != "archive" || in.Items[0]["revision"] != "v" || in.Items[0]["confirm_name"] != "archive" {
			t.Errorf("unexpected cleanup request %s %+v (%v)", r.Method, in, err)
		}
		deleted.Add(1)
		_, _ = w.Write([]byte(`{"status":"ok","results":[{"id":"archive","state":"removed"}]}`))
	})
	const base = "/api/docker-manager/hosts/"
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/docker-manager/": fixture, base + "local/cleanup/preview": preview,
		base + "h_nas/cleanup/preview": preview, base + "h_nas/cleanup": apply,
	}).CancelTimeout().Timeout(time.Minute)
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)
	page.MustWait(`()=>!!document.querySelector('[data-action="pick-host"][data-host-id="h_nas"]')`)
	click := func(selector string) { page.MustEval(`s=>document.querySelector(s).click()`, selector) }
	selectValue := func(selector, value string) {
		page.MustEval(`(s,v)=>{const e=document.querySelector(s);e.value=v;e.dispatchEvent(new Event('change',{bubbles:true}))}`, selector, value)
	}
	click(`[data-action="cleanup"]`)
	page.MustWait(`()=>document.querySelector('.dm-cleanup-list')?.textContent.includes('unused:1')`)
	if !page.MustEval(`()=>document.querySelector('[data-dact="apply"]').disabled && !document.querySelector('[data-pick]').checked`).Bool() {
		t.Fatal("preview preselected items or enabled deletion")
	}
	click(`[data-cleanup-page]`)
	page.MustWait(`()=>!document.querySelector('[data-dact="apply"]').disabled`)
	selectValue(`[data-cleanup-host]`, "h_nas")
	page.MustWait(`()=>!!document.querySelector('[data-pick]') && !document.querySelector('[data-pick]').checked`)
	selectValue(`[data-cleanup-kind]`, "volumes")
	page.MustWait(`()=>document.querySelector('.dm-cleanup-list')?.textContent.includes('archive')`)
	click(`[data-pick]`)
	if !page.MustEval(`()=>document.querySelector('[data-dact="apply"]').disabled`).Bool() {
		t.Fatal("volume removal was enabled before the exact name was typed")
	}
	page.MustEval(`()=>{const i=document.querySelector('[data-volume-name]');i.value='archive';i.dispatchEvent(new Event('input'))}`)
	click(`[data-dact="apply"]`)
	click(`[data-dact="no"]`)
	if deleted.Load() != 0 {
		t.Fatal("cancel sent a deletion")
	}
	click(`[data-dact="apply"]`)
	click(`[data-dact="yes"]`)
	page.MustWait(`()=>document.querySelector('[data-view] [role="status"]')?.textContent.includes('1 von 1')`)
	if deleted.Load() != 1 {
		t.Fatalf("cleanup ran %d times", deleted.Load())
	}
	click(`[data-dact="refresh"]`)
	page.MustWait(`()=>document.querySelector('[data-view] [role="status"]')?.textContent.includes('nichts aufzuräumen')`)
	previewError.Store(true)
	click(`[data-dact="refresh"]`)
	page.MustWait(`()=>!!document.querySelector('[data-view] [role="alert"]')`)
	previewError.Store(false)
	click(`[data-dact="close"]`)
	fixture.setReadonly(true)
	click(`[data-action="refresh"]`)
	page.MustWait(`()=>document.querySelector('[data-dm-app]').dataset.readonly==='true'`)
	click(`[data-action="cleanup"]`)
	page.MustWait(`()=>document.querySelector('[data-pick]')?.disabled && !document.querySelector('[data-dact="apply"]')`)
	if errors := page.MustEval(`()=>fixtureErrors.join('; ')`).String(); strings.TrimSpace(errors) != "" {
		t.Fatal(errors)
	}
}

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestKaggleConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	base, _ := url.Parse(configRefreshFixtureOrigin(t, "de", false))
	proxy := httputil.NewSingleHostReverseProxy(base)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/kaggle/status" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"enabled": true, "configured": true})
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer origin.Close()
	if file := os.Getenv("AURAGO_KAGGLE_CONFIG_AUDIT_URL_FILE"); file != "" {
		if err := os.WriteFile(file, []byte(origin.URL+"/config#kaggle"), 0600); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(20 * time.Minute); time.Now().Before(deadline); time.Sleep(time.Second) {
			if _, err := os.Stat(file + ".stop"); err == nil {
				return
			}
		}
		t.Fatal("manual config review deadline reached")
	}
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin.URL + "/config#kaggle")
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `()=>!!document.getElementById('kaggle-secret-save')`)
	if !page.MustEval(`()=>{const toggle=document.querySelector('[data-path="kaggle.enabled"]');const rect=toggle.getBoundingClientRect();return rect.width>rect.height&&getComputedStyle(toggle,'::before').content==='""';}`).Bool() {
		t.Error("Kaggle switch lost its visible track and thumb")
	}
	if page.MustEval(`()=>document.getElementById('kaggle-status').textContent.includes('Verbunden')`).Bool() {
		t.Error("stored credentials incorrectly reported a verified connection")
	}
	page.MustEval(`()=>{
 const fetchOriginal=window.fetch;
 window.fetch=async(url,options={})=>String(url)==='/api/kaggle/credentials'?new Promise(resolve=>window.releaseKaggleSave=()=>resolve(new Response('{}',{headers:{'Content-Type':'application/json'}}))):fetchOriginal(url,options);
}`)
	page.MustElement("#kaggle-token").MustInput("fixture-only-credential")
	page.MustElement("#kaggle-secret-save").MustClick()
	waitForJSBool(t, page, `()=>!!window.releaseKaggleSave`)
	page.MustEval(`async()=>{await selectSection('server');await selectSection('kaggle');}`)
	page.MustElement("#kaggle-token").MustInput("newer-fixture-draft")
	page.MustEval(`()=>window.releaseKaggleSave()`)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if page.MustElement("#kaggle-token").MustProperty("value").Str() != "newer-fixture-draft" {
		t.Error("a detached credential save cleared the new section's draft")
	}
	for _, width := range []int{390, 1440} {
		page.MustSetViewport(width, 900, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
			if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
				t.Errorf("config overflow at %d / %s", width, theme)
			}
		}
	}
}

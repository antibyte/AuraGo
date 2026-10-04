package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
)

// Keep the real desktop runtime and DOM, exposing only test entry points.
func desktopAuditBrowser(t *testing.T) *rod.Page {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>window.t=k=>k;window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));window.WebSocket=class extends EventTarget{close(){}};</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/audit-shell.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("startup seam missing")
	}
	shell = shell[:cut] + `window.auditTest={state,api,modalDialog,openApp,closeWindow,focusWindow,captureSessionSnapshot};` + shell[cut:]
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false"} {
		settings[key] = value
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/audit-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/desktop/bootstrap" {
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{}, "widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{}, "workspace": map[string]interface{}{"readonly": false}, "settings": settings})
		} else {
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	page := newSmokeBrowser(t).MustPage().Timeout(30 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>!!window.auditTest?.state.ws`)
	return page
}

func TestDesktopFileConflictBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`()=>{
        window.originalFetch=window.fetch;window.fileWrites=[];
        window.fetch=async (url,options)=>{
            if(url!=='/api/desktop/file'||options?.method!=='PUT')return originalFetch(url,options);
            const h=new Headers(options.headers),b=JSON.parse(options.body);fileWrites.push({path:b.path,match:h.get('If-Match'),create:h.get('If-None-Match')});
            if(h.has('If-Match')||b.path.includes('(1)'))return new Response(JSON.stringify({path:b.path,version:'"new"'}),{headers:{'Content-Type':'application/json'}});
            return new Response(JSON.stringify({code:'file_conflict',conflict:{path:b.path,version:'"observed"'}}),{status:412,headers:{'Content-Type':'application/json'}});
        };
        window.startWrite=()=>{window.writeResult=null;window.writeError='';auditTest.api('/api/desktop/file',{method:'PUT',body:JSON.stringify({path:'Documents/note.txt',content:'new'})}).then(r=>writeResult=r).catch(e=>{writeError=e.name;window.writeDetails=e.stack;});};
        startWrite();
    }`)
	page.MustWait(`()=>!!document.querySelector('[data-choice="replace"]')||!!window.writeError||!!window.writeResult`)
	if got := page.MustEval(`()=>window.writeDetails||JSON.stringify(window.writeResult)`).Str(); got != "null" {
		t.Fatalf("write completed without decision: %s", got)
	}
	page.MustElement(`[data-choice="replace"]`).MustClick()
	page.MustWait(`()=>!!window.writeResult`)
	if got := page.MustEval(`()=>fileWrites[1].match`).Str(); got != `"observed"` {
		t.Fatalf("replace used %q", got)
	}
	page.MustEval(`()=>startWrite()`)
	page.MustElement(`[data-choice="copy"]`).MustClick()
	page.MustWait(`()=>!!window.writeResult`)
	if got := page.MustEval(`()=>writeResult.path`).Str(); got != "Documents/note (1).txt" {
		t.Fatal(got)
	}
	page.MustEval(`()=>{window.beforeCancel=fileWrites.length;startWrite();}`)
	page.MustElement(`[data-cancel]`).MustClick()
	page.MustWait(`()=>window.writeError==='AbortError'`)
	if !page.MustEval(`()=>fileWrites.length===beforeCancel+1`).Bool() {
		t.Fatal("cancel retried mutation")
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}

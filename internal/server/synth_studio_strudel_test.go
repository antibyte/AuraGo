package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/ui"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestSynthStudioStrudelPlayerPolicy(t *testing.T) {
	s, token, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	handler := securityHeadersMiddleware(http.HandlerFunc(s.handleSynthStudioStrudel), false, false)
	for _, tc := range []struct {
		name, method, token string
		status              int
	}{
		{"anonymous", "GET", "", 401}, {"invalid token", "GET", "invalid", 403},
		{"read", "GET", token, 200}, {"head", "HEAD", token, 200}, {"mutation", "POST", token, 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "https://studio.test"+synthStudioStrudelPath, nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d", w.Code, tc.status)
			}
			if tc.status != 200 {
				return
			}
			csp := w.Header().Get("Content-Security-Policy")
			for _, part := range []string{"sandbox allow-scripts;", "connect-src 'none'", "default-src 'none'", "frame-ancestors 'self'", "https://studio.test/js/vendor/strudel/strudel.js"} {
				if !strings.Contains(csp, part) {
					t.Fatalf("missing %q in %s", part, csp)
				}
			}
			if strings.Contains(csp, "allow-same-origin") || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
				t.Fatal("frame isolation lost")
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
		})
	}
	s.Cfg.VirtualDesktop.Enabled = false
	r := httptest.NewRequest("GET", "https://studio.test"+synthStudioStrudelPath, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled Desktop: %d", w.Code)
	}
}

func TestSynthStudioStrudelBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		var ok bool
		bin, ok = launcher.LookPath()
		if !ok {
			t.Fatal("Chrome or Edge required")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Leakless(runtime.GOOS != "windows")
	control, err := l.Launch()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Kill()
	browser := rod.New().Context(ctx).ControlURL(control).MustConnect()
	defer browser.Close()
	s := newDesktopFilesystemTestServer(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc(synthStudioStrudelPath, s.handleSynthStudioStrudel)
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><link rel="stylesheet" href="/css/desktop-app-synth-studio.css"><div class="ss-app" style="height:600px"><section id="studio"></section></div><script src="/js/desktop/apps/synth-studio-strudel.js"></script><script>
window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));
window.life=new AbortController();window.readonly=false;window.before=0;
window.studio=SynthStudioStrudel.create(document.querySelector('#studio'),{tr:k=>k,signal:life.signal,blocked:()=>readonly,readonly:()=>readonly,beforePlay:()=>before++,confirm:async()=>true});
</script>`)
	})
	srv := httptest.NewServer(securityHeadersMiddleware(mux, false, false))
	defer srv.Close()
	page := browser.MustPage().Timeout(80 * time.Second)
	defer page.Close()
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	setCode := func(code string) {
		page.MustEval(`code=>{const el=document.querySelector('textarea');el.value=code;el.dispatchEvent(new Event('input'));}`, code)
	}
	setCode(`note("c4*8").s("sine").gain(.2).analyze(1)`)
	page.MustElement("[data-strudel-run]").MustClick()
	page.MustWait(`()=>['playing','sound','error'].includes(document.querySelector('output').dataset.strudelState)`)
	if page.MustEval(`()=>document.querySelector('output').dataset.strudelState==='error'`).Bool() {
		t.Fatal(page.MustElement("output").MustText())
	}
	frame := page.MustElement("iframe").MustFrame()
	if page.MustEval(`()=>document.querySelector('output').dataset.strudelState==='sound'`).Bool() {
		frame.MustElement("#sound").MustClick()
	}
	page.MustWait(`()=>document.querySelector('output').dataset.strudelState==='playing'`)
	frame.MustWait(`()=>{const a=strudel.analysers[1];if(!a)return false;const b=new Float32Array(a.fftSize);a.getFloatTimeDomainData(b);return b.some(v=>Math.abs(v)>.001);}`)
	if !frame.MustEval(`()=>['bd','sd','cp','hh'].every(name=>strudel.getSound(name)?.data.type==='synth')`).Bool() {
		t.Fatal("tutorial drum synthesis unavailable")
	}
	if !page.MustEval(`()=>{try{document.querySelector('iframe').contentWindow.document;return false}catch(e){return e.name==='SecurityError'}}`).Bool() {
		t.Fatal("frame shares desktop origin")
	}
	if !frame.MustEval(`async()=>{try{await fetch('/api/desktop/bootstrap');return false}catch{return true}}`).Bool() {
		t.Fatal("frame can fetch APIs")
	}
	setCode(`note("g4*8").s("triangle").gain(.2).analyze(1)`)
	page.MustElement("[data-strudel-run]").MustClick()
	page.MustWait(`()=>document.querySelector('output').dataset.strudelState==='playing'`)
	if page.MustEval(`()=>before`).Int() != 2 {
		t.Fatal("live update lost transport handoff")
	}
	for _, code := range page.MustEval(`()=>Object.values(SynthStudioStrudel.examples)`).Arr() {
		setCode(code.Str())
		page.MustElement("[data-strudel-run]").MustClick()
		page.MustWait(`()=>['playing','error'].includes(document.querySelector('output').dataset.strudelState)`)
		if page.MustEval(`()=>document.querySelector('output').dataset.strudelState==='error'`).Bool() {
			t.Fatal(page.MustElement("output").MustText())
		}
	}
	setCode(`note(`)
	page.MustElement("[data-strudel-run]").MustClick()
	page.MustWait(`()=>document.querySelector('output').dataset.strudelState==='error'`)
	if page.MustEval(`()=>!!document.querySelector('iframe')`).Bool() {
		t.Fatal("failed evaluation retained audio frame")
	}
	page.MustEval(`()=>dispatchEvent(new MessageEvent('message',{data:{strudel:1,type:'playing',request:1},origin:'null'}))`)
	if page.MustEval(`()=>document.querySelector('output').dataset.strudelState`).Str() != "error" {
		t.Fatal("forged message accepted")
	}
	setCode(`s("missing-aurago-sound")`)
	page.MustElement("[data-strudel-run]").MustClick()
	page.MustWait(`()=>document.querySelector('output').dataset.strudelState==='error'&&!document.querySelector('iframe')`)
	if !strings.Contains(page.MustElement("output").MustText(), "not found") {
		t.Fatal("missing sound failed silently")
	}
	codeFile := filepath.Join(t.TempDir(), "pattern.strudel")
	if err := os.WriteFile(codeFile, []byte(`note("e4").s("sine")`), 0600); err != nil {
		t.Fatal(err)
	}
	page.MustElement(`input[type=file]`).MustSetFiles(codeFile)
	page.MustWait(`()=>document.querySelector('textarea').value==='note("e4").s("sine")'`)
	page.MustReload().MustWaitLoad()
	if page.MustElement("textarea").MustProperty("value").Str() != `note("e4").s("sine")` {
		t.Fatal("browser draft was not recovered")
	}
	setCode(`note("c4").s("sine")`)
	page.MustElement("[data-strudel-run]").MustClick()
	page.MustElement("[data-strudel-stop]").MustClick()
	page.MustWait(`()=>document.querySelector('output').dataset.strudelState==='idle'&&!document.querySelector('iframe')`)
	page.MustEval(`()=>{readonly=true;studio.refresh()}`)
	if !page.MustElement("[data-strudel-run]").MustDisabled() {
		t.Fatal("readonly allows evaluation")
	}
	page.MustEval(`()=>life.abort()`)
	if page.MustEval(`()=>errors.length`).Int() != 0 {
		t.Fatal(page.MustEval(`()=>JSON.stringify(errors)`).Str())
	}
}

package server

import (
	"aurago/internal/i18n"
	"aurago/ui"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRTLSDRDesktopBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	s := rtlSDRServer(t)
	read := func(path string) string {
		b, err := ui.Content.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	i18n.Load(ui.Content, slog.Default())
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	scripts := `<script type="application/json" id="aurago-template-data">__DATA__</script><script src="/js/shared/template-data.js"></script><script>window._auragoSharedInitialized=true;</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/sdr-shell.js"></script><script src="/sdr-fixture.js"></script>`
	html = strings.Replace(html, "</body>", scripts+"</body>", 1)
	shell := read("js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell seam missing")
	}
	shell = shell[:cut] + `window.sdrTest={state,openApp,loadIconManifest,closeWindow,handleDesktopEvent};})();`
	fixture := `window.sdrErrors=[];window.addEventListener('error',e=>sdrErrors.push(e.message));window.addEventListener('unhandledrejection',e=>sdrErrors.push(String(e.reason)));const originalFetch=fetch.bind(window);window.fetch=(url,options)=>String(url).startsWith('/api/')&&!String(url).startsWith('/api/desktop/rtl-sdr/')&&!String(url).startsWith('/api/i18n?')?Promise.resolve(new Response('{}')):originalFetch(url,options);HTMLMediaElement.prototype.play=function(){return Promise.resolve()};window.fixtureReady=(async()=>{sdrTest.state.bootstrap={enabled:true,builtin_apps:[{id:'rtl-sdr',name:'RTL-SDR',icon:'radio'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};document.body.dataset.theme='standard';document.body.dataset.animations='false';document.getElementById('vd-disabled').hidden=true;await sdrTest.loadIconManifest();sdrTest.openApp('rtl-sdr');})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc("/api/desktop/rtl-sdr/", s.handleRTLSDR)
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, getI18NJSONForSections(normalizeLang(r.URL.Query().Get("lang")), strings.Split(r.URL.Query().Get("sections"), ",")...))
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := uiTemplateData("de", "desktop")
		fmt.Fprint(w, strings.Replace(html, "__DATA__", fmt.Sprint(data["TemplateDataJSON"]), 1))
	})
	mux.HandleFunc("/sdr-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/sdr-fixture.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, fixture)
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage().Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1280, 980, 1, false)
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>await fixtureReady`)
	wait := func(js string) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if page.MustEval(js).Bool() {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("condition %s; errors %s", js, page.MustEval(`()=>JSON.stringify(sdrErrors)`).Str())
	}
	wait(`()=>!!document.querySelector('[data-action="play"]') && document.querySelector('[data-sdr="station"]').textContent==='Radio Aurora'`)
	if page.MustEval(`()=>document.querySelector('.sdr-app').textContent.includes('rtlSdr.')`).Bool() {
		t.Fatal("untranslated labels")
	}
	page.MustElement(`[data-action="play"]`).MustClick()
	wait(`()=>RTLSDRRuntime.playing`)
	// Observe actual media starts as well as the real HTTP service state. Display
	// changes alone do not prove that the shared receiver was retuned.
	page.MustEval(`()=>{window.sdrStreams=[];const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){sdrStreams.push(this.getAttribute('src'));return play.call(this)};}`)
	page.MustEval(`()=>{const input=document.querySelector('[data-sdr="frequency"]');input.value='105.2';input.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>RTLSDRRuntime.currentTuning()?.frequency_hz===105200000&&sdrStreams.length===1`)
	if got := s.RTLSDR.Snapshot().Tuning.Frequency; got != 105200000 {
		t.Fatalf("frequency control did not retune receiver: %d", got)
	}
	// Hold a successful response long enough to edit again during a mode switch.
	page.MustEval(`()=>{const send=fetch;window.fetch=async(url,options)=>{const response=await send(url,options);if(String(url).endsWith('/rtl-sdr/tune')&&JSON.parse(options.body).tuning.mode==='am'){window.sdrTuneWaiting=true;await new Promise(resolve=>setTimeout(resolve,1200));}return response;};const mode=document.querySelector('[data-sdr="mode"]');mode.value='am';mode.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>window.sdrTuneWaiting===true`)
	page.MustEval(`()=>{const mode=document.querySelector('[data-sdr="mode"]');mode.value='nfm';mode.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>RTLSDRRuntime.currentTuning()?.mode==='nfm'&&sdrStreams.length===3`)
	if tuning := s.RTLSDR.Snapshot().Tuning; tuning.Mode != "nfm" || tuning.Bandwidth != 12500 || tuning.Frequency != 105200000 {
		t.Fatalf("latest mode change was dropped during retune: %+v", tuning)
	}
	if !page.MustEval(`()=>new Set(sdrStreams).size===3&&sdrStreams.every(url=>url.includes('/stream?client=')&&url.includes('&v='))`).Bool() {
		t.Fatal("retuning retained the previous buffered audio stream")
	}
	page.MustEval(`()=>{const mode=document.querySelector('[data-sdr="mode"]');mode.value='wfm';mode.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>RTLSDRRuntime.currentTuning()?.mode==='wfm'&&sdrStreams.length===4`)
	page.MustEval(`()=>{const agc=document.querySelector('[data-sdr="agc"]');agc.checked=false;agc.dispatchEvent(new Event('change',{bubbles:true}));const gain=document.querySelector('[data-sdr="gain"]');gain.value='40';gain.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>RTLSDRRuntime.currentTuning()?.agc===false&&RTLSDRRuntime.currentTuning()?.gain_db===40`)
	windowID := page.MustEval(`()=>[...sdrTest.state.windows.keys()][0]`).Str()
	page.MustEval(`async(id)=>await sdrTest.closeWindow(id)`, windowID)
	wait(`()=>!document.querySelector('.sdr-app')`)
	if !page.MustEval(`()=>RTLSDRRuntime.playing&&!document.querySelector('.sdr-mini').hidden`).Bool() {
		t.Fatal("closing window stopped listening")
	}
	page.MustEval(`async()=>await sdrTest.handleDesktopEvent({type:'rtl_sdr_recording_soon'})`)
	if !page.MustEval(`()=>document.body.textContent.includes('Eine geplante Aufnahme startet gleich')`).Bool() {
		t.Fatal("recording announcement missing while app is closed")
	}
	page.MustElement(`[data-sdr-mini="open"]`).MustClick()
	wait(`()=>!!document.querySelector('.sdr-app')`)
	page.MustEval(`()=>{const input=document.querySelector('[data-sdr="frequency"]');input.value='107.4';input.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustElement(`[data-action="stop"]`).MustClick()
	wait(`()=>!RTLSDRRuntime.playing`)
	page.MustEval(`async()=>await new Promise(resolve=>setTimeout(resolve,700))`)
	if page.MustEval(`()=>RTLSDRRuntime.playing`).Bool() || s.RTLSDR.Snapshot().Tuning.Frequency != 105200000 {
		t.Fatal("queued tuning restarted playback after Stop")
	}
	page.MustElement(`[data-tab="recordings"]`).MustClick()
	page.MustElement(`[data-sdr="name"]`).MustInput("Noon news")
	page.MustElement(`[data-sdr="transcribe"]`).MustClick()
	page.MustElement(`.sdr-job-form button[type="submit"]`).MustClick()
	wait(`()=>document.querySelector('[data-sdr="jobs"]').textContent.includes('Noon news')`)
	page.MustElement(`[data-tab="schedules"]`).MustClick()
	page.MustElement(`[data-sdr="name"]`).MustInput("Daily news")
	page.MustElement(`.sdr-job-form button[type="submit"]`).MustClick()
	wait(`()=>document.querySelector('[data-sdr="jobs"]').textContent.includes('Daily news')`)
	page.MustElement(`[data-tab="receive"]`).MustClick()
	wait(`()=>!!document.querySelector('[data-sdr="low"]')?.textContent`)
	page.MustEval(`()=>{const b=document.querySelector('[data-power="1000"]');b.focus();b.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowUp',bubbles:true}));}`)
	if !page.MustEval(`()=>document.activeElement?.dataset.power==='1000'`).Bool() {
		t.Fatal("digit tuning lost keyboard focus")
	}
	page.MustEval(`()=>{document.body.dataset.theme='fruity';const app=document.querySelector('.sdr-app');app.style.width='480px';app.style.height='650px';}`)
	if !page.MustEval(`()=>{const app=document.querySelector('.sdr-app'), footer=app.querySelector('footer');return footer.getBoundingClientRect().bottom<=app.getBoundingClientRect().bottom+1&&app.scrollWidth<=481}`).Bool() {
		t.Fatal("compact app overflows its footer or width")
	}
	page.MustEval(`()=>{document.body.dataset.theme='standard';const app=document.querySelector('.sdr-app');app.style.width='';app.style.height='';}`)
	if errors := page.MustEval(`()=>JSON.stringify(sdrErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
	if path := os.Getenv("AURAGO_RTLSDR_SCREENSHOT"); path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0700)
		page.MustScreenshot(path)
	}
}

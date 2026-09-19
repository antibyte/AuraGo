package server

import (
	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/personalradio"
	"aurago/ui"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const personalRadioBrowserFixture = `
window.radioErrors=[];window.addEventListener('error',e=>radioErrors.push(e.message));window.addEventListener('unhandledrejection',e=>radioErrors.push(String(e.reason)));
const originalFetch=window.fetch.bind(window);
window.radioI18nRequests=[];
window.fetch=(url,options)=>{const path=String(url);if(path.startsWith('/api/i18n?'))radioI18nRequests.push(path);return path.startsWith('/api/')&&!path.startsWith('/api/desktop/personal-radio/')&&!path.startsWith('/api/i18n?')?Promise.resolve(new Response('{}')):originalFetch(url,options);};
window.fixtureReady=(async()=>{
 if(Object.keys(window.I18N).some(key=>key.startsWith('personalRadio.')))throw Error('Radio translations must load through the app loader');
 radioTest.state.bootstrap={enabled:true,builtin_apps:[{id:'personal-radio',name:'Personal Radio',icon:'radio'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};
 document.body.dataset.theme='standard';document.body.dataset.animations='false';document.getElementById('vd-disabled').hidden=true;
 await radioTest.loadIconManifest();radioTest.openApp('personal-radio');
})();
`

func personalRadioBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	bin := ""
	for _, path := range []string{`C:\Program Files\Google\Chrome\Application\chrome.exe`, `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, "chromium", "google-chrome"} {
		if resolved, err := exec.LookPath(path); err == nil {
			bin = resolved
			break
		}
	}
	if bin == "" {
		t.Skip("Chrome/Edge unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	url, err := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("disable-gpu").Launch()
	if err != nil {
		t.Fatal(err)
	}
	browser := rod.New().ControlURL(url)
	if err = browser.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { browser.Close() })
	return browser
}

func TestPersonalRadioBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	read := func(path string) string {
		b, err := ui.Content.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.Directories.DataDir = t.TempDir()
	openingPlan := make(chan struct{})
	musicSteps := make(chan int, 2)
	svc, err := personalradio.New(personalradio.Options{Directory: filepath.Join(cfg.Directories.DataDir, "personal-radio"), Adapters: personalradio.Adapters{
		Plan: func(ctx context.Context, req personalradio.EditorialRequest) (personalradio.Plan, error) {
			if req.Opening == nil {
				return personalradio.Plan{}, nil
			}
			select {
			case <-openingPlan:
			case <-ctx.Done():
				return personalradio.Plan{}, ctx.Err()
			}
			return personalradio.Plan{Moderation: "Willkommen! Deine Musik wird vorbereitet. Sobald genügend Titel bereit sind, geht es los."}, nil
		},
		Speak: func(context.Context, personalradio.Station, string) (personalradio.Audio, error) {
			return personalradio.Audio{Data: personalRadioTestWave(8000, 3, 9), Extension: "wav"}, nil
		},
		Generate: func(ctx context.Context, _ personalradio.Station, _, _ string) (personalradio.Production, error) {
			var seed int
			select {
			case seed = <-musicSteps:
			case <-ctx.Done():
				return personalradio.Production{}, ctx.Err()
			}
			path := filepath.Join(cfg.Directories.DataDir, fmt.Sprintf("generated-%d.wav", seed))
			if err := os.WriteFile(path, personalRadioTestWave(8000, 180, seed), 0600); err != nil {
				return personalradio.Production{}, err
			}
			return personalradio.Production{Path: path, Title: fmt.Sprintf("New music %d", seed)}, nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	p := personalradio.DefaultStation()
	p.Name = "Late Night Radio"
	p.Topics = "Jazz und leise Gedanken"
	p.Mode = "local"
	p.Moderation = "off"
	p.ReserveMinutes = 5
	p.MinTracks = 2
	p, err = svc.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		_, err = svc.Import(context.Background(), p.ID, bytes.NewReader(personalRadioTestWave(8000, 180, i)), "wav", fmt.Sprintf("Night session %d", i), "local", "Lo-fi", 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{Cfg: cfg, PersonalRadio: svc}
	i18n.Load(ui.Content, slog.Default())
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	// Keep production section filtering, translation lookup and lazy app loading.
	// Reading the full locale JSON here would hide a missing loader registration.
	scripts := `<script type="application/json" id="aurago-template-data">__RADIO_TEMPLATE_DATA__</script>
<script src="/js/shared/template-data.js"></script>
<script>window._auragoSharedInitialized=true;</script>
<script src="/js/shared/shared-core.js"></script>
<script src="/js/shared/lazy-assets.js"></script>
<script src="/js/desktop/core/module-loader.js"></script>
<script src="/radio-shell.js"></script>
<script src="/radio-fixture.js"></script>`
	html = strings.Replace(html, "</body>", scripts+"</body>", 1)
	shell := read("js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell startup seam missing")
	}
	shell = shell[:cut] + `window.radioTest={state,openApp,loadIconManifest,closeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc("/api/desktop/personal-radio/", server.handlePersonalRadio)
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, getI18NJSONForSections(normalizeLang(r.URL.Query().Get("lang")), strings.Split(r.URL.Query().Get("sections"), ",")...))
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			lang = "de"
		}
		data := uiTemplateData(normalizeLang(lang), "desktop")
		fmt.Fprint(w, strings.Replace(html, "__RADIO_TEMPLATE_DATA__", fmt.Sprint(data["TemplateDataJSON"]), 1))
	})
	mux.HandleFunc("/radio-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/radio-fixture.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, personalRadioBrowserFixture)
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage().Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1280, 920, 1, false)
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>await fixtureReady`)
	wait := func(js string) {
		t.Helper()
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			if page.MustEval(js).Bool() {
				return
			}
			time.Sleep(60 * time.Millisecond)
		}
		t.Fatalf("condition failed: %s; errors: %s", js, page.MustEval(`()=>JSON.stringify(radioErrors)`).Str())
	}
	wait(`()=>window.PersonalRadioRuntime?.state?.stations?.length===1 && !!document.querySelector('.pr-app')`)
	assertLocalized := func() {
		t.Helper()
		if page.MustEval(`()=>document.querySelector('[data-app-id="personal-radio"]').outerHTML.includes('personalRadio.')`).Bool() {
			t.Fatal("Personal Radio exposes untranslated keys after production app loading")
		}
	}
	assertLocalized()
	if got := page.MustElement(".pr-app [data-action=new]").MustText(); got != "＋ Neuer Sender" {
		t.Fatal("German app label:", got)
	}
	page.MustElement(".pr-app [data-action=start]").MustClick()
	wait(`()=>PersonalRadioRuntime.position().position>500`)
	if got := svc.Snapshot(); got.Status != "playing" {
		t.Fatal("real playback not acknowledged", got.Status)
	}
	page.MustElement(".pr-app [data-action=pause]").MustClick()
	wait(`()=>PersonalRadioRuntime.state.state.status==='paused'`)
	before := page.MustEval(`()=>PersonalRadioRuntime.position().position`).Int()
	time.Sleep(180 * time.Millisecond)
	after := page.MustEval(`()=>PersonalRadioRuntime.position().position`).Int()
	if after-before > 50 {
		t.Fatal("pause advanced audio clock", before, after)
	}
	page.MustElement(".pr-app [data-action=start]").MustClick()
	wait(`()=>PersonalRadioRuntime.state.state.status!=='paused'`)
	page.MustEval(`async()=>{window.beforeClosePosition=PersonalRadioRuntime.position().position;await radioTest.closeWindow([...radioTest.state.windows.keys()][0]);}`)
	wait(`()=>!document.querySelector('.pr-app')&&!document.querySelector('.pr-mini').hidden&&PersonalRadioRuntime.position().position>beforeClosePosition+150`)
	if got := page.MustEval(`()=>document.querySelector('[data-pr-mini=stop]').getAttribute('aria-label')`).Str(); got != "Stoppen" {
		t.Fatal("German mini-player label:", got)
	}
	page.MustElement("[data-pr-mini=open]").MustClick()
	wait(`()=>!!document.querySelector('.pr-app')`)
	assertLocalized()
	if got := page.MustEval(`()=>radioI18nRequests.filter(url=>new URL(url,location.href).searchParams.get('sections')==='personalRadio').length`).Int(); got != 1 {
		t.Fatal("radio section should load once across close/reopen:", got)
	}
	page.MustElement(".pr-app [data-tab=library]").MustClick()
	wait(`()=>document.querySelectorAll('.pr-track').length===2`)
	captureLayouts := func(prefix string) {
		for _, theme := range []string{"standard", "fruity"} {
			for _, width := range []int{1060, 420} {
				page.MustEval(`(theme,width)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode='light';const w=document.querySelector('[data-app-id="personal-radio"]');w.style.width=width+'px';w.style.height='800px';w.style.left='10px';w.style.top='10px';}`, theme, width)
				if !page.MustEval(`()=>{const x=document.querySelector('.pr-app');return x.scrollWidth<=x.clientWidth+2}`).Bool() {
					t.Fatal("horizontal overflow", theme, width)
				}
				if prefix != "" {
					if !page.MustEval(`()=>{const box=document.querySelector('[data-pr=preparation]'),app=document.querySelector('.pr-app');return !box.hidden&&box.getBoundingClientRect().bottom<app.getBoundingClientRect().bottom;}`).Bool() {
						t.Fatal("startup panel clipped", theme, width)
					}
				}
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					os.MkdirAll(dir, 0755)
					png, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
					if err != nil {
						t.Fatal(err)
					}
					os.WriteFile(filepath.Join(dir, fmt.Sprintf("personal-radio-%s%s-%d.png", prefix, theme, width)), png, 0644)
				}
			}
		}
	}
	captureLayouts("")
	page.MustElement(".pr-app [data-action=stop]").MustClick()
	wait(`()=>PersonalRadioRuntime.state.state.status==='stopped'`)
	page.MustElement(".pr-app [data-action=settings]").MustClick()
	wait(`()=>!!document.querySelector('.pr-settings')`)
	assertLocalized()
	page.MustElement(".pr-settings [name=name]").MustSelectAllText().MustInput("Edited station")
	page.MustElement(".pr-settings [type=submit]").MustClick()
	wait(`()=>PersonalRadioRuntime.state.stations[0].name==='Edited station'`)
	page.MustElement(".pr-app [data-action=start]").MustClick()
	wait(`()=>PersonalRadioRuntime.position().position>500`)
	page.MustEval(`()=>{window.stopTestFetch=window.fetch;window.fetch=(url,options)=>String(url).endsWith('/stop')?Promise.reject(Error('radio_request_failed')):stopTestFetch(url,options);}`)
	page.MustElement(".pr-app [data-action=stop]").MustClick()
	wait(`()=>!PersonalRadioRuntime.active`)
	time.Sleep(2200 * time.Millisecond)
	if page.MustEval(`()=>PersonalRadioRuntime.position().position`).Int() != 0 {
		t.Fatal("failed stop request revived local playback")
	}
	page.MustEval(`async()=>{window.fetch=stopTestFetch;await PersonalRadioRuntime.control('stop');}`)
	wait(`()=>PersonalRadioRuntime.state.state.status==='stopped'`)
	other, err := svc.Start(p.ID, "another-radio-device", false)
	if err != nil {
		t.Fatal(err)
	}
	svc.Tick()
	page.MustEval(`async()=>await PersonalRadioRuntime.refresh()`)
	wait(`()=>PersonalRadioRuntime.state.state.owner==='another-radio-device'`)
	if page.MustElement(".pr-app [data-action=start]").MustProperty("disabled").Bool() {
		t.Fatal("another device's station cannot be taken over")
	}
	if err = svc.Control("another-radio-device", other.Epoch, "stop", ""); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>await PersonalRadioRuntime.refresh()`)
	// A cold station exercises the real startup UI and audio lifecycle while
	// deterministic provider gates make each preparation phase observable.
	cold := personalradio.DefaultStation()
	cold.Name = "Startup Radio"
	cold.Topics = "Jazz und leise Gedanken"
	cold.Mode = "generated"
	cold.ReserveMinutes, cold.MinTracks, cold.LibraryMinutes = 5, 2, 6
	cold, err = svc.SaveStation(cold)
	if err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async id=>{await PersonalRadioRuntime.refresh();const select=document.querySelector('[data-pr=station]');select.value=id;select.dispatchEvent(new Event('change'));document.querySelector('[data-tab=program]').click();}`, cold.ID)
	page.MustEval(`()=>{window.startFetch=window.fetch;window.fetch=(url,options)=>String(url).endsWith('/start')?Promise.reject(Error('radio_request_failed')):startFetch(url,options);}`)
	page.MustElement(".pr-app [data-action=start]").MustClick()
	wait(`()=>!PersonalRadioRuntime.startingStation&&!document.querySelector('[data-pr=notice]').hidden&&!document.querySelector('[data-action=start]').disabled`)
	page.MustEval(`()=>{window.fetch=(url,options)=>String(url).endsWith('/start')?new Promise(resolve=>{window.releaseRadioStart=()=>resolve(startFetch(url,options));}):startFetch(url,options);document.querySelector('[data-action=start]').click();}`)
	wait(`()=>!!window.releaseRadioStart`)
	if !page.MustEval(`()=>{const box=document.querySelector('[data-pr=preparation]');return !box.hidden&&box.innerText.includes('Sender wird gestartet')&&document.querySelector('[data-action=start]').disabled&&document.querySelector('[data-action=start]').getAttribute('aria-busy')==='true';}`).Bool() {
		t.Fatal("start has no immediate prominent pending feedback")
	}
	page.MustEval(`()=>{releaseRadioStart();window.fetch=startFetch;}`)
	wait(`()=>document.querySelector('[data-pr=preparation-title]').textContent==='Begrüßung wird vorbereitet…'`)
	if got := page.MustElement("[data-pr=preparation-counts]").MustText(); got != "0 / 2 Titel · 0 / 5 min bereit" {
		t.Fatal("incorrect initial progress", got)
	}
	close(openingPlan)
	wait(`()=>PersonalRadioRuntime.state.state.opening_status==='playing'&&PersonalRadioRuntime.position().position>0`)
	if got := svc.Snapshot(); got.MusicReady || got.BufferMS != 0 || got.Current == "" {
		t.Fatal("opening did not play before music readiness", got)
	}
	wait(`()=>PersonalRadioRuntime.state.state.opening_status==='done'&&PersonalRadioRuntime.state.state.music_busy&&document.querySelector('[data-pr=preparation-title]').textContent===t('personalRadio.generating')`)
	if !page.MustEval(`()=>{const box=document.querySelector('[data-pr=preparation]'),app=document.querySelector('.pr-app'),button=document.querySelector('[data-action=stop]');const r=box.getBoundingClientRect(),a=app.getBoundingClientRect();return !box.hidden&&r.top>=a.top&&r.bottom<a.bottom&&parseFloat(getComputedStyle(document.querySelector('[data-pr=preparation-title]')).fontSize)>=18&&!button.disabled&&document.querySelector('[data-pr=preparation-hint]').innerText.includes('automatisch');}`).Bool() {
		t.Fatal("preparation is not prominent and actionable after the opening")
	}
	assertLocalized()
	captureLayouts("startup-")
	page.MustEval(`()=>{document.querySelector('[data-app-id="personal-radio"]').style.height='600px';document.querySelector('[data-action=stop]').scrollIntoView({block:'nearest',behavior:'instant'});}`)
	if !page.MustEval(`()=>{const button=document.querySelector('[data-action=stop]'),app=document.querySelector('.pr-app'),r=button.getBoundingClientRect(),a=app.getBoundingClientRect();return r.top>=a.top-1&&r.bottom<=a.bottom+1&&app.scrollWidth<=app.clientWidth+2;}`).Bool() {
		t.Fatal("stop is unreachable in a small window", page.MustEval(`()=>{const app=document.querySelector('.pr-app');return JSON.stringify({button:document.querySelector('[data-action=stop]').getBoundingClientRect().toJSON(),app:app.getBoundingClientRect().toJSON(),scrollTop:app.scrollTop,scrollWidth:app.scrollWidth,clientWidth:app.clientWidth,scrollHeight:app.scrollHeight,clientHeight:app.clientHeight});}`).Str())
	}
	page.MustEval(`()=>{document.querySelector('[data-app-id="personal-radio"]').style.height='800px';document.querySelector('.pr-app').scrollTop=0;}`)
	musicSteps <- 11
	wait(`()=>document.querySelector('[data-pr=preparation-counts]').textContent==='1 / 2 Titel · 3 / 5 min bereit'`)
	if got := svc.Snapshot(); got.MusicReady || got.Current != "" {
		t.Fatal("music started before reserve", got)
	}
	if !page.MustEval(`()=>document.querySelector('[data-pr=preparation-progress]').value===0.5`).Bool() {
		t.Fatal("progress does not reflect both track and duration requirements")
	}
	musicSteps <- 12
	wait(`()=>PersonalRadioRuntime.state.state.music_ready&&PersonalRadioRuntime.position().position>500&&document.querySelector('[data-pr=preparation]').hidden`)
	if got := svc.Snapshot(); got.Status != "playing" || got.Queue[0].Kind != "music" {
		t.Fatal("music did not start automatically", got)
	}
	page.MustElement(".pr-app [data-action=stop]").MustClick()
	wait(`()=>PersonalRadioRuntime.state.state.status==='stopped'`)
	if errors := page.MustEval(`()=>JSON.stringify(radioErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
	page.MustNavigate(origin.URL + "/fixture?lang=en").MustWaitLoad()
	page.MustEval(`async()=>await fixtureReady`)
	wait(`()=>!!document.querySelector('.pr-app')`)
	assertLocalized()
	if got := page.MustElement(".pr-app [data-action=new]").MustText(); got != "＋ New station" {
		t.Fatal("English app label:", got)
	}
}

// OfflineAudioContext renders the actual production player with 120 short PCM
// segments. Checking every rendered sample catches gaps hidden by fake clocks.
func TestPersonalRadioAudioContinuityBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	origin := httptest.NewServer(http.FileServer(http.FS(ui.Content)))
	defer origin.Close()
	page := browser.MustPage(origin.URL + "/desktop.html").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	source, err := ui.Content.ReadFile("js/desktop/apps/personal-radio-player.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.AddScriptTag("", string(source)); err != nil {
		t.Fatal(err)
	}
	results := page.MustEval(`async()=>{
 const results=[];
 for(const mixed of [false,true]) {
  const rate=8000,total=120;
  const segments=Array.from({length:total},(_,i)=>({id:'s'+i,asset_id:'a'+i,kind:mixed&&i%7===3?'news':'music',duration_ms:mixed&&i%7===3?100:2500,rate:mixed?[8000,24000,44100][i%3]:8000}));
  let end=.15+segments[0].duration_ms/1000;
  for(let i=1;i<total;i++){const a=segments[i-1],b=segments[i];const overlap=a.kind==='music'&&b.kind==='music'?Math.min(2,a.duration_ms/8000,b.duration_ms/8000):0;end+=b.duration_ms/1000-overlap;}
  const ctx=new OfflineAudioContext(1,Math.ceil((end+1)*rate),rate);let starts=0,ends=0;const errors=[];
  const player=new PersonalRadioPlayer({audio:async id=>{const s=segments[Number(id.slice(1))],frames=s.duration_ms*s.rate/1000,a=new ArrayBuffer(44+frames*2),v=new DataView(a);v.setUint16(20,1,true);v.setUint16(22,1,true);v.setUint32(24,s.rate,true);v.setUint16(34,16,true);v.setUint32(40,frames*2,true);for(let i=44;i<a.byteLength;i+=2)v.setInt16(i,3000,true);return a;},event:(id,kind)=>{if(kind==='started')starts++;if(kind==='ended')ends++;if(kind==='failed')errors.push(id);},error:e=>errors.push(e.message),progress:()=>{}});
  // The offline clock is real; expose its suspended scheduling boundary as a
  // running state so the production pump can enqueue between rendered blocks.
  player.context=new Proxy(ctx,{get(target,key){if(key==='state')return 'running';const value=Reflect.get(target,key,target);return typeof value==='function'?value.bind(target):value;}});player.master=ctx.createGain();player.master.connect(ctx.destination);player.running=true;player.update(segments);await player.pump();
  const step=async at=>{await ctx.suspend(at);await player.pump();if(at+.5<end+.5)step(at+.5);await ctx.resume();};step(.5);
  const audio=await ctx.startRendering(),samples=audio.getChannelData(0);let gaps=0,longest=0,run=0,min=1,max=0;
  for(let i=Math.ceil(.16*rate);i<Math.floor((end-.01)*rate);i++){const v=Math.abs(samples[i]);if(v<.01){gaps++;run++;longest=Math.max(longest,run);}else run=0;min=Math.min(min,v);max=Math.max(max,v);}
  player.reset();results.push({mixed,starts,ends,gaps,longest_ms:longest/rate*1000,min,max,errors});
 }
 return results;
 }`)
	for _, result := range results.Arr() {
		t.Log(result.String())
		if result.Get("longest_ms").Num() > 50 || result.Get("starts").Int() != 120 || result.Get("errors").String() != "[]" {
			t.Fatal("audible gaps or missing transitions", result.String())
		}
	}
	recovery := page.MustEval(`async()=>{
 const ctx=new OfflineAudioContext(1,5*8000,8000),failures=[],starts=[];
 const player=new PersonalRadioPlayer({audio:async(id,offset)=>{if(id==='a0'&&offset>=90000)throw Error('radio_audio_failed');const a=new ArrayBuffer(44+30000*8*2),v=new DataView(a);v.setUint16(20,1,true);v.setUint16(22,1,true);v.setUint32(24,8000,true);v.setUint16(34,16,true);v.setUint32(40,a.byteLength-44,true);for(let i=44;i<a.byteLength;i+=2)v.setInt16(i,3000,true);return a;},event:(id,kind)=>{if(kind==='failed')failures.push(id);if(kind==='started')starts.push(id);},error:()=>{},progress:()=>{}});
 player.context=new Proxy(ctx,{get(target,key){if(key==='state')return 'running';const value=Reflect.get(target,key,target);return typeof value==='function'?value.bind(target):value;}});player.master=ctx.createGain();player.master.connect(ctx.destination);player.running=true;
 player.update(Array.from({length:3},(_,i)=>({id:'s'+i,asset_id:'a'+i,kind:'music',duration_ms:120000})));await player.pump();
 let resumedAt=0;
 const step=async at=>{await ctx.suspend(at);await player.pump();await player.pump();if(at===1)resumedAt=player.slots[0].start;if(at<2)step(2);await ctx.resume();};step(1);
 await ctx.startRendering();player.reset();return {failures,starts,resumedAt};
 }`)
	failures, starts := recovery.Get("failures").Arr(), recovery.Get("starts").Arr()
	if len(failures) != 1 || failures[0].Str() != "s0" || len(starts) != 2 || starts[0].Str() != "s0" || starts[1].Str() != "s1" || recovery.Get("resumedAt").Num() > 1.5 {
		t.Fatal("failed audio retained its old transition time", recovery.String())
	}
	opening := page.MustEval(`async()=>{
 const ctx=new OfflineAudioContext(1,7*8000,8000),starts=[],errors=[];
 const segments=[{id:'intro',asset_id:'intro',kind:'moderation',opening:true,duration_ms:1000},{id:'m1',asset_id:'m1',kind:'music',duration_ms:2000},{id:'m2',asset_id:'m2',kind:'music',duration_ms:2000}];
 const player=new PersonalRadioPlayer({audio:async id=>{const frames=segments.find(s=>s.id===id).duration_ms*8,a=new ArrayBuffer(44+frames*2),v=new DataView(a);v.setUint16(20,1,true);v.setUint16(22,1,true);v.setUint32(24,8000,true);v.setUint16(34,16,true);v.setUint32(40,frames*2,true);for(let i=44;i<a.byteLength;i+=2)v.setInt16(i,3000,true);return a;},event:(id,kind)=>{if(kind==='started')starts.push(id);},error:e=>errors.push(e.message),progress:()=>{}});
 player.context=new Proxy(ctx,{get(target,key){if(key==='state')return 'running';const value=Reflect.get(target,key,target);return typeof value==='function'?value.bind(target):value;}});player.master=ctx.createGain();player.master.connect(ctx.destination);player.running=true;
 player.update(segments.slice(0,1));await player.pump();const openingStart=player.slots[0].start;let primedEarly=false,musicStartedEarly=false;
 const steps=[.5,1.5,2,3,4,6];
 const pump=async index=>{const at=steps[index];await ctx.suspend(at);if(at===.5)player.update(segments.slice(0,2));if(at===2)player.update(segments);await player.pump();if(at<2){primedEarly||=player.primed;musicStartedEarly||=player.slots.some(s=>s.segment.kind==='music'&&s.start!=null);}if(index+1<steps.length)pump(index+1);await ctx.resume();};pump(0);
 const audio=await ctx.startRendering(),samples=audio.getChannelData(0);player.reset();
 return {openingStart,primedEarly,musicStartedEarly,starts,errors,openingSample:samples[4000],waitingSample:samples[12000],musicSample:samples[20000]};
 }`)
	openingStarts := opening.Get("starts").Arr()
	if opening.Get("openingStart").Num() != .15 || opening.Get("primedEarly").Bool() || opening.Get("musicStartedEarly").Bool() || len(openingStarts) != 3 || openingStarts[0].Str() != "intro" || openingStarts[1].Str() != "m1" || openingStarts[2].Str() != "m2" || opening.Get("errors").String() != "[]" || opening.Get("openingSample").Num() < .01 || opening.Get("waitingSample").Num() != 0 || opening.Get("musicSample").Num() < .01 {
		t.Fatal("opening bypassed music preparation or failed to bridge startup", opening.String())
	}
}

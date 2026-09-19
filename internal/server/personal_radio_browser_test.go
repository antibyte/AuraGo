package server

import (
	"aurago/internal/config"
	"aurago/internal/personalradio"
	"aurago/ui"
	"bytes"
	"context"
	"fmt"
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
window.fetch=(url,options)=>String(url).startsWith('/api/')&&!String(url).startsWith('/api/desktop/personal-radio/')?Promise.resolve(new Response('{}')):originalFetch(url,options);
window.fixtureReady=(async()=>{
 const words=await (await originalFetch('/lang/desktop/de.json')).json();window.i18n={t:key=>words[key]||key};window.t=key=>words[key]||key;
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
	svc, err := personalradio.New(personalradio.Options{Directory: filepath.Join(cfg.Directories.DataDir, "personal-radio")})
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
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/desktop-app-personal-radio.css"><style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	scripts := `<script src="/radio-shell.js"></script>`
	for _, name := range []string{"player", "runtime", "settings", ""} {
		file := "personal-radio"
		if name != "" {
			file += "-" + name
		}
		scripts += `<script src="/js/desktop/apps/` + file + `.js"></script>`
	}
	scripts += `<script src="/radio-fixture.js"></script>`
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
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
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
	wait(`()=>PersonalRadioRuntime.state?.stations?.length===1 && !!document.querySelector('.pr-app')`)
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
	page.MustElement("[data-pr-mini=open]").MustClick()
	wait(`()=>!!document.querySelector('.pr-app')`)
	page.MustElement(".pr-app [data-tab=library]").MustClick()
	wait(`()=>document.querySelectorAll('.pr-track').length===2`)
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{1060, 420} {
			page.MustEval(`(theme,width)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode='light';const w=document.querySelector('[data-app-id="personal-radio"]');w.style.width=width+'px';w.style.height='800px';w.style.left='10px';w.style.top='10px';}`, theme, width)
			if !page.MustEval(`()=>{const x=document.querySelector('.pr-app');return x.scrollWidth<=x.clientWidth+2}`).Bool() {
				t.Fatal("horizontal overflow", theme, width)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				os.MkdirAll(dir, 0755)
				png, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
				if err != nil {
					t.Fatal(err)
				}
				os.WriteFile(filepath.Join(dir, fmt.Sprintf("personal-radio-%s-%d.png", theme, width)), png, 0644)
			}
		}
	}
	page.MustElement(".pr-app [data-action=stop]").MustClick()
	wait(`()=>PersonalRadioRuntime.state.state.status==='stopped'`)
	page.MustElement(".pr-app [data-action=settings]").MustClick()
	wait(`()=>!!document.querySelector('.pr-settings')`)
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
	if errors := page.MustEval(`()=>JSON.stringify(radioErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
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
}

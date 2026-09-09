package ui

import (
	"aurago/internal/desktop"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// Exercise the real shell and lazy app modules against local, non-mutating fixtures.
func TestDesktopLeafyRuntimeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<script>window.bootErrors=[];addEventListener('error',e=>bootErrors.push(e.message));</script></head>`, 1)
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/aurora-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={renderWidgets,handleDesktopEvent,state,openApp,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,renderTaskbar,closeWindow,focusWindow,minimizeWindow,toggleMaximizeWindow,switchSpace,setWindowMenus,clearWindowMenus,showContextMenu,showDesktopContextMenu,closeContextMenu,iconMarkup,wireShellChromeControls,bindViewportMetrics,applyWindowSnap,disposeWebampMusic,handleDesktopKeydown,showWidgetManager,showAppManager,openDesktopFileDialog,openStartMenu,closeStartMenu,launchStandaloneWebamp};})();`
	apps, _ := json.Marshal(desktop.BuiltinApps())
	words := map[string]string{}
	fs.WalkDir(Content, "lang", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, "/de.json") {
			data, _ := fs.ReadFile(Content, path)
			part := map[string]string{}
			json.Unmarshal(data, &part)
			for key, value := range part {
				words[key] = value
			}
		}
		return nil
	})
	svc, err := desktop.NewService(desktop.Config{Enabled: true, WorkspaceDir: filepath.Join(t.TempDir(), "workspace"), DBPath: filepath.Join(t.TempDir(), "desktop.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	var clockMu sync.Mutex
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	var seq int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/desktop/plant", func(w http.ResponseWriter, r *http.Request) {
		clockMu.Lock()
		defer clockMu.Unlock()
		out, err := svc.Plant(r.Context(), now)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/api/desktop/plant/actions", func(w http.ResponseWriter, r *http.Request) {
		clockMu.Lock()
		defer clockMu.Unlock()
		var a desktop.PlantAction
		json.NewDecoder(r.Body).Decode(&a)
		out, err := svc.ApplyPlantAction(r.Context(), a, now)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(409)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "snapshot": out})
			return
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/fixture-grow", func(w http.ResponseWriter, r *http.Request) {
		clockMu.Lock()
		defer clockMu.Unlock()
		for h := 0; h < 24*21; h++ {
			now = now.Add(time.Hour)
			if h%24 == 0 {
				out, err := svc.Plant(context.Background(), now)
				if err != nil || out.Plant == nil {
					http.Error(w, "missing plant", 500)
					return
				}
				seq++
				_, err = svc.ApplyPlantAction(context.Background(), desktop.PlantAction{Action: "water", ID: fmt.Sprintf("fixture_water_%d", seq), Revision: out.Plant.Revision}, now)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
			if h%144 == 0 {
				out, _ := svc.Plant(context.Background(), now)
				seq++
				svc.ApplyPlantAction(context.Background(), desktop.PlantAction{Action: "fertilize", ID: fmt.Sprintf("fixture_feed_%d", seq), Revision: out.Plant.Revision}, now)
			}
		}
		w.Write([]byte("ok"))
	})
	mux.Handle("/files/desktop/Apps/nasscad/", http.StripPrefix("/files/desktop/Apps/nasscad/", http.FileServer(http.Dir("../internal/desktop/bundled_apps/nasscad"))))
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(words) })
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".mjs") {
			w.Header().Set("Content-Type", "text/javascript")
		}
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	}))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/aurora-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(apps)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture")
	defer page.Close()
	page.MustSetViewport(1920, 1080, 1, false)
	page.MustWaitLoad()
	page.Timeout(30 * time.Second).MustWait(`()=>typeof fixtureReady !== "undefined"`)
	if errors := page.MustEval(`()=>JSON.stringify(bootErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
	page.Timeout(30 * time.Second).MustEval(`async()=>{await fixtureReady;await document.fonts.ready}`)
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join("..", "reports", "leafy")
	}
	os.MkdirAll(dir, 0755)
	page.MustEval(`async()=>{
  await fixtureReady;
  const fetchFixture=window.fetch;
  window.fetch=(url,options)=>String(url).startsWith('/api/desktop/plant')?nativeFetch(url,options):fetchFixture(url,options);
  aurora.state.bootstrap.settings['desktop.show_widgets']=true;
  aurora.state.bootstrap.settings['appearance.wallpaper']='paper_waves';
  aurora.applyDesktopSettings();
  aurora.state.bootstrap.widgets=[{id:'builtin-leafy',title:'Leafy',type:'builtin',runtime:'builtin',visible:true,builtin:true,icon:'leafy'}];
  aurora.renderWidgets();
 }`)
	if err := page.Timeout(10 * time.Second).Wait(rod.Eval(`()=>!!window.AuraLeafy?.active?.layout`)); err != nil {
		page.MustScreenshot(filepath.Join(dir, "startup-error.png"))
		t.Fatal(page.MustEval(`()=>JSON.stringify({errors:[...bootErrors,...fixtureErrors],card:document.getElementById('vd-widgets').innerHTML,body:document.body.dataset,metrics:window.AuraLeafy?.active?.metrics()})`).Str())
	}
	page.MustElement(".vd-leafy-shell").MustClick()
	page.Timeout(10 * time.Second).MustElement("[data-action='replant']").MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>AuraLeafy.active.metrics().revision===1`)
	page.MustScreenshot(filepath.Join(dir, "desktop-seedling.png"))
	page.MustEval(`async()=>{const r=await nativeFetch('/fixture-grow');if(!r.ok)throw Error(await r.text());await aurora.handleDesktopEvent({type:'plant_changed'});}`)
	page.Timeout(20 * time.Second).MustWait(`()=>AuraLeafy.active.metrics().age>=500`)
	for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
		for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {390, 844}} {
			page.MustSetViewport(size[0], size[1], 1, size[0] < 821)
			if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: size[0] < 821}).Call(page); err != nil {
				t.Fatal(err)
			}
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`async ([theme,density])=>{aurora.state.bootstrap.settings['appearance.density']=density;fixtureTheme(theme);await fixtureOpen('files');await fixtureOpen('settings');fixtureArrange();await new Promise(r=>setTimeout(r,250));}`, []string{theme, density})
				suffix := ""
				if density == "compact" {
					suffix = "-compact"
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("desktop-%s-%dx%d%s.png", theme, size[0], size[1], suffix)))
				page.MustEval(`()=>fixtureCloseAll()`)
			}
		}
	}
	page.MustSetViewport(1920, 1080, 1, false)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>{fixtureTheme('standard');await new Promise(r=>setTimeout(r,250));const m=AuraLeafy.active.metrics();if(m.calls>35||m.triangles>320000)throw Error('GPU budget '+JSON.stringify(m));window.beforeLeafy=m;}`)
	t.Log("WebGL budget", page.MustEval(`()=>JSON.stringify(AuraLeafy.active.metrics())`).Str())
	page.MustEval(`async()=>{const n=AuraLeafy.active.metrics().frames;await new Promise(r=>setTimeout(r,300));if(AuraLeafy.active.metrics().frames!==n)throw Error('Idle renderer kept drawing');document.body.dataset.widgets='false';await new Promise(r=>setTimeout(r,100));const f=AuraLeafy.active.metrics().frames;await new Promise(r=>setTimeout(r,200));if(AuraLeafy.active.metrics().frames!==f||AuraLeafy.active.metrics().active)throw Error('Hidden renderer active');document.body.dataset.widgets='true';}`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>{const original=window.setTimeout;window.setTimeout=(fn,delay,...args)=>original(fn,delay===22000?10:delay,...args);document.body.dataset.animations='true';await new Promise(r=>original(r,80));const count=AuraLeafy.active.metrics().frames;await new Promise(r=>original(r,250));if(AuraLeafy.active.metrics().frames!==count)throw Error('Reduced motion drew breeze frames');document.body.dataset.animations='false';window.setTimeout=original;}`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{const move=document.querySelector('.vd-leafy-move');move.focus();move.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowLeft',bubbles:true}));if(!localStorage.getItem('aurago.leafy.anchor.v1'))throw Error('Pot position not saved');}`)
	grip := page.MustEval(`()=>{const r=document.querySelector('.vd-leafy-move').getBoundingClientRect();window.preDragFrames=AuraLeafy.active.metrics().frames;return{x:r.x+r.width/2,y:r.y+r.height/2}}`)
	page.Mouse.MustMoveTo(grip.Get("x").Num(), grip.Get("y").Num()).MustDown(proto.InputMouseButtonLeft).MustMoveTo(grip.Get("x").Num()+45, grip.Get("y").Num()-30)
	page.MustEval(`()=>{if(AuraLeafy.active.metrics().frames!==preDragFrames)throw Error('Drag rebuilt geometry');}`)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	page.MustEval(`()=>{if(document.querySelector('.vd-leafy-canvas').style.transform)throw Error('Drag left a canvas offset');}`)
	page.MustSetViewport(1366, 768, 2, false)
	page.MustEval(`()=>{fixtureTheme('fruity-dark');aurora.state.bootstrap.settings['appearance.density']='compact';aurora.applyDesktopSettings();}`)
	page.MustEval(`async()=>{await new Promise(r=>setTimeout(r,300))}`)
	page.MustScreenshot(filepath.Join(dir, "desktop-dpr2.png"))
	page.MustEval(`()=>{if(document.querySelector('.vd-leafy-panel').hidden)document.querySelector('.vd-leafy-shell').click();}`)
	page.Timeout(10 * time.Second).MustElement("[data-action='scissors']").MustClick()
	point := page.MustEval(`()=>{const c=document.querySelector('.vd-leafy-canvas'),r=c.getBoundingClientRect();for(const b of AuraLeafy.active.layout.branches)for(const p of b.points.slice(2)){const x=r.x+p.x,y=r.bottom-p.y;if(x>10&&x<innerWidth-10&&y>50&&y<innerHeight-80&&document.elementFromPoint(x,y)===c)return{x,y};}throw Error('No reachable stem')}`)
	page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num()).MustClick(proto.InputMouseButtonLeft)
	page.MustEval(`()=>{if(!document.querySelector('[data-branch]').value)throw Error('Pointer did not select stem')}`)
	page.Timeout(10 * time.Second).MustElement("[data-branch]").MustSelect("Ranke 1")
	page.Timeout(10 * time.Second).MustElement("[data-action='cut']").MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>!!document.querySelector('[data-action="undo_prune"]')`)
	page.MustScreenshot(filepath.Join(dir, "desktop-pruned.png"))
	page.Timeout(10 * time.Second).MustElement("[data-action='undo_prune']").MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>!document.querySelector('[data-action="undo_prune"]')`)
	page.Timeout(10 * time.Second).MustElement("[data-action='scissors']").MustClick()
	page.Timeout(10 * time.Second).MustElement("[data-action='vacation']").MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('[data-action="vacation"]').textContent.includes('beenden')`)
	page.Timeout(10 * time.Second).MustElement("[data-action='vacation']").MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('[data-action="vacation"]').textContent==='Urlaub'`)
	page.Timeout(10 * time.Second).MustElement("[data-action='hide']").MustClick()
	page.MustEval(`()=>{if(!document.querySelector('.vd-leafy').hidden||document.querySelector('.vd-leafy-shell').hidden)throw Error('Hide lost escape control')}`)
	page.Timeout(10 * time.Second).MustElement("[data-action='hide']").MustClick()
	page.MustEval(`()=>{const c=document.querySelector('.vd-leafy-canvas');c.dispatchEvent(new Event('webglcontextlost',{cancelable:true}));}`)
	page.Timeout(10 * time.Second).MustWait(`()=>AuraLeafy.active.metrics().renderer==='canvas2d'`)
	page.MustEval(`async()=>{await new Promise(r=>setTimeout(r,300))}`)
	page.MustScreenshot(filepath.Join(dir, "desktop-fallback.png"))
	page.MustEval(`async()=>{const rev=AuraLeafy.active.metrics().revision;document.querySelector('[data-action="water"]').click();for(let i=0;i<50&&AuraLeafy.active.metrics().revision===rev;i++)await new Promise(r=>setTimeout(r,20));if(AuraLeafy.active.metrics().revision!==rev+1)throw Error('Fallback care failed');}`)

	page.MustEval(`()=>{window.lastLeafy=AuraLeafy.active;aurora.switchSpace(2);aurora.switchSpace(1);if(AuraLeafy.active!==lastLeafy)throw Error('Spaces recreated plant');}`)
	t.Log(page.MustEval(`()=>JSON.stringify(AuraLeafy.active.metrics())`).Str())
	page.MustEval(`async()=>{const revision=AuraLeafy.active.metrics().revision;const widget=aurora.state.bootstrap.widgets[0];aurora.state.bootstrap.widgets=[];aurora.renderWidgets();aurora.state.bootstrap.widgets=[widget];aurora.renderWidgets();await new Promise(r=>setTimeout(r,700));if(AuraLeafy.active.metrics().revision!==revision)throw Error('Remount reset biology');if(document.querySelectorAll('.vd-leafy').length!==1)throw Error('Duplicate plant');}`)
	if errors := page.MustEval(`()=>JSON.stringify([...bootErrors,...fixtureErrors])`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
	page.MustEval(`()=>{aurora.state.bootstrap.widgets=[];aurora.renderWidgets();if(AuraLeafy.active||document.querySelector('.vd-leafy,.vd-leafy-panel,.vd-leafy-shell'))throw Error('Leafy lifecycle leaked');}`)
}

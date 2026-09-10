package ui

import (
	"aurago/internal/desktop"
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
	"testing"
	"time"
)

// Exercise the real shell and lazy app modules against local, non-mutating fixtures.
func TestDesktopAuroraBrowser(t *testing.T) {
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
	shell = shell[:cut] + `window.aurora={state,openApp,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,renderTaskbar,closeWindow,focusWindow,minimizeWindow,toggleMaximizeWindow,switchSpace,setWindowMenus,clearWindowMenus,showContextMenu,showDesktopContextMenu,closeContextMenu,iconMarkup,wireShellChromeControls,bindViewportMetrics,applyWindowSnap,disposeWebampMusic,handleDesktopKeydown,showWidgetManager,showAppManager,openDesktopFileDialog,openStartMenu,closeStartMenu,launchStandaloneWebamp};})();`
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
	mux := http.NewServeMux()
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
		dir = filepath.Join("..", "reports", "aurora")
	}
	os.MkdirAll(dir, 0755)
	phase := os.Getenv("AURAGO_AURORA_PHASE")
	if phase == "" {
		phase = "after"
	}
	if os.Getenv("AURAGO_SYSTEM_WORLD_MATRIX") == "1" {
		verifySystemWorldCity(t, page, dir)
		return
	}
	if os.Getenv("AURAGO_NOTES_MATRIX") == "1" {
		verifyNotesShell(t, page, dir)
		return
	}
	if os.Getenv("AURAGO_SHEETS_MATRIX") == "1" {
		verifySheetsShell(t, page, dir)
		return
	}
	if os.Getenv("AURAGO_WRITER_MATRIX") == "1" {
		verifyWriterShell(t, page, dir)
		return
	}
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		page.MustEval(`theme=>fixtureTheme(theme)`, theme)
		page.MustEval(`async()=>{await fixtureOpen('files');await fixtureOpen('settings');await fixtureOpen('agent-chat');fixtureArrange()}`)
		page.MustScreenshot(filepath.Join(dir, phase+"-"+theme+".png"))
		page.MustEval(`()=>fixtureCloseAll()`)
		for _, density := range []string{"comfortable", "compact"} {
			for _, dpr := range []int{1, 2} {
				page.MustSetViewport(1366, 768, float64(dpr), false)
				page.MustEval(`density=>{
                    aurora.state.bootstrap.settings['appearance.density']=density;
                    aurora.applyDesktopSettings();
                    aurora.showDesktopContextMenu({target:document.body,preventDefault(){},clientX:80,clientY:80});
                    const menu=document.querySelector('.vd-context-menu');
                    for(const item of menu.querySelectorAll(':scope > button, :scope > .vd-context-submenu > button')){
                        const icon=item.querySelector('.vd-context-icon > *');
                        if(!icon?.classList.contains('vd-mini-icon'))throw Error('Mixed menu icon family: '+item.textContent);
                    }
                }`, density)
				screenshot := filepath.Join(dir, fmt.Sprintf("context-%s-%s-dpr%d.png", theme, density, dpr))
				if dpr == 1 {
					page.MustElement(".vd-context-menu").MustScreenshot(screenshot)
				} else {
					// Rod's element clip uses CSS pixels at DPR 2; capture the viewport.
					page.MustScreenshot(screenshot)
				}
				page.MustEval(`()=>aurora.closeContextMenu(true)`)
			}
		}
		page.MustSetViewport(1920, 1080, 1, false)
	}
	if phase == "before" {
		return
	}
	page.MustEval(`async()=>{fixtureTheme('fruity-light');await fixtureOpen('files');await fixtureOpen('writer')}`)
	if result := page.MustEval(`()=>fixtureCheckMenus()`).Str(); result != "" {
		t.Fatal(result)
	}
	if result := page.MustEval(`()=>fixtureCheckIcons()`).Str(); result != "" {
		t.Fatal(result)
	}
	page.MustEval(`()=>fixtureCloseAll()`)

	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		page.MustEval(`async theme=>{fixtureTheme(theme);await fixtureOpen('files');window.fixtureWin=[...aurora.state.windows.values()].at(-1);Object.assign(fixtureWin.element.style,{left:'180px',top:'100px',width:'700px',height:'480px'});}`, theme)
		rect := page.MustEval(`()=>{const r=fixtureWin.element.getBoundingClientRect();return {x:r.x,y:r.y}}`)
		x, y := rect.Get("x").Num()+220, rect.Get("y").Num()+20
		page.Mouse.MustMoveTo(x, y).MustDown(proto.InputMouseButtonLeft).MustMoveTo(x+80, y+40).MustUp(proto.InputMouseButtonLeft)
		page.MustEval(`([x,y])=>{const r=fixtureWin.element.getBoundingClientRect();if(Math.abs(r.x-x-80)>2||Math.abs(r.y-y-40)>2)throw Error('Window drag failed');}`, []float64{rect.Get("x").Num(), rect.Get("y").Num()})
		edge := page.MustEval(`()=>{const r=fixtureWin.element.getBoundingClientRect(),h=fixtureWin.element.querySelector('.vd-resize-se').getBoundingClientRect();return {x:h.x+h.width/2,y:h.y+h.height/2,w:r.width,h:r.height}}`)
		x, y = edge.Get("x").Num(), edge.Get("y").Num()
		page.Mouse.MustMoveTo(x, y).MustDown(proto.InputMouseButtonLeft).MustMoveTo(x+60, y+40)
		page.MustEval(`async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))}`)
		page.Mouse.MustUp(proto.InputMouseButtonLeft)
		page.MustEval(`([w,h])=>{const r=fixtureWin.element.getBoundingClientRect();if(r.width<w+50||r.height<h+30)throw Error('Window resize failed '+JSON.stringify({r,w,h,theme:document.body.dataset.theme}));}`, []float64{edge.Get("w").Num(), edge.Get("h").Num()})
		page.MustEval(`()=>{aurora.applyWindowSnap(fixtureWin.element,'left-half');if(fixtureWin.snapped!=='left-half')throw Error('Snap failed');const r=fixtureWin.element.getBoundingClientRect();if(r.left<0||r.top<0||r.bottom>innerHeight)throw Error('Snap outside workspace');aurora.toggleMaximizeWindow(fixtureWin.id);if(!fixtureWin.maximized)throw Error('Maximize failed');aurora.toggleMaximizeWindow(fixtureWin.id);aurora.minimizeWindow(fixtureWin.id);aurora.focusWindow(fixtureWin.id);if(fixtureWin.element.style.display==='none')throw Error('Restore failed');}`)
		page.MustEval(`async()=>{await fixtureCheckWindowPopover();fixtureCloseAll()}`)
		page.MustEval(`()=>aurora.openStartMenu()`)
		page.MustScreenshot(filepath.Join(dir, "start-"+theme+".png"))
		page.MustEval(`()=>aurora.closeStartMenu()`)
		page.MustEval(`async()=>{await aurora.showAppManager()}`)
		page.MustScreenshot(filepath.Join(dir, "manager-"+theme+".png"))
		page.MustEval(`()=>document.querySelector('.vd-app-manager-backdrop [data-close]').click()`)
		page.MustEval(`()=>{aurora.openDesktopFileDialog({title:'Open file'});}`)
		page.MustScreenshot(filepath.Join(dir, "file-dialog-"+theme+".png"))
		page.MustEval(`()=>document.querySelector('.vd-file-dialog-close').click()`)
	}
	if os.Getenv("AURAGO_AURORA_MATRIX") == "1" {
		for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {390, 844}} {
			page.MustSetViewport(size[0], size[1], 1, size[0] < 821)
			if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: size[0] < 821}).Call(page); err != nil {
				t.Fatal(err)
			}
			for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
				for _, density := range []string{"comfortable", "compact"} {
					page.MustEval(`([theme,density])=>{aurora.state.bootstrap.settings['appearance.density']=density;fixtureTheme(theme)}`, []string{theme, density})
					page.MustEval(`async()=>{await fixtureOpen('files');await fixtureOpen('settings');await fixtureOpen('agent-chat');fixtureArrange()}`)
					name := fmt.Sprintf("%s-%s-%dx%d", theme, density, size[0], size[1])
					os.WriteFile(filepath.Join(dir, "matrix-"+name+".json"), []byte(page.MustEval(`()=>JSON.stringify({width:innerWidth,visual:visualViewport.width,coarse:matchMedia('(pointer:coarse)').matches,els:[...document.querySelectorAll('.vd-window.active,.vd-taskbar,.vd-taskbar-apps,.vd-taskbar-system')].map(e=>({class:e.className,rect:e.getBoundingClientRect().toJSON(),inline:e.style.cssText,css:Object.fromEntries(['width','height','max-width','max-height','left','right','transform','grid-template-rows'].map(k=>[k,getComputedStyle(e).getPropertyValue(k)]))}))})`).Str()), 0644)
					page.MustScreenshot(filepath.Join(dir, "matrix-"+name+".png"))
					page.MustEval(`async()=>{await fixtureEdgeMenu()}`)
					page.MustScreenshot(filepath.Join(dir, "menu-"+name+".png"))
					if size[0] < 821 {
						page.MustEval(`async()=>{aurora.closeContextMenu();await fixtureCheckTouchLayout()}`)
					}
					page.MustEval(`()=>fixtureCloseAll()`)
				}
			}
		}
		for _, dpr := range []int{1, 2} {
			page.MustSetViewport(1366, 900, float64(dpr), false)
			for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
				page.MustEval(`theme=>{fixtureTheme(theme);fixtureIconContact()}`, theme)
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("icons-%s-dpr%d.png", theme, dpr)))
				page.MustEval(`()=>document.getElementById('fixture-icon-contact').remove()`)
			}
		}
	}
	page.MustSetViewport(1920, 1080, 1, false)
	(proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	if os.Getenv("AURAGO_AURORA_ALL_APPS") == "1" {
		for _, app := range desktop.BuiltinApps() {
			if only := os.Getenv("AURAGO_AURORA_APP"); only != "" && app.ID != only {
				continue
			}
			t.Run(app.ID, func(t *testing.T) {
				// Retro apps keep their specialist browser acceptance; still capture their shell.
				page.Timeout(45*time.Second).MustEval(`async id=>{fixtureTheme('standard');await fixtureOpen(id)}`, app.ID)
				for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
					page.MustEval(`theme=>fixtureTheme(theme)`, theme)
					page.MustScreenshot(filepath.Join(dir, "app-"+app.ID+"-"+theme+".png"))
				}
				os.WriteFile(filepath.Join(dir, "app-"+app.ID+".json"), []byte(page.MustEval(`()=>fixtureCaptureInfo()`).Str()), 0644)
				result := page.MustEval(`id=>fixtureAppResult(id)`, app.ID).Str()
				if result != "" {
					t.Error(result)
					if app.ID == "music-player" {
						t.Log(page.MustEval(`async()=>{try{const m=await import('/js/vendor/webamp/webamp.bundle.min.mjs');return JSON.stringify({keys:Object.keys(m),supported:m.default?.browserIsSupported?.(),instance:!!aurora.state.webampMusic,host:document.getElementById('vd-webamp-host')?.outerHTML.slice(0,400),ids:[...document.querySelectorAll('[id]')].map(x=>x.id).filter(x=>/webamp|main-window|playlist/.test(x))})}catch(e){return e.stack}}`).Str())
					}
				}
				page.MustEval(`()=>fixtureCloseAll()`)
			})
		}
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

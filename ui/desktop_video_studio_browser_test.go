package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopVideoStudioBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var pngBytes bytes.Buffer
	art := image.NewRGBA(image.Rect(0, 0, 512, 288))
	for y := 0; y < 288; y++ {
		for x := 0; x < 512; x++ {
			art.Set(x, y, color.RGBA{R: uint8(34 + x/8), G: uint8(50 + y/5), B: uint8(94 + (x+y)/9), A: 255})
		}
	}
	if err := png.Encode(&pngBytes, art); err != nil {
		t.Fatal(err)
	}

	project := map[string]any{
		"version": 1, "name": "Browser fixture", "width": 1280, "height": 720, "fps": 30,
		"assets": []any{map[string]any{"id": "art1", "name": "Coast.png", "path": "media/art1_coast.png", "kind": "image", "duration_frames": 18000, "width": 512, "height": 288, "has_audio": false}},
		"tracks": []any{
			map[string]any{"id": "v1", "kind": "video", "name": "Video 1", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "v2", "kind": "video", "name": "Video 2", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "a1", "kind": "audio", "name": "Audio 1", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "a2", "kind": "audio", "name": "Audio 2", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "o1", "kind": "overlay", "name": "Overlay 1", "muted": false, "hidden": false, "locked": false, "clips": []any{map[string]any{
				"id": "c1", "asset_id": "art1", "start": 0, "offset": 0, "duration": 150,
				"x": 0, "y": 0, "width": 1, "height": 1, "rotation": 0, "opacity": 1, "volume": 1,
				"fade_in": 0, "fade_out": 0, "fit": "contain", "text": "", "text_style": nil, "transition": nil,
			}}},
		},
	}
	locales := make(map[string]map[string]string, 2)
	for _, language := range []string{"en", "de"} {
		var source map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+language+".json")), &source); err != nil {
			t.Fatalf("parse %s desktop locale: %v", language, err)
		}
		locales[language] = make(map[string]string)
		for key, value := range source {
			if strings.HasPrefix(key, "videoStudio.") {
				locales[language][key] = value
			}
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-video-studio.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}.desktop-body{font-family:system-ui,sans-serif;color:var(--vd-text,#edf0f5);background:var(--vd-theme-app-bg);}.vd-window-content{height:100%;width:100%;display:flex}#studio{flex:1;min-width:0;min-height:0}.vs-app{flex:1}.desktop-body[data-theme="fruity"]{--vd-text:#273144}</style></head><body class="desktop-body" data-theme="standard" data-fruity-mode="light"><main class="vd-window-content"><div id="studio"></div></main><script src="/js/desktop/apps/video-studio-preview.js"></script><script src="/js/desktop/apps/video-studio-timeline.js"></script><script src="/js/desktop/apps/video-studio.js"></script><script>
			window.fixtureErrors=[];window.fixtureCalls=[];window.addEventListener('error',e=>fixtureErrors.push(String(e.message)));
			window.fixtureLocales=`+mustJSON(t, locales)+`;const fixtureParams=new URLSearchParams(location.search);window.fixtureLang=fixtureParams.get('lang')||'en';document.body.dataset.theme=fixtureParams.get('theme')||'standard';document.documentElement.lang=window.fixtureLang;
			window.fixtureProject=`+mustJSON(t, project)+`;window.fixtureLatest=structuredClone(window.fixtureProject);window.fixtureReadonly=fixtureParams.has('readonly');window.fixtureConflict=false;window.fixtureConflictUsed=false;window.fixtureJobs=[];window.fixtureSaves=[];
			const originalFetch=window.fetch;const json=(data,status=200,headers={})=>new Response(JSON.stringify(data),{status,headers:Object.assign({'Content-Type':'application/json'},headers)});
			window.fetch=async(url,options={})=>{const path=String(url),method=String(options.method||'GET').toUpperCase();window.fixtureCalls.push([method,path]);
			if(path==='/api/desktop/video-studio/status')return json({enabled:true,desktop_enabled:true,ffmpeg_ready:true,read_only:window.fixtureReadonly,limits:{max_duration_frames:18000,canvas_sizes:[{width:1280,height:720},{width:720,height:1280},{width:720,height:720}]},generation:{enabled:true,configured:true,provider:'Fixture provider',model:'fixture-model',durations_seconds:[5,10],image_modes:[]}});
			if(path==='/api/desktop/video-studio/projects'&&method==='GET')return json({projects:[{id:'p1',project:{name:'Browser fixture'},desktop_path:'Documents/Video Studio/p1/project.json'}]});
			if(path==='/api/desktop/video-studio/projects/p1'&&method==='GET'){const p=window.fixtureConflict?window.fixtureLatest:window.fixtureProject;return json({id:'p1',project:p,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:window.fixtureConflict?'"vRemote"':'"v1"'});}
			if(path==='/api/desktop/video-studio/projects/p1'&&method==='PUT'){window.fixtureSaves.push(JSON.parse(options.body));window.fixtureLastIfMatch=new Headers(options.headers).get('If-Match');if(window.fixtureConflict&&!window.fixtureConflictUsed){window.fixtureConflictUsed=true;return json({error:'file_conflict',code:'file_conflict' },412);}window.fixtureProject=JSON.parse(options.body);return json({project:window.fixtureProject,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v2"'});}
			if(path==='/api/desktop/video-studio/jobs?project_id=p1')return json({jobs:window.fixtureJobs});
			if(path==='/api/desktop/video-studio/projects/p1/jobs'&&method==='POST'){window.fixtureLastRenderIfMatch=new Headers(options.headers).get('If-Match');const job={id:'j1',project_id:'p1',kind:'render',status:'queued',progress:0};window.fixtureJobs=[job];return json({job},202);}
			if(path==='/api/desktop/video-studio/jobs/j1')return json({job:{id:'j1',project_id:'p1',kind:'render',status:'succeeded',progress:1,artifact:{name:'Browser fixture.mp4',download_url:'/fixture.mp4',size:1234}}});
			return originalFetch(url,options);};
			const t=key=>window.fixtureLocales[window.fixtureLang]?.[key]||key;
			const paths={save:'M5 3h12l4 4v14H3V3h2zm2 2v5h10V5H7zm1 9v5h8v-5H8z',video:'M3 5h18v14H3zM10 9v6l5-3z',upload:'M12 16V4m-5 5 5-5 5 5M4 16v4h16v-4',folder:'M3 6h7l2 2h9v11H3z',scissors:'M5 6a2 2 0 1 0 0 4 2 2 0 0 0 0-4zm0 8a2 2 0 1 0 0 4 2 2 0 0 0 0-4zM7 9l12 9M7 15 19 6'};
			const ctx={t,esc:v=>String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),iconMarkup:key=>'<svg class="vs-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="'+(paths[key]||paths.video)+'"/></svg>',setWindowBeforeClose:(id,fn)=>window.fixtureCloseGuard=fn,setWindowMenus:()=>{},clearWindowMenus:()=>{},promptDialog:async()=>false,confirmDialog:async()=>false};
			window.VideoStudioApp.render(document.getElementById('studio'),'fixture',ctx);
		</script></body></html>`)
	})
	mux.HandleFunc("/api/desktop/video-studio/projects/p1/media/art1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes.Bytes())
	})
	mux.HandleFunc("/fixture.mp4", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("fixture"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(45 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>document.querySelector('.vs-app') && (document.querySelectorAll('.vs-track-row').length===5 || document.querySelector('[data-notice]')?.textContent)`)
	if !page.MustEval(`()=>document.querySelectorAll('.vs-track-row').length===5 && !!document.querySelector('.vs-clip')`).Bool() {
		t.Fatalf("Video Studio fixture failed to load: %s", page.MustEval(`()=>JSON.stringify({notice:document.querySelector('[data-notice]')?.textContent,disabled:document.querySelector('[data-disabled]')?.textContent,calls:window.fixtureCalls,errors:window.fixtureErrors})`).Str())
	}
	if errs := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).Str(); errs != "[]" {
		t.Fatalf("Video Studio fixture errors: %s", errs)
	}
	if !page.MustEval(`()=>document.querySelector('.vs-app').getBoundingClientRect().width===1280 && document.querySelector('.vs-export-button').disabled===false`).Bool() {
		t.Fatalf("wide editor did not render a usable project: %s", page.MustEval(`()=>JSON.stringify({rect:document.querySelector('.vs-app').getBoundingClientRect().toJSON(),disabled:document.querySelector('.vs-export-button').disabled,readonly:window.VideoStudioApp.instances.get('fixture').readonly,status:window.VideoStudioApp.instances.get('fixture').status})`).Str())
	}
	screenshotDir := os.Getenv("AURAGO_VIDEO_STUDIO_SCREENSHOTS")
	if screenshotDir == "" {
		screenshotDir = filepath.Join("reports", "video-studio-browser")
	}
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		t.Fatal(err)
	}
	page.MustScreenshot(filepath.Join(screenshotDir, "standard-1280x900.png"))
	page.MustNavigate(server.URL + "/fixture?theme=fruity&lang=de")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>document.querySelector('.vs-clip') && document.querySelector('[data-action="new-project"]')?.textContent.includes('Neues Projekt')`)
	if !page.MustEval(`()=>{const probe=document.createElement('span');probe.style.color=getComputedStyle(document.querySelector('.vs-app')).getPropertyValue('--vs-muted');document.body.append(probe);const expected=getComputedStyle(probe).color;probe.remove();return getComputedStyle(document.querySelector('.vs-timecode')).color===expected}`).Bool() {
		t.Fatalf("Fruity timecode does not use the readable theme muted color: %s", page.MustEval(`()=>JSON.stringify({timecode:getComputedStyle(document.querySelector('.vs-timecode')).color,muted:getComputedStyle(document.querySelector('.vs-app')).getPropertyValue('--vs-muted')})`).Str())
	}
	page.MustScreenshot(filepath.Join(screenshotDir, "fruity-1280x900.png"))
	page.MustEval(`async()=>{document.querySelector('[data-action="open-ai"]').click();await new Promise(resolve=>setTimeout(resolve,50));}`)
	if !page.MustEval(`()=>!!document.querySelector('[data-ai-form]')`).Bool() {
		t.Fatalf("AI action did not open: %s", page.MustEval(`()=>JSON.stringify({button:document.querySelector('[data-action="open-ai"]')?.outerHTML,notice:document.querySelector('[data-notice]')?.textContent,readonly:window.VideoStudioApp.instances.get('fixture').readonly,generation:window.VideoStudioApp.instances.get('fixture').status.generation,errors:window.fixtureErrors})`).Str())
	}
	if page.MustEval(`()=>!!document.querySelector('[data-ai-image]')`).Bool() {
		t.Fatal("provider without first-frame support received an image control")
	}
	if !page.MustEval(`()=>document.querySelector('[data-ai-ratio]').options.length===3 && document.querySelector('.vs-modal').getBoundingClientRect().height>0`).Bool() {
		t.Fatal("AI modal lacks ratios or is not reachable")
	}
	page.MustEval(`()=>document.querySelector('[data-modal-close]').click()`)

	// Exercise a real pointer gesture through the timeline's document/window listeners.
	page.MustEval(`()=>document.querySelector('.vs-clip').scrollIntoView({block:'center'})`)
	point := page.MustEval(`()=>{const r=document.querySelector('.vs-clip').getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}}`)
	page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num()).MustDown(proto.InputMouseButtonLeft).MustMoveTo(point.Get("x").Num()+52, point.Get("y").Num()).MustUp(proto.InputMouseButtonLeft)
	waitForJSBool(t, page, `()=>window.VideoStudioApp.instances.get('fixture').project.tracks.find(t=>t.id==='o1').clips[0].start===30`)
	page.MustElement(`[data-action="undo"]`).MustClick()
	waitForJSBool(t, page, `()=>window.VideoStudioApp.instances.get('fixture').project.tracks.find(t=>t.id==='o1').clips[0].start===0`)
	if !page.MustEval(`()=>document.querySelector('.vs-clip.is-selected') && document.querySelector('.vs-inspector').getBoundingClientRect().width>0`).Bool() {
		t.Fatal("selected clip and inspector are not both visible in the wide editor")
	}
	page.MustScreenshot(filepath.Join(screenshotDir, "selected-clip-inspector-1280x900.png"))
	page.MustSetViewport(390, 844, 1, false)
	page.MustEval(`()=>document.querySelector('.vs-inspector-toggle').click()`)
	if page.MustEval(`()=>document.querySelector('.vs-app').scrollWidth>document.querySelector('.vs-app').clientWidth+1`).Bool() {
		t.Fatal("Video Studio overflows at 390px")
	}
	page.MustScreenshot(filepath.Join(screenshotDir, "mobile-390x844.png"))
	if !page.MustEval(`()=>document.querySelector('.vs-app').classList.contains('vs-show-inspector') && document.querySelector('.vs-inspector').getBoundingClientRect().width>0`).Bool() {
		t.Fatal("narrow inspector did not open as a visible drawer")
	}
	page.MustEval(`()=>document.querySelector('.vs-inspector').dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))`)

	page.MustSetViewport(1280, 900, 1, false)
	page.MustEval(`()=>document.querySelector('[data-action="export"]').click()`)
	if !page.MustEval(`()=>!!document.querySelector('[data-export-form]')`).Bool() {
		t.Fatalf("export action did not open: %s", page.MustEval(`()=>JSON.stringify({button:document.querySelector('[data-action="export"]')?.outerHTML,rect:document.querySelector('.vs-app').getBoundingClientRect().toJSON(),notice:document.querySelector('[data-notice]')?.textContent,readonly:window.VideoStudioApp.instances.get('fixture').readonly,epoch:window.VideoStudioApp.instances.get('fixture').projectEpoch,errors:window.fixtureErrors})`).Str())
	}
	if !page.MustEval(`()=>document.querySelector('.vs-modal').getBoundingClientRect().height>0`).Bool() {
		t.Fatal("export dialog is not visible")
	}
	page.MustEval(`()=>document.querySelector('[data-export-form] button[type="submit"]').click()`)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !page.MustEval(`()=>window.fixtureCalls.some(([method,path])=>method==='POST'&&path==='/api/desktop/video-studio/projects/p1/jobs')`).Bool() {
		time.Sleep(25 * time.Millisecond)
	}
	if !page.MustEval(`()=>window.fixtureCalls.some(([method,path])=>method==='POST'&&path==='/api/desktop/video-studio/projects/p1/jobs')`).Bool() {
		t.Fatalf("export submit did not create a render job: %s", page.MustEval(`()=>{const s=window.VideoStudioApp.instances.get('fixture');return JSON.stringify({calls:window.fixtureCalls,saves:window.fixtureSaves,etag:s.etag,dirty:s.dirty,saveState:s.saveState,dragging:s.timelineDragging,notice:s.q('[data-notice]').textContent,errors:window.fixtureErrors})}`).Str())
	}
	if got := page.MustEval(`()=>window.fixtureLastRenderIfMatch`).Str(); got != `"v2"` {
		t.Fatalf("render did not bind to saved project ETag: %q", got)
	}
	page.MustEval(`()=>{window.fixtureConflict=true;const input=document.querySelector('[data-field="start"]');input.value='45';input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('[data-action="save"]').click()}`)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !page.MustEval(`()=>!!document.querySelector('[data-conflict="replace"]')`).Bool() {
		time.Sleep(25 * time.Millisecond)
	}
	if !page.MustEval(`()=>!!document.querySelector('[data-conflict="replace"]')`).Bool() {
		t.Fatalf("stale save did not show conflict choices: %s", page.MustEval(`()=>{const s=window.VideoStudioApp.instances.get('fixture');return JSON.stringify({start:document.querySelector('[data-field="start"]')?.value,selected:s.selectedClipId,dirty:s.dirty,conflict:s.conflict,etag:s.etag,used:window.fixtureConflictUsed,lastIfMatch:window.fixtureLastIfMatch,calls:window.fixtureCalls,saves:window.fixtureSaves,host:s.q('[data-conflict-host]').innerHTML,notice:s.q('[data-notice]').textContent,errors:window.fixtureErrors})}`).Str())
	}
	page.MustEval(`()=>document.querySelector('[data-conflict="replace"]').click()`)
	waitForJSBool(t, page, `()=>window.fixtureLastIfMatch==='"vRemote"'`)
	if !page.MustEval(`()=>window.fixtureConflictUsed && !window.VideoStudioApp.instances.get('fixture').conflict`).Bool() {
		t.Fatal("conflict Replace did not use the observed latest version")
	}

	// Read-only is enforced in the app, not merely by disabled presentation.
	page.MustNavigate(server.URL + "/fixture?readonly=1")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>document.querySelector('.vs-clip') && document.querySelector('.vs-app').classList.contains('is-readonly')`)
	if !page.MustEval(`()=>document.querySelector('.vs-export-button').disabled && document.querySelector('[data-canvas]').disabled && document.querySelector('[data-place-asset]').disabled`).Bool() {
		t.Fatal("read-only editor left a write control enabled")
	}
	before := page.MustEval(`()=>JSON.stringify(window.VideoStudioApp.instances.get('fixture').project)`).Str()
	page.MustEval(`()=>document.querySelector('.vs-app').dispatchEvent(new KeyboardEvent('keydown',{key:'z',ctrlKey:true,bubbles:true}))`)
	if after := page.MustEval(`()=>JSON.stringify(window.VideoStudioApp.instances.get('fixture').project)`).Str(); after != before {
		t.Fatal("read-only keyboard undo changed the project")
	}

}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

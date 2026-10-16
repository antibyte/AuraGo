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

	"github.com/go-rod/rod/lib/input"
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
		_, _ = fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-video-studio.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}.desktop-body{font-family:system-ui,sans-serif;color:var(--vd-text,#edf0f5);background:var(--vd-theme-app-bg);}.vd-window-content{height:100%;width:100%;display:flex}#studio{flex:1;min-width:0;min-height:0}.vs-app{flex:1}.desktop-body[data-theme="fruity"]{--vd-text:#273144}</style></head><body class="desktop-body" data-theme="standard" data-fruity-mode="light"><main class="vd-window-content"><div id="studio"></div></main><script src="/js/desktop/apps/video-studio-icons.js"></script><script src="/js/desktop/apps/video-studio-time.js"></script><script src="/js/desktop/apps/video-studio-media.js"></script><script src="/js/desktop/apps/video-studio-preview.js"></script><script src="/js/desktop/apps/video-studio-timeline.js"></script><script src="/js/desktop/apps/video-studio-ruler.js"></script><script src="/js/desktop/apps/video-studio-edit.js"></script><script src="/js/desktop/apps/video-studio-navigation.js"></script><script src="/js/desktop/apps/video-studio-source-edit.js"></script><script src="/js/desktop/apps/video-studio-folders.js"></script><script src="/js/desktop/apps/video-studio-inspector.js"></script><script src="/js/desktop/apps/video-studio-core.js"></script><script src="/js/desktop/apps/video-studio-library.js"></script><script src="/js/desktop/apps/video-studio-artwork.js"></script><script src="/js/desktop/apps/video-studio-jobs.js"></script><script src="/js/desktop/apps/video-studio-stage.js"></script><script src="/js/desktop/apps/video-studio-editing.js"></script><script src="/js/desktop/apps/video-studio-source.js"></script><script src="/js/desktop/apps/video-studio-tracks.js"></script><script src="/js/desktop/apps/video-studio-markers.js"></script><script src="/js/desktop/apps/video-studio-transport.js"></script><script src="/js/desktop/apps/video-studio-derive.js"></script><script src="/js/desktop/apps/video-studio-projects.js"></script><script src="/js/desktop/apps/video-studio-mediabin.js"></script><script src="/js/desktop/apps/video-studio-menus.js"></script><script src="/js/desktop/apps/video-studio.js"></script><script>
			window.fixtureErrors=[];window.fixtureCalls=[];window.addEventListener('error',e=>fixtureErrors.push(String(e.message)));
			window.fixtureLocales=`+mustJSON(t, locales)+`;const fixtureParams=new URLSearchParams(location.search);window.fixtureLang=fixtureParams.get('lang')||'en';document.body.dataset.theme=fixtureParams.get('theme')||'standard';document.documentElement.lang=window.fixtureLang;
			window.fixtureProject=`+mustJSON(t, project)+`;window.fixtureLatest=structuredClone(window.fixtureProject);window.fixtureReadonly=fixtureParams.has('readonly');window.fixtureConflict=false;window.fixtureConflictUsed=false;window.fixtureJobs=[];window.fixtureSaves=[];window.fixtureArtworkRequests=[];window.fixtureArtworkStates={};window.fixtureLastJobBody=null;
			const originalFetch=window.fetch;const json=(data,status=200,headers={})=>new Response(JSON.stringify(data),{status,headers:Object.assign({'Content-Type':'application/json'},headers)});
			window.fetch=async(url,options={})=>{const path=String(url),method=String(options.method||'GET').toUpperCase();window.fixtureCalls.push([method,path]);
			if(path==='/api/desktop/video-studio/status')return json({enabled:true,desktop_enabled:true,ffmpeg_ready:true,read_only:window.fixtureReadonly,limits:{max_duration_frames:18000,max_duration_seconds:3600,frame_rates:[{fps:24,ntsc:true},{fps:25},{fps:30},{fps:60}],canvas_sizes:[{width:1280,height:720},{width:720,height:1280},{width:720,height:720}]},generation:{enabled:true,configured:true,provider:'Fixture provider',model:'fixture-model',durations_seconds:[5,10],image_modes:[]}});
			if(path==='/api/desktop/video-studio/projects'&&method==='GET')return json({projects:[{id:'p1',project:{name:'Browser fixture'},desktop_path:'Documents/Video Studio/p1/project.json'}]});
			if(path==='/api/desktop/video-studio/projects/p1'&&method==='GET'){const p=window.fixtureConflict?window.fixtureLatest:window.fixtureProject;return json({id:'p1',project:p,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:window.fixtureConflict?'"vRemote"':'"v1"'});}
			if(path==='/api/desktop/video-studio/projects?sizes=1'&&method==='GET')return json({projects:(window.fixtureDeleted||[]).length?[]:[{id:'p1',project:{name:'Browser fixture'},desktop_path:'Documents/Video Studio/p1/project.json',storage_bytes:2097152,updated_at:'2026-10-15T09:00:00Z'}]});
			if(path==='/api/desktop/video-studio/projects/p1'&&method==='DELETE'){window.fixtureDeleted=(window.fixtureDeleted||[]).concat(path);return json({deleted:true,id:'p1'});}
			if(path==='/api/desktop/video-studio/projects/p1/versions'&&method==='GET')return json({versions:[{id:'11111111-1111-4111-8111-111111111111',kind:'auto',label:'',created_at:'2026-10-15T10:00:00Z',size_bytes:1000,name:'Browser fixture'}]});
			if(path==='/api/desktop/video-studio/projects/p1/versions/11111111-1111-4111-8111-111111111111/restore'&&method==='POST'){window.fixtureRestoreIfMatch=new Headers(options.headers).get('If-Match');window.fixtureRestoreSaves=window.fixtureSaves.length;return json({id:'p1',project:window.fixtureProject,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v2"'});}
			if(path==='/api/desktop/video-studio/projects/p1'&&method==='PUT'){window.fixtureSaves.push(JSON.parse(options.body));window.fixtureLastIfMatch=new Headers(options.headers).get('If-Match');if(window.fixtureConflict&&!window.fixtureConflictUsed){window.fixtureConflictUsed=true;return json({error:'file_conflict',code:'file_conflict' },412);}window.fixtureProject=JSON.parse(options.body);return json({project:window.fixtureProject,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v2"'});}
			if(path==='/api/desktop/video-studio/projects/p1/media'&&method==='POST'){const id='art-job-'+(window.fixtureArtworkRequests.length+1),assetId='art-asset-'+(window.fixtureArtworkRequests.length+1),job={id,project_id:'p1',kind:'probe',status:'queued',progress:0};window.fixtureArtworkStates[id]=job;window.fixtureArtworkRequests.push({id,assetId});return json({job,asset:{id:assetId,name:'Title.png',path:'media/'+assetId+'.png',kind:'image',duration_frames:18000,width:1280,height:720,has_audio:false}},202);}if(path.startsWith('/api/desktop/video-studio/jobs/art-job-'))return json({job:window.fixtureArtworkStates[path.split('/').pop()]});if(path==='/api/desktop/video-studio/jobs?project_id=p1')return json({jobs:window.fixtureJobs});
			if(path==='/api/desktop/video-studio/projects/p1/jobs'&&method==='POST'){window.fixtureLastRenderIfMatch=new Headers(options.headers).get('If-Match');window.fixtureLastJobBody=JSON.parse(options.body);const job={id:'j1',project_id:'p1',kind:'render',status:'queued',progress:0};window.fixtureJobs=[job];return json({job},202);}
			if(path==='/api/desktop/video-studio/jobs/j1/cancel'&&method==='POST')return json({error:'job_finished',code:'job_finished',message:'Already finished.',job:{id:'j1',project_id:'p1',kind:'render',status:'succeeded',progress:1}},409);
			if(path==='/api/desktop/video-studio/jobs/j1')return json({job:{id:'j1',project_id:'p1',kind:'render',status:'succeeded',progress:1,artifact:{name:'Browser fixture.mp4',download_url:'/fixture.mp4',size:1234}}});
			return originalFetch(url,options);};
			const t=key=>window.fixtureLocales[window.fixtureLang]?.[key]||key;
			const paths={save:'M5 3h12l4 4v14H3V3h2zm2 2v5h10V5H7zm1 9v5h8v-5H8z',video:'M3 5h18v14H3zM10 9v6l5-3z',upload:'M12 16V4m-5 5 5-5 5 5M4 16v4h16v-4',folder:'M3 6h7l2 2h9v11H3z',scissors:'M5 6a2 2 0 1 0 0 4 2 2 0 0 0 0-4zm0 8a2 2 0 1 0 0 4 2 2 0 0 0 0-4zM7 9l12 9M7 15 19 6'};
			const ctx={t,esc:v=>String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),iconMarkup:key=>'<svg class="vs-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="'+(paths[key]||paths.video)+'"/></svg>',setWindowBeforeClose:(id,fn)=>window.fixtureCloseGuard=fn,setWindowMenus:()=>{},clearWindowMenus:()=>{},promptDialog:async()=>false,confirmDialog:async()=>window.fixtureConfirm===true};
			window.VideoStudioApp.render(document.getElementById('studio'),'fixture',ctx);
		</script></body></html>`)
	})
	mux.HandleFunc("/api/desktop/video-studio/projects/p1/media/", func(w http.ResponseWriter, _ *http.Request) {
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
	waitForJSBool(t, page, `()=>document.querySelector('.vs-clip') && document.querySelector('[data-action="export"]')?.textContent.includes('Exportieren')`)
	if !page.MustEval(`()=>{const probe=document.createElement('span');probe.style.color=getComputedStyle(document.querySelector('.vs-app')).getPropertyValue('--vs-muted');document.body.append(probe);const expected=getComputedStyle(probe).color;probe.remove();return getComputedStyle(document.querySelector('.vs-timecode')).color===expected}`).Bool() {
		t.Fatalf("Fruity timecode does not use the readable theme muted color: %s", page.MustEval(`()=>JSON.stringify({timecode:getComputedStyle(document.querySelector('.vs-timecode')).color,muted:getComputedStyle(document.querySelector('.vs-app')).getPropertyValue('--vs-muted')})`).Str())
	}
	page.MustScreenshot(filepath.Join(screenshotDir, "fruity-1280x900.png"))
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
	// Medium and narrow windows: the breakpoint widths reach the columns (a container query cannot style .vs-app
	// itself), and the narrow library rail shows the list and the media bar without sideways scrolling.
	for _, size := range []struct{ width, library, header int }{{1100, 248, 180}, {860, 224, 180}} {
		page.MustSetViewport(size.width, 900, 1, false)
		waitForJSBool(t, page, fmt.Sprintf(`()=>Math.round(document.querySelector('.vs-library').getBoundingClientRect().width)===%d`, size.library))
		got := page.MustEval(`()=>{const q=s=>document.querySelector(s),fits=el=>el.scrollWidth<=el.clientWidth;q('[data-action="media-view"][data-view="list"]').click();
			const result={header:q('.vs-ruler-label').offsetWidth,list:q('[data-assets]').dataset.view==='list'&&fits(q('[data-assets]')),bar:fits(q('[data-media-bar]')),library:fits(q('.vs-library'))};
			q('[data-action="media-view"][data-view="grid"]').click();return JSON.stringify(result)}`).Str()
		if want := fmt.Sprintf(`{"header":%d,"list":true,"bar":true,"library":true}`, size.header); got != want {
			t.Fatalf("library rail at %dpx: got %s, want %s", size.width, got, want)
		}
		page.MustScreenshot(filepath.Join(screenshotDir, fmt.Sprintf("library-rail-%dx900.png", size.width)))
	}
	page.MustSetViewport(390, 844, 1, false)
	page.MustEval(`()=>document.querySelector('.vs-inspector-toggle').click()`)
	if page.MustEval(`()=>document.querySelector('.vs-app').scrollWidth>document.querySelector('.vs-app').clientWidth+1`).Bool() {
		t.Fatal("Video Studio overflows at 390px")
	}
	page.MustScreenshot(filepath.Join(screenshotDir, "mobile-390x844.png"))
	if !page.MustEval(`()=>document.querySelector('.vs-app').classList.contains('vs-show-inspector') && document.querySelector('.vs-inspector').getBoundingClientRect().width>0`).Bool() {
		t.Fatal("narrow inspector did not open as a visible drawer")
	}
	page.MustEval(`()=>{const controls=Array.from(document.querySelector('.vs-inspector').querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled)')).filter(el=>el.getClientRects().length);window.fixtureDrawerFirst=controls[0];window.fixtureDrawerLast=controls.at(-1);controls.at(-1).focus();window.fixtureDrawerFocused=document.activeElement===controls.at(-1);}`)
	page.Keyboard.MustType(input.Tab)
	if !page.MustEval(`()=>document.activeElement===window.fixtureDrawerFirst`).Bool() {
		t.Fatalf("drawer Tab did not wrap to its first control: %s", page.MustEval(`()=>JSON.stringify({active:document.activeElement?.outerHTML.slice(0,160),first:window.fixtureDrawerFirst?.outerHTML.slice(0,160),firstConnected:window.fixtureDrawerFirst?.isConnected,last:window.fixtureDrawerLast?.outerHTML.slice(0,200),lastFocused:window.fixtureDrawerFocused,errors:window.fixtureErrors})`).Str())
	}
	page.MustEval(`()=>document.querySelector('.vs-inspector input').focus()`)
	page.Keyboard.MustType(input.Escape)
	if page.MustEval(`()=>document.querySelector('.vs-app').classList.contains('vs-show-inspector')`).Bool() {
		t.Fatal("Escape in a drawer input did not dismiss the inspector")
	}

	page.MustSetViewport(1280, 900, 1, false)
	page.MustEval(`()=>document.querySelector('[data-action="export"]').click()`)
	if !page.MustEval(`()=>!!document.querySelector('[data-export-form]')`).Bool() {
		t.Fatalf("export action did not open: %s", page.MustEval(`()=>JSON.stringify({button:document.querySelector('[data-action="export"]')?.outerHTML,rect:document.querySelector('.vs-app').getBoundingClientRect().toJSON(),notice:document.querySelector('[data-notice]')?.textContent,readonly:window.VideoStudioApp.instances.get('fixture').readonly,epoch:window.VideoStudioApp.instances.get('fixture').projectEpoch,errors:window.fixtureErrors})`).Str())
	}
	if !page.MustEval(`()=>document.querySelector('.vs-modal').getBoundingClientRect().height>0`).Bool() {
		t.Fatal("export dialog is not visible")
	}
	// A dialog makes the editor behind it inert, so Tab and Space cannot reach toolbar actions.
	if !page.MustEval(`()=>document.querySelector('.vs-toolbar').inert&&document.querySelector('.vs-timeline-panel').inert&&!document.querySelector('[data-modal-host]').inert`).Bool() {
		t.Fatal("editor behind the export dialog is still interactive")
	}
	page.MustEval(`()=>document.querySelector('[data-export-form] button[type="submit"]').click()`)
	if !page.MustEval(`async()=>{await Promise.resolve();return !document.querySelector('.vs-toolbar').inert}`).Bool() {
		t.Fatal("closing the export dialog left the editor inert")
	}
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
	// A cancel that reaches a finished job is answered 409 job_finished; that is not an error.
	if !page.MustEval(`async()=>{const s=window.VideoStudioApp.instances.get('fixture');await window.VideoStudioParts.cancelJob(s,'j1');const notice=document.querySelector('[data-notice]');return window.fixtureCalls.some(([method,path])=>method==='POST'&&path==='/api/desktop/video-studio/jobs/j1/cancel')&&!(notice&&!notice.hidden&&notice.classList.contains('is-error'))}`).Bool() {
		t.Fatal("cancelling an already finished job showed an error")
	}
	page.MustEval(`()=>{window.fixtureConflict=true;window.fixtureLatest.name='Edited in another window';const input=document.querySelector('[data-field="start"]');input.value='45';input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('[data-action="save"]').click()}`)
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

	// Media and projects: the export dialog offers the in/out range, the bin switches views and creates a folder, the
	// project manager duplicates and deletes, and the versions list restores with If-Match. Waits are short polls. The page
	// is navigated right after, so the fixture state starts fresh; the switches, preferences and draft this flow touched are
	// put back anyway.
	page.MustEval(`async()=>{
		const P=window.VideoStudioParts, s=window.VideoStudioApp.instances.get('fixture');
		const until=async(test,message)=>{for(let i=0;i<150;i++){const value=test();if(value)return value;await new Promise(resolve=>setTimeout(resolve,20));}throw new Error(message+': '+JSON.stringify({calls:window.fixtureCalls.slice(-8),notice:document.querySelector('[data-notice]')?.textContent,errors:window.fixtureErrors}));};
		const openManager=async()=>{if(document.querySelector('[data-project-popover]').hidden)document.querySelector('[data-action="project-menu"]').click();document.querySelector('[data-action="all-projects"]').click();await until(()=>document.querySelector('[data-manager] [data-manager-row]')||document.querySelector('[data-manager] .vs-hint'),'the project manager did not open');};
		const sizeCalls=()=>window.fixtureCalls.filter(([method,path])=>method==='GET'&&path==='/api/desktop/video-studio/projects?sizes=1').length;
		const previous={conflict:window.fixtureConflict,confirm:window.fixtureConfirm};
		window.fixtureConflict=false;window.fixtureConfirm=true;
		try{
			// Range export: both points set, so the dialog offers the range; the clip ends at frame 150, inside the points.
			P.mutate(s,'in/out',project=>{project.in_point=30;project.out_point=90;});
			document.querySelector('[data-action="export"]').click();
			const scope=await until(()=>document.querySelector('[data-export-scope]'),'the export dialog did not offer the range');
			if(scope.options.length!==2||!scope.options[1].textContent.includes('–'))throw new Error('the range option does not show its times: '+scope.innerHTML);
			if(P.exportName(s,{result:{range:{start:30,end:90}}})!=='Browser fixture (in-out).mp4'||P.exportName(s,{result:{}})!=='Browser fixture.mp4')throw new Error('the download name does not follow the range');
			scope.value='range';scope.dispatchEvent(new Event('change',{bubbles:true}));
			if(!document.querySelector('[data-export-form] .vs-facts').textContent.includes('(in-out).mp4'))throw new Error('the dialog facts did not follow the range');
			window.fixtureLastJobBody=null;
			document.querySelector('[data-export-form] [type="submit"]').click();
			await until(()=>window.fixtureLastJobBody,'the range export was not requested');
			const range=window.fixtureLastJobBody.range;
			if(window.fixtureLastJobBody.kind!=='render'||!range||range.start!==30||range.end!==90)throw new Error('the render job did not carry the in/out range: '+JSON.stringify(window.fixtureLastJobBody));
			P.mutate(s,'in/out',project=>{delete project.in_point;delete project.out_point;});

			// Media bin: one item reads in the singular, list and grid views, a new folder that opens and then shows as a tile.
			const countText=document.querySelector('[data-asset-count]').textContent;
			if(countText!==window.fixtureLocales[window.fixtureLang]['videoStudio.assetCountOne'])throw new Error('one item does not use the singular label: '+countText);
			document.querySelector('[data-action="media-view"][data-view="list"]').click();
			if(document.querySelector('[data-assets]').dataset.view!=='list'||!document.querySelector('.vs-asset-head'))throw new Error('the list view did not render');
			// The list is an ARIA table: a named table, a header row of column headers that hold the sort buttons (aria-sort
			// only on the sorted column), and item rows of cells.
			const table=document.querySelector('[data-assets]');
			const headers=[...table.querySelectorAll('.vs-asset-head [role="columnheader"]')];
			const sorted=headers.filter(header=>header.hasAttribute('aria-sort'));
			const rows=[...table.querySelectorAll('[data-asset-id]')];
			if(table.getAttribute('role')!=='table'||!table.getAttribute('aria-label')||table.querySelector('.vs-asset-head').getAttribute('role')!=='row')throw new Error('the list is not a named table with a header row: '+table.outerHTML.slice(0,300));
			if(headers.length!==8||headers.slice(0,7).some(header=>!header.querySelector('button[data-media-sort]'))||table.querySelector('button[role="columnheader"],button[aria-sort]'))throw new Error('the column headers do not hold the sort buttons');
			if(sorted.length!==1||sorted[0].getAttribute('aria-sort')!=='ascending')throw new Error('aria-sort is not on the sorted column only: '+sorted.map(header=>header.outerHTML).join(' '));
			if(!rows.length||rows.some(row=>row.getAttribute('role')!=='row'||[...row.children].some(child=>child.getAttribute('role')!=='cell'&&child.getAttribute('aria-hidden')!=='true')||row.querySelectorAll('[role="cell"]').length!==8))throw new Error('an item is not a row of cells: '+rows.map(row=>row.outerHTML.slice(0,200)).join(' '));
			if(table.querySelectorAll(':scope > :not([role="row"])').length)throw new Error('the table holds something that is not a row');
			sorted[0].querySelector('button').focus();sorted[0].querySelector('button').click();
			const descending=document.querySelector('[data-assets] .vs-asset-head [aria-sort]');
			if(!descending||descending.getAttribute('aria-sort')!=='descending'||document.activeElement!==descending.querySelector('button'))throw new Error('sorting again did not announce descending on the focused header');
			descending.querySelector('button').click();
			document.querySelector('[data-action="media-view"][data-view="grid"]').click();
			if(document.querySelector('[data-assets]').dataset.view!=='grid')throw new Error('the grid view did not come back');
			if(document.querySelector('[data-assets]').hasAttribute('role')||document.querySelector('[data-assets] [role="row"]'))throw new Error('the grid view kept the table roles');
			document.querySelector('[data-action="new-folder"]').click();
			const folderName=await until(()=>document.querySelector('[data-folder-name]'),'the folder dialog did not open');
			folderName.value='Trips';
			document.querySelector('[data-folder-form] [type="submit"]').click();
			await until(()=>s.mediaFolder==='Trips'&&document.querySelector('[data-media-bar] [data-folder="Trips"]'),'the folder was not created and opened');
			document.querySelector('[data-media-bar] [data-folder=""]').click();
			if(s.mediaFolder!==''||!document.querySelector('[data-assets] [data-folder="Trips"]'))throw new Error('the new folder is not a tile at the top level');

			// Project manager: it asks for the sizes, and duplicate sends the copy name as a duplicate job.
			await openManager();
			if(!sizeCalls())throw new Error('the project manager did not ask for the sizes');
			document.querySelector('[data-manager-duplicate="p1"]').click();
			const copyName=await until(()=>document.querySelector('[data-duplicate-name]'),'the duplicate dialog did not open');
			copyName.value='Browser copy';
			window.fixtureLastJobBody=null;
			const sizeCallsBefore=sizeCalls();
			document.querySelector('[data-duplicate-form] [type="submit"]').click();
			await until(()=>window.fixtureLastJobBody&&window.fixtureLastJobBody.kind==='duplicate','duplicate was not requested');
			if(window.fixtureLastJobBody.name!=='Browser copy')throw new Error('duplicate sent the wrong name: '+JSON.stringify(window.fixtureLastJobBody));
			await until(()=>sizeCalls()>sizeCallsBefore,'the project list was not reloaded when the copy finished');

			// Versions: restore saves the editor first, then sends the saved ETag and loads the restored project.
			await openManager();
			await until(()=>document.querySelectorAll('[data-version-restore]').length===1,'the versions list did not show the fixture version');
			document.querySelector('[data-version-restore]').click();
			await until(()=>window.fixtureRestoreIfMatch,'restore was not requested');
			if(window.fixtureRestoreIfMatch!=='"v2"')throw new Error('restore did not send the saved ETag: '+window.fixtureRestoreIfMatch);
			if(!((window.fixtureSaves[window.fixtureRestoreSaves-1]||{}).folders||[]).includes('Trips'))throw new Error('the editor did not save its folder before the restore');
			await until(()=>s.etag==='"v1"'&&s.project,'the restored project was not loaded');

			// Delete: the confirm answers yes, the endpoint is called, and the editor shows the empty state.
			await openManager();
			document.querySelector('[data-manager-delete="p1"]').click();
			await until(()=>(window.fixtureDeleted||[]).length,'delete did not call the project endpoint');
			if(!window.fixtureCalls.some(([method,path])=>method==='DELETE'&&path==='/api/desktop/video-studio/projects/p1'))throw new Error('delete did not call the project endpoint');
			await until(()=>!s.project&&!s.projectId,'the deleted project stayed open');
			if(window.fixtureErrors.length)throw new Error('script errors during the media and project flow: '+JSON.stringify(window.fixtureErrors));
		}finally{
			window.fixtureConflict=previous.conflict;
			if(previous.confirm===undefined)delete window.fixtureConfirm;else window.fixtureConfirm=previous.confirm;
			Object.keys(localStorage).filter(key=>key.startsWith('video-studio-draft:')||key==='aurago.videoStudio.mediaView'||key==='aurago.videoStudio.mediaSort').forEach(key=>localStorage.removeItem(key));
		}
	}`)

	// Isolate asynchronous editing from the preceding pointer, export, and conflict flows.
	page.MustEval(`()=>window.VideoStudioApp.dispose('fixture')`)
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>!!document.querySelector('.vs-clip')`)
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

	page.MustEval(`async()=>{
		const wait=async test=>{for(let i=0;i<700;i++){if(test())return;await new Promise(resolve=>setTimeout(resolve,10));}throw new Error('Video Studio editor regression timed out: '+JSON.stringify({condition:String(test),revision:s.artworkRevision,selection:s.selectionRevision,dirty:s.dirty,conflict:s.conflict,clips:s.project.tracks.flatMap(track=>track.clips),assets:s.project.assets.map(asset=>asset.id),jobs:s.jobs,notice:document.querySelector('[data-notice]').textContent,calls:window.fixtureCalls.slice(-12),states:window.fixtureArtworkStates,errors:window.fixtureErrors}))};
		const s=window.VideoStudioApp.instances.get('fixture'), requests=window.fixtureArtworkRequests;
		const assetFor=req=>({id:req.assetId,name:'Title.png',path:'media/'+req.assetId+'.png',kind:'image',duration_frames:18000,width:1280,height:720,has_audio:false});
		const addRemoteAsset=req=>{if(!window.fixtureProject.assets.some(asset=>asset.id===req.assetId))window.fixtureProject.assets.push(assetFor(req));};
		const finish=req=>{const job=window.fixtureArtworkStates[req.id];job.status='succeeded';job.progress=1;job.result={asset:{id:req.assetId}};addRemoteAsset(req);};
		s.preview.seek(150);document.querySelector('[data-action="add-title"]').click();
		await wait(()=>requests.length===1);
		window.fixtureProject=structuredClone(s.project);addRemoteAsset(requests[0]);finish(requests[0]);
		await wait(()=>s.project.tracks.find(track=>track.id==='o1').clips.some(clip=>clip.text&&clip.asset_id===requests[0].assetId));
		const title=s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.text);
		window.fixtureTitleClipId=title.id;s.timelineOptions.onSelect(title.id);s.dirty=false;window.fixtureProject=structuredClone(s.project);
		document.querySelector('[data-action="apply-text"]').click();
		await wait(()=>requests.length===2);
		title.text_style={font_size:90,font_family:'Georgia',alignment:'center',color:'#ffffff',background_color:'#101319',background_opacity:0,bold:false,italic:false};
		document.querySelector('[data-action="apply-text"]').click();
		await wait(()=>requests.length===3);
		window.fixtureProject=structuredClone(s.project);
		window.fixtureProject.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).text_style={color:'#ffffff',font_family:'Georgia',background_color:'#101319',alignment:'center',font_size:90};
		addRemoteAsset(requests[1]);addRemoteAsset(requests[2]);finish(requests[2]);
		await wait(()=>s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).asset_id===requests[2].assetId);
		finish(requests[1]);
		// The job poller replaces s.jobs every few seconds; finishedJobs records the terminal poll reliably.
		await wait(()=>s.finishedJobs&&s.finishedJobs.has(requests[1].id));
		await new Promise(resolve=>setTimeout(resolve,200));
		if(s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).asset_id!==requests[2].assetId)throw new Error('older artwork response replaced the newer Apply result');
		// A recorded style edit schedules its own artwork; an explicit Apply replaces that timer, and Undo invalidates the upload.
		const historyBeforeBold=s.history.length;
		document.querySelector('[data-text-toggle="bold"]').click();
		if(s.history.length!==historyBeforeBold+1||!s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).text_style.bold)throw new Error('bold toggle did not record one undo step');
		document.querySelector('[data-action="apply-text"]').click();
		await wait(()=>requests.length===4);
		document.querySelector('[data-action="undo"]').click();
		if(s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).text_style.bold)throw new Error('undo did not restore the previous title style');
		const undoneClip=s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id),undoAsset=undoneClip.asset_id,undoHistory=s.history.length,undoRedo=s.redo.length;
		finish(requests[3]);
		await wait(()=>!s.artworkJobs.has(title.id));
		if(s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).asset_id!==undoAsset||s.history.length!==undoHistory||s.redo.length!==undoRedo)throw new Error('stale artwork after Undo changed clip or history');
		const beforeRefresh=s.project;s.dirty=false;window.fixtureProject=structuredClone(s.project);
		document.querySelector('[data-action="apply-text"]').click();
		await wait(()=>requests.length===5);addRemoteAsset(requests[4]);finish(requests[4]);
		await wait(()=>s.project!==beforeRefresh&&s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).asset_id===requests[4].assetId);
		const titleClip=()=>s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id);
		const requestsBeforeUndo=requests.length;
		document.querySelector('[data-text-toggle="italic"]').click();
		document.querySelector('[data-action="undo"]').click();
		await new Promise(resolve=>setTimeout(resolve,1300));
		if(requests.length!==requestsBeforeUndo||titleClip().text_style.italic)throw new Error('Undo did not cancel the pending title apply');
		const textarea=document.querySelector('[data-text-field="text"]');
		textarea.value='Renamed title';textarea.dispatchEvent(new Event('input',{bubbles:true}));textarea.dispatchEvent(new Event('change',{bubbles:true}));
		s.timelineOptions.onSelect('');
		await wait(()=>requests.length===requestsBeforeUndo+1);
		// The fixture's PUT replaces the whole stored project, so let the autosave land before the probe finishes.
		await wait(()=>!s.dirty&&!s.saving);
		addRemoteAsset(requests.at(-1));finish(requests.at(-1));
		await wait(()=>titleClip().text==='Renamed title'&&titleClip().asset_id===requests.at(-1).assetId);
		const clipCount=s.project.tracks.reduce((n,track)=>n+track.clips.length,0),button=document.querySelector('[data-action="undo"]'),playing=s.preview.isPlaying();
		const space=new KeyboardEvent('keydown',{key:' ',code:'Space',bubbles:true,cancelable:true});button.dispatchEvent(space);
		const app=document.querySelector('.vs-app'),ctrlS=new KeyboardEvent('keydown',{key:'s',code:'KeyS',ctrlKey:true,bubbles:true,cancelable:true});app.dispatchEvent(ctrlS);
		const handled=new KeyboardEvent('keydown',{key:' ',code:'Space',bubbles:true,cancelable:true});handled.preventDefault();app.dispatchEvent(handled);
		if(space.defaultPrevented||s.preview.isPlaying()!==playing||ctrlS.defaultPrevented||s.project.tracks.reduce((n,track)=>n+track.clips.length,0)!==clipCount||s.preview.isPlaying()!==playing)throw new Error('native button Space, Ctrl+S, or a prehandled key triggered an editor shortcut');
	}`)

	page.MustEval(`()=>{const s=window.VideoStudioApp.instances.get('fixture');window.fixtureUndoCount=s.history.length;document.querySelector('[data-action="undo"]').focus();}`)
	page.Keyboard.MustType(input.Space)
	if !page.MustEval(`()=>{const s=window.VideoStudioApp.instances.get('fixture');return s.history.length===window.fixtureUndoCount-1&&!s.preview.isPlaying();}`).Bool() {
		t.Fatal("native Space did not activate Undo independently of preview playback")
	}
	page.MustEval(`()=>document.querySelector('.vs-app').focus()`)
	page.Keyboard.MustType(input.Space)
	if !page.MustEval(`()=>window.VideoStudioApp.instances.get('fixture').preview.isPlaying()`).Bool() {
		t.Fatal("Space on the editor canvas did not start playback")
	}
	page.Keyboard.MustType(input.Space)
	page.MustEval(`async()=>{
		const wait=async test=>{for(let i=0;i<700;i++){if(test())return;await new Promise(resolve=>setTimeout(resolve,10));}throw new Error('Video Studio generation regression timed out')};
		const s=window.VideoStudioApp.instances.get('fixture');
		s.timelineOptions.onSelect('');
		for(const provider of ['minimax','veo']){
			s.status.generation.provider=provider;window.fixtureLastJobBody=null;document.querySelector('[data-action="open-ai"]').click();
			const ratio=document.querySelector('[data-ai-ratio]');if(!!ratio!==(provider==='veo'))throw new Error('aspect-ratio control does not match provider capabilities');
			if(ratio)ratio.value='9:16';document.querySelector('[data-ai-prompt]').value='Fixture request';document.querySelector('[data-ai-form]').dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
			await wait(()=>window.fixtureLastJobBody!==null);
			const payload=window.fixtureLastJobBody;if(payload.kind!=='generate'||(provider==='minimax'?'aspect_ratio' in payload:payload.aspect_ratio!=='9:16'))throw new Error('generation payload has incorrect aspect ratio');
		}
		window.fixtureJobs=[
			{id:'quota',kind:'generate',status:'failed',error:'project_size_limit'},
			{id:'asset',kind:'generate',status:'failed',error:'asset_size_limit'},
			{id:'import',kind:'generate',status:'failed',error:'generation_import_failed'},
			{id:'uncertain',kind:'generate',status:'failed',error:'generation_failed',external_status_unknown:true},
			{id:'interrupted',kind:'generate',status:'interrupted',external_status_unknown:true},
			{id:'running',kind:'generate',status:'running',external_status_unknown:true}
		];
		await wait(()=>document.querySelectorAll('.vs-job-error').length===5);
		const messages=Array.from(document.querySelectorAll('.vs-job-error'),el=>el.textContent),locale=window.fixtureLocales.en;
		for(const key of ['projectStorageFull','fileTooLarge','generatedImportFailed','generationStatusUnknown'])if(!messages.includes(locale['videoStudio.'+key]))throw new Error('missing localized job error: '+key);
		if(document.querySelector('.vs-job-running .vs-job-error'))throw new Error('running provider request was reported as uncertain');
	}`)

	page.MustEval(`async()=>{
		const wait=async test=>{for(let i=0;i<500;i++){if(test())return;await new Promise(resolve=>setTimeout(resolve,10));}throw new Error('project settings timed out: '+JSON.stringify({fps:s.project.fps,errors:window.fixtureErrors}))};
		const s=window.VideoStudioApp.instances.get('fixture');
		window.fixtureConfirm=true;
		document.querySelector('[data-action="project-menu"]').click();
		document.querySelector('[data-action="project-settings"]').click();
		const form=document.querySelector('[data-settings-form]');
		if(!form)throw new Error('project settings did not open');
		form.querySelector('[data-settings-rate]').value='25';
		form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
		await wait(()=>s.project.fps===25);
		const clip=s.project.tracks.find(track=>track.id==='o1').clips.find(item=>item.id==='c1');
		if(clip.start!==0||clip.duration!==125)throw new Error('the 25 fps rescale did not round clip edges: '+JSON.stringify(clip));
		document.querySelector('[data-action="undo"]').click();
		if(s.project.fps!==30)throw new Error('undo did not restore 30 fps');
		const quality=document.querySelector('[data-preview-quality]');quality.value='quarter';quality.dispatchEvent(new Event('change',{bubbles:true}));
		await wait(()=>document.querySelector('[data-preview]').width===320);
		quality.value='auto';quality.dispatchEvent(new Event('change',{bubbles:true}));
		window.fixtureConfirm=false;
	}`)

	page.MustEval(`async()=>{
		const s=window.VideoStudioApp.instances.get('fixture'),app=document.querySelector('.vs-app');
		const key=(k,o={},target=app)=>{const e=new KeyboardEvent('keydown',Object.assign({key:k,bubbles:true,cancelable:true},o));target.dispatchEvent(e);return e;};
		const all=()=>s.project.tracks.find(t=>t.id==='o1').clips.slice().sort((a,b)=>a.start-b.start);
		const others=all().filter(c=>c.id!=='c1').map(c=>c.id);
		const o1=()=>all().filter(c=>!others.includes(c.id));
		app.focus();
		s.timelineOptions.onSelect('c1');
		if(!key('c',{ctrlKey:true}).defaultPrevented)throw new Error('Ctrl+C was not handled');
		s.preview.seek(300);
		key('v',{ctrlKey:true});
		if(o1().length!==2||o1()[1].start!==300)throw new Error('Ctrl+V did not paste at the playhead: '+JSON.stringify(o1()));
		key('b');
		if(s.tool!=='blade'||document.querySelector('[data-timeline]').dataset.tool!=='blade')throw new Error('B did not pick the blade tool');
		const pasted=document.querySelector('[data-clip-id="'+o1()[1].id+'"]'),r=pasted.getBoundingClientRect();
		pasted.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,clientX:r.left+r.width/2,clientY:r.top+r.height/2}));
		if(o1().length!==3)throw new Error('a blade click did not cut the clip: '+JSON.stringify(o1()));
		key('v');
		if(s.tool!=='select')throw new Error('V did not return to the selection tool');
		s.timelineOptions.onSelect(o1()[1].id);
		const lastStart=o1()[2].start,removed=o1()[1].duration;
		key('Delete',{shiftKey:true});
		if(o1().length!==2||o1()[1].start!==lastStart-removed)throw new Error('Shift+Del did not ripple delete: '+JSON.stringify(o1()));
		const clipEl=document.querySelector('[data-clip-id="c1"]'),cr=clipEl.getBoundingClientRect();
		clipEl.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:cr.left+10,clientY:cr.top+10}));
		const menu=document.querySelector('.vs-context-menu[role="menu"]');
		if(!menu||!menu.contains(document.activeElement)||!menu.querySelector('[role="menuitem"]'))throw new Error('the clip menu did not open with focus');
		key('Escape',{},document.activeElement);
		if(document.querySelector('.vs-context-menu'))throw new Error('Escape did not close the clip menu');
		key('z',{ctrlKey:true});key('z',{ctrlKey:true});key('z',{ctrlKey:true});
		if(o1().length!==1||o1()[0].id!=='c1'||all().length!==others.length+1)throw new Error('undo did not restore the clips: '+JSON.stringify(all()));
	}`)

	page.MustEval(`async()=>{
		const s=window.VideoStudioApp.instances.get('fixture'),app=document.querySelector('.vs-app');
		const wait=async test=>{for(let i=0;i<500;i++){if(test())return;await new Promise(resolve=>setTimeout(resolve,10));}throw new Error('clip flow timed out')};
		const key=(k,o={},target=app)=>{const e=new KeyboardEvent('keydown',Object.assign({key:k,bubbles:true,cancelable:true},o));target.dispatchEvent(e);return e;};
		const clipOf=id=>{for(const t of s.project.tracks){const c=t.clips.find(x=>x.id===id);if(c)return c;}return null;};
		// Track ids before this flow: the cleanup at the end removes the tracks and media the flow added.
		const before=new Set(s.project.tracks.map(t=>t.id));
		const video={id:'vid1',name:'Clip.mp4',path:'media/vid1_clip.mp4',kind:'video',duration_frames:900,width:1280,height:720,has_audio:true,media_url:'/fixture.mp4'};
		s.project.assets.push(video);
		s.project.tracks.find(t=>t.id==='v1').clips.push({id:'vc',asset_id:'vid1',start:0,offset:0,duration:150,x:0,y:0,width:1,height:1,rotation:0,opacity:1,volume:1,fade_in:0,fade_out:0,fit:'contain',text:'',text_style:null,transition:null});
		s.dirty=true;
		s.timelineOptions.onSelect('vc');
		app.focus();
		document.querySelector('[data-action="clip-detach"]').click();
		await wait(()=>s.project.tracks.some(t=>t.kind==='audio'&&t.clips.some(c=>c.asset_id==='vid1')));
		const detached=clipOf('vc');
		if(!detached||!detached.audio_detached||!detached.link)throw new Error('detach did not link the clips: '+JSON.stringify(detached));
		s.timelineOptions.onSelect('vc');
		const speed=document.querySelector('[data-field="speed"]');
		speed.value='2';speed.dispatchEvent(new Event('change',{bubbles:true}));
		await wait(()=>clipOf('vc').speed===2);
		const partner=s.project.tracks.flatMap(t=>t.clips).find(c=>c.link===detached.link&&c.id!=='vc');
		if(!partner||partner.speed!==2)throw new Error('the linked partner did not take the speed');
		// A click selects the whole link group; Alt-click selects the clip alone.
		const selectedOnTimeline=()=>Array.from(document.querySelectorAll('.vs-clip.is-selected'),el=>el.dataset.clipId).sort();
		document.querySelector('[data-clip-id="vc"]').click();
		if(JSON.stringify(selectedOnTimeline())!==JSON.stringify(['vc',partner.id].sort()))throw new Error('a click did not select the link group: '+selectedOnTimeline().join(','));
		document.querySelector('[data-clip-id="vc"]').dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,altKey:true}));
		if(JSON.stringify(selectedOnTimeline())!==JSON.stringify(['vc']))throw new Error('Alt-click did not select the clip alone: '+selectedOnTimeline().join(','));
		// Commands act on the selection as it is: after the Alt-click only vc is selected, so only vc is disabled.
		app.focus();key('E',{shiftKey:true});
		if(!clipOf('vc').disabled||clipOf(partner.id).disabled)throw new Error('Shift+E did not disable only the Alt-selected clip');
		key('E',{shiftKey:true});
		if(clipOf('vc').disabled)throw new Error('Shift+E did not enable the clip again');
		// With the whole link group selected, Shift+E disables and enables the pair.
		document.querySelector('[data-clip-id="vc"]').click();
		key('E',{shiftKey:true});
		if(!clipOf('vc').disabled||!clipOf(partner.id).disabled)throw new Error('Shift+E did not disable the linked pair');
		key('E',{shiftKey:true});
		if(clipOf('vc').disabled||clipOf(partner.id).disabled)throw new Error('Shift+E did not enable the linked pair again');
		s.timelineOptions.onSelect('vc');
		key('l',{ctrlKey:true});
		if(clipOf('vc').link)throw new Error('Ctrl+L did not unlink the clip');
		const swatch=document.querySelector('[data-action="clip-label"][data-value="teal"]');
		swatch.focus();swatch.click();
		if(clipOf('vc').label!=='teal')throw new Error('the label swatch did not set teal');
		await new Promise(resolve=>setTimeout(resolve,100));
		if(!document.activeElement||document.activeElement.dataset.value!=='teal')throw new Error('focus left the clicked label swatch: '+(document.activeElement&&document.activeElement.outerHTML.slice(0,90)));
		// Remove what the flow added (the clip, its partner, the detached audio track and the asset) so the next flow
		// starts from the same empty video track (1d expects v1 empty). History keeps whole-project snapshots, so an
		// undo of an earlier step still restores a valid project.
		s.timelineOptions.onSelect('');
		s.project.tracks=s.project.tracks.filter(t=>before.has(t.id)).map(t=>Object.assign(t,{clips:t.clips.filter(c=>c.asset_id!=='vid1')}));
		s.project.assets=s.project.assets.filter(a=>a.id!=='vid1');
		s.dirty=true;
	}`)

	// 1c: navigation keys (J/K/L, I/O, Alt+X, M, Shift+M, arrows, Ctrl+G) and the track rename. The flow restores the
	// markers, in/out points, track name and playhead it found, so the next flow starts from the same project.
	page.MustEval(`async()=>{
		const s=window.VideoStudioApp.instances.get('fixture'),app=document.querySelector('.vs-app');
		const key=(k,o={},target=app)=>{const e=new KeyboardEvent('keydown',Object.assign({key:k,bubbles:true,cancelable:true},o));target.dispatchEvent(e);return e;};
		const up=k=>window.dispatchEvent(new KeyboardEvent('keyup',{key:k,bubbles:true}));
		const base=JSON.stringify({markers:s.project.markers,in_point:s.project.in_point,out_point:s.project.out_point}),startFrame=s.frame;
		app.focus();
		s.preview.pause(); s.preview.seek(60);
		key('l'); if(!s.preview.isPlaying())throw new Error('L did not start playback');
		key('l'); if(s.shuttle.rate()!==2)throw new Error('a second L did not double the rate: '+s.shuttle.rate());
		key('k'); up('k'); if(s.preview.isPlaying())throw new Error('K did not stop playback');
		if(s.preview.rate()!==1)throw new Error('stopping did not return the preview to 1x');
		s.preview.seek(120); key('i');
		if(s.project.in_point!==120)throw new Error('I did not set the in point: '+JSON.stringify(s.project.in_point));
		{const steps=s.history.length;key('i');if(s.history.length!==steps)throw new Error('I at the frame the in point already has added an undo step');}
		s.preview.seek(240); key('o');
		if(s.project.out_point!==240)throw new Error('O did not set the out point');
		key('x',{altKey:true});
		if('in_point' in s.project||'out_point' in s.project)throw new Error('Alt+X did not clear both points');
		s.preview.seek(150); key('m');
		if(!(s.project.markers||[]).some(m=>m.frame===150))throw new Error('M did not add a marker at the playhead');
		key('m');
		if(!document.querySelector('.vs-marker-pop'))throw new Error('M on a marker did not open its popover');
		key('Escape',{},document.activeElement);
		if(document.querySelector('.vs-marker-pop'))throw new Error('Escape did not close the marker popover');
		s.preview.seek(0); key('M',{shiftKey:true});
		if(s.frame!==150)throw new Error('Shift+M did not step to the next marker: '+s.frame);
		const before=s.frame; key('ArrowDown');
		if(s.frame<before)throw new Error('the down arrow moved the playhead backwards');
		key('g',{ctrlKey:true});
		if(!document.querySelector('[data-goto-input]'))throw new Error('Ctrl+G did not open go to timecode');
		key('Escape',{},document.activeElement);
		const named=document.querySelector('[data-track-name]');
		if(!named)throw new Error('no track name to rename');
		const trackId=named.dataset.trackName,originalName=s.project.tracks.find(t=>t.id===trackId).name;
		named.dispatchEvent(new MouseEvent('dblclick',{bubbles:true,cancelable:true}));
		const input=document.querySelector('[data-track-rename]');
		if(!input)throw new Error('double-click did not start renaming');
		input.value='Lead';
		input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}));
		if(s.project.tracks.find(t=>t.id===trackId).name!=='Lead')throw new Error('Enter did not rename the track');
		// A rename that is still open when a press on a clip begins is committed first (the gesture would drop it).
		document.querySelector('[data-track-name="'+trackId+'"]').dispatchEvent(new MouseEvent('dblclick',{bubbles:true,cancelable:true}));
		const second=document.querySelector('[data-track-rename]');
		if(!second)throw new Error('double-click did not start the second rename');
		second.value='Kept';
		const pressed=document.querySelector('[data-clip-id]'),box=pressed.getBoundingClientRect();
		pressed.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,cancelable:true,button:0,pointerId:7,clientX:box.left+box.width/2,clientY:box.top+box.height/2}));
		window.dispatchEvent(new PointerEvent('pointerup',{bubbles:true,pointerId:7}));
		if(s.project.tracks.find(t=>t.id===trackId).name!=='Kept')throw new Error('pressing a clip dropped the open track rename');
		if(document.querySelector('[data-track-rename]'))throw new Error('the rename field stayed open after the press');
		// The time button is named by its content: the time first, then what the button does.
		const timeButton=document.querySelector('[data-action="goto-time"]');
		if(timeButton.hasAttribute('aria-label')||!timeButton.querySelector('[data-time]')||!timeButton.querySelector('.vs-sr-only'))throw new Error('the time button lost its accessible text');
		// Put back what the flow changed.
		s.project.tracks.find(t=>t.id===trackId).name=originalName;
		const restore=JSON.parse(base||'{}');
		for(const field of ['markers','in_point','out_point']){if(restore[field]===undefined)delete s.project[field];else s.project[field]=restore[field];}
		s.dirty=true;
		s.preview.seek(startFrame);
		window.VideoStudioParts.renderTimeline(s);
		if(JSON.stringify({markers:s.project.markers,in_point:s.project.in_point,out_point:s.project.out_point})!==base)throw new Error('the navigation flow did not restore the markers and in/out points');
	}`)

	// Source monitor insert: timeline in/out, double-click the still in the library, `,` inserts at in with out - in.
	// The still lands on the video track because 1b lets video tracks accept stills.
	page.MustEval(`async()=>{
		const s=window.VideoStudioApp.instances.get('fixture'),app=document.querySelector('.vs-app');
		const key=(k,o={},target=app)=>{const e=new KeyboardEvent('keydown',Object.assign({key:k,bubbles:true,cancelable:true},o));target.dispatchEvent(e);return e;};
		const v1=()=>s.project.tracks.find(t=>t.id==='v1').clips.slice().sort((a,b)=>a.start-b.start);
		app.focus();
		s.preview.seek(30);key('i');s.preview.seek(90);key('o');
		if(s.project.in_point!==30||s.project.out_point!==90)throw new Error('timeline in and out were not set: '+JSON.stringify({in:s.project.in_point,out:s.project.out_point}));
		document.querySelector('[data-asset-id="art1"]').dispatchEvent(new MouseEvent('dblclick',{bubbles:true,cancelable:true}));
		if(s.stageMode!=='source'||s.sourceAssetId!=='art1')throw new Error('double-click did not open the source monitor');
		if(document.querySelector('[data-source]').hidden)throw new Error('the source monitor section stayed hidden');
		// The source stops at its end (no wrap): from the last frame it stops at once and stays there. The still has 150 frames.
		s.sourcePreview.seek(149);s.sourcePreview.play();
		await new Promise(resolve=>setTimeout(resolve,400));
		if(s.sourcePreview.isPlaying()||s.sourceFrame<149)throw new Error('the source did not stop at its end: '+JSON.stringify({playing:s.sourcePreview.isPlaying(),frame:s.sourceFrame}));
		// Escape in the library search field edits the text and keeps the source monitor open.
		const search=document.querySelector('[data-search]');search.focus();key('Escape',{},search);
		if(s.stageMode!=='source')throw new Error('Escape in the library search field left the source monitor');
		app.focus();
		key(',');
		if(v1().length!==1||v1()[0].start!==30||v1()[0].duration!==60)throw new Error('insert with timeline in and out did not place 60 frames at in: '+JSON.stringify(v1()));
		if(s.frame!==90)throw new Error('the playhead did not move to the end of the edit: '+s.frame);
		key('Escape');
		if(s.stageMode!=='timeline')throw new Error('Escape did not return to the timeline');
		key('z',{ctrlKey:true});
		if(v1().length!==0)throw new Error('undo did not remove the source insert: '+JSON.stringify(v1()));
	}`)

	// Picture and sound handles: a sound handle is refused on a video lane; a picture handle places the still's range.
	page.MustEval(`()=>{
		const s=window.VideoStudioApp.instances.get('fixture'),app=document.querySelector('.vs-app');
		const key=(k,o={},target=app)=>{const e=new KeyboardEvent('keydown',Object.assign({key:k,bubbles:true,cancelable:true},o));target.dispatchEvent(e);return e;};
		const lane=document.querySelector('.vs-track-lane[data-track-id="v2"]'),r=lane.getBoundingClientRect();
		const drop=part=>{const dt=new DataTransfer();dt.setData('application/x-video-studio-asset','art1');dt.setData('application/x-video-studio-part',part);lane.dispatchEvent(new DragEvent('drop',{dataTransfer:dt,clientX:r.left+20,clientY:r.top+10,bubbles:true,cancelable:true}));};
		const v2=()=>s.project.tracks.find(t=>t.id==='v2').clips;
		drop('sound');
		if(v2().length!==0)throw new Error('a sound drop of a still was not refused: '+JSON.stringify(v2()));
		drop('picture');
		if(v2().length!==1||v2()[0].duration!==150||v2()[0].asset_id!=='art1')throw new Error('a picture drop did not place the still on the lane: '+JSON.stringify(v2()));
		key('z',{ctrlKey:true});
		if(v2().length!==0)throw new Error('undo did not remove the dropped still');
	}`)

	// Source position slider: after a scrub with the mouse the editor gets the keys back (I marks in, Space plays), and a
	// slider that has keyboard focus passes every key but its own (arrows, Home, End, Page Up and Down) to the source monitor.
	scrub := page.MustEval(`()=>{
		const s=window.VideoStudioApp.instances.get('fixture');
		window.VideoStudioParts.setStageMode(s,'source');
		s.sourcePreview.pause();s.sourcePreview.seek(0);
		const bar=document.querySelector('[data-source-scrub]');bar.scrollIntoView({block:'nearest'});
		const r=bar.getBoundingClientRect();
		return {x:r.left+r.width*0.3,y:r.top+r.height/2};
	}`)
	page.Mouse.MustMoveTo(scrub.Get("x").Num(), scrub.Get("y").Num()).MustDown(proto.InputMouseButtonLeft).MustMoveTo(scrub.Get("x").Num()+20, scrub.Get("y").Num()).MustUp(proto.InputMouseButtonLeft)
	page.MustEval(`()=>{
		const s=window.VideoStudioApp.instances.get('fixture');
		if(s.sourceFrame<10)throw new Error('the mouse scrub did not move the source: '+s.sourceFrame);
		if(document.activeElement!==document.querySelector('.vs-app'))throw new Error('the keys did not go back to the editor after a mouse scrub: '+document.activeElement.className);
	}`)
	page.Keyboard.MustType(input.KeyI)
	page.MustEval(`()=>{
		const s=window.VideoStudioApp.instances.get('fixture');
		if(s.sourceIn!==Math.round(s.sourceFrame))throw new Error('I after a mouse scrub did not mark in: '+JSON.stringify({in:s.sourceIn,frame:s.sourceFrame}));
	}`)
	page.Keyboard.MustType(input.Space)
	if !page.MustEval(`()=>window.VideoStudioApp.instances.get('fixture').sourcePreview.isPlaying()`).Bool() {
		t.Fatal("Space after a mouse scrub did not play the source")
	}
	page.Keyboard.MustType(input.Space)
	page.MustEval(`()=>{
		const s=window.VideoStudioApp.instances.get('fixture'),app=document.querySelector('.vs-app');
		const key=(k,o={},target=app)=>{const e=new KeyboardEvent('keydown',Object.assign({key:k,bubbles:true,cancelable:true},o));target.dispatchEvent(e);return e;};
		const v1=()=>s.project.tracks.find(t=>t.id==='v1').clips;
		if(s.sourcePreview.isPlaying())throw new Error('a second Space did not pause the source');
		const bar=document.querySelector('[data-source-scrub]');
		s.sourcePreview.seek(20);
		// Keyboard use of the slider keeps its focus: a change without a pointer press does not move it away.
		bar.focus();bar.dispatchEvent(new Event('change',{bubbles:true}));
		if(document.activeElement!==bar)throw new Error('a keyboard change on the slider took its focus');
		for(const k of ['ArrowLeft','ArrowRight','ArrowUp','ArrowDown','Home','End','PageUp','PageDown']){
			if(key(k,{},bar).defaultPrevented)throw new Error('the slider lost its own key '+k);
		}
		if(s.sourceFrame!==20)throw new Error('a slider key moved the source as well: '+s.sourceFrame);
		key('i',{},bar);
		if(s.sourceIn!==20)throw new Error('I on the focused slider did not mark in: '+s.sourceIn);
		key(' ',{code:'Space'},bar);
		if(!s.sourcePreview.isPlaying())throw new Error('Space on the focused slider did not play the source');
		key(' ',{code:'Space'},bar);
		if(s.sourcePreview.isPlaying())throw new Error('a second Space on the focused slider did not pause the source');
		const io={in:s.project.in_point,out:s.project.out_point};
		key(',',{},bar);
		if(v1().length!==1||v1()[0].start!==io.in||v1()[0].duration!==io.out-io.in)throw new Error('the comma on the focused slider did not insert: '+JSON.stringify(v1()));
		key('z',{ctrlKey:true});
		if(v1().length!==0)throw new Error('undo did not remove the insert made from the slider');
		key('Escape',{},bar);
		if(s.stageMode!=='timeline')throw new Error('Escape on the focused slider did not return to the timeline');
	}`)

	// Picture only / Sound only are buttons too: Enter, Space or a click places that part at the edit point (overwrite),
	// Ctrl+Enter inserts, and the playhead moves to the end of the edit. A still has no sound, so its sound handle is off.
	page.MustEval(`()=>{
		const s=window.VideoStudioApp.instances.get('fixture');
		window.VideoStudioParts.setStageMode(s,'source');
		const handle=document.querySelector('[data-source-drag="picture"]');
		if(!/Enter/.test(handle.title))throw new Error('the picture handle does not hint at the keyboard: '+handle.title);
		if(!document.querySelector('[data-source-drag="sound"]').disabled)throw new Error('the sound handle of a still is enabled');
		s.preview.seek(0);handle.focus();
	}`)
	page.Keyboard.MustType(input.Enter)
	page.MustEval(`()=>{
		const s=window.VideoStudioApp.instances.get('fixture'),app=document.querySelector('.vs-app');
		const key=(k,o={},target=app)=>{const e=new KeyboardEvent('keydown',Object.assign({key:k,bubbles:true,cancelable:true},o));target.dispatchEvent(e);return e;};
		const v1=()=>s.project.tracks.find(t=>t.id==='v1').clips.slice().sort((a,b)=>a.start-b.start);
		const io={in:s.project.in_point,out:s.project.out_point},handle=document.querySelector('[data-source-drag="picture"]');
		if(v1().length!==1||v1()[0].asset_id!=='art1'||v1()[0].start!==io.in||v1()[0].duration!==io.out-io.in)throw new Error('Enter on the picture handle did not place the picture at in: '+JSON.stringify(v1()));
		if(s.frame!==io.out)throw new Error('the playhead did not move to the end of the picture edit: '+s.frame);
		if(document.activeElement!==handle)throw new Error('the picture handle lost its focus');
		// Ctrl+Enter inserts: the clip that is already there is pushed right instead of being overwritten.
		if(!key('Enter',{ctrlKey:true},handle).defaultPrevented)throw new Error('Ctrl+Enter on the picture handle was not taken');
		if(v1().length!==2||v1()[0].start!==io.in||v1()[1].start!==io.out)throw new Error('Ctrl+Enter on the picture handle did not insert: '+JSON.stringify(v1().map(c=>[c.start,c.duration])));
		key('z',{ctrlKey:true});key('z',{ctrlKey:true});
		if(v1().length!==0)throw new Error('undo did not remove the picture edits: '+JSON.stringify(v1()));
		// A click without a drag does the same as Enter; a drag (dragstart) does not count as a click.
		s.preview.seek(0);
		handle.dispatchEvent(new DragEvent('dragstart',{dataTransfer:new DataTransfer(),bubbles:true,cancelable:true}));
		handle.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,detail:1}));
		if(v1().length!==0)throw new Error('a click that follows a drag placed the picture');
		handle.dispatchEvent(new DragEvent('dragend',{bubbles:true}));
		return new Promise(resolve=>setTimeout(resolve,20)).then(()=>{
			handle.click();
			if(v1().length!==1||v1()[0].start!==io.in)throw new Error('a click on the picture handle did not place the picture: '+JSON.stringify(v1()));
			key('z',{ctrlKey:true});
			if(v1().length!==0)throw new Error('undo did not remove the clicked picture');
			window.VideoStudioParts.setStageMode(s,'timeline');
		});
	}`)

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

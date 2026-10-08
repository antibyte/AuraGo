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
		_, _ = fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-video-studio.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}.desktop-body{font-family:system-ui,sans-serif;color:var(--vd-text,#edf0f5);background:var(--vd-theme-app-bg);}.vd-window-content{height:100%;width:100%;display:flex}#studio{flex:1;min-width:0;min-height:0}.vs-app{flex:1}.desktop-body[data-theme="fruity"]{--vd-text:#273144}</style></head><body class="desktop-body" data-theme="standard" data-fruity-mode="light"><main class="vd-window-content"><div id="studio"></div></main><script src="/js/desktop/apps/video-studio-icons.js"></script><script src="/js/desktop/apps/video-studio-media.js"></script><script src="/js/desktop/apps/video-studio-preview.js"></script><script src="/js/desktop/apps/video-studio-timeline.js"></script><script src="/js/desktop/apps/video-studio-inspector.js"></script><script src="/js/desktop/apps/video-studio.js"></script><script>
			window.fixtureErrors=[];window.fixtureCalls=[];window.addEventListener('error',e=>fixtureErrors.push(String(e.message)));
			window.fixtureLocales=`+mustJSON(t, locales)+`;const fixtureParams=new URLSearchParams(location.search);window.fixtureLang=fixtureParams.get('lang')||'en';document.body.dataset.theme=fixtureParams.get('theme')||'standard';document.documentElement.lang=window.fixtureLang;
			window.fixtureProject=`+mustJSON(t, project)+`;window.fixtureLatest=structuredClone(window.fixtureProject);window.fixtureReadonly=fixtureParams.has('readonly');window.fixtureConflict=false;window.fixtureConflictUsed=false;window.fixtureJobs=[];window.fixtureSaves=[];window.fixtureArtworkRequests=[];window.fixtureArtworkStates={};window.fixtureLastJobBody=null;
			const originalFetch=window.fetch;const json=(data,status=200,headers={})=>new Response(JSON.stringify(data),{status,headers:Object.assign({'Content-Type':'application/json'},headers)});
			window.fetch=async(url,options={})=>{const path=String(url),method=String(options.method||'GET').toUpperCase();window.fixtureCalls.push([method,path]);
			if(path==='/api/desktop/video-studio/status')return json({enabled:true,desktop_enabled:true,ffmpeg_ready:true,read_only:window.fixtureReadonly,limits:{max_duration_frames:18000,canvas_sizes:[{width:1280,height:720},{width:720,height:1280},{width:720,height:720}]},generation:{enabled:true,configured:true,provider:'Fixture provider',model:'fixture-model',durations_seconds:[5,10],image_modes:[]}});
			if(path==='/api/desktop/video-studio/projects'&&method==='GET')return json({projects:[{id:'p1',project:{name:'Browser fixture'},desktop_path:'Documents/Video Studio/p1/project.json'}]});
			if(path==='/api/desktop/video-studio/projects/p1'&&method==='GET'){const p=window.fixtureConflict?window.fixtureLatest:window.fixtureProject;return json({id:'p1',project:p,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:window.fixtureConflict?'"vRemote"':'"v1"'});}
			if(path==='/api/desktop/video-studio/projects/p1'&&method==='PUT'){window.fixtureSaves.push(JSON.parse(options.body));window.fixtureLastIfMatch=new Headers(options.headers).get('If-Match');if(window.fixtureConflict&&!window.fixtureConflictUsed){window.fixtureConflictUsed=true;return json({error:'file_conflict',code:'file_conflict' },412);}window.fixtureProject=JSON.parse(options.body);return json({project:window.fixtureProject,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v2"'});}
			if(path==='/api/desktop/video-studio/projects/p1/media'&&method==='POST'){const id='art-job-'+(window.fixtureArtworkRequests.length+1),assetId='art-asset-'+(window.fixtureArtworkRequests.length+1),job={id,project_id:'p1',kind:'probe',status:'queued',progress:0};window.fixtureArtworkStates[id]=job;window.fixtureArtworkRequests.push({id,assetId});return json({job,asset:{id:assetId,name:'Title.png',path:'media/'+assetId+'.png',kind:'image',duration_frames:18000,width:1280,height:720,has_audio:false}},202);}if(path.startsWith('/api/desktop/video-studio/jobs/art-job-'))return json({job:window.fixtureArtworkStates[path.split('/').pop()]});if(path==='/api/desktop/video-studio/jobs?project_id=p1')return json({jobs:window.fixtureJobs});
			if(path==='/api/desktop/video-studio/projects/p1/jobs'&&method==='POST'){window.fixtureLastRenderIfMatch=new Headers(options.headers).get('If-Match');window.fixtureLastJobBody=JSON.parse(options.body);const job={id:'j1',project_id:'p1',kind:'render',status:'queued',progress:0};window.fixtureJobs=[job];return json({job},202);}
			if(path==='/api/desktop/video-studio/jobs/j1')return json({job:{id:'j1',project_id:'p1',kind:'render',status:'succeeded',progress:1,artifact:{name:'Browser fixture.mp4',download_url:'/fixture.mp4',size:1234}}});
			return originalFetch(url,options);};
			const t=key=>window.fixtureLocales[window.fixtureLang]?.[key]||key;
			const paths={save:'M5 3h12l4 4v14H3V3h2zm2 2v5h10V5H7zm1 9v5h8v-5H8z',video:'M3 5h18v14H3zM10 9v6l5-3z',upload:'M12 16V4m-5 5 5-5 5 5M4 16v4h16v-4',folder:'M3 6h7l2 2h9v11H3z',scissors:'M5 6a2 2 0 1 0 0 4 2 2 0 0 0 0-4zm0 8a2 2 0 1 0 0 4 2 2 0 0 0 0-4zM7 9l12 9M7 15 19 6'};
			const ctx={t,esc:v=>String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),iconMarkup:key=>'<svg class="vs-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="'+(paths[key]||paths.video)+'"/></svg>',setWindowBeforeClose:(id,fn)=>window.fixtureCloseGuard=fn,setWindowMenus:()=>{},clearWindowMenus:()=>{},promptDialog:async()=>false,confirmDialog:async()=>false};
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
		const wait=async test=>{for(let i=0;i<700;i++){if(test())return;await new Promise(resolve=>setTimeout(resolve,10));}throw new Error('Video Studio editor regression timed out: '+JSON.stringify({condition:String(test),revision:s.artworkRevision,selection:s.selectionRevision,dirty:s.dirty,conflict:s.conflict,clips:s.project.tracks.flatMap(track=>track.clips),assets:s.project.assets.map(asset=>asset.id),jobs:s.jobs,notice:document.querySelector('[data-notice]').textContent}))};
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
		await wait(()=>s.jobs.some(job=>job.id===requests[1].id&&job.status==='succeeded'));
		if(s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).asset_id!==requests[2].assetId)throw new Error('older artwork response replaced the newer Apply result');
		s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).text_style={font_size:100,font_family:'Impact',alignment:'center',color:'#ffffff',background_color:'#101319',background_opacity:0,bold:false,italic:false};
		document.querySelector('[data-action="apply-text"]').click();
		await wait(()=>requests.length===4);
		document.querySelector('[data-action="undo"]').click();
		const undoneClip=s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id),undoAsset=undoneClip.asset_id,undoHistory=s.history.length,undoRedo=s.redo.length;
		finish(requests[3]);
		await wait(()=>s.jobs.some(job=>job.id===requests[3].id&&job.status==='succeeded'));
		if(s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).asset_id!==undoAsset||s.history.length!==undoHistory||s.redo.length!==undoRedo)throw new Error('stale artwork after Undo changed clip or history');
		const beforeRefresh=s.project;s.dirty=false;window.fixtureProject=structuredClone(s.project);
		document.querySelector('[data-action="apply-text"]').click();
		await wait(()=>requests.length===5);addRemoteAsset(requests[4]);finish(requests[4]);
		await wait(()=>s.project!==beforeRefresh&&s.project.tracks.find(track=>track.id==='o1').clips.find(clip=>clip.id===title.id).asset_id===requests[4].assetId);
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

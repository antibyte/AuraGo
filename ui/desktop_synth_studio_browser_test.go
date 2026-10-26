package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopSynthStudioBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/desktop-app-synth-studio.css"><style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	scripts := []string{"/synth-shell.js", "/js/desktop/apps/writer-session.js", "/js/vendor/synth-studio/gm-data.js", "/js/desktop/apps/synth-studio-presets.js", "/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js", "/js/desktop/apps/synth-studio-model.js", "/js/desktop/apps/synth-studio-music.js", "/js/desktop/apps/synth-studio-voices.js", "/js/desktop/apps/synth-studio-audio.js", "/js/desktop/apps/synth-studio-midi.js", "/js/desktop/apps/synth-studio-storage.js", "/js/desktop/apps/synth-studio-editor.js", "/js/desktop/apps/synth-studio-workspace.js", "/js/desktop/apps/synth-studio-compose.js", "/js/desktop/apps/synth-studio-performance.js", "/js/desktop/apps/synth-studio-stage.js", "/js/desktop/apps/synth-studio.js", "/testdata/synth-studio-fixture.js"}
	var tags strings.Builder
	for _, path := range scripts {
		fmt.Fprintf(&tags, `<script src="%s"></script>`, path)
	}
	html = strings.Replace(html, "</body>", tags.String()+"</body>", 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `const synthOfficeContext=officeAppContext;officeAppContext=c=>({...synthOfficeContext(c),saveFileDialog:async()=>({path:'Documents/Synth Studio/Browser test.aurasynth'}),confirmDialog:async()=>true,promptDialog:async()=> 'Browser song'});window.synthTest={state,openApp,loadIconManifest,closeWindow,minimizeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/synth-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(120 * time.Second)
	defer func() {
		_, _ = page.Eval(`()=>{for(const id of [...SynthStudioApp.instances.keys()])SynthStudioApp.dispose(id)}`)
		_ = page.Close()
	}()
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await synthFixtureReady;await synthInstance().storage.ready;}`)
	check := func(name, js string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatalf("%s; errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	check("app starts without errors", `()=>fixtureErrors.length===0 && document.querySelectorAll('.ss-library-item').length===209`)
	page.MustElement("[data-template=electronic]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().tracks.length===4`)
	check("template is four bars and paused", `()=>{const p=synthInstance().project();return p.loop.end===7680&&!document.querySelector('[data-action=play]').classList.contains('is-active')}`)
	check("clip repeat bounds allocation before expansion", `()=>{
        const p=SynthStudioModel.create('Repeat limit'),t=SynthStudioModel.addTrack(p,'bass-sub'),c=SynthStudioModel.createClip(t.instrument,0);
        c.length=1;c.notes=Array.from({length:10},(_,i)=>({id:'bounded-'+i,pitch:60,start:0,duration:1,velocity:.5}));t.clips.push(c);
        const host=document.createElement('div');document.body.append(host);let code='';
        const editor=SynthStudioEditor.create(host,{getProject:()=>p,readonly:()=>false,tr:k=>k,mutate:fn=>{try{fn(SynthStudioModel.clone(p))}catch(e){code=e.code}}});
        editor.render();host.querySelector('[data-resize]').dispatchEvent(new PointerEvent('pointerdown',{button:0,clientX:0,shiftKey:true,bubbles:true}));
        document.dispatchEvent(new PointerEvent('pointermove',{clientX:500}));document.dispatchEvent(new PointerEvent('pointerup',{clientX:500}));
        editor.dispose();host.remove();return code==='E_PROJECT_LIMIT'&&c.notes.length===10;
    }`)
	page.MustElement(".ss-app [data-action=demo]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().tracks.length>=5`)
	check("demo has audible notes", `()=>synthInstance().project().tracks.every(t=>t.clips.some(c=>c.notes.length))`)
	page.MustElement(".ss-clip").MustClick()
	check("clip opens notes and FX rack", `()=>document.querySelectorAll('.ss-note').length>0 && document.querySelectorAll('.ss-inspector [data-param]').length===10`)
	page.MustEval(`()=>{window.originalTone=synthInstance().project().tracks[0].params.tone;const e=document.querySelector('[data-param=tone]');e.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));for(const v of [.2,.4,.65]){e.value=v;e.dispatchEvent(new Event('input',{bubbles:true}));}window.dispatchEvent(new PointerEvent('pointerup'));}`)
	check("parameter gesture commits", `()=>synthInstance().project().tracks[0].params.tone===.65`)
	page.MustElement("[data-action=undo]").MustClick()
	check("one undo restores whole gesture", `()=>synthInstance().project().tracks[0].params.tone===originalTone`)
	page.MustEval(`()=>{const e=document.querySelector('[data-param=tone]');e.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));e.value=.12;e.dispatchEvent(new Event('input',{bubbles:true}));window.dispatchEvent(new PointerEvent('pointercancel'));}`)
	check("cancelled gesture leaves model untouched", `()=>synthInstance().project().tracks[0].params.tone===originalTone`)
	page.MustEval(`()=>{const p=synthInstance().project(),t=p.tracks[1];synthInstance().editor.select(t.id,t.clips[0].id);window.generatorOriginal=JSON.stringify(synthInstance().project());}`)
	page.MustElement("[data-work-tab=performance]").MustClick()
	page.MustEval(`()=>{const e=document.querySelector('[data-music=generator]');e.value='arp';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustElement("[data-compose=generate]").MustClick()
	check("preview keeps original unchanged", `()=>JSON.stringify(synthInstance().project())===generatorOriginal&&!document.querySelector('.ss-preview').hidden`)
	page.MustElement("[data-compose=variant]").MustClick()
	page.MustElement("[data-compose=cancel]").MustClick()
	check("cancel preserves complete original", `()=>JSON.stringify(synthInstance().project())===generatorOriginal`)
	page.MustElement("[data-compose=generate]").MustClick()
	page.MustElement("[data-compose=apply]").MustClick()
	check("generator produces editable notes", `()=>synthInstance().project().tracks[1].clips[0].notes.length===8`)
	page.MustElement("[data-action=undo]").MustClick()
	check("generator is one undo step", `()=>JSON.stringify(synthInstance().project())===generatorOriginal`)
	check("rapid chord changes cancel pending audition", `async()=>{
        const audio=synthInstance().audio,unlock=audio.unlock,noteOn=audio.noteOn,resolvers=[];let sounding=0;
        audio.unlock=()=>new Promise(r=>resolvers.push(r));audio.noteOn=()=>{sounding++;return()=>sounding--;};
        const pads=document.querySelectorAll('[data-chord]');
        try{
            pads[0].dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
            pads[1].dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
            for(const resolve of resolvers)resolve();await Promise.resolve();await Promise.resolve();
            const chordOnly=sounding===3;pads[1].dispatchEvent(new KeyboardEvent('keyup',{key:'Enter',bubbles:true}));return chordOnly&&sounding===0;
        }finally{audio.unlock=unlock;audio.noteOn=noteOn;}
    }`)
	page.MustEval(`()=>{const p=synthInstance().project(),t=p.tracks[0];synthInstance().editor.select(t.id,t.clips[0].id);}`)
	before := page.MustEval(`()=>synthInstance().project().tracks.length`).Int()
	page.MustElement("[data-category=leads]").MustClick()
	page.MustElement("[data-library-id=lead-saw] [data-insert]").MustClick()
	check("instrument creates track", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, before+1))
	page.MustElement(".ss-app [data-action=undo]").MustClick()
	check("undo restores track count", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, before))
	page.MustElement(".ss-app [data-action=redo]").MustClick()
	check("redo restores instrument", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, before+1))
	// Exercise the actual custom drag/drop payload through the arrangement listener.
	page.MustEval(`()=>{const d=new DataTransfer();d.setData('application/x-aurago-synth',JSON.stringify({type:'pattern',id:SynthStudioModel.patterns.find(p=>p.category==='leads').id}));document.querySelector('[data-empty-drop]').dispatchEvent(new DragEvent('drop',{dataTransfer:d,bubbles:true,cancelable:true}));}`)
	check("pattern drop creates notes", `()=>synthInstance().project().tracks.at(-1).clips[0].notes.length>0`)
	page.MustEval(`async()=>{await synthInstance().storage.save();}`)
	check("project saved through API", `()=>synthFiles.size===1 && [...synthFiles.values()][0].content.includes('"version":2') && !synthInstance().storage.state.dirty`)
	page.MustEval(`()=>{synthFailWrite=true;document.querySelector('[data-tempo]').value='135';document.querySelector('[data-tempo]').dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustEval(`async()=>{try{await synthInstance().storage.save()}catch{}}`)
	check("failed save preserves project", `()=>synthInstance().storage.state.dirty && synthInstance().project().tempo===135`)
	page.MustEval(`async()=>{synthFailWrite=false;await synthInstance().storage.save();}`)
	page.MustElement(".ss-app [data-action=play]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-position]').textContent!=='01 : 01'`)
	page.MustElement(".ss-app [data-action=stop]").MustClick()
	check("stop resets transport", `()=>document.querySelector('[data-position]').textContent==='01 : 01'`)
	// Record through the real window keyboard listeners and shared audio clock.
	page.MustEval(`()=>{const c=document.querySelector('[data-count-in]');c.checked=false;c.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustElement(".ss-app [data-action=record]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-action=record]').getAttribute('aria-pressed')==='true'`)
	page.MustEval(`()=>document.querySelector('.ss-app').dispatchEvent(new KeyboardEvent('keydown',{key:'a',bubbles:true}))`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-position]').textContent!=='01 : 01'`)
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent('keyup',{key:'a'}))`)
	page.MustElement(".ss-app [data-action=stop]").MustClick()
	check("keyboard recording commits timed notes", `()=>{const n=synthInstance().project().tracks.at(-1).clips.at(-1).notes;return n.length===1&&n[0].pitch===60&&n[0].duration>=120&&n[0].duration<1920;}`)
	page.MustEval(`async()=>{await synthInstance().storage.save();}`)

	page.MustElement("[data-work-tab=performance]").MustClick()
	page.MustEval(`()=>{const set=(s,v)=>{const e=document.querySelector(s);e.value=v;e.dispatchEvent(new Event('change',{bubbles:true}));};set('[data-tempo]',240);set('[data-loop-start]',1);set('[data-loop-end]',2);}`)
	page.MustElement("[data-stage=sceneCreate]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().scenes.length===1`)
	page.MustEval(`()=>{const set=(s,v)=>{const e=document.querySelector(s);e.value=v;e.dispatchEvent(new Event('change',{bubbles:true}));};set('[data-loop-end]',3);set('[data-loop-start]',2);}`)
	page.MustElement("[data-stage=sceneCreate]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().scenes.length===2`)
	page.MustElement("[data-scene]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ss-scene-pads .active').length===1`)
	waitForJSBool(t, page, `()=>synthInstance().audio.elapsed()>.1`)
	page.MustEval(`()=>{document.querySelectorAll('[data-scene]')[1].click();document.querySelectorAll('[data-scene]')[0].click();}`)
	check("latest scene is visibly queued", `()=>document.querySelector('[data-scene]').classList.contains('queued')`)
	waitForJSBool(t, page, `()=>!synthInstance().audio.pendingChange()`)
	check("latest scene becomes active", `()=>document.querySelector('[data-scene]').classList.contains('active')`)
	page.MustElement("[data-action=stop]").MustClick()
	check("stop clears queued scenes", `()=>!document.querySelector('.ss-scene-pads .queued')&&!document.querySelector('.ss-scene-pads .active')`)
	page.MustEval(`()=>{const t=synthInstance().project().tracks[1];synthInstance().editor.select(t.id,t.clips[0].id);window.beforeTake=JSON.stringify(synthInstance().project());window.sourcePath=synthInstance().storage.state.path;}`)
	page.MustElement("[data-stage=takeRecord]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().stage.active()&&synthInstance().audio.running()`)
	page.MustEval(`async()=>{const e=document.querySelector('[data-macro=tone]');e.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,button:0}));e.value=.2;e.dispatchEvent(new Event('input',{bubbles:true}));await new Promise(r=>setTimeout(r,100));e.value=.8;e.dispatchEvent(new Event('input',{bubbles:true}));window.dispatchEvent(new PointerEvent('pointerup'));}`)
	page.MustEval(`()=>document.querySelectorAll('[data-scene]')[0].click()`)
	waitForJSBool(t, page, `()=>!synthInstance().audio.pendingChange()`)
	page.MustElement("[data-stage=takeStop]").MustClick()
	page.MustEval(`async()=>await synthInstance().stage.guard()`)
	check("stopping a take restores editing", `()=>!document.querySelector('[data-tempo]').disabled&&!document.querySelector('[data-compose=generate]').disabled`)
	check("take preserves original and records automation", `()=>{const p=synthInstance().stage.readyTake();return JSON.stringify(synthInstance().project())===beforeTake&&p&&!p.loop.enabled&&p.tracks[1].automation.some(v=>v.parameter==='tone'&&v.value===.8);}`)
	page.MustEval(`async()=>{
        const p=synthInstance().stage.readyTake(),key=location.origin+':synth-studio:take:recovery-test';
        await OfficeSession.draft('put',key,{id:'recovery-take',content:JSON.stringify(p)},'synth-studio');
        window.recoveryHost=document.createElement('div');document.body.append(recoveryHost);window.recoveryLife=new AbortController();
        let ready;const loaded=new Promise(r=>ready=r);
        window.recoveredStage=SynthStudioStage.create(recoveryHost,{tr:k=>k,esc:s=>s,signal:recoveryLife.signal,audio:{meter:()=>0},project:()=>SynthStudioModel.create('Source'),track:()=>null,editable:()=>true,canPlay:()=>true,playing:()=>false,cursor:()=>0,whenReady:()=>loaded,confirm:async()=>true,sessionKey:'recovery-test',transport:recoveryHost,fail:e=>fixtureErrors.push(String(e))});
        recoveredStage.stop();ready();
    }`)
	waitForJSBool(t, page, `()=>!!recoveredStage.readyTake()`)
	check("take recovery waits for project loading", `()=>recoveredStage.readyTake().tracks.length===synthInstance().stage.readyTake().tracks.length`)
	page.MustEval(`()=>{recoveryLife.abort();recoveryHost.remove();}`)
	page.MustElement("[data-stage=takeAdopt]").MustClick()
	waitForJSBool(t, page, `()=>!synthInstance().stage.readyTake()`)
	check("take becomes a distinct unsaved project", `()=>synthInstance().storage.state.path===''&&synthInstance().storage.state.dirty&&synthInstance().project().scenes.length===0&&JSON.stringify(JSON.parse(synthFiles.get(sourcePath).content))===beforeTake`)
	page.MustEval(`()=>{const t=synthInstance().project().tracks[1];synthInstance().editor.select(t.id,t.clips[0].id);window.beforePointEdit=JSON.stringify(synthInstance().project());document.querySelector('.ss-automation-editor').open=true;const e=document.querySelector('[data-automation-value]');e.value='.33';document.querySelector('[data-stage=automationApply]').click();}`)
	check("take automation remains editable", `()=>synthInstance().project().tracks[1].automation.find(p=>p.parameter==='tone').value===.33`)
	page.MustElement("[data-action=undo]").MustClick()
	check("automation edit is one undo step", `()=>JSON.stringify(synthInstance().project())===beforePointEdit`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity"} {
		for _, mode := range []string{"dark", "light"} {
			for _, width := range []int{1280, 760, 420} {
				page.MustEval(`(theme,mode,width)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode=mode;document.body.dataset.mode=mode;document.body.dataset.scheme=mode;const w=document.querySelector('[data-app-id="synth-studio"]');w.classList.remove('maximized');w.style.width=width+'px';w.style.height='800px';w.style.left='8px';w.style.top='8px';}`, theme, mode, width)
				page.MustElement("[data-work-tab=notes]").MustClick()
				page.MustElement(".ss-clip").MustClick()
				page.MustElement("[data-tool=select]").MustClick()
				page.MustEval(`()=>{const editor=document.querySelector('[data-note-editor]');window.beforeNoteEdit=JSON.stringify(synthInstance().project());editor.dispatchEvent(new KeyboardEvent('keydown',{key:'a',ctrlKey:true,bubbles:true}));document.querySelector('[data-copy-notes]').click();}`)
				page.MustElement("[data-paste-notes]").MustClick()
				check("copy/paste remains usable at this width", `()=>JSON.stringify(synthInstance().project())!==beforeNoteEdit`)
				page.MustElement("[data-action=undo]").MustClick()
				check("note edit restores the whole project", `()=>JSON.stringify(synthInstance().project())===beforeNoteEdit`)
				page.MustElement("[data-work-tab=performance]").MustClick()
				t.Log(theme, mode, width, page.MustEval(`()=>{const a=document.querySelector('.ss-app'),r=a.getBoundingClientRect(),f=document.querySelector('.ss-footer').getBoundingClientRect();return JSON.stringify({width:a.clientWidth,scrollWidth:a.scrollWidth,bottom:r.bottom,footerBottom:f.bottom,arrange:document.querySelector('.ss-arrangement').clientHeight})}`).Str())
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("synth-%s-%s-%d.png", theme, mode, width)), page.MustScreenshot(), 0644); err != nil {
						t.Fatal(err)
					}
				}
				check("responsive bounds", `()=>{const a=document.querySelector('.ss-app'),r=a.getBoundingClientRect(),f=document.querySelector('.ss-footer').getBoundingClientRect();return a.scrollWidth<=a.clientWidth+1 && f.bottom<=r.bottom+1 && document.querySelector('.ss-arrangement').clientHeight>=100;}`)
				check("performance pads have touch targets", `()=>[...document.querySelectorAll('.ss-scene-pads button,.ss-chord-pads button')].every(e=>{const r=e.getBoundingClientRect();return r.width>=44&&r.height>=44})`)
			}
		}
	}
	page.MustEval(`()=>{const t=synthInstance().project().tracks[1];synthInstance().editor.select(t.id,t.clips[0].id);window.beforeTouch=JSON.stringify(synthInstance().project());document.querySelector('[data-xy]').scrollIntoView({block:'center'});}`)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		t.Fatal(err)
	}
	xy := page.MustEval(`()=>{const r=document.querySelector('[data-xy]').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}}`)
	touch := func(kind proto.InputDispatchTouchEventType, x, y float64) {
		t.Helper()
		points := []*proto.InputTouchPoint{}
		if kind != proto.InputDispatchTouchEventTypeTouchEnd && kind != proto.InputDispatchTouchEventTypeTouchCancel {
			points = append(points, &proto.InputTouchPoint{X: x, Y: y})
		}
		if err := (proto.InputDispatchTouchEvent{Type: kind, TouchPoints: points}).Call(page); err != nil {
			t.Fatal(err)
		}
	}
	touch(proto.InputDispatchTouchEventTypeTouchStart, xy.Get("x").Num()+xy.Get("w").Num()*.25, xy.Get("y").Num()+xy.Get("h").Num()*.75)
	touch(proto.InputDispatchTouchEventTypeTouchMove, xy.Get("x").Num()+xy.Get("w").Num()*.75, xy.Get("y").Num()+xy.Get("h").Num()*.25)
	touch(proto.InputDispatchTouchEventTypeTouchEnd, 0, 0)
	check("touch XY commits both parameters", `()=>{const p=synthInstance().project().tracks[1].params;return Math.abs(p.tone-.75)<.02&&Math.abs(p.reverb-.75)<.02}`)
	page.MustElement("[data-action=undo]").MustClick()
	check("one undo restores XY gesture", `()=>JSON.stringify(synthInstance().project())===beforeTouch`)
	check("XY marker follows restored sound", `()=>Math.abs(parseFloat(document.querySelector('.ss-xy-dot').style.left)/100-synthInstance().project().tracks[1].params.tone)<.001`)
	page.MustEval(`()=>{document.querySelector('[data-xy]').scrollIntoView({block:'center'});}`)
	xy = page.MustEval(`()=>{const r=document.querySelector('[data-xy]').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}}`)
	touch(proto.InputDispatchTouchEventTypeTouchStart, xy.Get("x").Num()+xy.Get("w").Num()*.2, xy.Get("y").Num()+xy.Get("h").Num()*.2)
	touch(proto.InputDispatchTouchEventTypeTouchCancel, 0, 0)
	check("touch cancellation preserves project", `()=>JSON.stringify(synthInstance().project())===beforeTouch`)
	page.MustElement("[data-macro=tone]").MustFocus()
	for _, kind := range []proto.InputDispatchKeyEventType{proto.InputDispatchKeyEventTypeKeyDown, proto.InputDispatchKeyEventTypeKeyDown, proto.InputDispatchKeyEventTypeKeyUp} {
		if err := (proto.InputDispatchKeyEvent{Type: kind, Key: "ArrowRight", Code: "ArrowRight", WindowsVirtualKeyCode: 39}).Call(page); err != nil {
			t.Fatal(err)
		}
	}
	check("keyboard slider changes sound", `()=>JSON.stringify(synthInstance().project())!==beforeTouch`)
	page.MustElement("[data-action=undo]").MustClick()
	check("one undo restores held keyboard gesture", `()=>JSON.stringify(synthInstance().project())===beforeTouch`)
	page.MustEval(`()=>{const root=document.querySelector('.ss-app');root.classList.add('ss-library-collapsed');root.classList.remove('ss-library-open');document.querySelector('[data-action=library]').click();}`)
	check("compact library opens after desktop collapse", `()=>getComputedStyle(document.querySelector('.ss-library')).display!=='none'`)
	check("reduced motion respected", `()=>matchMedia('(prefers-reduced-motion: reduce)').matches&&getComputedStyle(document.querySelector('.ss-xy-dot')).animationName==='none'`)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
	check("live readonly revocation", `()=>document.querySelector('[data-action=record]').disabled && document.querySelector('[data-action=save]').disabled`)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:auth-ended'))`)
	check("logout disposes editor", `()=>SynthStudioApp.instances.size===0`)
	check("no unhandled browser errors", `()=>fixtureErrors.length===0`)
}

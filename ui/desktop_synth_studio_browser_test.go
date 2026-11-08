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
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/desktop-app-synth-studio.css"><link rel="stylesheet" href="/css/desktop-app-synth-studio-modern.css"><style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	scripts := []string{"/synth-shell.js", "/js/desktop/apps/writer-session.js", "/js/vendor/synth-studio/gm-data.js", "/js/desktop/apps/synth-studio-presets.js", "/js/desktop/apps/synth-studio-designer.js", "/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js", "/js/desktop/apps/synth-studio-model.js", "/js/desktop/apps/synth-studio-fx.js", "/js/desktop/apps/synth-studio-music.js", "/js/desktop/apps/synth-studio-voices.js", "/js/desktop/apps/synth-studio-audio.js", "/js/desktop/apps/synth-studio-export.js", "/js/desktop/apps/synth-studio-midi.js", "/js/desktop/apps/synth-studio-storage.js", "/js/desktop/apps/synth-studio-editor.js", "/js/desktop/apps/synth-studio-controls.js", "/js/desktop/apps/synth-studio-workspace.js", "/js/desktop/apps/synth-studio-compose.js", "/js/desktop/apps/synth-studio-performance.js", "/js/desktop/apps/synth-studio-stage.js", "/js/desktop/apps/synth-studio.js", "/testdata/synth-studio-fixture.js"}
	var tags strings.Builder
	scripts = append([]string{"/js/desktop/apps/synth-studio-strudel.js"}, scripts...)
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
	// The page context carries one deadline for the whole run. The demo holds 138
	// clips and the responsive pass renders the arrangement twelve times, so the
	// budget has to cover the complete flow.
	page := newSmokeBrowser(t).MustPage().Timeout(180 * time.Second)
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
	openLibrary := func() {
		t.Helper()
		if !page.MustEval(`()=>document.querySelector('.ss-app').classList.contains('ss-library-open')`).Bool() {
			page.MustEval(`()=>document.querySelector('[data-action=library]').click()`)
		}
		waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('.ss-library')).visibility==='visible'`)
	}
	closeLibrary := func() {
		t.Helper()
		if page.MustEval(`()=>document.querySelector('.ss-app').classList.contains('ss-library-open')`).Bool() {
			page.MustEval(`()=>document.querySelector('[data-action=library]').click()`)
		}
		waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('.ss-library')).visibility==='hidden'`)
	}
	check("app starts without errors", `()=>fixtureErrors.length===0 && document.querySelectorAll('.ss-library-item').length===249`)
	// Regression: a restored Mix mode used to call renderMaster() during the
	// workspace's first render, before the app had initialised its master
	// constants, so the window never opened.
	reopen := func(seed string) {
		t.Helper()
		page.MustEval(`async(seed)=>{
			if(seed==='')localStorage.removeItem('aurago:synth-studio:view:2');else localStorage.setItem('aurago:synth-studio:view:2',seed);
			const id=[...SynthStudioApp.instances.keys()][0];
			await synthTest.closeWindow(id);
		}`, seed)
		waitForJSBool(t, page, `()=>SynthStudioApp.instances.size===0`)
		page.MustEval(`()=>{synthTest.openApp('synth-studio')}`)
		waitForJSBool(t, page, `()=>{const i=synthInstance();return !!i&&!!i.storage&&!!document.querySelector('[data-master-panel]')}`)
		page.MustEval(`async()=>{await synthInstance().storage.ready;}`)
	}
	reopen(`{"panel":"mix"}`)
	check("restored Mix mode opens without errors", `()=>fixtureErrors.length===0&&!!synthInstance()`)
	check("restored Mix mode shows the populated master panel", `()=>{const p=document.querySelector('[data-master-panel]');return !p.hidden&&p.querySelectorAll('[data-master-param]').length===14&&document.querySelector('[data-work-tab=mix]').getAttribute('aria-selected')==='true'}`)
	reopen("")
	check("stored mode restored for later checks", `()=>fixtureErrors.length===0&&document.querySelector('[data-master-panel]').hidden`)
	page.MustElement("[data-template=electronic]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().tracks.length===5`)
	check("template is sixteen bars and paused", `()=>{const p=synthInstance().project();return p.loop.end===30720&&!document.querySelector('[data-action=play]').classList.contains('is-active')}`)
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
	// The demo replaces the document asynchronously: the project is loaded and
	// then written to disk, and every storage state re-renders the arrangement.
	// Wait for the settled demo so the clip handle stays valid.
	waitForJSBool(t, page, `()=>{const p=synthInstance().project(),s=synthInstance().storage.state;return p.name==='Neon Sketch'&&p.loop.end===61440&&!s.loading}`)
	check("demo has audible notes", `()=>synthInstance().project().tracks.every(t=>t.clips.some(c=>c.notes.length))`)
	page.MustElement(".ss-clip").MustClick()
	check("clip opens notes and FX rack", `()=>document.querySelectorAll('.ss-note').length>0 && document.querySelectorAll('.ss-inspector [data-param]').length===8`)
	check("range sliders are drawn as knobs", `()=>{const i=[...document.querySelectorAll('.ss-inspector [data-param]')];return i.length===8&&i.every(e=>e.classList.contains('ss-ctl-native')&&e.nextElementSibling?.classList.contains('ss-ctl-knob'))}`)
	page.MustElement("[data-work-tab=sound]").MustClick()
	knob := page.MustElement(".ss-inspector [data-param=tone] + .ss-ctl-knob")
	box := knob.MustShape().Box()
	cx, cy := box.X+box.Width/2, box.Y+box.Height/2
	toneBefore := page.MustEval(`()=>synthInstance().project().tracks[0].params.tone`).Num()
	dy := -60.0
	if toneBefore > 0.5 {
		dy = 60.0
	}
	page.Mouse.MustMoveTo(cx, cy)
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo(cx, cy+dy)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	check("dragging a knob changes the model", `()=>Math.abs(synthInstance().project().tracks[0].params.tone-`+fmt.Sprint(toneBefore)+`)>.05`)
	check("the knob shows the new value", `()=>{const i=document.querySelector('.ss-inspector [data-param=tone]');return Math.abs(parseFloat(i.nextElementSibling.style.getPropertyValue('--ss-f'))-Number(i.value))<.02}`)
	page.MustElement("[data-action=undo]").MustClick()
	check("a knob drag is one undo step", `()=>Math.abs(synthInstance().project().tracks[0].params.tone-`+fmt.Sprint(toneBefore)+`)<.001`)
	page.MustElement("[data-work-tab=notes]").MustClick()
	page.MustEval(`()=>{window.originalTone=synthInstance().project().tracks[0].params.tone;const e=document.querySelector('[data-param=tone]');e.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));for(const v of [.2,.4,.65]){e.value=v;e.dispatchEvent(new Event('input',{bubbles:true}));}window.dispatchEvent(new PointerEvent('pointerup'));}`)
	check("parameter gesture commits", `()=>synthInstance().project().tracks[0].params.tone===.65`)
	page.MustElement("[data-action=undo]").MustClick()
	check("one undo restores whole gesture", `()=>synthInstance().project().tracks[0].params.tone===originalTone`)
	page.MustEval(`()=>{const e=document.querySelector('[data-param=tone]');e.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));e.value=.12;e.dispatchEvent(new Event('input',{bubbles:true}));window.dispatchEvent(new PointerEvent('pointercancel'));}`)
	check("cancelled gesture leaves model untouched", `()=>synthInstance().project().tracks[0].params.tone===originalTone`)
	// The master strip belongs to the project, not to a track, and it lives in
	// the transport area so the mix can be judged without opening a panel.
	page.MustElement("[data-action=master]").MustClick()
	waitForJSBool(t, page, `()=>{const p=document.querySelector('[data-master-panel]');return !!p&&!p.hidden}`)
	check("master panel opens with every control", `()=>{const p=document.querySelector('[data-master-panel]');return p.querySelectorAll('[data-master-param]').length===14&&!!p.querySelector('[data-master-select=echoSync]')&&!!p.querySelector('[data-master-param=threshold]')&&!!p.querySelector('[data-master-param=knee]')}`)
	page.MustEval(`()=>{const e=document.querySelector('[data-master-param=threshold]');e.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));e.value=-20;e.dispatchEvent(new Event('input',{bubbles:true}));window.dispatchEvent(new PointerEvent('pointerup'));}`)
	check("master gesture commits into the project", `()=>synthInstance().project().master.threshold===-20`)
	page.MustElement("[data-action=undo]").MustClick()
	check("master gesture is one undo step", `()=>synthInstance().project().master.threshold!==-20`)
	page.MustEval(`()=>{const e=document.querySelector('[data-master-select=echoSync]');e.value='1/16';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
	check("master sync selection commits", `()=>synthInstance().project().master.echoSync==='1/16'`)
	page.MustElement("[data-action=undo]").MustClick()
	check("master sync is one undo step", `()=>synthInstance().project().master.echoSync!=='1/16'`)
	page.MustElement("[data-action=master]").MustClick()
	waitForJSBool(t, page, `()=>{const p=document.querySelector('[data-master-panel]');return !!p&&p.hidden}`)
	page.MustEval(`()=>{const p=synthInstance().project(),t=p.tracks[2],clip=t.clips.find(clip=>clip.length===p.ppq*4);synthInstance().editor.select(t.id,clip.id);window.generatorOriginal=JSON.stringify(synthInstance().project());window.generatorClip=()=>synthInstance().project().tracks[2].clips.find(clip=>clip.length===synthInstance().project().ppq*4);}`)
	page.MustElement("[data-work-tab=performance]").MustClick()
	check("chord and strum arp modes are offered", `()=>{const s=document.querySelector('[data-music=order]');return !!s?.querySelector('option[value=chord]')&&!!s?.querySelector('option[value=strum]');}`)
	check("live performance effects are offered", `()=>document.querySelectorAll('[data-stage=fxFilter],[data-stage=fxStutter]').length===2`)
	check("live performance effects are enabled", `()=>!document.querySelector('[data-stage=fxFilter]').disabled&&!document.querySelector('[data-stage=fxStutter]').disabled`)
	page.MustEval(`()=>document.querySelector('[data-stage=fxStutter]').click()`)
	check("stem export is offered", `()=>!!document.querySelector('[data-export] option[value=stems]')`)
	page.MustEval(`()=>{const e=document.querySelector('[data-music=generator]');e.value='arp';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustElement("[data-compose=generate]").MustClick()
	check("preview keeps original unchanged", `()=>JSON.stringify(synthInstance().project())===generatorOriginal&&!document.querySelector('.ss-preview').hidden`)
	page.MustElement("[data-compose=variant]").MustClick()
	page.MustElement("[data-compose=cancel]").MustClick()
	check("cancel preserves complete original", `()=>JSON.stringify(synthInstance().project())===generatorOriginal`)
	page.MustElement("[data-compose=generate]").MustClick()
	page.MustElement("[data-compose=apply]").MustClick()
	check("generator produces editable notes", `()=>generatorClip().notes.length===8`)
	page.MustElement("[data-action=undo]").MustClick()
	check("generator is one undo step", `()=>JSON.stringify(synthInstance().project())===generatorOriginal`)
	page.MustEval(`()=>{const set=(s,v)=>{const e=document.querySelector(s);e.value=v;e.dispatchEvent(new Event('change',{bubbles:true}));};set('[data-music=generator]','progression');set('[data-music=progression]','pop');set('[data-music=progression_rate]',1);}`)
	page.MustEval(`()=>document.querySelector('[data-compose=generate]').click()`)
	page.MustEval(`()=>document.querySelector('[data-compose=apply]').click()`)
	check("progression generator writes diatonic chords", `()=>{const n=generatorClip().notes;return n.length>=3&&n.length%3===0&&n.every(x=>[0,2,4,5,7,9,11].includes(((x.pitch%12)+12)%12));}`)
	page.MustElement("[data-action=undo]").MustClick()
	check("progression is one undo step", `()=>JSON.stringify(synthInstance().project())===generatorOriginal`)
	page.MustEval(`()=>{const set=(s,v)=>{const e=document.querySelector(s);e.value=v;e.dispatchEvent(new Event('change',{bubbles:true}));};set('[data-music=generator]','groove');set('[data-music=groove_transpose]',1);}`)
	page.MustEval(`()=>document.querySelector('[data-compose=generate]').click()`)
	page.MustEval(`()=>document.querySelector('[data-compose=apply]').click()`)
	check("groove lab transposes the clip", `()=>{const p=synthInstance().project(),before=JSON.parse(generatorOriginal).tracks[2].clips.find(c=>c.length===p.ppq*4).notes,after=generatorClip().notes,a=after.map(n=>n.pitch).sort((x,y)=>x-y),b=before.map(n=>n.pitch+1).sort((x,y)=>x-y);return a.length===b.length&&a.every((x,i)=>x===b[i]);}`)
	page.MustElement("[data-action=undo]").MustClick()
	check("groove lab is one undo step", `()=>JSON.stringify(synthInstance().project())===generatorOriginal`)
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
	openLibrary()
	page.MustElement("[data-category=leads]").MustClick()
	page.MustElement("[data-library-id=lead-saw] [data-insert]").MustClick()
	check("instrument creates track", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, before+1))
	page.MustElement(".ss-app [data-action=undo]").MustClick()
	check("undo restores track count", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, before))
	page.MustElement(".ss-app [data-action=redo]").MustClick()
	check("redo restores instrument", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, before+1))
	closeLibrary()
	// The programmable synthesizer: design the selected track's sound, save it
	// as a browser-local synth and see it applied as the track instrument. The
	// prompt seam of the fixture answers the name dialog with "Browser song".
	page.MustEval(`()=>{const p=synthInstance().project(),t=p.tracks.find(track=>track.instrument!=='drums-kit'&&track.instrument!=='gm-drums'&&!track.instrument.startsWith('drum-')&&track.clips.length);if(document.querySelector('.ss-sound-workbench').hidden)document.querySelector('[data-work-tab=sound]').click();synthInstance().editor.select(t.id,t.clips[0].id);window.designTrackId=t.id;window.designInstrument=t.instrument;window.designWave=(SynthStudioPresets.get(t.instrument).patch||{}).wave||'sawtooth';}`)
	page.MustElement(".ss-inspector [data-action=design]").MustClick()
	check("designer opens the full parameter surface", `()=>document.querySelectorAll('.ss-designer [data-designer-param]').length>=30&&document.querySelectorAll('.ss-designer-group').length===7&&!SynthStudioPresets.presets.some(p=>p.id==='user-preview')`)
	page.MustEval(`()=>{const e=document.querySelector('[data-designer-param=level]');e.value=1.5;e.dispatchEvent(new Event('input',{bubbles:true}));}`)
	check("parameter change drives the preview instrument", `()=>SynthStudioPresets.get('user-preview')&&SynthStudioPresets.get('user-preview').patch.level===1.5`)
	page.MustElement("[data-designer-action=saveAs]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().tracks.find(t=>t.id===designTrackId).instrument.startsWith('user-')`)
	check("saved synth becomes the track instrument", `()=>{const t=synthInstance().project().tracks.find(t=>t.id===designTrackId);return SynthStudioPresets.get(t.instrument).patch.level===1.5}`)
	check("saved synth is browser-local and listed", `()=>localStorage.getItem('aurago:synth-studio:patches:1').includes('Browser song')&&document.querySelector('[data-footer-count]').textContent.startsWith('121 SYNTHS')&&!!document.querySelector('[data-category=user]')`)
	check("designer switches to update mode", `()=>{const name=document.querySelector('[data-designer-name]');return !!name&&name.value==='Browser song'&&!!document.querySelector('[data-designer-action=save]')}`)
	page.MustElement(".ss-app [data-action=undo]").MustClick()
	check("applying the synth is one undo step", `()=>synthInstance().project().tracks.find(t=>t.id===designTrackId).instrument===designInstrument`)
	page.MustElement(".ss-app [data-action=redo]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().tracks.find(t=>t.id===designTrackId)?.instrument.startsWith('user-')`)
	page.MustElement("[data-designer-action=close]").MustClick()
	check("designer returns to the sound panel", `()=>!document.querySelector('[data-designer]')&&document.querySelectorAll('.ss-inspector [data-param]').length===8`)
	// A user synth is a first-class instrument: it can be inserted from the
	// library like a factory patch, and the pencil opens its saved draft again.
	openLibrary()
	page.MustElement("[data-category=user]").MustClick()
	userID := page.MustEval(`()=>SynthStudioPresets.presets.find(p=>p.category==='user').id`).Str()
	tracksBeforeUser := page.MustEval(`()=>synthInstance().project().tracks.length`).Int()
	page.MustEval(fmt.Sprintf(`()=>{document.querySelector('[data-library-id=%q] [data-insert]').click();}`, userID))
	waitForJSBool(t, page, fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, tracksBeforeUser+1))
	check("user synth inserts as an instrument", fmt.Sprintf(`()=>synthInstance().project().tracks.at(-1).instrument===%q`, userID))
	page.MustElement(".ss-app [data-action=undo]").MustClick()
	waitForJSBool(t, page, fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, tracksBeforeUser))
	check("insert undo removes only the user-synth track", fmt.Sprintf(`()=>synthInstance().project().tracks.filter(t=>t.instrument===%q).length===1`, userID))
	openLibrary()
	page.MustElement("[data-library-id=" + userID + "] [data-edit-patch]").MustClick()
	check("library pencil reopens the saved synth", `()=>!!document.querySelector('[data-designer]')&&document.querySelector('[data-designer-name]').value==='Browser song'&&document.querySelector('[data-designer-param=wave]').value===designWave`)
	check("inactive designer sections start collapsed", `()=>{const g=n=>document.querySelector('[data-designer] details[data-group='+n+']');return g('grp_oscillator').open&&g('grp_filter').open&&g('grp_envelope').open&&!!g('grp_fm')&&!!g('grp_lfo')}`)
	page.MustEval(`()=>{const d=document.querySelector('[data-designer] details[data-group=grp_lfo]');window.lfoWasOpen=d.open;d.querySelector('summary').click();}`)
	check("a section toggles and keeps its state", `()=>new Promise(r=>setTimeout(()=>r(document.querySelector('[data-designer] details[data-group=grp_lfo]').open===!lfoWasOpen),50))`)
	page.MustElement("[data-designer-action=close]").MustClick()
	closeLibrary()
	page.MustElement("[data-work-tab=notes]").MustClick()
	// The track menu removes a track from the timeline. Reaching the delete
	// button at the bottom of the sound panel is not a way to delete a track.
	page.MustEval(`()=>{document.querySelectorAll('[data-track-menu]')[1].click();}`)
	check("track menu offers the track actions", `()=>{const m=document.querySelector('.ss-track-menu');return !!m&&!!m.querySelector('[data-track-menu-action=delete]')&&!!m.querySelector('[data-track-menu-action=duplicate]');}`)
	menuTracks := page.MustEval(`()=>synthInstance().project().tracks.length`).Int()
	page.MustElement(".ss-track-menu [data-track-menu-action=delete]").MustClick()
	waitForJSBool(t, page, fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, menuTracks-1))
	check("track menu deletes the track", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, menuTracks-1))
	check("deleted track leaves a valid selection", `()=>{const p=synthInstance().project(),id=synthInstance().editor.selection().trackId;return !id||p.tracks.some(t=>t.id===id);}`)
	page.MustElement(".ss-app [data-action=undo]").MustClick()
	check("undo restores the deleted track", fmt.Sprintf(`()=>synthInstance().project().tracks.length===%d`, menuTracks))
	// Exercise the actual custom drag/drop payload through the arrangement listener.
	page.MustEval(`()=>{const d=new DataTransfer();d.setData('application/x-aurago-synth',JSON.stringify({type:'pattern',id:SynthStudioModel.patterns.find(p=>p.category==='leads').id}));document.querySelector('[data-empty-drop]').dispatchEvent(new DragEvent('drop',{dataTransfer:d,bubbles:true,cancelable:true}));}`)
	check("pattern drop creates notes", `()=>synthInstance().project().tracks.at(-1).clips[0].notes.length>0`)
	page.MustEval(`async()=>{await synthInstance().storage.save();}`)
	check("project saved through API", `()=>synthFiles.size===1 && [...synthFiles.values()][0].content.includes('"version":3') && !synthInstance().storage.state.dirty`)
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
	// The demo ships two scenes, the two new ones follow them.
	waitForJSBool(t, page, `()=>synthInstance().project().scenes.length===3`)
	page.MustEval(`()=>{const set=(s,v)=>{const e=document.querySelector(s);e.value=v;e.dispatchEvent(new Event('change',{bubbles:true}));};set('[data-loop-end]',3);set('[data-loop-start]',2);}`)
	page.MustElement("[data-stage=sceneCreate]").MustClick()
	waitForJSBool(t, page, `()=>synthInstance().project().scenes.length===4`)
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
				page.MustEval(`()=>document.querySelector('.ss-clip').scrollIntoView({block:'nearest',inline:'nearest'})`)
				page.MustElement(".ss-clip").MustClick()
				page.MustElement("[data-tool=select]").MustClick()
				page.MustEval(`()=>{const p=synthInstance().project();const t=p.tracks.find(track=>track.clips.some(c=>c.notes.length));if(t){const c=t.clips.find(c=>c.notes.length);synthInstance().editor.select(t.id,c.id);}synthInstance().editor.setPlayhead(0);const editor=document.querySelector('[data-note-editor]');window.beforeNoteEdit=JSON.stringify(synthInstance().project());editor.dispatchEvent(new KeyboardEvent('keydown',{key:'a',ctrlKey:true,bubbles:true}));document.querySelector('[data-copy-notes]').click();}`)
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
				check("responsive bounds", `()=>{const a=document.querySelector('.ss-app'),r=a.getBoundingClientRect(),f=document.querySelector('.ss-footer').getBoundingClientRect();return a.scrollWidth<=a.clientWidth+1 && f.bottom<=r.bottom+1 && document.querySelector('.ss-performance-workbench').clientHeight>=100;}`)
				check("performance pads have touch targets", `()=>[...document.querySelectorAll('.ss-scene-pads button,.ss-chord-pads button')].every(e=>{const r=e.getBoundingClientRect();return r.width>=44&&r.height>=44})`)
				page.MustElement("[data-work-tab=strudel]").MustClick()
				check("Strudel editor stays inside the workbench", `()=>{const el=document.querySelector('[data-strudel-code]'),p=el.closest('section'),r=p.getBoundingClientRect();return !p.hidden&&el.clientWidth>120&&r.right<=document.querySelector('.ss-app').getBoundingClientRect().right+1&&p.scrollWidth<=p.clientWidth+1;}`)
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("synth-strudel-%s-%s-%d.png", theme, mode, width)), page.MustScreenshot(), 0644); err != nil {
						t.Fatal(err)
					}
				}
				page.MustElement("[data-work-tab=performance]").MustClick()
			}
		}
	}
	page.MustEval(`()=>{const w=document.querySelector('[data-app-id="synth-studio"]');w.classList.remove('maximized');w.style.width='1500px';w.style.height='800px';w.style.left='8px';w.style.top='8px';}`)
	// Modes: one at a time, the arrangement only in Arrange, the note editor only with a clip.
	page.MustElement("[data-work-tab=notes]").MustClick()
	check("arrange shows the arrangement", `()=>document.querySelector('.ss-app').dataset.mode==='arrange'&&getComputedStyle(document.querySelector('.ss-arrangement')).display!=='none'`)
	page.MustEval(`()=>synthInstance().editor.select('','')`)
	check("note editor hides without a clip", `()=>document.querySelector('.ss-note-editor').hidden`)
	page.MustEval(`()=>{const p=synthInstance().project(),t=p.tracks[0];synthInstance().editor.select(t.id,t.clips[0].id)}`)
	check("note editor opens with a clip", `()=>!document.querySelector('.ss-note-editor').hidden`)
	page.MustElement("[data-work-tab=sound]").MustClick()
	check("sound mode hides the arrangement and fills the area", `()=>{const a=document.querySelector('.ss-arrangement');const p=document.querySelector('.ss-sound-workbench');return getComputedStyle(a).display==='none'&&!p.hidden&&p.getBoundingClientRect().width>600}`)
	page.MustElement("[data-work-tab=mix]").MustClick()
	check("mixer opens the master panel", `()=>!document.querySelector('[data-master-panel]').hidden&&document.querySelectorAll('[data-master-panel] [data-master-param]').length===14`)
	page.MustElement("[data-work-tab=notes]").MustClick()
	check("leaving the mixer closes what it opened", `()=>document.querySelector('[data-master-panel]').hidden`)
	check("mode is remembered", `()=>JSON.parse(localStorage.getItem('aurago:synth-studio:view:2')).panel==='notes'`)
	page.MustEval(`()=>{const w=document.querySelector('[data-app-id="synth-studio"]');w.classList.remove('maximized');w.style.width='420px';w.style.height='800px';w.style.left='8px';w.style.top='8px';}`)
	page.MustElement("[data-work-tab=performance]").MustClick()
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
	check("performance macros are drawn as knobs", `()=>[...document.querySelectorAll('[data-macro]')].length===2&&[...document.querySelectorAll('[data-macro]')].every(e=>e.classList.contains('ss-ctl-native')&&e.nextElementSibling?.classList.contains('ss-ctl-knob'))`)
	page.MustEval(`()=>document.querySelector('[data-macro=tone] + .ss-ctl-knob').scrollIntoView({block:'center'})`)
	macroBox := page.MustElement("[data-macro=tone] + .ss-ctl-knob").MustShape().Box()
	macroX, macroY := macroBox.X+macroBox.Width/2, macroBox.Y+macroBox.Height/2
	macroDY := -60.0
	macroBefore := page.MustEval(`()=>synthInstance().project().tracks[1].params.tone`).Num()
	if macroBefore > 0.5 {
		macroDY = 60.0
	}
	page.Mouse.MustMoveTo(macroX, macroY)
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo(macroX, macroY+macroDY)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	check("dragging a macro knob changes the sound and the readout", `()=>{const e=document.querySelector('[data-macro=tone]');return Math.abs(synthInstance().project().tracks[1].params.tone-`+fmt.Sprint(macroBefore)+`)>.05&&Number(e.parentElement.querySelector('output').textContent)===Number(Number(e.value).toFixed(2))}`)
	page.MustElement("[data-action=undo]").MustClick()
	check("a macro knob drag is one undo step", `()=>JSON.stringify(synthInstance().project())===beforeTouch&&Math.abs(parseFloat(document.querySelector('[data-macro=tone] + .ss-ctl-knob').style.getPropertyValue('--ss-f'))-Number(document.querySelector('[data-macro=tone]').value))<.02`)
	page.MustEval(`()=>{const root=document.querySelector('.ss-app');root.classList.remove('ss-library-open');document.querySelector('[data-action=library]').click();}`)
	waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('.ss-library')).visibility==='visible'`)
	check("library drawer opens", `()=>document.querySelector('.ss-app').classList.contains('ss-library-open')`)
	check("reduced motion respected", `()=>matchMedia('(prefers-reduced-motion: reduce)').matches&&getComputedStyle(document.querySelector('.ss-xy-dot')).animationName==='none'`)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
	check("live readonly revocation", `()=>document.querySelector('[data-action=record]').disabled && document.querySelector('[data-action=save]').disabled`)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:auth-ended'))`)
	check("logout disposes editor", `()=>SynthStudioApp.instances.size===0`)
	check("no unhandled browser errors", `()=>fixtureErrors.length===0`)
}

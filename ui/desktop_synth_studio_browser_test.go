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
)

func TestDesktopSynthStudioBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/desktop-app-synth-studio.css"><style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	scripts := []string{"/synth-shell.js", "/js/desktop/apps/writer-session.js", "/js/vendor/synth-studio/gm-data.js", "/js/desktop/apps/synth-studio-presets.js", "/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js", "/js/desktop/apps/synth-studio-model.js", "/js/desktop/apps/synth-studio-audio.js", "/js/desktop/apps/synth-studio-midi.js", "/js/desktop/apps/synth-studio-storage.js", "/js/desktop/apps/synth-studio-editor.js", "/js/desktop/apps/synth-studio.js", "/testdata/synth-studio-fixture.js"}
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
	defer page.Close()
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await synthFixtureReady;await synthInstance().storage.ready;}`)
	check := func(name, js string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatalf("%s; errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	check("app starts without errors", `()=>fixtureErrors.length===0 && document.querySelectorAll('.ss-library-item').length===10`)
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
	check("clip opens notes", `()=>document.querySelectorAll('.ss-note').length>0`)
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
	check("project saved through API", `()=>synthFiles.size===1 && [...synthFiles.values()][0].content.includes('"version":1') && !synthInstance().storage.state.dirty`)
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

	for _, theme := range []string{"standard", "fruity"} {
		for _, mode := range []string{"dark", "light"} {
			for _, width := range []int{1280, 760, 420} {
				page.MustEval(`(theme,mode,width)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode=mode;document.body.dataset.mode=mode;const w=document.querySelector('[data-app-id="synth-studio"]');w.classList.remove('maximized');w.style.width=width+'px';w.style.height='800px';w.style.left='8px';w.style.top='8px';}`, theme, mode, width)
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
			}
		}
	}
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
	check("live readonly revocation", `()=>document.querySelector('[data-action=record]').disabled && document.querySelector('[data-action=save]').disabled`)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:auth-ended'))`)
	check("logout disposes editor", `()=>SynthStudioApp.instances.size===0`)
	check("no unhandled browser errors", `()=>fixtureErrors.length===0`)
}

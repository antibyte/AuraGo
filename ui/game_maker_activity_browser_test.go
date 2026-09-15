package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestGameMakerActivityTerminalBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	events := make(chan map[string]any, 80)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/game-maker/projects/forest/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": connected\n\n")
		w.(http.Flusher).Flush()
		for {
			select {
			case event := <-events:
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "id: %v\nevent: %v\ndata: %s\n\n", event["id"], event["type"], data)
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/fonts/fonts.css"><link rel="stylesheet" href="/shared-variables.css">
<link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-game-maker-studio.css">
<style>html,body,#app{width:100%;height:100%;margin:0}*{box-sizing:border-box}</style></head>
<body class="desktop-body" data-theme="standard"><div id="app"></div>
<script src="/js/desktop/apps/game-maker-studio-modals.js"></script>
<script src="/js/desktop/apps/game-maker-studio-api.js"></script>
<script src="/js/desktop/apps/game-maker-studio-activity.js"></script>
<script src="/js/desktop/apps/game-maker-studio-preview.js"></script>
<script src="/js/desktop/apps/game-maker-studio.js"></script>
<script>
window.activityTimers=new Set();const nativeTimeout=window.setTimeout,nativeClear=window.clearTimeout;
window.setTimeout=(fn,ms,...args)=>{let id=nativeTimeout(()=>{activityTimers.delete(id);fn(...args)},ms);if(fn.name==='tick')activityTimers.add(id);return id};
window.clearTimeout=id=>{activityTimers.delete(id);nativeClear(id)};
const projects=[{id:'forest',name:'Waldabenteuer',description:'Ein Plattformspiel mit Münzen und Gegnern im Wald.',dimension:'2d',status:'draft',current_revision:0},{id:'other',name:'Weltraum',dimension:'3d',status:'draft',current_revision:0}];
fetch('/lang/desktop/de.json').then(r=>r.json()).then(lang=>GameMakerStudioApp.render(document.getElementById('app'),'fixture',{
 esc:value=>String(value??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),t:(key,args)=>{let value=lang[key]||key;for(const [k,v] of Object.entries(args||{}))value=value.replace('{'+k+'}',v);return value},
 api:async path=>path.endsWith('/capabilities')?{enabled:true,allow_create:true,allow_edit:true,skills_ready:true,phaser_version:'4.2.1',three_version:'0.185.1',active_job:{job_id:'job',project_id:'forest',status:'planning',phase:'planning'}}:path.endsWith('/projects')?{projects}:path.endsWith('/preview-token')?{url:'/preview-fixture',token:'fixture'}:{project:projects.find(p=>path.endsWith('/'+p.id))||projects[0],messages:[]}
}));
</script></body></html>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(45 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustSetViewport(1920, 1080, 1, false)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>document.querySelector('.gm-build-line')?.textContent.length>4&&GameMakerStudioApp.instances.get('fixture').eventSource.readyState===1`)
	// Observe actual incremental typing before testing the reduced-motion path.
	page.MustEval(`()=>GameMakerStudioApp.instances.get('fixture').activity.event({type:'file_changed',payload:{path:'typing-probe-'+ 'x'.repeat(180)}})`)
	page.MustWait(`()=>document.querySelector('.gm-build-lines').lastElementChild.textContent.includes('typing-probe')`)
	if !page.MustEval(`()=>document.querySelector('.gm-build-lines').lastElementChild.textContent.length<180`).Bool() {
		t.Fatal("expected a partially typed line")
	}
	page.MustEval(`()=>{const activity=GameMakerStudioApp.instances.get('fixture').activity;activity.reset();activity.sync()}`)
	page.MustEval(`()=>{window.iconY=document.querySelector('.gm-preview-empty img').getBoundingClientRect().y;window.fixedCopy=document.querySelector('.gm-preview-empty strong').textContent}`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	send := func(id int, kind string, payload map[string]any) {
		events <- map[string]any{"id": id, "project_id": "forest", "job_id": "job", "type": kind, "payload": payload}
	}
	send(1, "phase", map[string]any{"phase": "building"})
	send(2, "tool_call", map[string]any{"attempted": true, "tool": "game_maker_file"})
	for i, path := range []string{"src/world.ts", "src/player.ts", "src/coins.ts", "src/enemies.ts", "src/levels.ts", "src/audio.ts", "src/main.ts"} {
		send(i+3, "file_changed", map[string]any{"path": path})
	}
	page.MustWait(`()=>document.querySelector('.gm-build-lines')?.textContent.includes('src/main.ts')`)
	if !page.MustEval(`()=>Math.abs(document.querySelector('.gm-preview-empty img').getBoundingClientRect().y-iconY)<1&&document.querySelector('.gm-preview-empty strong').textContent===fixedCopy&&getComputedStyle(document.querySelector('.gm-build-cursor')).animationName==='none'&&getComputedStyle(document.querySelector('.gm-build-terminal')).fontFamily.includes('Press Start 2P')`).Bool() {
		t.Fatal("foreground moved or reduced motion/font contract failed")
	}
	// Raw model output, unknown tool fields and stale job events are not terminal input.
	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');for(const type of ['text_delta','thinking','diagnostic'])s.activity.event({type,payload:{content:'PRIVATE_SENTINEL',message:'PRIVATE_SENTINEL'}});s.activity.event({type:'file_changed',job_id:'old',payload:{path:'STALE_SENTINEL'}})}`)
	if dir := os.Getenv("AURAGO_GAME_TERMINAL_SCREENSHOTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, theme := range []string{"standard", "light", "dark"} {
			for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {390, 844}} {
				page.MustSetViewport(size[0], size[1], 1, size[0] == 390)
				page.MustEval(`theme=>{document.body.dataset.theme=theme==='standard'?'standard':'fruity';document.body.dataset.fruityMode=theme}`, theme)
				page.MustElement("[data-gm-preview]").MustScrollIntoView()
				page.MustEval(`async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))}`)
				page.MustWait(`()=>{const lines=document.querySelector('.gm-build-lines');return lines.scrollHeight-lines.scrollTop-lines.clientHeight<2}`)
				if !page.MustEval(`()=>document.querySelector('.gm-build-terminal').getBoundingClientRect().right<=innerWidth+1`).Bool() {
					t.Fatal("terminal extends beyond the visible preview")
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("terminal-%s-%d.png", theme, size[0])), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	// Backpressure, replay deduplication, hidden views and preview replacement.
	page.MustSetViewport(1366, 768, 1, false)
	send(10, "file_changed", map[string]any{"path": "<img src=x onerror=alert(1)>"})
	page.MustWait(`()=>document.querySelector('.gm-build-lines').textContent.includes('<img')`)
	if !page.MustEval(`()=>!document.querySelector('.gm-build-lines img')&&!/PRIVATE_SENTINEL|STALE_SENTINEL/.test(document.querySelector('.gm-build-lines').textContent)`).Bool() {
		t.Fatal("terminal interpreted markup or exposed private/stale content")
	}
	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');for(let id=11;id<90;id++)s.activity.event({id,type:'file_changed',payload:{path:'src/file-'+id+'.ts'}});s.activity.event({id:89,type:'file_changed',payload:{path:'REPLAY_SENTINEL'}})}`)
	page.MustWait(`()=>document.querySelector('.gm-build-lines').textContent.includes('file-89.ts')`)
	if !page.MustEval(`()=>document.querySelector('.gm-build-lines').children.length<=24&&!document.querySelector('.gm-build-lines').textContent.includes('REPLAY_SENTINEL')`).Bool() {
		t.Fatal("terminal history or replay was unbounded")
	}
	page.MustEval(`()=>document.querySelector('[data-gm-preview]').style.display='none'`)
	page.MustWait(`()=>activityTimers.size===0`)
	page.MustEval(`()=>document.querySelector('[data-gm-preview]').style.display=''`)
	page.MustWait(`()=>activityTimers.size===1`)
	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');s.job.status='failed';s.activity.sync()}`)
	if !page.MustEval(`()=>activityTimers.size===0&&!document.querySelector('.gm-build-terminal')`).Bool() {
		t.Fatal("finished job retained the active terminal")
	}
	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');s.job={id:'retry',status:'building',phase:'building'};s.activity.sync()}`)
	page.MustWait(`()=>activityTimers.size===1&&document.querySelectorAll('.gm-build-line').length===1`)
	page.MustEval(`()=>GameMakerStudioApp.instances.get('fixture').refreshPreview()`)
	page.MustWait(`()=>!!document.querySelector('.gm-preview-frame')&&!document.querySelector('.gm-build-terminal')&&activityTimers.size===0`)
	page.MustElement(`[data-project-id="other"]`).MustClick()
	page.MustWait(`()=>GameMakerStudioApp.instances.get('fixture').project.id==='other'&&!document.querySelector('.gm-preview-frame')`)
	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');s.job={id:'another',status:'planning',phase:'planning'};s.activity.sync()}`)
	page.MustWait(`()=>activityTimers.size===1`)
	page.MustEval(`()=>GameMakerStudioApp.dispose('fixture')`)
	if !page.MustEval(`()=>activityTimers.size===0&&!document.querySelector('.gm-build-terminal')`).Bool() {
		t.Fatal("terminal survived disposal")
	}
}

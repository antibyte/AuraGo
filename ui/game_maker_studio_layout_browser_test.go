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
)

const gameMakerStudioLayoutFixture = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
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
window.fixtureErrors=[];
addEventListener('error',e=>fixtureErrors.push(e.error?.stack||e.message));
addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
window.fixtureNotices=[];
const now=Date.now();
window.fixtureProjects=[
 {id:'forest',name:'Waldabenteuer mit einem sehr langen Projektnamen für die Seitenleiste',description:'Ein Plattformspiel mit Münzen, Gegnern und drei Leveln im Wald.',dimension:'2d',status:'ready',current_revision:3,project_key:'Games/waldabenteuer',provider_id:'main',model:'step-5-preview',updated_at:new Date(now-5*60000).toISOString()},
 {id:'orbit',name:'Orbit Garden',description:'Sammle Sterne im All.',dimension:'3d',status:'failed',current_revision:0,project_key:'Games/orbit',updated_at:new Date(now-3*3600000).toISOString()},
 {id:'blocks',name:'Blockwelt',description:'Baue eine Hütte.',dimension:'3d',variant:'voxel',status:'draft',current_revision:1,project_key:'Games/blockwelt',updated_at:new Date(now-2*86400000).toISOString()}];
window.fixtureMessages={forest:[
 {role:'user',content:'Ein Plattformspiel mit Münzen, Gegnern und drei Leveln im Wald.',created_at:new Date(now-20*60000).toISOString()},
 {role:'assistant',content:'Steuerung: Pfeiltasten bewegen, Leertaste springt, R startet neu.\n\nZiel: Sammle in jedem der drei Waldlevel alle Münzen und erreiche das Tor. Wildschweine patrouillieren auf den Wegen, Fledermäuse stoßen in dunklen Bereichen herab.',created_at:new Date(now-12*60000).toISOString()},
 {role:'user',content:'Bitte mach die Gegner etwas langsamer und füge einen Doppelsprung hinzu.',created_at:new Date(now-6*60000).toISOString()}],orbit:[],blocks:[]};
window.fixtureCapabilities={enabled:true,readonly:false,allow_create:true,allow_edit:true,allow_delete:true,allow_media_generation:true,skills_ready:true,code_studio:true,
 phaser_version:'4.2.1',three_version:'0.185.1',voxel_version:1,default_provider_id:'main',default_model:'step-5-preview',
 providers:[{id:'main',name:'StepFun',model:'step-5-preview'},{id:'local',name:'Lokales Modell',model:'qwen3'}],active_job:null,skills:[]};
fetch('/lang/desktop/de.json').then(r=>r.json()).then(lang=>{window.fixtureLang=lang;GameMakerStudioApp.render(document.getElementById('app'),'fixture',{
 esc:value=>String(value??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),
 t:(key,args)=>{let value=lang[key]||key;for(const [k,v] of Object.entries(args||{}))value=value.replaceAll('{{'+k+'}}',v).replaceAll('{'+k+'}',v);return value},
 confirmDialog:async()=>true,promptDialog:async(title,value)=>value,notify:n=>fixtureNotices.push(n),openApp:()=>{},
 api:async(path,options)=>{
  if(path.endsWith('/capabilities'))return fixtureCapabilities;
  if(path.endsWith('/projects')&&(!options||!options.method||options.method==='GET'))return {projects:fixtureProjects};
  if(path.endsWith('/preview-token'))return {url:'/preview-fixture',token:'fixture',revision:3};
  if(path.endsWith('/revisions'))return {revisions:[{number:3,summary:'Doppelsprung',created_at:new Date(now-5*60000).toISOString()},{number:2,summary:'Langsamere Gegner',created_at:new Date(now-40*60000).toISOString()},{number:1,summary:'Erste Version',created_at:new Date(now-90*60000).toISOString()}]};
  if(path.endsWith('/jobs'))return {id:'job-new',status:'queued',phase:'queued'};
  const project=fixtureProjects.find(p=>path.endsWith('/'+p.id))||fixtureProjects[0];
  return {project,messages:fixtureMessages[project.id]||[]};
 }});window.fixtureReady=true;});
</script></body></html>`

// The Studio must stay usable at desktop, laptop and phone widths in every state
// a job can reach. Screenshots are optional review material, not assertions.
func TestGameMakerStudioLayoutBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	events := make(chan map[string]any, 80)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/preview-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body style="margin:0;background:#16324f;display:grid;place-items:center;height:100vh;color:#fff;font:600 20px system-ui"><canvas width="960" height="540" style="width:100%;height:100%;background:linear-gradient(#1d6fa5,#0b2239)"></canvas></body></html>`)
	})
	mux.HandleFunc("/api/game-maker/projects/", func(w http.ResponseWriter, r *http.Request) {
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
		fmt.Fprint(w, gameMakerStudioLayoutFixture)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustSetViewport(1440, 900, 1, false)
	page.MustWait(`()=>window.fixtureReady&&GameMakerStudioApp.instances.get('fixture')?.project?.id==='forest'&&!!document.querySelector('.gm-preview-frame')`)

	reports := os.Getenv("GAMEMAKER_STUDIO_REPORTS")
	snapshot := func(name string) {
		t.Helper()
		if reports == "" {
			return
		}
		if err := os.MkdirAll(reports, 0755); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))}`)
		if err := os.WriteFile(filepath.Join(reports, name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(name, js string, args ...any) {
		t.Helper()
		result, err := page.Eval(js, args...)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		if !result.Value.Bool() {
			t.Errorf("%s; errors=%s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	layout := func(name string) {
		t.Helper()
		check(name+": studio fits the window without horizontal overflow", `()=>{const s=document.querySelector('.gm-studio');return s.scrollWidth<=s.clientWidth+1&&document.documentElement.scrollWidth<=innerWidth+1&&[...s.querySelectorAll('.gm-workspace,.gm-agent-pane,.gm-preview-pane,.gm-conversation')].every(e=>e.scrollWidth<=e.clientWidth+1)}`)
		check(name+": every visible control stays inside the window", `()=>{const s=document.querySelector('.gm-studio').getBoundingClientRect();return [...document.querySelectorAll('.gm-topbar button,.gm-pane-head button,.gm-pane-head select,.gm-change-form button,.gm-library-head button')].filter(b=>b.offsetParent).every(b=>{const r=b.getBoundingClientRect();return r.left>=s.left-1&&r.right<=s.right+1})}`)
		check(name+": no control text is clipped", `()=>[...document.querySelectorAll('.gm-topbar button,.gm-preview-tools button,.gm-change-form button,.gm-library-head button,.gm-pane-head button')].filter(b=>b.offsetParent).every(b=>b.scrollWidth<=b.clientWidth+2)`)
		check(name+": a new game can always be started", `()=>[...document.querySelectorAll('[data-gm-action="new"]')].filter(b=>b.offsetParent&&!b.disabled).length===1`)
	}
	send := func(id int, kind string, payload map[string]any) {
		events <- map[string]any{"id": id, "project_id": "forest", "job_id": "job", "type": kind, "payload": payload}
	}

	layout("ready")
	check("ready: conversation and change form are visible", `()=>{const f=document.querySelector('[data-gm-change-form] textarea').getBoundingClientRect();return f.bottom<=innerHeight+1&&f.height>20&&document.querySelectorAll('.gm-message').length===3}`)
	check("ready: long project names do not push the revision out", `()=>{const card=document.querySelector('.gm-project-card').getBoundingClientRect(),rev=document.querySelector('.gm-project-card .gm-project-revision').getBoundingClientRect();return rev.right<=card.right+1&&rev.width>8}`)
	check("ready: the library names each state in the user's language", `()=>{const labels=[...document.querySelectorAll('.gm-project-card small')].map(s=>s.textContent);return labels.length===3&&labels[0].startsWith(fixtureLang['game_maker.status_ready'])&&labels[1].startsWith(fixtureLang['game_maker.status_failed'])&&labels[2].startsWith(fixtureLang['game_maker.status_draft'])&&!labels.some(l=>/^(ready|failed|draft)/.test(l))}`)
	snapshot("studio-01-ready")

	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');s.job={id:'job',status:'building',phase:'building'};s.jobStartedAt=Date.now()-75000;s.activeJob={job_id:'job',project_id:'forest',status:'building',phase:'building'};}`)
	send(1, "phase", map[string]any{"phase": "building"})
	send(2, "file_changed", map[string]any{"path": "src/main.ts"})
	send(3, "tool_call", map[string]any{"attempted": true, "tool": "game_maker_validate"})
	page.MustWait(`()=>!document.querySelector('[data-gm-job-banner]').hidden&&document.querySelector('[data-gm-conversation]').textContent.includes('src/main.ts')`)
	layout("running")
	check("running: stop stays reachable and the form is locked", `()=>{const stop=document.querySelector('[data-gm-action="stop"]').getBoundingClientRect();return stop.width>30&&stop.right<=innerWidth&&document.querySelector('[data-gm-change-form] textarea').disabled}`)
	check("running: the conversation keeps the free space, not the phase row", `()=>{const pane=document.querySelector('.gm-agent-pane').getBoundingClientRect(),phases=document.querySelector('[data-gm-phases]').getBoundingClientRect(),banner=document.querySelector('[data-gm-job-banner]').getBoundingClientRect(),log=document.querySelector('[data-gm-conversation]').getBoundingClientRect(),form=document.querySelector('[data-gm-change-form]').getBoundingClientRect();return phases.height<90&&banner.height<90&&log.top<=phases.bottom+2&&log.height>pane.height/2&&Math.abs(form.bottom-pane.bottom)<2}`)
	snapshot("studio-02-running")
	page.MustEval(`()=>{const notice=document.querySelector('[data-gm-capability-notice]');notice.hidden=false;notice.querySelector('[data-gm-capability-title]').textContent='Fixture notice'}`)
	check("running: an additional notice does not displace the conversation", `()=>{const pane=document.querySelector('.gm-agent-pane').getBoundingClientRect(),phases=document.querySelector('[data-gm-phases]').getBoundingClientRect(),log=document.querySelector('[data-gm-conversation]').getBoundingClientRect(),form=document.querySelector('[data-gm-change-form]').getBoundingClientRect();return phases.height<90&&log.height>pane.height/3&&Math.abs(form.bottom-pane.bottom)<2&&Math.abs(log.bottom-form.top)<2}`)
	page.MustEval(`()=>{document.querySelector('[data-gm-capability-notice]').hidden=true}`)

	send(4, "validation_result", map[string]any{"result": map[string]any{"gameplay_status": "unavailable", "rules_status": "unverified", "repairable": true, "checks": []map[string]any{{"id": "required_rules", "status": "unavailable", "repairable": true, "expected": "pickup_events increased 0", "observed": "before=0, after=0; target=item no_target (inputs=0, contacts=0, effects=0)"}}}})
	send(5, "job_status", map[string]any{"status": "failed", "error": "game validation failed: required_rules: expected pickup_events increased 0; observed before=0, after=0; target=item no_target (inputs=0, contacts=0, effects=0)"})
	page.MustWait(`()=>!!document.querySelector('.gm-result-card.is-error')`)
	layout("failed")
	check("failed: the result card offers a retry inside the conversation", `()=>{const card=document.querySelector('.gm-result-card.is-error'),log=document.querySelector('[data-gm-conversation]').getBoundingClientRect(),r=card.getBoundingClientRect();return !!card.querySelector('[data-gm-retry]')&&r.left>=log.left-1&&r.right<=log.right+1}`)
	check("failed: long technical errors wrap instead of overflowing", `()=>{const card=document.querySelector('.gm-result-card.is-error');return card.scrollWidth<=card.clientWidth+1}`)
	check("failed: the card explains the failure and keeps the report one click away", `()=>{const card=document.querySelector('.gm-result-card.is-error'),details=card.querySelector('details');return card.querySelector('.gm-result-hint').textContent===fixtureLang['game_maker.failure_hint_validation']&&!details.open&&details.querySelector('summary').textContent===fixtureLang['game_maker.failure_details']&&details.querySelector('p').textContent.includes('required_rules: expected pickup_events increased 0')}`)
	snapshot("studio-03-failed")
	page.MustElement(`.gm-result-card.is-error summary`).MustClick()
	check("failed: the opened report stays inside the conversation", `()=>{const card=document.querySelector('.gm-result-card.is-error'),report=card.querySelector('details p').getBoundingClientRect(),box=card.getBoundingClientRect();return card.querySelector('details').open&&report.height>10&&report.right<=box.right+1&&card.scrollWidth<=card.clientWidth+1}`)
	snapshot("studio-03b-failed-details")

	page.MustElement(`[data-gm-action="new"]`).MustClick()
	page.MustWait(`()=>!!document.querySelector('[data-gm-create]')`)
	dialog := `()=>{const s=document.querySelector('.gm-studio').getBoundingClientRect(),modal=document.querySelector('.gm-create-modal'),m=modal.getBoundingClientRect(),submit=modal.querySelector('footer .gm-primary').getBoundingClientRect(),close=modal.querySelector('header [data-modal-close]').getBoundingClientRect();
const inside=r=>r.top>=m.top-1&&r.bottom<=m.bottom+1&&r.left>=m.left-1&&r.right<=m.right+1&&r.width>20;
const top=(x,y)=>modal.contains(document.elementFromPoint(x,y))&&!!document.elementFromPoint(x,y).closest('footer,header');
return m.top>=s.top-1&&m.bottom<=s.bottom+1&&m.left>=s.left-1&&m.right<=s.right+1&&modal.scrollWidth<=modal.clientWidth+1&&inside(submit)&&inside(close)&&top(submit.left+submit.width/2,submit.top+submit.height/2)&&top(close.left+close.width/2,close.top+close.height/2)}`
	check("create: the dialog fits the window and its actions stay reachable", dialog)
	snapshot("studio-04-create")
	page.MustEval(`()=>{const modal=document.querySelector('.gm-create-modal');modal.scrollTop=modal.scrollHeight}`)
	check("create: title and actions stay pinned after scrolling to the last field", dialog)
	check("create: the last field is not covered by the pinned actions", `()=>{const modal=document.querySelector('.gm-create-modal'),footer=modal.querySelector('footer').getBoundingClientRect(),fields=[...modal.querySelectorAll('input,select,textarea')].filter(f=>f.offsetParent),last=fields[fields.length-1].getBoundingClientRect();return last.bottom<=footer.top+1}`)
	page.MustEval(`()=>{document.querySelector('.gm-create-modal').scrollTop=0;GameMakerStudioModals.modalError(document.querySelector('[data-gm-modal]'),'Fixture failure')}`)
	check("create: an error appears above the pinned actions and in view", `()=>{const modal=document.querySelector('.gm-create-modal'),error=modal.querySelector('.gm-modal-error'),e=error.getBoundingClientRect(),footer=modal.querySelector('footer').getBoundingClientRect(),header=modal.querySelector('header').getBoundingClientRect();return modal.lastElementChild.tagName==='FOOTER'&&e.height>10&&e.bottom<=footer.top+1&&e.top>=header.bottom-1}`)
	snapshot("studio-04b-create-error")
	page.MustElement(`[data-gm-create] [data-modal-close]`).MustClick()
	page.MustWait(`()=>!document.querySelector('[data-gm-create]')`)

	page.MustElement(`[data-gm-action="revisions"]`).MustClick()
	page.MustWait(`()=>!!document.querySelector('.gm-modal')`)
	snapshot("studio-05-revisions")
	page.MustEval(`()=>document.querySelector('.gm-modal [data-modal-close]').click()`)

	page.MustSetViewport(1100, 700, 1, false)
	page.MustEval(`async()=>{await new Promise(r=>setTimeout(r,120))}`)
	layout("laptop")
	snapshot("studio-06-laptop")

	// A narrow window on a wide screen must adapt exactly like a narrow screen.
	for _, width := range []int{900, 780, 640, 420} {
		page.MustEval(`async width=>{document.getElementById('app').style.width=width+'px';await new Promise(r=>setTimeout(r,120))}`, width)
		name := fmt.Sprintf("window %dpx", width)
		layout(name)
		check(name+": a project can be chosen without the sidebar", `()=>{const select=document.querySelector('[data-gm-mobile-projects]');return document.querySelector('.gm-library').offsetParent===null&&select.offsetParent!==null&&select.options.length===3&&select.getBoundingClientRect().width>=120}`)
		check(name+": panes share the window as designed", `width=>{const a=document.querySelector('.gm-agent-pane').getBoundingClientRect(),p=document.querySelector('.gm-preview-pane').getBoundingClientRect();return width>760?Math.abs(a.top-p.top)<2&&p.left>=a.right-1&&a.width>=300:p.top>=a.bottom-1&&a.width<=width&&a.width>=width-20}`, width)
		snapshot(fmt.Sprintf("studio-06b-window-%d", width))
	}
	page.MustElement(`.gm-narrow-new`).MustClick()
	page.MustWait(`()=>!!document.querySelector('[data-gm-create]')`)
	check("window 420px: the create dialog fits the window", dialog)
	snapshot("studio-06c-window-create")
	page.MustElement(`[data-gm-create] [data-modal-close]`).MustClick()
	page.MustEval(`async()=>{document.getElementById('app').style.width='';await new Promise(r=>setTimeout(r,120))}`)

	page.MustSetViewport(390, 844, 1, true)
	page.MustEval(`async()=>{await new Promise(r=>setTimeout(r,120))}`)
	layout("phone")
	check("phone: a project can be chosen without the sidebar", `()=>{const select=document.querySelector('[data-gm-mobile-projects]');return !!select&&!select.hidden&&select.offsetParent!==null&&select.options.length===3}`)
	snapshot("studio-07-phone")
	page.MustElement(`.gm-narrow-new`).MustClick()
	page.MustWait(`()=>!!document.querySelector('[data-gm-create]')`)
	check("phone: the create dialog fits the window", dialog)
	snapshot("studio-08-phone-create")
	page.MustElement(`[data-gm-create] [data-modal-close]`).MustClick()

	page.MustSetViewport(1440, 900, 1, false)
	page.MustEval(`async()=>{document.body.dataset.theme='fruity';document.body.dataset.fruityMode='light';await new Promise(r=>setTimeout(r,120))}`)
	layout("light")
	snapshot("studio-09-light")
	check("no script errors", `()=>fixtureErrors.length===0`)
}

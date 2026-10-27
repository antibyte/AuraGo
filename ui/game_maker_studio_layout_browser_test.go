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
 phaser_version:'4.2.1',three_version:'0.186.1',voxel_version:1,default_provider_id:'main',default_model:'step-5-preview',
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
  if(path.endsWith('/workspace-drift'))return window.fixtureDrift||{revision:3,changed:[],added:[],removed:[],changed_count:0,added_count:0,removed_count:0,truncated:false};
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
		fmt.Fprint(w, ": connected\n\nevent: stream_ready\ndata: {\"type\":\"stream_ready\"}\n\n")
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
		check(name+": every visible control stays inside the window", `()=>{const s=document.querySelector('.gm-studio').getBoundingClientRect();return [...document.querySelectorAll('.gm-topbar button,.gm-pane-head button,.gm-pane-head select,.gm-change-form button,.gm-library-head button,.gm-steps button')].filter(b=>b.offsetParent).every(b=>{const r=b.getBoundingClientRect();return r.left>=s.left-1&&r.right<=s.right+1})}`)
		check(name+": no control text is clipped", `()=>[...document.querySelectorAll('.gm-topbar button,.gm-preview-tools button,.gm-change-form button,.gm-library-head button,.gm-pane-head button,.gm-steps button')].filter(b=>b.offsetParent).every(b=>b.scrollWidth<=b.clientWidth+2)`)
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
	page.MustWait(`()=>!document.querySelector('[data-gm-job-banner]').hidden&&!document.querySelector('[data-gm-job-feed]').hidden&&document.querySelector('[data-gm-job-feed-lines]').textContent.includes('src/main.ts')`)
	layout("running")
	check("running: stop stays reachable and the form is locked", `()=>{const stop=document.querySelector('[data-gm-action="stop"]').getBoundingClientRect();return stop.width>30&&stop.right<=innerWidth&&document.querySelector('[data-gm-change-form] textarea').disabled}`)
	check("running: the conversation keeps the free space, not the phase row", `()=>{const pane=document.querySelector('.gm-agent-pane').getBoundingClientRect(),phases=document.querySelector('[data-gm-phases]').getBoundingClientRect(),banner=document.querySelector('[data-gm-job-banner]').getBoundingClientRect(),log=document.querySelector('[data-gm-conversation]').getBoundingClientRect(),form=document.querySelector('[data-gm-change-form]').getBoundingClientRect();return phases.height<90&&banner.height<90&&log.top<=phases.bottom+2&&log.height>pane.height/2&&Math.abs(form.bottom-pane.bottom)<2}`)
	snapshot("studio-02-running")
	page.MustEval(`()=>{const notice=document.querySelector('[data-gm-capability-notice]');notice.hidden=false;notice.querySelector('[data-gm-capability-title]').textContent='Fixture notice'}`)
	check("running: an additional notice does not displace the conversation", `()=>{const pane=document.querySelector('.gm-agent-pane').getBoundingClientRect(),phases=document.querySelector('[data-gm-phases]').getBoundingClientRect(),log=document.querySelector('[data-gm-conversation]').getBoundingClientRect(),form=document.querySelector('[data-gm-change-form]').getBoundingClientRect();return phases.height<90&&log.height>pane.height/3&&Math.abs(form.bottom-pane.bottom)<2&&Math.abs(log.bottom-form.top)<2}`)
	page.MustEval(`()=>{document.querySelector('[data-gm-capability-notice]').hidden=true}`)
	send(35, "phase", map[string]any{"phase": "validating"})
	page.MustWait(`()=>{const hint=document.querySelector('[data-gm-validating-hint]');return hint&&!hint.hidden&&hint.textContent===fixtureLang['game_maker.validating_preview_hint']}`)

	send(4, "validation_result", map[string]any{"result": map[string]any{"gameplay_status": "unavailable", "rules_status": "unverified", "repairable": true, "checks": []map[string]any{{"id": "required_rules", "status": "unavailable", "repairable": true, "expected": "pickup_events increased", "observed": "before=0, after=0; target=item no_target (inputs=0, contacts=0, effects=0)"}}}})
	send(5, "job_status", map[string]any{"status": "failed", "error": "game validation failed: required_rules: expected pickup_events increased; observed before=0, after=0; target=item no_target (inputs=0, contacts=0, effects=0)", "usage": map[string]any{"calls": 2, "tokens": 110}})
	page.MustWait(`()=>!!document.querySelector('.gm-result-card.is-error')`)
	layout("failed")
	check("failed: the result card offers a retry inside the conversation", `()=>{const card=document.querySelector('.gm-result-card.is-error'),log=document.querySelector('[data-gm-conversation]').getBoundingClientRect(),r=card.getBoundingClientRect();return !!card.querySelector('[data-gm-retry]')&&r.left>=log.left-1&&r.right<=log.right+1}`)
	check("failed: long technical errors wrap instead of overflowing", `()=>{const card=document.querySelector('.gm-result-card.is-error');return card.scrollWidth<=card.clientWidth+1}`)
	check("failed: the card explains the failure and keeps the report one click away", `()=>{const card=document.querySelector('.gm-result-card.is-error'),details=card.querySelector('details');return card.querySelector('.gm-result-hint').textContent===fixtureLang['game_maker.failure_hint_validation']&&!details.open&&details.querySelector('summary').textContent===fixtureLang['game_maker.failure_details']&&details.querySelector('p').textContent.includes('required_rules: expected pickup_events increased')}`)
	check("failed: the card shows measured calls and tokens and an unknown price", `()=>{const line=document.querySelector('.gm-result-card.is-error .gm-result-usage');const tokens=fixtureLang['game_maker.usage_tokens'].replaceAll('{{count}}','110');const expected=fixtureLang['game_maker.usage_line'].replaceAll('{{calls}}','2').replaceAll('{{tokens}}',tokens).replaceAll('{{cost}}',fixtureLang['game_maker.usage_cost_unknown']);return !!line&&line.textContent===expected&&line.scrollWidth<=line.clientWidth+1}`)
	snapshot("studio-03-failed")
	page.MustElement(`.gm-result-card.is-error summary`).MustClick()
	check("failed: the opened report stays inside the conversation", `()=>{const card=document.querySelector('.gm-result-card.is-error'),report=card.querySelector('details p').getBoundingClientRect(),box=card.getBoundingClientRect();return card.querySelector('details').open&&report.height>10&&report.right<=box.right+1&&card.scrollWidth<=card.clientWidth+1}`)
	snapshot("studio-03b-failed-details")
	page.MustEval(`()=>{window.fixtureDrift={revision:3,changed:['src/main.ts'],added:[],removed:[],changed_count:1,added_count:0,removed_count:0,truncated:false}}`)
	send(40, "job_status", map[string]any{"status": "failed", "error": "game validation failed: required_rules: expected pickup_events increased; observed before=0, after=0; target=item no_target (inputs=0, contacts=0, effects=0)"})
	page.MustWait(`()=>{const badge=document.querySelector('[data-gm-drift]');const adopt=badge&&badge.querySelector('[data-gm-action="adopt"]');return badge&&!badge.hidden&&adopt&&!adopt.hidden}`)
	check("drift: the badge stays inside the preview", `()=>{const shell=document.querySelector('.gm-preview-shell').getBoundingClientRect(),badge=document.querySelector('[data-gm-drift]').getBoundingClientRect(),studio=document.querySelector('.gm-studio');return badge.width>20&&badge.left>=shell.left-1&&badge.right<=shell.right+1&&studio.scrollWidth<=studio.clientWidth+1}`)
	page.MustEval(`()=>{window.fixtureDrift={revision:3,changed:[],added:[],removed:[],changed_count:0,added_count:0,removed_count:0,truncated:false}}`)
	send(41, "job_status", map[string]any{"status": "failed", "error": "game validation failed: required_rules: expected pickup_events increased; observed before=0, after=0; target=item no_target (inputs=0, contacts=0, effects=0)"})
	page.MustWait(`()=>document.querySelector('[data-gm-drift]').hidden&&document.querySelector('[data-gm-drift-hint]').hidden`)

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

	page.MustElement(`[data-gm-more-toggle]`).MustClick()
	page.MustElement(`[data-gm-action="revisions"]`).MustClick()
	page.MustWait(`()=>!!document.querySelector('.gm-modal')`)
	snapshot("studio-05-revisions")
	page.MustEval(`()=>document.querySelector('.gm-modal [data-modal-close]').click()`)

	page.MustSetViewport(1100, 700, 1, false)
	page.MustEval(`async()=>{await new Promise(r=>setTimeout(r,120))}`)
	layout("laptop")
	snapshot("studio-06-laptop")

	// A narrow window on a wide screen must adapt exactly like a narrow screen.
	// Validation selects the preview step. Put the conversation back before the
	// narrow checks, which expect that default.
	page.MustEval(`()=>{const studio=document.querySelector('.gm-studio');studio.setAttribute('data-gm-step','agent');studio.querySelectorAll('[data-gm-steps] [data-gm-step]').forEach(b=>b.setAttribute('aria-pressed',b.getAttribute('data-gm-step')==='agent'?'true':'false'))}`)
	for _, width := range []int{900, 780, 640, 420} {
		page.MustEval(`async width=>{document.getElementById('app').style.width=width+'px';await new Promise(r=>setTimeout(r,120))}`, width)
		name := fmt.Sprintf("window %dpx", width)
		layout(name)
		check(name+": a project can be chosen without the sidebar", `()=>{const select=document.querySelector('[data-gm-mobile-projects]');return document.querySelector('.gm-library').offsetParent===null&&select.offsetParent!==null&&select.options.length===3&&select.getBoundingClientRect().width>=120}`)
		check(name+": panes share the window as designed", `width=>{const studio=document.querySelector('.gm-studio'),a=document.querySelector('.gm-agent-pane').getBoundingClientRect(),p=document.querySelector('.gm-preview-pane').getBoundingClientRect(),bar=document.querySelector('[data-gm-steps]');if(width>760)return Math.abs(a.top-p.top)<2&&p.left>=a.right-1&&a.width>=300&&bar.offsetParent===null;return studio.getAttribute('data-gm-step')==='agent'&&bar.offsetParent!==null&&a.width<=width&&a.width>=width-20&&document.querySelector('.gm-preview-pane').offsetParent===null&&a.height>120}`, width)
		snapshot(fmt.Sprintf("studio-06b-window-%d", width))
	}
	check("window 420px: the preview is its own step", `async()=>{
		const fail=(reason)=>{throw new Error(reason)};
		const studio=document.querySelector('.gm-studio');
		const bar=document.querySelector('[data-gm-steps]');
		const agent=document.querySelector('.gm-agent-pane');
		const preview=document.querySelector('.gm-preview-pane');
		const agentBtn=bar.querySelector('[data-gm-step="agent"]');
		const previewBtn=bar.querySelector('[data-gm-step="preview"]');
		const state=GameMakerStudioApp.instances.get('fixture');
		if (!bar || bar.offsetParent===null) fail('step bar is hidden');
		if (agentBtn.textContent!==fixtureLang['game_maker.step_agent'] || previewBtn.textContent!==fixtureLang['game_maker.step_preview']) fail('step labels');
		if (studio.getAttribute('data-gm-step')!=='agent' || preview.offsetParent || !agent.offsetParent) fail('agent step does not own the workspace');
		previewBtn.click();
		await new Promise(r=>setTimeout(r,40));
		if (studio.getAttribute('data-gm-step')!=='preview' || !preview.offsetParent || agent.offsetParent) fail('preview step did not replace the agent');
		if (studio.scrollWidth>studio.clientWidth+1) fail('preview step overflows');
		state.job=Object.assign({}, state.job||{}, {id:'job', phase:'validating', status:'validating'});
		const saved=state.context.confirmDialog;
		let asks=0;
		state.context.confirmDialog=async()=>{asks++; return false};
		agentBtn.click();
		await new Promise(r=>setTimeout(r,40));
		if (asks!==1 || studio.getAttribute('data-gm-step')!=='preview' || !preview.offsetParent) fail('a declined check left the preview');
		state.context.confirmDialog=async()=>{asks++; return true};
		agentBtn.click();
		await new Promise(r=>setTimeout(r,40));
		if (asks!==2 || studio.getAttribute('data-gm-step')!=='agent' || preview.offsetParent) fail('confirming the check did not return to the agent');
		state.context.confirmDialog=saved;
		state.lastPhase='building';
		state.validationPreviewDismissed=true;
		return true;
	}`)
	send(42, "phase", map[string]any{"phase": "validating"})
	page.MustWait(`()=>{const s=GameMakerStudioApp.instances.get('fixture');return s.lastPhase==='validating'&&document.querySelector('.gm-studio').getAttribute('data-gm-step')==='agent'}`)
	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');s.validationPreviewDismissed=false;s.lastPhase='building'}`)
	send(43, "phase", map[string]any{"phase": "validating"})
	page.MustWait(`()=>document.querySelector('.gm-studio').getAttribute('data-gm-step')==='preview'&&!!document.querySelector('.gm-preview-pane').offsetParent`)
	page.MustEval(`()=>{
		const s=GameMakerStudioApp.instances.get('fixture');
		s.validationPreviewDismissed=false;
		s.lastPhase='';
		s.job.phase='failed';
		s.job.status='failed';
		s.jobActive=false;
		const banner=document.querySelector('[data-gm-job-banner]');
		if (banner) banner.hidden=true;
		const studio=document.querySelector('.gm-studio');
		studio.setAttribute('data-gm-step','agent');
		studio.querySelectorAll('[data-gm-steps] [data-gm-step]').forEach(b=>b.setAttribute('aria-pressed', b.getAttribute('data-gm-step')==='agent'?'true':'false'));
	}`)
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
	page.MustEval(`()=>new Promise((resolve,reject)=>{const s=document.createElement('script');s.src='/js/desktop/apps/game-maker-studio-assets.js';s.onload=resolve;s.onerror=()=>reject(new Error('assets script failed'));document.body.append(s)})`)
	check("selection: each project keeps its last pick and retry sends none", `async()=>{
		const fail=(reason)=>{throw new Error(reason)};
		const A=window.GameMakerStudioAssets, state=GameMakerStudioApp.instances.get('fixture');
		const key='aurago.game-maker.asset-selection';
		localStorage.setItem(key, JSON.stringify({projects:{
			forest:{packs:['../nope','pine','pine'], models:['engine_-1.7','../x'], assets:[{pack_id:'aurago-low-poly', asset_id:'oak'},{pack_id:'../x', asset_id:'y'}], dimensions:{'aurago-low-poly':'3d','../x':'2d'}, presentation:{effects:['rain'], sounds:[{event:'step', sound:'step-grass'}], environment:'', quality:'nope', tweaks:{a:1}}},
			'bad id':{packs:['pine']},
			orbit:{packs:['space']}
		}, order:['forest','bad id','orbit']}));
		A.applyProjectSelection(state);
		if (state.selectedAssetPackIDs.join(',')!=='pine' || state.selectedModelAssetIDs.join(',')!=='engine_-1.7') fail('stored forest selection was not restored');
		if (!state.selectedAssetSelections[0] || state.selectedAssetSelections[0].asset_id!=='oak' || state.selectedAssetDimensions['aurago-low-poly']!=='3d') fail('stored asset dimension was not restored');
		if (!state.selectedPresentation || state.selectedPresentation.effects[0]!=='rain' || state.selectedPresentation.quality!=='auto' || state.selectedPresentation.tweaks) fail('presentation extras were kept');
		A.applyProjectSelection(state, 'orbit');
		if (state.selectedAssetPackIDs.join(',')!=='space') fail('switching projects kept the previous selection');
		A.applyProjectSelection(state, 'missing');
		if (state.selectedAssetPackIDs.length || state.selectedPresentation) fail('a project without a memory kept the previous selection');
		state.selectedAssetPackIDs=['pine'];
		state.selectedModelAssetIDs=['engine_-1.7'];
		state.selectedAssetSelections=[{pack_id:'aurago-low-poly', asset_id:'oak'}];
		state.selectedAssetDimensions={'aurago-low-poly':'3d'};
		state.selectedPresentation={effects:['rain'], sounds:[{event:'step', sound:'step-grass'}], environment:'', quality:'auto'};
		A.rememberAssetSelection(state, 'forest');
		state.selectedAssetPackIDs=['space'];
		state.selectedModelAssetIDs=[];
		state.selectedAssetSelections=[];
		state.selectedAssetDimensions={};
		state.selectedPresentation=null;
		A.rememberAssetSelection(state, 'orbit');
		A.applyProjectSelection(state, 'forest');
		const resume=A.jobSelection(state, true);
		const flat=A.jobSelection(state, false);
		const solid=A.jobSelection(state, false, '3d');
		if (resume.asset_pack_ids.length || resume.model_asset_ids.length || resume.asset_selections.length || resume.presentation!==null) fail('retry included the remembered selection');
		if (flat.asset_pack_ids[0]!=='pine' || flat.model_asset_ids.length || solid.model_asset_ids[0]!=='engine_-1.7') fail('the next change request lost the remembered selection');
		A.holdSelection(state);
		state.selectedAssetPackIDs=['cards'];
		const layer=state.container.querySelector('[data-gm-modal]');
		layer.hidden=false;
		layer.innerHTML='<form data-gm-create></form>';
		GameMakerStudioModals.closeModal(state);
		if (state.selectionHold || state.selectedAssetPackIDs[0]!=='pine') fail('cancelling a new game kept the draft selection');
		A.holdSelection(state);
		state.selectedAssetPackIDs=['nature'];
		layer.hidden=false;
		layer.innerHTML='<section class="gm-assets-modal"></section>';
		GameMakerStudioModals.closeModal(state);
		if (state.selectedAssetPackIDs[0]!=='nature') fail('closing the asset browser dropped the draft');
		A.forgetAssetSelection('forest');
		A.applyProjectSelection(state, 'forest');
		if (state.selectedAssetPackIDs.length) fail('a deleted project kept its selection');
		for (let i=0;i<30;i++){state.selectedAssetPackIDs=['pack-'+i]; A.rememberAssetSelection(state, 'p'+i);}
		const stored=JSON.parse(localStorage.getItem(key));
		if (stored.order.length!==25 || stored.projects.p0 || !stored.projects.p29 || stored.order[stored.order.length-1]!=='p29') fail('selection memory did not keep the newest 25 projects');
		state.selectedAssetPackIDs=['pine'];
		state.selectedModelAssetIDs=[];
		state.selectedAssetSelections=[];
		state.selectedAssetDimensions={};
		state.selectedPresentation={effects:['rain'], sounds:[{event:'step', sound:'step-grass'}], environment:'', quality:'auto'};
		const calls=[];
		state.api.startJob=async(id, body)=>{calls.push({id, body}); return {id:'job-memory', status:'queued', phase:'planning', prompt: body.resume ? 'continue' : ''};};
		const form=state.container.querySelector('[data-gm-change-form]');
		const input=form.querySelector('textarea');
		state.jobActive=false;
		input.disabled=false;
		form.querySelector('button').disabled=false;
		input.value='add a coin';
		form.requestSubmit();
		const wait=n=>new Promise((resolve,reject)=>{const start=Date.now(); const tick=()=>{if(calls.length>=n) return resolve(); if(Date.now()-start>2000) return reject(new Error('startJob was not called')); setTimeout(tick, 20);}; tick();});
		await wait(1);
		if (calls[0].body.asset_pack_ids[0]!=='pine' || calls[0].body.model_asset_ids.length || calls[0].body.presentation.effects[0]!=='rain') fail('the change request did not send the visible selection');
		if (state.selectedAssetPackIDs[0]!=='pine') fail('starting a change cleared the selection');
		state.jobActive=false;
		input.disabled=false;
		const retry=document.createElement('button');
		retry.type='button';
		retry.dataset.gmRetry='1';
		state.container.querySelector('.gm-studio').append(retry);
		retry.click();
		await wait(2);
		retry.remove();
		const again=calls[1].body;
		if (!again.resume || again.asset_pack_ids.length || again.model_asset_ids.length || again.asset_selections.length || again.presentation!==null) fail('retry sent the remembered selection');
		if (state.selectedAssetPackIDs[0]!=='pine') fail('retry cleared the visible selection');
		localStorage.removeItem(key);
		return true;
	}`)
	check("no script errors", `()=>fixtureErrors.length===0`)
}

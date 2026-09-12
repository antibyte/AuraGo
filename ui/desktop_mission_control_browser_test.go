package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in headless smoke test for the Mission Control desktop app. Renders the
// real modules against a fake /api/missions/v2 backend, walks list → detail →
// history → editor → compact mode and checks text safety and translations.
// Set AURAGO_RUN_BROWSER_SMOKE=1 to run, AURAGO_BROWSER_ARTIFACT_DIR to keep PNGs.
func TestDesktopMissionControlBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	modules := ""
	for _, name := range []string{"schedule", "triggers", "menus", "list", "detail", "editor"} {
		modules += readDesktopAssetText(t, "js/desktop/apps/mission-control-"+name+".js") + "\n"
	}
	shell := readDesktopAssetText(t, "js/desktop/apps/mission-control.js")
	browser := newSmokeBrowser(t)
	themes := map[string]string{
		"de": "--vd-theme-app-bg:#14171c;--vd-theme-panel-bg:#1c2026;--vd-theme-panel-bg-strong:#232830;--vd-theme-chrome-bg:#181c22;--vd-theme-border:#343a44;--vd-theme-border-strong:#4a515d;--vd-text:#eef1f6;--vd-muted:#a5adba;--vd-accent:#27c7a6;--vd-accent-r:39;--vd-accent-g:199;--vd-accent-b:166;--vd-control-bg:rgba(255,255,255,.08);--vd-control-hover:rgba(255,255,255,.12)",
		"en": "--vd-theme-app-bg:#f3f5f9;--vd-theme-panel-bg:#ffffff;--vd-theme-panel-bg-strong:#eef1f6;--vd-theme-chrome-bg:#f7f8fb;--vd-theme-border:#d7dce5;--vd-theme-border-strong:#b9c1ce;--vd-text:#1a1e26;--vd-muted:#5c6470;--vd-accent:#0f8f7a;--vd-accent-r:15;--vd-accent-g:143;--vd-accent-b:122;--vd-control-bg:rgba(255,255,255,.66);--vd-control-hover:rgba(0,0,0,.05)",
	}
	for _, lang := range []string{"de", "en"} {
		t.Run(lang, func(t *testing.T) {
			words := readDesktopAssetText(t, "lang/desktop/"+lang+".json")
			missionsWords := readDesktopAssetText(t, "lang/missions/"+lang+".json")
			var dict map[string]string
			if err := json.Unmarshal([]byte(words), &dict); err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.Handle("/", http.FileServer(http.FS(Content)))
			mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `<html lang="`+lang+`"><head><link rel="stylesheet" href="/css/desktop-app-mission-control.css"><style>
html,body{margin:0;height:100%;font-family:system-ui,sans-serif;--ds-space-2:8px;--ds-space-3:12px;--ds-space-6:24px;--ds-radius-sm:6px;--ds-radius-md:10px;--ds-radius-full:999px;--ds-type-size-xs:11px;--ds-type-size-sm:13px;--ds-type-size-md:14px;--ds-type-size-lg:17px;--ds-type-size-xl:20px;--ds-motion-duration-fast:120ms;--ds-motion-ease-standard:ease;`+themes[lang]+`}
#win{width:1100px;height:720px;color:var(--vd-text)}</style></head><body class="desktop-body" data-theme="standard"><div id="win"></div><script>
const words=Object.assign({},`+missionsWords+`,`+words+`);
const t=(k,v)=>{let s=words[k]||k;if(v){for(const [a,b] of Object.entries(v)){s=s.replaceAll('{{'+a+'}}',String(b));}}return s;};
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';');
window.requests=[];window.toasts=[];window.menus=null;window.contextMenus=[];window.confirmAnswer=true;window.beforeClose=null;
const now=Date.now();
window.missions=[
 {id:'m1',name:'<img src=x onerror=window.pwned=1> Backup',prompt:'Back up the NAS and report.',execution_type:'scheduled',schedule:'0 9 * * *',enabled:true,priority:'high',runner_type:'local',status:'idle',last_run:new Date(now-3600e3).toISOString(),last_result:'success',last_output:'{"choices":[{"message":{"content":"Backup finished: 12 files"}}]}',run_count:14,next_run:new Date(now+7200e3).toISOString(),preparation_status:'prepared'},
 {id:'m2',name:'Morning digest',prompt:'Summarize overnight logs.',execution_type:'triggered',trigger_type:'webhook',trigger_config:{webhook_id:'w1',webhook_slug:'digest',min_interval_seconds:120},enabled:true,priority:'medium',runner_type:'local',status:'running',last_run:new Date(now-86400e3).toISOString(),last_result:'error',last_output:'boom',run_count:3,preparation_status:'none'},
 {id:'m3',name:'Paused thing',prompt:'Nothing yet.',execution_type:'manual',enabled:false,priority:'low',runner_type:'remote',remote_nest_id:'n1',remote_nest_name:'Nest One',remote_egg_id:'e1',remote_egg_name:'Egg A',status:'idle',run_count:0,preparation_status:'none'}
];
window.queue={items:[{mission_id:'m2'},{mission_id:'m3'}],running:'m2'};
window.runHistory={entries:[{id:'r1',mission_id:'m1',trigger_type:'scheduled',status:'success',output:'ok',started_at:new Date(now-3600e3).toISOString(),completed_at:new Date(now-3590e3).toISOString(),duration_ms:10000},{id:'r2',mission_id:'m1',trigger_type:'manual',status:'error',output:'Cancelled by user',started_at:new Date(now-7200e3).toISOString(),duration_ms:2000}],total:2,limit:25,offset:0};
const api=async(url,opts)=>{requests.push([opts&&opts.method||'GET',url,opts&&opts.body||null]);
 if(opts&&opts.method==='POST'&&url==='/api/missions/v2'){const m=Object.assign({id:'m4',status:'idle',run_count:0},JSON.parse(opts.body));window.missions.push(m);return m;}
 if(url==='/api/missions/v2')return {missions:window.missions,queue:window.queue};
 if(url.startsWith('/api/missions/v2/history?'))return window.runHistory;
 if(url==='/api/missions/v2/remote-targets')return {targets:[{nest_id:'n1',egg_id:'e1',nest_name:'Nest One',egg_name:'Egg A'}]};
 if(url.startsWith('/api/cheatsheets'))return [{id:'c1',name:'Docker',abstract:'Container basics'}];
 if(url==='/api/webhooks')return [{id:'w1',slug:'digest',name:'Digest hook'}];
 if(url.endsWith('/prepared'))return {analysis:{summary:'Plan summary',step_plan:[{step:1,action:'Check disk',expectation:'free space'}],essential_tools:[{tool_name:'shell',purpose:'run rsync'}],pitfalls:[{risk:'slow link',mitigation:'retry'}]},confidence:0.87};
 if(url.endsWith('/cancel'))return {status:'cancelling'};
 if(url.endsWith('/run'))return {status:'queued'};
 return {};};
const ctx={esc,t,api,notify:(m,k)=>toasts.push([m,k||'info']),readonly:false,iconMarkup:()=>'',setWindowMenus:(id,m)=>{window.menus=m;},clearWindowMenus:()=>{},showContextMenu:(x,y,items)=>contextMenus.push(items),wireContextMenuBoundary:()=>{},confirmDialog:async()=>window.confirmAnswer,promptDialog:async()=>'',setWindowBeforeClose:(id,fn)=>{window.beforeClose=fn;},isActive:()=>true,loadBootstrap:()=>{},updateWindowContext:()=>{}};
window.AuraSSE={handlers:{},on(n,f){this.handlers[n]=f;},off(n){delete this.handlers[n];},emit(n,p){this.handlers[n]&&this.handlers[n](p);}};
`+modules+`
`+shell+`
window.MissionControlApp.render(document.getElementById('win'),'w1',ctx);</script></body></html>`)
			})
			server := httptest.NewServer(mux)
			defer server.Close()
			page := browser.MustPage(server.URL + "/fixture").Timeout(60 * time.Second)
			defer page.Close()
			page.MustSetViewport(1120, 740, 1, false)
			page.MustWaitLoad()
			waitForJSBool(t, page, `()=>document.querySelectorAll('.vd-mc-row').length===3`)

			artifactDir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
			shoot := func(name string) {
				if artifactDir == "" {
					return
				}
				time.Sleep(300 * time.Millisecond)
				if err := os.MkdirAll(artifactDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(artifactDir, "mission-control-"+name+"-"+lang+".png"), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// List: sections, text safety, the first visible (running) mission is
			// auto-selected and its hero rendered; loading/empty placeholders are gone.
			shoot("overview")
			initial := page.MustEval(`()=>{
				const root=document.querySelector('.vd-mc');
				const visible=el=>!!el && el.getClientRects().length>0;
				const sections=[...root.querySelectorAll('.vd-mc-list [data-mc-section]')].map(s=>s.dataset.mcSection);
				const selected=root.querySelector('.vd-mc-row.is-selected');
				const hero=root.querySelector('.vd-mc-hero-title');
				return {
					safe: !window.pwned && root.querySelector('.vd-mc-list img')===null,
					sections: sections.join(','),
					selectedId: selected ? selected.dataset.mcId : '',
					heroText: hero ? hero.textContent : '',
					heroVisible: visible(hero),
					placeholdersHidden: !visible(root.querySelector('[data-mc-loading]')) && !visible(root.querySelector('[data-mc-loaderror]')) && !visible(root.querySelector('.vd-mc-detail-empty')) && !visible(root.querySelector('.vd-mc-editor')) && !visible(root.querySelector('[data-mc-back]')),
					counts: root.querySelector('[data-mc-status-counts]').textContent,
					menus: Array.isArray(window.menus) ? window.menus.length : -1
				};
			}`)
			if !initial.Get("safe").Bool() || initial.Get("sections").Str() != "running,waiting,missions" ||
				initial.Get("selectedId").Str() != "m2" || initial.Get("heroText").Str() != "Morning digest" ||
				!initial.Get("heroVisible").Bool() || !initial.Get("placeholdersHidden").Bool() ||
				initial.Get("counts").Str() == "" || initial.Get("menus").Int() != 2 {
				t.Fatalf("initial list/detail render failed: %s", initial.JSON("", "  "))
			}

			// Running mission (m2): Cancel button, progress strip, queued badge for m3.
			page.MustEval(`()=>document.querySelector('[data-mc-id="m2"]').click()`)
			waitForJSBool(t, page, `()=>document.querySelector('.vd-mc-hero-title').textContent==='Morning digest'`)
			if !page.MustEval(`()=>{
				const root=document.querySelector('.vd-mc');
				return root.querySelector('[data-mc-action="cancel"]')!==null
					&& !root.querySelector('[data-mc-progress]').hidden
					&& root.querySelector('[data-mc-id="m3"]').dataset.state==='queued';
			}`).Bool() {
				t.Fatal("running mission hero did not expose cancel/progress")
			}
			page.MustEval(`()=>document.querySelector('[data-mc-action="cancel"]').click()`)
			waitForJSBool(t, page, `()=>requests.some(r=>r[0]==='POST'&&r[1]==='/api/missions/v2/m2/cancel')`)
			shoot("running")

			// History tab on m1 loads the fake entries and shows the cancelled pill.
			page.MustEval(`()=>document.querySelector('[data-mc-id="m1"]').click()`)
			waitForJSBool(t, page, `()=>document.querySelector('.vd-mc-hero-title').textContent.includes('Backup')`)
			page.MustEval(`()=>document.querySelector('[data-mc-tab="history"]').click()`)
			waitForJSBool(t, page, `()=>document.querySelectorAll('.vd-mc-run').length===2`)
			if !page.MustEval(`()=>document.querySelector('.vd-mc-run .vd-mc-pill[data-state="cancelled"]')!==null && requests.some(r=>r[1].startsWith('/api/missions/v2/history?mission_id=m1'))`).Bool() {
				t.Fatal("history did not render the cancelled run")
			}
			shoot("history")

			// Prepared context card loads on demand.
			page.MustEval(`()=>document.querySelector('[data-mc-tab="overview"]').click()`)
			page.MustEval(`()=>document.querySelector('[data-mc-action="viewPrep"]').click()`)
			waitForJSBool(t, page, `()=>document.querySelector('.vd-mc-prep-steps')!==null`)

			// Context menu on a row is delivered through showContextMenu.
			page.MustEval(`()=>document.querySelector('[data-mc-id="m1"]').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:100,clientY:100}))`)
			if !page.MustEval(`()=>contextMenus.length===1 && contextMenus[0].length>5`).Bool() {
				t.Fatal("row context menu not shown")
			}

			// Editor: open, validation, schedule builder, save payload.
			page.MustEval(`()=>document.querySelector('[data-mc-new]').click()`)
			waitForJSBool(t, page, `()=>!document.querySelector('.vd-mc-editor').hidden && document.activeElement && document.activeElement.name==='name'`)
			page.MustEval(`()=>document.querySelector('[data-mc-editor-save]').click()`)
			waitForJSBool(t, page, `()=>document.querySelectorAll('.vd-mc-field.is-invalid').length===2`)
			shoot("editor-errors")
			page.MustEval(`()=>{
				const f=document.querySelector('.vd-mc-editor');
				f.elements.name.value='Weekly report';f.elements.name.dispatchEvent(new Event('input',{bubbles:true}));
				f.elements.prompt.value='Write the weekly report';f.elements.prompt.dispatchEvent(new Event('input',{bubbles:true}));
				f.querySelector('input[name="execution_type"][value="scheduled"]').click();
			}`)
			waitForJSBool(t, page, `()=>!document.querySelector('[data-mc-when="scheduled"]').hidden && document.querySelector('[data-mc-sched="mode"]')!==null`)
			page.MustEval(`()=>{const sel=document.querySelector('[data-mc-sched="mode"]');sel.value='weekly';sel.dispatchEvent(new Event('change',{bubbles:true}));}`)
			waitForJSBool(t, page, `()=>document.querySelectorAll('[data-mc-sched-day]').length===7`)
			page.MustEval(`()=>{document.querySelector('[data-mc-sched-day="5"]').click();}`)
			if !page.MustEval(`()=>{const code=document.querySelector('[data-mc-sched-preview] code').textContent;return code==='0 9 * * 1,5' && !document.querySelector('[data-mc-editor-unsaved]').hidden;}`).Bool() {
				t.Fatal("schedule builder did not produce the expected cron / dirty state")
			}
			shoot("editor")
			// Dirty guard: window close asks, selecting another mission asks.
			page.MustEval(`()=>{window.confirmAnswer=false;}`)
			if page.MustEval(`()=>window.beforeClose()`).Bool() {
				t.Fatal("beforeClose must refuse while the editor is dirty and the user declines")
			}
			page.MustEval(`()=>{window.confirmAnswer=true;document.querySelector('[data-mc-editor-save]').click();}`)
			waitForJSBool(t, page, `()=>document.querySelector('.vd-mc-editor').hidden && document.querySelector('.vd-mc-hero-title').textContent==='Weekly report'`)
			if !page.MustEval(`()=>{const r=requests.find(x=>x[0]==='POST'&&x[1]==='/api/missions/v2');const b=JSON.parse(r[2]);return b.schedule==='0 9 * * 1,5' && b.execution_type==='scheduled' && b.enabled===true && b.trigger_config===null;}`).Bool() {
				t.Fatal("save payload mismatch")
			}
			if !page.MustEval(`()=>toasts.some(x=>x[0]===t('desktop.mc_toast_created'))`).Bool() {
				t.Fatal("created toast missing")
			}

			// SSE update re-renders without losing selection.
			page.MustEval(`()=>{window.missions[0].last_result='error';AuraSSE.emit('mission_update',{missions:window.missions,queue:window.queue});}`)
			waitForJSBool(t, page, `()=>document.querySelector('[data-mc-id="m1"]').dataset.state==='error' && document.querySelector('.vd-mc-hero-title').textContent==='Weekly report'`)

			// Compact mode: narrow the window, list first, back button after selecting.
			page.MustEval(`()=>{document.getElementById('win').style.width='600px';}`)
			waitForJSBool(t, page, `()=>document.querySelector('.vd-mc').classList.contains('is-compact')`)
			page.MustEval(`()=>document.querySelector('[data-mc-id="m2"]').click()`)
			waitForJSBool(t, page, `()=>document.querySelector('.vd-mc').classList.contains('is-compact-detail') && !document.querySelector('[data-mc-back]').hidden`)
			shoot("compact")
			page.MustEval(`()=>document.querySelector('[data-mc-back]').click()`)
			waitForJSBool(t, page, `()=>!document.querySelector('.vd-mc').classList.contains('is-compact-detail')`)

			// No untranslated keys anywhere in the rendered app.
			if text := page.MustElement("#win").MustText(); strings.Contains(text, "desktop.mc_") || strings.Contains(text, "missions.") {
				t.Fatalf("untranslated text in %s: %q", lang, text)
			}
			// Dispose removes listeners and SSE handler.
			page.MustEval(`()=>window.MissionControlApp.dispose('w1')`)
			if !page.MustEval(`()=>!AuraSSE.handlers.mission_update`).Bool() {
				t.Fatal("dispose left the SSE handler registered")
			}
		})
	}
}

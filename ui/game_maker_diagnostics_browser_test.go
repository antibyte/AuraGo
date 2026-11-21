package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGameMakerDiagnosticFramesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html>
<iframe id="game" sandbox="allow-scripts" srcdoc="<script>addEventListener('message',e=>parent.postMessage(e.data,'*'))</script>"></iframe>
<script src="/js/desktop/apps/game-maker-studio-preview.js"></script><script>
window.reports=[];window.state={frame:document.getElementById('game'),project:{id:'p'},previewProjectID:'p',channelID:'channel',
job:{status:'building'},previewGrant:{token:'parent-only',validation_id:'build',expires_at:new Date(Date.now()+60000).toISOString()},previewReported:new Set(),previewDiagnostics:[],
addDiagnostic:diagnostic=>reports.push(diagnostic)};
addEventListener('message',e=>GameMakerStudioPreview.handleMessage(state,e));
window.send=(message,channel='channel')=>state.frame.contentWindow.postMessage({source:'aurago-game',type:'runtime_error',channel,message,
frames:[{file:'dist/game.js',line:10,column:3,secret:'dropped'},{file:'private.txt',line:1,column:1},{file:'dist/game.js',line:-1,column:1},
{file:'dist/game.js',line:12,column:5},{file:'dist/game.js',line:1e99,column:1},{file:'dist/game.js',line:20,column:4}]},'*');
</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(15 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustEval(`()=>send('first')`)
	page.MustWait(`()=>reports.length===1`)
	if !page.MustEval(`()=>reports[0].message==='first'&&reports[0].level==='runtime'&&!JSON.stringify(reports).includes('parent-only')`).Bool() {
		t.Fatal("missing local diagnostic or leaked credential", page.MustEval(`()=>reports`))
	}
	page.MustEval(`()=>{send('wrong channel','wrong');window.postMessage({source:'aurago-game',channel:'channel',type:'runtime_error',message:'wrong source'},'*');send('first')}`)
	// Two animation frames let queued cross-frame messages complete.
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if page.MustEval(`()=>reports.length`).Int() != 1 {
		t.Fatal("stale/foreign report accepted")
	}
}

func TestGameMakerDiagnosticsFixActionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/preview-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body style="margin:0;background:#16324f"><script>
const channel = decodeURIComponent((location.hash.split('#gm-channel=')[1] || '').split('&')[0]);
window.addEventListener('message', e => {
	if (e.data === 'trigger-runtime-error') {
		parent.postMessage({ source: 'aurago-game', type: 'runtime_error', channel, message: 'Uncaught TypeError: (intermediate value).makeRotation is not a function' }, '*');
	}
});
</script></body></html>`)
	})
	mux.HandleFunc("/api/game-maker/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": connected\n\nevent: stream_ready\ndata: {\"type\":\"stream_ready\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
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
window.fixtureStartCalls=[];
window.fixtureProject={id:'pool',name:'pool 3d',description:'Pool',dimension:'3d',status:'ready',current_revision:1,project_key:'Games/pool',provider_id:'main',model:'m',updated_at:new Date().toISOString()};
window.fixtureCapabilities={enabled:true,readonly:false,allow_create:true,allow_edit:true,allow_delete:true,allow_media_generation:true,skills_ready:true,code_studio:false,
 phaser_version:'4.2.1',three_version:'0.186.1',voxel_version:1,default_provider_id:'main',default_model:'m',providers:[{id:'main',name:'Main',model:'m'}],active_job:null,skills:[]};
fetch('/lang/desktop/de.json').then(r=>r.json()).then(lang=>{window.fixtureLang=lang;GameMakerStudioApp.render(document.getElementById('app'),'diag-fix',{
 esc:value=>String(value??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),
 t:(key,args)=>{let value=lang[key]||key;for(const [k,v] of Object.entries(args||{}))value=value.replaceAll('{{'+k+'}}',v).replaceAll('{'+k+'}',v);return value},
 confirmDialog:async()=>true,promptDialog:async(_t,v)=>v,notify:()=>{},openApp:()=>{},
 api:async(path,options)=>{
  if(path.endsWith('/capabilities'))return fixtureCapabilities;
  if(path.endsWith('/projects')&&(!options||!options.method||options.method==='GET'))return {projects:[fixtureProject]};
  if(path.endsWith('/preview-token'))return {url:'/preview-fixture',token:'fixture',revision:1};
  if(path.endsWith('/workspace-drift'))return {revision:1,changed:[],added:[],removed:[],changed_count:0,added_count:0,removed_count:0,truncated:false};
  if(path.endsWith('/jobs')){const body=JSON.parse(options.body||'{}');fixtureStartCalls.push(body);return {id:'job-fix',status:'queued',phase:'queued',prompt:body.prompt};}
  return {project:fixtureProject,messages:[]};
 }});window.fixtureReady=true;});
</script></body></html>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(30 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustSetViewport(1280, 800, 1, false)
	page.MustWait(`()=>window.fixtureReady&&GameMakerStudioApp.instances.get('diag-fix')?.project?.id==='pool'&&!!document.querySelector('.gm-preview-frame')`)
	page.MustWait(`()=>!document.querySelector('[data-gm-preview-loading]')||document.querySelector('[data-gm-preview-loading]').hidden`)

	if !page.MustEval(`()=>document.querySelector('[data-gm-fix-diagnostics]').hidden===true`).Bool() {
		t.Fatal("fix button should be hidden before errors occur")
	}

	page.MustEval(`()=>{GameMakerStudioApp.instances.get('diag-fix').addDiagnostic({level:'info',message:'just info'});}`)
	if !page.MustEval(`()=>document.querySelector('[data-gm-fix-diagnostics]').hidden===true`).Bool() {
		t.Fatal("fix button should stay hidden for info diagnostics")
	}

	page.MustEval(`()=>{document.querySelector('.gm-preview-frame').contentWindow.postMessage('trigger-runtime-error','*');}`)
	page.MustWait(`()=>!document.querySelector('[data-gm-fix-diagnostics]').hidden`)

	if !page.MustEval(`()=>{
		const details=document.querySelector('[data-gm-diagnostics]');
		const btn=document.querySelector('[data-gm-fix-diagnostics]');
		const studio=document.querySelector('.gm-studio');
		return details.open===true && details.classList.contains('has-errors') && !btn.disabled && btn.scrollWidth<=btn.clientWidth+2 && studio.scrollWidth<=studio.clientWidth+1;
	}`).Bool() {
		t.Fatal("fix button or diagnostics drawer in unexpected state after runtime error")
	}

	page.MustEval(`()=>{document.querySelector('[data-gm-change-form] textarea').value='keep my draft';}`)
	page.MustElement(`[data-gm-fix-diagnostics]`).MustClick()
	page.MustWait(`()=>fixtureStartCalls.length===1`)

	raw := page.MustEval(`()=>JSON.stringify({call:fixtureStartCalls[0],open:document.querySelector('[data-gm-diagnostics]').open,disabled:document.querySelector('[data-gm-fix-diagnostics]').disabled,draft:document.querySelector('[data-gm-change-form] textarea').value})`).Str()
	var got struct {
		Call struct {
			Prompt             string `json:"prompt"`
			Resume             bool   `json:"resume"`
			PreviewDiagnostics []struct {
				Level   string `json:"level"`
				Message string `json:"message"`
			} `json:"preview_diagnostics"`
		} `json:"call"`
		Open     bool   `json:"open"`
		Disabled bool   `json:"disabled"`
		Draft    string `json:"draft"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	const wantErr = "Uncaught TypeError: (intermediate value).makeRotation is not a function"
	if got.Call.Resume {
		t.Fatalf("expected resume=false, got true")
	}
	if got.Call.Prompt == "" || len(got.Call.PreviewDiagnostics) != 1 || got.Call.PreviewDiagnostics[0].Message != wantErr {
		t.Fatalf("unexpected startJob payload: %+v", got.Call)
	}
	if !got.Open {
		t.Fatal("clicking fix button should not toggle diagnostics details closed")
	}
	if !got.Disabled {
		t.Fatal("fix button should disable while repair job is active")
	}
	if got.Draft != "keep my draft" {
		t.Fatalf("unrelated draft overwritten: %q", got.Draft)
	}
}

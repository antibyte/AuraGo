package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGameMakerVoxelBridgeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html>
<iframe id="game" sandbox="allow-scripts" srcdoc="<script>addEventListener('message',e=>{if(e.data.test)parent.postMessage(e.data,'*');else parent.postMessage({reply:e.data},'*')})</script>"></iframe>
<script src="/js/desktop/apps/game-maker-studio-preview.js"></script><script>
window.calls=[];window.replies=[];window.recoveries=[];window.state={frame:document.getElementById('game'),project:{id:'p',variant:'voxel',current_revision:1},previewProjectID:'p',channelID:'bound',previewGrant:{play_token:'parent-only',revision:1},resolvePlayStateRevisionConflict:async()=>recoveries.push(state.playStateBusy),api:{playState:async(...args)=>{calls.push(args);if(window.conflict)throw {code:'conflict'};return {version:1,state:null}}}};
addEventListener('message',e=>{if(e.data.reply){replies.push(e.data.reply);return}GameMakerStudioPreview.handleMessage(state,e)});
window.send=(change={})=>state.frame.contentWindow.postMessage({test:true,source:'aurago-voxel',type:'play_state',channel:'bound',request:crypto.randomUUID(),operation:'load',version:0,...change},'*');</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(15 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustEval(`()=>send()`)
	page.MustWait(`()=>calls.length===1&&replies.length===1`)
	if !page.MustEval(`()=>calls[0][0]==='p'&&calls[0][1]==='parent-only'&&!JSON.stringify(replies).includes('parent-only')`).Bool() {
		t.Fatal("save binding or credential leakage")
	}
	page.MustEval(`()=>{send({channel:'wrong'});window.postMessage({source:'aurago-voxel',type:'play_state',channel:'bound',request:'x',operation:'load',version:0},'*');send({operation:'delete-project'});state.previewProjectID='other';send()}`)
	page.MustEval(`()=>new Promise(r=>setTimeout(r,100))`)
	if page.MustEval(`()=>calls.length`).Int() != 1 {
		t.Fatal("foreign operation/frame/project accepted")
	}
	page.MustEval(`()=>{state.previewProjectID='p';state.previewGrant.validation_id='test';send()}`)
	page.MustWait(`()=>replies.length===2`)
	if !page.MustEval(`()=>replies[1].result.temporary&&calls.length===1`).Bool() {
		t.Fatal("validation received a stored save")
	}
	page.MustEval(`()=>send({operation:'save',payload:{}})`)
	page.MustWait(`()=>replies.length===3`)
	if !page.MustEval(`()=>replies[2].error&&calls.length===1`).Bool() {
		t.Fatal("validation wrote a save")
	}
	page.MustEval(`()=>{delete state.previewGrant.validation_id;window.conflict=true;send({operation:'save',payload:{}})}`)
	page.MustWait(`()=>replies.length===4`)
	if !page.MustEval(`()=>replies[3].error==='conflict'&&recoveries.length===0`).Bool() {
		t.Fatal("save conflict hidden")
	}
	page.MustEval(`()=>send({operation:'save',payload:'x'.repeat(4*1024*1024)})`)
	page.MustWait(`()=>replies.length===5`)
	if page.MustEval(`()=>calls.length`).Int() != 2 {
		t.Fatal("oversized save forwarded")
	}
	page.MustEval(`()=>send({operation:'load'})`)
	page.MustWait(`()=>replies.length===6&&recoveries.length===1`)
	if !page.MustEval(`()=>replies[5].error==='conflict'&&recoveries[0]===false`).Bool() {
		t.Fatal("revision-invalid GET did not recover after releasing the bridge lock")
	}
	page.MustEval(`()=>{window.conflict=false;state.playStateRevisionInvalid=false;state.project.current_revision=2;send({operation:'load'})}`)
	page.MustWait(`()=>replies.length===7&&recoveries.length===2`)
	if !page.MustEval(`()=>replies[6].error==='conflict'&&recoveries[1]===false`).Bool() {
		t.Fatal("locally stale revision did not request host recovery")
	}
}

func TestGameMakerVoxelCreateBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><link rel="stylesheet" href="/css/desktop-app-game-maker-studio.css"><div id="app"></div>
<script src="/js/desktop/apps/game-maker-studio-modals.js"></script><script src="/js/desktop/apps/game-maker-studio-api.js"></script><script src="/js/desktop/apps/game-maker-studio-preview.js"></script><script src="/js/desktop/apps/game-maker-studio.js"></script><script>
window.errors=[];window.creates=[];window.confirmations=[];addEventListener('error',e=>errors.push(e.message));
GameMakerStudioApp.render(document.getElementById('app'),'voxel',{esc:v=>String(v??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),t:k=>k,setWindowBeforeClose:(id,f)=>window.closeGuard=f,confirmDialog:async(...args)=>{confirmations.push(args);return true},api:async(path,opts)=>{
if(path.endsWith('/capabilities'))return {enabled:true,allow_create:true,allow_edit:true,skills_ready:true,three_version:'0.186.1',voxel_version:1,providers:[{id:'local',model:'test'}],default_provider_id:'local',default_model:'test'};
if(path.endsWith('/projects')&&opts.method==='POST'){creates.push(JSON.parse(opts.body));throw Error('End of local creation fixture')}
if(path.endsWith('/projects'))return {projects:[]};return {};
}});</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(12 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustWait(`()=>!document.querySelector('[data-gm-action="new"]').disabled`)
	page.MustElement(`[data-gm-action="new"]`).MustClick()
	page.MustElement(`input[name="name"]`).MustInput("Voxel island")
	page.MustElement(`[data-idea="14"]`).MustClick()
	if !page.MustEval(`()=>document.querySelector('input[value="voxel"]').checked`).Bool() {
		t.Fatal("voxel idea did not choose its variant")
	}
	page.MustElement(`[data-gm-create] button[type="submit"]`).MustClick()
	page.MustWait(`()=>creates.length===1`)
	if !page.MustEval(`()=>creates[0].dimension==='3d'&&creates[0].variant==='voxel'&&creates[0].provider_id==='local'`).Bool() {
		t.Fatal("creation lost variant or provider")
	}
	// The shell waits for save acknowledgement; explicit discard remains possible.
	page.MustEval(`()=>{GameMakerStudioPreview.flush=async()=>false;}`)
	if !page.MustEval(`async()=>await closeGuard()&&confirmations.length===1&&confirmations[0][1]==='game_maker.leave_unsaved'`).Bool() {
		t.Fatal("failed save closed without explicit discard")
	}
	page.MustEval(`()=>GameMakerStudioApp.dispose('voxel')`)
	if !page.MustEval(`()=>closeGuard===null&&errors.length===0`).Bool() {
		t.Fatal("close lifecycle leaked or failed")
	}
}

func TestGameMakerRevisionModalRestorePreflushAndReadonlyBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/js/", http.StripPrefix("/", http.FileServer(http.Dir("."))))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><div id="host"><div data-gm-modal hidden></div></div>
<script src="/js/desktop/apps/game-maker-studio-modals.js"></script><script>
window.calls=[];window.state={container:document.getElementById('host'),project:{id:'p',current_revision:2},capabilities:{allow_edit:false},disposed:false,
 context:{esc:v=>String(v??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),t:k=>k,confirmDialog:async()=>{calls.push('confirm');return true}},
 api:{revisions:async id=>({revisions:[{number:1,source:'test',summary:'old',file_count:1},{number:2,source:'test',summary:'current',file_count:1}]}),restore:async(id,rev)=>calls.push('restore:'+id+':'+rev)},
 preparePreviewReplacement:async()=>{calls.push('flush');return true},reloadProjectRecord:async()=>{calls.push('reload');return true},refreshPreview:async options=>calls.push('refresh:'+options.skipFlush),fail:error=>calls.push('fail:'+error.message)};
const helpers={showModal:GameMakerStudioModals.showModal,closeModal:GameMakerStudioModals.closeModal,setModalBusy:GameMakerStudioModals.setModalBusy,modalError:GameMakerStudioModals.modalError,confirmAction:GameMakerStudioModals.confirmAction};
window.openRevisions=()=>GameMakerStudioModals.showRevisionsModal(state,helpers);
</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(12 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustEval(`()=>openRevisions()`)
	page.MustWait(`()=>!!document.querySelector('[data-restore="1"]')`)
	if !page.MustEval(`()=>{const b=document.querySelector('[data-restore="1"]');return b.disabled&&document.querySelector('.gm-readonly-notice')?.textContent==='game_maker.readonly_notice'&&b.getAttribute('aria-label')?.includes('game_maker.readonly_notice')}`).Bool() {
		t.Fatal("read-only revision history did not explain and disable restore")
	}
	page.MustEval(`()=>{document.querySelector('[data-restore="1"]').click();}`)
	if page.MustEval(`()=>calls.length`).Int() != 0 {
		t.Fatal("read-only restore reached a mutation or confirmation")
	}
	page.MustEval(`()=>{GameMakerStudioModals.closeModal(state);state.capabilities.allow_edit=true;openRevisions();}`)
	page.MustWait(`()=>document.querySelector('[data-restore="1"]')&&!document.querySelector('[data-restore="1"]').disabled`)
	page.MustElement(`[data-restore="1"]`).MustClick()
	page.MustWait(`()=>calls.includes('refresh:true')`)
	if !page.MustEval(`()=>calls.join(',')==='confirm,flush,restore:p:1,reload,refresh:true'`).Bool() {
		t.Fatalf("restore did not flush before mutation and refresh: %s", page.MustEval(`()=>calls.join(',')`).Str())
	}
}

func TestGameMakerPlayerRevisionConflictReloadsTrustedHostBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var projectReads, grants, playLoads atomic.Int32
	var lastProjectRevision, lastGrantRevision atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("/js/", http.StripPrefix("/", http.FileServer(http.Dir("."))))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><p id="status" role="status"></p><main></main>
<script id="game-player-data" type="application/json">{"projectID":"p","i18n":{"game_maker":{"preview_loading":"Loading","preview_timeout":"Unavailable"}}}</script>
<script src="/js/desktop/apps/game-maker-studio-api.js"></script><script src="/js/desktop/apps/game-maker-studio-preview.js"></script><script src="/js/desktop/apps/game-maker-player.js"></script>`)
	})
	mux.HandleFunc("/api/game-maker/projects/p", func(w http.ResponseWriter, r *http.Request) {
		revision := 1
		if playLoads.Load() > 0 {
			revision = 2
		}
		projectReads.Add(1)
		lastProjectRevision.Store(int32(revision))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"project":{"id":"p","name":"Test player","variant":"voxel","dimension":"3d","current_revision":%d}}`, revision)
	})
	mux.HandleFunc("/api/game-maker/projects/p/preview-token", func(w http.ResponseWriter, r *http.Request) {
		revision := 1
		if projectReads.Load() > 1 {
			revision = 2
		}
		grants.Add(1)
		lastGrantRevision.Store(int32(revision))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"url":"/frame","token":"preview-only","play_token":"parent-only","revision":%d}`, revision)
	})
	mux.HandleFunc("/api/game-maker/projects/p/play-state", func(w http.ResponseWriter, r *http.Request) {
		if playLoads.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"error":"revision conflict"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"version":0,"state":null}`)
	})
	mux.HandleFunc("/frame", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><script>
const channel=new URLSearchParams(location.hash.slice(1)).get('gm-channel');
addEventListener('message',e=>{if(e.data?.type==='flush')parent.postMessage({source:'aurago-voxel',type:'flushed',channel:e.data.channel,ok:false},'*')});
parent.postMessage({source:'aurago-voxel',type:'play_state',channel,request:'load-'+Math.random(),operation:'load',version:0},'*');
</script>`)
	})
	mux.HandleFunc("/counts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"projects":%d,"grants":%d,"loads":%d,"project_revision":%d,"grant_revision":%d}`,
			projectReads.Load(), grants.Load(), playLoads.Load(), lastProjectRevision.Load(), lastGrantRevision.Load())
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(15 * time.Second).MustWaitLoad()
	defer page.Close()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(`async()=>{const c=await(await fetch('/counts')).json();return c.projects>=2&&c.grants>=2&&c.loads>=2&&c.project_revision===2&&c.grant_revision===2}`).Bool() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !page.MustEval(`async()=>{const c=await(await fetch('/counts')).json();return c.projects===2&&c.grants===2&&c.loads===2&&c.project_revision===2&&c.grant_revision===2}`).Bool() {
		t.Fatalf("stale revision did not reload the player with a fresh host grant: counts=%s status=%s body=%s", page.MustEval(`async()=>JSON.stringify(await(await fetch('/counts')).json())`).Str(), page.MustElement("#status").MustText(), page.MustElement("body").MustText())
	}
}

func TestGameMakerVoxelExternalRevisionResolvesUnsavedFrameBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	publish := make(chan struct{})
	releaseOldRead := make(chan struct{})
	var releaseOnce sync.Once
	releaseOld := func() { releaseOnce.Do(func() { close(releaseOldRead) }) }
	var readOnce atomic.Bool
	var oldReadFinished atomic.Bool
	var oldReadCanceled atomic.Bool
	oldReadDone := make(chan struct{})
	var oldReadDoneOnce sync.Once
	var loads atomic.Int32
	var eventRequests atomic.Int32
	var publishSent atomic.Bool
	mux := http.NewServeMux()
	mux.Handle("/js/", http.StripPrefix("/", http.FileServer(http.Dir("."))))
	mux.HandleFunc("/frame", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><script>
const channel=new URLSearchParams(location.hash.slice(1)).get('gm-channel');
addEventListener('message',e=>{if(e.data?.type==='flush')parent.postMessage({source:'aurago-voxel',type:'flushed',channel:e.data.channel,ok:false},'*')});
parent.postMessage({source:'aurago-voxel',type:'play_state',channel,request:'load-'+Math.random(),operation:'load',version:0},'*');
</script>`)
	})
	mux.HandleFunc("/api/game-maker/projects/p/events", func(w http.ResponseWriter, r *http.Request) {
		eventRequests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		select {
		case <-publish:
			fmt.Fprint(w, "id: 1\nevent: preview_reload\ndata: {\"type\":\"preview_reload\",\"payload\":{\"revision\":2}}\n\n")
			fmt.Fprint(w, "id: 2\nevent: revision\ndata: {\"type\":\"revision\",\"payload\":{\"revision\":{\"number\":2}}}\n\n")
			w.(http.Flusher).Flush()
			publishSent.Store(true)
		case <-r.Context().Done():
			return
		}
		<-r.Context().Done()
	})
	mux.HandleFunc("/api/game-maker/projects/p/play-state", func(w http.ResponseWriter, r *http.Request) {
		loadNumber := loads.Add(1)
		if loadNumber == 1 {
			readOnce.Store(true)
			<-releaseOldRead
			oldReadCanceled.Store(r.Context().Err() != nil)
			oldReadFinished.Store(true)
			oldReadDoneOnce.Do(func() { close(oldReadDone) })
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"error":"published revision changed"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"version":0,"state":null}`)
	})
	mux.HandleFunc("/counts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"loads":%d,"old_read_finished":%t,"event_requests":%d,"publish_sent":%t}`, loads.Load(), oldReadFinished.Load(), eventRequests.Load(), publishSent.Load())
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><div id="app"></div><script>
window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message||String(e.error||'error')));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason||'rejection')));
</script>
<script src="/js/desktop/apps/game-maker-studio-modals.js"></script><script src="/js/desktop/apps/game-maker-studio-api.js"></script><script src="/js/desktop/apps/game-maker-studio-preview.js"></script><script src="/js/desktop/apps/game-maker-studio.js"></script><script>
window.project={id:'p',name:'Voxel',description:'Voxel world',variant:'voxel',dimension:'3d',status:'ready',current_revision:1};window.revision=1;window.confirmations=[];window.trace=[];
const originalFlush=GameMakerStudioPreview.flush;GameMakerStudioPreview.flush=async state=>{trace.push('flush');return originalFlush(state)};
GameMakerStudioApp.render(document.getElementById('app'),'fixture',{esc:v=>String(v??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),t:k=>k,
 confirmDialog:async(...args)=>{trace.push('confirm');confirmations.push(args);return true},
 api:async(path,opts)=>{
  if(path.endsWith('/capabilities'))return {enabled:true,allow_create:true,allow_edit:true,allow_delete:true,skills_ready:true,voxel_version:1,three_version:'0.186.1'};
  if(path==='/api/game-maker/projects')return {projects:[project]};
  if(path.endsWith('/projects/p/events'))return {};
  if(path.endsWith('/projects/p/preview-token')){trace.push('grant');return {url:'/frame',token:'host-only',play_token:'parent-only',revision};}
  if(path.endsWith('/projects/p')){trace.push('getProject');project={...project,current_revision:revision};return {project,messages:[]};}
  return {};
 }});
</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	defer releaseOld()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(20 * time.Second).MustWaitLoad()
	defer page.Close()
	ready := false
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(`async()=>{const c=await(await fetch('/counts')).json();return c.loads===1&&GameMakerStudioApp.instances.get('fixture')?.previewGrant?.revision===1}`).Bool() {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("initial preview did not become ready: counts=%s state=%s body=%s errors=%s scripts=%s",
			page.MustEval(`async()=>JSON.stringify(await(await fetch('/counts')).json())`).Str(),
			page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');return JSON.stringify(s&&{project:s.project,grant:s.previewGrant,frame:s.frame?.src,channel:s.channelID,disposed:s.disposed})}`).Str(),
			page.MustElement("body").MustText(),
			page.MustEval(`()=>JSON.stringify(window.fixtureErrors||[])`).Str(),
			page.MustEval(`()=>JSON.stringify(performance.getEntriesByType('resource').filter(x=>x.name.includes('/js/')).map(x=>({name:x.name,status:x.responseStatus,duration:x.duration})))`).Str())
	}
	if !readOnce.Load() {
		t.Fatal("old revision read did not start")
	}
	page.MustEval(`()=>{window.trace.length=0;window.revision=2;}`)
	close(publish)
	updated := false
	deadline = time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(`async()=>{const c=await(await fetch('/counts')).json();return c.loads>=2&&GameMakerStudioApp.instances.get('fixture')?.project?.current_revision===2}`).Bool() {
			updated = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !updated {
		t.Fatalf("external revision event did not refresh preview: counts=%s state=%s trace=%s errors=%s body=%s",
			page.MustEval(`async()=>JSON.stringify(await(await fetch('/counts')).json())`).Str(),
			page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');return JSON.stringify(s&&{project:s.project,grant:s.previewGrant,frame:s.frame?.src,channel:s.channelID,eventReady:s.eventSource?.readyState,eventURL:s.eventSource?.url,refresh:s.revisionRefreshIdentity&&{projectID:s.revisionRefreshIdentity.projectID}})}`).Str(),
			page.MustEval(`()=>trace.join(',')`).Str(),
			page.MustEval(`()=>JSON.stringify(window.fixtureErrors||[])`).Str(),
			page.MustElement("body").MustText())
	}
	if !page.MustEval(`()=>confirmations.length===1&&trace[0]==='flush'&&trace[1]==='confirm'&&trace.indexOf('getProject')>1&&trace.indexOf('grant')>trace.indexOf('getProject')`).Bool() {
		t.Fatalf("external revision was not confirmed before replacing with a fresh frame: trace=%s confirmations=%s", page.MustEval(`()=>trace.join(',')`).Str(), page.MustEval(`()=>JSON.stringify(confirmations)`).Str())
	}
	if !page.MustEval(`async()=>{const c=await(await fetch('/counts')).json();return c.loads===2&&GameMakerStudioApp.instances.get('fixture')?.previewGrant?.revision===2&&trace.filter(x=>x==='grant').length===1}`).Bool() {
		t.Fatal("new frame could not load while the old revision-bound request was still pending")
	}
	page.MustEval(`()=>window.frameBeforeOld409=GameMakerStudioApp.instances.get('fixture').frame`)
	releaseOld()
	select {
	case <-oldReadDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("old load handler did not finish after release: counts=%s", page.MustEval(`async()=>JSON.stringify(await(await fetch('/counts')).json())`).Str())
	}
	if oldReadCanceled.Load() {
		t.Log("old request cancelled; no late409 delivery claimed")
	}
	oldResponse := false
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(`async()=>{const c=await(await fetch('/counts')).json();return c.loads===2&&c.old_read_finished===true}`).Bool() {
			oldResponse = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !oldResponse {
		t.Fatalf("delayed 409 response was not recorded: counts=%s", page.MustEval(`async()=>JSON.stringify(await(await fetch('/counts')).json())`).Str())
	}
	if !page.MustEval(`()=>GameMakerStudioApp.instances.get('fixture').frame===window.frameBeforeOld409&&confirmations.length===1&&trace.filter(x=>x==='grant').length===1`).Bool() {
		t.Fatal("late conflict from the replaced frame triggered a second recovery")
	}
}

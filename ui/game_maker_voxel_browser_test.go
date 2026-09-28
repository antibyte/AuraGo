package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
window.calls=[];window.replies=[];window.state={frame:document.getElementById('game'),project:{id:'p',variant:'voxel',current_revision:1},previewProjectID:'p',channelID:'bound',previewGrant:{play_token:'parent-only',revision:1},api:{playState:async(...args)=>{calls.push(args);if(window.conflict)throw {code:'conflict'};return {version:1,state:null}}}};
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
	if !page.MustEval(`()=>replies[3].error==='conflict'`).Bool() {
		t.Fatal("save conflict hidden")
	}
	page.MustEval(`()=>send({operation:'save',payload:'x'.repeat(4*1024*1024)})`)
	page.MustWait(`()=>replies.length===5`)
	if page.MustEval(`()=>calls.length`).Int() != 2 {
		t.Fatal("oversized save forwarded")
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
if(path.endsWith('/capabilities'))return {enabled:true,allow_create:true,allow_edit:true,skills_ready:true,three_version:'0.185.1',voxel_version:1,providers:[{id:'local',model:'test'}],default_provider_id:'local',default_model:'test'};
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

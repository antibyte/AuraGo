package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopSessionActionsEditorDecisionsBrowser(t *testing.T) {
	var writes atomic.Int32
	var failWrite atomic.Bool
	var saved atomic.Value
	saved.Store("")
	fileAPI := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			if strings.Contains(r.URL.Query().Get("path"), "/untitled-") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			_ = json.NewEncoder(w).Encode(map[string]string{"content": "baseline", "version": `"v1"`})
			return
		}
		if r.Method != http.MethodPut {
			http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		writes.Add(1)
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
			return
		}
		if failWrite.Load() {
			http.Error(w, `{"error":"write_failed"}`, http.StatusInternalServerError)
			return
		}
		saved.Store(body.Content)
		_ = json.NewEncoder(w).Encode(map[string]string{"path": "Documents/session.txt", "version": `"v2"`})
	})
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/file": fileAPI})
	page.MustEval(`()=>{
		window.sessionLogoutCalls=0;window.sessionProblems=[];window.includeVirtual=false;window.virtualSaves=0;
		AuraDesktopSessionActions.configure({
			canLogout:()=>true,
			logout:async()=>{window.sessionLogoutCalls++;return true;},
			onProblem:async problem=>{window.sessionProblems.push(problem.code);}
		});
		window.sessionWindows=()=>[...auditTest.state.windows.values()].map(win=>({id:win.id,title:win.title,appId:win.appId,beforeClose:win.beforeClose}));
		AuraDesktopSessionActions.configure({getWindows:()=>sessionWindows().concat(includeVirtual?[{id:'zz-session-cancel',title:'Later window',appId:'fixture'}]:[])});
		auditTest.openApp('editor',{path:'Documents/session.txt'});
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-editor textarea')&&!document.querySelector('.vd-editor textarea').readOnly`)
	if got := page.MustEval(`()=>document.querySelector('.vd-editor textarea').value`).Str(); got != "baseline" {
		t.Fatalf("editor fixture loaded %q", got)
	}
	page.MustEval(`()=>{
		const editorID=[...auditTest.state.windows.values()].find(win=>win.appId==='editor').id;
		window.includeVirtual=true;
		AuraDesktopSessionActions.register('zz-session-cancel',{
			inspect:()=>({dirty:true,revision:1,label:'Later document'}),
			decide:async()=>{window.virtualDecisionCount=(window.virtualDecisionCount||0)+1;return 'cancel';},
			save:async()=>{window.virtualSaves++;return {revision:1};}
		});
		window.sessionFirstID=editorID;
		const input=document.querySelector('.vd-editor textarea');input.value='edit remains after cancel';input.dispatchEvent(new Event('input',{bubbles:true}));
		window.sessionAttempt=AuraDesktopSessionActions.requestLogout().then(value=>{window.sessionAttemptResult=value;});
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="save"]')`)
	if count := page.MustEval(`()=>document.querySelectorAll('.vd-modal [data-cancel],.vd-modal [data-choice]').length`).Int(); count != 3 {
		t.Fatalf("session choice modal has %d actions, want Save, Without saving, and Cancel", count)
	}
	page.MustElement(`.vd-modal [data-choice="save"]`).MustClick()
	page.MustWait(`()=>window.sessionAttemptResult===false`)
	if page.MustEval(`()=>virtualDecisionCount===1&&virtualSaves===0&&sessionLogoutCalls===0`).Bool() == false {
		t.Fatal("a later cancellation did not prevent every earlier save and logout")
	}
	if writes.Load() != 0 {
		t.Fatal("editor persisted content before all windows agreed to logout")
	}
	page.MustEval(`()=>{includeVirtual=false;AuraDesktopSessionActions.register('zz-session-cancel',null);window.sessionAttemptResult=null;window.sessionAttempt=AuraDesktopSessionActions.requestLogout().then(value=>{window.sessionAttemptResult=value;});}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-cancel]')`)
	page.MustElement(`.vd-modal [data-cancel]`).MustClick()
	page.MustWait(`()=>window.sessionAttemptResult===false`)
	if writes.Load() != 0 || page.MustEval(`()=>sessionLogoutCalls===0&&document.querySelector('.vd-editor textarea')?.value==='edit remains after cancel'`).Bool() == false {
		t.Fatal("Cancel changed the draft, wrote it, or closed the editor")
	}

	page.MustEval(`()=>{window.sessionAttemptResult=null;window.sessionAttempt=AuraDesktopSessionActions.requestLogout().then(value=>{window.sessionAttemptResult=value;});}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="save"]')`)
	page.MustElement(`.vd-modal [data-choice="save"]`).MustClick()
	page.MustWait(`()=>window.sessionAttemptResult===true`)
	if writes.Load() != 1 || saved.Load() != "edit remains after cancel" || page.MustEval(`()=>sessionLogoutCalls===1`).Bool() == false {
		t.Fatal("successful editor Save did not persist the revision before logout")
	}

	failWrite.Store(true)
	page.MustEval(`()=>{const input=document.querySelector('.vd-editor textarea');input.value='failed write stays open';input.dispatchEvent(new Event('input',{bubbles:true}));window.sessionAttemptResult=null;window.sessionAttempt=AuraDesktopSessionActions.requestLogout().then(value=>{window.sessionAttemptResult=value;});}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="save"]')`)
	page.MustElement(`.vd-modal [data-choice="save"]`).MustClick()
	page.MustWait(`()=>window.sessionAttemptResult===false`)
	if writes.Load() != 2 || page.MustEval(`()=>sessionLogoutCalls===1&&sessionProblems.includes('save_failed')&&document.querySelector('.vd-editor textarea')?.value==='failed write stays open'`).Bool() == false {
		t.Fatal("failed editor save did not leave the open document available for recovery")
	}
	failWrite.Store(false)
	page.MustEval(`()=>{
		const host=document.querySelector('.vd-editor');
		const input=host.querySelector('textarea');input.value='unsaved before New';input.dispatchEvent(new Event('input',{bubbles:true}));
		host.querySelector('[data-load-actions]').hidden=false;
		host.querySelector('[data-new]').click();
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="save"]')`)
	page.MustElement(`.vd-modal [data-cancel]`).MustClick()
	page.MustWait(`()=>!document.querySelector('.vd-modal')`)
	if writes.Load() != 2 || !page.MustEval(`()=>document.querySelector('.vd-editor textarea')?.value==='unsaved before New'`).Bool() {
		t.Fatal("cancelling New discarded the current Editor draft")
	}
	page.MustEval(`()=>{document.querySelector('.vd-editor [data-load-actions]').hidden=false;document.querySelector('.vd-editor [data-new]').click();}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="save"]')`)
	page.MustElement(`.vd-modal [data-choice="keep"]`).MustClick()
	page.MustWait(`()=>document.querySelector('.vd-editor .vd-path')?.textContent.startsWith('Documents/untitled-')`)
	if writes.Load() != 2 || !page.MustEval(`()=>document.querySelector('.vd-editor textarea')?.value===''`).Bool() {
		t.Fatal("New did not wait for the explicit Without saving decision or wrote the previous draft")
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatalf("browser runtime errors: %s", errors)
	}
}

func TestDesktopSessionActionsStartUsesAuthenticatedLogoutBrowser(t *testing.T) {
	var enabled atomic.Bool
	var logoutCalls atomic.Int32
	status := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"enabled": enabled.Load(), "authenticated": true,
			"session_scope": strings.Repeat("a", 64),
		})
	})
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/auth/status": status})
	page.MustEval(`async()=>{
		window.fixtureStatusReady=true;
		window._auragoSharedInitialized=true;
		const oldTranslate=window.t;
		await new Promise((resolve,reject)=>{const script=document.createElement('script');script.src='/js/shared/shared-core.js';script.onload=resolve;script.onerror=reject;document.head.append(script);});
		window.t=oldTranslate;
		await checkAuth();
	}`)
	if !page.MustEval(`()=>document.getElementById('vd-start-session').hidden`).Bool() {
		t.Fatal("logout control was visible while authentication was disabled")
	}
	enabled.Store(true)
	page.MustEval(`async()=>{await checkAuth();window.performAuthenticatedLogout=async()=>{window.__authenticatedLogoutCalls=(window.__authenticatedLogoutCalls||0)+1;return true;};}`)
	page.MustWait(`()=>!document.getElementById('vd-start-session').hidden`)
	page.MustEval(`()=>document.getElementById('vd-start-logout').click()`)
	page.MustWait(`()=>window.__authenticatedLogoutCalls===1`)
	logoutCalls.Add(int32(page.MustEval(`()=>window.__authenticatedLogoutCalls`).Int()))
	if logoutCalls.Load() != 1 {
		t.Fatal("Start logout did not reach the authenticated logout continuation exactly once")
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatalf("browser runtime errors: %s", errors)
	}
}

func TestDesktopSessionActionsMailerRealAppBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
		await AuraDesktopModules.loadAppAssets('mailer');
		window.mailerWrites=0;window.mailerDeletes=0;window.mailerFail=false;window.mailerLogoutCalls=0;window.mailerProblems=[];
		window.mailerStored={id:'persisted-draft',revision:7,account_id:'account-1',to:[],cc:[],bcc:[],subject:'Previously saved',text:'Recovery stays available',html:'',attachments:[]};
		window.mailerWindows=[{id:'mailer-session-real',appId:'mailer',title:'Mailer',beforeClose:null}];
		window.mailerChoices=['save'];
		AuraDesktopSessionActions.configure({getWindows:()=>mailerWindows,canLogout:()=>true,logout:async()=>{mailerLogoutCalls++;return true;},onProblem:async problem=>mailerProblems.push(problem.code)});
		const host=document.createElement('div');host.id='mailer-session-host';document.body.append(host);
		const ctx={
			state:{accounts:[{id:'account-1',name:'Fixture',address:'fixture@example.test'}],contacts:[]},readonly:false,
			t:key=>key,api:async(url,options={})=>{
				if(url.endsWith('/templates')||url.endsWith('/signatures'))return [];
				if(url.includes('/drafts/')){
					if(options.method==='PUT'){
						mailerWrites++;if(mailerFail){const error=new Error('fixture save failed');error.status=503;throw error;}
						mailerStored=Object.assign({},JSON.parse(options.body),{revision:mailerStored.revision+1});return mailerStored;
					}
					if(options.method==='DELETE'){mailerDeletes++;return {};}
				}
				return {};
			},
			registerSessionAction:(id,participant)=>AuraDesktopSessionActions.register(id,participant),
			decideSessionAction:async()=>mailerChoices.shift()||'cancel',
			setWindowBeforeClose:(id,handler)=>{const win=mailerWindows.find(item=>item.id===id);if(win)win.beforeClose=handler;},
			registerWindowCleanup:()=>{},confirmDialog:async()=>true
		};
		window.mailerSessionInstance=MailerCompose.render(host,'mailer-session-real',ctx,{draft:mailerStored,accountId:'account-1'});
		await window.mailerSessionInstance.editorPromise;
		window.mailerSessionReady=true;
	}`)
	page.MustWait(`()=>window.mailerSessionReady===true`)
	page.MustEval(`async()=>{
		const subject=document.querySelector('#mailer-session-host [data-subject]');
		subject.value='Unsaved edit before a later cancellation';subject.dispatchEvent(new Event('input',{bubbles:true}));
		clearTimeout(mailerSessionInstance.saveTimer);mailerSessionInstance.saveTimer=0;
		AuraDesktopSessionActions.register('zz-late-cancel',{
			inspect:()=>({dirty:true,revision:1,label:'Later app'}),
			decide:async()=> 'cancel',save:async()=>({revision:1})
		});
		mailerWindows.push({id:'zz-late-cancel',appId:'fixture',title:'Later app',beforeClose:null});
		window.mailerFirstResult=await AuraDesktopSessionActions.requestLogout();
	}`)
	if !page.MustEval(`()=>mailerFirstResult===false&&mailerWrites===0&&mailerDeletes===0&&mailerLogoutCalls===0&&mailerStored.subject==='Previously saved'&&document.querySelector('#mailer-session-host [data-subject]').value==='Unsaved edit before a later cancellation'`).Bool() {
		t.Fatal("a later app cancellation changed or deleted the Mailer draft")
	}
	page.MustEval(`async()=>{
		mailerWindows=mailerWindows.filter(item=>item.id!=='zz-late-cancel');AuraDesktopSessionActions.register('zz-late-cancel',null);mailerChoices=['save'];mailerFail=true;
		const subject=document.querySelector('#mailer-session-host [data-subject]');subject.value='Keep the failed edit';subject.dispatchEvent(new Event('input',{bubbles:true}));
		clearTimeout(mailerSessionInstance.saveTimer);mailerSessionInstance.saveTimer=0;
		window.mailerFailureResult=await AuraDesktopSessionActions.requestLogout();
	}`)
	if !page.MustEval(`()=>mailerFailureResult===false&&mailerWrites===1&&mailerDeletes===0&&mailerLogoutCalls===0&&mailerProblems.includes('save_failed')&&mailerStored.subject==='Previously saved'&&document.querySelector('#mailer-session-host [data-subject]').value==='Keep the failed edit'`).Bool() {
		t.Fatal("a failed Mailer draft save did not preserve the existing draft and open edit")
	}
	page.MustEval(`async()=>{mailerSessionInstance.readonly=true;mailerChoices=['cancel'];window.mailerReadonlyResult=await AuraDesktopSessionActions.requestLogout();}`)
	if !page.MustEval(`()=>mailerReadonlyResult===false&&mailerDeletes===0&&mailerStored.subject==='Previously saved'&&document.querySelector('#mailer-session-host [data-subject]').value==='Keep the failed edit'`).Bool() {
		t.Fatal("a read-only Mailer state hid the existing unsaved draft from logout decisions")
	}
	page.MustEval(`()=>{mailerSessionInstance.readonly=false;}`)
	page.MustEval(`async()=>{mailerChoices=['keep'];window.mailerKeepResult=await AuraDesktopSessionActions.requestLogout();}`)
	if !page.MustEval(`()=>mailerKeepResult===true&&mailerDeletes===0&&mailerStored.subject==='Previously saved'&&document.querySelector('#mailer-session-host [data-subject]').value==='Keep the failed edit'`).Bool() {
		t.Fatal("Without saving deleted the already persisted Mailer draft")
	}
	page.MustEval(`()=>{AuraDesktopSessionActions.register('mailer-session-real',null);MailerCompose.dispose('mailer-session-real');}`)
}

func TestDesktopSessionActionsSynthRealAppBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
		await AuraDesktopModules.loadAppAssets('synth-studio');
		window.synthSessionStops=0;window.synthSessionWrites=0;window.synthSessionRunning=false;window.synthSessionFail=false;window.synthSessionLogoutCalls=0;window.synthSessionProblems=[];
		const originalCreate=SynthStudioAudio.create.bind(SynthStudioAudio);
		SynthStudioAudio.create=options=>{
			const audio=originalCreate(options);
			audio.unlock=async()=>{};audio.play=()=>{synthSessionRunning=true;return new Promise(()=>{});};audio.running=()=>synthSessionRunning;
			audio.stop=()=>{synthSessionStops++;synthSessionRunning=false;return 0};audio.panic=()=>{};audio.updateTracks=()=>{};audio.setAutomation=()=>{};audio.setManual=()=>{};
			audio.currentTick=()=>0;audio.elapsed=()=>0;audio.dispose=()=>{};
			return audio;
		};
		window.synthSessionWindows=[{id:'synth-session-real',appId:'synth-studio',title:'Synth Studio',beforeClose:null}];window.synthSessionChoices=['keep'];
		AuraDesktopSessionActions.configure({getWindows:()=>synthSessionWindows,canLogout:()=>true,logout:async()=>{synthSessionLogoutCalls++;return true;},onProblem:async problem=>synthSessionProblems.push(problem.code)});
		const host=document.createElement('div');host.id='synth-session-host';document.body.append(host);
		const ctx={windowId:'synth-session-real',t:key=>key,esc:value=>String(value),api:async(url,options={})=>{
			if(options.method==='PUT'){synthSessionWrites++;if(synthSessionFail)throw new Error('fixture save failed');return{path:JSON.parse(options.body).path,version:'"synth-session-v1"'};}
			return{};
		},saveFileDialog:async()=>({path:'Documents/Synth Studio/session.aurasynth'}),registerSessionAction:(id,participant)=>AuraDesktopSessionActions.register(id,participant),
			decideSessionAction:async()=>synthSessionChoices.shift()||'cancel',setWindowBeforeClose:(id,handler)=>{synthSessionWindows[0].beforeClose=handler;},registerWindowCleanup:()=>{},confirmDialog:async()=>true};
		window.synthSessionInstance=SynthStudioApp.render(host,'synth-session-real',ctx);
		await synthSessionInstance.storage.ready;
		window.synthSessionReady=true;
	}`)
	page.MustWait(`()=>window.synthSessionReady===true`)
	page.MustEval(`async()=>{
		document.querySelector('#synth-session-host [data-action="play"]').click();
		await new Promise(resolve=>setTimeout(resolve,0));
		const tempo=document.querySelector('#synth-session-host [data-tempo]');tempo.value='121';tempo.dispatchEvent(new Event('change',{bubbles:true}));
		window.synthStopsBeforeCancel=synthSessionStops;
		AuraDesktopSessionActions.register('zz-late-cancel',{
			inspect:()=>({dirty:true,revision:1,label:'Later app'}),decide:async()=> 'cancel',save:async()=>({revision:1})
		});
		synthSessionWindows.push({id:'zz-late-cancel',appId:'fixture',title:'Later app',beforeClose:null});
		window.synthCancelResult=await AuraDesktopSessionActions.requestLogout();
	}`)
	synthCancelState := page.MustEval(`()=>JSON.stringify({
		result:synthCancelResult,stops:synthSessionStops,stopsBefore:synthStopsBeforeCancel,writes:synthSessionWrites,
		playActive:document.querySelector('#synth-session-host [data-action="play"]')?.classList.contains('is-active'),
		storage:synthSessionInstance.storage.state,problems:synthSessionProblems,
		windows:synthSessionWindows.map(item=>item.id)
	})`).Str()
	if !page.MustEval(`()=>synthCancelResult===false&&synthSessionStops===synthStopsBeforeCancel&&synthSessionWrites===0&&document.querySelector('#synth-session-host [data-action="play"]').classList.contains('is-active')&&synthSessionInstance.storage.state.dirty`).Bool() {
		t.Fatalf("a later cancellation stopped Synth playback or discarded the dirty project; observed %s", synthCancelState)
	}
	page.MustEval(`async()=>{
		synthSessionWindows=synthSessionWindows.filter(item=>item.id!=='zz-late-cancel');AuraDesktopSessionActions.register('zz-late-cancel',null);
		synthSessionChoices=['save'];synthSessionFail=true;
		window.synthFailureResult=await AuraDesktopSessionActions.requestLogout();
	}`)
	synthFailureState := page.MustEval(`()=>JSON.stringify({
		result:synthFailureResult,stops:synthSessionStops,stopsBefore:synthStopsBeforeCancel,writes:synthSessionWrites,logoutCalls:synthSessionLogoutCalls,
		playActive:document.querySelector('#synth-session-host [data-action="play"]')?.classList.contains('is-active'),
		storage:synthSessionInstance.storage.state,problems:synthSessionProblems
	})`).Str()
	if !page.MustEval(`()=>synthFailureResult===false&&synthSessionWrites===1&&synthSessionLogoutCalls===0&&synthSessionStops===synthStopsBeforeCancel&&document.querySelector('#synth-session-host [data-action="play"]').classList.contains('is-active')&&synthSessionInstance.storage.state.dirty`).Bool() {
		t.Fatalf("a failed Synth save closed or stopped the recoverable dirty project; observed %s", synthFailureState)
	}
	page.MustEval(`()=>{AuraDesktopSessionActions.register('synth-session-real',null);SynthStudioApp.dispose('synth-session-real');}`)
}

func TestDesktopSessionActionsZiroRealAppBrowser(t *testing.T) {
	var uploads atomic.Int32
	var failUpload atomic.Bool
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><script>
		const params=new URLSearchParams(location.hash.slice(1)),parentOrigin=params.get('parent_origin'),challenge=params.get('aurago_ziro');
		let generation=0,dirty=false,saved=0,discarded=0;
		parent.postMessage({ziro:1,type:'ready',challenge},parentOrigin);
		addEventListener('message',event=>{if(event.data?.ziro!==1||event.data.type!=='connect'||!event.ports[0])return;const port=event.ports[0];port.onmessage=message=>{
			const data=message.data||{};let payload={};
			if(data.type==='draft')payload={draft:false};
			if(data.type==='new'){generation++;dirty=true;payload={};}
			if(data.type==='snapshot')payload={generation,files:[{name:'Project.kicad_pro',bytes:new Uint8Array([1,2,3])}]};
			if(data.type==='saved'){saved++;dirty=false;payload={};}
			if(data.type==='discard'){discarded++;dirty=false;payload={};}
			port.postMessage({ziro:1,reply:data.id,payload});
		};});
		window.parent.postMessage({test:'ready'} ,parentOrigin);
		</script>`)
	}))
	defer bridge.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/desktop/directory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]string{"path": body.Path})
	})
	mux.HandleFunc("/api/desktop/upload", func(w http.ResponseWriter, r *http.Request) {
		uploads.Add(1)
		if failUpload.Load() {
			http.Error(w, `{"error":"fixture upload failed"}`, http.StatusInternalServerError)
			return
		}
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			http.Error(w, `{"error":"invalid_form"}`, http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"missing_file"}`, http.StatusBadRequest)
			return
		}
		_ = file.Close()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"path": strings.TrimSuffix(r.FormValue("path"), "/") + "/" + header.Filename, "version": `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`})
	})
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/directory": mux, "/api/desktop/upload": mux})
	page.MustEval(`(url)=>{window.ziroBridgeURL=url;}`, bridge.URL+"/bridge")
	page.MustEval(`async()=>{
		await AuraLazyAssets.loadScript('/js/desktop/apps/ziro-files.js');
		await AuraLazyAssets.loadScript('/js/desktop/apps/ziro-designer.js');
		window.ziroLogoutCalls=0;window.ziroProblems=[];window.ziroChoices=['save'];window.ziroWindows=[{id:'ziro-session-real',appId:'store-ziro-designer',title:'Ziro Designer',beforeClose:null}];
		AuraDesktopSessionActions.configure({getWindows:()=>ziroWindows,canLogout:()=>true,logout:async()=>{ziroLogoutCalls++;return true;},onProblem:async problem=>ziroProblems.push(problem.code)});
		const host=document.createElement('div');host.id='ziro-session-host';document.body.append(host);
		const context={ziro_new:'Logout fixture'};
		const deps={
			t:key=>key,esc:value=>String(value),api:async(url,options={})=>{
				if(url==='/api/desktop/ziro/open')return{url:ziroBridgeURL};
				if(url.startsWith('/api/desktop/files?')){const error=new Error('missing folder');error.status=404;throw error;}
				const response=await fetch(url,options);if(!response.ok){const error=new Error('HTTP '+response.status);error.status=response.status;throw error;}return response.json();
			},openURL:'/api/desktop/ziro/open',makeSandboxedFrame:url=>{const frame=document.createElement('iframe');frame.setAttribute('sandbox','allow-scripts allow-same-origin');frame.src=url;window.ziroFrame=frame;return frame;},
			registerWindowCleanup:(id,cleanup)=>{window.ziroCleanup=cleanup;},registerSessionAction:(id,participant)=>{window.ziroParticipant=participant;AuraDesktopSessionActions.register(id,participant);},
			decideSessionAction:async()=>ziroChoices.shift()||'cancel',modalDialog:async()=>null,openFileDialog:async()=>({canceled:true}),saveFileDialog:async()=>({canceled:false,path:'Shared/Ziro Designer/Logout fixture.kicad_pro'}),
			isReadonly:()=>false,updateContext:()=>{},setBeforeClose:handler=>{ziroWindows[0].beforeClose=handler;},openProjectWindow:()=>false
		};
		await ZiroDesignerApp.render(host,'ziro-session-real',{id:'store-ziro-designer'},context,deps);
		window.ziroSessionReady=true;
	}`)
	page.MustWait(`()=>window.ziroSessionReady===true&&!!document.querySelector('#ziro-session-host iframe')`)
	page.MustWait(`()=>window.ziroParticipant?.inspect().dirty===true`)
	page.MustEval(`async()=>{
		AuraDesktopSessionActions.register('zz-late-cancel',{
			inspect:()=>({dirty:true,revision:1,label:'Later app'}),decide:async()=> 'cancel',save:async()=>({revision:1})
		});
		ziroWindows.push({id:'zz-late-cancel',appId:'fixture',title:'Later app',beforeClose:null});
		window.ziroCancelResult=await AuraDesktopSessionActions.requestLogout();
	}`)
	if uploads.Load() != 0 || !page.MustEval(`()=>ziroCancelResult===false&&ziroLogoutCalls===0&&ziroParticipant.inspect().dirty===true`).Bool() {
		t.Fatal("a later cancellation saved, discarded, or closed the live Ziro project")
	}
	failUpload.Store(true)
	page.MustEval(`async()=>{
		ziroWindows=ziroWindows.filter(item=>item.id!=='zz-late-cancel');AuraDesktopSessionActions.register('zz-late-cancel',null);
		ziroChoices=['save'];
		window.ziroFailureResult=await AuraDesktopSessionActions.requestLogout();
	}`)
	if uploads.Load() != 1 || !page.MustEval(`()=>ziroFailureResult===false&&ziroLogoutCalls===0&&ziroParticipant.inspect().dirty===true&&ziroProblems.includes('save_failed')`).Bool() {
		t.Fatal("a failed Ziro save did not keep the unsaved project open")
	}
	page.MustEval(`()=>{AuraDesktopSessionActions.register('ziro-session-real',null);ziroCleanup?.();}`)
}

func TestDesktopSessionActionsStorymakerRealAppBrowser(t *testing.T) {
	fixture := newStorymakerFixture(t)
	var failSave atomic.Bool
	var saveAttempts atomic.Int32
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, `/story`) {
			saveAttempts.Add(1)
			if failSave.Load() {
				w.Header().Set(`Content-Type`, `application/json`)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"fixture save failed"}`))
				return
			}
		}
		fixture.ServeHTTP(w, r)
	})
	page := storymakerBrowserPage(t, api)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="new-title"]')`)
	page.MustElement(`[data-stm="new-title"]`).MustInput(`Session action fixture`)
	storymakerClick(t, page, `[data-stm="create-project"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="project-title"]')?.textContent==='Session action fixture'`)
	ids := fixture.projectIDs()
	if len(ids) != 1 {
		t.Fatalf(`storymaker projects after create = %v`, ids)
	}
	id := ids[0]
	before, _ := fixture.storedStory(id)
	page.MustEval(`()=> {
		const original=window.setTimeout.bind(window);
		window.setTimeout=(fn,delay,...args)=>original(fn,delay===700?60000:delay,...args);
		window.storySessionLogoutCalls=0;window.storySessionFake=true;
		AuraDesktopSessionActions.configure({
			getWindows:()=>[...auditTest.state.windows.values()].map(win=>({id:win.id,title:win.title,appId:win.appId,beforeClose:win.beforeClose}))
				.concat(storySessionFake?[{id:'zz-late-cancel',appId:'fixture',title:'Later app',beforeClose:null}]:[]),
			canLogout:()=>true,logout:async()=>{storySessionLogoutCalls++;return true;},onProblem:async()=>{}
		});
		AuraDesktopSessionActions.register('zz-late-cancel',{
			inspect:()=>({dirty:true,revision:1,label:'Later app'}),decide:async()=> 'cancel',save:async()=>({revision:1})
		});
		const field=document.querySelector('[data-stm="narration"]');
		field.value='An edit that must stay in this open project';field.dispatchEvent(new Event('input',{bubbles:true}));
		window.storySessionFirst=AuraDesktopSessionActions.requestLogout().then(result=>{window.storySessionFirstResult=result;});
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="save"]')`)
	page.MustElement(`.vd-modal [data-choice="save"]`).MustClick()
	page.MustWait(`()=>window.storySessionFirstResult===false`)
	if saveAttempts.Load() != 0 {
		t.Fatal(`Storymaker saved before a later window cancelled`)
	}
	after, _ := fixture.storedStory(id)
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if string(beforeJSON) != string(afterJSON) || !page.MustEval(`()=>storySessionLogoutCalls===0&&document.querySelector('[data-stm="save-state"]')?.dataset.state==='dirty'`).Bool() {
		t.Fatal(`later cancellation changed the persisted story or closed the dirty editor`)
	}
	failSave.Store(true)
	page.MustEval(`()=>{storySessionFake=false;AuraDesktopSessionActions.register('zz-late-cancel',null);window.storySessionFailure=AuraDesktopSessionActions.requestLogout().then(result=>{window.storySessionFailureResult=result;});}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="save"]')`)
	page.MustElement(`.vd-modal [data-choice="save"]`).MustClick()
	page.MustWait(`()=>window.storySessionFailureResult===false`)
	if saveAttempts.Load() != 1 || !page.MustEval(`()=>storySessionLogoutCalls===0&&document.querySelector('[data-stm="save-state"]')?.dataset.state==='error'`).Bool() {
		t.Fatal(`failed Storymaker save did not leave the project open and dirty`)
	}
	page.MustEval(`()=>AuraDesktopSessionActions.register([...auditTest.state.windows.values()].find(win=>win.appId==='storymaker')?.id,null)`)
}

func TestDesktopSessionActionsSheetsPendingEditsBrowser(t *testing.T) {
	testSheetsAppBrowser(t, func(t *testing.T, page *rod.Page) {
		page.MustEval(`async()=>{
			await new Promise((resolve,reject)=>{const script=document.createElement('script');script.src='/js/desktop/core/session-actions-runtime.js';script.onload=resolve;script.onerror=reject;document.head.append(script);});
			window.sheetSessionChoices=['cancel'];window.sheetSessionLogouts=0;window.sheetSessionProblems=[];
			const context=window.sheetsContext;
			context.registerSessionAction=(id,participant)=>{window.sheetSessionParticipant=participant;window.sheetSessionUnregister=AuraDesktopSessionActions.register(id,participant);return window.sheetSessionUnregister;};
			context.decideSessionAction=async()=>sheetSessionChoices.shift()||'cancel';
			AuraDesktopSessionActions.configure({getWindows:()=>[{id:'test',title:'Sheets',appId:'sheets'}],canLogout:()=>true,
				logout:async()=>{sheetSessionLogouts++;return true;},onProblem:async problem=>sheetSessionProblems.push(problem.code)});
			const first=SheetsApp.instances.get('test');
			await first.act('new');
			if(!first.session||!first.book)throw new Error('Sheets New did not create a workbook: '+(document.querySelector('#host [data-notice-text]')?.textContent||''));
			await first.state.save();
			if(first.session.dirty)throw new Error('Sheets fixture could not persist the clean starting workbook');
			context.path=first.path;
			window.sheetSessionInitTimedOut=false;setTimeout(()=>{window.sheetSessionInitTimedOut=true;},60000);
			SheetsApp.dispose('test');SheetsApp.render(document.getElementById('host'),'test',context);
		}`)
		page.MustWait(`()=>!!(SheetsApp.instances.get('test')?.session&&SheetsApp.instances.get('test')?.book)||!!document.querySelector('#host [data-notice][data-error="true"]')||sheetSessionInitTimedOut===true`)
		if !page.MustEval(`()=>!!(SheetsApp.instances.get('test')?.session&&SheetsApp.instances.get('test')?.book)`).Bool() {
			t.Fatalf("Sheets did not finish initializing after logout action wiring: %s", page.MustEval(`()=>JSON.stringify({timedOut:sheetSessionInitTimedOut,instance:!!SheetsApp.instances.get('test'),session:!!SheetsApp.instances.get('test')?.session,book:!!SheetsApp.instances.get('test')?.book,notice:document.querySelector('#host [data-notice-text]')?.textContent||'',noticeError:document.querySelector('#host [data-notice]')?.dataset.error||'',loadState:document.querySelector('#host [data-loading]')?.dataset.state||'',errors:window.sheetErrors||[]})`).Str())
		}
		if !page.MustEval(`()=>!sheetSessionParticipant.inspect().dirty&&!SheetsApp.instances.get('test').session.dirty`).Bool() {
			t.Fatalf("Sheets did not start from a persisted clean workbook: %s", page.MustEval(`()=>JSON.stringify({participant:sheetSessionParticipant.inspect(),dirty:SheetsApp.instances.get('test').session.dirty,path:SheetsApp.instances.get('test').path})`).Str())
		}
		page.MustEval(`async()=>{SheetsApp.instances.get('test').session.suspend();await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));}`)
		rect := page.MustEval(`async cell=>{const app=SheetsApp.instances.get('test'),range=app.book.getActiveSheet().getRange(cell);range.activate();await new Promise(resolve=>setTimeout(resolve,150));const cellRect=range.getCellRect(),base=document.querySelector('[data-engine]').getBoundingClientRect();window.sheetGridTargetDiagnostic={x:base.left+cellRect.left+20,y:base.top+cellRect.top+10,cellRect,baseRect:base.toJSON()};return{x:window.sheetGridTargetDiagnostic.x,y:window.sheetGridTargetDiagnostic.y};}`, "A2")
		page.Mouse.MustMoveTo(rect.Get("x").Num(), rect.Get("y").Num())
		page.Mouse.MustClick(proto.InputMouseButtonLeft)
		if err := page.Mouse.Click(proto.InputMouseButtonLeft, 2); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`()=>{window.sheetGridEditTimedOut=false;setTimeout(()=>{window.sheetGridEditTimedOut=true;},4000);}`)
		page.MustWait(`()=>SheetsApp.instances.get('test').book.isCellEditing()===true||sheetGridEditTimedOut===true`)
		if !page.MustEval(`()=>SheetsApp.instances.get('test').book.isCellEditing()===true`).Bool() {
			t.Fatalf("Sheets grid cell did not enter edit mode after double-click: %s", page.MustEval(`()=>JSON.stringify({editing:SheetsApp.instances.get('test')?.book?.isCellEditing?.(),target:sheetGridTargetDiagnostic,activeElement:{tag:document.activeElement?.tagName,className:document.activeElement?.className,contentEditable:document.activeElement?.contentEditable,ariaLabel:document.activeElement?.getAttribute('aria-label'),outer:document.activeElement?.outerHTML?.slice(0,300)},elementsAtPoint:document.elementsFromPoint(sheetGridTargetDiagnostic.x,sheetGridTargetDiagnostic.y).slice(0,8).map(element=>({tag:element.tagName,className:element.className,ariaLabel:element.getAttribute('aria-label'),text:element.textContent?.slice(0,60)})),selection:SheetsApp.instances.get('test')?.book?.getActiveSheet()?.getActiveRange?.()?.getRange?.(),host:document.getElementById('host')?.getBoundingClientRect()?.toJSON()})`).Str())
		}
		if err := (proto.InputInsertText{Text: "unsaved grid value"}).Call(page); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`async()=>{window.sheetGridInspectBefore=sheetSessionParticipant.inspect();window.sheetGridResult=await AuraDesktopSessionActions.requestLogout();window.sheetGridInspectAfter=sheetSessionParticipant.inspect();}`)
		if !page.MustEval(`()=>sheetGridResult===false&&sheetGridInspectBefore.dirty&&sheetGridInspectAfter.dirty&&sheetSessionLogouts===0`).Bool() {
			t.Fatal("a pending grid edit was invisible to logout inspection or changed on cancel")
		}
		page.MustEval(`async()=>{sheetSessionChoices=['save'];window.sheetGridSaveResult=await AuraDesktopSessionActions.requestLogout();}`)
		if !page.MustEval(`()=>sheetGridSaveResult===true&&sheetSessionLogouts===1&&!SheetsApp.instances.get('test').session.dirty&&SheetsApp.instances.get('test').book.getActiveSheet().getRange('A2').getRawValue()==='unsaved grid value'`).Bool() {
			t.Fatalf("saving a pending grid edit did not acknowledge its own commit: %s", page.MustEval(`()=>JSON.stringify({result:sheetGridSaveResult,logouts:sheetSessionLogouts,participant:sheetSessionParticipant.inspect(),dirty:SheetsApp.instances.get('test').session.dirty,value:SheetsApp.instances.get('test').book.getActiveSheet().getRange('A2').getRawValue(),problems:sheetSessionProblems,errors:sheetErrors})`).Str())
		}
		page.MustEval(`async()=>{
			const app=SheetsApp.instances.get('test'),sheet=app.book.getActiveSheet();
			sheet.getRange('A2').activate();
			await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
			const selected=sheet.getActiveRange()?.getRange();window.sheetFormulaTarget=[selected?.startRow,selected?.startColumn];
			const formula=document.querySelector('[data-formula]');formula.focus();formula.value='=1+2';formula.dispatchEvent(new Event('input',{bubbles:true}));window.sheetFormulaInspect=sheetSessionParticipant.inspect();
		}`)
		if !page.MustEval(`()=>sheetFormulaInspect.dirty`).Bool() {
			t.Fatal("an uncommitted formula edit was invisible to logout inspection")
		}
		page.MustEval(`async()=>{sheetSessionChoices=['save'];window.sheetFormulaSaveResult=await AuraDesktopSessionActions.requestLogout();}`)
		page.MustEval(`()=>{window.sheetFormulaWaitTimedOut=false;setTimeout(()=>{window.sheetFormulaWaitTimedOut=true;},5000);}`)
		page.MustWait(`()=>SheetsApp.instances.get('test').book.getActiveSheet().getRange('A2').getRawValue()===3||sheetFormulaWaitTimedOut===true`)
		if !page.MustEval(`()=>sheetFormulaSaveResult===true&&sheetSessionLogouts===2&&!SheetsApp.instances.get('test').session.dirty&&SheetsApp.instances.get('test').book.getActiveSheet().getRange('A2').getRawValue()===3`).Bool() {
			t.Fatalf("saving a pending formula did not acknowledge its own commit: %s", page.MustEval(`()=>JSON.stringify({result:sheetFormulaSaveResult,logouts:sheetSessionLogouts,target:sheetFormulaTarget,active:SheetsApp.instances.get('test')?.book?.getActiveSheet()?.getActiveRange?.()?.getRange?.(),participant:sheetSessionParticipant.inspect(),dirty:SheetsApp.instances.get('test').session.dirty,value:SheetsApp.instances.get('test').book.getActiveSheet().getRange('A2').getRawValue(),problems:sheetSessionProblems,errors:sheetErrors})`).Str())
		}
		page.MustEval(`()=>{
			const nativeFetch=window.fetch.bind(window);window.sheetFailNextPatch=true;
			window.fetch=(input,init={})=>{
				const url=typeof input==='string'?input:input.url;
				if(window.sheetFailNextPatch&&init.method==='PATCH'&&url.includes('/api/desktop/office/workbook?')){
					window.sheetFailNextPatch=false;
					return Promise.resolve(new Response(JSON.stringify({error:'fixture PATCH rejected'}),{status:500,headers:{'Content-Type':'application/json'}}));
				}
				return nativeFetch(input,init);
			};
			SheetsApp.instances.get('test').book.getActiveSheet().getRange('A3').setValue('preserved after failed save');
		}`)
		page.MustEval(`async()=>{sheetSessionChoices=['save'];window.sheetFailedSaveResult=await AuraDesktopSessionActions.requestLogout();window.sheetFailedSaveDraft=await OfficeSession.draft('get',location.origin+':'+SheetsApp.instances.get('test').path,undefined,'sheets');}`)
		if !page.MustEval(`()=>sheetFailedSaveResult===false&&sheetSessionLogouts===2&&!!SheetsApp.instances.get('test').book&&SheetsApp.instances.get('test').session.dirty&&SheetsApp.instances.get('test').book.getActiveSheet().getRange('A3').getRawValue()==='preserved after failed save'&&!!sheetFailedSaveDraft?.snapshot?.workbook&&sheetSessionProblems.includes('save_failed')`).Bool() {
			t.Fatalf("a failed Sheets PATCH did not leave the document and recoverable draft intact: %s", page.MustEval(`()=>JSON.stringify({result:sheetFailedSaveResult,logouts:sheetSessionLogouts,dirty:SheetsApp.instances.get('test')?.session?.dirty,value:SheetsApp.instances.get('test')?.book?.getActiveSheet().getRange('A3').getRawValue(),draft:!!sheetFailedSaveDraft,revision:sheetFailedSaveDraft?.revision,path:sheetFailedSaveDraft?.path,problems:sheetSessionProblems,errors:sheetErrors})`).Str())
		}
		page.MustEval(`()=>{sheetSessionUnregister?.();SheetsApp.dispose('test');}`)
	})
}

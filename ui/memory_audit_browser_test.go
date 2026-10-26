package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestMemoryAuditBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("headless browser smoke test requires Chrome or Edge")
	}
	const chatFixture = `<!doctype html><html lang="en" data-theme="dark"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-utilities.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/chat.bundle.css"></head><body>
<button id="session-toggle-btn" class="session-edge-tab"></button><div id="session-backdrop" class="session-backdrop"></div>
<aside id="session-drawer" class="session-drawer"><div class="session-drawer-header"><span class="session-drawer-title">Conversations</span><button id="session-drawer-close" class="session-drawer-close">Close</button></div><button id="session-new-btn" class="session-new-btn">New chat</button><div id="session-list" class="session-list"></div></aside>
<script>localStorage.setItem('aurago-session-id','default');window.I18N={'chat.sessions_delete':'Delete conversation','chat.sessions_delete_hint':'Choose whether associated memory may also be deleted.','chat.sessions_delete_memories':'Delete derived memories too','chat.sessions_delete_backups':'Backups are retained.','chat.sessions_delete_scope_conflict':'The deletion scope changed. Refresh and review the current session before trying again.','chat.sessions_delete_failed':'Could not delete conversation.','chat.sessions_delete_report':'deleted {deleted}; retained {retained}','common.btn_cancel':'Cancel','common.btn_delete':'Delete','common.error':'Error'};window.t=k=>window.I18N[k]||k;window.alerts=[];window.showAlert=async(title,message)=>window.alerts.push({title,message});</script>
<script src="/js/chat/modules/session-drawer.js"></script></body></html>`
	const dashboardFixture = `<!doctype html><html lang="en" data-theme="dark"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-utilities.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/tokens.css"><link rel="stylesheet" href="/css/dashboard.css"><link rel="stylesheet" href="/css/dashboard-shell.css"><link rel="stylesheet" href="/css/precision-workspace.css"><link rel="stylesheet" href="/css/precision-pages.css"></head><body class="pw-page pw-operational-page" data-workspace-page="dashboard" data-density="comfortable"><main><section class="dash-section"><article class="dash-card" id="card-memory"><details open><summary>Memory health</summary><div class="dash-disclosure-body" id="memory-health-summary"></div></details></article></section></main><script>window.DashSections={register(){}};window.DashDisclosures={setCount(){}};</script><script src="/js/dashboard/sections/memory.js"></script></body></html>`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture/chat", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, chatFixture)
	})
	mux.HandleFunc("/fixture/dashboard", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, dashboardFixture)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Leakless(false).Set("disable-gpu").Set("disable-dev-shm-usage")
	controlURL, err := launch.Launch()
	if err != nil {
		t.Skipf("headless browser smoke test skipped; browser launch failed: %v", err)
	}
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect browser: %v", err)
	}
	t.Cleanup(func() {
		_ = browser.Close()
		launch.Kill()
		launch.Cleanup()
	})

	for _, theme := range []string{"dark", "light"} {
		t.Run("webchat-"+theme, func(t *testing.T) {
			page := browser.MustPage(server.URL + "/fixture/chat").Timeout(30 * time.Second)
			defer page.Close()
			page.MustWaitLoad()
			result := page.MustEval(`async(theme)=>{
 document.documentElement.dataset.theme=theme;
 localStorage.setItem('aurago-session-id','default');
 const requests=[];
 const wait=async fn=>{for(let i=0;i<200;i++){if(fn())return;await new Promise(r=>setTimeout(r,10))}throw Error('timed out waiting for UI state')};
 window.fetch=async(url,options={})=>{
   url=String(url);requests.push({url,method:options.method||'GET'});
   if(url==='/api/chat/sessions/default')return {ok:true,status:200,json:async()=>({session:{id:'default'}})};
   if(url==='/api/chat/sessions')return {ok:true,status:200,json:async()=>({sessions:[{id:'private-session',preview:'Private conversation',message_count:2}]})};
   if(url.startsWith('/api/chat/sessions/private-session?delete_memories='))return {ok:false,status:409,json:async()=>({code:'deletion_scope_conflict'})};
   throw Error('unexpected request '+url);
 };
 SessionDrawer.init();SessionDrawer.open();
 await wait(()=>document.querySelector('[data-delete-id="private-session"]'));
 const drawer=document.getElementById('session-drawer');
 if(!drawer.classList.contains('open')||getComputedStyle(drawer).visibility==='hidden')throw Error('production session drawer did not open');
 document.querySelector('[data-delete-id="private-session"]').click();
 await wait(()=>document.querySelector('dialog[open]'));
 const dialog=document.querySelector('dialog');dialog.querySelector('input').checked=true;dialog.querySelector('button[value=delete]').click();
 await wait(()=>window.alerts.length===1);
 const deletion=requests.find(item=>item.method==='DELETE');
 if(!deletion||!deletion.url.endsWith('delete_memories=true'))throw Error('explicit memory deletion scope was not submitted');
 if(window.alerts[0].message!==I18N['chat.sessions_delete_scope_conflict'])throw Error('409 scope conflict was not explained to the user');
 if(!document.querySelector('[data-delete-id="private-session"]')||!document.querySelector('.session-item-preview')?.textContent.includes('Private conversation'))throw Error('409 conflict removed or hid the session');
 const style=getComputedStyle(drawer);if(!style.backgroundColor||style.width==='0px')throw Error('production webchat drawer styles were not applied');
 return true;
 }`, theme)
			if !result.Bool() {
				t.Fatal("webchat privacy scope conflict did not remain reviewable")
			}
		})

		t.Run("dashboard-"+theme, func(t *testing.T) {
			page := browser.MustPage(server.URL + "/fixture/dashboard").Timeout(30 * time.Second)
			defer page.Close()
			page.MustWaitLoad()
			result := page.MustEval(`theme=>{
 document.documentElement.dataset.theme=theme;
 window.LANG='en';
 window.t=key=>({
  'dashboard.memory_strategy_mode':'Strategy','dashboard.memory_strategy_reason':'Reason','dashboard.memory_strategy_mode_balanced':'Balanced',
  'dashboard.memory_health_retrieved':'Retrieved','dashboard.memory_health_predicted':'Predicted','dashboard.memory_health_distinct':'Distinct','dashboard.memory_health_total':'Tracked','dashboard.memory_health_confirmed_rate':'Confirmed rate','dashboard.memory_health_unverified':'Unverified','dashboard.memory_curator_archived':'Archived','dashboard.memory_health_stale':'Stale','dashboard.memory_health_recent_episodes':'Recent episodes','dashboard.memory_pending_title':'Pending actions','dashboard.memory_conflicts_title':'Conflicts','dashboard.memory_pending_writes':'Pending writes','dashboard.memory_exhausted_writes':'Exhausted writes','dashboard.memory_terminal_consolidations':'Terminal consolidations','dashboard.memory_reflection_actionables':'Reflection actionables','dashboard.memory_corpus_documents':'Corpus documents','dashboard.memory_corpus_dimensions':'Vector dimensions','dashboard.memory_corpus_storage':'Corpus storage','dashboard.memory_corpus_vector_estimate':'Vector estimate'
 }[key]||key);
 window.esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
 window.DashSections={register(){}};window.DashDisclosures={setCount(){}};
 const data={pending_memory_writes:2,exhausted_memory_writes:3,terminal_consolidation_failures:4,memory_health:{strategy:{mode:'balanced'},confidence:{},usage:{},corpus:{available:true,documents:17,dimensions:[384],database_bytes:1048576,wal_bytes:524288,vector_payload_bytes:262144}},episodic:{},pending_actions:[],memory_conflicts:[]};
 renderMemoryHealth(data);
 const summary=document.getElementById('memory-health-summary');
 const text=summary.textContent;
 for(const [label,value] of [['Pending writes','2'],['Exhausted writes','3'],['Terminal consolidations','4'],['Corpus documents','17'],['Vector dimensions','384']]){
   const cell=[...summary.querySelectorAll('.memory-health-item')].find(node=>node.querySelector('.memory-health-label')?.textContent===label);
   if(!cell||cell.querySelector('.memory-health-value')?.textContent!==value)throw Error('missing dashboard metric '+label+': '+text);
 }
 const storage=[...summary.querySelectorAll('.memory-health-item')].find(node=>node.querySelector('.memory-health-label')?.textContent==='Corpus storage');
 if(!storage||storage.querySelector('.memory-health-value')?.textContent.replace(',','.')!=='1.5 MiB')throw Error('missing dashboard corpus storage metric: '+text);
 renderMemoryHealth({...data,exhausted_memory_writes:null,terminal_consolidation_failures:null,memory_health:{...data.memory_health,corpus:{available:false}}});
 for(const label of ['Exhausted writes','Terminal consolidations','Corpus documents','Vector dimensions','Corpus storage','Vector estimate']){
   const cell=[...summary.querySelectorAll('.memory-health-item')].find(node=>node.querySelector('.memory-health-label')?.textContent===label);
   if(!cell||cell.querySelector('.memory-health-value')?.textContent!=='—')throw Error('missing metric must be unknown, not zero: '+label);
 }
 const card=document.getElementById('card-memory');
 if(!getComputedStyle(card).backgroundColor)throw Error('production dashboard styles were not applied');
 return true;
 }`, theme)
			if !result.Bool() {
				t.Fatal("dashboard memory metrics were not rendered")
			}
		})
	}
}

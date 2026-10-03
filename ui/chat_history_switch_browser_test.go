package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestChatHistorySwitchDropsLateBatchesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><div id="chat"></div></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture").Timeout(30 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	result := page.MustEval(`async()=>{
 const source=await(await fetch('/js/chat/chat-history.js')).text();
 window.activeSession='A';window.SessionDrawer={getActiveSessionId:()=>activeSession};
 window.chatContent=document.getElementById('chat');window.conversation=[];window.debugMode=false;
 window.isDebugOnlyHistoryMessage=()=>false;window.hideTodoPanel=()=>{};window.t=key=>key;
 window.appendMessage=(_role,content)=>{const item=document.createElement('p');item.textContent=content;chatContent.appendChild(item)};
 const pendingFrames=[];window.requestAnimationFrame=cb=>{pendingFrames.push(cb);return pendingFrames.length};
 window.fetch=async url=>{
   if(String(url).startsWith('/api/plans/active'))return {ok:true,json:async()=>({plan:null})};
   if(String(url).includes('session_id=A'))return {ok:true,json:async()=>Array.from({length:45},(_,i)=>({role:'assistant',content:'A-'+i}))};
   if(String(url).includes('session_id=B'))return {ok:true,json:async()=>[{role:'assistant',content:'B-only'}]};
   throw Error('unexpected '+url);
 };
 window.updatePlanPanel=()=>{};
 (0,eval)(source.replace(/if \(document\.readyState === 'loading'\)[\s\S]*$/, ''));
 const until=async predicate=>{for(let i=0;i<100;i++){if(predicate())return;await new Promise(r=>setTimeout(r,10))}throw Error('history condition timed out')};
 const first=window.onSessionSwitch('A');await until(()=>chatContent.children.length===20&&pendingFrames.length===1);
 activeSession='B';await window.onSessionSwitch('B');
 pendingFrames.shift()(0);await first;
 const shown=[...chatContent.children].map(x=>x.textContent);
 if(shown.length!==1||shown[0]!=='B-only'||conversation.length!==1||conversation[0].content!=='B-only')throw Error('old history leaked: '+JSON.stringify({shown,conversation}));
 return true;
 }`)
	if !result.Bool() {
		t.Fatal("old history batch leaked into new session")
	}
}

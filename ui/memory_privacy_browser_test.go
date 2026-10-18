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

func TestMemoryPrivacyControlsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="stylesheet" href="/css/session-drawer.css"></head><body><div id="session-drawer"><div id="session-list"></div></div><div id="session-backdrop"></div><button id="session-toggle-btn"></button><button id="session-drawer-close"></button><button id="session-new-btn"></button><div id="memory-note-protection"></div></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page := browser.MustPage(srv.URL + "/fixture").Timeout(30 * time.Second)
			defer page.Close()
			page.MustWaitLoad()
			result := page.MustEval(`async(theme)=>{
 document.documentElement.dataset.theme=theme;
 const drawerSource=await(await fetch('/js/chat/modules/session-drawer.js')).text();
 const widgetsSource=await(await fetch('/js/dashboard/dashboard-widgets.js')).text();
 const until=async fn=>{for(let i=0;i<100;i++){if(fn())return;await new Promise(r=>setTimeout(r,10))}throw Error('condition timed out')};
 const deleted=[],patches=[],reports=[];
 window.I18N={'chat.sessions_delete_report':'deleted {deleted}; retained {retained}'};
 window.t=k=>k;window.showAlert=async(title,text)=>reports.push(text);
 window.fetch=async(url,options={})=>{
   if(url==='/api/dashboard/notes'&&options.method==='PATCH'){patches.push(JSON.parse(options.body));return {ok:true,json:async()=>({status:'ok'})}}
   if(url==='/api/dashboard/notes')return {ok:true,json:async()=>({notes:[{id:1,title:'Private note',protected:true,keep_forever:false}]})};
   if(options.method==='DELETE'){deleted.push(String(url));return {ok:true,json:async()=>({deletion:{deleted_memories:2,retained_memories:1}})}}
   if(String(url).startsWith('/api/chat/sessions/'))return {ok:true,json:async()=>({session:{id:'default'}})};
   if(url==='/api/chat/sessions')return {ok:true,json:async()=>({sessions:[{id:'private',preview:'Private chat',message_count:1}]})};
   throw Error('unexpected '+url);
 };
 localStorage.setItem('aurago-session-id','default');
 (0,eval)(drawerSource);SessionDrawer.init();SessionDrawer.open();
 await until(()=>document.querySelector('[data-delete-id]'));
 document.querySelector('[data-delete-id]').click();await until(()=>document.querySelector('dialog[open]'));
 let dialog=document.querySelector('dialog');if(dialog.querySelector('input').checked)throw Error('memory deletion must default off');
 dialog.querySelector('button[value=cancel]').click();await until(()=>!document.querySelector('dialog'));if(deleted.length)throw Error('cancel deleted data');
 document.querySelector('[data-delete-id]').click();await until(()=>document.querySelector('dialog[open]'));
 document.querySelector('dialog button[value=delete]').click();await until(()=>deleted.length===1);
 if(!deleted[0].endsWith('delete_memories=false'))throw Error('default requested memory deletion');
 await until(()=>!document.querySelector('dialog'));
 document.querySelector('[data-delete-id]').click();await until(()=>document.querySelector('dialog[open]'));
 dialog=document.querySelector('dialog');dialog.querySelector('input').checked=true;dialog.querySelector('button[value=delete]').click();
 await until(()=>deleted.length===2&&reports.length===1);
 if(!deleted[1].endsWith('delete_memories=true')||!reports[0].includes('retained 1'))throw Error('missing explicit scope/report');
 (0,eval)(widgetsSource);await renderManagedNoteProtection();
 const flags=document.querySelectorAll('#memory-note-protection input');
 if(flags.length!==2||!flags[0].checked||flags[1].checked)throw Error('persisted note flags not displayed');
 flags[0].click();await until(()=>patches.length===1&&!flags[0].disabled);
 if(patches[0].protected!==false||patches[0].keep_forever!==false||patches[0].id!==1)throw Error('explicit unlock was not saved');
 flags[1].click();await until(()=>patches.length===2&&!flags[1].disabled);
 if(patches[1].keep_forever!==true)throw Error('forever flag was not saved');
 return true;
 }`, theme)
			if !result.Bool() {
				t.Fatal("privacy controls failed")
			}
		})
	}
}

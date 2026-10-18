package ui

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestContactPrivacyControlsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="stylesheet" href="/css/desktop-app-people.css"></head><body><main id="people"></main><div id="delete-modal"><p id="delete-confirm-text"></p><div id="contact-delete-scope" hidden><input id="contact-delete-memories" type="checkbox"></div><input id="delete-target-id"><input id="delete-target-type"><button id="knowledge-delete-confirm-btn"></button></div></body></html>`)
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
 const peopleSource=await(await fetch('/js/desktop/apps/people.js')).text();
 const knowledgeSource=await(await fetch('/js/knowledge/main.js')).text();
 document.documentElement.dataset.theme=theme;
 const until=async fn=>{for(let i=0;i<100;i++){if(fn())return;await new Promise(r=>setTimeout(r,10))}throw Error('condition timed out')};
 const deleted=[],messages=[];let fail=false;
 window.fetch=async(url,options={})=>{
  if(options.method==='DELETE'){
   deleted.push(String(url));if(fail){fail=false;return {ok:false,status:503,text:async()=> 'unavailable'}}
   const optional=String(url).endsWith('delete_memories=true');
   return {ok:true,json:async()=>({status:optional?'partial':'deleted',contact_deleted:true,deleted_graph_items:optional?1:0,retained_graph_items:1})};
  }
  return {ok:true,json:async()=>String(url).includes('/contacts')?[{id:'private',name:'Private Contact'}]:[]};
 };
 window.t=(key,vars)=>Object.entries(vars||{}).reduce((text,[name,value])=>text.replace('{'+name+'}',value),key);
 window.showToast=(message)=>messages.push(message);window.closeModal=id=>document.getElementById(id).classList.remove('active');
 (0,eval)(peopleSource);
 PeopleApp.render(document.getElementById('people'),'fixture',{t,notify:value=>messages.push(value.message)});
 await until(()=>document.querySelector('.vd-people-card'));
 const open=async()=>{document.querySelector('.vd-people-card').click();await until(()=>document.querySelector('.vd-people-detail-delete'));document.querySelector('.vd-people-detail-delete').click();await until(()=>document.querySelector('.vd-people-modal-backdrop.active'));};
 await open();if(document.querySelector('.vd-people-delete-memories').checked)throw Error('People default deletes knowledge');
 document.querySelector('.vd-people-modal-cancel').click();await new Promise(r=>setTimeout(r,20));if(deleted.length)throw Error('cancel deleted contact');
 await open();document.querySelector('.vd-people-modal-save').click();await until(()=>deleted.length===1&&messages.length===1);
 if(!deleted[0].endsWith('delete_memories=false'))throw Error('People missing default scope');
 await until(()=>!document.querySelector('.vd-people-modal-backdrop.active'));await open();document.querySelector('.vd-people-delete-memories').checked=true;document.querySelector('.vd-people-modal-save').click();await until(()=>deleted.length===2&&messages.length===2);
 if(!deleted[1].endsWith('delete_memories=true')||!messages[1].includes('delete_partial'))throw Error('People missing optin/partial report');
 await open();fail=true;document.querySelector('.vd-people-modal-save').click();await until(()=>deleted.length===3&&messages.length===3);await new Promise(r=>setTimeout(r,30));if(deleted.length!==3)throw Error('People silently retried failed deletion');
 PeopleApp.dispose('fixture');
 (0,eval)(knowledgeSource);window.loadContacts=()=>{};
 askDeleteContact('private','Private Contact');if(document.getElementById('contact-delete-memories').checked)throw Error('Knowledge default deletes memories');
 await confirmDelete();if(!deleted[3].endsWith('delete_memories=false'))throw Error('Knowledge missing default scope');
 askDeleteContact('private','Private Contact');document.getElementById('contact-delete-memories').checked=true;await confirmDelete();
 if(!deleted[4].endsWith('delete_memories=true')||!messages.at(-1).includes('delete_partial'))throw Error('Knowledge missing optin/partial report');
 askDeleteContact('private','Private Contact');if(document.getElementById('contact-delete-memories').checked)throw Error('stale optin reused');
 return true;
 }`, theme)
			if !result.Bool() {
				t.Fatal("contact privacy controls failed")
			}
		})
	}
}

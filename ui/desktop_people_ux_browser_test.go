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

// TestDesktopPeopleUxBrowser covers the address-book polish: the relationship
// filter and the search query combine, a search without hits gets its own
// empty state, cards open with the keyboard, Escape closes the detail panel and
// knowledge-graph persons stay read only.
func TestDesktopPeopleUxBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="stylesheet" href="/css/desktop-app-people.css"></head><body><main id="people"></main></body></html>`)
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
 const peopleSource=await(await fetch('/js/desktop/apps/people.js')).text();
 const contacts=[
   {id:'c-1',name:'Ada Lovelace',relationship:'friend',email:'ada@example.test',phone:'+49123456789',birthday:'1990-05-04'},
   {id:'c-2',name:'Grace Hopper',relationship:'friend',email:'grace@example.test'},
   {id:'c-3',name:'Alan Turing',relationship:'colleague',email:'alan@example.test'}
 ];
 const kgPersons=[{id:'kg-1',label:'Graph Person',properties:{email:'graph@example.test'}}];
 const until=async fn=>{for(let i=0;i<200;i++){if(fn())return;await new Promise(r=>setTimeout(r,10))}throw Error('condition timed out')};
 window.fetch=async(url)=>{
  const path=String(url);
  if(path.includes('/people/kg-persons'))return {ok:true,json:async()=>kgPersons};
  if(path.includes('/people/lookup'))return {ok:true,json:async()=>({nodes:[]})};
  if(path.includes('/people/upcoming'))return {ok:true,json:async()=>[]};
  if(path.includes('/contacts')){
   const query=new URL(path,'http://localhost').searchParams.get('q');
   if(query)return {ok:true,json:async()=>contacts.filter(c=>c.name.toLowerCase().includes(query.toLowerCase()))};
   return {ok:true,json:async()=>contacts};
  }
  return {ok:true,json:async()=>[]};
 };
 window.t=(key,vars)=>Object.entries(vars||{}).reduce((text,[name,value])=>text.replace('{'+name+'}',value),key);
 (0,eval)(peopleSource);
 PeopleApp.render(document.getElementById('people'),'ux',{t,notify:()=>{}});
 await until(()=>document.querySelectorAll('.vd-people-card').length===3);

 const cards=()=>document.querySelectorAll('.vd-people-card');
 const names=()=>[...cards()].map(card=>card.querySelector('.vd-people-card-name').textContent);
 const failures=[];

 // The sidebar counts every contact and offers the relationship groups.
 const allCount=document.querySelector('.vd-people-sidebar-item[data-filter="all"] .vd-people-count')?.textContent;
 if(allCount!=='3')failures.push('sidebar count is '+allCount);
 if(!document.querySelector('.vd-people-sidebar-item[data-filter="friend"]'))failures.push('relationship group missing');

 // A relationship filter narrows the grid.
 document.querySelector('.vd-people-sidebar-item[data-filter="friend"]').click();
 await until(()=>cards().length===2);
 if(!names().every(name=>name!=='Alan Turing'))failures.push('group filter kept a foreign contact');

 // The search query combines with the active group filter.
 const search=document.querySelector('.vd-people-search');
 search.value='Alan';
 search.dispatchEvent(new Event('input',{bubbles:true}));
 await until(()=>document.querySelector('.vd-people-content .vd-people-empty'));
 const emptyTitle=document.querySelector('.vd-people-empty-title')?.textContent||'';
 if(!emptyTitle.includes('no_results_title'))failures.push('search empty state is '+emptyTitle);
 if(!document.querySelector('.vd-people-empty-action[data-action="clear-search"]'))failures.push('search empty state has no reset');

 // Clearing the search keeps the group filter active.
 document.querySelector('.vd-people-empty-action[data-action="clear-search"]').click();
 await until(()=>cards().length===2);

 // The keyboard opens a contact and Escape closes the detail again.
 const first=cards()[0];
 first.focus();
 first.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
 await until(()=>document.querySelector('.vd-people-detail')?.classList.contains('open'));
 if(document.querySelector('.vd-people-detail-name')?.textContent!=='Ada Lovelace')failures.push('keyboard did not open the first contact');
 document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
 await until(()=>!document.querySelector('.vd-people-detail')?.classList.contains('open'));

 // Knowledge-graph persons are read only.
 document.querySelector('.vd-people-sidebar-item[data-filter="kg"]').click();
 await until(()=>cards().length===1);
 cards()[0].click();
 await until(()=>document.querySelector('.vd-people-detail')?.classList.contains('open'));
 if(document.querySelector('.vd-people-detail-delete'))failures.push('knowledge graph person offers a delete action');
 if(!document.querySelector('.vd-people-detail-note'))failures.push('knowledge graph person has no read-only note');

 PeopleApp.dispose('ux');
 return failures.length?failures.join(' | '):'ok';
}`).Str()
	if result != "ok" {
		t.Fatalf("people address book ux regressions: %s", result)
	}
}

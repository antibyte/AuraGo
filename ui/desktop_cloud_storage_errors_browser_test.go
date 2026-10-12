package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDesktopCloudStorageErrorsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, cloudStorageBrowserHTML)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{1280, 430} {
			t.Run(fmt.Sprintf("%s_%d", theme, width), func(t *testing.T) {
				page := browser.MustPage().Timeout(25 * time.Second)
				defer page.Close()
				page.MustSetViewport(width, 850, 1, false)
				page.MustNavigate(server.URL + "/fixture?theme=" + theme).MustWaitLoad()
				_, err := page.Eval(`async()=>{
const wait=async(fn,label)=>{const end=Date.now()+3000;while(!fn()){if(Date.now()>end)throw Error(label+' '+JSON.stringify(__errors));await new Promise(resolve=>setTimeout(resolve,10));}};
await wait(()=>document.querySelector('#one [data-path="Documents/same.txt"]')&&test.pathChanges.two,'initial listing');
const originalFetch=window.fetch;let failure=null;
window.fetch=async(url,options)=>{
 const route=new URL(url,location.origin).pathname;
 if(failure&&route==='/api/desktop/storage/'+failure.endpoint){
  if(failure.stream)return new Response('data: '+JSON.stringify({status:'error',error:failure.error,partial:true})+'\n\n',{headers:{'Content-Type':'text/event-stream'}});
  return new Response(JSON.stringify({error:failure.error}),{status:failure.status,headers:{'Content-Type':'application/json'}});
 }
 return originalFetch(url,options);
};
const loc={provider:'koofr',mount_id:'shared',path:''};
for(const [code,status,http] of [['storage_access_denied',403,403],['storage_unavailable',503,500]]){
 test.notifications.length=0;failure={endpoint:'files',error:code+' (HTTP '+http+')',status};
 FileManager.navigateTo('one',loc);
 await wait(()=>test.notifications.length,'listing failure');
 const message=test.notifications.at(-1).message;
 if(!message.includes(strings['desktop.fm.'+code])||!message.includes('(HTTP '+http+')')||message.includes('storage_'))throw Error('untranslated provider failure: '+message);
 if(test.pathChanges.two.provider!=='local')throw Error('sibling window changed');
}
failure=null;FileManager.navigateTo('one',loc);
await wait(()=>document.querySelector('#one [data-path="same.txt"]'),'retry did not restore listing');
test.notifications.length=0;failure={endpoint:'transfer',stream:true,error:'storage_unavailable (HTTP 503)'};
const completed=await FileManager.transferStorage('one',[{...loc,path:'same.txt'}],{provider:'local',path:'Documents'},'copy');
const message=test.notifications.at(-1)?.message||'';
if(completed||!message.includes(strings['desktop.fm.storage_unavailable'])||!message.includes(strings['desktop.fm.storage_partial'])||message.includes('storage_unavailable'))throw Error('transfer failure lost its localized or partial status: '+message);
if(__errors.length)throw Error(JSON.stringify(__errors));
}`)
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

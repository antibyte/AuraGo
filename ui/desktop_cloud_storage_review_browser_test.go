package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDesktopCloudStorageReviewBrowser(t *testing.T) {
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
	checks := map[string]string{
		"policy_keeps_readable_listing": `
click('[data-storage-provider="webdav"]');await wait(()=>test.pathChanges.one?.provider==='webdav'&&document.querySelector('#one [data-path="same.txt"]'),'WebDAV');
test.policy.webdav_readonly=true;document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{webdav_visible:true,webdav_readonly:true}}));
await wait(()=>root().dataset.readonly==='true','readonly');
await wait(()=>document.querySelector('#one [data-path="same.txt"]'),'readable files disappeared after policy update');`,
		"policy_does_not_cancel_local_listing": `
const apiBefore=api;let resolveListing;window.api=async(url,options)=>url.includes('/api/desktop/files?')?new Promise(resolve=>resolveListing=resolve):apiBefore(url,options);
FileManager.render(document.querySelector('#one'),'one','Documents',callbacks('one'));
await wait(()=>resolveListing,'pending local listing');document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:false}}));
resolveListing({files:makeFiles('Documents')});await wait(()=>document.querySelector('#one [data-path="Documents/same.txt"]'),'local listing lost to cloud policy event');`,
		"unrelated_policy_keeps_transfer": `
click('[data-storage-provider="webdav"]');await wait(()=>document.querySelector('#one [data-path="same.txt"]'),'WebDAV files');test.hold=true;
const pending=FileManager.transferStorage('one',[{provider:'webdav',path:'same.txt'}],{provider:'local',path:'Documents'},'copy');
await wait(()=>test.transfers.length&&document.querySelector('[data-storage-cancel]'),'pending transfer');
const reads=test.calls.filter(url=>url.endsWith('/sources')).length;
document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{serial_browser_enabled:false}}));
await wait(()=>test.calls.filter(url=>url.endsWith('/sources')).length>reads,'policy refresh');await new Promise(resolve=>setTimeout(resolve,50));
if(test.aborted||!document.querySelector('[data-storage-cancel]'))throw Error('unrelated policy cancelled transfer');
document.querySelector('[data-storage-cancel]').click();await pending;`,
		"revoked_policy_cancels_transfer": `
click('[data-storage-provider="webdav"]');await wait(()=>document.querySelector('#one [data-path="same.txt"]'),'WebDAV files');test.hold=true;
const pending=FileManager.transferStorage('one',[{provider:'webdav',path:'same.txt'}],{provider:'local',path:'Documents'},'copy');
await wait(()=>test.transfers.length&&document.querySelector('[data-storage-cancel]'),'pending transfer');
test.policy.webdav=false;document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{webdav_visible:false}}));
await pending;if(!test.aborted)throw Error('revoked transfer remained active');
await wait(()=>test.pathChanges.one?.provider==='local','revoked location cleared');`,
		"mount_permissions_reload": `
const fetchBefore=fetch;window.fetch=async(url,options)=>{const result=await fetchBefore(url,options);if(!String(url).endsWith('/mounts'))return result;const data=await result.json();if(test.policy.koofr_readonly)data.mounts.forEach(m=>m.write=false);return new Response(JSON.stringify(data));};
test.policy.koofr_readonly=true;document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{koofr_readonly:true}}));
click('[data-storage-group]');await wait(()=>document.querySelector('#one [data-storage-mount="shared"]'),'mounts');click('[data-storage-mount="shared"]');
await wait(()=>root().dataset.readonly==='true','read-only mount');const reads=test.mountCalls;
test.policy.koofr_readonly=false;document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{koofr_readonly:false}}));
await wait(()=>test.mountCalls>reads&&root().dataset.readonly!=='true','mount permissions remained stale');`,
		"split_navigation_retains_provider": `
click('[data-action="toggle-split"]');const start=test.calls.length;click('[data-storage-provider="webdav"]');
await wait(()=>test.pathChanges.one?.provider==='webdav','navigation callback');
await new Promise(resolve=>setTimeout(resolve,50));
if(test.calls.slice(start).some(url=>url.startsWith('/api/desktop/files?')))throw Error('cloud navigation reached local file API in split view');
if(!document.querySelector('#one [data-fm-breadcrumb]').textContent.includes('WebDAV'))throw Error('split renderer replaced provider identity');`,
		"split_restarts_aborted_listing": `
click('[data-action="toggle-split"]');const fetchBefore=fetch;let held=false,aborted=false;
window.fetch=(url,options={})=>String(url).includes('/storage/files?')?new Promise((resolve,reject)=>{held=true;options.signal.addEventListener('abort',()=>{aborted=true;reject(new DOMException('Cancelled','AbortError'));});}):fetchBefore(url,options);
click('[data-storage-provider="webdav"]');await wait(()=>held,'pending cloud list');
document.querySelector('#one .fm-pane[data-pane="right"]').dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
await wait(()=>aborted&&test.pathChanges.one?.provider==='local','switch to loaded local pane');
if(!document.querySelector('#one .fm-pane[data-pane="right"] [data-path="Documents/same.txt"]'))throw Error('local pane inherited remote loading state');
window.fetch=fetchBefore;document.querySelector('#one .fm-pane[data-pane="left"]').dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
await wait(()=>test.pathChanges.one?.provider==='webdav'&&document.querySelector('#one .fm-pane[data-pane="left"] [data-path="same.txt"]'),'aborted pane did not reload');`,
		"split_drop_targets_inactive_provider": `
click('[data-storage-provider="webdav"]');await wait(()=>document.querySelector('#one [data-path="same.txt"]'),'WebDAV files');click('[data-action="toggle-split"]');
click('[data-storage-group]');await wait(()=>document.querySelector('#one [data-storage-mount="shared"]'),'mounts');click('[data-storage-mount="shared"]');
await wait(()=>test.pathChanges.one?.mount_id==='shared','left Koofr');await new Promise(resolve=>setTimeout(resolve,50));
const transfer=new DataTransfer();transfer.setData('application/x-aurago-storage-files',JSON.stringify({entries:[{provider:'koofr',mount_id:'primary',path:'test.txt'}]}));
document.querySelector('#one .fm-pane[data-pane="right"] [data-fm-main]').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,ctrlKey:true,dataTransfer:transfer}));
await wait(()=>test.transfers.length,'drop transfer');if(test.transfers[0].destination.provider!=='webdav')throw Error('inactive pane drop used active provider');`,
		"split_drop_checks_destination_rights": `
click('[data-storage-provider="webdav"]');await wait(()=>document.querySelector('#one [data-path="same.txt"]'),'WebDAV files');click('[data-action="toggle-split"]');
click('[data-storage-group]');await wait(()=>document.querySelector('#one [data-storage-mount="readonly"]'),'mounts');click('[data-storage-mount="readonly"]');
await wait(()=>root().dataset.readonly==='true','read-only active pane');
const transfer=new DataTransfer();transfer.setData('application/x-aurago-desktop-files',JSON.stringify({paths:['Documents/test.txt']}));
document.querySelector('#one .fm-pane[data-pane="right"] [data-fm-main]').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,ctrlKey:true,dataTransfer:transfer}));
await wait(()=>test.transfers.length&&!document.querySelector('[data-storage-cancel]'),'writable inactive pane drop');
if(test.transfers[0].destination.provider!=='webdav'||test.transfers[0].location.provider!=='local')throw Error('drop identity');
document.querySelector('#one .fm-pane[data-pane="right"]').dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
await wait(()=>root().dataset.readonly!=='true','pane permissions did not update');
if(test.menus.one.find(m=>m.id==='file').items.find(m=>m.id==='new-file').disabled)throw Error('pane write menu remained disabled');
const denied=new DataTransfer();denied.setData('application/x-aurago-storage-files',JSON.stringify({entries:[{provider:'webdav',path:'same.txt'}]}));
document.querySelector('#one .fm-pane[data-pane="left"] [data-fm-main]').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,ctrlKey:true,dataTransfer:denied}));
await new Promise(resolve=>setTimeout(resolve,50));if(test.transfers.length!==1)throw Error('drop to read-only pane started a transfer');`,
	}
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{1280, 430} {
			for name, check := range checks {
				t.Run(fmt.Sprintf("%s_%d_%s", theme, width, name), func(t *testing.T) {
					page := browser.MustPage().Timeout(20 * time.Second)
					defer page.Close()
					page.MustSetViewport(width, 850, 1, false)
					page.MustNavigate(server.URL + "/fixture?theme=" + theme).MustWaitLoad()
					_, err := page.Eval(`async()=>{
const wait=async(fn,label)=>{const end=Date.now()+2500;while(!fn()){if(Date.now()>end)throw Error(label+' '+JSON.stringify(__errors));await new Promise(resolve=>setTimeout(resolve,10));}};
const root=()=>document.querySelector('#one .file-manager');const click=selector=>{root().dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));const node=document.querySelector('#one '+selector);if(!node)throw Error('missing '+selector);node.click();};
await wait(()=>root()&&document.querySelector('#one [data-storage-provider="webdav"]')&&document.querySelector('#one [data-path="Documents/same.txt"]'),'initial listing');
` + check + `
if(__errors.length)throw Error(JSON.stringify(__errors));}`)
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDesktopCloudStorageBrowser(t *testing.T) {
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
				page := browser.MustPage().Timeout(35 * time.Second)
				defer page.Close()
				page.MustSetViewport(width, 850, 1, false)
				page.MustNavigate(server.URL + "/fixture?theme=" + theme).MustWaitLoad()
				page.MustEval(cloudStorageBrowserChecks)
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
					page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("cloud-storage-%s-%d.png", theme, width)))
				}
			})
		}
	}
}

const cloudStorageBrowserHTML = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/css/desktop-base.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-common.css"><link rel="stylesheet" href="/css/desktop-app-file-manager.css"><link rel="stylesheet" href="/css/desktop-app-chat.css"><link rel="stylesheet" href="/css/desktop-shell-overrides.css">
<style>body{margin:0;background:var(--vd-bg,#20242d);color:var(--vd-text,#eee)}.host{height:600px;width:100%;max-width:100%;overflow:hidden}#two{display:none}#content{display:none}</style>
<div id="one" class="host"></div><div id="two" class="host"></div><div id="content"></div>
<script>
document.body.className='desktop-body';document.body.dataset.theme=new URLSearchParams(location.search).get('theme');document.body.dataset.fruityMode='light';
window.__errors=[];addEventListener('error',e=>__errors.push(e.error?.stack||e.message));addEventListener('unhandledrejection',e=>__errors.push(String(e.reason)));
window.test={calls:[],menus:{},policy:{webdav:true,koofr:true,readonly:false},mountCalls:0,transfers:[],conflicts:0,hold:false,aborted:false,confirmations:[],notifications:[],pathChanges:{}};
const mounts=[{id:'primary',name:'Koofr',online:true,read:true,write:true},{id:'shared',name:'Connected Drive',online:true,read:true,write:true},{id:'readonly',name:'Shared photos',online:true,read:true,write:false},{id:'offline',name:'Unavailable',online:false,read:true,write:true}];
const makeFiles=path=>[{name:'same.txt',path:(path?path+'/':'')+'same.txt',type:'file',size:3,version:'"v1"',content_type:'text/plain'},{name:'Folder',path:(path?path+'/':'')+'Folder',type:'directory',size:0}];
const nativeFetch=window.fetch.bind(window);window.fetch=async(url,options={})=>{const u=new URL(url,location.origin);if(!u.pathname.startsWith('/api/'))return nativeFetch(url,options);test.calls.push(u.pathname+u.search);
let body={};if(u.pathname.endsWith('/sources'))body=test.policy;
else if(u.pathname.endsWith('/mounts')){test.mountCalls++;body={mounts};}
else if(u.pathname.endsWith('/files'))body={files:makeFiles(u.searchParams.get('path')||'')};
else if(u.pathname.endsWith('/content'))return new Response('cloud preview');
else if(u.pathname.endsWith('/transfer')){const request=JSON.parse(options.body);test.transfers.push(request);
if(test.conflictNext){test.conflictNext=false;return new Response(JSON.stringify({code:'file_conflict',conflict:{path:request.destination.path,version:'"observed"'}}),{status:409,headers:{'Content-Type':'application/json'}});}
if(test.hold)return new Response(new ReadableStream({start(controller){controller.enqueue(new TextEncoder().encode('data: {"status":"started","total":3}\n\n'));options.signal.addEventListener('abort',()=>{test.aborted=true;controller.error(new DOMException('Cancelled','AbortError'));});}}),{headers:{'Content-Type':'text/event-stream'}});
return new Response('data: {"status":"started","total":3}\n\ndata: {"status":"progress","done":3,"total":3}\n\ndata: {"status":"done"}\n\n',{headers:{'Content-Type':'text/event-stream'}});}
return new Response(JSON.stringify(body),{headers:{'Content-Type':'application/json'}});};
window.api=async(url,options)=>{if(url.startsWith('/api/desktop/')&&!url.startsWith('/api/desktop/storage/')&&!url.startsWith('/api/desktop/files?'))throw Error('cloud reached local operation: '+url);return (await fetch(url,options)).json();};
window.t=(key,vars={})=>{let value=window.strings?.[key]||key;for(const [k,v]of Object.entries(vars))value=value.replaceAll('{{'+k+'}}',v).replaceAll('{'+k+'}',v);return value};
window.esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));window.escapeHtml=esc;window.escapeAttr=esc;
window.state={};window.normalizeIconName=key=>String(key);window.versionedIconAssetPath=path=>path;window.isFruityTheme=()=>document.body.dataset.theme==='fruity';window.iconUrlStyle=path=>'url('+path+')';window.iconMarkup=(key,fallback,css,size)=>miniIconMarkup(key,css,size)||fallback;
window.configData={webdav:{enabled:false},koofr:{enabled:false}};window.cfgSecretPlaceholder=()=>'';window.markDirty=()=>{test.dirty=true};window.setNestedValue=(obj,path,value)=>{const [a,b]=path.split('.');obj[a][b]=value};
window.toggleBool=node=>{const [a,b]=node.dataset.path.split('.');configData[a][b]=!configData[a][b];node.classList.toggle('on',configData[a][b]);markDirty()};
function callbacks(id){return{api,t,esc,iconMarkup,directories:['Documents'],readonly:false,maxFileSize:1048576,showNotification:n=>test.notifications.push(n),setWindowMenus:(id,menus)=>test.menus[id]=menus,clearWindowMenus(){},closeContextMenu(){document.querySelectorAll('.fm-context-menu').forEach(node=>node.remove())},promptDialog:async()=> 'new.txt',confirmDialog:async text=>{test.confirmations.push(text);return true},storageConflict:async()=>{test.conflicts++;return test.replace?'replace':'copy'},openFile(){throw Error('cloud editor opened directly')},openMedia(){throw Error('cloud media opened as local')},onPathChange:(path,loc)=>test.pathChanges[id]=loc};}
</script><script src="/cfg/form-builder.js"></script><script src="/cfg/webdav.js"></script><script src="/cfg/koofr.js"></script><script src="/js/desktop/core/mini-icons-runtime.js"></script><script src="/js/desktop/core/session-runtime.js"></script><script src="/js/desktop/bundles/file-manager.bundle.js"></script><script>
Promise.all([fetch('/lang/desktop/de.json').then(r=>r.json()),fetch('/img/desktop-mini/manifest.json').then(r=>r.json())]).then(([strings,manifest])=>{window.strings=strings;state.miniIconManifest=manifest;refreshMiniIconTheme();FileManager.render(document.querySelector('#one'),'one','Documents',callbacks('one'));FileManager.render(document.querySelector('#two'),'two','Documents',callbacks('two'));});
</script>`

const cloudStorageBrowserChecks = `async()=>{
const wait=async(fn,label)=>{const end=Date.now()+5000;while(!fn()){if(Date.now()>end)throw Error(label+' '+JSON.stringify(__errors));await new Promise(r=>setTimeout(r,10))}};
const root=()=>document.querySelector('#one .file-manager');const click=selector=>{const node=document.querySelector('#one '+selector);if(!node)throw Error('missing '+selector);root().dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));node.click()};
const menu=(id,group,action)=>test.menus[id].find(m=>m.id===group).items.find(m=>m.id===action).action();
const remote=()=>test.pathChanges.one?.provider==='webdav'&&document.querySelector('#one [data-path="same.txt"]');
await wait(()=>root()&&document.querySelector('#one [data-storage-provider="webdav"]'),'sources');
if(test.mountCalls!==0||test.calls.some(u=>u.startsWith('/api/desktop/storage/files')))throw Error('provider accessed before navigation');
for(const provider of ['webdav','koofr']){window[provider==='webdav'?'renderWebDAVSection':'renderKoofrSection']({icon:'',label:provider,desc:''});const input=document.querySelector('[data-path="'+provider+'.show_in_filemanager"]');if(!input)throw Error('config toggle missing');if(configData[provider].show_in_filemanager)throw Error('toggle defaults on');input.click();if(!configData[provider].show_in_filemanager)throw Error('toggle not saved to draft');}
click('[data-storage-provider="webdav"]');await wait(remote,'WebDAV');
click('[data-storage-group]');await wait(()=>test.mountCalls===1&&document.querySelector('[data-storage-mount="shared"]'),'lazy mounts');
if(!document.querySelector('[data-storage-mount="offline"]').disabled)throw Error('offline mount enabled');
click('[data-storage-mount="readonly"]');await wait(()=>test.pathChanges.one?.mount_id==='readonly'&&root().dataset.readonly==='true','mount readonly');
if(!test.menus.one.find(m=>m.id==='file').items.find(m=>m.id==='new-file').disabled)throw Error('readonly write menu');
click('[data-storage-mount="shared"]');await wait(()=>test.pathChanges.one?.mount_id==='shared'&&document.querySelector('#one [data-path="same.txt"]'),'shared mount');
const favDrop=new DataTransfer();document.querySelector('#one [data-path="Folder"]').dispatchEvent(new DragEvent('dragstart',{bubbles:true,dataTransfer:favDrop}));document.querySelector('#one .fm-favorites-section').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:favDrop}));
const favorite=JSON.parse(localStorage.getItem('aurago.fm.favorites')).find(loc=>loc.provider==='koofr'&&loc.mount_id==='shared');if(favorite?.path!=='Folder')throw Error('favorite identity');
const restored=sanitizeSessionContext({path:'Folder',storage:favorite});if(JSON.stringify(restored.storage)!==JSON.stringify(favorite)||sanitizeSessionContext({path:'Folder',storage:{provider:'unknown',path:'Folder'}}).path)throw Error('session identity');
click('[data-storage-provider="webdav"]');await wait(remote,'favorite navigation start');click('[data-fav-index="0"]');await wait(()=>test.pathChanges.one?.mount_id==='shared'&&test.pathChanges.one.path==='Folder','favorite source retained');
click('[data-storage-mount="readonly"]');await wait(()=>test.pathChanges.one?.mount_id==='readonly','history setup');click('[data-storage-mount="shared"]');await wait(()=>test.pathChanges.one?.mount_id==='shared','tab setup');
click('[data-action="new-tab"]');await wait(()=>document.querySelectorAll('#one .fm-tab').length===2,'new tab');
click('[data-storage-provider="webdav"]');await wait(remote,'webdav tab');click('.fm-tab[data-tab-index="0"]');await wait(()=>test.pathChanges.one?.mount_id==='shared','tab source retained');
click('[data-action="back"]');await wait(()=>test.pathChanges.one?.mount_id==='readonly','history source retained');
click('[data-storage-provider="webdav"]');await wait(remote,'copy source');click('[data-path="same.txt"]');menu('one','edit','copy');
if(window.AuraDesktopFileClipboard.entries?.[0].provider!=='webdav'||window.AuraDesktopFileClipboard.paths)throw Error('clipboard identity');
FileManager.navigateTo('two',{provider:'koofr',mount_id:'shared',path:'Folder'});await wait(()=>test.pathChanges.two?.path==='Folder','second window');
click('[data-storage-provider="local"]');await wait(()=>test.pathChanges.one?.provider==='local'&&document.querySelector('#one [data-path="same.txt"]'),'local destination');
test.conflictNext=true;await menu('one','edit','paste');await wait(()=>test.transfers.length===2&&!document.querySelector('[data-storage-cancel]'),'copy with conflict');
if(test.transfers[1].location.provider!=='webdav'||test.transfers[1].destination.provider!=='local'||test.transfers[1].destination.path!=='same (1).txt')throw Error('wrong transfer or conflict destination');
if(test.pathChanges.two.provider!=='koofr'||test.pathChanges.two.mount_id!=='shared'||test.pathChanges.two.path!=='Folder')throw Error('sibling window changed');
click('[data-storage-provider="webdav"]');await wait(remote,'preview source');click('[data-path="same.txt"]');document.querySelector('#one [data-path="same.txt"]').dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await wait(()=>document.querySelector('.fm-preview-text-pre')?.textContent==='cloud preview','read-only preview');
click('[data-path="same.txt"]');await menu('one','edit','delete');if(!test.confirmations.some(s=>s.includes('WebDAV')&&s.includes('Papierkorb')))throw Error('remote delete confirmation');
await wait(remote,'post delete refresh');const dt=new DataTransfer();dt.setData('application/x-aurago-storage-files',JSON.stringify({entries:[{provider:'koofr',mount_id:'shared',path:'Folder/same.txt'}]}));root().dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:dt,ctrlKey:true}));await wait(()=>test.transfers.length===3&&!document.querySelector('[data-storage-cancel]'),'cloud drag/drop');if(test.transfers[2].mode!=='copy'||test.transfers[2].location.mount_id!=='shared')throw Error('drag identity');
await wait(remote,'cancel source');click('[data-path="same.txt"]');menu('one','edit','copy');test.hold=true;const pending=FileManager.transferStorage('one',AuraDesktopFileClipboard.entries,{provider:'local',path:'Documents'},'copy');await wait(()=>document.querySelector('[data-storage-cancel]'),'progress');document.querySelector('[data-storage-cancel]').click();await pending;if(!test.aborted)throw Error('transfer not cancelled');
test.hold=false;test.policy.webdav=false;document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{webdav_visible:false}}));await wait(()=>test.pathChanges.one?.provider==='local'&&!document.querySelector('#one [data-storage-provider="webdav"]'),'live revocation');
if(__errors.length)throw Error(JSON.stringify(__errors));
test.policy.webdav=true;document.dispatchEvent(new CustomEvent('aurago:desktop-policy'));await wait(()=>document.querySelector('#one [data-storage-provider="webdav"]'),'restore source');click('[data-storage-provider="webdav"]');await wait(remote,'final view');if(innerWidth<820)click('[data-action="sidebar-toggle"]');
if(document.querySelector('#one .fm-preview-panel'))click('.fm-preview-close');if(document.querySelector('#one .fm-preview-panel'))throw Error('preview did not close');
if(innerWidth<820){if(document.querySelector('#one .fm-sidebar-backdrop').hidden)throw Error('sidebar backdrop lost after render');click('.fm-sidebar-backdrop');if(root().dataset.sidebarOpen)throw Error('sidebar did not close');click('[data-action="sidebar-toggle"]');}
if(document.documentElement.scrollWidth>innerWidth+2)throw Error('horizontal page overflow');
}`

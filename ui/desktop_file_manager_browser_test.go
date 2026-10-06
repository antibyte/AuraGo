package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFileManagerWindowAsyncAndRenameBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	parts := []string{
		"js/desktop/file-manager/core-render.js",
		"js/desktop/file-manager/core-render-components.js",
		"js/desktop/file-manager/actions-input.js",
		"js/desktop/file-manager/actions-operations.js",
		"js/desktop/file-manager/tabs.js",
		"js/desktop/file-manager/preview-panel.js",
		"js/desktop/file-manager/advanced-actions.js",
		"js/desktop/file-manager/lifecycle-export.js",
	}
	var bundle strings.Builder
	for _, path := range parts {
		content, err := Content.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		bundle.Write(content)
		bundle.WriteByte('\n')
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><div id="one"></div><div id="two"></div>
<script>
window.__errors=[];window.addEventListener('error',event=>__errors.push(event.error?.stack||event.message));
window.state={bootstrap:{},filesPath:'Documents'};
window.fmTest={pending:{},calls:[],mutations:[],holdMutations:false,pendingMutations:[],resolve(path,files){const queue=this.pending[path]||[];if(!queue.length)throw Error('no pending listing for '+path);queue.shift()({files});},resolveAll(path,files){while(this.pending[path]?.length)this.resolve(path,files);}};
window.api=(url,options={})=>{const u=new URL(url,location.origin);if(u.pathname==='/api/desktop/files'){const path=u.searchParams.get('path')||'';fmTest.calls.push(path);return new Promise(resolve=>{(fmTest.pending[path]||= []).push(resolve);});}if(u.pathname==='/api/desktop/file'&&options.method==='PATCH'){const body=JSON.parse(options.body||'{}');fmTest.mutations.push(body);return new Promise(resolve=>{const done=()=>resolve({path:body.new_path});if(fmTest.holdMutations)fmTest.pendingMutations.push(done);else done();});}return Promise.resolve({});};
window.t=(key)=>key;window.esc=(value)=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
window.iconMarkup=()=>'';window.showNotification=()=>{};window.isReadonly=()=>false;window.maxFileSize=()=>0;
</script><script src="/fm.js"></script><script>
const callbacks={directories:[],api:window.api,t:window.t,esc:window.esc,readonly:false,setWindowMenus(){},clearWindowMenus(){}};
try{FileManager.render(document.querySelector('#one'),'one','Documents/one',callbacks);
FileManager.render(document.querySelector('#two'),'two','Documents/two',callbacks);}catch(error){__errors.push(error.stack||String(error));}
</script>`)
	})
	mux.HandleFunc("/fm.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte(bundle.String()))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(30 * time.Second)
	defer page.Close()
	page.MustNavigate(server.URL).MustWaitLoad()
	page.MustEval(`async()=>{const end=Date.now()+5000;while(Date.now()<end&&(!fmTest.pending['Documents/one']?.length||!fmTest.pending['Documents/two']?.length))await new Promise(r=>setTimeout(r,10));if(!fmTest.pending['Documents/one']?.length||!fmTest.pending['Documents/two']?.length)throw Error('initial listings did not start: '+JSON.stringify({calls:fmTest.calls,errors:__errors,manager:typeof FileManager}));
fmTest.resolve('Documents/two',[{path:'Documents/two/two.txt',name:'two.txt',type:'file',size:1},{path:'Documents/two/drag-zone',name:'drag-zone',type:'directory',size:0}]);
fmTest.resolve('Documents/one',[{path:'Documents/one/one.txt',name:'one.txt',type:'file',size:1}]);
await new Promise(r=>setTimeout(r,50));
FileManager.navigateTo('one','Documents/one-old');FileManager.navigateTo('one','Documents/one-new');
while(Date.now()<end&&(!fmTest.pending['Documents/one-old']?.length||!fmTest.pending['Documents/one-new']?.length))await new Promise(r=>setTimeout(r,10));
fmTest.resolve('Documents/one-new',[{path:'Documents/one-new/current.txt',name:'current.txt',type:'file',size:1},{path:'Documents/one-new/dest',name:'dest',type:'directory',size:0}]);await new Promise(r=>setTimeout(r,30));
fmTest.resolve('Documents/one-old',[{path:'Documents/one-old/stale.txt',name:'stale.txt',type:'file',size:1}]);await new Promise(r=>setTimeout(r,50));
let row=document.querySelector('#one [data-path="Documents/one-new/current.txt"]');if(!row||document.querySelector('#one [data-path="Documents/one-old/stale.txt"]'))throw Error('stale listing replaced the newer window listing');
if(!document.querySelector('#two [data-path="Documents/two/two.txt"]')||document.querySelector('#two [data-path*="Documents/one/"]'))throw Error('sibling window listing was contaminated');
const bDragZone=document.querySelector('#two [data-path="Documents/two/drag-zone"]');if(!bDragZone)throw Error('B drag target missing');
bDragZone.dispatchEvent(new DragEvent('dragstart',{bubbles:true,dataTransfer:new DataTransfer()}));
document.querySelector('#one .file-manager').dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
fmTest.holdMutations=true;const transfer=new DataTransfer();transfer.setData('application/x-aurago-desktop-files',JSON.stringify({source:'file-manager',paths:['Documents/one-new/current.txt']}));
document.querySelector('#one [data-path="Documents/one-new/dest"]').dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:transfer}));
while(Date.now()<end&&!fmTest.pendingMutations.length)await new Promise(r=>setTimeout(r,10));if(!fmTest.pendingMutations.length)throw Error('A drop mutation did not start');
document.querySelector('#two .file-manager').dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));fmTest.pendingMutations.shift()();fmTest.holdMutations=false;await new Promise(r=>setTimeout(r,50));
const over=new DragEvent('dragover',{bubbles:true,cancelable:true,dataTransfer:new DataTransfer()});bDragZone.dispatchEvent(over);if(over.dataTransfer.dropEffect!=='none')throw Error('A delayed drop cleared B drag state');
fmTest.resolveAll('Documents/one-new',[{path:'Documents/one-new/current.txt',name:'current.txt',type:'file',size:1},{path:'Documents/one-new/dest',name:'dest',type:'directory',size:0}]);await new Promise(r=>setTimeout(r,30));row=document.querySelector('#one [data-path="Documents/one-new/current.txt"]');
row.focus();row.dispatchEvent(new KeyboardEvent('keydown',{key:'F2',bubbles:true}));
let rename=document.querySelector('#one [data-rename-input]');if(!rename)throw Error('F2 did not start inline rename');rename.value='';rename.dispatchEvent(new Event('input',{bubbles:true}));
rename.dispatchEvent(new KeyboardEvent('keydown',{key:'h',ctrlKey:true,bubbles:true}));rename=document.querySelector('#one [data-rename-input]');if(!rename||rename.value!=='')throw Error('empty rename draft was lost during rerender');
rename.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));if(document.querySelector('#one [data-rename-input]'))throw Error('Escape did not cancel inline rename');
row=document.querySelector('#one [data-path="Documents/one-new/current.txt"]');row.focus();row.dispatchEvent(new KeyboardEvent('keydown',{key:'F2',bubbles:true}));
const renameA=document.querySelector('#one [data-rename-input]');if(!renameA)throw Error('A rename did not start');renameA.value='renamed-a.txt';renameA.dispatchEvent(new Event('input',{bubbles:true}));
document.querySelector('#two .file-manager').dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));renameA.blur();
while(Date.now()<end&&!fmTest.mutations.some(item=>item.new_path==='Documents/one-new/renamed-a.txt'))await new Promise(r=>setTimeout(r,10));
if(!fmTest.mutations.some(item=>item.old_path==='Documents/one-new/current.txt'&&item.new_path==='Documents/one-new/renamed-a.txt'))throw Error('A blur while B was active did not finish A rename against A state: '+JSON.stringify(fmTest.mutations));
if(document.querySelector('#two [data-rename-input]')||!document.querySelector('#two [data-path="Documents/two/two.txt"]'))throw Error('A rename blur changed B rename or draft state');
}`)
}

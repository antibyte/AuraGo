package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fileManagerAsyncBrowserSource(t *testing.T, includeFileManager bool) []byte {
	t.Helper()
	parts := []string{"js/desktop/core/desktop-file-drops.js"}
	if includeFileManager {
		parts = append(parts,
			"js/desktop/file-manager/core-render.js",
			"js/desktop/file-manager/core-render-components.js",
			"js/desktop/file-manager/actions-input.js",
			"js/desktop/file-manager/actions-operations.js",
			"js/desktop/file-manager/tabs.js",
			"js/desktop/file-manager/preview-panel.js",
			"js/desktop/file-manager/advanced-actions.js",
			"js/desktop/file-manager/lifecycle-export.js",
		)
	}
	var source strings.Builder
	for _, path := range parts {
		content, err := Content.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source.Write(content)
		source.WriteByte('\n')
	}
	return []byte(source.String())
}

func TestFileManagerAsyncActionsStayWithOriginBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	source := fileManagerAsyncBrowserSource(t, true)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><div id="one"></div><div id="two"></div>
<script>
window.__errors=[];window.addEventListener('error',event=>__errors.push(event.error?.stack||event.message));
window.fmTest={activeId:'A',files:{},listCalls:[],writes:[],holdWrites:false,prompts:[],menus:{},contexts:{},desktopRefreshes:[],fmRefreshes:[],bootstrapLoads:0,
filesFor(path){return this.files[path]||[];},
finish(index){const item=this.writes[index];if(!item||item.done)return;item.done=true;item.resolve();},
finishNext(){const item=this.writes.find(entry=>!entry.done);if(item)this.finish(this.writes.indexOf(item));},
wait:async function(predicate,label){const end=Date.now()+5000;while(Date.now()<end){if(predicate())return;await new Promise(resolve=>setTimeout(resolve,5));}throw Error('timed out: '+label+'; writes='+JSON.stringify(this.writes.map(item=>({url:item.url,body:item.body,done:item.done})))+'; lists='+JSON.stringify(this.listCalls)+'; errors='+JSON.stringify(__errors));},
activate(id){this.activeId=id;const root=document.querySelector('#'+(id==='A'?'one':'two')+' .file-manager');if(root)root.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));},
row(id,path){return document.querySelector('#'+(id==='A'?'one':'two')+' [data-path="'+path+'"]');},
click(id,path,ctrl){this.activate(id);const item=this.row(id,path);if(!item)throw Error('missing row '+path);item.dispatchEvent(new MouseEvent('click',{bubbles:true,ctrlKey:!!ctrl}));},
menu(id,groupId,itemId){const group=(this.menus[id]||[]).find(item=>item.id===groupId);const item=group&&group.items.find(entry=>entry.id===itemId);if(!item||typeof item.action!=='function')throw Error('missing menu '+id+'/'+groupId+'/'+itemId);return item.action;},
contextAction(id,path,label){this.activate(id);const item=this.row(id,path);if(!item)throw Error('missing context row '+path);item.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));const entry=(this.contexts[id]||[]).find(value=>value.label===label);if(!entry||typeof entry.action!=='function')throw Error('missing context action '+label);return entry.action;},
prompt(index){const prompt=this.prompts[index];if(!prompt)throw Error('missing prompt '+index);return prompt;}
};
function normalizeDesktopPath(path){return String(path==null?'':path).replaceAll('\\','/').replace(/\/+/g,'/').replace(/^\/+|\/+$/g,'');}
function file(path,name,type){return {path:path,name:name,type:type||'file',size:1};}
window.state={bootstrap:{},filesPath:'Desktop',activeWindowId:'desktop-files',windows:new Map([['desktop-files',{id:'desktop-files',appId:'files'}]])};
window.$=id=>document.getElementById(id);window.loadBootstrap=async()=>{fmTest.bootstrapLoads++;return {};};
window.renderFiles=(id,path)=>fmTest.desktopRefreshes.push({id,path});window.clampDesktopIconPosition=(x,y)=>({x,y});
window.desktopIconGridEnabled=()=>false;window.desktopIconGridUsedCells=()=>new Set();window.desktopSound=()=>{};window.saveIconPosition=()=>{};
window.t=(key)=>key;window.esc=(value)=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
window.iconMarkup=()=>'';window.showNotification=()=>{};window.showDesktopNotification=()=>{};window.isReadonly=()=>false;window.maxFileSize=()=>0;
fmTest.files['Documents/A']=[file('Documents/A/a1.txt','a1.txt'),file('Documents/A/a2.txt','a2.txt'),file('Documents/A/archive.zip','archive.zip'),file('Documents/A/rename1.txt','rename1.txt'),file('Documents/A/rename2.txt','rename2.txt'),file('Documents/A/dest','dest','directory'),file('Documents/A/other','other','directory')];
fmTest.files['Documents/A/dest']=[file('Documents/A/dest/a1.txt','a1.txt')];fmTest.files['Documents/A/other']=[file('Documents/A/other/other.txt','other.txt')];
fmTest.files['Documents/B']=[file('Documents/B/b-copy.txt','b-copy.txt'),file('Documents/B/b.txt','b.txt'),file('Documents/B/target','target','directory')];fmTest.files['Documents/B/target']=[];fmTest.files['']=[file('root.txt','root.txt')];
window.api=(url,options={})=>{const u=new URL(url,location.origin);if(u.pathname==='/api/desktop/files'){const path=u.searchParams.get('path')||'';fmTest.listCalls.push({path:path,active:fmTest.activeId});return Promise.resolve({files:fmTest.filesFor(path)});}const method=options.method||'GET';const body=options.body?JSON.parse(options.body):{};return new Promise(resolve=>{const entry={url:u.pathname,method:method,body:body,done:false,resolve:()=>resolve(method==='PATCH'?{path:body.new_path}:{} )};fmTest.writes.push(entry);if(!fmTest.holdWrites)entry.resolve();});};
function callbacks(id){return {directories:[],api:window.api,t:window.t,esc:window.esc,readonly:false,promptDialog:(title,value)=>new Promise(resolve=>fmTest.prompts.push({windowId:id,title:title,value:value,resolve:resolve})),showContextMenu:(x,y,items)=>{fmTest.contexts[id]=items;},setWindowMenus:(windowId,menus)=>{fmTest.menus[windowId]=menus;},clearWindowMenus(){},closeContextMenu(){},refreshDesktop:()=>fmTest.fmRefreshes.push(id)};}
</script><script src="/fm.js"></script><script>
try{FileManager.render(document.querySelector('#one'),'A','Documents/A',callbacks('A'));FileManager.render(document.querySelector('#two'),'B','Documents/B',callbacks('B'));}catch(error){__errors.push(error.stack||String(error));}
</script>`)
	})
	mux.HandleFunc("/fm.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write(source)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(45 * time.Second)
	defer page.Close()
	page.MustNavigate(server.URL).MustWaitLoad()
	page.MustEval(`async()=>{
const wait=(predicate,label)=>fmTest.wait(predicate,label);
await wait(()=>document.querySelector('#one [data-path="Documents/A/a1.txt"]')&&document.querySelector('#two [data-path="Documents/B/b-copy.txt"]'),'initial listings');
const a1='Documents/A/a1.txt',a2='Documents/A/a2.txt';
fmTest.click('A',a1,false);fmTest.click('A',a2,true);fmTest.menu('A','edit','cut')();
FileManager.navigateTo('A','Documents/A/dest');fmTest.holdWrites=true;const firstPasteWrite=fmTest.writes.length;
fmTest.menu('A','edit','paste')();await wait(()=>fmTest.writes.length>firstPasteWrite,'first shared paste mutation');
if(fmTest.writes[firstPasteWrite].body.new_path!=='Documents/A/dest/a1 (2).txt')throw Error('paste did not use the destination collision snapshot: '+JSON.stringify(fmTest.writes[firstPasteWrite]));
FileManager.navigateTo('A','Documents/A/other');await wait(()=>fmTest.listCalls.some(item=>item.path==='Documents/A/other'),'same-window navigation');
fmTest.click('B','Documents/B/b-copy.txt',false);fmTest.menu('B','edit','copy')();const newerClipboard=window.AuraDesktopFileClipboard;
fmTest.finish(firstPasteWrite);await wait(()=>fmTest.writes.length===firstPasteWrite+2,'second captured paste mutation');
if(fmTest.writes[firstPasteWrite+1].body.old_path!==a2||fmTest.writes[firstPasteWrite+1].body.new_path!=='Documents/A/dest/a2.txt')throw Error('paste changed sources or destination after navigation: '+JSON.stringify(fmTest.writes[firstPasteWrite+1]));
fmTest.finish(firstPasteWrite+1);await wait(()=>fmTest.fmRefreshes.includes('A'),'origin file-manager refresh');
if(window.AuraDesktopFileClipboard!==newerClipboard||newerClipboard.mode!=='copy'||newerClipboard.paths[0]!=='Documents/B/b-copy.txt')throw Error('paste cleared a newer clipboard from the second window');
if(fmTest.desktopRefreshes.length!==0)throw Error('shared paste refreshed the globally active file-manager: '+JSON.stringify(fmTest.desktopRefreshes));
if(fmTest.bootstrapLoads!==1)throw Error('shared paste skipped desktop bootstrap refresh');

FileManager.navigateTo('B','Documents/B/target');fmTest.activate('B');const bPaste=fmTest.writes.length;fmTest.menu('B','edit','paste')();
await wait(()=>fmTest.writes.length>bPaste,'second-window clipboard paste');
if(fmTest.writes[bPaste].url!=='/api/desktop/copy'||fmTest.writes[bPaste].body.source_path!=='Documents/B/b-copy.txt'||fmTest.writes[bPaste].body.dest_path!=='Documents/B/target/b-copy.txt')throw Error('newer clipboard was not usable in B: '+JSON.stringify(fmTest.writes[bPaste]));
fmTest.finish(bPaste);await wait(()=>fmTest.fmRefreshes.filter(id=>id==='B').length>0,'B paste refresh');

window.AuraDesktopFileOps.setClipboard('copy',['Documents/A/a1.txt']);FileManager.navigateTo('A','');fmTest.activate('A');const rootPaste=fmTest.writes.length;fmTest.menu('A','edit','paste')();
await wait(()=>fmTest.writes.length>rootPaste,'root-path paste');
if(fmTest.listCalls[fmTest.listCalls.length-1].path!==''||fmTest.writes[rootPaste].body.dest_path!=='a1.txt')throw Error('empty root destination was not preserved: '+JSON.stringify({lists:fmTest.listCalls.slice(-2),write:fmTest.writes[rootPaste]}));
fmTest.finish(rootPaste);await wait(()=>fmTest.fmRefreshes.filter(id=>id==='A').length>1,'root paste refresh');

// Without shared FileOps, paste must commit undo to A and leave B's local clipboard alone.
const desktopOps=window.AuraDesktopFileOps;window.AuraDesktopFileOps=null;FileManager.navigateTo('A','Documents/A');await wait(()=>fmTest.row('A',a2),'fallback source listing');
fmTest.click('A',a2,false);fmTest.menu('A','edit','cut')();FileManager.navigateTo('A','Documents/A/dest');fmTest.holdWrites=true;
const fallbackStart=fmTest.writes.length;const fallbackPaste=fmTest.menu('A','edit','paste')();await wait(()=>fmTest.writes.length>fallbackStart,'fallback paste mutation');
if(fmTest.writes[fallbackStart].body.old_path!==a2||fmTest.writes[fallbackStart].body.new_path!=='Documents/A/dest/a2.txt')throw Error('fallback paste changed captured move paths: '+JSON.stringify(fmTest.writes[fallbackStart]));
FileManager.navigateTo('B','Documents/B');await wait(()=>fmTest.row('B','Documents/B/b-copy.txt'),'B fallback clipboard source listing');fmTest.click('B','Documents/B/b-copy.txt',false);fmTest.menu('B','edit','copy')();fmTest.files['Documents/A/dest'].push(file('Documents/A/dest/a2.txt','a2.txt'));fmTest.finish(fallbackStart);await fallbackPaste;
const writesBeforeEmptyUndo=fmTest.writes.length;FileManager.navigateTo('B','Documents/B');await wait(()=>fmTest.row('B','Documents/B/b.txt'),'B undo source listing');fmTest.click('B','Documents/B/b.txt',false);document.dispatchEvent(new KeyboardEvent('keydown',{key:'z',ctrlKey:true,bubbles:true}));
await new Promise(resolve=>setTimeout(resolve,30));if(fmTest.writes.length!==writesBeforeEmptyUndo)throw Error('B undo consumed A paste history');
fmTest.click('A','Documents/A/dest/a2.txt',false);document.dispatchEvent(new KeyboardEvent('keydown',{key:'z',ctrlKey:true,bubbles:true}));
await wait(()=>fmTest.writes.length>writesBeforeEmptyUndo,'origin A undo');const undoWrite=fmTest.writes[writesBeforeEmptyUndo];
if(undoWrite.method!=='PATCH'||undoWrite.body.old_path!=='Documents/A/dest/a2.txt'||undoWrite.body.new_path!==a2)throw Error('A undo did not reverse its own paste: '+JSON.stringify(undoWrite));
const aRefreshBeforeUndo=fmTest.fmRefreshes.filter(id=>id==='A').length;fmTest.finish(writesBeforeEmptyUndo);await wait(()=>fmTest.fmRefreshes.filter(id=>id==='A').length>aRefreshBeforeUndo,'fallback undo refresh');
fmTest.holdWrites=false;FileManager.navigateTo('B','Documents/B/target');fmTest.activate('B');const bLocalPaste=fmTest.writes.length;await fmTest.menu('B','edit','paste')();
if(fmTest.writes.length!==bLocalPaste+1||fmTest.writes[bLocalPaste].url!=='/api/desktop/copy'||fmTest.writes[bLocalPaste].body.source_path!=='Documents/B/b-copy.txt')throw Error('A paste changed B local clipboard: '+JSON.stringify(fmTest.writes.slice(bLocalPaste)));
window.AuraDesktopFileOps=desktopOps;fmTest.holdWrites=false;

FileManager.navigateTo('A','Documents/A');await wait(()=>fmTest.row('A',a1),'archive source listing');fmTest.click('A',a1,false);fmTest.activate('A');const aRefreshBeforeArchive=fmTest.fmRefreshes.filter(id=>id==='A').length;const compress=fmTest.contextAction('A',a1,'desktop.fm.compress_zip');compress();
await wait(()=>fmTest.prompts.length===1,'archive name prompt');FileManager.navigateTo('A','Documents/A/other');fmTest.activate('B');fmTest.prompt(0).resolve('packed.zip');
const archiveWrite=fmTest.writes.length;await wait(()=>fmTest.writes.length>archiveWrite,'archive request');
const archive=fmTest.writes[archiveWrite];if(archive.url!=='/api/desktop/archive'||archive.body.dest!=='Documents/A/packed.zip'||archive.body.paths.length!==1||archive.body.paths[0]!==a1)throw Error('archive used mutable selection/path: '+JSON.stringify(archive));
fmTest.finish(archiveWrite);await wait(()=>fmTest.fmRefreshes.filter(id=>id==='A').length>aRefreshBeforeArchive,'archive origin refresh');

FileManager.navigateTo('A','Documents/A');const zipPath='Documents/A/archive.zip';await wait(()=>fmTest.row('A',zipPath),'extract source listing');fmTest.click('A',zipPath,false);const aRefreshBeforeExtract=fmTest.fmRefreshes.filter(id=>id==='A').length;const extract=fmTest.contextAction('A',zipPath,'desktop.fm.extract_zip_to');extract();
await wait(()=>fmTest.prompts.length===2,'extract destination prompt');if(fmTest.prompt(1).value!=='Documents/A')throw Error('extract prompt default was not captured from A');
FileManager.navigateTo('A','Documents/A/other');fmTest.activate('B');fmTest.prompt(1).resolve('Documents/A/extracted');
const extractWrite=fmTest.writes.length;await wait(()=>fmTest.writes.length>extractWrite,'extract request');
const extraction=fmTest.writes[extractWrite];if(extraction.url!=='/api/desktop/extract'||extraction.body.path!==zipPath||extraction.body.dest!=='Documents/A/extracted')throw Error('extract used mutable window state: '+JSON.stringify(extraction));
fmTest.finish(extractWrite);await wait(()=>fmTest.fmRefreshes.filter(id=>id==='A').length>aRefreshBeforeExtract,'extract origin refresh');

FileManager.navigateTo('A','Documents/A');await wait(()=>fmTest.row('A','Documents/A/rename1.txt'),'batch source listing');fmTest.click('A','Documents/A/rename1.txt',false);fmTest.click('A','Documents/A/rename2.txt',true);const aRefreshBeforeBatch=fmTest.fmRefreshes.filter(id=>id==='A').length;
const batch=fmTest.contextAction('A','Documents/A/rename2.txt','desktop.fm.batch_rename');batch();
await wait(()=>document.querySelector('.fm-modal-overlay [data-rename]'),'batch rename dialog');
const modal=document.querySelector('.fm-modal-overlay');const prefix=modal.querySelector('[name="prefix"]');prefix.value='Batch-';prefix.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(resolve=>setTimeout(resolve,150));FileManager.navigateTo('A','Documents/A/other');FileManager.navigateTo('B','Documents/B');await wait(()=>fmTest.row('B','Documents/B/b.txt'),'B batch window listing');fmTest.click('B','Documents/B/b.txt',false);
modal.querySelector('[data-rename]').click();const batchWrite=fmTest.writes.length;await wait(()=>fmTest.writes.length>batchWrite,'batch rename request');
const rename=fmTest.writes[batchWrite];const operations=rename.body.operations||[];
if(rename.url!=='/api/desktop/batch-rename'||operations.length!==2||operations[0].old_path!=='Documents/A/rename1.txt'||operations[0].new_name!=='Batch-rename1.txt'||operations[1].old_path!=='Documents/A/rename2.txt'||operations[1].new_name!=='Batch-rename2.txt')throw Error('batch rename did not retain A selection: '+JSON.stringify(rename));
fmTest.finish(batchWrite);await wait(()=>fmTest.fmRefreshes.filter(id=>id==='A').length>aRefreshBeforeBatch,'batch rename origin refresh');

// Closing A during a shared multi-file paste must stop later writes.
FileManager.navigateTo('A','Documents/A/dest');window.AuraDesktopFileOps.setClipboard('cut',[a1,a2]);fmTest.activate('A');fmTest.holdWrites=true;
const closePasteStart=fmTest.writes.length;fmTest.menu('A','edit','paste')();await wait(()=>fmTest.writes.length>closePasteStart,'close paste first write');
FileManager.dispose('A');fmTest.finish(closePasteStart);await wait(()=>fmTest.writes[closePasteStart].done,'closed paste settlement');
await new Promise(resolve=>setTimeout(resolve,40));if(fmTest.writes.length!==closePasteStart+1)throw Error('closed originating window dispatched more paste writes');

// A ZIP prompt may finish after its window closes, but must not dispatch.
FileManager.render(document.querySelector('#one'),'A','Documents/A',callbacks('A'));await wait(()=>document.querySelector('#one [data-path="Documents/A/a1.txt"]'),'reopened A');
fmTest.click('A',a1,false);const closedZip=fmTest.contextAction('A',a1,'desktop.fm.compress_zip');closedZip();
await wait(()=>fmTest.prompts.length===3,'second archive prompt');const beforeClosedZip=fmTest.writes.length;FileManager.dispose('A');fmTest.prompt(2).resolve('must-not-create.zip');
await new Promise(resolve=>setTimeout(resolve,40));if(fmTest.writes.length!==beforeClosedZip)throw Error('closed originating window dispatched an archive request');

// Batch rename confirmation after disposal must not send the captured payload.
FileManager.render(document.querySelector('#one'),'A','Documents/A',callbacks('A'));await wait(()=>document.querySelector('#one [data-path="Documents/A/rename1.txt"]'),'reopened A for batch close');
fmTest.click('A','Documents/A/rename1.txt',false);fmTest.click('A','Documents/A/rename2.txt',true);
fmTest.contextAction('A','Documents/A/rename2.txt','desktop.fm.batch_rename')();await wait(()=>document.querySelector('.fm-modal-overlay [data-rename]'),'closed batch dialog');
const closedModal=document.querySelector('.fm-modal-overlay');const closedPrefix=closedModal.querySelector('[name="prefix"]');closedPrefix.value='Closed-';closedPrefix.dispatchEvent(new Event('input',{bubbles:true}));
await new Promise(resolve=>setTimeout(resolve,150));FileManager.dispose('A');const beforeClosedBatch=fmTest.writes.length;closedModal.querySelector('[data-rename]').click();
await new Promise(resolve=>setTimeout(resolve,40));if(fmTest.writes.length!==beforeClosedBatch)throw Error('closed originating window dispatched a batch rename request');
if(__errors.length)throw Error('browser errors: '+JSON.stringify(__errors));
}`)
}

func TestDesktopFileClipboardPastePreservesNewSameContentBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	source := fileManagerAsyncBrowserSource(t, false)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><div id="workspace"></div>
<script>
window.__errors=[];window.addEventListener('error',event=>__errors.push(event.error?.stack||event.message));
window.fmTest={writes:[],renders:[],loads:0,wait:async function(predicate,label){const end=Date.now()+5000;while(Date.now()<end){if(predicate())return;await new Promise(resolve=>setTimeout(resolve,5));}throw Error('timed out: '+label+'; writes='+JSON.stringify(this.writes)+'; errors='+JSON.stringify(__errors));}};
window.normalizeDesktopPath=path=>String(path==null?'':path).replaceAll('\\','/').replace(/\/+/g,'/').replace(/^\/+|\/+$/g,'');
window.state={bootstrap:{},filesPath:'Desktop',activeWindowId:'desktop-files',windows:new Map([['desktop-files',{id:'desktop-files',appId:'files'}]])};
window.$=id=>document.getElementById(id);window.loadBootstrap=async()=>{fmTest.loads++;return {};};window.renderFiles=(id,path)=>fmTest.renders.push({id,path});
window.clampDesktopIconPosition=(x,y)=>({x,y});window.desktopIconGridEnabled=()=>false;window.desktopIconGridUsedCells=()=>new Set();window.desktopSound=()=>{};window.saveIconPosition=()=>{};
window.t=key=>key;window.showDesktopNotification=()=>{};
window.api=(url,options={})=>{const u=new URL(url,location.origin);if(u.pathname==='/api/desktop/files')return Promise.resolve({files:[]});const body=options.body?JSON.parse(options.body):{};return new Promise(resolve=>fmTest.writes.push({url:u.pathname,method:options.method,body:body,resolve:()=>resolve(options.method==='PATCH'?{path:body.new_path}:{})}));};
</script><script src="/fm.js"></script><script>
try{window.originalClipboard={mode:'cut',paths:['Desktop/source.txt']};window.AuraDesktopFileClipboard=window.originalClipboard;
window.pastePromise=window.AuraDesktopFileOps.paste('Desktop/target');}catch(error){__errors.push(error.stack||String(error));}
</script>`)
	})
	mux.HandleFunc("/fm.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write(source)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(30 * time.Second)
	defer page.Close()
	page.MustNavigate(server.URL).MustWaitLoad()
	page.MustEval(`async()=>{
await fmTest.wait(()=>fmTest.writes.length===1,'shared clipboard write');
const replacement={mode:'cut',paths:['Desktop/source.txt']};window.AuraDesktopFileClipboard=replacement;
fmTest.writes[0].resolve();await window.pastePromise;
if(window.AuraDesktopFileClipboard!==replacement)throw Error('paste cleared a newer same-content clipboard object');
if(fmTest.loads!==1||fmTest.renders.length!==1||fmTest.renders[0].id!=='desktop-files')throw Error('desktop-context paste did not preserve its default refresh behavior: '+JSON.stringify({loads:fmTest.loads,renders:fmTest.renders}));
if(__errors.length)throw Error('browser errors: '+JSON.stringify(__errors));
}`)
}

package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestDesktopFileDropsStayOnDesktopBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/file-drops.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprint(w, rawDesktopAssetText(t, "js/desktop/core/desktop-file-drops.js"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8">
<section id="vd-workspace">
  <div id="vd-icons"><button class="vd-icon" id="desktop-icon">File</button></div>
  <div id="vd-window-layer"><div class="vd-window"><img id="app-image" alt="App image"><div id="app-content">App content</div></div></div>
  <div class="vd-widget"><img id="widget-image" alt="Widget image"></div>
  <div class="vd-modal-backdrop"><button id="dialog-control">Dialog</button></div>
</section>
<script>
window.fixtureErrors=[];window.addEventListener('error',e=>fixtureErrors.push(e.message));
window.addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
window.state={bootstrap:{},windows:new Map()};window.fixtureMutations=[];
window.$=id=>document.getElementById(id);window.normalizeDesktopPath=path=>String(path||'');
window.clampDesktopIconPosition=(x,y)=>({x,y});window.desktopIconGridEnabled=()=>false;
window.saveIconPosition=()=>{};window.desktopSound=()=>{};window.loadBootstrap=async()=>{};
window.t=key=>key;window.showDesktopNotification=notice=>fixtureErrors.push(notice.message);
window.api=async(url,options={})=>{
  if(options.method){
    const path=options.body instanceof FormData?options.body.get('path'):JSON.parse(options.body).new_path;
    fixtureMutations.push({url,method:options.method,path});
    return {path:options.body instanceof FormData?'Desktop/'+options.body.get('file').name:path};
  }
  return {files:[]};
};
window.openApp=()=>{};
window.FileManager={storageReady:()=>true,transferStorage:async(id,entries,destination,mode)=>fixtureMutations.push({url:'cloud-transfer',path:destination.path,mode})};
</script><script src="/file-drops.js"></script><script>
wireDesktopFileDrops();
window.fixtureDrop=async(selector,kind,consumed=false)=>{
  const target=document.querySelector(selector),dataTransfer=new DataTransfer();
  if(kind==='host')dataTransfer.items.add(new File(['image'],'tool_clamp.png',{type:'image/png'}));
  else if(kind==='local')dataTransfer.setData('application/x-aurago-desktop-files',JSON.stringify({paths:['Pictures/tool_clamp.png']}));
  else dataTransfer.setData('application/x-aurago-storage-files',JSON.stringify({entries:[{provider:'s3',path:'tool_clamp.png'}]}));
  if(consumed)for(const type of ['dragover','drop'])target.addEventListener(type,e=>e.preventDefault(),{once:true});
  target.dispatchEvent(new DragEvent('dragover',{bubbles:true,cancelable:true,dataTransfer}));
  const highlighted=$('vd-workspace').classList.contains('vd-desktop-file-drop-target');
  target.dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer}));
  await new Promise(resolve=>setTimeout(resolve,0));
  return highlighted;
};
</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	profile := t.TempDir()
	page := newSmokeBrowserWith(t, func(l *launcher.Launcher) { l.UserDataDir(profile) }).MustPage(server.URL).Timeout(30 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>{
  const hover=new DataTransfer();hover.items.add(new File(['image'],'tool_clamp.png',{type:'image/png'}));
  const dragover=()=>new DragEvent('dragover',{bubbles:true,cancelable:true,dataTransfer:hover});
  $('vd-workspace').dispatchEvent(dragover());
  if(!$('vd-workspace').classList.contains('vd-desktop-file-drop-target'))throw Error('Desktop hover was not highlighted');
  $('app-image').dispatchEvent(dragover());
  if($('vd-workspace').classList.contains('vd-desktop-file-drop-target'))throw Error('Desktop hover remained highlighted over an app');
  for(const selector of ['#app-image','#app-content','#widget-image','#dialog-control']){
    for(const kind of ['host','local','cloud']){
      fixtureMutations.length=0;
      const highlighted=await fixtureDrop(selector,kind);
      if(highlighted||fixtureMutations.length)throw Error(selector+' '+kind+' reached the desktop: '+JSON.stringify(fixtureMutations));
    }
  }
  if(await fixtureDrop('#desktop-icon','host',true)||fixtureMutations.length)throw Error('A consumed drop reached the desktop');
  for(const selector of ['#vd-workspace','#vd-icons','#desktop-icon']){
    for(const kind of ['host','local','cloud']){
      fixtureMutations.length=0;
      if(!await fixtureDrop(selector,kind))throw Error('Desktop drop target was not highlighted');
      if(fixtureMutations.length!==1||!fixtureMutations[0].path.startsWith('Desktop'))throw Error('Desktop '+kind+' drop failed: '+JSON.stringify(fixtureMutations));
    }
  }
  if(fixtureErrors.length)throw Error(JSON.stringify(fixtureErrors));
}`)
}

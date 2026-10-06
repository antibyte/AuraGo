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

func TestDesktopStoreTerminalDrawerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8">
<link rel="stylesheet" href="/css/desktop-windows.css">
<link rel="stylesheet" href="/css/desktop-app-software-store.css">
<style>body{margin:0;font:13px sans-serif;background:#05070a}#app{width:100vw;height:100vh}
body{--vd-theme-panel-bg:#17202c;--vd-theme-border:#334155}
body[data-theme=fruity]{--vd-theme-panel-bg:#e9edf4;--vd-theme-border:#bac1ce;--vd-theme-muted:#475569}</style>
</head><body><div id="app"></div>
<script src="/js/shared/lazy-assets.js"></script>
<script src="/js/desktop/apps/store-terminal-preview.js"></script>
<script>
window.sockets=[]; window.errors=[];
addEventListener('error',e=>errors.push(e.message));
addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));
window.WebSocket=class {
 static OPEN=1; static CONNECTING=0;
 constructor(url){this.url=url;this.readyState=0;this.sent=[];sockets.push(this);
  setTimeout(()=>{if(this.readyState!==0)return;this.readyState=1;this.onopen?.();
   this.onmessage?.({data:url.includes('bootstrap=1')?'CommandCode ready\r\n> ':'/workspace $ '});},0);}
 send(data){this.sent.push(typeof data==='string'?data:new TextDecoder().decode(data));}
 close(){this.readyState=3;this.onclose?.();}
};
Object.defineProperty(navigator,'clipboard',{value:{readText:async()=> 'pwd\r',writeText:async()=>{}}});
window.fixtureReady=(async()=>{
 const words=await(await fetch('/lang/desktop/de.json')).json();
 window.previewReady=false;
 const realFetch=window.fetch;
 window.fetch=(url,options)=>String(url).includes('/preview-status')
  ?Promise.resolve({ok:true,json:async()=>({ready:previewReady,target:'http://127.0.0.1:5173'})}):realFetch(url,options);
 await StoreTerminalPreviewApp.render('commandcode',{id:'commandcode',name:'CommandCode'},'commandcode',{
  contentEl:()=>document.getElementById('app'),t:key=>words[key]||key,
  esc:value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),
  iconMarkup:()=>'<span aria-hidden="true">+</span>',appName:()=> 'CommandCode',
  registerWindowCleanup:(_id,fn)=>{window.cleanup=fn;},showDesktopNotification:()=>{},
  api:async()=>({url:'/preview'}),storeFrameURL:url=>url,
  makeSandboxedFrame:(_url,_app,_unused,_win,classes)=>{const f=document.createElement('iframe');f.className=classes;f.srcdoc='<h1>Preview ready</h1>';return f;}
 });
})();
</script></body></html>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(45 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>await fixtureReady`)
	page.MustWait(`()=>sockets.length===1 && sockets[0].readyState===1`)
	check := func(js string, message string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatal(message)
		}
	}
	check(`()=>document.querySelector('[data-store-shell-pane]').hidden`, "shell must start collapsed")
	page.MustElement("[data-store-shell-toggle]").MustClick()
	page.MustWait(`()=>sockets.length===2 && sockets[1].readyState===1`)
	check(`()=>!sockets[1].url.includes('bootstrap=') && !document.querySelector('[data-store-terminal-session]').hidden`, "shell must use the normal endpoint and keep CommandCode visible")
	page.MustElement("[data-store-shell-hide]").MustClick()
	check(`()=>sockets.every(s=>s.readyState===1) && document.querySelector('[data-store-shell-pane]').hidden`, "hiding must preserve both sessions")
	page.MustElement("[data-store-shell-toggle]").MustClick()
	check(`()=>sockets.length===2`, "showing must reuse the shell")
	page.MustElement("[data-store-shell-pane] [data-store-terminal-paste]").MustClick()
	page.MustWait(`()=>sockets[1].sent.includes('pwd\r')`)
	check(`()=>!sockets[0].sent.includes('pwd\r')`, "shell paste leaked to CommandCode")
	page.MustElement(".vd-store-terminal-pane [data-store-terminal-paste]").MustClick()
	page.MustWait(`()=>sockets[0].sent.includes('pwd\r')`)
	page.MustElement("[data-store-terminal-new]").MustClick()
	page.MustWait(`()=>sockets.length===3 && sockets[2].readyState===1`)
	page.MustEval(`()=>sockets[1].close()`)
	check(`()=>document.querySelector('[data-store-shell-connection]').dataset.state==='connected' && document.querySelector('[data-store-terminal-connection]').dataset.state==='connected'`, "inactive shell closure changed another session's status")
	page.MustElement("[data-store-shell-tabs] [data-store-terminal-tab]:last-child").MustClick()
	page.MustElement("[data-store-shell-pane] [data-store-terminal-restart]").MustClick()
	page.MustWait(`()=>sockets.length===4 && sockets[3].readyState===1`)
	check(`()=>sockets[0].readyState===1 && sockets[2].readyState===3`, "restarting shell interrupted CommandCode or leaked the old socket")
	page.MustEval(`()=>{
 const input=document.querySelector('[data-store-shell-terminal] [data-store-terminal-session]:not([hidden]) textarea');
 input.focus();
 const event=new Event('paste',{bubbles:true,cancelable:true});
 event.clipboardData={getData:()=> 'echo shell-input\r'};input.dispatchEvent(event);
}`)
	check(`()=>sockets[3].sent.includes('echo shell-input\r') && !sockets[0].sent.includes('echo shell-input\r')`, "focused shell input reached CommandCode")
	page.MustElement("[data-store-shell-hide]").MustClick()
	check(`()=>document.querySelector('[data-store-terminal]').contains(document.activeElement)`, "hiding shell must restore CommandCode focus")
	page.MustElement("[data-store-shell-toggle]").MustClick()
	page.MustEval(`()=>{previewReady=true;}`)
	page.MustWait(`()=>!!document.querySelector('.vd-store-preview-pane iframe')`)
	for _, size := range [][2]int{{1366, 900}, {800, 700}, {390, 667}} {
		page.MustSetViewport(size[0], size[1], 1, false)
		for _, theme := range []string{"standard", "fruity"} {
			page.MustEval(`theme=>{document.body.dataset.theme=theme;}`, theme)
			page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
			check(`()=>{
 const app=document.getElementById('app').getBoundingClientRect();
 return ['[data-store-terminal]','[data-store-shell-terminal]','.vd-store-preview-pane'].every(s=>{
  const r=document.querySelector(s).getBoundingClientRect();return r.height>40 && r.width>100 && r.bottom<=app.bottom+1 && r.right<=app.right+1;
 }) && document.documentElement.scrollWidth<=innerWidth;
}`, "terminal or preview overflows or collapses at the tested size")
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("commandcode-%s-%d.png", theme, size[0])))
			}
		}
	}
	page.MustEval(`()=>[...document.querySelectorAll('[data-store-shell-tabs] [data-store-terminal-close]')].forEach(button=>button.click())`)
	check(`()=>document.querySelector('[data-store-shell-pane]').hidden && sockets[0].readyState===1 && sockets.slice(1).every(s=>s.readyState===3)`, "closing the last shell must collapse the drawer without stopping CommandCode")
	page.MustEval(`()=>cleanup()`)
	check(`()=>sockets.every(s=>s.readyState===3)`, "window cleanup leaked a session")
	if errors := page.MustEval(`()=>JSON.stringify(errors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

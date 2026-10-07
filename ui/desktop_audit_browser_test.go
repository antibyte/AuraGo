package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
)

// Keep the real desktop runtime and DOM, exposing only test entry points.
func desktopAuditBrowser(t *testing.T) *rod.Page {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	words := readDesktopAssetText(t, "lang/desktop/de.json")
	html = strings.Replace(html, "</body>", `<script>window.fixtureWords=`+words+`;window.t=(k,v)=>{let s=fixtureWords[k]||k;for(const [key,value] of Object.entries(v||{}))s=s.replaceAll('{{'+key+'}}',value).replaceAll('{'+key+'}',value);return s;};window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));window.WebSocket=class extends EventTarget{close(){}};</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/audit-shell.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("startup seam missing")
	}
	shell = shell[:cut] + `window.auditTest={state,api,modalDialog,openApp,closeWindow,focusWindow,captureSessionSnapshot,restoreDesktopSession,switchSpace,refreshSpacesForViewport,applyResize,applyWindowSnap,workspaceBoundsForWindow,openSpotlight,closeSpotlight,renderQuickChatWidget,clearWidgetRuntime,openClockPopup,toggleNotificationCenter,applyDesktopSettings};` + shell[cut:]
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false"} {
		settings[key] = value
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/audit-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/desktop/bootstrap" {
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{}, "widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{}, "workspace": map[string]interface{}{"readonly": false}, "settings": settings})
		} else {
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	page := newSmokeBrowser(t).MustPage().Timeout(30 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>!!window.auditTest?.state.ws`)
	return page
}

func TestDesktopAuditAppearanceBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`()=>auditTest.openApp('calculator')`)
	dir := filepath.Join("..", "reports", "desktop-audit-browser")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity"} {
		for _, mode := range []string{"light", "dark"} {
			for _, width := range []int{1280, 420} {
				page.MustSetViewport(width, 900, 1, false)
				page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();a.refreshSpacesForViewport();const w=a.state.windows.get(a.state.activeWindowId).element;a.applyWindowSnap(w,'left-half');}`, theme, mode)
				if !page.MustEval(`()=>{const w=auditTest.state.windows.get(auditTest.state.activeWindowId).element,r=w.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth+1&&r.top>=0&&r.top<innerHeight-60;}`).Bool() {
					t.Fatal("unreachable window", theme, mode, width)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%s-%d.png", theme, mode, width)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestDesktopAuditShellBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`()=>{
   const a=auditTest;a.openApp('calculator');window.normalID=a.state.activeWindowId;
   a.openApp('editor');window.topID=a.state.activeWindowId;a.state.windows.get(topID).alwaysOnTop=true;a.focusWindow(topID);a.focusWindow(normalID);
   a.switchSpace('3');a.openApp('files');a.switchSpace('1');a.focusWindow(normalID);
   const snapshot=a.captureSessionSnapshot();window.savedSnapshot=snapshot;
   a.state.windows.forEach(w=>w.element.remove());a.state.windows.clear();a.state.activeWindowId='';
   a.state.bootstrap.settings['session.windows']=JSON.stringify(snapshot);a.state.bootstrap.settings['windows.restore_session']='true';a.state._sessionRestored=false;
   window.restored=false;a.restoreDesktopSession().then(()=>restored=true);
 }`)
	page.MustWait(`()=>window.restored`)
	if !page.MustEval(`()=>auditTest.state.windows.get(auditTest.state.activeWindowId).sessionKey===savedSnapshot.activeWindowKey && [...auditTest.state.windows.values()].some(w=>w.alwaysOnTop)`).Bool() {
		t.Fatal("focus was replaced by always-on-top stacking")
	}
	page.MustEval(`()=>auditTest.switchSpace('3')`)
	page.MustSetViewport(420, 740, 1, false)
	page.MustEval(`()=>auditTest.refreshSpacesForViewport()`)
	if got := page.MustEval(`()=>auditTest.captureSessionSnapshot().activeSpaceId`).Str(); got != "3" {
		t.Fatal("compact lost logical space: " + got)
	}
	page.MustSetViewport(1280, 900, 1, false)
	page.MustEval(`()=>auditTest.refreshSpacesForViewport()`)
	if got := page.MustEval(`()=>auditTest.state.activeSpaceId`).Str(); got != "3" {
		t.Fatal("wide lost logical space")
	}
	if !page.MustEval(`()=>{
   const a=auditTest,w=[...a.state.windows.values()].find(w=>w.appId==='files').element;
   for(const theme of ['standard','fruity']) {a.state.bootstrap.settings['appearance.theme']=theme;a.applyDesktopSettings();
     for(const edge of ['nw','ne','sw','se'])for(const delta of [-4000,4000]){
       a.applyResize(w,edge,{left:80,top:60,width:700,height:550},delta,delta);
       const b=a.workspaceBoundsForWindow(),x=parseFloat(w.style.left),y=parseFloat(w.style.top);
       if(x<0||y<0||x+w.offsetWidth>b.width+1||y+w.offsetHeight>b.height+1)return false;
     }
     for(const zone of ['left-half','right-half','top-left','top-right']){
       a.applyWindowSnap(w,zone);const b=a.workspaceBoundsForWindow();
       if(parseFloat(w.style.left)+parseFloat(w.style.width)>b.width+1||parseFloat(w.style.top)+parseFloat(w.style.height)>b.height+1)return false;
     }
   }return true;
 }`).Bool() {
		t.Fatal("resize or snap escaped the work area")
	}
	page.MustEval(`()=>{auditTest.openClockPopup(document.getElementById('vd-clock'));}`)
	page.MustWait(`()=>!document.getElementById('vd-clock-popup').hidden`)
	page.MustEval(`()=>document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))`)
	if !page.MustEval(`()=>document.getElementById('vd-clock-popup').hidden&&document.activeElement.id==='vd-clock'`).Bool() {
		t.Fatal("clock Escape did not return focus")
	}
	page.MustEval(`()=>auditTest.toggleNotificationCenter(document.getElementById('vd-notification-button'))`)
	page.MustEval(`()=>document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))`)
	if !page.MustEval(`()=>document.getElementById('vd-notification-center').hidden&&document.activeElement.id==='vd-notification-button'`).Bool() {
		t.Fatal("notifications Escape did not return focus")
	}
}

func TestDesktopAuditAsyncBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`()=>{
   window.baseFetch=fetch;window.searches={};window.chatCancelled=false;
   window.fetch=(url,options={})=>{
     if(String(url).startsWith('/api/desktop/search?'))return new Promise(resolve=>{searches[new URL(url,location.origin).searchParams.get('query')]=resolve;});
     if(url==='/api/desktop/chat/stream'){window.chatSignal=options.signal;return Promise.resolve(new Response(new ReadableStream({cancel(){chatCancelled=true;}})));}
     return baseFetch(url,options);
   };
   auditTest.openSpotlight();const input=document.querySelector('.vd-spotlight-input');input.value='older';input.dispatchEvent(new Event('input'));
 }`)
	page.MustWait(`()=>!!searches.older`)
	page.MustEval(`()=>{const i=document.querySelector('.vd-spotlight-input');i.value='newer';i.dispatchEvent(new Event('input'));}`)
	page.MustWait(`()=>!!searches.newer`)
	page.MustEval(`()=>searches.newer(new Response(JSON.stringify({files:[{path:'newer.txt',name:'CURRENT'}]}),{headers:{'Content-Type':'application/json'}}))`)
	page.MustWait(`()=>document.querySelector('[data-spotlight-results]').textContent.includes('CURRENT')`)
	page.MustEval(`()=>searches.older(new Response(JSON.stringify({files:[{path:'older.txt',name:'STALE'}]}),{headers:{'Content-Type':'application/json'}}))`)
	page.MustEval(`()=>{auditTest.closeSpotlight();auditTest.openSpotlight();}`)
	if page.MustEval(`()=>document.querySelector('[data-spotlight-results]').textContent.includes('STALE')`).Bool() {
		t.Fatal("old search entered reopened Spotlight")
	}
	page.MustEval(`()=>{
   auditTest.closeSpotlight();const host=document.createElement('div');document.body.append(host);window.chatHost=host;auditTest.renderQuickChatWidget(host);
   host.querySelector('input').value='fixture';host.querySelector('form').dispatchEvent(new Event('submit',{cancelable:true}));
 }`)
	page.MustWait(`()=>!!window.chatSignal`)
	page.MustEval(`()=>{auditTest.clearWidgetRuntime();chatHost.remove();}`)
	page.MustWait(`()=>chatSignal.aborted&&chatCancelled&&!auditTest.state.chatBusy`)
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}

func TestDesktopPrintSandboxBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
   window.printAttack=0;window.printCount=0;
   window.printLife=new AbortController();
   window.printJob=await AuraDesktopPrint.create({title:'Print fixture',signal:printLife.signal,html:'<!doctype html><html><body><script>parent.printAttack++</scr'+'ipt><svg onload="parent.printAttack++"></svg><img onload="parent.printAttack++" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a2n8AAAAASUVORK5CYII="><p>Printable content</p></body></html>'});
   printJob.frame.contentWindow.print=()=>{printCount++;};await printJob.print();
 }`)
	if !page.MustEval(`()=>printAttack===0&&printCount===1&&printJob.frame.sandbox.contains('allow-modals')&&!printJob.frame.sandbox.contains('allow-scripts')&&printJob.document.images[0].naturalWidth===1&&printJob.document.body.textContent.includes('Printable content')`).Bool() {
		t.Fatal("print sandbox or resource loading failed")
	}
	page.MustEval(`()=>printLife.abort()`)
	if page.MustEval(`()=>printJob.frame.isConnected`).Bool() {
		t.Fatal("print frame survived owner cleanup")
	}
}

func TestDesktopFileConflictBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`()=>{
        window.originalFetch=window.fetch;window.fileWrites=[];
        window.fetch=async (url,options)=>{
            if(url!=='/api/desktop/file'||options?.method!=='PUT')return originalFetch(url,options);
            const h=new Headers(options.headers),b=JSON.parse(options.body);fileWrites.push({path:b.path,match:h.get('If-Match'),create:h.get('If-None-Match')});
            if(h.has('If-Match')||b.path.includes('(1)'))return new Response(JSON.stringify({path:b.path,version:'"new"'}),{headers:{'Content-Type':'application/json'}});
            return new Response(JSON.stringify({code:'file_conflict',conflict:{path:b.path,version:'"observed"'}}),{status:412,headers:{'Content-Type':'application/json'}});
        };
        window.startWrite=()=>{window.writeResult=null;window.writeError='';auditTest.api('/api/desktop/file',{method:'PUT',body:JSON.stringify({path:'Documents/note.txt',content:'new'})}).then(r=>writeResult=r).catch(e=>{writeError=e.name;window.writeDetails=e.stack;});};
        startWrite();
    }`)
	page.MustWait(`()=>!!document.querySelector('[data-choice="replace"]')||!!window.writeError||!!window.writeResult`)
	if got := page.MustEval(`()=>window.writeDetails||JSON.stringify(window.writeResult)`).Str(); got != "null" {
		t.Fatalf("write completed without decision: %s", got)
	}
	page.MustElement(`[data-choice="replace"]`).MustClick()
	page.MustWait(`()=>!!window.writeResult`)
	if got := page.MustEval(`()=>fileWrites[1].match`).Str(); got != `"observed"` {
		t.Fatalf("replace used %q", got)
	}
	page.MustEval(`()=>startWrite()`)
	page.MustElement(`[data-choice="copy"]`).MustClick()
	page.MustWait(`()=>!!window.writeResult`)
	if got := page.MustEval(`()=>writeResult.path`).Str(); got != "Documents/note (1).txt" {
		t.Fatal(got)
	}
	page.MustEval(`()=>{window.beforeCancel=fileWrites.length;startWrite();}`)
	page.MustElement(`[data-cancel]`).MustClick()
	page.MustWait(`()=>window.writeError==='AbortError'`)
	if !page.MustEval(`()=>fileWrites.length===beforeCancel+1`).Bool() {
		t.Fatal("cancel retried mutation")
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}

package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

// Real shell, xterm and CRT; the socket never opens a host shell.
const terminalRetroFixture = `
window.fixtureErrors=[];
addEventListener('error',e=>fixtureErrors.push(e.message));
addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
window.fixtureSockets=[];window.fixtureCrt=[];window.fixtureTerms=[];window.fixtureDraws=0;
const draw=WebGLRenderingContext.prototype.drawArrays;
WebGLRenderingContext.prototype.drawArrays=function(...args){
 fixtureDraws++;const result=draw.apply(this,args);
 if(window.fixtureCaptureResolve){
  const pixels=new Uint8Array(this.drawingBufferWidth*this.drawingBufferHeight*4);
  this.readPixels(0,0,this.drawingBufferWidth,this.drawingBufferHeight,this.RGBA,this.UNSIGNED_BYTE,pixels);
  let hash=0,lit=0;for(let i=0;i<pixels.length;i+=4){hash=(Math.imul(hash,31)+pixels[i]+pixels[i+1]*3+pixels[i+2]*7)|0;if(Math.max(pixels[i],pixels[i+1],pixels[i+2])>120)lit++;}
  const resolve=fixtureCaptureResolve;window.fixtureCaptureResolve=null;resolve({hash,lit});
 }
 return result;
};
window.fixtureCapture=()=>new Promise(resolve=>window.fixtureCaptureResolve=resolve);
const getContext=HTMLCanvasElement.prototype.getContext;
HTMLCanvasElement.prototype.getContext=function(kind,...args){
 if(window.fixtureNoGL && /webgl/.test(kind))return null;
 return getContext.call(this,kind,...args);
};
const createCrt=TerminalCrt.create;
TerminalCrt.create=function(opts){const crt=createCrt(opts);fixtureCrt.push(crt);return crt;};
const Xterm=Terminal;
window.Terminal=class extends Xterm{constructor(opts){super({...opts,cursorBlink:false});fixtureTerms.push(this);}loadAddon(a){try{return super.loadAddon(a);}catch(e){fixtureErrors.push('addon: '+e.message);throw e;}}};
window.WebSocket=class {
 static OPEN=1;static CLOSED=3;
 constructor(){this.readyState=1;this.sent=[];fixtureSockets.push(this);setTimeout(()=>this.onopen?.(),0);}
 send(value){this.sent.push(value);}close(){this.readyState=3;this.onclose?.();}
};
const nativeFetch=window.fetch.bind(window);
window.fetch=(url,opts)=>String(url).startsWith('/api/')?Promise.resolve(new Response('{}')):nativeFetch(url,opts);
window.fixtureWrite=()=>new Promise(resolve=>fixtureTerms.at(-1).write(
 '\x1b[2J\x1b[Haurago@homelab:~$ uname -a\r\n'+
 'Linux homelab 6.12.0 x86_64 GNU/Linux\r\n\r\n'+
 '                 A U R A G O\r\n'+
 '          PERSONAL AI / HOME LAB SYSTEM\r\n\r\n'+
 '  CPU     [||||||||.................]  32%\r\n'+
 '  MEMORY  [||||||||||||.............]  48%\r\n'+
 '  UPTIME  12 days, 08:42:17\r\n\r\n'+
 '\x1b[32m  [OK]\x1b[0m Agent online\r\n'+
 '\x1b[33m  [OK]\x1b[0m Workspace mounted\r\n'+
 '\x1b[36m  [OK]\x1b[0m All services ready\r\n\r\n'+
 'aurago@homelab:~$ ls\r\n'+
 'documents/  projects/  scripts/  readme.md\r\n\r\n'+
 'The quick brown fox jumps over the lazy dog.\r\n'+
 '0123456789  <> [] {} / \\ | @ # $ % & *\r\n\r\n'+
 'aurago@homelab:~$ ',resolve));
window.fixtureReady=(async()=>{
 const words=await (await nativeFetch('/lang/desktop/de.json')).json();
 window.i18n={t:key=>words[key]||key};window.t=key=>words[key]||key;
 localStorage.setItem('aurago.desktop.terminal.style','amber');
 localStorage.setItem('aurago.desktop.terminal.audioMuted','true');
 terminalTest.state.bootstrap={enabled:true,builtin_apps:[{id:'terminal',name:'Terminal',icon:'terminal'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};
 document.body.dataset.theme='standard';document.body.dataset.animations='false';
 document.getElementById('vd-disabled').hidden=true;
 await terminalTest.loadIconManifest();terminalTest.openApp('terminal');
})();
window.fixtureStyle=id=>{const s=document.querySelector('select[data-terminal-style]');s.value=id;s.dispatchEvent(new Event('change'));};
`

func TestDesktopTerminalRetroBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/xterm.css"><link rel="stylesheet" href="/css/desktop-app-terminal.css"></head>`, 1)
	scripts := []string{"/terminal-shell.js", "/js/vendor/xterm.min.js", "/js/vendor/xterm-addon-fit.min.js", "/js/vendor/xterm-addon-canvas.min.js", "/js/desktop/apps/terminal-styles.js", "/js/desktop/apps/terminal-crt.js", "/js/desktop/apps/terminal-audio.js", "/js/desktop/apps/terminal.js", "/terminal-fixture.js"}
	var tags strings.Builder
	for _, src := range scripts {
		fmt.Fprintf(&tags, `<script src="%s"></script>`, src)
	}
	html = strings.Replace(html, "</body>", tags.String()+"</body>", 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.terminalTest={state,openApp,loadIconManifest,closeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	for route, source := range map[string]string{"/fixture": html, "/terminal-shell.js": shell, "/terminal-fixture.js": terminalRetroFixture} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			if route == "/fixture" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			} else {
				w.Header().Set("Content-Type", "text/javascript")
			}
			fmt.Fprint(w, source)
		})
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	// A real browser scale is needed: CDP emulation alone leaves ResizeObserver's
	// devicePixelContentBoxSize at the host scale and mis-sizes xterm's canvases.
	browser := rod.New().ControlURL(launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("disable-gpu").Set("force-device-scale-factor", "2").MustLaunch()).MustConnect()
	defer browser.MustClose()
	page := browser.MustPage().Timeout(120 * time.Second)
	defer page.Close()
	page.MustSetViewport(1280, 850, 2, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;}`)
	waitForJSBool(t, page, `()=>fixtureTerms.length===1 && document.querySelector('[data-terminal-renderer="webgl"]')!==null`)
	check := func(t *testing.T, name, js string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatalf("%s; browser errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	snapshot := func(name string) {
		t.Helper()
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustEval(`()=>{const w=document.querySelector('.vd-window');w.style.width='960px';w.style.height='690px';w.style.left='150px';w.style.top='70px';}`)
	for _, style := range []string{"amber", "green", "vintage", "apple2", "commodore64", "ibm3278", "mono-green", "transparent-green"} {
		t.Run(style, func(t *testing.T) {
			page.MustEval(`async id=>{fixtureStyle(id);await document.fonts.ready;await new Promise(r=>setTimeout(r,120));await fixtureWrite();await new Promise(r=>setTimeout(r,100));}`, style)
			check(t, "live CRT", `()=>!fixtureCrt.at(-1).usesFallback() && document.querySelector('[data-terminal-renderer="webgl"]')!==null`)
			check(t, "one session", `()=>fixtureSockets.length===1 && fixtureSockets[0].readyState===1 && fixtureTerms.length===1`)
			check(t, "visible phosphor", `async()=>(await fixtureCapture()).lit>500`)
			check(t, "text inset inside glass", `()=>{const s=document.querySelector('.vd-terminal-screen').getBoundingClientRect(),x=document.querySelector('.xterm-screen').getBoundingClientRect();return x.left>=s.left+15 && x.top>=s.top+15 && x.right<=s.right-10 && x.bottom<=s.bottom-10;}`)
			snapshot("terminal-" + style)
		})
	}
	check(t, "reduced motion freezes time and persistence", `async()=>{document.body.dataset.animations='false';await new Promise(r=>setTimeout(r,120));const before=await fixtureCapture();await new Promise(r=>setTimeout(r,140));return before.hash===(await fixtureCapture()).hash;}`)
	page.MustEval(`()=>{fixtureStyle('amber');document.body.dataset.animations='true';}`)
	check(t, "animated grain changes output", `async()=>{await new Promise(r=>setTimeout(r,100));const before=await fixtureCapture();await new Promise(r=>setTimeout(r,150));return before.hash!==(await fixtureCapture()).hash;}`)
	page.MustEval(`()=>document.querySelector('.vd-window').classList.add('vd-space-hidden')`)
	check(t, "hidden space pauses GPU", `async()=>{await new Promise(r=>setTimeout(r,60));const n=fixtureDraws;await new Promise(r=>setTimeout(r,100));return n===fixtureDraws;}`)
	page.MustEval(`()=>{document.querySelector('.vd-window').classList.remove('vd-space-hidden');document.body.dataset.animations='false';}`)
	page.MustSetViewport(440, 700, 2, false)
	page.MustEval(`()=>{const w=document.querySelector('.vd-window');w.style.width='400px';w.style.height='580px';w.style.left='10px';w.style.top='50px';}`)
	check(t, "resize and DPR budget", `async()=>{await new Promise(r=>setTimeout(r,150));const a=document.querySelector('.vd-terminal-crt-overlay'),s=document.querySelector('.vd-terminal-screen'),r=s.getBoundingClientRect();return a.width<=Math.ceil(r.width*1.25) && a.height<=Math.ceil(r.height*1.25) && s.scrollWidth<=s.clientWidth+1;}`)
	check(t, "xterm retains device pixel geometry", `()=>{const d=fixtureTerms[0]._core._renderService.dimensions;return Math.abs(d.device.canvas.width-d.css.canvas.width*devicePixelRatio)<2;}`)
	page.MustElement(".xterm-helper-textarea").MustType(input.KeyA)
	check(t, "keyboard reaches existing session", `()=>fixtureSockets[0].sent.join('').includes('a')`)
	check(t, "selection remains available", `()=>{fixtureTerms[0].select(0,fixtureTerms[0].buffer.active.baseY,6);return fixtureTerms[0].getSelection().length>0;}`)
	page.MustEval(`()=>fixtureTerms[0].clearSelection()`)
	check(t, "keyboard focus cannot scroll the glass overlay", `()=>{const s=document.querySelector('.vd-terminal-screen'),a=document.querySelector('.vd-terminal-crt-overlay'),r=s.getBoundingClientRect(),c=a.getBoundingClientRect();return s.scrollTop===0 && Math.abs(r.top-c.top)<1 && Math.abs(r.bottom-c.bottom)<1;}`)
	snapshot("terminal-compact-dpr2")
	page.MustEval(`()=>fixtureStyle('modern')`)
	check(t, "modern restores xterm", `()=>!document.querySelector('.vd-terminal-crt-overlay') && getComputedStyle(document.querySelector('.xterm-screen')).opacity==='1' && fixtureTerms[0].options.fontSize===13`)
	page.MustEval(`()=>{window.fixtureNoGL=true;fixtureStyle('green');}`)
	check(t, "CSS fallback keeps session and readable text", `()=>fixtureCrt.at(-1).usesFallback() && document.querySelector('[data-terminal-fallback="css"]')!==null && getComputedStyle(document.querySelector('.xterm-screen')).opacity==='1' && fixtureSockets.length===1`)
	snapshot("terminal-css-fallback")
	page.MustEval(`()=>{fixtureStyle('modern');window.fixtureNoGL=false;fixtureStyle('amber');}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-terminal-renderer="webgl"]')!==null`)
	page.MustEval(`()=>document.querySelector('.vd-terminal-crt-overlay').dispatchEvent(new Event('webglcontextlost',{cancelable:true}))`)
	check(t, "context loss reveals native terminal", `()=>document.querySelector('[data-terminal-fallback="css"]')!==null && !document.querySelector('.vd-terminal-crt-overlay') && getComputedStyle(document.querySelector('.xterm-screen')).opacity==='1'`)
	page.MustEval(`()=>TerminalApp.dispose()`)
	check(t, "disposed session", `async()=>{const n=fixtureDraws;await new Promise(r=>setTimeout(r,100));return n===fixtureDraws && fixtureSockets[0].readyState===3 && !document.querySelector('.vd-terminal-crt-overlay');}`)
	check(t, "no browser errors", `()=>fixtureErrors.length===0`)
}

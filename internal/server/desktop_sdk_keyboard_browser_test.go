package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestDesktopSDKKeyboardBridgeUsesLivePortForSDKAndLegacyApps(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the headless browser test")
	}
	browserPath := ""
	for _, candidate := range []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
	} {
		if _, err := os.Stat(candidate); err == nil {
			browserPath = candidate
			break
		}
	}
	if browserPath == "" {
		t.Skip("Chrome or Edge is required for the headless browser test")
	}

	readSource := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(content)
	}
	quickconnect := readSource("../../ui/js/desktop/apps/quickconnect-launchpad-chat.js")
	core := readSource("../../ui/js/desktop/core/sdk-events-bootstrap.js")
	menus := readSource("../../ui/js/desktop/core/menus-and-routing.js")
	sdk := readSource("../../ui/js/desktop/aura-desktop-sdk.js")
	var production strings.Builder
	for _, item := range []struct{ source, name string }{
		{quickconnect, "nextSDKChannelChallenge"}, {quickconnect, "revokeSDKFrameClient"},
		{quickconnect, "sdkFrameClient"}, {quickconnect, "beginSDKChannelHandshake"},
		{quickconnect, "ensureSDKFrameLifecycleObserver"}, {quickconnect, "findSDKFrame"},
		{quickconnect, "isCurrentSDKClient"}, {quickconnect, "handleSDKChannelHandshake"},
		{menus, "isEditableTarget"}, {core, "relayGeneratedFrameKeyboardEvent"},
	} {
		production.WriteString(extractBrowserTestFunction(t, item.source, item.name))
		production.WriteByte('\n')
	}

	foreignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!doctype html><script>
window.addEventListener('message',event=>{if(event.data&&event.data.type==='aurago.desktop.key-event')parent.postMessage({type:'foreign-key',kind:'message'},'*');});
window.addEventListener('keydown',()=>parent.postMessage({type:'foreign-key',kind:'keydown'},'*'));
parent.postMessage({type:'foreign-ready'},'*');
</script>`)
	}))
	defer foreignServer.Close()

	var appServer *httptest.Server
	appServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sdk.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = fmt.Fprint(w, sdk)
		case "/app":
			w.Header().Set("Content-Type", "text/html")
			sdkTag := ""
			if r.URL.Query().Get("id") == "sdk" {
				sdkTag = `<script src="/sdk.js"></script>`
			}
			appHTML := `<!doctype html><html><head><meta charset="utf-8">__SDK_TAG__</head><body>
<button id="focus" tabindex="0">Focus</button><canvas id="canvas" tabindex="0"></canvas><script>
const id=new URLSearchParams(location.search).get('id');
function report(target,event){parent.postMessage({type:'keyboard-report',id,target,eventType:event.type,key:event.key,code:event.code,location:event.location,repeat:event.repeat,ctrlKey:event.ctrlKey,shiftKey:event.shiftKey,altKey:event.altKey,metaKey:event.metaKey},'*');}
document.querySelector('#focus').addEventListener('keydown',event=>report('focus',event));
document.querySelector('#focus').addEventListener('keyup',event=>report('focus',event));
document.querySelector('#canvas').addEventListener('keydown',event=>report('canvas',event));
document.querySelector('#canvas').addEventListener('keyup',event=>report('canvas',event));
document.addEventListener('keydown',event=>report('document',event),true);
document.addEventListener('keyup',event=>report('document',event),true);
window.addEventListener('keydown',event=>report('window',event),true);
window.addEventListener('keyup',event=>report('window',event),true);
document.querySelector('#focus').focus();
window.addEventListener('message',event=>{if(event.data==='self-navigate')location.href='__FOREIGN_URL__/foreign';});
</script></body></html>`
			appHTML = strings.Replace(appHTML, "__SDK_TAG__", sdkTag, 1)
			appHTML = strings.Replace(appHTML, "__FOREIGN_URL__", foreignServer.URL, 1)
			appHTML = string(injectDesktopSDKChannelHTML([]byte(appHTML)))
			appHTML = string(injectDesktopAppKeyBridgeHTML([]byte(appHTML)))
			_, _ = fmt.Fprint(w, appHTML)
		case "/fixture":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, browserHostPageForKeyboard(appServer.URL, production.String()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer appServer.Close()

	launch := launcher.New().Bin(browserPath).Headless(true).NoSandbox(true)
	browserURL, err := launch.Launch()
	if err != nil {
		t.Fatalf("launch browser: %v", err)
	}
	browser := rod.New().ControlURL(browserURL)
	if err := browser.Connect(); err != nil {
		launch.Cleanup()
		t.Fatalf("connect browser: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close(); launch.Cleanup() })
	page := browser.MustPage(appServer.URL + "/fixture").Timeout(45 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustWaitLoad()
	page.MustWait(`()=>!!window.keyboardTest?.frames.sdk&&!!window.keyboardTest?.frames.legacy`)
	page.MustEval(`async()=>{const end=Date.now()+12000;while(Date.now()<end){const t=window.keyboardTest;if(t.frames.sdk.isConnected&&t.frames.legacy.isConnected&&isCurrentSDKClient(sdkFrameClients.get(t.frames.sdk))&&isCurrentSDKClient(sdkFrameClients.get(t.frames.legacy)))return;await new Promise(r=>setTimeout(r,20));}}`)
	if !page.MustEval(`()=>isCurrentSDKClient(sdkFrameClients.get(keyboardTest.frames.sdk))&&isCurrentSDKClient(sdkFrameClients.get(keyboardTest.frames.legacy))`).Bool() {
		t.Fatalf("SDK handshake did not establish live ports: %s", page.MustEval(`()=>JSON.stringify({sdk:sdkFrameClients.get(keyboardTest.frames.sdk),legacy:sdkFrameClients.get(keyboardTest.frames.legacy),errors:keyboardTest.errors})`).Str())
	}

	page.MustEval(`()=>{
 const t=keyboardTest;
 document.addEventListener('keydown',relayGeneratedFrameKeyboardEvent);
 document.addEventListener('keyup',relayGeneratedFrameKeyboardEvent);
 for(const id of ['sdk','legacy']){
  t.state.activeWindowId=id;
  document.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',code:'ArrowRight',location:2,repeat:true,shiftKey:true,bubbles:true,cancelable:true}));
  document.dispatchEvent(new KeyboardEvent('keyup',{key:'ArrowRight',code:'ArrowRight',location:2,shiftKey:true,bubbles:true,cancelable:true}));
 }
}`)
	page.MustEval(`async()=>{const end=Date.now()+5000;while(Date.now()<end&&['sdk','legacy'].some(id=>!keyboardTest.reports[id].some(event=>event.target==='window'&&event.eventType==='keyup')))await new Promise(r=>setTimeout(r,10));}`)
	if !page.MustEval(`()=>['sdk','legacy'].every(id=>{const r=keyboardTest.reports[id];return ['focus','canvas','document','window'].every(target=>r.some(e=>e.target===target&&e.eventType==='keydown'&&e.key==='ArrowRight'&&e.code==='ArrowRight'&&e.location===2&&e.repeat===true&&e.shiftKey===true)&&r.some(e=>e.target===target&&e.eventType==='keyup'&&e.key==='ArrowRight'&&e.code==='ArrowRight'&&e.location===2&&e.shiftKey===true));})`).Bool() {
		t.Fatalf("SDK and SDK-less apps must receive keydown/keyup on focused, canvas, document and window targets with key fields intact: %s", page.MustEval(`()=>JSON.stringify(keyboardTest.reports)`).Str())
	}

	page.MustEval(`()=>keyboardTest.frames.sdk.contentWindow.postMessage('self-navigate','*')`)
	if !page.MustEval(`async()=>{const end=Date.now()+8000;while(Date.now()<end){const t=keyboardTest,frame=t.frames.sdk,client=sdkFrameClients.get(frame);if(t.foreignReady&&client&&client.port===null&&client.challenge!==0&&!isCurrentSDKClient(client))return true;await new Promise(r=>setTimeout(r,20));}return false;}`).Bool() {
		t.Fatalf("foreign document load must revoke the prior frame port: %s", page.MustEval(`()=>JSON.stringify({foreignReady:keyboardTest.foreignReady,client:sdkFrameClients.get(keyboardTest.frames.sdk)})`).Str())
	}
	if !page.MustEval(`()=>{
 const t=keyboardTest,frame=t.frames.sdk,client=sdkFrameClients.get(frame);
 const before=t.foreignKeys.length;
 t.state.activeWindowId='sdk';
 const handled=relayGeneratedFrameKeyboardEvent(new KeyboardEvent('keydown',{key:'x',code:'KeyX',bubbles:true,cancelable:true}));
 return !handled&&client.port===null&&client.challenge!==0&&before===t.foreignKeys.length;
}`).Bool() {
		t.Fatalf("foreign self-navigation must revoke the live port before keyboard relay: %s", page.MustEval(`()=>JSON.stringify({foreign:keyboardTest.foreignKeys,client:sdkFrameClients.get(keyboardTest.frames.sdk)})`).Str())
	}
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,100))`)
	if page.MustEval(`()=>keyboardTest.foreignKeys.length`).Int() != 0 {
		t.Fatalf("foreign page received keyboard activity: %s", page.MustEval(`()=>JSON.stringify(keyboardTest.foreignKeys)`).Str())
	}

	page.MustEval(`async()=>{
 const t=keyboardTest,frame=t.frames.legacy,client=sdkFrameClients.get(frame),before=t.reports.legacy.length;
 t.removedClient=client;
 frame.remove();
 await new Promise(resolve=>setTimeout(resolve,50));
 t.state.activeWindowId='legacy';
 const handled=relayGeneratedFrameKeyboardEvent(new KeyboardEvent('keydown',{key:'z',code:'KeyZ',bubbles:true,cancelable:true}));
	 t.removedRelayHandled=handled;
	 t.removedRelayReportsUnchanged=t.reports.legacy.length===before;
	}`)
	if !page.MustEval(`()=>{
	 const t=keyboardTest,frame=t.frames.legacy,client=t.removedClient;
	 return client.port===null&&!isCurrentSDKClient(client)&&!frame.isConnected&&!t.removedRelayHandled&&t.removedRelayReportsUnchanged;
	}`).Bool() {
		t.Fatalf("removing an app frame must close its keyboard channel and stop relays: %s", page.MustEval(`()=>JSON.stringify({reports:keyboardTest.reports.legacy,client:sdkFrameClients.get(keyboardTest.frames.legacy)})`).Str())
	}
}

func browserHostPageForKeyboard(appURL, production string) string {
	page := strings.ReplaceAll(`<!doctype html><html><head><meta charset="utf-8"></head><body><script>
const SDK_CHANNEL_CHALLENGE_TYPE='aurago.desktop.channel.challenge',SDK_CHANNEL_HANDSHAKE_TYPE='aurago.desktop.channel.handshake';
const sdkFrameClients=new Map();let sdkChallengeSequence=0,sdkFrameObserver=null;
const state={bootstrap:{widgets:[]},activeWindowId:'sdk'};
function handleSDKMessage(){}function allApps(){return [{id:'sdk'},{id:'legacy'}];}
function cssSel(value){return CSS.escape(String(value));}
window.keyboardTest={state,frames:{},reports:{sdk:[],legacy:[]},foreignKeys:[],foreignReady:false,errors:[]};
window.addEventListener('error',event=>keyboardTest.errors.push(event.message));
window.addEventListener('message',event=>{const msg=event.data||{};if(msg.type==='keyboard-report')keyboardTest.reports[msg.id].push(msg);if(msg.type==='foreign-key')keyboardTest.foreignKeys.push(msg);if(msg.type==='foreign-ready')keyboardTest.foreignReady=true;});
/*PRODUCTION_FUNCTIONS*/
window.addEventListener('message',handleSDKChannelHandshake);
ensureSDKFrameLifecycleObserver();
function createFrame(id,cap){const frame=document.createElement('iframe');frame.className='vd-generated-frame';frame.dataset.appId=id;frame.dataset.windowId=id;frame.dataset.sdkChannel=cap;frame.sandbox='allow-scripts';frame.addEventListener('load',()=>beginSDKChannelHandshake(frame));keyboardTest.frames[id]=frame;frame.src='__APP_URL__/app?id='+id+'#__aurago_sdk_channel='+cap;document.body.appendChild(frame);}
createFrame('sdk','a'.repeat(64));createFrame('legacy','b'.repeat(64));
</script></body></html>`, "/*PRODUCTION_FUNCTIONS*/", production)
	return strings.ReplaceAll(page, "__APP_URL__", appURL)
}

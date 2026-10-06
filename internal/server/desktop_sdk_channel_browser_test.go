package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestDesktopSDKChannelUsesProductionLifecycleAndActions(t *testing.T) {
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
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(content)
	}
	qc := readSource("../../ui/js/desktop/apps/quickconnect-launchpad-chat.js")
	core := readSource("../../ui/js/desktop/core/sdk-events-bootstrap.js")
	sdk := readSource("../../ui/js/desktop/aura-desktop-sdk.js")
	var production strings.Builder
	for _, item := range []struct{ source, name string }{
		{qc, "nextSDKChannelChallenge"}, {qc, "revokeSDKFrameClient"}, {qc, "sdkFrameClient"}, {qc, "beginSDKChannelHandshake"},
		{qc, "findSDKFrame"}, {qc, "isCurrentSDKClient"}, {qc, "handleSDKChannelHandshake"}, {qc, "sendSDKResponse"}, {qc, "postSDKMenuAction"},
		{core, "declaredPermissions"}, {core, "hasPermission"}, {core, "requirePermission"}, {core, "handleSDKMessage"}, {core, "runSDKAction"},
	} {
		production.WriteString(extractBrowserTestFunction(t, item.source, item.name))
		production.WriteByte('\n')
	}
	appHTML := `<!doctype html><html><head><meta charset="utf-8"><script src="/sdk.js"></script></head><body><script>
const id=new URLSearchParams(location.search).get('id');
const report=value=>parent.postMessage(Object.assign({type:'app-report',id,origin:window.origin,hash:location.hash},value),'*');
AuraDesktop.ui.menu.onAction(actionId=>report({kind:'menu',actionId}));
window.addEventListener('message',async event=>{if(event.data!=='test-permission-revocation')return;try{await AuraDesktop.fs.write('Documents/Notes/shared.md','permission revoked');report({kind:'permission-check',ok:true});}catch(error){report({kind:'permission-check',ok:false,message:error.message});}});
(async()=>{try{const path='Documents/Notes/shared.md';const read=await AuraDesktop.fs.read(path);
if(id==='one'){const first=await AuraDesktop.fs.write(path,'first');const second=await AuraDesktop.fs.write(path,'second');report({kind:'workflow',readVersion:read.version,firstVersion:first.version,secondVersion:second.version});}
else{await new Promise(resolve=>setTimeout(resolve,350));await AuraDesktop.fs.write(path,'stale sibling',read.version);report({kind:'unexpected-sibling-success',readVersion:read.version});}
}catch(error){report({kind:'workflow-error',status:error.status||0,message:error.message});}})();
</script></body></html>`
	replacementHTML := `<!doctype html><html><body><script>
window.addEventListener('message',event=>{const msg=event.data||{};if(msg.type!=='aurago.desktop.channel.challenge')return;
const channel=new MessageChannel();event.source.postMessage({type:'aurago.desktop.channel.handshake',capability:'0'.repeat(64),challenge:msg.challenge},event.origin,[channel.port2]);
channel.port1.postMessage({type:'aurago.desktop.request',id:'forged',action:'desktop:context',payload:{}});
});
</script></body></html>`
	var appServer *httptest.Server
	appServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sdk.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(sdk))
		case "/app":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(injectDesktopSDKChannelHTML([]byte(appHTML)))
		case "/replacement":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(replacementHTML))
		case "/fixture":
			w.Header().Set("Content-Type", "text/html")
			page := strings.ReplaceAll(browserHostPage, "__APP_SERVER_URL__", appServer.URL)
			page = strings.Replace(page, "/*PRODUCTION_FUNCTIONS*/", production.String(), 1)
			_, _ = w.Write([]byte(page))
		default:
			http.NotFound(w, r)
		}
	}))
	defer appServer.Close()
	foreignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(replacementHTML))
	}))
	defer foreignServer.Close()
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
	if !page.MustEval(`()=>!!window.sdkTest`).Bool() {
		t.Fatalf("browser fixture script failed: %s; source start: %s", page.MustEval(`()=>JSON.stringify(window.__testErrors||[])`).Str(), page.MustEval(`()=>JSON.stringify((document.scripts[1]?.textContent||'').split('\\n').slice(0,12))`).Str())
	}
	page.MustEval(`async()=>{const end=Date.now()+12000;while(Date.now()<end&&(!window.sdkTest.reports.one||!window.sdkTest.reports.two))await new Promise(r=>setTimeout(r,20));}`)
	if !page.MustEval(`()=>window.sdkTest.reports.one?.kind==='workflow'&&window.sdkTest.reports.one.firstVersion==='"v2"'&&window.sdkTest.reports.one.secondVersion==='"v3"'&&window.sdkTest.reports.two?.kind==='workflow-error'&&window.sdkTest.reports.two.status===412`).Bool() {
		t.Fatalf("production SDK action handler must retain updated per-frame versions and reject a stale sibling: %s", page.MustEval(`()=>JSON.stringify(window.sdkTest)`).Str())
	}
	page.MustEval(`()=>{window.sdkTest.expectedCalls=window.sdkTest.apiCalls;window.sdkTest.permissions=[];window.sdkTest.frames.one.contentWindow.postMessage('test-permission-revocation','*');}`)
	page.MustEval(`async()=>{const end=Date.now()+3000;while(Date.now()<end&&window.sdkTest.permissionCheck?.kind!=='permission-check')await new Promise(r=>setTimeout(r,10));}`)
	if !page.MustEval(`()=>window.sdkTest.permissionCheck?.ok===false&&window.sdkTest.apiCalls===window.sdkTest.expectedCalls`).Bool() {
		t.Fatalf("SDK requests must re-resolve app manifest permissions after handshake: %s", page.MustEval(`()=>JSON.stringify(window.sdkTest.permissionCheck)`).Str())
	}
	if !page.MustEval(`()=>window.sdkTest.reports.one.origin==='null'&&window.sdkTest.reports.two.origin==='null'&&window.sdkTest.reports.one.hash==='#section=one'&&window.sdkTest.reports.two.hash==='#section=two'`).Bool() {
		t.Fatalf("document-bound bootstrap must restore fragment hashes inside opaque app frames: %s", page.MustEval(`()=>JSON.stringify(window.sdkTest.reports)`).Str())
	}
	page.MustEval(`()=>postSDKMenuAction('one','only-one')`)
	page.MustEval(`async()=>{const end=Date.now()+3000;while(Date.now()<end&&!window.sdkTest.actions.one.length)await new Promise(r=>setTimeout(r,10));}`)
	if !page.MustEval(`()=>window.sdkTest.actions.one[0]==='only-one'&&window.sdkTest.actions.two.length===0`).Bool() {
		t.Fatalf("SDK channel action leaked between windows: %s", page.MustEval(`()=>JSON.stringify(window.sdkTest.actions)`).Str())
	}
	initialCalls := page.MustEval(`()=>{window.sdkTest.expectedCalls=window.sdkTest.apiCalls;return window.sdkTest.apiCalls;}`).Int()
	page.MustEval(`url=>{window.sdkTest.frames.one.src=url;}`, foreignServer.URL+"/replacement")
	page.MustEval(`url=>{window.sdkTest.frames.two.src=url;}`, appServer.URL+"/replacement")
	page.MustEval(`async()=>{const end=Date.now()+3000;while(Date.now()<end&&window.sdkTest.rejected<2)await new Promise(r=>setTimeout(r,10));}`)
	if !page.MustEval(`()=>window.sdkTest.rejected>=2&&window.sdkFrameClients.get(window.sdkTest.frames.one).port===null&&window.sdkFrameClients.get(window.sdkTest.frames.two).port===null&&window.sdkTest.apiCalls===window.sdkTest.expectedCalls`).Bool() {
		t.Fatalf("foreign and same-origin opaque replacements must not inherit SDK capability: %s", page.MustEval(`()=>JSON.stringify({rejected:window.sdkTest.rejected,calls:window.sdkTest.apiCalls,expected:window.sdkTest.expectedCalls})`).Str())
	}
	if initialCalls <= 0 {
		t.Fatal("test fixture did not exercise production SDK action handlers")
	}
}

const browserHostPage = `<!doctype html><html><head><script>window.__testErrors=[];window.addEventListener('error',event=>window.__testErrors.push(event.message+' at '+event.filename+':'+event.lineno));</script></head><body><script>
const SDK_CHANNEL_CHALLENGE_TYPE='aurago.desktop.channel.challenge',SDK_CHANNEL_HANDSHAKE_TYPE='aurago.desktop.channel.handshake';
const SDK_REQUEST_TYPE='aurago.desktop.request',SDK_RESPONSE_TYPE='aurago.desktop.response',SDK_RUNTIME='aura-desktop-sdk@1';
const APP_URL='__APP_SERVER_URL__';
const sdkFrameClients=new Map();window.sdkFrameClients=sdkFrameClients;let sdkChallengeSequence=0;
const state={bootstrap:{widgets:[]},iconManifest:{},iconThemeManifests:{}};
const allApps=()=>[{id:'app',name:'Fixture',permissions:sdkTest.permissions}];
const sdkTest={frames:{},reports:{},actions:{one:[],two:[]},apiCalls:0,rejected:0,latest:'"v1"',readWaiters:[],expectedCalls:0,permissions:['files:read','files:write']};window.sdkTest=sdkTest;
window.addEventListener('message',event=>{const msg=event.data||{};if(msg.type==='app-report'){if(msg.kind==='menu')sdkTest.actions[msg.id].push(msg.actionId);else if(msg.kind==='permission-check')sdkTest.permissionCheck=msg;else sdkTest.reports[msg.id]=msg;return;}if(msg.type==='aurago.desktop.channel.handshake')setTimeout(()=>{const client=sdkFrameClients.get(findSDKFrame(event.source));if(!client?.port)sdkTest.rejected++;},0);});
function t(key){return key;}function sdkBootstrap(){return {};}function settingValue(){return '';}function cssSel(value){return CSS.escape(String(value));}
function resizeWidgetToContent(){}function reloadWidgetFrame(){return Promise.resolve(true);}function setWindowMenus(){}function clearWindowMenus(){}function showContextMenu(){}function closeContextMenu(){}function showDesktopNotification(){}function openApp(){}
function openDesktopFileDialog(){return Promise.resolve({canceled:true});}function saveDesktopFileDialog(){return Promise.resolve({canceled:true});}function importHostFiles(){return Promise.resolve({canceled:true});}function exportWorkspaceFile(){return Promise.resolve({});}function loadBootstrap(){return Promise.resolve();}
async function api(url,options={}){const parsed=new URL(url,location.origin);sdkTest.apiCalls++;if(options.signal?.aborted)throw new DOMException('Aborted','AbortError');
if(parsed.pathname==='/api/desktop/file'&&(!options.method||options.method==='GET')){return await new Promise(resolve=>{sdkTest.readWaiters.push(()=>resolve({path:parsed.searchParams.get('path'),content:'body',version:sdkTest.latest}));if(sdkTest.readWaiters.length===2)sdkTest.readWaiters.splice(0).forEach(done=>done());});}
if(parsed.pathname==='/api/desktop/file'&&options.method==='PUT'){const headers=new Headers(options.headers||{});if(headers.get('If-Match')!==sdkTest.latest){const error=new Error('file_conflict');error.status=412;throw error;}sdkTest.latest='"v'+(Number(sdkTest.latest.replace(/\D/g,''))+1)+'"';return {path:JSON.parse(options.body).path,version:sdkTest.latest};}
return {};}
/*PRODUCTION_FUNCTIONS*/
window.addEventListener('message',handleSDKChannelHandshake);
function createFrame(id,cap,hash){const frame=document.createElement('iframe');frame.className='vd-generated-frame';frame.dataset.appId='app';frame.dataset.windowId=id;frame.dataset.sdkChannel=cap;frame.sandbox='allow-scripts';sdkTest.frames[id]=frame;frame.addEventListener('load',()=>beginSDKChannelHandshake(frame));frame.src=APP_URL+'/app?id='+id+'#__aurago_sdk_channel='+cap+'&__aurago_sdk_original_hash='+encodeURIComponent(hash);document.body.appendChild(frame);}
createFrame('one','a'.repeat(64),'#section=one');createFrame('two','b'.repeat(64),'#section=two');
</script></body></html>`

func extractBrowserTestFunction(t *testing.T, source, name string) string {
	t.Helper()
	needle := "function " + name + "("
	start := strings.Index(source, needle)
	if start < 0 {
		needle = "async function " + name + "("
		start = strings.Index(source, needle)
	}
	if start < 0 {
		t.Fatalf("production function %s not found", name)
	}
	if start >= len("async ") && source[start-len("async "):start] == "async " {
		start -= len("async ")
	}
	body := strings.Index(source[start:], "{") + start
	depth, quote := 0, byte(0)
	lineComment, blockComment, escaped := false, false, false
	for i := body; i < len(source); i++ {
		ch := source[i]
		if lineComment {
			if ch == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if ch == '*' && i+1 < len(source) && source[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '/' && i+1 < len(source) && source[i+1] == '/' {
			lineComment = true
			i++
			continue
		}
		if ch == '/' && i+1 < len(source) && source[i+1] == '*' {
			blockComment = true
			i++
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			quote = ch
			continue
		}
		if ch == '{' {
			depth++
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				return source[start : i+1]
			}
		}
	}
	t.Fatalf("production function %s has unbalanced braces", name)
	return ""
}

package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDesktopMeshCoreBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><link rel="stylesheet" href="/css/desktop-base.css"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-meshcore.css"><style>body{margin:0}#host{width:1080px;height:720px;max-width:100vw}</style></head><body class="desktop-body" data-theme="standard" data-fruity-mode="light"><div id="host"></div><script src="/js/vendor/qrcode.min.js"></script><script src="/js/desktop/apps/meshcore.js"></script></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	browser := newSmokeBrowser(t)
	page := browser.MustPage(srv.URL + "/fixture").Timeout(40 * time.Second)
	defer page.Close()
	page.MustSetViewport(1100, 760, 1, false)
	page.MustEval(`async () => {
        window.translations=await (await fetch('/lang/desktop/de.json')).json();
        window.conv='a'.repeat(64);window.channel='b'.repeat(64);window.identity='c'.repeat(64);
        window.requests=[];window.messageRows=[{id:'protected',seq:1,conversation_id:conv,direction:'incoming',origin:'radio',text:'',at:1800000000,protected:true,review:'suspicious',parts:[]}, {id:'safe',seq:2,conversation_id:conv,direction:'incoming',origin:'radio',text:'Hallo aus dem Mesh! 👋',at:1800000001,protected:false,parts:[]}, {id:'agent',seq:3,conversation_id:conv,direction:'outgoing',origin:'agent',text:'Die nächste Wetterstation ist erreichbar.',at:1800000002,send_state:'delivered',parts:[{number:1,state:'delivered'}]}];
        window.fetch=async (url,opt={})=>{
            requests.push({url:String(url),body:opt.body?JSON.parse(opt.body):null,signal:opt.signal});
            let value={ok:true};
            if(String(url).endsWith('/bootstrap')) value={enabled:true,history_days:90,history_messages:10000,channel_text_limit:126,status:{state:window.offline?'disconnected':'connected',identity_key:identity,name:'AuraGo Base'},conversations:[{id:conv,identity_key:identity,kind:'direct',target:'d'.repeat(64),name:'Bergstation',type:1,active:true,can_send:!window.offline,favorite:true,unread:2,protected:true,last_at:1800000002},{id:channel,identity_key:identity,kind:'channel',target:'e'.repeat(64),channel:0,channel_kind:'public',name:'Public',active:true,can_send:!window.offline,unread:0,last_at:1800000000,preview:'Guten Morgen!'}]};
            if(String(url).includes('/messages?')) {if(window.failHistory) throw Error('offline');if(window.holdHistory) await new Promise((resolve,reject)=>{window.releaseHistory=resolve;opt.signal.addEventListener('abort',()=>{window.abortObserved=true;reject(new DOMException('Aborted','AbortError'));});});value={messages:String(url).includes(channel)?[]:messageRows.slice().reverse()};}
            if(String(url).endsWith('/reveal')) value={text:'<img src=x onerror="window.injected=true">'};
            if(String(url).endsWith('/send')) {if(window.rejectSend){window.rejectSend=false;throw Error('lost response');}if(window.holdSend)await new Promise(resolve=>{window.releaseSend=resolve;});value={id:'manual-reserved'};}
            if(String(url).endsWith('/invitation'))value={invitation:'meshcore://channel/add?name=Team&secret='+'ab'.repeat(16)};
            return new Response(JSON.stringify(value),{status:200,headers:{'Content-Type':'application/json'}});
        };
        window.startMessenger=()=>MeshCoreApp.render(document.getElementById('host'),'mesh-test',{t:key=>translations[key]||key,updateWindowContext:(_,ctx)=>window.savedContext=ctx});startMessenger();
    }`)
	waitForJSBool(t, page, `() => document.querySelectorAll('.mc-conversation').length===2`)
	page.MustElement(".mc-conversation").MustClick()
	waitForJSBool(t, page, `() => document.querySelectorAll('.mc-message').length===3`)
	if !page.MustEval(`() => savedContext.conversation_id===conv && document.querySelector('[data-message-id="protected"] .mc-message-text').textContent===translations['desktop.meshcore_protected']`).Bool() {
		t.Fatal("protected text/context")
	}
	page.MustElement(`[data-message-id="protected"] button`).MustClick()
	waitForJSBool(t, page, `() => document.querySelector('[data-message-id="protected"] .mc-message-text').textContent.startsWith('<img')`)
	if page.MustEval(`() => !!window.injected || !!document.querySelector('.mc-message img')`).Bool() {
		t.Fatal("message HTML executed")
	}
	page.MustEval(`() => {const text=document.querySelector('textarea[data-mc-role="compose"]');text.value='🌍'.repeat(80);text.dispatchEvent(new Event('input'));}`)
	if !page.MustEval(`() => document.querySelector('[data-mc-role="counter"]').textContent==='320 B · 3/3' && [...document.querySelectorAll('[data-mc-role="parts"] pre')].every(el=>new TextEncoder().encode(el.textContent).length<=133)`).Bool() {
		t.Fatal("UTF-8 packet preview")
	}
	page.MustEval(`() => {const text=document.querySelector('textarea[data-mc-role="compose"]');text.value='🌍'.repeat(100);text.dispatchEvent(new Event('input'));}`)
	if !page.MustEval(`() => document.querySelector('[data-mc-role="send"]').disabled`).Bool() {
		t.Fatal("four packets accepted")
	}
	page.MustEval(`() => {const text=document.querySelector('textarea[data-mc-role="compose"]');text.value='Entwurf';text.dispatchEvent(new Event('input'));text.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',shiftKey:true,bubbles:true}));text.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',isComposing:true,bubbles:true}));MeshCoreApp.dispose('mesh-test');startMessenger();}`)
	waitForJSBool(t, page, `() => document.querySelectorAll('.mc-conversation').length===2`)
	page.MustElement(".mc-conversation").MustClick()
	waitForJSBool(t, page, `() => document.querySelector('[data-mc-role="compose"]').value==='Entwurf'`)
	if page.MustEval(`() => requests.some(r=>r.url.endsWith('/send'))`).Bool() {
		t.Fatal("IME/Shift+Enter sent")
	}
	page.MustEval(`() => {window.rejectSend=true;document.querySelector('[data-mc-role="compose"]').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));}`)
	waitForJSBool(t, page, `() => !document.querySelector('[data-mc-role="error"]').hidden && !document.querySelector('[data-mc-role="send"]').disabled`)
	page.MustEval(`() => {window.holdSend=true;document.querySelector('[data-mc-role="send"]').click();document.querySelector('[data-mc-role="send"]').click();}`)
	waitForJSBool(t, page, `() => !!window.releaseSend`)
	if !page.MustEval(`() => {const calls=requests.filter(r=>r.url.endsWith('/send'));return calls.length===2 && calls[0].body.id===calls[1].body.id && calls[1].body.conversation===conv && document.querySelector('[data-mc-role="send"]').disabled}`).Bool() {
		t.Fatal("duplicate send or unstable retry ID")
	}
	page.MustEval(`() => {releaseSend();window.holdSend=false;}`)
	waitForJSBool(t, page, `() => document.querySelector('[data-mc-role="compose"]').value===''`)
	page.MustEval(`() => document.querySelectorAll('.mc-conversation')[1].click()`)
	waitForJSBool(t, page, `() => savedContext.conversation_id===channel && !document.querySelector('.mc-message')`)
	page.MustElement(`[data-mc="details"]`).MustClick()
	page.MustEval(`() => [...document.querySelectorAll('.mc-detail button')].find(b=>b.textContent===translations['desktop.meshcore_mute']).click()`)
	waitForJSBool(t, page, `() => requests.some(r=>r.body?.muted===true && r.body.conversation===channel)`)
	page.MustEval(`() => [...document.querySelectorAll('.mc-detail button')].find(b=>b.textContent===translations['desktop.meshcore_share']).click()`)
	if page.MustEval(`() => requests.some(r=>r.url.endsWith('/invitation'))`).Bool() {
		t.Fatal("invitation fetched without explicit reveal")
	}
	page.MustElement(`.mc-dialog .mc-primary`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('.mc-dialog textarea')`)
	if !page.MustEval(`() => !!document.querySelector('.mc-qr canvas, .mc-qr img') && !Object.values(localStorage).some(v=>v.includes('secret=') || v.includes('ab'.repeat(16)))`).Bool() {
		t.Fatal("QR missing or secret persisted")
	}
	page.MustElement(`.mc-dialog header button`).MustClick()
	waitForJSBool(t, page, `() => !document.querySelector('.mc-dialog')`)
	if page.MustEval(`() => !!document.querySelector('.mc-dialog') || document.body.textContent.includes('secret=')`).Bool() {
		t.Fatal("invitation remains after closing")
	}
	page.MustEval(`() => document.querySelectorAll('.mc-conversation')[0].click()`)
	waitForJSBool(t, page, `() => savedContext.conversation_id===conv && document.querySelectorAll('.mc-message').length===3`)
	// Incoming updates must not move a reader away from older messages.
	page.MustEval(`() => {messageRows=Array.from({length:30},(_,i)=>({id:'row'+i,seq:i+10,direction:i%2?'incoming':'outgoing',origin:i%2?'radio':'manual',text:'Nachricht '+i+'\nFunkverbindung zur Bergstation',at:1800000100+i,send_state:'device_accepted',parts:[]}));document.querySelector('[data-mc="refresh"]').click();}`)
	waitForJSBool(t, page, `() => document.querySelectorAll('.mc-message').length>=30 && !document.querySelector('[data-mc="refresh"]').disabled`)
	page.MustEval(`() => {const box=document.querySelector('[data-mc-role="messages"]');box.scrollTop=120;window.previousTop=box.scrollTop;messageRows.push({id:'latest',seq:99,direction:'incoming',origin:'radio',text:'Neue Nachricht',at:1800001000,parts:[]});document.dispatchEvent(new CustomEvent('aurago:meshcore-change',{detail:{conversation_id:conv}}));}`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-message-id="latest"]')`)
	if !page.MustEval(`() => Math.abs(document.querySelector('[data-mc-role="messages"]').scrollTop-previousTop)<2 && !document.querySelector('[data-mc="latest"]').hidden`).Bool() {
		t.Fatal("new message moved scroll")
	}
	page.MustElement(`[data-mc="latest"]`).MustClick()
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`theme=>document.body.dataset.theme=theme`, theme)
		for _, width := range []int{1080, 600, 390} {
			page.MustEval(`width=>document.getElementById('host').style.width=width+'px'`, width)
			page.MustEval(`() => new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
			if page.MustEval(`() => {const el=document.querySelector('.vd-meshcore');return el.scrollWidth>el.clientWidth+1}`).Bool() {
				t.Fatalf("overflow %s %d", theme, width)
			}
			if width < 700 && !page.MustEval(`() => getComputedStyle(document.querySelector('.mc-sidebar')).display==='none' && getComputedStyle(document.querySelector('.mc-chat')).display==='flex'`).Bool() {
				t.Fatal("narrow chat layout")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && width != 600 {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				name := theme + "-wide.png"
				if width == 390 {
					name = theme + "-narrow.png"
				}
				if err := os.WriteFile(filepath.Join(dir, "meshcore-messenger-"+name), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustElement(`[data-mc="back"]`).MustClick()
	if !page.MustEval(`() => getComputedStyle(document.querySelector('.mc-chat')).display==='none'`).Bool() {
		t.Fatal("narrow back navigation")
	}
	page.MustElement(".mc-conversation").MustClick()
	page.MustEval(`() => {window.offline=true;document.querySelector('[data-mc="refresh"]').click();}`)
	waitForJSBool(t, page, `() => document.querySelector('[data-mc-role="status"]').dataset.state==='disconnected'`)
	if !page.MustEval(`() => document.querySelector('[data-mc-role="send"]').disabled && document.querySelector('[data-mc="add-channel"]').disabled`).Bool() {
		t.Fatal("offline send/actions enabled")
	}
	page.MustEval(`() => {window.holdHistory=true;document.dispatchEvent(new CustomEvent('aurago:meshcore-change',{detail:{conversation_id:conv}}));}`)
	waitForJSBool(t, page, `() => !!window.releaseHistory`)
	page.MustEval(`() => MeshCoreApp.dispose('mesh-test')`)
	if !page.MustEval(`() => !!window.abortObserved`).Bool() {
		t.Fatal("dispose did not abort request")
	}
}

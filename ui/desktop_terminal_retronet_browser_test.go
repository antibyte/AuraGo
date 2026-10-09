package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Real shell bundle, xterm, CRT and Retro-Net modules; fake sockets and a fake API.
const terminalRetroNetFixture = `
window.fixtureErrors=[];
addEventListener('error',e=>fixtureErrors.push(e.error&&e.error.stack||e.message));
addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason&&e.reason.stack||e.reason)));
const motion=new URLSearchParams(location.search).get('motion')||'full';
// Page-level override: the Windows animation setting flips headless Chrome's media query otherwise.
const nativeMatchMedia=window.matchMedia.bind(window);
window.matchMedia=query=>{
 if(!/prefers-reduced-motion/.test(query))return nativeMatchMedia(query);
 const matches=motion==='reduce'&&/:\s*reduce\)/.test(query);
 return {matches,media:query,onchange:null,addEventListener(){},removeEventListener(){},addListener(){},removeListener(){},dispatchEvent(){return false;}};
};
window.fixtureSockets=[];window.fixtureTerms=[];window.fixtureCalls=[];window.fixtureOscillators=0;
window.fixtureCanEdit=true;window.fixtureSettingsError='';window.fixtureDirectoryStatus=0;
// Synthesized tones: every oscillator started in the page (key clicks only sound on real keydown events,
// which the modem checks never send).
if(window.AudioContext){
 const createOscillator=AudioContext.prototype.createOscillator;
 AudioContext.prototype.createOscillator=function(...args){fixtureOscillators++;return createOscillator.apply(this,args);};
}
const Xterm=Terminal;
window.Terminal=class extends Xterm{constructor(opts){super({...opts,cursorBlink:false});fixtureTerms.push(this);}};
window.WebSocket=class{
 static CONNECTING=0;static OPEN=1;static CLOSING=2;static CLOSED=3;
 constructor(url){
  if(fixtureSockets.some(s=>s.readyState!==3))fixtureErrors.push('second live socket: '+url);
  this.url=String(url);this.readyState=0;this.sent=[];this.binaryType='blob';this.closedBy='';
  fixtureSockets.push(this);
  setTimeout(()=>{if(this.readyState===0){this.readyState=1;this.onopen&&this.onopen({});}},0);
 }
 send(value){if(this.readyState!==1){fixtureErrors.push('send on a socket that is not open: '+this.url);return;}this.sent.push(value instanceof Uint8Array?new Uint8Array(value):value);}
 close(code){if(this.readyState===3)return;this.readyState=3;this.closedBy='client';setTimeout(()=>this.onclose&&this.onclose({code:code||1005,wasClean:true}),0);}
 serverClose(code){if(this.readyState===3)return;this.readyState=3;this.closedBy='server';this.onclose&&this.onclose({code:code||1000,wasClean:true});}
};
const nativeFetch=window.fetch.bind(window);
const json=(body,status)=>Promise.resolve(new Response(JSON.stringify(body),{status:status||200,headers:{'Content-Type':'application/json'}}));
const catalogEntry=(id,name,category,protocol,host,port,kind,charset,descriptionId)=>Object.assign({id,name,description_key:'desktop.terminal_retronet_entry_'+(descriptionId||id).replace(/-/g,'_'),category,protocol,host,port,own:false},protocol==='telnet'?{kind,charset}:{user:'guest',host_key:'SHA256:'+'B'.repeat(43)});
window.fixtureCatalog=[
 catalogEntry('telehack','Telehack','classics','telnet','telehack.com',23,'world','utf8'),
 catalogEntry('towel','Star Wars ASCII','classics','telnet','towel.blinkenlights.nl',23,'world','utf8'),
 catalogEntry('vertrauen','Vertrauen','bbs','telnet','vert.synchro.net',23,'bbs','cp437'),
 catalogEntry('discworld','Discworld MUD','muds','telnet','discworld.atuin.net',4242,'world','utf8')
];
for(let i=0;i<30;i++)fixtureCatalog.push(catalogEntry('fill-'+i,'Filler MUD '+i,'muds','telnet','aardwolf.org',4000,'world','utf8','aardwolf'));
fixtureCatalog.push(catalogEntry('sshtron','SSHTron','games','ssh','sshtron.zachlatta.com',22));
window.fixtureOwn=[
 {id:'own-homeboard001',name:'Home Board',description:'My own test board',category:'own',protocol:'ssh',host:'bbs.example.org',port:2222,user:'guest',host_key:'SHA256:'+'A'.repeat(43),own:true},
 {id:'own-fresh0000001',name:'Fresh SSH',description:'Never contacted',category:'own',protocol:'ssh',host:'ssh.example.org',port:22,user:'guest',own:true}
];
window.fixtureStatus=refreshed=>{const now=new Date().toISOString();return {telehack:{state:'online',checked_at:now},towel:{state:'offline',checked_at:now,last_online_at:new Date(Date.now()-3*86400000).toISOString()},vertrauen:{state:'online',checked_at:now},discworld:{state:'unknown'},sshtron:{state:refreshed?'online':'unknown'}};};
window.fetch=(url,opts)=>{
 const options=opts||{},path=String(url).split('?')[0],method=String(options.method||'GET').toUpperCase();
 if(!path.startsWith('/api/'))return nativeFetch(url,options);
 const call={path,method,body:options.body?String(options.body):'',failed:false};fixtureCalls.push(call);
 if(path==='/api/desktop/retronet/directory'){
  if(fixtureDirectoryStatus){call.failed=true;return json({error:'retro-net is disabled'},fixtureDirectoryStatus);}
  // Entry saves re-read this list before every PUT: it always carries the current own entries.
  return json({entries:[...fixtureCatalog,...fixtureOwn],status:fixtureStatus(false),stale:true,can_edit:fixtureCanEdit});
 }
 if(path==='/api/desktop/retronet/status'&&method==='POST')return json({status:fixtureStatus(true)});
 if(path==='/api/desktop/settings'&&method==='PUT'){
  const request=JSON.parse(call.body);
  if(request.key!=='retronet.entries')return json({settings:{}});
  if(fixtureSettingsError){call.failed=true;return json({error:fixtureSettingsError},400);}
  fixtureOwn=JSON.parse(request.value).entries.map(e=>({...e,category:'own',own:true}));
  return json({settings:{}});
 }
 return json({});
};
window.fixtureTerm=()=>fixtureTerms.at(-1);
window.fixtureRoot=()=>[...document.querySelectorAll('.vd-terminal-app')].at(-1);
window.fixtureSocket=()=>fixtureSockets.at(-1);
window.fixtureLive=()=>fixtureSockets.filter(s=>s.readyState!==3);
window.fixtureFlush=()=>new Promise(r=>{const term=fixtureTerm();if(!term)return r();term.write('',r);});
window.fixtureLines=()=>{const term=fixtureTerm();if(!term||!term.buffer)return [];const b=term.buffer.active,out=[];for(let y=0;y<term.rows;y++){const line=b.getLine(b.viewportY+y);out.push(line?line.translateToString(true):'');}return out;};
window.fixtureText=async()=>{await fixtureFlush();return fixtureLines().join('\n');};
window.fixtureFlat=async()=>{await fixtureFlush();return fixtureLines().join('');};
window.fixtureSelectedLine=async()=>{await fixtureFlush();return fixtureLines().find(l=>l.startsWith('>'))||'';};
window.fixtureInput=async data=>{fixtureTerm().input(data,true);await fixtureFlush();};
// Keys typed within ~400 ms of a result are ignored (a service hanging up must not dismiss it unread).
window.fixtureDismiss=async key=>{await new Promise(r=>setTimeout(r,450));await fixtureInput(key);};
window.fixtureControl=message=>fixtureSocket().onmessage&&fixtureSocket().onmessage({data:JSON.stringify(message)});
window.fixtureData=text=>fixtureSocket().onmessage&&fixtureSocket().onmessage({data:new TextEncoder().encode(text).buffer});
window.fixtureSent=socket=>(socket||fixtureSocket()).sent.map(f=>typeof f==='string'?'txt:'+f:'bin:'+new TextDecoder().decode(f));
window.fixturePuts=()=>fixtureCalls.filter(c=>c.path==='/api/desktop/settings'&&c.method==='PUT'&&!c.failed).map(c=>JSON.parse(c.body)).filter(b=>b.key==='retronet.entries').map(b=>JSON.parse(b.value));
window.fixtureDialog=kind=>document.querySelector('dialog[data-terminal-retronet-dialog="'+kind+'"]');
window.fixtureForm=(kind,values,submit)=>{const d=fixtureDialog(kind);for(const [name,value] of Object.entries(values||{})){const f=d.querySelector('[name="'+name+'"]');f.value=value;f.dispatchEvent(new Event('change',{bubbles:true}));}if(submit)d.querySelector('button[type="submit"]').click();return true;};
window.fixtureDialogError=kind=>{const d=fixtureDialog(kind),e=d&&d.querySelector('[data-retronet-error]');return e&&!e.hidden?e.textContent:'';};
window.fixtureStyle=id=>{const s=fixtureRoot().querySelector('select[data-terminal-style]');s.value=id;s.dispatchEvent(new Event('change'));};
window.fixtureBoot=async(retronet,context)=>{
 for(const id of [...terminalTest.state.windows.keys()])terminalTest.closeWindow(id);
 const until=Date.now()+4000;
 while(terminalTest.state.windows.size&&Date.now()<until)await new Promise(r=>setTimeout(r,25));
 terminalTest.state.bootstrap.retronet_enabled=retronet;
 terminalTest.openApp('terminal',context);
 await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
};
window.fixtureReady=(async()=>{
 const words=await (await nativeFetch('/lang/desktop/de.json')).json();
 window.t=(key,params)=>{let s=words[key]||key;if(params)for(const [name,value] of Object.entries(params))s=s.split('{{'+name+'}}').join(String(value));return s;};
 window.i18n={t:window.t};
 const animated=motion==='full';
 // Subtests share the origin: start every run from the same storage.
 localStorage.clear();
 localStorage.setItem('aurago.desktop.terminal.style','amber');
 localStorage.setItem('aurago.desktop.terminal.audioMuted','1');
 localStorage.setItem('aurago.desktop.terminal.baud','0');
 localStorage.setItem('aurago.desktop.terminal.retronet.last','vertrauen');
 terminalTest.state.bootstrap={enabled:true,retronet_enabled:true,builtin_apps:[{id:'terminal',name:'Terminal',icon:'terminal'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false,'windows.animations':animated}};
 document.body.dataset.theme='standard';document.body.dataset.animations=animated?'true':'false';
 document.getElementById('vd-disabled').hidden=true;
 await terminalTest.loadIconManifest();
 terminalTest.openApp('terminal');
})();
`

func newTerminalRetroNetServer(t *testing.T) *httptest.Server {
	t.Helper()
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/xterm.css"><link rel="stylesheet" href="/css/desktop-app-terminal.css"></head>`, 1)
	scripts := []string{
		"/terminal-shell.js",
		"/js/vendor/xterm.min.js",
		"/js/vendor/xterm-addon-fit.min.js",
		"/js/vendor/xterm-addon-webgl.min.js",
		"/js/desktop/apps/terminal-styles.js",
		"/js/desktop/apps/terminal-crt.js",
		"/js/desktop/apps/terminal-audio.js",
		"/js/desktop/apps/terminal-text.js",
		"/js/desktop/apps/terminal-modem.js",
		"/js/desktop/apps/terminal-retronet-directory.js",
		"/js/desktop/apps/terminal-retronet-session.js",
		"/js/desktop/apps/terminal-retronet-entries.js",
		"/js/desktop/apps/terminal.js",
		"/terminal-retronet-fixture.js",
	}
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
	shell = shell[:cut] + `window.terminalTest={state,openApp,loadIconManifest,closeWindow,applyDesktopSettings,renderTaskbar};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	for route, source := range map[string]string{"/fixture": html, "/terminal-shell.js": shell, "/terminal-retronet-fixture.js": terminalRetroNetFixture} {
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
	t.Cleanup(srv.Close)
	return srv
}

func TestDesktopTerminalRetroNetBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	srv := newTerminalRetroNetServer(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	browser := rod.New().ControlURL(launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("disable-gpu").Set("force-device-scale-factor", "1").MustLaunch()).MustConnect()
	defer browser.MustClose()
	for _, motion := range []string{"full", "reduce"} {
		t.Run(motion, func(t *testing.T) {
			runTerminalRetroNetBrowser(t, browser, srv.URL, motion)
		})
	}
}

func runTerminalRetroNetBrowser(t *testing.T, browser *rod.Browser, base, motion string) {
	page := browser.MustPage().Timeout(240 * time.Second)
	defer page.Close()
	preference := "no-preference"
	if motion == "reduce" {
		preference = "reduce"
	}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: preference}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustSetViewport(1280, 850, 1, false)
	page.MustNavigate(base + "/fixture?motion=" + motion).MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;}`)
	full := motion == "full"

	evalBool := func(js string) (bool, error) {
		res, err := page.Eval(js)
		if err != nil {
			return false, err
		}
		return res.Value.Bool(), nil
	}
	fail := func(name string, err error) {
		t.Helper()
		screen, errs := "", ""
		if res, evalErr := page.Eval(`()=>fixtureLines().join('\n')`); evalErr == nil {
			screen = res.Value.Str()
		}
		if res, evalErr := page.Eval(`()=>JSON.stringify(fixtureErrors)`); evalErr == nil {
			errs = res.Value.Str()
		}
		t.Fatalf("%s (%v)\nscreen:\n%s\nbrowser errors: %s", name, err, screen, errs)
	}
	check := func(name, js string) {
		t.Helper()
		if ok, err := evalBool(js); err != nil || !ok {
			fail(name, err)
		}
	}
	wait := func(name, js string) {
		t.Helper()
		var lastErr error
		for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(40 * time.Millisecond) {
			ok, err := evalBool(js)
			if err == nil && ok {
				return
			}
			lastErr = err
		}
		fail(name, lastErr)
	}
	run := func(js string) {
		t.Helper()
		if _, err := page.Eval(js); err != nil {
			fail("run "+js, err)
		}
	}
	// Loaded directory: entry rows carry reachability markers (the last entry is preselected, so the visible
	// part of the list depends on it).
	backInDirectory := `async()=>{const text=await fixtureText();return fixtureRoot().dataset.terminalMode==='directory'&&text.includes(t('desktop.terminal_retronet_title'))&&/\[[* ?]\]/.test(text)&&fixtureLive().length===0;}`

	// 1. Directory: chrome, numbering, reachability, preselection, aria-live.
	wait("directory rendered and stale status refreshed", `async()=>(await fixtureText()).includes(t('desktop.terminal_retronet_title'))&&(await fixtureText()).includes('Telehack')&&fixtureCalls.some(c=>c.path==='/api/desktop/retronet/status'&&c.method==='POST')`)
	check("directory chrome and reachability markers", `async()=>{const text=await fixtureText(),root=fixtureRoot();return root.dataset.terminalMode==='directory'&&root.dataset.terminalState==='desktop.terminal_directory'&&text.includes(t('desktop.terminal_retronet_local_shell'))&&text.includes(t('desktop.terminal_retronet_cat_classics').toLocaleUpperCase())&&text.includes('Telehack')&&text.includes(t('desktop.terminal_retronet_entry_telehack').slice(0,20))&&text.includes('[*]')&&text.includes('[ ]')&&text.includes('[?]')&&text.includes(t('desktop.terminal_retronet_last_seen',{when:''}).trim())&&fixtureSockets.length===0&&root.querySelector('[data-terminal-retronet-action]').hidden&&!root.querySelector('[data-terminal-baud-label]').hidden;}`)
	check("last entry preselected and announced", `async()=>{const line=await fixtureSelectedLine(),live=fixtureRoot().querySelector('[data-terminal-announce]');return line.startsWith('>03')&&line.includes('Vertrauen')&&live.getAttribute('aria-live')==='polite'&&live.textContent.includes('Vertrauen')&&live.textContent.includes(t('desktop.terminal_retronet_status_online'));}`)
	check("admin help is shown", `async()=>(await fixtureText()).includes(t('desktop.terminal_retronet_help_admin').slice(0,12))`)

	// 2. Keyboard selection.
	run(`()=>fixtureTerm().focus()`)
	page.Keyboard.MustType(input.ArrowDown)
	wait("arrow key moves and announces the selection", `async()=>(await fixtureSelectedLine()).includes('Discworld MUD')&&fixtureRoot().querySelector('[data-terminal-announce]').textContent.includes('Discworld MUD')`)
	run(`async()=>{await fixtureInput('0');await fixtureInput('1');}`)
	check("two typed digits jump to the entry", `async()=>(await fixtureSelectedLine()).startsWith('>01')`)
	run(`async()=>{await fixtureInput('\x1b[F');}`)
	check("End scrolls to the last entry", `async()=>{const text=await fixtureText();return (await fixtureSelectedLine()).includes('Fresh SSH')&&text.includes('▲')&&!text.includes(t('desktop.terminal_retronet_local_shell'));}`)
	run(`async()=>{await fixtureInput('\x1b[H');}`)
	check("Home returns to the local shell", `async()=>{const text=await fixtureText();return (await fixtureSelectedLine()).startsWith('>00')&&text.includes('▼');}`)
	run(`async()=>{await fixtureInput('\x1b[6~');}`)
	check("PgDn moves by a page", `async()=>Number((await fixtureSelectedLine()).slice(1,3))>=10`)
	run(`async()=>{await fixtureInput('\x1b[H');}`)

	// 3. Mouse: click selects, a quick second tap dials.
	point := page.MustEval(`async()=>{await fixtureFlush();const term=fixtureTerm(),row=fixtureLines().findIndex(l=>l.includes('Telehack')),r=term.element.querySelector('.xterm-screen').getBoundingClientRect();return {x:r.left+r.width*0.4,y:r.top+(row+0.5)*r.height/term.rows};}`)
	page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num()).MustClick(proto.InputMouseButtonLeft)
	wait("a click selects without dialing", `async()=>(await fixtureSelectedLine()).includes('Telehack')&&fixtureSockets.length===0`)
	time.Sleep(650 * time.Millisecond)
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	check("a slow second click does not dial", `()=>fixtureSockets.length===0`)
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	wait("a quick second tap dials", `()=>fixtureSockets.length===1`)

	// 4. Dialing: modem transcript, buffering, CONNECT.
	check("dial opens one Retro-Net socket keyed by entry ID", `()=>{const s=fixtureSocket(),u=new URL(s.url),term=fixtureTerm(),root=fixtureRoot(),btn=root.querySelector('[data-terminal-retronet-action]');return u.pathname==='/api/desktop/retronet/connect'&&u.searchParams.get('entry')==='telehack'&&u.searchParams.get('cols')===String(term.cols)&&u.searchParams.get('rows')===String(term.rows)&&[...u.searchParams.keys()].sort().join(',')==='cols,entry,rows'&&s.binaryType==='arraybuffer'&&root.dataset.terminalMode==='dialing'&&root.dataset.terminalState==='desktop.terminal_dialing'&&fixtureLive().length===1&&localStorage.getItem('aurago.desktop.terminal.retronet.last')==='telehack'&&!btn.hidden&&btn.textContent===t('desktop.terminal_retronet_hangup');}`)
	if full {
		check("the dial line is not printed at once", `async()=>!(await fixtureText()).includes('ATDT telehack.com')`)
	} else {
		check("reduced motion prints the dial lines at once", `async()=>{const text=await fixtureText();return text.includes('ATZ')&&text.includes('OK')&&text.includes('ATDT telehack.com');}`)
	}
	wait("socket open", `()=>fixtureSocket().readyState===1`)
	run(`()=>{fixtureData('EARLY BANNER\r\n');fixtureControl({type:'connected',protocol:'telnet',kind:'world',charset:'utf8'});fixtureControl({type:'echo',remote:false,hidden:false});}`)
	if full {
		check("CONNECT waits for the dial animation", `async()=>{const text=await fixtureText();return !text.includes('CONNECT')&&!text.includes('EARLY BANNER')&&fixtureRoot().dataset.terminalMode==='dialing';}`)
		run(`async()=>{await fixtureInput('x');}`)
	}
	wait("CONNECT, Telnet notice, then the buffered data", `async()=>{const text=await fixtureText(),c=text.indexOf('CONNECT 14400'),n=text.indexOf(t('desktop.terminal_retronet_telnet_notice')),b=text.indexOf('EARLY BANNER');return text.includes('ATZ')&&text.includes('ATDT telehack.com')&&c>=0&&n>c&&b>n;}`)
	check("connected; the skip key stays local", `()=>{const root=fixtureRoot();return fixtureSent(fixtureSocket()).length===0&&root.dataset.terminalMode==='retro'&&root.dataset.terminalState==='desktop.terminal_connected'&&fixtureTerm().options.convertEol===false;}`)
	check("a muted modem plays no tones", `()=>fixtureOscillators===0`)

	// 5. Local line editing for a world entry, hidden echo (password prompt), then character mode.
	run(`async()=>{await fixtureInput('look');}`)
	check("line mode echoes locally without sending", `async()=>(await fixtureText()).includes('look')&&fixtureSent(fixtureSocket()).length===0`)
	run(`async()=>{await fixtureInput('\x7f');await fixtureInput('k\r');}`)
	check("Enter sends the edited line with CR", `()=>{const s=fixtureSent(fixtureSocket());return s.length===1&&s[0]==='bin:look\r';}`)
	run(`async()=>{await fixtureInput('\x1b[A');}`)
	check("Up recalls the previous line", `()=>{const b=fixtureTerm().buffer.active;return b.getLine(b.baseY+b.cursorY).translateToString(true).includes('look');}`)
	run(`async()=>{await fixtureInput('\r');await fixtureInput('say one\r\nsay two\r\n');await fixtureInput('\x03');}`)
	check("history line, line-by-line paste and raw Ctrl+C", `()=>fixtureSent(fixtureSocket()).join('|')==='bin:look\r|bin:look\r|bin:say one\r|bin:say two\r|bin:\x03'`)
	run(`async()=>{fixtureControl({type:'echo',remote:false,hidden:true});await fixtureInput('s3crex');await fixtureInput('\x7f');await fixtureInput('\x1b[A');}`)
	check("hidden echo buffers the line invisibly and sends nothing yet", `async()=>!(await fixtureText()).includes('s3cr')&&fixtureSent(fixtureSocket()).length===5`)
	run(`async()=>{await fixtureInput('t\r');}`)
	check("Enter sends the hidden line, which never appears in xterm", `async()=>!(await fixtureText()).includes('s3cr')&&fixtureSent(fixtureSocket()).at(-1)==='bin:s3cret\r'&&fixtureSent(fixtureSocket()).length===6`)
	run(`async()=>{fixtureControl({type:'echo',remote:false,hidden:false});await fixtureInput('\x1b[A');}`)
	check("the hidden line is not recalled by ArrowUp", `async()=>{const b=fixtureTerm().buffer.active,current=b.getLine(b.baseY+b.cursorY).translateToString(true);return current.includes('say two')&&!(await fixtureText()).includes('s3cr');}`)
	run(`async()=>{await fixtureInput('\x15');fixtureControl({type:'echo',remote:true,hidden:false});await fixtureInput('Q');await fixtureInput('Z');}`)
	check("remote:true is character mode: keys go out at once without local echo", `async()=>{const s=fixtureSent(fixtureSocket());return s.length===8&&s[6]==='bin:Q'&&s[7]==='bin:Z'&&!(await fixtureText()).includes('QZ');}`)
	run(`async()=>{await fixtureInput('A'.repeat(40000));}`)
	check("a large paste goes out in binary frames of at most 16 KiB", `()=>{const frames=fixtureSocket().sent.slice(8);return frames.length===3&&frames.every(f=>f instanceof Uint8Array&&f.byteLength<=16384)&&frames.reduce((n,f)=>n+f.byteLength,0)===40000&&fixtureSent(fixtureSocket()).slice(8).join('')==='bin:'+'A'.repeat(16384)+'bin:'+'A'.repeat(16384)+'bin:'+'A'.repeat(40000-32768);}`)
	run(`()=>fixtureControl({type:'echo',remote:false,hidden:false})`)

	// 6. Service data and world resizes.
	run(`()=>fixtureData('\r\nWillkommen in Überwald\r\n')`)
	wait("service output reaches xterm", `async()=>(await fixtureText()).includes('Willkommen in Überwald')`)
	run(`async()=>{const w=document.querySelector('.vd-window');w.style.width='820px';}`)
	wait("a world resize goes out as a JSON frame", `()=>fixtureSent(fixtureSocket()).some(f=>{if(!f.startsWith('txt:'))return false;const m=JSON.parse(f.slice(4));return m.type==='resize'&&m.cols===fixtureTerm().cols&&m.rows===fixtureTerm().rows;})`)

	// 7. Baud throttle.
	run(`()=>{const s=fixtureRoot().querySelector('select[data-terminal-baud]');s.value='300';s.dispatchEvent(new Event('change'));fixtureData('\r\n'+'#'.repeat(600)+'\r\nEND-OF-BLOCK\r\n');}`)
	wait("300 baud releases output a few bytes at a time", `async()=>(await fixtureText()).includes('#')&&localStorage.getItem('aurago.desktop.terminal.baud')==='300'`)
	check("300 baud holds the rest back", `async()=>{const text=await fixtureText();return !text.includes('END-OF-BLOCK')&&(text.match(/#/g)||[]).length<600;}`)
	run(`()=>{const s=fixtureRoot().querySelector('select[data-terminal-baud]');s.value='0';s.dispatchEvent(new Event('change'));}`)
	wait("switching the throttle off flushes the queue", `async()=>(await fixtureText()).includes('END-OF-BLOCK')`)

	// 8. Style switch keeps xterm and the socket.
	run(`()=>{window.fixtureKeep=fixtureSocket();fixtureStyle('green');}`)
	wait("the green style is applied", `()=>{const root=fixtureRoot(),theme=fixtureTerm().options.theme;
		return root.dataset.terminalStyle==='green'&&theme.foreground===TerminalStyles.profile('green').theme.foreground;}`)
	check("a style switch keeps xterm and the socket", `()=>fixtureSocket()===fixtureKeep&&fixtureKeep.readyState===1&&
		fixtureTerms.length===1&&fixtureLive().length===1&&fixtureRoot().dataset.terminalMode==='retro'`)

	// 9. Ctrl+] hangs up; any key returns to the directory with the last entry selected.
	run(`async()=>{await fixtureInput('\x1d');}`)
	wait("Ctrl+] hangs up with NO CARRIER", `async()=>{const text=await fixtureText(),root=fixtureRoot();return fixtureKeep.readyState===3&&fixtureKeep.closedBy==='client'&&text.includes('NO CARRIER')&&text.includes(t('desktop.terminal_retronet_result_local_hangup'))&&root.dataset.terminalMode==='result'&&root.dataset.terminalState==='desktop.terminal_stopped';}`)
	check("the escape key never reaches the service", `()=>!fixtureSent(fixtureKeep).some(f=>f.includes('\x1d'))`)
	run(`async()=>{await fixtureDismiss('q');}`)
	wait("any key returns to the directory", `async()=>fixtureRoot().dataset.terminalMode==='directory'&&(await fixtureSelectedLine()).includes('Telehack')&&fixtureLive().length===0`)

	// 10. A result frame from the server; a key typed while it hangs up does not dismiss the result.
	run(`async()=>{await fixtureInput('0');await fixtureInput('2');await fixtureInput('\r');}`)
	wait("second session opens", `()=>fixtureSockets.length===2&&fixtureSocket().readyState===1&&new URL(fixtureSocket().url).searchParams.get('entry')==='towel'`)
	run(`async()=>{fixtureControl({type:'result',code:'BUSY',reason:'refused'});fixtureSocket().serverClose(1000);await fixtureInput('x');}`)
	check("the result shows the Hayes code and its explanation", `async()=>{const text=await fixtureText();return text.includes('BUSY')&&text.includes(t('desktop.terminal_retronet_result_refused'))&&text.includes(t('desktop.terminal_retronet_press_key'))&&fixtureRoot().dataset.terminalMode==='result'&&fixtureLive().length===0;}`)
	run(`async()=>{await fixtureDismiss(' ');}`)
	wait("back in the directory after BUSY", backInDirectory)

	// 10b. A service that answers and hangs up during the dial: its banner shows before the result.
	run(`async()=>{await fixtureInput('0');await fixtureInput('2');await fixtureInput('\r');}`)
	wait("third session opens", `()=>fixtureSockets.length===3&&fixtureSocket().readyState===1&&new URL(fixtureSocket().url).searchParams.get('entry')==='towel'`)
	if full {
		check("the dial animation is still running", `async()=>fixtureRoot().dataset.terminalMode==='dialing'&&!(await fixtureText()).includes('CONNECT')`)
	}
	run(`()=>{fixtureControl({type:'connected',protocol:'telnet',kind:'world',charset:'utf8'});fixtureData('ALL NODES BUSY\r\n');fixtureControl({type:'result',code:'BUSY',reason:'limit'});fixtureSocket().serverClose(1000);}`)
	wait("the banner appears before the result lines", `async()=>{await fixtureFlush();const lines=fixtureLines(),find=f=>lines.findIndex(f),connect=find(l=>l.startsWith('CONNECT ')),banner=find(l=>l.includes('ALL NODES BUSY')),code=find(l=>l.trim()==='BUSY'),why=find(l=>l.includes(t('desktop.terminal_retronet_result_limit').slice(0,20)));return fixtureRoot().dataset.terminalMode==='result'&&connect>=0&&banner>connect&&code>banner&&why>code&&fixtureLive().length===0;}`)
	run(`async()=>{await fixtureDismiss('q');}`)
	wait("back in the directory after the busy banner", backInDirectory)

	// 11. BBS: fixed 80x25 grid with the VGA font.
	run(`async()=>{await fixtureInput('0');await fixtureInput('3');await fixtureInput('\r');}`)
	wait("a BBS dials with a fixed 80x25 grid", `()=>{const s=fixtureSocket(),u=new URL(s.url),term=fixtureTerm();return u.searchParams.get('entry')==='vertrauen'&&u.searchParams.get('cols')==='80'&&u.searchParams.get('rows')==='25'&&term.cols===80&&term.rows===25&&fixtureRoot().dataset.terminalGeometry==='bbs'&&term.options.fontFamily.includes('Aura VGA');}`)
	// The fit settles over two or three frames after the font has loaded.
	wait("the VGA font fills and fits the glass, centered", `async()=>{
		const loaded=(await document.fonts.load('16px "Aura VGA"')).some(f=>f.status==='loaded');
		await new Promise(r=>setTimeout(r,300));
		const term=fixtureTerm(),scr=term.element.querySelector('.xterm-screen').getBoundingClientRect(),
			glass=fixtureRoot().querySelector('.vd-terminal-screen').getBoundingClientRect();
		return loaded&&Number.isInteger(term.options.fontSize*2)&&
			scr.width<=glass.width+0.5&&scr.height<=glass.height+0.5&&
			Math.max(scr.width/glass.width,scr.height/glass.height)>0.75&&
			Math.abs((scr.left-glass.left)-(glass.right-scr.right))<=2&&
			Math.abs((scr.top-glass.top)-(glass.bottom-scr.bottom))<=2;
	}`)
	wait("BBS socket open", `()=>fixtureSocket().readyState===1`)
	run(`()=>{fixtureControl({type:'connected',protocol:'telnet',kind:'bbs',charset:'cp437'});fixtureControl({type:'echo',remote:false,hidden:false});fixtureData('\x1b[2J\x1b[HVERTRAUEN LOGIN: ');}`)
	if full {
		run(`async()=>{await fixtureInput('x');}`)
	}
	wait("BBS output arrives", `async()=>(await fixtureText()).includes('VERTRAUEN LOGIN:')`)
	run(`async()=>{await fixtureInput('guest\r');}`)
	check("BBS input stays in character mode even without remote echo", `async()=>{const s=fixtureSent(fixtureSocket());return s.length===1&&s[0]==='bin:guest\r'&&!(await fixtureText()).includes('LOGIN: guest');}`)
	run(`async()=>{window.fixtureBbsFont=fixtureTerm().options.fontSize;const w=document.querySelector('.vd-window');w.style.width='640px';w.style.height='520px';}`)
	wait("a resize refits the VGA grid", `()=>{const term=fixtureTerm(),scr=term.element.querySelector('.xterm-screen').getBoundingClientRect(),glass=fixtureRoot().querySelector('.vd-terminal-screen').getBoundingClientRect();return term.cols===80&&term.rows===25&&term.options.fontSize<fixtureBbsFont&&scr.width<=glass.width+0.5&&scr.height<=glass.height+0.5;}`)
	check("80x25 mode never sends resize frames", `()=>!fixtureSent(fixtureSocket()).some(f=>f.startsWith('txt:'))`)
	run(`()=>{window.fixtureKeep=fixtureSocket();fixtureStyle('amber');}`)
	wait("the amber style is applied in 80x25 mode", `()=>{const root=fixtureRoot(),term=fixtureTerm();
		return root.dataset.terminalStyle==='amber'&&term.options.theme.foreground===TerminalStyles.profile('amber').theme.foreground&&
			term.cols===80&&term.rows===25&&term.options.fontFamily.includes('Aura VGA');}`)
	check("a style switch keeps 80x25 and the socket", `()=>fixtureSocket()===fixtureKeep&&fixtureKeep.readyState===1&&
		fixtureLive().length===1&&fixtureTerms.length===1&&fixtureRoot().dataset.terminalGeometry==='bbs'`)
	run(`()=>fixtureRoot().querySelector('[data-terminal-retronet-action]').click()`)
	wait("the toolbar button hangs up", `async()=>(await fixtureText()).includes('NO CARRIER')&&fixtureRoot().dataset.terminalMode==='result'&&fixtureLive().length===0`)
	run(`async()=>{const w=document.querySelector('.vd-window');w.style.width='960px';w.style.height='720px';await fixtureDismiss('q');}`)
	wait("leaving 80x25 restores the style font and fit", `async()=>{const term=fixtureTerm(),root=fixtureRoot();return root.dataset.terminalMode==='directory'&&(await fixtureText()).includes('Telehack')&&!root.hasAttribute('data-terminal-geometry')&&term.options.fontFamily===TerminalStyles.profile(root.dataset.terminalStyle).fontFamily&&!term.element.style.paddingLeft&&!(term.cols===80&&term.rows===25);}`)

	// 12. SSH first contact for an own entry.
	run(`async()=>{await fixtureInput('3');await fixtureInput('7');await fixtureInput('\r');}`)
	wait("an own SSH entry dials by ID", `()=>{const u=new URL(fixtureSocket().url);return u.searchParams.get('entry')==='own-fresh0000001'&&fixtureSocket().readyState===1&&!fixtureRoot().hasAttribute('data-terminal-geometry');}`)
	run(`()=>fixtureControl({type:'hostkey_prompt',key_type:'ssh-ed25519',fingerprint:'SHA256:'+'C'.repeat(43)})`)
	wait("first contact shows the fingerprint and a localized question", `async()=>{const flat=await fixtureFlat();return flat.includes(t('desktop.terminal_retronet_hostkey_title'))&&flat.includes('ssh-ed25519')&&flat.includes('SHA256:'+'C'.repeat(43))&&flat.includes('('+t('desktop.terminal_retronet_hostkey_yes')+'/'+t('desktop.terminal_retronet_hostkey_no')+')')&&fixtureRoot().dataset.terminalMode==='dialing';}`)
	// ASCII y in one run, the localized letter (upper case) in the other; both are accepted case-insensitively.
	answer := `t('desktop.terminal_retronet_hostkey_yes').toUpperCase()`
	if full {
		answer = `'y'`
	}
	run(`async()=>{await fixtureInput('x');await fixtureInput(` + answer + `);}`)
	check("the answer goes out as hostkey_decision and echoes the localized letter", `async()=>{const s=fixtureSent(fixtureSocket());await fixtureFlush();const lines=fixtureLines(),question=lines.findIndex(l=>l.includes('('+t('desktop.terminal_retronet_hostkey_yes')+'/'));return s.length===1&&s[0]==='txt:{"type":"hostkey_decision","accept":true}'&&question>=0&&lines[question].trimEnd().endsWith(' '+t('desktop.terminal_retronet_hostkey_yes'));}`)
	run(`()=>fixtureControl({type:'connected',protocol:'ssh'})`)
	wait("SSH connects without the Telnet notice", `async()=>{const text=await fixtureText();return text.includes('CONNECT 14400')&&!text.includes(t('desktop.terminal_retronet_telnet_notice'))&&fixtureRoot().dataset.terminalMode==='retro';}`)
	run(`async()=>{await fixtureInput('\x1d');await fixtureDismiss('q');}`)
	wait("back in the directory after SSH", backInDirectory)
	check("the dialed own entry stays selected", `async()=>(await fixtureSelectedLine()).includes('Fresh SSH')`)

	// 13. Own entries (admin): validation, create, edit, server error, delete, catalog guard.
	run(`async()=>{await fixtureInput('n');}`)
	wait("N opens the new-entry dialog", `()=>!!document.querySelector('dialog[data-terminal-retronet-dialog="new"][open]')`)
	run(`()=>fixtureForm('new',{name:'New Board',host:'10.0.0.5',port:'23'},true)`)
	check("private addresses are rejected before saving", `()=>fixtureDialogError('new')===t('desktop.terminal_retronet_error_private_host')&&fixturePuts().length===0`)
	run(`()=>fixtureForm('new',{host:'bbs.example.net',port:'25'},true)`)
	check("mail ports are rejected", `()=>fixtureDialogError('new')===t('desktop.terminal_retronet_error_mail_port')&&fixturePuts().length===0`)
	run(`()=>fixtureForm('new',{port:'2323',kind:'bbs'},true)`)
	wait("the new entry is saved and selected", `async()=>!fixtureDialog('new')&&(await fixtureSelectedLine()).includes('New Board')`)
	check("the save re-read the directory, then stored the complete own-entry document", `()=>{const doc=fixturePuts().at(-1),added=doc.entries.find(e=>e.name==='New Board'),home=doc.entries.find(e=>e.id==='own-homeboard001'),put=fixtureCalls.findIndex(c=>c.method==='PUT'&&c.path==='/api/desktop/settings'),read=fixtureCalls.slice(0,put).map(c=>c.path).lastIndexOf('/api/desktop/retronet/directory');return read>=0&&doc.version===1&&doc.entries.length===3&&/^own-[a-z0-9]{12}$/.test(added.id)&&added.protocol==='telnet'&&added.kind==='bbs'&&added.charset==='cp437'&&added.host==='bbs.example.net'&&added.port===2323&&!('user' in added)&&!('own' in added)&&!('category' in added)&&home.host_key==='SHA256:'+'A'.repeat(43)&&home.user==='guest';}`)
	run(`async()=>{await fixtureInput('3');await fixtureInput('6');await fixtureInput('e');}`)
	wait("E opens the editor with the stored values", `()=>{const d=document.querySelector('dialog[data-terminal-retronet-dialog="edit"][open]');return !!d&&d.querySelector('[name="name"]').value==='Home Board'&&d.querySelector('[name="protocol"]').value==='ssh'&&d.querySelector('[name="user"]').value==='guest'&&!d.querySelector('[name="user"]').closest('label').hidden&&d.querySelector('[name="kind"]').closest('label').hidden;}`)
	run(`()=>fixtureForm('edit',{name:'Home Board II'},true)`)
	wait("a rename keeps the pinned host key", `()=>{const doc=fixturePuts().at(-1),home=doc.entries.find(e=>e.id==='own-homeboard001');return !fixtureDialog('edit')&&doc.entries.length===3&&home.name==='Home Board II'&&home.host_key==='SHA256:'+'A'.repeat(43);}`)
	run(`async()=>{await fixtureInput('3');await fixtureInput('6');await fixtureInput('e');}`)
	wait("the editor opens again", `()=>!!document.querySelector('dialog[data-terminal-retronet-dialog="edit"][open]')`)
	run(`()=>fixtureForm('edit',{port:'2223'},true)`)
	wait("changing the port clears the host key", `()=>{const home=fixturePuts().at(-1).entries.find(e=>e.id==='own-homeboard001');return !fixtureDialog('edit')&&home.port===2223&&!('host_key' in home);}`)
	run(`async()=>{window.fixtureSettingsError='invalid desktop setting value for retronet.entries';await fixtureInput('3');await fixtureInput('6');await fixtureInput('e');}`)
	wait("the editor opens for the failing save", `()=>!!document.querySelector('dialog[data-terminal-retronet-dialog="edit"][open]')`)
	run(`()=>fixtureForm('edit',{name:'Broken'},true)`)
	wait("server errors stay in the dialog", `()=>fixtureDialogError('edit')===t('desktop.terminal_retronet_error_save',{message:fixtureSettingsError})&&!!document.querySelector('dialog[data-terminal-retronet-dialog="edit"][open]')`)
	run(`()=>{window.fixtureSettingsError='';fixtureDialog('edit').querySelector('[data-retronet-cancel]').click();}`)
	check("cancel asks before discarding unsaved edits", `()=>{const d=fixtureDialog('edit');return !!d&&d.open&&!d.querySelector('[data-retronet-confirm]').hidden;}`)
	run(`()=>fixtureDialog('edit').querySelector('[data-retronet-discard]').click()`)
	wait("discard closes without saving", `()=>!fixtureDialog('edit')&&fixtureOwn.some(e=>e.name==='Home Board II')&&!fixtureOwn.some(e=>e.name==='Broken')`)
	run(`async()=>{await fixtureInput('3');await fixtureInput('8');await fixtureInput('\x1b[3~');}`)
	wait("Del asks for confirmation", `()=>{const d=document.querySelector('dialog[data-terminal-retronet-dialog="delete"][open]');return !!d&&d.textContent.includes('New Board');}`)
	run(`()=>fixtureDialog('delete').querySelector('button[type="submit"]').click()`)
	wait("delete stores the remaining entries", `async()=>{const doc=fixturePuts().at(-1);return !fixtureDialog('delete')&&doc.entries.length===2&&!doc.entries.some(e=>e.name==='New Board')&&!(await fixtureText()).includes('New Board');}`)
	run(`async()=>{await fixtureInput('0');await fixtureInput('1');await fixtureInput('e');}`)
	wait("E on a catalog entry explains why", `async()=>(await fixtureSelectedLine()).startsWith('>01')&&(await fixtureText()).includes(t('desktop.terminal_retronet_not_own'))`)
	check("catalog entries cannot be edited", `()=>!fixtureDialog('edit')&&!document.querySelector('dialog[data-terminal-retronet-dialog]')`)

	// 14. Local shell 00 (Code Studio socket), Ctrl+] back, non-admin directory.
	run(`async()=>{window.fixtureCanEdit=false;await fixtureInput('0');await fixtureInput('0');await fixtureInput('\r');}`)
	wait("00 opens today's shell socket", `()=>{const s=fixtureSocket(),root=fixtureRoot(),btn=root.querySelector('[data-terminal-retronet-action]');return new URL(s.url).pathname==='/api/code-studio/terminal'&&s.binaryType==='arraybuffer'&&s.readyState===1&&root.dataset.terminalMode==='shell'&&root.dataset.terminalState==='desktop.terminal_running'&&fixtureTerm().options.convertEol===true&&fixtureLive().length===1&&!btn.hidden&&btn.textContent===t('desktop.terminal_retronet_directory');}`)
	run(`async()=>{await fixtureInput('ls');}`)
	check("shell keystrokes stay text frames", `()=>fixtureSocket().sent.includes('ls')`)
	run(`async()=>{window.fixtureShell=fixtureSocket();await fixtureInput('\x1d');}`)
	wait("Ctrl+] leaves the shell for the directory", `async()=>fixtureShell.readyState===3&&!fixtureShell.sent.includes('\x1d')&&fixtureRoot().dataset.terminalMode==='directory'&&(await fixtureText()).includes('Telehack')&&fixtureLive().length===0`)
	// Keys are handled in order: once the later jump to 02 shows, N has been processed.
	run(`async()=>{await fixtureInput('n');await fixtureInput('0');await fixtureInput('2');}`)
	wait("the keys after N are handled", `async()=>(await fixtureSelectedLine()).startsWith('>02')`)
	check("non-admins get no entry editor", `async()=>!fixtureDialog('new')&&!document.querySelector('dialog[data-terminal-retronet-dialog]')&&
		!(await fixtureText()).includes(t('desktop.terminal_retronet_help_admin').slice(0,12))`)

	// 15. Modem sounds follow the key-click rules.
	run(`async()=>{localStorage.setItem('aurago.desktop.terminal.audioMuted','0');await fixtureInput('0');await fixtureInput('1');await fixtureInput('\r');}`)
	if full {
		wait("an unmuted retro dial synthesizes tones", `()=>fixtureOscillators>0`)
	} else {
		// Reduced motion prints the whole transcript at once and finishes the dial: no tone can follow.
		wait("reduced motion prints the dial at once", `async()=>(await fixtureText()).includes('ATDT telehack.com')`)
		check("reduced motion never plays modem tones", `()=>fixtureOscillators===0`)
	}
	run(`async()=>{localStorage.setItem('aurago.desktop.terminal.audioMuted','1');await fixtureInput('\x1d');}`)
	wait("hang-up while dialing", `async()=>(await fixtureText()).includes('NO CARRIER')&&fixtureLive().length===0&&fixtureRoot().dataset.terminalMode==='result'`)
	run(`async()=>{await fixtureDismiss('q');}`)
	wait("back in the directory after the audio check", backInDirectory)

	// 16. Policy: unrelated events change nothing; switching the toggle off falls back to the shell.
	run(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{serial_browser_enabled:false,serial_host_enabled:false}}))`)
	check("a policy event without retronet_enabled changes nothing", `()=>fixtureRoot().dataset.terminalMode==='directory'&&fixtureLive().length===0`)
	run(`()=>{terminalTest.state.bootstrap.retronet_enabled=false;document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{retronet_enabled:false}}));}`)
	wait("toggle off switches the directory to the shell", `()=>{const root=fixtureRoot(),s=fixtureSocket();return root.dataset.terminalMode==='shell'&&new URL(s.url).pathname==='/api/code-studio/terminal'&&s.readyState===1&&root.querySelector('[data-terminal-retronet-action]').hidden&&root.querySelector('[data-terminal-baud-label]').hidden&&fixtureLive().length===1;}`)
	run(`async()=>{await fixtureInput('\x1d');}`)
	check("with Retro-Net off Ctrl+] reaches the shell", `()=>fixtureSocket().sent.includes('\x1d')&&fixtureRoot().dataset.terminalMode==='shell'`)

	// 17. A new window with the toggle off behaves exactly as today.
	run(`async()=>{window.fixtureMark=fixtureSockets.length;window.fixtureCallMark=fixtureCalls.length;await fixtureBoot(false);}`)
	wait("toggle off opens the Code Studio shell", `()=>{const created=fixtureSockets.slice(fixtureMark);
		return created.length>=1&&created[0].readyState===1&&new URL(created[0].url).pathname==='/api/code-studio/terminal'&&
			fixtureRoot().dataset.terminalMode==='shell'&&fixtureRoot().dataset.terminalState==='desktop.terminal_running';}`)
	check("toggle off opens nothing else", `()=>{const root=fixtureRoot();
		return fixtureSockets.length===fixtureMark+1&&fixtureLive().length===1&&
			!fixtureCalls.slice(fixtureCallMark).some(c=>c.path.startsWith('/api/desktop/retronet'))&&
			root.querySelector('[data-terminal-retronet-action]').hidden&&root.querySelector('[data-terminal-baud-label]').hidden;}`)

	// 18. "Open Terminal Here" goes straight to the shell; the toolbar opens the directory; double-click dials.
	run(`async()=>{window.fixtureMark=fixtureSockets.length;window.fixtureCallMark=fixtureCalls.length;await fixtureBoot(true,{path:'/workspace/projects'});}`)
	wait("path context opens the shell directly", `()=>{const created=fixtureSockets.slice(fixtureMark),root=fixtureRoot();
		return created.length>=1&&created[0].readyState===1&&new URL(created[0].url).pathname==='/api/code-studio/terminal'&&
			root.dataset.terminalMode==='shell'&&root.dataset.terminalState==='desktop.terminal_running';}`)
	check("path context opens no directory", `()=>{const btn=fixtureRoot().querySelector('[data-terminal-retronet-action]');
		return fixtureSockets.length===fixtureMark+1&&fixtureLive().length===1&&
			!fixtureCalls.slice(fixtureCallMark).some(c=>c.path==='/api/desktop/retronet/directory')&&
			!btn.hidden&&btn.textContent===t('desktop.terminal_retronet_directory');}`)
	run(`()=>fixtureRoot().querySelector('[data-terminal-retronet-action]').click()`)
	wait("the toolbar opens the directory", backInDirectory)
	run(`async()=>{await fixtureFlush();const term=fixtureTerm(),row=fixtureLines().findIndex(l=>l.includes('Telehack')),screen=term.element.querySelector('.xterm-screen'),r=screen.getBoundingClientRect();screen.dispatchEvent(new MouseEvent('dblclick',{bubbles:true,clientX:r.left+20,clientY:r.top+(row+0.5)*r.height/term.rows}));}`)
	wait("a double-click dials", `()=>new URL(fixtureSocket().url).searchParams.get('entry')==='telehack'&&fixtureLive().length===1&&fixtureRoot().dataset.terminalMode==='dialing'`)
	run(`async()=>{await fixtureInput('\x1d');await fixtureDismiss('q');}`)
	wait("back in the directory after the double-click dial", backInDirectory)

	// 18b. The directory answers HTTP 403 (Retro-Net switched off server-side): today's shell, nothing of the
	// directory left on the fresh screen, the cursor visible again, exactly one socket.
	run(`async()=>{window.fixtureDirectoryStatus=403;window.fixtureMark=fixtureSockets.length;window.fixtureCallMark=fixtureCalls.length;await fixtureBoot(true);}`)
	wait("HTTP 403 falls back to today's shell", `async()=>{const created=fixtureSockets.slice(fixtureMark),root=fixtureRoot(),term=fixtureTerm(),text=await fixtureText();return fixtureCalls.slice(fixtureCallMark).some(c=>c.path==='/api/desktop/retronet/directory'&&c.failed)&&created.length===1&&new URL(created[0].url).pathname==='/api/code-studio/terminal'&&created[0].readyState===1&&fixtureLive().length===1&&root.dataset.terminalMode==='shell'&&root.dataset.terminalState==='desktop.terminal_running'&&term.options.convertEol===true&&!text.trim()&&root.querySelector('[data-terminal-retronet-action]').hidden&&root.querySelector('[data-terminal-baud-label]').hidden;}`)
	check("the shell screen is the normal buffer with a visible cursor and auto-wrap", `()=>{const term=fixtureTerm();return term.buffer.active.type==='normal'&&term._core.coreService.isCursorHidden===false&&term.modes.wraparoundMode===true&&!fixtureLines().join('').trim();}`)
	run(`async()=>{window.fixtureDirectoryStatus=0;await fixtureInput('ls');}`)
	check("the fallback shell takes keys and Ctrl+] stays with it", `async()=>{await fixtureInput('\x1d');const s=fixtureSocket();return s.sent.includes('ls')&&s.sent.includes('\x1d')&&fixtureRoot().dataset.terminalMode==='shell'&&fixtureLive().length===1;}`)

	// 19. Dispose closes everything; the one-socket invariant held throughout.
	run(`()=>TerminalApp.dispose()`)
	check("dispose closes every socket", `()=>fixtureLive().length===0`)
	check("no browser errors and never two live sockets", `()=>fixtureErrors.length===0`)
}

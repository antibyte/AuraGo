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

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// The fixture uses the real desktop shell, menus and window lifecycle. Only
// bootstrap, catalog and Audio are local fakes; no external streams are opened.
const radioReceiverFixture = `
window.fixtureErrors=[];window.addEventListener('error',e=>fixtureErrors.push(e.message));
window.addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
window.fixtureAudio=[];
window.Audio=class extends EventTarget {
 constructor(){super();this.paused=true;this.attrs={};fixtureAudio.push(this);}
 set src(v){this.attrs.src=v;}get src(){return this.attrs.src||'';}
 getAttribute(k){return this.attrs[k]||null;}removeAttribute(k){delete this.attrs[k];}
 async play(){if(window.rejectAudio)throw Error('fixture playback failure');this.paused=false;this.dispatchEvent(new Event('playing'));}
 pause(){this.paused=true;this.dispatchEvent(new Event('pause'));}load(){}
};
window.fixtureStations=['Yes.fm (Toledo,oh) 8…','Радио Пассаж','哈尔滨音乐广播','Point-Radio','FM CIUDAD 94.7','Soundsystem'].map((name,i)=>({stationuuid:'station-'+i,name,url_resolved:'https://radio.invalid/'+i,codec:i>2?'MP3':'AAC',bitrate:[32,64,50,160,48,128][i],countrycode:['US','RU','CN','GR','AR','DE'][i],clickcount:12000-i*250,favicon:''}));
window.fixtureRequests=[];window.heldCatalog=[];window.heldStreams=[];
const nativeFetch=window.fetch.bind(window);
window.fetch=async (url,opts={})=>{
 const path=String(url); if(!path.startsWith('/api/'))return nativeFetch(url,opts);
 fixtureRequests.push({path,signal:opts.signal});
 if(path.startsWith('/api/radio-browser')) {
  if(window.holdCatalog && !path.includes('/json/url/'))return new Promise(resolve=>heldCatalog.push(()=>resolve(new Response(JSON.stringify(fixtureStations)))));
  if(window.holdStreams && path.includes('/json/url/'))return new Promise(resolve=>heldStreams.push(()=>resolve(new Response(JSON.stringify({url:'https://radio.invalid/delayed'})))));
  if(window.rejectCatalog && !path.includes('/json/url/'))return new Response('{}',{status:503});
  const data=path.includes('/json/url/')?{url:'https://radio.invalid/live'}:path.includes('name=none')?[]:fixtureStations;
  return new Response(JSON.stringify(data));
 }
 return new Response('{}');
};
window.fixtureReady=(async()=>{
 const words=await (await nativeFetch('/lang/desktop/de.json')).json();
 window.i18n={t:key=>words[key]||key};
 window.t=key=>words[key]||key;
 localStorage.removeItem('aurago.radio.favorites.v1');
 radioTest.state.bootstrap={enabled:true,builtin_apps:[{id:'radio',name:'Radio',icon:'radio'},{id:'calculator',name:'Calculator',icon:'calculator'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};
 document.body.dataset.theme='standard';document.body.dataset.animations='false';
 document.getElementById('vd-disabled').hidden=true;
 await radioTest.loadIconManifest();
 radioTest.openApp('radio');
})();
`

func TestDesktopRadioBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/radio.css"><style>body{margin:0} .vd-shell{height:100vh}</style></head>`, 1)
	html = strings.Replace(html, "</body>", `<script src="/radio-shell.js"></script><script src="/js/desktop/apps/radio.js"></script><script src="/js/desktop/apps/calculator.js"></script><script src="/radio-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.radioTest={state,openApp,loadIconManifest,closeWindow,minimizeWindow,focusWindow,toggleMaximizeWindow};})();`
	artifacts := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if artifacts != "" {
		if err := os.MkdirAll(artifacts, 0755); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]string{"radio-preview.html": html, "radio-shell.js": shell, "radio-fixture.js": radioReceiverFixture} {
			if err := os.WriteFile(filepath.Join(artifacts, name), []byte(value), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/radio-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/radio-fixture.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, radioReceiverFixture)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1480, 1100, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{try{await fixtureReady;}catch(e){fixtureErrors.push(String(e));}}`)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
	waitForJSBool(t, page, `() => document.querySelectorAll('.radio-card:not(.radio-skeleton)').length===6`)
	check := func(name, js string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatalf("%s; browser errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	check("initial controls", `()=>!document.querySelector('[data-category="favorites"]').hidden && document.querySelector('[data-action="toggle"]').disabled && fixtureAudio[0].paused`)
	// Match the supplied image's favorite state for visual comparison.
	page.MustEval(`()=>document.querySelectorAll('.radio-heart').forEach(button=>button.click())`)
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{1424, 960, 390} {
			page.MustSetViewport(max(width+30, 440), 1100, 1, false)
			page.MustEval(`(theme,width)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode='light';const w=document.querySelector('[data-app-id="radio"]');w.style.width=width+'px';w.style.height='990px';w.style.left='8px';w.style.top='8px';}`, theme, width)
			check("responsive grid", fmt.Sprintf(`()=>getComputedStyle(document.querySelector('.radio-grid')).gridTemplateColumns.split(' ').length===%d`, map[int]int{1424: 3, 960: 2, 390: 1}[width]))
			check("player visible", `()=>{const w=document.querySelector('[data-app-id="radio"]').getBoundingClientRect(),p=document.querySelector('.radio-player').getBoundingClientRect();return p.bottom<=w.bottom && p.top>w.top && p.width<w.width;}`)
			check("title controls on right", `()=>{const title=document.querySelector('.vd-window-title').getBoundingClientRect(),actions=document.querySelector('.vd-window-actions').getBoundingClientRect();return actions.left>title.left && actions.right<=document.querySelector('.vd-window').getBoundingClientRect().right;}`)
			check("no horizontal overflow", `()=>{const a=document.querySelector('.radio-app');return a.scrollWidth<=a.clientWidth+1;}`)
			if artifacts != "" {
				if err := os.WriteFile(filepath.Join(artifacts, fmt.Sprintf("radio-%s-%d.png", theme, width)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustSetViewport(1480, 1100, 1, false)
	page.MustEval(`()=>document.querySelectorAll('.radio-heart').forEach(button=>button.click())`)
	for _, width := range []int{960, 390} {
		page.MustEval(`width=>{const w=document.querySelector('.vd-window');w.style.width=width+'px';w.style.height='600px';}`, width)
		check("short window usable", `()=>{const list=document.querySelector('.radio-scroll').getBoundingClientRect(),player=document.querySelector('.radio-player').getBoundingClientRect(),w=document.querySelector('.vd-window').getBoundingClientRect();return list.height>=120 && player.bottom<=w.bottom;}`)
		if artifacts != "" {
			if err := os.WriteFile(filepath.Join(artifacts, fmt.Sprintf("radio-short-%d.png", width)), page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustEval(`()=>{document.body.dataset.theme='standard';document.querySelector('.vd-window').style.width='1320px';}`)
	page.MustEval(`()=>document.querySelector('.vd-window').style.height='920px'`)
	page.MustElement("[data-category='favorites']").MustClick()
	check("empty favorites", `()=>!!document.querySelector('.radio-empty') && document.querySelector('[data-tune]').disabled`)
	page.MustElement("[data-category='pop']").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.radio-card').length===6`)
	page.MustElement(".radio-heart").MustClick()
	check("favorite does not play", `()=>fixtureAudio[0].paused && JSON.parse(localStorage.getItem('aurago.radio.favorites.v1')).length===1`)
	page.MustElement(".radio-heart").MustType(input.Enter)
	check("keyboard favorite isolated", `()=>fixtureAudio[0].paused && JSON.parse(localStorage.getItem('aurago.radio.favorites.v1')).length===0`)
	page.MustEval(`()=>{const k=document.querySelector('[data-tune]');for(let i=0;i<12;i++)k.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowLeft'}));}`)
	check("tuning lower bound", `()=>document.querySelector('.radio-card').classList.contains('selected')`)
	knob := page.MustEval(`()=>{const r=document.querySelector('[data-tune]').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,r:r.width*.35};}`)
	move := func(x, y float64) {
		t.Helper()
		if err := page.Mouse.MoveTo(proto.Point{X: x, Y: y}); err != nil {
			t.Fatal(err)
		}
	}
	x, y, r := knob.Get("x").Num(), knob.Get("y").Num(), knob.Get("r").Num()
	move(x+r, y)
	if err := page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	move(x+r*.87, y+r*.5)
	if err := page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	check("pointer rotation selects without playing", `()=>document.querySelectorAll('.radio-card')[1].classList.contains('selected') && fixtureAudio[0].paused`)
	page.MustEval(`()=>document.querySelector('[data-tune]').dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowLeft'}))`)
	page.MustEval(`()=>{const k=document.querySelector('[data-tune]');k.focus();k.dispatchEvent(new WheelEvent('wheel',{deltaY:100,cancelable:true}));}`)
	check("focused wheel tunes", `()=>document.querySelectorAll('.radio-card')[1].classList.contains('selected') && fixtureAudio[0].paused`)
	page.MustEval(`()=>{const k=document.querySelector('[data-tune]');k.blur();k.dispatchEvent(new WheelEvent('wheel',{deltaY:100}));k.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowLeft'}));}`)
	page.MustEval(`()=>{const k=document.querySelector('[data-tune]');k.focus();k.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));}`)
	check("tuning selection only", `()=>document.querySelectorAll('.radio-card')[1].classList.contains('selected') && fixtureAudio[0].paused`)
	page.MustElement("[data-tune]").MustClick()
	waitForJSBool(t, page, `()=>!fixtureAudio[0].paused`)
	check("tuner plays selection", `()=>document.querySelector('[data-now-title]').textContent===fixtureStations[1].name`)
	page.MustEval(`()=>{for(let i=0;i<12;i++)document.querySelector('[data-tune]').dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight'}));}`)
	check("tuning upper bound", `()=>document.querySelectorAll('.radio-card')[5].classList.contains('selected') && document.querySelector('[data-now-title]').textContent===fixtureStations[1].name`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	check("reduced motion", `()=>[...document.querySelectorAll('.radio-level i')].every(bar=>getComputedStyle(bar).animationName==='none')`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{window.holdStreams=true;document.querySelectorAll('[data-play-station]')[0].click();}`)
	waitForJSBool(t, page, `()=>heldStreams.length===1`)
	page.MustEval(`()=>document.querySelectorAll('[data-play-station]')[2].click()`)
	waitForJSBool(t, page, `()=>heldStreams.length===2`)
	page.MustEval(`()=>{heldStreams.pop()();window.holdStreams=false;}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-now-title]').textContent===fixtureStations[2].name && !fixtureAudio[0].paused`)
	page.MustEval(`()=>heldStreams.shift()()`)
	check("stale stream discarded", `()=>document.querySelector('[data-now-title]').textContent===fixtureStations[2].name && !fixtureAudio[0].paused`)
	page.MustElement("[data-action='toggle']").MustClick()
	check("pause", `()=>fixtureAudio[0].paused && !document.querySelector('.radio-app').classList.contains('playing')`)
	page.MustElement("[data-action='toggle']").MustClick()
	page.MustElement("[data-action='mute']").MustClick()
	page.MustEval(`()=>{const v=document.querySelector('[data-volume]');v.value='31';v.dispatchEvent(new Event('input'));}`)
	check("volume/mute", `()=>fixtureAudio[0].volume===.31 && fixtureAudio[0].muted`)
	page.MustElement("[data-action='stop']").MustClick()
	check("stop", `()=>fixtureAudio[0].paused && !fixtureAudio[0].src`)
	page.MustElement("[data-action='toggle']").MustClick()
	waitForJSBool(t, page, `()=>!fixtureAudio[0].paused && !!fixtureAudio[0].src`)
	page.MustEval(`()=>{window.holdCatalog=true;const s=document.querySelector('[data-search]');s.value='older';s.dispatchEvent(new Event('input'));}`)
	waitForJSBool(t, page, `()=>heldCatalog.length===1`)
	page.MustEval(`()=>{window.holdCatalog=false;const s=document.querySelector('[data-search]');s.value='none';s.dispatchEvent(new Event('input'));}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('.radio-empty')`)
	page.MustEval(`()=>heldCatalog.shift()()`)
	check("stale search discarded", `()=>!!document.querySelector('.radio-empty')`)
	page.MustEval(`()=>{window.rejectCatalog=true;document.querySelector('[data-category="rock"]').click();}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-status]').hidden && !document.querySelector('.radio-skeleton')`)
	page.MustEval(`()=>{window.rejectCatalog=false;document.querySelector('[data-category="pop"]').click();}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.radio-card').length===6`)
	page.MustEval(`()=>{window.rejectAudio=true;document.querySelector('[data-play-station]').click();}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-toast]').hidden && !document.querySelector('.radio-app').classList.contains('playing')`)
	page.MustEval(`()=>{window.rejectAudio=false;window.holdStreams=true;document.querySelector('[data-play-station]').click();}`)
	waitForJSBool(t, page, `()=>heldStreams.length===1`)
	page.MustElement("[data-action='maximize']").MustClick()
	check("maximize", `()=>document.querySelector('.vd-window').classList.contains('maximized')`)
	page.MustElement("[data-action='maximize']").MustClick()
	page.MustEval(`()=>{const w=document.querySelector('.vd-window');w.style.width='960px';w.style.height='700px';w.style.top='90px';w.style.left='100px';}`)
	move(320, 112)
	if err := page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	move(350, 132)
	if err := page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	check("window drag", `()=>{const w=document.querySelector('.vd-window');return parseInt(w.style.left)>100 && parseInt(w.style.top)>90;}`)
	edge := page.MustEval(`()=>{const r=document.querySelector('.vd-resize-se').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2};}`)
	move(edge.Get("x").Num(), edge.Get("y").Num())
	if err := page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	move(edge.Get("x").Num()+35, edge.Get("y").Num()+20)
	if err := page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	check("window resize", `()=>parseInt(document.querySelector('.vd-window').style.width)>960`)
	page.MustEval(`()=>{radioTest.openApp('calculator');}`)
	check("other app keeps its skin", `()=>{const radio=document.querySelector('[data-app-id="radio"]'),calc=document.querySelector('[data-app-id="calculator"]');return !!calc && getComputedStyle(calc).backgroundImage!==getComputedStyle(radio).backgroundImage && !calc.querySelector('.radio-app');}`)
	page.MustEval(`()=>radioTest.closeWindow([...radioTest.state.windows.values()].find(w=>w.appId==='calculator').id)`)
	page.MustElement("[data-action='minimize']").MustClick()
	page.MustEval(`()=>radioTest.focusWindow([...radioTest.state.windows.keys()][0])`)
	page.MustEval(`()=>{window.holdCatalog=true;const s=document.querySelector('[data-search]');s.value='closing';s.dispatchEvent(new Event('input'));}`)
	waitForJSBool(t, page, `()=>heldCatalog.length===1`)
	page.MustElement("[data-action='close']").MustClick()
	page.MustEval(`()=>{heldStreams.shift()();heldCatalog.shift()();}`)
	check("dispose prevents late playback", `()=>!document.querySelector('.radio-app') && fixtureAudio[0].paused && !fixtureAudio[0].src && fixtureRequests.filter(r=>r.path.includes('/json/url/')).at(-1).signal.aborted`)
	check("dispose aborts catalog", `()=>fixtureRequests.filter(r=>r.path.includes('name=closing')).at(-1).signal.aborted`)
	check("no JS errors", `()=>fixtureErrors.length===0`)
}

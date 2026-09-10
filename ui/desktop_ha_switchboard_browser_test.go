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

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

const haSwitchboardFixture = `
window.fixtureErrors=[];window.addEventListener('error',e=>fixtureErrors.push(e.message));window.addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
const nativeFetch=window.fetch.bind(window),key='ha_switchboard.board';
window.fixtureEntities=['Wohnbereich','Küche','Schlafzimmer','Werkstatt','Gartenlicht','Terrasse'].map((name,i)=>({entity_id:'switch.test_'+i,friendly_name:name,state:i%2?'off':'on'}));
window.fixtureEntities.push({entity_id:'switch.extra',friendly_name:'Steckdose Büro',state:'off'});
window.fixtureSettings=JSON.parse(sessionStorage.getItem('ha-fixture-settings')||'null')||{[key]:JSON.stringify({version:1,switches:fixtureEntities.slice(0,6).map(e=>({entity_id:e.entity_id}))})};
window.fixtureRequests=[];window.fixtureWrites=[];window.heldStates=[];window.heldWrites=[];window.ready=true;window.readonly=false;window.failed=false;window.saveFailed=false;window.holdStates=false;window.holdWrites=false;
const jsonResponse=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
function haSnapshot(all){const board=JSON.parse(fixtureSettings[key]);return {ready,readonly,board_readonly:false,can_on:!readonly,can_off:!readonly,board,entities:fixtureEntities.filter(e=>all||board.switches.some(b=>b.entity_id===e.entity_id)).map(e=>({...e})),checked_at:new Date().toISOString()};}
window.fetch=async(url,opts={})=>{
 const path=String(url);if(!path.startsWith('/api/'))return nativeFetch(url,opts);
 fixtureRequests.push({path,method:opts.method||'GET',signal:opts.signal});
 if(path==='/api/desktop/home-assistant/states'){const snap=haSnapshot(false);if(holdStates)return new Promise(resolve=>heldStates.push(()=>resolve(jsonResponse(snap))));return failed?jsonResponse({error:'ha_unavailable'},502):jsonResponse(snap);}
 if(path==='/api/desktop/home-assistant/entities')return failed?jsonResponse({error:'ha_unavailable'},502):jsonResponse(haSnapshot(true));
 if(path==='/api/desktop/home-assistant/switch'){
  const command=JSON.parse(opts.body);fixtureWrites.push(command);
  const finish=()=>{fixtureEntities.find(e=>e.entity_id===command.entity_id).state=command.state;if(window.lostWriteReply)throw Error('connection lost after dispatch');return jsonResponse({accepted:true});};
  if(holdWrites)return new Promise(resolve=>heldWrites.push(()=>resolve(finish())));
  return readonly?jsonResponse({error:'switch_not_allowed'},403):finish();
 }
 if(path==='/api/desktop/settings'){
  if(opts.method==='PUT'){if(saveFailed)return jsonResponse({error:'save failed'},500);const b=JSON.parse(opts.body);fixtureSettings[b.key]=b.value;sessionStorage.setItem('ha-fixture-settings',JSON.stringify(fixtureSettings));}
  return jsonResponse({settings:fixtureSettings});
 }
 return jsonResponse({});
};
window.fixtureReady=(async()=>{
 const words=await(await nativeFetch('/lang/desktop/de.json')).json();window.i18n={t:key=>words[key]||key};window.t=key=>words[key]||key;
 haTest.state.bootstrap={enabled:true,readonly:false,builtin_apps:[{id:'ha-switchboard',name:'HA Switchboard',icon:'ha-switchboard'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{...fixtureSettings,'appearance.theme':'standard','windows.restore_session':'false'}};
 document.body.dataset.theme='standard';document.body.dataset.animations='false';document.getElementById('vd-disabled').hidden=true;
 await haTest.loadIconManifest();haTest.openApp('ha-switchboard');
})();
`

func TestHASwitchboardBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	html = strings.Replace(html, "</body>", `<script src="/js/desktop/core/module-loader.js"></script><script src="/ha-shell.js"></script><script src="/ha-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("missing shell seam")
	}
	shell = shell[:cut] + `window.haTest={state,openApp,loadIconManifest,closeWindow,minimizeWindow,focusWindow,toggleMaximizeWindow,handleDesktopEvent};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/ha-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/ha-fixture.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, haSwitchboardFixture)
	})
	artifacts := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if artifacts != "" {
		if err := os.MkdirAll(artifacts, 0755); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]string{"ha-preview.html": html, "ha-shell.js": shell, "ha-fixture.js": haSwitchboardFixture} {
			if err := os.WriteFile(filepath.Join(artifacts, name), []byte(value), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(150 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-bay').length===6 && document.querySelector('.ha-board').classList.contains('ha-art-ready')`)
	check := func(name, js string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatalf("%s; errors=%s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	check("real initial state", `()=>document.querySelectorAll('.ha-switch[aria-checked=true]').length===3 && document.querySelector('[data-ha=total]').textContent==='3 / 6' && !document.querySelector('.ha-switch').disabled`)
	check("local assets loaded", `()=>performance.getEntriesByType('resource').some(e=>e.name.includes('ha-switchboard.js')) && performance.getEntriesByType('resource').some(e=>e.name.includes('ha-switchboard.css'))`)
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme.startsWith('fruity')?'fruity':'standard';document.body.dataset.fruityMode=theme.endsWith('dark')?'dark':'light';const w=document.querySelector('.vd-window');w.style.cssText+=';width:1300px;height:820px;left:20px;top:20px';}`, theme)
		check("six physical bays", `()=>getComputedStyle(document.querySelector('.ha-bays')).gridTemplateColumns.split(' ').length===6`)
		check("cabinet remains wooden", `()=>getComputedStyle(document.querySelector('.vd-window')).backgroundImage.includes('walnut.png')`)
		check("controls remain on the right", `()=>{const w=document.querySelector('.vd-window').getBoundingClientRect(),a=document.querySelector('.vd-window-actions').getBoundingClientRect();return a.left>w.left+w.width/2 && a.right<w.right;}`)
		if artifacts != "" {
			page.MustScreenshot(filepath.Join(artifacts, "switchboard-"+theme+".png"))
		}
		check("dial labels fit without overlap", `()=>{const label=document.querySelector('.ha-dial-label').getBBox(),marks=[...document.querySelectorAll('.ha-dial-mark')].map(e=>e.getBBox());return label.x>=20 && label.x+label.width<=180 && marks.every(m=>m.y+m.height<label.y);}`)
		for _, count := range []int{2, 6} {
			page.MustEval(`count=>{fixtureSettings[key]=JSON.stringify({version:1,switches:fixtureEntities.slice(0,count).map(e=>({entity_id:e.entity_id}))});document.dispatchEvent(new CustomEvent('aurago:ha-board-change',{detail:{raw:fixtureSettings[key]}}));}`, count)
			waitForJSBool(t, page, fmt.Sprintf(`()=>document.querySelectorAll('.ha-bay').length===%d`, count))
			for _, height := range []int{540, 620, 700} {
				page.MustEval(`height=>document.querySelector('.vd-window').style.height=height+'px'`, height)
				check(fmt.Sprintf("%s %d switches fit at %dpx", theme, count, height), `()=>{const deck=document.querySelector('.ha-deck'),r=deck.getBoundingClientRect();return deck.scrollHeight<=deck.clientHeight+1 && [...document.querySelectorAll('.ha-switch-status,.ha-channel,.ha-dial,.ha-meter-readout,.ha-indicators')].every(e=>{const b=e.getBoundingClientRect();return b.top>=r.top && b.bottom<=r.bottom+1;});}`)
				if artifacts != "" && theme == "standard" && height == 620 {
					page.MustScreenshot(filepath.Join(artifacts, fmt.Sprintf("switchboard-short-%d.png", count)))
				}
			}
		}
		page.MustEval(`()=>document.querySelector('.vd-window').style.height='820px'`)
	}
	page.MustEval(`()=>{const w=document.querySelector('.vd-window');w.style.width='360px';}`)
	check("narrow board", `()=>getComputedStyle(document.querySelector('.ha-bays')).gridTemplateColumns.split(' ').length===1 && document.querySelector('.ha-board').scrollWidth<=document.querySelector('.ha-board').clientWidth+1`)
	if artifacts != "" {
		page.MustScreenshot(filepath.Join(artifacts, "switchboard-narrow.png"))
	}
	page.MustEval(`()=>document.querySelector('.vd-window').style.width='1300px'`)
	page.MustElement(".vd-window-button[data-action=maximize]").MustClick()
	check("maximize works", `()=>document.querySelector('.vd-window').classList.contains('maximized')`)
	page.MustElement(".vd-window-button[data-action=maximize]").MustClick()
	check("restore works", `()=>!document.querySelector('.vd-window').classList.contains('maximized')`)
	page.MustEval(`()=>{const w=document.querySelector('.vd-window');w.style.cssText+=';width:960px;height:700px;left:100px;top:90px';}`)
	page.MustEval(`async()=>{await Promise.all(document.querySelector('.vd-window').getAnimations().map(a=>a.finished.catch(()=>{})));}`)
	for _, selector := range []string{".vd-window-titlebar", ".vd-resize-se"} {
		point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2};}`, selector)
		start := proto.Point{X: point.Get("x").Num(), Y: point.Get("y").Num()}
		if err := page.Mouse.MoveTo(start); err != nil {
			t.Fatal(err)
		}
		if err := page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
			t.Fatal(err)
		}
		if err := page.Mouse.MoveTo(proto.Point{X: start.X + 35, Y: start.Y + 20}); err != nil {
			t.Fatal(err)
		}
		if err := page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
			t.Fatal(err)
		}
	}
	check("drag and resize", `()=>{const w=document.querySelector('.vd-window');return parseInt(w.style.left)>100 && parseInt(w.style.top)>90 && parseInt(w.style.width)>960 && parseInt(w.style.height)>700;}`)
	page.MustEval(`()=>document.querySelector('.vd-window').style.cssText+=';width:1300px;height:820px;left:20px;top:20px'`)
	page.MustEval(`()=>{window.initialBoard=fixtureSettings[key];for(let i=7;i<12;i++)fixtureEntities.push({entity_id:'switch.more_'+i,friendly_name:'Reserve '+i,state:'off'});fixtureSettings[key]=JSON.stringify({version:1,switches:fixtureEntities.map(e=>({entity_id:e.entity_id}))});document.dispatchEvent(new CustomEvent('aurago:ha-board-change',{detail:{raw:fixtureSettings[key]}}));}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-bay').length===12`)
	if artifacts != "" {
		page.MustScreenshot(filepath.Join(artifacts, "switchboard-twelve.png"))
	}
	page.MustEval(`()=>{fixtureSettings[key]=JSON.stringify({version:1,switches:[{entity_id:'switch.test_0'}]});document.dispatchEvent(new CustomEvent('aurago:ha-board-change',{detail:{raw:fixtureSettings[key]}}));}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-bay').length===1`)
	if artifacts != "" {
		page.MustScreenshot(filepath.Join(artifacts, "switchboard-one.png"))
	}
	page.MustEval(`()=>{ready=false;fixtureSettings[key]=JSON.stringify({version:1,switches:[]});document.dispatchEvent(new CustomEvent('aurago:ha-board-change',{detail:{raw:fixtureSettings[key]}}));}`)
	waitForJSBool(t, page, `()=>document.querySelector('.ha-empty') && !document.querySelector('[data-ha=setup]').hidden`)
	check("setup path", `()=>document.querySelector('[data-ha=setup]').getAttribute('href')==='/config#home_assistant' && fixtureWrites.length===0`)
	if artifacts != "" {
		page.MustScreenshot(filepath.Join(artifacts, "switchboard-empty.png"))
	}
	page.MustEval(`()=>{ready=true;fixtureEntities=fixtureEntities.slice(0,7);fixtureSettings[key]=initialBoard;document.dispatchEvent(new CustomEvent('aurago:ha-board-change',{detail:{raw:initialBoard}}));}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-bay').length===6 && !document.querySelector('[data-ha=manage]').disabled`)
	page.MustElement("[data-ha=manage]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-result').length===7`)
	if artifacts != "" {
		page.MustScreenshot(filepath.Join(artifacts, "switchboard-selection.png"))
	}
	page.MustElement("[data-ha-search]").MustInput("Steckdose")
	check("search is read-only", `()=>document.querySelectorAll('.ha-result').length===1 && fixtureWrites.length===0`)
	page.MustElement("[data-ha-select='switch.extra']").MustClick()
	page.MustElement("[data-ha-label='switch.extra']").MustInput("Büro")
	page.MustEval(`()=>document.querySelector('[data-ha-up="6"]').click()`)
	page.MustEval(`()=>saveFailed=true`)
	page.MustElement("[data-ha-apply]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-ha-dialog-message]').textContent.includes('Speichern fehlgeschlagen')`)
	check("failed save retains selection", `()=>document.querySelectorAll('.ha-selected-row').length===7 && document.querySelector('[data-ha-label="switch.extra"]').value==='Büro'`)
	page.MustEval(`()=>saveFailed=false`)
	page.MustElement("[data-ha-apply]").MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('.ha-drawer') && document.querySelectorAll('.ha-bay').length===7`)
	check("order saved", `()=>JSON.parse(fixtureSettings[key]).switches[5].entity_id==='switch.extra' && fixtureWrites.length===0`)
	page.MustReload().MustWaitLoad()
	page.MustEval(`async()=>await fixtureReady`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-bay').length===7 && !document.querySelector('.ha-switch').disabled`)
	check("persistent labels", `()=>document.querySelector('[data-entity="switch.extra"] .ha-name').textContent==='Büro'`)
	// Hold an old poll and a write to prove neither acceptance nor a late snapshot confirms state.
	page.MustEval(`()=>{holdStates=true;document.querySelector('[data-ha=refresh]').click();}`)
	waitForJSBool(t, page, `()=>heldStates.length===1`)
	page.MustEval(`()=>{holdStates=false;holdWrites=true;document.querySelector('.ha-switch').click();}`)
	waitForJSBool(t, page, `()=>heldWrites.length===1`)
	check("pending state", `()=>document.querySelector('.ha-switch').disabled && document.querySelector('.ha-switch').getAttribute('aria-checked')==='true'`)
	page.MustEval(`()=>{document.querySelector('.ha-switch').click();holdWrites=false;heldWrites.shift()();}`)
	waitForJSBool(t, page, `()=>!document.querySelector('.ha-switch').disabled && document.querySelector('.ha-switch').getAttribute('aria-checked')==='false'`)
	page.MustEval(`async()=>{heldStates.shift()();await new Promise(r=>setTimeout(r,30));}`)
	check("old poll ignored and one explicit write", `()=>fixtureWrites.length===1 && fixtureWrites[0].state==='off' && document.querySelector('.ha-switch').getAttribute('aria-checked')==='false'`)
	page.MustElement(".ha-switch").MustFocus()
	page.Keyboard.MustType(input.Space)
	waitForJSBool(t, page, `()=>document.querySelector('.ha-switch').getAttribute('aria-checked')==='true' && !document.querySelector('.ha-switch').disabled`)
	page.MustEval(`()=>{window.lostWriteReply=true;document.querySelector('.ha-switch').click();}`)
	waitForJSBool(t, page, `()=>document.querySelector('.ha-switch').getAttribute('aria-checked')==='false' && document.querySelector('.ha-switch-status span').textContent==='Aus'`)
	check("lost reply reconciled without retry", `()=>fixtureWrites.length===3`)
	page.MustEval(`()=>window.lostWriteReply=false`)
	page.MustEval(`()=>{fixtureEntities[0].state='off';fixtureEntities[1].state='unavailable';fixtureEntities[2].friendly_name='<img src=x onerror=alert(1)>';readonly=true;document.querySelector('[data-ha=refresh]').click();}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-entity="switch.test_1"]').dataset.state==='unavailable'`)
	check("external states readonly and escaped text", `()=>document.querySelectorAll('.ha-switch:not(:disabled)').length===0 && document.querySelector('[data-entity="switch.test_2"] .ha-name').textContent.startsWith('<img') && !document.querySelector('.ha-name img')`)
	check("unavailable entity can be identified", `()=>document.querySelector('[data-entity="switch.test_1"] .ha-switch').title==='switch.test_1: Nicht verfügbar' && document.querySelector('[data-entity="switch.test_1"] .ha-name').title.includes('switch.test_1')`)
	page.MustEval(`()=>{readonly=false;failed=true;document.querySelector('[data-ha=refresh]').click();}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-ha=status]').textContent==='Verbindung nicht verfügbar'`)
	check("offline controls locked", `()=>document.querySelectorAll('.ha-switch:not(:disabled)').length===0`)
	page.MustEval(`()=>{failed=false;document.querySelector('[data-ha=refresh]').click();}`)
	waitForJSBool(t, page, `()=>!document.querySelector('.ha-switch').disabled`)
	page.MustElement("[data-ha=manage]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-result').length===7`)
	page.MustEval(`()=>{const changed={version:1,switches:[{entity_id:'switch.extra',label:'Other window'}]};fixtureSettings[key]=JSON.stringify(changed);document.dispatchEvent(new CustomEvent('aurago:ha-board-change',{detail:{raw:fixtureSettings[key]}}));}`)
	check("conflicting draft preserved", `()=>document.querySelectorAll('.ha-selected-row').length===7 && document.querySelector('[data-ha-apply]').disabled && !document.querySelector('[data-ha-reload]').hidden`)
	page.MustElement("[data-ha-reload]").MustClick()
	check("reload conflict", `()=>document.querySelectorAll('.ha-selected-row').length===1 && !document.querySelector('[data-ha-apply]').disabled`)
	page.MustElement("[data-ha-cancel]").MustClick()
	page.MustEval(`()=>{const w=[...haTest.state.windows.values()][0];w.element.classList.add('vd-space-hidden');window.pauseCount=fixtureRequests.length;}`)
	page.MustEval(`async()=>await new Promise(r=>setTimeout(r,5100))`)
	check("inactive space pauses polling", `()=>fixtureRequests.length===pauseCount`)
	page.MustEval(`()=>[...haTest.state.windows.values()][0].element.classList.remove('vd-space-hidden')`)
	waitForJSBool(t, page, `()=>fixtureRequests.length>pauseCount`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	check("reduced motion", `()=>getComputedStyle(document.querySelector('.ha-needle')).transitionDuration==='0s' && getComputedStyle(document.querySelector('.ha-pose')).transitionDuration==='0s'`)
	page.MustSetViewport(390, 844, 1, true)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>document.querySelector('.vd-window').style.cssText+=';width:380px;height:740px;left:5px;top:5px'`)
	check("touch layout", `()=>matchMedia('(pointer:coarse)').matches && document.querySelector('.ha-board').scrollWidth<=document.querySelector('.ha-board').clientWidth+1 && document.querySelector('.ha-switch').getBoundingClientRect().width>=44`)
	page.MustElement("[data-ha=manage]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ha-result').length===7`)
	check("touch dialog", `()=>getComputedStyle(document.querySelector('.ha-drawer-columns')).gridTemplateColumns.split(' ').length===1 && document.querySelector('[data-ha-cancel]').getBoundingClientRect().height>=44 && document.querySelector('.ha-drawer').scrollWidth<=document.querySelector('.ha-drawer').clientWidth+1`)
	page.MustElement("[data-ha-cancel]").MustClick()
	page.MustEval(`()=>{holdStates=true;document.querySelector('[data-ha=refresh]').click();}`)
	waitForJSBool(t, page, `()=>heldStates.length===1`)
	page.MustEval(`()=>{haTest.closeWindow([...haTest.state.windows.values()][0].id);window.closeCount=fixtureRequests.length;heldStates.shift()();}`)
	page.MustEval(`async()=>await new Promise(r=>setTimeout(r,5100))`)
	check("closed window releases polling", `()=>fixtureRequests.length===closeCount && !document.querySelector('.ha-board') && fixtureErrors.length===0`)
}

func TestHASwitchboardLocales(t *testing.T) {
	var english map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &english); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &words); err != nil {
			t.Fatal(err)
		}
		for key, value := range english {
			if strings.HasPrefix(key, "desktop.ha_") {
				if words[key] == "" {
					t.Errorf("%s missing %s", lang, key)
				}
				for _, token := range regexp.MustCompile(`\{\w+\}`).FindAllString(value, -1) {
					if !strings.Contains(words[key], token) {
						t.Errorf("%s %s missing %s", lang, key, token)
					}
				}
			}
		}
	}
}

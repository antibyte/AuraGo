package server

import (
	"aurago/internal/i18n"
	"aurago/ui"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

const bluetoothFixture = `window.btErrors=[];window.addEventListener('error',e=>btErrors.push(e.message));window.addEventListener('unhandledrejection',e=>btErrors.push(String(e.reason)));
window.btPosts=[];
window.btSnapshot={revision:1,present:true,adapter:{path:'/org/bluez/hci0',address:'00:11:22:33:44:55',name:'aurago-test',powered:true},devices:[
{address:'AA:BB:CC:DD:EE:01',alias:'Sony WH-1000XM5',paired:true,connected:true,trusted:true,audio:true,type:'headphones',battery:80},
{address:'AA:BB:CC:DD:EE:02',alias:'MX Keys',paired:true,connected:false,trusted:true,type:'keyboard'},
{address:'AA:BB:CC:DD:EE:03',name:'Pixel 9',alias:'Pixel 9',paired:false,type:'phone',rssi:-50},
{address:'AA:BB:CC:DD:EE:04',alias:'AA-BB-CC-DD-EE-04',paired:false,type:'other',rssi:-90}],
discovery:{active:false},discoverable:{active:false},interaction_id:'',status:{audio:{usable:true}}};
window.btInteraction={id:'0123456789abcdef0123456789abcdef',kind:'confirm_passkey',device_address:'AA:BB:CC:DD:EE:03',device_name:'Pixel 9',passkey:'482913',remaining_seconds:20};
const btJSON=body=>Promise.resolve(new Response(JSON.stringify(body),{headers:{'Content-Type':'application/json'}}));
const btFetch=fetch.bind(window);
window.fetch=(url,options)=>{const u=String(url);
if(u.startsWith('/api/bluetooth/status'))return btJSON(btSnapshot);
if(u.startsWith('/api/bluetooth/interactions/')){if(options&&options.method==='POST'){btPosts.push({url:u,body:JSON.parse(options.body)});return btJSON({status:'ok'})}return btJSON({status:'ok',interaction:btInteraction})}
if(u.startsWith('/api/bluetooth/')){btPosts.push({url:u,body:options&&options.body?JSON.parse(options.body):null});return btJSON({status:'accepted'})}
if(u.startsWith('/api/')&&!u.startsWith('/api/i18n?'))return btJSON({});
return btFetch(url,options)};
window.fixtureReady=(async()=>{btTest.state.bootstrap={enabled:true,builtin_apps:[{id:'bluetooth',name:'Bluetooth',icon:'bluetooth'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};document.body.dataset.theme='standard';document.body.dataset.animations='false';document.getElementById('vd-disabled').hidden=true;await btTest.loadIconManifest();btTest.openApp('bluetooth');})();`

func TestDesktopBluetoothBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	read := func(path string) string {
		b, err := ui.Content.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	i18n.Load(ui.Content, slog.Default())
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	scripts := `<script type="application/json" id="aurago-template-data">__DATA__</script><script src="/js/shared/template-data.js"></script><script>window._auragoSharedInitialized=true;</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/bt-shell.js"></script><script src="/bt-fixture.js"></script>`
	html = strings.Replace(html, "</body>", scripts+"</body>", 1)
	shell := read("js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell seam missing")
	}
	shell = shell[:cut] + `window.btTest={state,openApp,loadIconManifest,closeWindow,handleDesktopEvent};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, getI18NJSONForSections(normalizeLang(r.URL.Query().Get("lang")), strings.Split(r.URL.Query().Get("sections"), ",")...))
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := uiTemplateData("de", "desktop")
		fmt.Fprint(w, strings.Replace(html, "__DATA__", fmt.Sprint(data["TemplateDataJSON"]), 1))
	})
	mux.HandleFunc("/bt-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/bt-fixture.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, bluetoothFixture)
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage().Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1100, 820, 1, false)
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>await fixtureReady`)
	wait := func(js string) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if page.MustEval(js).Bool() {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("condition %s; errors %s", js, page.MustEval(`()=>JSON.stringify(btErrors)`).Str())
	}
	wait(`()=>[...document.querySelectorAll('.bt-device-name')].some(e=>e.textContent==='Sony WH-1000XM5')`)
	if page.MustEval(`()=>document.querySelector('.bt-app').textContent.includes('bluetooth.')`).Bool() {
		t.Fatal("untranslated labels")
	}
	if !page.MustEval(`()=>!!document.querySelector('[data-action="toggle-unnamed"]')&&!document.querySelector('.bt-app').textContent.includes('AA-BB-CC-DD-EE-04')`).Bool() {
		t.Fatal("unnamed devices are not collapsed")
	}
	page.MustEval(`()=>document.querySelector('[data-op="connect"][data-address="AA:BB:CC:DD:EE:02"]').click()`)
	wait(`()=>btPosts.some(p=>p.url==='/api/bluetooth/devices/action'&&p.body.operation==='connect'&&p.body.wait===false)`)

	page.MustEval(`async()=>{btSnapshot=Object.assign({},btSnapshot,{revision:2,interaction_id:btInteraction.id});await btTest.handleDesktopEvent({type:'bluetooth_changed',payload:{revision:2}})}`)
	wait(`()=>document.querySelector('.bt-passkey')?.textContent==='482 913'`)
	page.MustEval(`()=>document.querySelector('[data-action="answer"][data-accept="true"]').click()`)
	wait(`()=>btPosts.some(p=>p.url.endsWith('/interactions/'+btInteraction.id)&&p.body.accept===true)`)
	page.MustEval(`async()=>{btSnapshot=Object.assign({},btSnapshot,{revision:3,interaction_id:''});await btTest.handleDesktopEvent({type:'bluetooth_changed',payload:{revision:3}})}`)
	wait(`()=>document.querySelector('[data-bt="dialog"]').hidden`)

	if path := os.Getenv("AURAGO_BLUETOOTH_SCREENSHOT"); path != "" {
		_ = os.WriteFile(path, page.MustScreenshot(), 0o600)
		page.MustEval(`()=>{document.body.dataset.theme='fruity'}`)
		_ = os.WriteFile(strings.TrimSuffix(path, ".png")+"-fruity.png", page.MustScreenshot(), 0o600)
		page.MustEval(`()=>{document.body.dataset.theme='standard'}`)
	}

	page.MustEval(`async()=>{btSnapshot={revision:4,present:false,reason:'gone',devices:[],discovery:{active:false},discoverable:{active:false}};await btTest.handleDesktopEvent({type:'bluetooth_changed',payload:{revision:4}})}`)
	wait(`()=>document.querySelector('.bt-app').dataset.state==='unavailable'`)

	windowID := page.MustEval(`()=>[...btTest.state.windows.keys()][0]`).Str()
	page.MustEval(`async(id)=>await btTest.closeWindow(id)`, windowID)
	wait(`()=>!document.querySelector('.bt-app')`)
	page.MustEval(`async()=>await btTest.handleDesktopEvent({type:'bluetooth_interaction',payload:{id:btInteraction.id}})`)
	wait(`()=>document.body.textContent.includes('Kopplungsanfrage')`)
}

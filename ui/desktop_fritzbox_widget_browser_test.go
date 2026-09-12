package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in headless smoke test: renders the Fritz!Box widget against a fake
// overview endpoint, walks through all four pages, verifies text safety,
// translations, the disabled/error states and cleanup. Set
// AURAGO_BROWSER_ARTIFACT_DIR to keep per-page screenshots.
func TestDesktopFritzBoxWidgetBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	sysmon := readDesktopAssetText(t, "js/desktop/core/widget-sysmon-runtime.js")
	charts := readDesktopAssetText(t, "js/desktop/core/widget-fritzbox-charts.js")
	runtime := readDesktopAssetText(t, "js/desktop/core/widget-fritzbox-runtime.js")
	browser := newSmokeBrowser(t)
	// German renders on the dark token set, English on a light one so both
	// theme directions are exercised.
	themes := map[string]string{
		"de": "background:#14171c;--vd-text:#eee;--vd-text-muted:#aaa;--vd-bg:#20242a;--vd-theme-control-bg:#30343a;--vd-accent:#7abaff;--vd-accent-r:122;--vd-accent-g:186;--vd-accent-b:255;--vd-coral:#ff7a6e;--vd-amber:#f5b942;--vd-border:#555;--ds-color-fg-primary:#eee;--ds-color-fg-muted:#aaa;--ds-color-control-bg:#30343a",
		"en": "background:#eef1f6;--vd-text:#1a1e26;--vd-text-muted:#5c6470;--vd-bg:#ffffff;--vd-theme-control-bg:#e6e9ef;--vd-accent:#2f7de1;--vd-accent-r:47;--vd-accent-g:125;--vd-accent-b:225;--vd-coral:#d9534f;--vd-amber:#c98a00;--vd-border:#c8cdd6;--ds-color-fg-primary:#1a1e26;--ds-color-fg-muted:#5c6470;--ds-color-control-bg:#e6e9ef",
	}
	for _, lang := range []string{"de", "en"} {
		t.Run(lang, func(t *testing.T) {
			words := readDesktopAssetText(t, "lang/desktop/"+lang+".json")
			var dict map[string]string
			if err := json.Unmarshal([]byte(words), &dict); err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.Handle("/", http.FileServer(http.FS(Content)))
			mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `<html lang="`+lang+`"><head><link rel="stylesheet" href="/css/desktop-widgets.css"><link rel="stylesheet" href="/css/desktop-widget-fritzbox.css"><style>
body{margin:0;padding:16px;font-family:system-ui,sans-serif;--ds-radius-sm:6px;--ds-motion-ease-standard:ease;`+themes[lang]+`}
#card{width:320px;color:var(--vd-text)}</style></head><body class="desktop-body"><div id="card"></div><script>
const words=`+words+`;
const t=(k,v)=>{let s=words[k]||k;if(v){for(const [a,b] of Object.entries(v)){s=s.replaceAll('{{'+a+'}}',String(b)).replaceAll('{'+a+'}',String(b));}}return s;};
const esc=s=>String(s).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';');
const cleanups=[];const registerWidgetCleanup=f=>cleanups.push(f);
window.requests=[];window.mode='ok';
const monitorDown=Array.from({length:20},(_,i)=>Math.round(2e6+Math.sin(i/2)*1.5e6+i*1e5));
const monitorUp=Array.from({length:20},(_,i)=>Math.round(4e5+Math.cos(i/3)*2e5));
window.payload={
 capabilities:{system:true,connection:true,devices:true,telephony:true},
 system:{model:'FRITZ!Box 7590 AX',firmware:'7.83',uptime_seconds:865000},
 connection:{online:true,status:'Connected',uptime_seconds:96000,external_ipv4:'203.0.113.42',external_ipv6:'2001:db8::1',ipv6_prefix:'2001:db8::/56',access_type:'dsl',link_up:true,max_down_bps:250e6,max_up_bps:40e6,down_bps:monitorDown[19]*8,up_bps:monitorUp[19]*8,total_sent_bytes:52e9,total_received_bytes:734e9,monitor:{interval_seconds:5,down_bps:monitorDown.map(v=>v*8),up_bps:monitorUp.map(v=>v*8)}},
 devices:{total:27,active:9,lan_active:4,wlan_active:5,hosts_truncated:false,hosts:[{name:'<img src=x onerror=window.pwned=1>',ip:'192.168.178.20',active:true,interface:'lan'},{name:'Laptop',ip:'192.168.178.31',active:true,interface:'wlan'},{name:'NAS',ip:'192.168.178.2',active:true,interface:'lan'},{name:'Phone',ip:'192.168.178.44',active:true,interface:'wlan'},{name:'Old TV',ip:'192.168.178.60',active:false,interface:'lan'}],wlans:[{index:1,ssid:'Home',enabled:true,channel:6,band:'2.4',guest:false},{index:2,ssid:'Home 5G',enabled:true,channel:100,band:'5',guest:false},{index:3,ssid:'Guests',enabled:false,channel:0,band:'',guest:true}]},
 telephony:{missed_today:2,tam_available:true,tam_new:1,calls:[{type:'missed',date:'12.09.26 09:12',timestamp:new Date(Date.now()-3600000).toISOString(),name:'<b>Mom</b>',number:'+49301234567',called:'',duration:''},{type:'incoming',date:'12.09.26 08:40',timestamp:new Date(Date.now()-7200000).toISOString(),name:'',number:'+4930765432',called:'',duration:'0:12'},{type:'outgoing',date:'11.09.26 19:03',timestamp:new Date(Date.now()-86400000).toISOString(),name:'Office',number:'+4930111111',called:'',duration:'1:05'}]}
};
const api=async(url,opts)=>{requests.push(url);if(mode==='disabled')throw Error('fritzbox_disabled');if(mode==='fail')throw Error('offline');const sections=new URL(url,'http://x').searchParams.get('sections').split(',');const now=new Date().toISOString();const out={ready:true,capabilities:payload.capabilities,generated_at:now,errors:{},fetched_at:{},stale:{}};for(const s of sections){if(payload[s]){out[s]=payload[s];out.fetched_at[s]=now;}}return out;};
`+sysmon+`
`+charts+`
`+runtime+`
renderFritzBoxWidget(document.querySelector('#card'));</script></body></html>`)
			})
			server := httptest.NewServer(mux)
			defer server.Close()
			page := browser.MustPage(server.URL + "/fixture").Timeout(45 * time.Second)
			defer page.Close()
			page.MustSetViewport(360, 360, 2, false)
			page.MustWaitLoad()
			waitForJSBool(t, page, `()=>document.querySelector('.vd-fritz.is-ready')!==null`)

			artifactDir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
			shoot := func(name string) {
				if artifactDir == "" {
					return
				}
				time.Sleep(500 * time.Millisecond) // let the page slide transition settle
				if err := os.MkdirAll(artifactDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(artifactDir, "fritzbox-widget-"+name+"-"+lang+".png"), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Connection page: status, KPIs, gauges and facts are populated.
			shoot("connection")
			connection := page.MustEval(`()=>{
				const root=document.querySelector('.vd-fritz');
				const dots=root.querySelectorAll('.vd-fritz-dotbtn');
				return {
					dots: dots.length===4 && dots[0].classList.contains('is-active'),
					status: root.querySelector('[data-fritz=status-text]').textContent===t('desktop.widget_fritzbox_online'),
					down: root.querySelector('[data-fritz=down-number]').textContent!=='–',
					gauge: root.querySelector('[data-fritz=down-gauge] svg')!==null,
					ipv4: root.querySelector('[data-fritz=ipv4]').textContent==='203.0.113.42',
					access: root.querySelector('[data-fritz=access]').textContent===t('desktop.widget_fritzbox_access_dsl'),
					model: root.querySelector('[data-fritz=model]').textContent==='FRITZ!Box 7590 AX',
					width: root.scrollWidth<=320,
					scrollWidth: root.scrollWidth,
					overflow: [...root.querySelectorAll('*')].filter(el=>el.getBoundingClientRect().right>root.getBoundingClientRect().right+0.5).slice(0,6).map(el=>el.className+':'+Math.round(el.getBoundingClientRect().right))
				};
			}`).String()
			if strings.Contains(connection, "false") {
				t.Fatalf("connection page did not render the expected values: %s", connection)
			}
			shoot("connection")

			// Traffic page via the next arrow: chart SVG, legend and peaks.
			page.MustElement("[data-fritz=next]").MustClick()
			waitForJSBool(t, page, `()=>document.querySelector('[data-fritz-page=traffic]').classList.contains('is-active') && document.querySelector('[data-fritz=chart] svg')!==null`)
			if !page.MustEval(`()=>{
				const root=document.querySelector('.vd-fritz');
				return root.querySelectorAll('[data-fritz=chart] path').length>=2
					&& root.querySelector('[data-fritz=peak-down]').textContent!=='–'
					&& root.querySelector('[data-fritz=legend-span]').textContent.length>0
					&& localStorage.getItem('aurago.desktop.fritzbox.page')==='traffic'
					&& root.querySelector('[data-fritz=page-title]').textContent===t('desktop.widget_fritzbox_page_traffic');
			}`).Bool() {
				t.Fatal("traffic page did not render the chart")
			}
			shoot("traffic")

			// Horizontal wheel/trackpad gesture pages forward for mouse users
			// and is consumed so the browser does not navigate history.
			if !page.MustEval(`()=>{
				const viewport=document.querySelector('[data-fritz=viewport]');
				const ev=new WheelEvent('wheel',{deltaX:60,deltaY:4,bubbles:true,cancelable:true});
				viewport.dispatchEvent(ev);
				return ev.defaultPrevented;
			}`).Bool() {
				t.Fatal("horizontal wheel gesture was not consumed by the pager")
			}
			waitForJSBool(t, page, `()=>document.querySelector('[data-fritz-page=devices]').classList.contains('is-active')`)

			// Touch swipe back: the card captures the pointer, so move/up
			// events arrive on the document rather than on the viewport.
			page.MustEval(`()=>{
				const viewport=document.querySelector('[data-fritz=viewport]');
				const opts={pointerId:7,pointerType:'touch',isPrimary:true,button:0,buttons:1,bubbles:true,cancelable:true};
				viewport.dispatchEvent(new PointerEvent('pointerdown',Object.assign({clientX:80,clientY:120},opts)));
				document.body.dispatchEvent(new PointerEvent('pointermove',Object.assign({clientX:140,clientY:124},opts)));
				document.body.dispatchEvent(new PointerEvent('pointermove',Object.assign({clientX:230,clientY:126},opts)));
				document.body.dispatchEvent(new PointerEvent('pointerup',Object.assign({clientX:230,clientY:126,buttons:0},opts)));
			}`)
			waitForJSBool(t, page, `()=>document.querySelector('[data-fritz-page=traffic]').classList.contains('is-active') && !document.querySelector('[data-fritz=track]').classList.contains('is-dragging')`)

			// Regression: a mouse click inside the widget (whose pointerup is
			// swallowed by the capturing card) must not leave the pager in a
			// drag mode that follows every later mouse movement.
			if !page.MustEval(`()=>{
				const viewport=document.querySelector('[data-fritz=viewport]');
				const track=document.querySelector('[data-fritz=track]');
				const before=track.style.transform;
				const mouse={pointerId:1,pointerType:'mouse',isPrimary:true,button:0,bubbles:true,cancelable:true};
				viewport.dispatchEvent(new PointerEvent('pointerdown',Object.assign({clientX:100,clientY:120,buttons:1},mouse)));
				document.body.dispatchEvent(new PointerEvent('pointerup',Object.assign({clientX:100,clientY:120,buttons:0},mouse)));
				for (const x of [140,190,260,40]) {
					viewport.dispatchEvent(new PointerEvent('pointermove',Object.assign({clientX:x,clientY:120,buttons:0},mouse)));
				}
				return track.style.transform===before && !track.classList.contains('is-dragging')
					&& document.querySelector('[data-fritz-page=traffic]').classList.contains('is-active');
			}`).Bool() {
				t.Fatal("mouse movement after a click must not drag the pager")
			}

			// Devices page via keyboard: hostile host name stays text.
			page.MustEval(`()=>{const root=document.querySelector('.vd-fritz');root.focus();root.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));}`)
			waitForJSBool(t, page, `()=>document.querySelector('[data-fritz-page=devices]').classList.contains('is-active')`)
			if !page.MustEval(`()=>{
				const root=document.querySelector('.vd-fritz');
				const hosts=root.querySelectorAll('[data-fritz=hosts] .vd-fritz-host');
				return !window.pwned && hosts.length===5
					&& root.querySelector('[data-fritz=hosts] img')===null
					&& hosts[0].querySelector('.vd-fritz-host-name').textContent===payload.devices.hosts[0].name
					&& hosts[4].classList.contains('is-more')
					&& root.querySelector('[data-fritz=ring] svg')!==null
					&& root.querySelectorAll('[data-fritz=wlans] .vd-fritz-wlan').length===3
					&& root.querySelector('[data-fritz=wlans] .is-guest')!==null
					&& root.querySelector('[data-fritz=dev-active]').textContent==='9';
			}`).Bool() {
				t.Fatal("devices page failed value or text-safety checks")
			}
			shoot("devices")

			// Telephony page via dot: badges and call rows.
			page.MustEval(`()=>document.querySelectorAll('.vd-fritz-dotbtn')[3].click()`)
			waitForJSBool(t, page, `()=>document.querySelector('[data-fritz-page=telephony]').classList.contains('is-active')`)
			if !page.MustEval(`()=>{
				const root=document.querySelector('.vd-fritz');
				const calls=root.querySelectorAll('[data-fritz=calls] .vd-fritz-call');
				return calls.length===3 && root.querySelector('[data-fritz=calls] b')===null
					&& calls[0].querySelector('.vd-fritz-call-who').textContent==='<b>Mom</b>'
					&& calls[0].classList.contains('is-missed')
					&& root.querySelector('[data-fritz=missed]').textContent==='2'
					&& root.querySelector('[data-fritz=tam-badge]').classList.contains('has-new')
					&& root.querySelector('[data-fritz=next]').disabled;
			}`).Bool() {
				t.Fatal("telephony page failed value or text-safety checks")
			}
			shoot("telephony")

			// Height stays within the seeded widget bounds on every page.
			if h := page.MustEval(`()=>document.querySelector('.vd-fritz').getBoundingClientRect().height`).Num(); h > 300 {
				t.Fatalf("widget height %.0f exceeds the 300px seed", h)
			}

			// Error state keeps the last values and shows the stale banner.
			page.MustEval(`()=>{window.mode='fail';document.querySelector('[data-fritz=retry]').click();}`)
			waitForJSBool(t, page, `()=>!document.querySelector('[data-fritz=banner]').hidden && document.querySelector('.vd-fritz').classList.contains('is-stale')`)
			if !page.MustEval(`()=>document.querySelector('[data-fritz=banner-text]').textContent.includes(t('desktop.widget_fritzbox_error')) && document.querySelector('[data-fritz=missed]').textContent==='2'`).Bool() {
				t.Fatal("error banner or stale values missing")
			}
			shoot("error")

			// Disabled integration shows the hint and hides the pager.
			page.MustEval(`()=>{window.mode='disabled';document.querySelector('[data-fritz=retry]').click();}`)
			waitForJSBool(t, page, `()=>document.querySelector('.vd-fritz').classList.contains('is-disabled')`)
			if got := page.MustElement("[data-fritz=notice]").MustText(); got != dict["desktop.widget_fritzbox_disabled_hint"] {
				t.Fatalf("disabled hint: %q", got)
			}
			if !page.MustEval(`()=>document.querySelector('[data-fritz=dots]').hidden && document.querySelector('[data-fritz=banner]').hidden && document.querySelector('[data-fritz=page-title]').textContent===''`).Bool() {
				t.Fatal("disabled state must hide pager, banner and page title")
			}
			shoot("disabled")

			// Cleanup stops polling.
			page.MustEval(`()=>{cleanups.forEach(f=>f());window.requestsAfterCleanup=requests.length;}`)
			time.Sleep(6 * time.Second)
			if !page.MustEval(`()=>requests.length===window.requestsAfterCleanup`).Bool() {
				t.Fatal("widget kept polling after cleanup")
			}
			if text := page.MustElement("#card").MustText(); strings.Contains(text, "desktop.widget_fritzbox_") || strings.Contains(text, "desktop.") {
				t.Fatalf("untranslated widget text: %q", text)
			}
		})
	}
}

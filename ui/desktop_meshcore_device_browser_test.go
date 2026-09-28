package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDesktopMeshCoreDeviceBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><link rel="stylesheet" href="/css/desktop-base.css"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-meshcore.css"><style>body{margin:0}#host{width:1080px;height:720px;max-width:100vw}</style></head><body class="desktop-body" data-theme="standard" data-fruity-mode="light"><div id="host"></div><script src="/js/desktop/apps/meshcore-device.js"></script><script src="/js/desktop/apps/meshcore.js"></script></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	browser := newSmokeBrowser(t)
	page := browser.MustPage(srv.URL + "/fixture").Timeout(40 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustSetViewport(1100, 760, 1, false)
	page.MustEval(`async()=>{
 window.translations=await(await fetch('/lang/desktop/de.json')).json();window.calls=[];window.identity='a'.repeat(64);window.target='b'.repeat(64);window.conv='c'.repeat(64);window.rev=1;window.app={history_days:90,history_messages:10000,allow_device_settings:true,allow_remote_diagnostics:true,settings_revision:'app1'};
 window.values={name:'Base',latitude:51,longitude:7,advert_location_policy:0,manual_add_contacts:1,telemetry_base:1,telemetry_location:0,telemetry_environment:0,auto_add_mask:2,auto_add_max_hops:0,frequency_khz:869525,bandwidth_hz:250000,spreading_factor:11,coding_rate:5,tx_power_dbm:22,multi_acks:0,repeat:false,path_hash_mode:1};
 window.deviceStatus=()=>({state:window.offline?'disconnected':window.uncertain?'settings_uncertain':'connected',identity_key:identity,name:values.name,firmware:'1.14',snapshot_at:1800000000,channel_capacity:8,channels:[],contacts:[{key:target,name:'Bergstation',type:1,flags:3,last_advert_timestamp:1800000000,last_modified_timestamp:1800000000,advertised_position:{latitude:50,longitude:8},out_path:{route:'unknown',hops:null}}],device:{protocol_version:10,contact_capacity:100,manufacturer:'Test',build_date:'2026-09-01'},radio:{configured_position:{latitude:values.latitude,longitude:values.longitude},frequency_khz:values.frequency_khz,bandwidth_hz:values.bandwidth_hz,spreading_factor:11,coding_rate_denominator:5,tx_power_dbm:22,max_tx_power_dbm:30,multi_acks:0}});
 window.device=()=>({status:deviceStatus(),values:{...values},revision:String(rev),allow_device_settings:app.allow_device_settings,allow_remote_diagnostics:app.allow_remote_diagnostics,clock:1800000000,features:Object.fromEntries(['identity','radio','clock','other','auto_add','auto_add_max_hops','repeat','path_hash','multi_ack'].map(k=>[k,'available'])),local:{at:1800000000,groups:{storage:{at:1800000000,state:'available',values:{battery_mv:3700,storage_used_kb:20,storage_total_kb:100}},radio:{at:1800000000,state:'available',values:{last_rssi_dbm:-90,last_snr_db:-6.25}}}}});
 window.fetch=async(url,opt={})=>{
 const route=String(url).split('/messenger/')[1],body=opt.body?JSON.parse(opt.body):null;calls.push({route,body});let data={},code=200;
 if(route==='bootstrap')data={enabled:true,...app,trusted_nodes:[target],status:deviceStatus(),conversations:[{id:conv,identity_key:window.conversationIdentity||identity,target,kind:'direct',name:'Bergstation',type:1,active:true,can_send:!window.offline}]};
 if(route?.startsWith('messages?'))data={messages:[{id:'rx',seq:1,conversation_id:conv,direction:'incoming',origin:'radio',text:'Hallo',at:1800000000,parts:[],details:{sender_timestamp:1799999995,received_at:1800000000,text_type:0,reception:{snr_db:-6.25,path:{route:'direct',hops:null},frame_type:16,companion_frame_bytes:30},sender_contact:deviceStatus().contacts[0],receiver:deviceStatus()}}]};
 if(route==='device'){if(window.offline)throw Error('offline');data=device();}
 if(route==='device-settings'){if(body.revision!==String(rev)){code=409;data={error:'settings_conflict',device:device()};}else if(body.section==='reconcile'){window.uncertain=false;data=device();}else if(window.partial){values.name='Partial';rev++;window.uncertain=true;window.partial=false;code=409;data={error:'outcome_unknown',device:device()};}else{values={...body.values};rev++;data=device();}}
 if(route==='settings'){app={...app,...body,settings_revision:'app2'};data={ok:true};}
 if(route==='diagnostics'){window.polls=0;data=window.busy?{error:'busy'}:{id:'job1',state:'pending'};code=window.busy?409:202;}
 if(route==='diagnostics?id=job1'){polls++;data=polls<2?{state:'pending'}:{state:'completed',target,completed_at:1800000000,telemetry:[{channel:1,type:116,name:'voltage',unit:'V',values:[3.7]}]};}
 return new Response(JSON.stringify(data),{status:code,headers:{'Content-Type':'application/json'}});
 };
 window.timerCallbacks=[];const realSetInterval=window.setInterval;window.setInterval=(fn,ms)=>{timerCallbacks.push({fn,ms});return realSetInterval(fn,ms)};
 window.clockOffset=0;const realNow=Date.now;Date.now=()=>realNow()+clockOffset;
 MeshCoreApp.render(document.getElementById('host'),'device-test',{t:k=>translations[k]||k});
 }`)
	waitForJSBool(t, page, `()=>!!document.querySelector('.mc-conversation')`)
	page.MustElement(".mc-conversation").MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-message-id="rx"]')`)
	if !page.MustEval(`()=>document.querySelector('.mc-rx-badge').textContent==='-6.25 dB' && document.querySelectorAll('.mc-rx-badge').length===1`).Bool() {
		t.Fatal("unknown hops rendered as zero")
	}
	page.MustEval(`()=>[...document.querySelectorAll('[data-message-id="rx"] button')].find(b=>b.title===translations['desktop.meshcore_details']).click()`)
	if !page.MustEval(`()=>document.querySelector('.mc-dialog').textContent.includes(translations['desktop.meshcore_contact_snapshot']) && document.querySelector('.mc-dialog').textContent.includes('50°, 8°')`).Bool() {
		t.Fatal("reception details")
	}
	page.MustElement(".mc-dialog header button").MustClick()
	page.MustElement(`[data-mc="self"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('.mc-device-page').textContent.includes('3700 mV')`)
	if page.MustEval(`()=>calls.some(c=>c.route==='diagnostics')`).Bool() {
		t.Fatal("automatic remote probe")
	}
	page.MustEval(`()=>{window.deviceReads=calls.filter(c=>c.route==='device').length;clockOffset+=31000;Object.defineProperty(document,'hidden',{value:true,configurable:true});timerCallbacks.filter(t=>t.ms===30000).forEach(t=>t.fn());}`)
	if !page.MustEval(`()=>calls.filter(c=>c.route==='device').length===deviceReads`).Bool() {
		t.Fatal("hidden page polled device")
	}
	page.MustEval(`()=>{delete document.hidden;timerCallbacks.filter(t=>t.ms===30000).forEach(t=>t.fn());}`)
	waitForJSBool(t, page, `()=>calls.filter(c=>c.route==='device').length===deviceReads+1`)
	page.MustElement(`[data-mc="settings"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-mc-field="name"]')`)
	page.MustEval(`()=>{const el=document.querySelector('[data-mc-field="name"]');el.value='Entwurf';el.dispatchEvent(new Event('input',{bubbles:true}));values.name='External';rev++;document.querySelector('[data-mc="refresh"]').click();}`)
	waitForJSBool(t, page, `()=>calls.filter(c=>c.route==='device').length>=2 && !document.querySelector('[data-mc="refresh"]').disabled`)
	if !page.MustEval(`()=>document.querySelector('[data-mc-field="name"]').value==='Entwurf'`).Bool() {
		t.Fatal("refresh overwrote draft")
	}
	page.MustElement(`[data-mc-section="identity"] .mc-primary`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-mc-section="identity"] .mc-feedback').textContent.startsWith(translations['desktop.meshcore_error_settings_conflict'])`)
	page.MustEval(`()=>[...document.querySelectorAll('[data-mc-section="identity"] button')].find(b=>b.textContent===translations['desktop.meshcore_discard']).click()`)
	if !page.MustEval(`()=>document.querySelector('[data-mc-field="name"]').value==='External'`).Bool() {
		t.Fatal("discard did not load actual values")
	}
	page.MustEval(`()=>{const el=document.querySelector('[data-mc-field="name"]');el.value='Verified';el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`[data-mc-section="identity"] .mc-primary`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-mc-section="identity"] .mc-feedback').textContent===translations['desktop.meshcore_saved_verified']`)
	page.MustEval(`()=>{const el=document.querySelector('[data-mc-field="frequency_khz"]');el.value=869500;el.dispatchEvent(new Event('input',{bubbles:true}));window.beforeRadio=calls.filter(c=>c.route==='device-settings').length;}`)
	page.MustElement(`[data-mc-section="radio"] .mc-primary`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('.mc-dialog')`)
	if !page.MustEval(`()=>document.querySelector('.mc-dialog').textContent.includes('869525 → 869500') && calls.filter(c=>c.route==='device-settings').length===beforeRadio`).Bool() {
		t.Fatal("radio wrote before review")
	}
	page.MustElement(".mc-dialog .mc-primary").MustClick()
	waitForJSBool(t, page, `()=>values.frequency_khz===869500 && document.querySelector('[data-mc-section="radio"] .mc-feedback').dataset.feedback==='success' && !document.querySelector('[data-mc-section="radio"] fieldset').disabled`)
	page.MustEval(`()=>{window.partial=true;const el=document.querySelector('[data-mc-field="name"]');el.value='Draft after partial write';el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`[data-mc-section="identity"] .mc-primary`).MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('[data-mc-device-action="reconcile"]').hidden && document.querySelector('[data-mc-section="identity"] fieldset').disabled`)
	if !page.MustEval(`()=>document.querySelector('[data-mc-field="name"]').value==='Draft after partial write' && document.querySelector('[data-mc-section="identity"] .mc-feedback').textContent.includes('Partial')`).Bool() {
		t.Fatal("partial write lost draft or actual values")
	}
	page.MustElement(`[data-mc-device-action="reconcile"]`).MustClick()
	page.MustElement(".mc-dialog .mc-primary").MustClick()
	waitForJSBool(t, page, `()=>!window.uncertain && document.querySelector('[data-mc-device-action="reconcile"]').hidden`)
	page.MustEval(`()=>[...document.querySelectorAll('[data-mc-section="identity"] button')].find(b=>b.textContent===translations['desktop.meshcore_discard']).click()`)
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{1080, 390} {
			page.MustEval(`(theme,width)=>{document.body.dataset.theme=theme;document.getElementById('host').style.width=width+'px';}`, theme, width)
			if page.MustEval(`()=>{const p=document.querySelector('.mc-device-page');return p.scrollWidth>p.clientWidth+1 || p.textContent.includes('desktop.meshcore_');}`).Bool() {
				t.Fatalf("settings overflow/missing translation: %s %d", theme, width)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				os.MkdirAll(dir, 0755)
				name := "meshcore-settings-" + theme + "-wide.png"
				if width == 390 {
					name = "meshcore-settings-" + theme + "-narrow.png"
				}
				if err := os.WriteFile(filepath.Join(dir, name), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustEval(`()=>{document.getElementById('host').style.width='1080px';app.allow_device_settings=false;document.querySelector('[data-mc="refresh"]').click();}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-mc-section="identity"] fieldset').disabled`)
	page.MustEval(`()=>{const el=document.querySelector('[data-mc-field="history_days"]');el.value=180;el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`[data-mc-section="app"] .mc-primary`).MustClick()
	waitForJSBool(t, page, `()=>app.history_days===180`)
	page.MustEval(`()=>document.querySelector('.mc-page-nav button').click()`)
	page.MustElement(`[data-mc="details"]`).MustClick()
	if !page.MustEval(`()=>document.querySelector('.mc-detail').textContent.includes(translations['desktop.meshcore_agent_trusted']) && document.querySelector('.mc-detail').textContent.includes(translations['desktop.meshcore_messenger_favorite'])`).Bool() {
		t.Fatal("trust and favorites not distinguished")
	}
	page.MustEval(`()=>{window.busy=true;[...document.querySelectorAll('.mc-detail button')].find(b=>b.textContent===translations['desktop.meshcore_get_telemetry']).click();}`)
	waitForJSBool(t, page, `()=>document.querySelector('.mc-dialog')?.textContent.includes(translations['desktop.meshcore_error_busy'])`)
	page.MustElement(".mc-dialog header button").MustClick()
	page.MustEval(`()=>window.busy=false`)
	page.MustEval(`()=>[...document.querySelectorAll('.mc-detail button')].find(b=>b.textContent===translations['desktop.meshcore_get_telemetry']).click()`)
	waitForJSBool(t, page, `()=>document.querySelector('.mc-dialog')?.textContent.includes('3.7 V')`)
	if page.MustEval(`()=>Object.values(localStorage).some(v=>v.includes('latitude')||v.includes('telemetry'))`).Bool() {
		t.Fatal("diagnostics persisted in browser")
	}
	page.MustElement(".mc-dialog header button").MustClick()
	page.MustElement(`[data-mc="settings"]`).MustClick()
	page.MustEval(`()=>{window.offline=true;document.querySelector('[data-mc="refresh"]').click();}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-mc-role="status"]').dataset.state==='disconnected'`)
	if !page.MustEval(`()=>document.querySelector('[data-mc-section="identity"] .mc-primary').disabled`).Bool() {
		t.Fatal("offline write enabled")
	}
	page.MustEval(`()=>MeshCoreApp.dispose('device-test')`)
}

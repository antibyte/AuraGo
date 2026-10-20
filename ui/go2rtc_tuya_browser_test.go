package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGo2RTCTuyaSetupBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("/js/", http.FileServer(http.FS(Content)))
			mux.Handle("/css/", http.FileServer(http.FS(Content)))
			mux.Handle("/fonts/", http.FileServer(http.FS(Content)))
			mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
				dict := mustReadUIFile(t, "lang/common/de.json")
				fmt.Fprintf(w, `<!doctype html><html lang="de"><head><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/go2rtc-tuya.css"></head><body class="desktop-body" data-theme="%s"><script>window.I18N=%s;window.errors=[];window.created=[];window.t=(key,p)=>{let s=I18N[key]||key;for(const [k,v] of Object.entries(p||{}))s=s.replaceAll('{{'+k+'}}',v);return s};addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)))</script><script src="/js/go2rtc-tuya.js"></script></body></html>`, theme, dict)
			})
			mux.HandleFunc("/api/go2rtc/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				accounts := []map[string]interface{}{{"id": "smart", "name": "Home", "method": "smart", "region": "eu-central", "camera_count": 0}, {"id": "cloud", "name": "Cloud", "method": "cloud", "region": "eu-central", "camera_count": 0}}
				switch r.URL.Path {
				case "/api/go2rtc/tuya/accounts":
					json.NewEncoder(w).Encode(map[string]interface{}{"accounts": accounts})
				case "/api/go2rtc/tuya/discover":
					w.Write([]byte(`{"status":"ok","setup_token":"smart-token","candidates":[{"id":"opaque-device","name":"Door"}]}`))
				case "/api/go2rtc/tuya/test":
					var body map[string]string
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["account_id"] == "cloud" {
						if body["device_id"] == "working-device" || body["setup_token"] == "cloud-token" && body["candidate_id"] == "cloud-device" {
							w.Write([]byte(`{"status":"ok","width":640,"height":360,"video_codecs":["H264"],"setup_token":"cloud-token","candidates":[{"id":"cloud-device","name":""}]}`))
							return
						}
						<-r.Context().Done()
						return
					}
					if body["resolution"] == "hd" {
						w.Write([]byte(`{"status":"error","error_code":"tuya_connection_failed","suggestion":"sd"}`))
					} else {
						w.Write([]byte(`{"status":"ok","width":640,"height":360,"video_codecs":["H264"],"setup_token":"tested-token","candidates":[{"id":"tested-device","name":"Door"}]}`))
					}
				case "/api/go2rtc/streams":
					var body struct {
						ID   string            `json:"id"`
						Tuya map[string]string `json:"tuya"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body.Tuya["device_id"] != "" || body.Tuya["setup_token"] != "tested-token" || body.Tuya["main_resolution"] != "sd" || body.Tuya["sub_resolution"] != "" {
						t.Error("camera did not use tested opaque selection", body)
					}
					w.Write([]byte(`{"status":"ok","stream":{"id":"door","name":"Door","tuya":{"account_id":"smart","main_resolution":"sd","sub_resolution":""}}}`))
				default:
					http.NotFound(w, r)
				}
			})
			server := httptest.NewServer(mux)
			defer server.Close()
			browser := newSmokeBrowser(t)
			page := browser.MustPage(server.URL + "/fixture").Timeout(35 * time.Second)
			defer page.MustClose()
			width := 1024
			if theme == "fruity" {
				width = 390
			}
			page.MustSetViewport(width, 800, 1, false)
			page.MustWait(`()=>!!window.AuraGoTuya`)
			page.MustEval(`()=>{window.dialog=AuraGoTuya.open({mode:'setup',t,onCreated:r=>created.push(r)});}`)
			page.MustWait(`()=>!!document.querySelector('[data-tuya-field="account"] option[value="smart"]')`)
			page.MustEval(`()=>document.querySelector('[data-tuya-action="discover"]').click()`)
			page.MustWait(`()=>!!document.querySelector('[data-tuya-field="selected"] option[value="opaque-device"]')`)
			page.MustEval(`()=>{const s=document.querySelector('[data-tuya-field="selected"]');s.value='opaque-device';s.dispatchEvent(new Event('change'));document.querySelector('[data-tuya-action="test_hd"]').click()}`)
			page.MustWait(`()=>!!document.querySelector('[data-tuya-action="use_sd"]')`)
			page.MustEval(`()=>document.querySelector('[data-tuya-action="test_sd"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-tuya-result="sd"]')?.textContent.includes('640')`)
			page.MustEval(`()=>document.querySelector('[data-tuya-action="use_sd"]').click()`)
			if !page.MustEval(`()=>{const dialog=document.querySelector('dialog');return dialog.getBoundingClientRect().width<=innerWidth&&!document.querySelector('[data-tuya-action="create_camera"]').disabled}`).Bool() {
				t.Fatal("dialog overflows or tested SD was not accepted")
			}
			page.MustEval(`()=>document.querySelector('[data-tuya-action="create_camera"]').click()`)
			page.MustWait(`()=>created.length===1&&!document.querySelector('dialog')`)
			page.MustEval(`()=>{window.dialog=AuraGoTuya.open({mode:'accounts',t})}`)
			page.MustWait(`()=>!!document.querySelector('[data-tuya-edit="cloud"]')`)
			page.MustEval(`()=>{document.querySelector('[data-tuya-edit="cloud"]').click();const input=document.querySelector('[data-tuya-field="draft.client_secret"]');input.value='unsaved-secret';input.dispatchEvent(new Event('input'));}`)
			if !page.MustEval(`()=>document.querySelector('[data-tuya-field="draft.client_secret"]').value==='unsaved-secret'`).Bool() {
				t.Fatal("account draft lost")
			}
			page.MustEval(`()=>document.querySelector('[data-tuya-action="close"]').click()`)
			page.MustEval(`()=>{window.dialog=AuraGoTuya.open({mode:'setup',t})}`)
			page.MustWait(`()=>!!document.querySelector('[data-tuya-field="account"] option[value="cloud"]')`)
			page.MustEval(`()=>{const a=document.querySelector('[data-tuya-field="account"]');a.value='cloud';a.dispatchEvent(new Event('change'));const d=document.querySelector('[data-tuya-field="device"]');d.value='working-device';d.dispatchEvent(new Event('input'));document.querySelector('[data-tuya-action="test_hd"]').click();}`)
			page.MustWait(`()=>document.querySelector('[data-tuya-result="hd"]')?.textContent.includes('640')`)
			if !page.MustEval(`()=>{const s=document.querySelector('[data-tuya-field="sub"]');s.value='';s.dispatchEvent(new Event('change'));return !document.querySelector('[data-tuya-action="create_camera"]').disabled}`).Bool() {
				t.Fatal("removing an untested substream did not update camera readiness")
			}
			if !page.MustEval(`()=>{const s=document.querySelector('[data-tuya-field="sub"]');s.value='sd';s.dispatchEvent(new Event('change'));return document.querySelector('[data-tuya-action="create_camera"]').disabled}`).Bool() {
				t.Fatal("an untested substream kept the create button enabled")
			}
			page.MustEval(`()=>{const s=document.querySelector('[data-tuya-field="sub"]');s.value='';s.dispatchEvent(new Event('change'));const d=document.querySelector('[data-tuya-field="device"]');d.focus();d.value='device123';d.dispatchEvent(new Event('input'));}`)
			if !page.MustEval(`()=>document.querySelector('[data-tuya-action="create_camera"]').disabled&&!document.querySelector('[data-tuya-result]')&&document.activeElement===document.querySelector('[data-tuya-field="device"]')`).Bool() {
				t.Fatal("a changed device retained an old test result or lost input focus")
			}
			page.MustEval(`()=>document.querySelector('[data-tuya-action="test_sd"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-tuya-action="test_sd"]').disabled`)
			page.MustEval(`()=>document.querySelector('[data-tuya-action="close"]').click()`)
			page.MustWait(`()=>!document.querySelector('dialog')`)
			if got := page.MustEval(`()=>errors.join('\n')`).Str(); got != "" {
				t.Fatal(got)
			}
			if page.MustEval(`()=>document.body.innerText.includes('unsaved-secret')||document.body.innerText.includes('common.go2rtc_tuya.')`).Bool() {
				t.Fatal("secret or missing translation displayed")
			}
		})
	}
}

func TestGo2RTCTuyaConfigKeepsOtherDraftsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(40 * time.Second)
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	page.MustWait(`()=>!!window.AuraConfigState&&!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{configData.go2rtc.streams=[{id:'existing',name:'Saved',enabled:false,source_configured:true}];AuraConfigState.init(configData);await selectSection('go2rtc');const field=document.querySelector('[data-path="go2rtc.streams.0.name"]');field.value='Unsaved';field.dispatchEvent(new Event('input',{bubbles:true}));window.AuraGoTuya={open:opts=>{window.tuyaOptions=opts;return {close(){}}}};go2rtcTuyaOpen('setup');tuyaOptions.onChanged({stream:{id:'tuya-new',name:'New',enabled:false,tuya:{account_id:'home',main_resolution:'hd',sub_resolution:'sd'}}});}`)
	if !page.MustEval(`()=>AuraConfigState.get('go2rtc.streams')[0].name==='Unsaved'&&AuraConfigState.get('go2rtc.streams',{saved:true})[0].name==='Saved'&&AuraConfigState.get('go2rtc.streams').length===2&&AuraConfigState.buildPatch().go2rtc.streams.length===2`).Bool() {
		t.Fatal("Tuya import overwrote an existing camera draft or saved baseline")
	}
	if !strings.Contains(page.MustEval(`()=>JSON.stringify(AuraConfigState.get('go2rtc.streams')[1])`).Str(), "tuya-new") {
		t.Fatal("camera import missing")
	}
}

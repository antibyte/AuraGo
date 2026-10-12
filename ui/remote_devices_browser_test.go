package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

func TestRemoteDeviceTranslations(t *testing.T) {
	for _, locale := range strings.Fields("cs da de el en es fr hi it ja nl no pl pt sv zh") {
		data, err := Content.ReadFile("lang/config/remote_control/" + locale + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var words map[string]string
		if err := json.Unmarshal(data, &words); err != nil {
			t.Fatal(err)
		}
		for _, key := range strings.Fields("refresh devices_error manage save_device device_read_only write_enabled effective capabilities version unknown pending confirmed error server_enforced saved revoke delete revoked upgrade_required confirm_removal paths_inherit security_hint") {
			if words["config.remote_control."+key] == "" {
				t.Errorf("%s lacks %s", locale, key)
			}
		}
		if words["config.remote_control.discovery_port_label"] != "" {
			t.Errorf("%s retains discovery field", locale)
		}
	}
}

func TestRemoteDevicesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview").Timeout(90 * time.Second)
	defer page.Close()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
 const original=window.fetch; window.remoteGets=0; window.remoteWrites=[]; window.remoteFail=false;
 window.remoteRows=[{id:'fixture-device',name:'Arbeitsrechner',os:'windows',arch:'amd64',version:'security-test',status:'connected',is_connected:true,read_only:false,allowed_paths:[],effective_read_only:false,effective_allowed_paths:['C:\\Freigabe'],capabilities:['sysinfo','file_read','shell_exec'],config_state:'confirmed'}];
 window.fetch=async(url,opts={})=>{
  if(String(url)==='/api/remote/devices') {window.remoteGets++;return window.remoteFail?new Response('{}',{status:503}):Response.json(window.remoteRows);}
	 if(String(url).startsWith('/api/remote/devices/')) {window.remoteWrites.push({url:String(url),method:opts.method,body:opts.body});if(opts.method==='DELETE'){window.remoteRows=[];return Response.json({status:'deleted'});}window.remoteRows[0].config_state='pending';if(String(url).endsWith('/revoke')){window.remoteRows[0].status='revoked';window.remoteRows[0].is_connected=false;}return Response.json(window.remoteRows[0]);}
  return original(url,opts);
 };
 configData.remote_control={enabled:true,allowed_paths:[]};await selectSection('remote_control',{scrollBehavior:'auto'});
 }`)
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-device-id] summary')`)
	page.MustElement("[data-device-id] summary").MustFocus()
	page.Keyboard.MustType(input.Enter)
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-device-id] details[open]')`)
	page.MustElement("[data-rc-paths]").MustInput("C:\\Entwurf")
	page.MustEval(`async()=>{await loadRemoteDevices()}`)
	if !page.MustEval(`()=>document.querySelector('[data-rc-paths]').value==='C:\\Entwurf' && document.activeElement===document.querySelector('[data-rc-paths]')`).Bool() {
		t.Fatal("polling lost focused draft")
	}
	for _, surface := range []struct{ name, theme string }{{"standard", "dark"}, {"fruity-light", "light"}, {"fruity-dark", "dark"}} {
		for _, width := range []int{390, 1440} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.querySelector('[data-device-id]').scrollIntoView({block:'center'})}`, surface.theme)
			if !page.MustEval(`()=>{let e=document.querySelector('[data-device-id]');return e.scrollWidth<=e.clientWidth+1&&[...e.querySelectorAll('button')].every(b=>b.getBoundingClientRect().height>=44)}`).Bool() {
				t.Errorf("overflow or small controls: %s/%d", surface.name, width)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("remote-devices-%s-%d.png", surface.name, width)))
			}
		}
	}
	page.MustElement("[data-rc-readonly]").MustClick()
	page.MustElement("[data-rc-action=save]").MustClick()
	waitForJSBool(t, page, `()=>window.remoteWrites.length===1&&document.querySelector('[data-rc-feedback]').textContent===rcText('saved')`)
	if !page.MustEval(`()=>{const body=JSON.parse(remoteWrites[0].body);return remoteWrites[0].method==='PUT'&&body.read_only&&body.allowed_paths[0]==='C:\\Entwurf'&&document.querySelector('.rc-device-status').textContent.includes(rcText('pending'))}`).Bool() {
		t.Fatal("saved policy or pending feedback missing")
	}
	page.MustEval(`async()=>{window.remoteFail=true;await loadRemoteDevices()}`)
	if !page.MustEval(`()=>!!document.getElementById('rc-devices-error').textContent`).Bool() {
		t.Fatal("load error disguised as empty device list")
	}
	page.MustEval(`async()=>{window.remoteFail=false;await loadRemoteDevices();window.remoteBeforePoll=remoteGets}`)
	page.MustEval(`async()=>{await new Promise(r=>setTimeout(r,11000))}`)
	if !page.MustEval(`()=>remoteGets>remoteBeforePoll`).Bool() {
		t.Fatal("visible section did not refresh after ten seconds")
	}
	page.MustElement("[data-rc-action=revoke]").MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('.modal-overlay.active')`)
	page.Keyboard.MustType(input.Escape)
	waitForJSBool(t, page, `()=>!document.querySelector('.modal-overlay.active')`)
	if page.MustEval(`()=>remoteWrites.length`).Int() != 1 {
		t.Fatal("cancelled confirmation mutated the device")
	}
	page.MustElement("[data-rc-action=revoke]").MustClick()
	page.MustElement("#shared-modal-confirm, #modal-confirm").MustClick()
	waitForJSBool(t, page, `()=>remoteWrites.length===2&&remoteWrites[1].method==='POST'&&remoteWrites[1].url.endsWith('/revoke')&&document.querySelector('.rc-device-status').textContent.includes(rcText('revoked'))`)
	page.MustElement("[data-rc-action=delete]").MustClick()
	page.MustElement("#shared-modal-confirm, #modal-confirm").MustClick()
	waitForJSBool(t, page, `()=>remoteWrites.length===3&&remoteWrites[2].method==='DELETE'&&!document.querySelector('[data-device-id]')`)
	page.MustEval(`async()=>{await selectSection('overview');window.remoteAfterLeave=remoteGets;await new Promise(r=>setTimeout(r,10500))}`)
	if !page.MustEval(`()=>remoteGets===remoteAfterLeave`).Bool() {
		t.Fatal("hidden section kept polling")
	}
}

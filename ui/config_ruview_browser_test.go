package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestRuViewConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#ruview").Timeout(60 * time.Second)
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.getElementById('ruview-udp_port')`)
	page.MustSetViewport(1440, 1000, 1, false)
	clickToggle := func(path string) {
		selector := `[data-path="ruview.` + path + `"]`
		page.MustElement(selector).MustScrollIntoView()
		point := page.MustEval(`selector => {const r=document.querySelector(selector).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`, selector).Arr()
		page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ruview-config.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !page.MustEval(`() => {
	 const field = document.getElementById('ruview-udp_port');
	 return field.dataset.path === 'ruview.udp_port' && field.classList.contains('field-input') &&
	   !!field.closest('.field-group').querySelector('.field-label');
	}`).Bool() {
		t.Fatal("RuView inputs bypass the shared form styles and save binding")
	}
	if !page.MustEval(`() => !hasUnsavedConfigChanges() && document.getElementById('ruview-check').disabled &&
	 document.getElementById('ruview-setup').disabled &&
	 !performance.getEntriesByType('resource').some(entry => entry.name.includes('/api/ruview/'))`).Bool() {
		t.Fatal("opening the form must stay clean, passive and disabled until enabled")
	}
	if !page.MustEval(`() => {
	 const control=document.querySelector('[data-path="ruview.readonly"]');
	 return control.checkVisibility() && !control.closest('details') && document.getElementById('ruview-readonly-notice').checkVisibility();
	}`).Bool() {
		t.Fatal("read-only and its consequences must be visible without opening advanced permissions")
	}
	page.MustEval(`() => {
	 window.ruviewRequests=[]; window.ruviewErrors=[];
	 window.addEventListener('error', event => window.ruviewErrors.push(event.message));
	 const original=window.fetch;
	 window.fetch=async(url,options={})=>{
	  if (String(url).startsWith('/api/ruview/')) {
	   window.ruviewRequests.push({url:String(url),options});
	   if (window.ruviewOffline) throw new Error('offline');
	   if (String(url)==='/api/ruview/status') return new Response(JSON.stringify({enabled:true,readonly:false}));
	   if (String(url)==='/api/ruview/firmware') return new Response(JSON.stringify({firmware:[]}));
	   await new Promise(resolve => setTimeout(resolve,120));
	   return new Response(JSON.stringify({fresh:true,data_state:'live_unverified'}));
	  }
	  return original(url,options);
	 };
	}`)
	clickToggle("enabled")
	page.MustElement("#ruview-mode").MustSelect("Externe Instanz")
	page.MustElement("#ruview-url").MustInput("http://192.168.1.20:3000")
	page.MustElement("#ruview-api_token").MustInput("fixture-only-token")
	clickToggle("allow_private")
	waitForJSBool(t, page, `() => !document.getElementById('ruview-receiver-suggestion').hidden`)
	page.MustElement("#ruview-use-receiver").MustClick()
	page.MustEval(`async () => { await ruviewConfigTest(); await ruviewConfigFlash(); }`)
	if !page.MustEval(`() => {
	 const patch=AuraConfigState.buildPatch().ruview;
	 return patch.enabled && patch.mode==='external' && patch.url==='http://192.168.1.20:3000' &&
	  patch.api_token==='fixture-only-token' && patch.allow_private && patch.receiver_ip==='192.168.1.20' &&
	  document.getElementById('ruview-check').disabled && window.ruviewRequests.length===0 &&
	  !document.getElementById('ruview-external-fields').hidden && document.getElementById('ruview-managed-fields').hidden;
	}`).Bool() {
		t.Fatalf("edits must enter the shared draft and block actions until saved: %s", page.MustEval(`() => {
		 const patch=AuraConfigState.buildPatch().ruview||{};
		 return {keys:Object.keys(patch),enabled:patch.enabled,mode:patch.mode,url:patch.url,tokenMatches:patch.api_token==='fixture-only-token',allowPrivate:patch.allow_private,receiver:patch.receiver_ip,
		 checkDisabled:document.getElementById('ruview-check').disabled,requests:window.ruviewRequests.length,errors:window.ruviewErrors,
		 externalHidden:document.getElementById('ruview-external-fields').hidden,managedHidden:document.getElementById('ruview-managed-fields').hidden};
		}`).JSON("", ""))
	}
	page.MustElement("#btnSave").MustClick()
	waitForJSBool(t, page, `() => !hasUnsavedConfigChanges() && !document.getElementById('ruview-check').disabled`)
	if !page.MustEval(`async () => {
	 const saved=(await (await fetch('/api/config')).json()).ruview;
	 return saved.enabled && saved.mode==='external' && saved.url==='http://192.168.1.20:3000' &&
	  saved.api_token==='fixture-only-token' && saved.receiver_ip==='192.168.1.20';
	}`).Bool() {
		t.Fatal("config save did not persist the edited RuView values")
	}
	if !page.MustEval(`() => document.getElementById('ruview-setup').disabled &&
	 document.getElementById('ruview-setup-reason').textContent===t('ruview.write_first')`).Bool() {
		t.Fatal("board setup must explain the read-only lock")
	}
	page.MustEval(`async () => { await Promise.all([ruviewConfigTest(),ruviewConfigTest()]); }`)
	if !page.MustEval(`() => window.ruviewRequests.length===1 && window.ruviewRequests[0].options.method==='POST' &&
	 document.getElementById('ruview-result').textContent===t('ruview.received')`).Bool() {
		t.Fatal("explicit test must issue one POST and show the result")
	}
	page.MustEval(`async () => {window.ruviewOffline=true; await ruviewConfigTest(); window.ruviewOffline=false;}`)
	if !page.MustEval(`() => document.getElementById('ruview-result').textContent===t('ruview.error') &&
	 !document.getElementById('ruview-check').disabled`).Bool() {
		t.Fatal("failed connection test must expose the failure and release its button")
	}
	clickToggle("readonly")
	if !page.MustEval(`() => document.getElementById('ruview-readonly-notice').hidden`).Bool() {
		t.Fatal("read-only notice must follow the visible switch")
	}
	page.MustElement("#btnSave").MustClick()
	waitForJSBool(t, page, `() => !hasUnsavedConfigChanges() && !document.getElementById('ruview-setup').disabled`)
	page.MustElement("#ruview-setup").MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('dialog[open]')`)
	if !page.MustEval(`() => ['/js/shared/lazy-assets.js','/css/ruview.css','/js/ruview/wizard.mjs','/js/ruview/protocol.mjs','/js/ruview/flash.mjs'].every(path =>
	 performance.getEntriesByType('resource').some(entry => {const url=new URL(entry.name);return url.pathname===path && url.searchParams.get('v')===CONFIG_ASSET_VERSION;}))`).Bool() {
		t.Fatal("board setup resources must load with the active Config asset version")
	}
	if page.MustElement(`dialog input[name="receiver"]`).MustProperty("value").Str() != "192.168.1.20" {
		t.Fatal("board wizard did not receive the saved receiver address")
	}
	for _, theme := range []string{"dark", "light"} {
		page.MustEval(`theme => {document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme;}`, theme)
		page.MustEval(`() => new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
		if !page.MustEval(`theme => {
		 const dialog=document.querySelector('dialog'), style=getComputedStyle(dialog);
		 const rgb=style.backgroundColor.match(/[\d.]+/g).map(Number), brightness=(rgb[0]+rgb[1]+rgb[2])/3;
		 const box=dialog.querySelector('[name="keep"]').getBoundingClientRect();
		 return (theme==='dark'?brightness<100:brightness>150) && box.width===20 && box.height===20 && dialog.querySelectorAll('section h3').length===3;
		}`, theme).Bool() {
			t.Fatalf("%s Config wizard lost its theme, checkbox dimensions or numbered sections", theme)
		}
	}
	page.MustElement(`dialog button[value="close"]`).MustClick()
	waitForJSBool(t, page, `() => !document.querySelector('dialog[open]')`)
	page.MustEval(`() => {const el=document.getElementById('ruview-receiver_ip');el.value='';el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement("#btnSave").MustClick()
	waitForJSBool(t, page, `() => !hasUnsavedConfigChanges()`)
	if page.MustElement("#ruview-setup").MustProperty("disabled").Bool() {
		t.Fatal("an unknown receiver must not block opening the wizard that helps discover it")
	}

	page.MustElement("#ruview-mode").MustSelect("Verwaltetes Docker")
	clickToggle("udp_enabled")
	page.MustElement("#ruview-udp_bind").MustInput("192.168.1.20")
	page.MustElement("#ruview-udp_allow").MustInput("192.168.1.50, 192.168.1.0/24")
	if !page.MustEval(`() => JSON.stringify(AuraConfigState.buildPatch().ruview.udp_allow)===JSON.stringify(['192.168.1.50','192.168.1.0/24'])`).Bool() {
		t.Fatal("allowed board addresses must be saved as an array")
	}
	page.MustElement("#ruview-source").MustSelect("Ausdrückliche Demo")
	if !page.MustEval(`() => document.querySelector('.ruview-board-panel').hidden &&
	 getComputedStyle(document.querySelector('.ruview-board-panel')).display==='none' &&
	 document.getElementById('ruview-source-help').textContent===t('ruview.source_help_simulated')`).Bool() {
		t.Fatal("demo must hide board settings and explain generated data")
	}
	page.MustElement("#ruview-source").MustSelect("ESP32-CSI")
	if !page.MustEval(`() => document.getElementById('ruview-udp_allow').value==='192.168.1.50, 192.168.1.0/24'`).Bool() {
		t.Fatal("source changes lost the draft network settings")
	}
	page.MustElement("#ruview-mode").MustSelect("Externe Instanz")
	page.MustElement("#ruview-source").MustSelect("Host-WLAN-RSSI")
	page.MustElement("#ruview-mode").MustSelect("Verwaltetes Docker")
	if !page.MustEval(`() => !document.getElementById('ruview-mode-notice').hidden &&
	 document.querySelector('#ruview-source option[value="wifi"]').disabled`).Bool() {
		t.Fatal("unsupported Docker/host-Wi-Fi combination needs a visible correction hint")
	}
	page.MustElement("#ruview-source").MustSelect("ESP32-CSI")
	page.MustElement("#btnSave").MustClick()
	waitForJSBool(t, page, `() => !hasUnsavedConfigChanges()`)
	page.MustEval(`async () => { await selectSection('overview'); await selectSection('ruview'); }`)
	if !page.MustEval(`() => document.getElementById('ruview-udp_allow').value==='192.168.1.50, 192.168.1.0/24' && window.ruviewErrors.length===0`).Bool() {
		t.Fatal("saved settings or listeners did not survive leaving and reopening the section")
	}
	page.MustElement(`[data-path="ruview.enabled"]`).MustFocus()
	page.Keyboard.MustType(input.Space)
	if !page.MustEval(`() => !AuraConfigState.get('ruview.enabled') && hasUnsavedConfigChanges()`).Bool() {
		t.Fatal("shared toggle must remain keyboard accessible")
	}
}

func TestRuViewConfigReceiverHintsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#ruview").Timeout(60 * time.Second)
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.getElementById('ruview-use-receiver')`)
	for _, tc := range []struct{ mode, endpoint, bind, want string }{
		{"docker", "tcp://10.2.0.3:2375", "", "10.2.0.3"},
		{"docker", "tcp://10.2.0.3:2375", "192.168.4.2", "192.168.4.2"},
		{"docker", "unix:///var/run/docker.sock", "", ""},
		{"docker", "tcp://10.2.0.999:2375", "", ""},
		{"native", "tcp://10.2.0.3:2375", "", ""},
		{"external", "http://172.16.4.2:3000", "192.168.4.2", "172.16.4.2"},
		{"external", "http://127.0.0.1:3000", "192.168.4.2", ""},
		{"external", "http://8.8.8.8:3000", "192.168.4.2", ""},
		{"external", "http://ruview.local:3000", "192.168.4.2", ""},
	} {
		page.MustEval(`(mode,endpoint,bind) => {
		 const set=(id,value)=>{const el=document.getElementById('ruview-'+id);el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}));};
		 AuraConfigState.set('docker.host',endpoint);
		 set('mode',mode);set('url',endpoint);set('udp_bind',bind);set('receiver_ip','');
		}`, tc.mode, tc.endpoint, tc.bind)
		if tc.want != "" {
			waitForJSBool(t, page, `() => !document.getElementById('ruview-receiver-suggestion').hidden`)
			if got := page.MustElement("#ruview-use-receiver").MustAttribute("data-address"); got == nil || *got != tc.want {
				t.Fatalf("%s/%s: wrong receiver suggestion", tc.mode, tc.endpoint)
			}
			if page.MustElement("#ruview-receiver_ip").MustProperty("value").Str() != "" {
				t.Fatal("a suggestion must not silently change the receiver")
			}
			page.MustElement("#ruview-use-receiver").MustClick()
			if !page.MustEval(`want => AuraConfigState.get('ruview.receiver_ip')===want && AuraConfigState.get('ruview.readonly') &&
			 !AuraConfigState.get('ruview.udp_enabled') && document.getElementById('ruview-receiver-suggestion').hidden`, tc.want).Bool() {
				t.Fatal("accepting an address must update the draft without granting write or UDP access")
			}
			wantBind := tc.bind
			if tc.mode != "external" && wantBind == "" {
				wantBind = tc.want
			}
			if page.MustElement("#ruview-udp_bind").MustProperty("value").Str() != wantBind {
				t.Fatal("existing bindings must survive; only an empty managed binding may inherit the receiver")
			}
		} else if !page.MustEval(`() => document.getElementById('ruview-receiver-suggestion').hidden`).Bool() {
			t.Fatalf("%s/%s must not suggest a loopback, public or unresolved address", tc.mode, tc.endpoint)
		}
	}
	if page.MustEval(`() => performance.getEntriesByType('resource').some(entry=>entry.name.includes('/api/ruview/'))`).Bool() {
		t.Fatal("receiver suggestions must not trigger runtime or network probes")
	}
}

func TestRuViewConfigLayoutBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	browser := newSmokeBrowser(t)
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(locale, func(t *testing.T) {
			page := browser.MustPage(configRefreshFixtureOrigin(t, locale, false) + "/config#ruview").Timeout(60 * time.Second)
			defer func() {
				page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
				page.MustClose()
			}()
			waitForJSBool(t, page, `() => !!document.getElementById('ruview-config')`)
			for _, mode := range []string{"docker", "external", "native"} {
				page.MustEval(`mode => {
				 const el=document.getElementById('ruview-mode'); el.value=mode; el.dispatchEvent(new Event('change',{bubbles:true}));
				 document.querySelectorAll('#ruview-config details').forEach(el=>el.open=true);
				 const udp=document.querySelector('[data-path="ruview.udp_enabled"]'); if(!udp.classList.contains('on'))udp.click();
				}`, mode)
				for _, width := range []int{390, 1440} {
					page.MustSetViewport(width, 1000, 1, width == 390)
					for _, theme := range []string{"dark", "light"} {
						page.MustEval(`theme => {document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme;}`, theme)
						page.MustEval(`() => new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
						problems := page.MustEval(`() => {
						 const root=document.getElementById('content'), issues=[];
						 if(root.scrollWidth>root.clientWidth+1)issues.push('horizontal overflow');
						 if(/ruview\.[a-z_]+|config\.refresh\.[a-z_]+/.test(root.innerText))issues.push('untranslated text');
						 for(const field of root.querySelectorAll('input:not([type="hidden"]),select')){
						  if(!field.checkVisibility())continue;
						  const group=field.closest('.field-group'),label=group?.querySelector('.field-label');
						  if(!label){issues.push('missing label: '+field.id);continue;}
						  const a=field.getBoundingClientRect(),b=label.getBoundingClientRect();
						  if(a.top<b.bottom+4)issues.push('label overlap: '+field.id);
						  if(a.height<44||parseFloat(getComputedStyle(field).fontSize)<16)issues.push('small control: '+field.id);
						  if(!field.getAttribute('aria-labelledby'))issues.push('unnamed control: '+field.id);
						 }
						 const buttons=[...root.querySelectorAll('.cfg-actions-row button,.cfg-actions-row a')].filter(el=>el.checkVisibility());
						 for(let i=0;i<buttons.length;i++)for(let j=i+1;j<buttons.length;j++){
						  const a=buttons[i].getBoundingClientRect(),b=buttons[j].getBoundingClientRect();
						  if(a.left<b.right&&a.right>b.left&&a.top<b.bottom&&a.bottom>b.top)issues.push('overlapping actions');
						 }
						 return issues;
						}`).Arr()
						if len(problems) > 0 {
							t.Errorf("%s/%d/%s: %v", mode, width, theme, problems)
						}
						if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && locale == "de" {
							for _, bottom := range []bool{false, true} {
								page.MustEval(`bottom => {const root=document.getElementById('content');root.scrollTop=bottom?root.scrollHeight:0;}`, bottom)
								if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("ruview-%s-%d-%s-bottom-%t.png", mode, width, theme, bottom)), page.MustScreenshot(), 0o644); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
				}
			}
		})
	}
}

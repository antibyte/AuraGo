package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectionGuardLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var english map[string]string
	for _, locale := range locales {
		data, err := os.ReadFile(filepath.Join("lang", "config", "guardian", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err = json.Unmarshal(data, &values); err != nil {
			t.Fatal(err)
		}
		if locale == "en" {
			english = values
		}
		for _, key := range []string{"title", "desc", "enabled", "enable_help", "action", "quarantine", "datamark", "datamark_help", "size", "download", "cancel", "test", "loading", "failed", "test_ok", "state_not_installed", "state_downloading", "state_verifying", "state_ready", "state_active", "state_error", "state_unsupported", "state_cancelled"} {
			if strings.TrimSpace(values["config.guardian.local_"+key]) == "" {
				t.Fatalf("%s missing %s", locale, key)
			}
		}
	}
	for _, locale := range locales {
		if locale == "en" {
			continue
		}
		data, _ := os.ReadFile(filepath.Join("lang", "config", "guardian", locale+".json"))
		var values map[string]string
		_ = json.Unmarshal(data, &values)
		if values["config.guardian.local_desc"] == english["config.guardian.local_desc"] {
			t.Fatalf("%s has English fallback", locale)
		}
	}
}

func TestInjectionGuardConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer func() { page.MustEval(`()=>{resetDirtySnapshot();window.onbeforeunload=null;}`); page.MustClose() }()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
 const original=window.fetch; window.guardCalls=[];
 window.guardState={available:true,enabled:false,ready:false,action:'quarantine',state:'not_installed',size_bytes:302796789,runtime_size_bytes:8200000,progress:{percent:0}};
 window.fetch=async (url,opts={})=>{
  if (String(url).startsWith('/api/guardian/injection-guard/')) {
   const action=String(url).split('/').pop();guardCalls.push({action,method:opts.method||'GET'});
   if(action==='download'){guardState.state='downloading';guardState.progress.percent=42;}
   if(action==='cancel'){guardState.state='error';guardState.error='cancelled';}
   return new Response(JSON.stringify(guardState),{status:action==='download'?202:200,headers:{'Content-Type':'application/json'}});
  }
  return original(url,opts);
 };
 }`)
	page.MustEval(`async()=>{await selectSection('guardian',{scrollBehavior:'auto'});resetDirtySnapshot();}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-guard-status]')?.textContent==='Nicht installiert'`)
	if page.MustEval(`()=>guardCalls.some(x=>x.method==='POST')`).Bool() {
		t.Fatal("page load performed installation")
	}
	if !page.MustEval(`()=>document.querySelector('[data-path="guardian.injection_guard.enabled"]').disabled`).Bool() {
		t.Fatal("uninstalled activation enabled")
	}
	page.MustElement(`[data-guard-action="download"]`).MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('[data-guard-progress]').hidden && document.querySelector('[data-guard-progress]').value===42`)
	page.MustElement(`[data-guard-action="cancel"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-guard-status]').textContent==='Download abgebrochen'`)
	page.MustEval(`()=>{guardState.ready=true;guardState.state='ready';guardState.error='';document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-path="guardian.injection_guard.enabled"]').disabled`)
	page.MustElement(`[data-path="guardian.injection_guard.enabled"]`).MustClick()
	page.MustEval(`()=>{const s=document.querySelector('#injection-guard-action');s.value='datamark';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustEval(`()=>document.dispatchEvent(new Event('aurago:config-saved'))`)
	waitForJSBool(t, page, `()=>{const p=AuraConfigState.buildPatch();return p.guardian?.injection_guard?.enabled===true && p.guardian.injection_guard.action==='datamark';}`)
	if page.MustEval(`()=>guardState.enabled`).Bool() {
		t.Fatal("draft toggle activated model before saving")
	}
	page.MustElement(`[data-guard-action="test"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-guard-result]').textContent==='Selbsttest bestanden.'`)
	if page.MustEval(`()=>document.querySelector('[data-guard-action="cancel"]').getClientRects().length>0`).Bool() {
		t.Fatal("cancel visible without active download")
	}
	if !page.MustEval(`async()=>await saveConfig()`).Bool() {
		t.Fatal("config save failed")
	}
	if !page.MustEval(`async()=>{const c=await (await fetch('/api/config')).json();return c.guardian.injection_guard.enabled===true && c.guardian.injection_guard.action==='datamark';}`).Bool() {
		t.Fatal("saved settings lost boolean or action")
	}
	page.MustEval(`()=>{guardState.enabled=true;document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-guard-status]').textContent==='Aktiv'`)
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"dark", "light"} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.getElementById('injection-guard-card').scrollIntoView();}`, theme)
			if page.MustEval(`()=>{const c=document.getElementById('content');return document.documentElement.scrollWidth>innerWidth+1||c.scrollWidth>c.clientWidth+1;}`).Bool() {
				t.Errorf("overflow %s %d", theme, width)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("injection-guard-%s-%d.png", theme, width)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustEval(`()=>{guardState.available=false;guardState.ready=false;guardState.state='unsupported';document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-guard-status]').textContent==='Auf dieser Plattform nicht verfügbar'`)
	if page.MustEval(`()=>document.querySelector('[data-path="guardian.injection_guard.enabled"]').disabled`).Bool() {
		t.Fatal("cannot disable previously enabled unavailable model")
	}
	page.MustEval(`()=>{window.dispatchEvent(new Event('cfg:section-leave'));document.getElementById('content').innerHTML='other section';window.leftCalls=guardCalls.length;document.dispatchEvent(new Event('aurago:config-saved'));}`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1700))`)
	if page.MustEval(`()=>guardCalls.length!==leftCalls`).Bool() {
		t.Fatal("detached card still polls")
	}
}

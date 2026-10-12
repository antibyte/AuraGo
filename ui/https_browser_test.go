package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

func TestHTTPSConfigOptOutBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshFixtureOrigin(t, "de", false)
	upstream, _ := url.Parse(origin)
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	var saves atomic.Int32
	var tlsOrigin string
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.Request.URL.Path == "/api/config" && response.Request.Method == "PUT" {
			saves.Add(1)
			_ = response.Body.Close()
			payload, _ := json.Marshal(map[string]interface{}{"status": "saved", "needs_restart": true, "restart_url": origin + "/config", "restart_from": tlsOrigin})
			response.Body = io.NopCloser(bytes.NewReader(payload))
			response.ContentLength = int64(len(payload))
			response.Header.Set("Content-Length", fmt.Sprint(len(payload)))
		}
		if saves.Load() > 0 {
			response.Header.Set("Strict-Transport-Security", "max-age=0")
		}
		return nil
	}
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/restart" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"Restarting"}`))
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer tlsServer.Close()
	tlsOrigin = tlsServer.URL
	browser := newSmokeBrowser(t).MustIgnoreCertErrors(true)
	page := browser.MustPage(tlsOrigin + "/config#server")
	// Browser cleanup also closes the page without blocking on unsaved drafts.
	waitForJSBool(t, page, `() => !!document.querySelector('[data-path="server.https.enabled"]')`)
	for _, width := range []int{1440, 390} {
		for _, theme := range []string{"dark", "light"} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustEval(`theme => {document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme; const domain=document.querySelector('[data-path="server.https.domain"]'); domain.value='draft.local'; domain.dispatchEvent(new Event('input',{bubbles:true})); document.querySelector('[data-path="server.https.enabled"]').focus();}`, theme)
			page.Keyboard.MustType(input.Space)
			waitForJSBool(t, page, `() => document.querySelector('#shared-modal-overlay')?.style.display==='flex'`)
			page.MustEval(`() => {const text=document.querySelector('#shared-modal-message').textContent; if(!text.includes('Sprachchat') || !text.includes('localhost') || !text.includes('HSTS')) throw Error(text); if(document.documentElement.scrollWidth>innerWidth+1) throw Error('horizontal overflow'); const buttons=document.querySelector('#shared-modal-confirm').getBoundingClientRect(); if(buttons.bottom>innerHeight || buttons.left<0) throw Error('confirmation outside viewport');}`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				_ = os.MkdirAll(dir, 0755)
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("https-confirm-%s-%d.png", theme, width)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
			page.MustElement("#shared-modal-cancel").MustClick()
			waitForJSBool(t, page, `() => !_srvHttpsConfirmPending`)
			page.MustEval(`() => {if(!configData.server.https.enabled || document.querySelector('[data-path="server.https.domain"]').value!=='draft.local') throw Error('cancel lost switch or draft'); if(document.querySelector('[data-path="server.https.http_port"]').value!=='0') throw Error('disabled HTTP listener changed');}`)
		}
	}
	if saves.Load() != 0 {
		t.Fatal("confirmation or cancellation saved config")
	}
	page.MustEval(`() => document.querySelector('[data-path="server.https.enabled"]').focus()`)
	page.Keyboard.MustType(input.Space)
	waitForJSBool(t, page, `() => document.querySelector('#shared-modal-overlay')?.style.display==='flex'`)
	page.MustElement("#shared-modal-confirm").MustClick()
	waitForJSBool(t, page, `() => configData.server.https.enabled===false`)
	page.MustEval(`async () => {await new Promise(requestAnimationFrame); await new Promise(requestAnimationFrame); if(document.activeElement!==document.querySelector('[data-path="server.https.enabled"]')) throw Error('HTTPS switch lost keyboard focus after confirmation');}`)
	if saves.Load() != 0 {
		t.Fatal("confirm saved without Save")
	}
	page.MustEval(`async () => {if(!await saveConfig()) throw Error('save failed'); const response=await fetch('/api/config'); if(response.headers.get('Strict-Transport-Security')!=='max-age=0') throw Error('HSTS not cleared on refresh');}`)
	if saves.Load() != 1 {
		t.Fatalf("saves=%d", saves.Load())
	}
	page.MustEval(`target => {sessionStorage.removeItem('aurago.config.restart');_configRestart=null;const write=Storage.prototype.setItem;try {Storage.prototype.setItem=()=>{throw Error('storage unavailable');};rememberConfigRestart({restart_from:location.origin,restart_url:target+'/config'});if(configRestartTarget()!==target+'/config')throw Error('address lost without storage');}finally{Storage.prototype.setItem=write;}}`, origin)
	page.MustEval(`target => {if(configRestartTarget()!==target+'/config') throw Error('wrong restart target'); const link=document.querySelector('#saveStatus a'); if(!link || link.href!==target+'/config') throw Error('missing future address'); rememberConfigRestart({restart_from:location.origin,restart_url:''}); if(configRestartTarget()!==location.href) throw Error('proxy retained stale target'); rememberConfigRestart({restart_from:'http://internal:8088',restart_url:'http://internal:8088/config'}); if(configRestartTarget()!==location.href) throw Error('proxy origin overwritten'); rememberConfigRestart({restart_from:location.origin,restart_url:target+'/config'});}`, origin)
	// Exercise the existing restart action and the actual HTTPS -> HTTP browser navigation.
	page.MustEval(`async () => {const later=window.setTimeout;window.setTimeout=(fn,ms,...args)=>later(fn,ms===8000?50:ms,...args);await restartAuraGo(true);}`)
	waitForJSBool(t, page, `() => location.origin === `+fmt.Sprintf("%q", origin))
}

func TestSetupHTTPSContextBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var translations map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/setup/de.json"), &translations); err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(map[string]interface{}{"systemLang": "de", "buildVersion": "https-test", "i18n": translations})
	html := strings.NewReplacer("{{.Lang}}", "de", "{{.BuildVersion}}", "https-test", "{{.TemplateDataJSON}}", string(metadata)).Replace(string(mustReadUIFile(t, "setup.html")))
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/setup" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, html)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/setup/status":
				_, _ = io.WriteString(w, `{"needs_setup":true,"csrf_token":"fixture","https":{"enabled":true,"cert_mode":"selfsigned","port":8443}}`)
			case "/api/i18n":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": translations})
			default:
				_, _ = io.WriteString(w, `{"profiles":[],"personalities":[],"auth":{"password_set":false}}`)
			}
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	}))
	defer fixture.Close()
	page := newSmokeBrowser(t).MustPage(fixture.URL + "/setup")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!window.renderSetupHTTPSStatus && !document.getElementById('setup-https-certificate').classList.contains('is-hidden')`)
	for _, secure := range []bool{true, false} {
		page.MustSetViewport(390, 844, 1, false)
		page.MustEval(`(secure,translations) => {window.t=key=>translations[key]||key;Object.defineProperty(window,'isSecureContext',{value:secure,configurable:true});applyI18N(); const status=document.getElementById('setup-https-status');if(status.classList.contains('setup-https-insecure')===secure) throw Error('scheme mistaken for secure context');if(!document.getElementById('setup-https-transport').textContent.includes('8443')) throw Error('port missing');if(document.documentElement.scrollWidth>innerWidth+1) throw Error('setup overflows');}`, secure, translations)
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "https-setup-390.png"), page.MustScreenshot(), 0644)
	}
}

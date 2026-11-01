package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// whatsAppFixtureOrigin serves the Config fixture with a saved, enabled,
// dedicated WhatsApp section. answer replaces every /api/whatsapp/ route.
func whatsAppFixtureOrigin(t *testing.T, answer http.HandlerFunc) string {
	t.Helper()
	base, _ := url.Parse(configRefreshFixtureOriginWith(t, "de", false, func(cfg map[string]any) {
		section, _ := cfg["whatsapp"].(map[string]any)
		if section == nil {
			section = map[string]any{}
			cfg["whatsapp"] = section
		}
		section["enabled"] = true
		section["mode"] = "dedicated"
		section["owner_numbers"] = []any{"+491701234567"}
	}))
	proxy := httputil.NewSingleHostReverseProxy(base)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/whatsapp/") {
			answer(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(origin.Close)
	return origin.URL
}

// waitForWhatsAppCount waits until the fixture counter reaches want.
func waitForWhatsAppCount(t *testing.T, counter *atomic.Int32, want int32, what string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); counter.Load() < want; time.Sleep(25 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the fixture never saw %d %s", want, what)
		}
	}
}

func writeWhatsAppFixtureJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func TestWhatsAppConfigLinkPanelBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var linkGets atomic.Int32
	var linked atomic.Bool
	var postedMu sync.Mutex
	var posted map[string]any
	origin := whatsAppFixtureOrigin(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/whatsapp/status":
			state := "not_linked"
			if linked.Load() {
				state = "connecting"
			}
			writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"status": state, "whatsapp": map[string]any{"enabled": true, "state": state, "linked": linked.Load(), "linked_jid_masked": "***4567"}})
		case "/api/whatsapp/link":
			if r.Method == http.MethodPost {
				body := map[string]any{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				postedMu.Lock()
				posted = body
				postedMu.Unlock()
				writeWhatsAppFixtureJSON(w, http.StatusAccepted, map[string]any{"state": "waiting"})
				return
			}
			switch n := linkGets.Add(1); {
			case n == 1: // resume probe when the section renders
				writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"state": "idle"})
			case n == 2:
				writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"state": "qr", "qr_png": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="})
			default:
				linked.Store(true)
				writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"state": "success"})
			}
		default:
			writeWhatsAppFixtureJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		}
	})
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin + "/config#whatsapp")
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `()=>{const b=document.getElementById('whatsapp-link-qr');return !!b&&!b.disabled}`)
	if !page.MustEval(`()=>document.getElementById('whatsapp-unlink').disabled&&document.getElementById('whatsapp-test').disabled`).Bool() {
		t.Error("unlink and test are enabled without a linked, healthy device")
	}
	page.MustElement("#whatsapp-link-qr").MustClick()
	waitForJSBool(t, page, `()=>{const img=document.getElementById('whatsapp-qr');return !img.hidden&&img.src.startsWith('data:image/png;base64,')}`)
	waitForJSBool(t, page, `()=>document.getElementById('whatsapp-link-view').hidden&&document.getElementById('whatsapp-linked').textContent.includes('***4567')`)
	postedMu.Lock()
	method, force := posted["method"], posted["force"]
	postedMu.Unlock()
	if method != "qr" || force != false {
		t.Errorf("link request = method %v, force %v; want qr without force", method, force)
	}
	if page.MustEval(`()=>document.getElementById('whatsapp-qr').hasAttribute('src')`).Bool() {
		t.Error("the QR image stayed in the page after the link finished")
	}
	gets := linkGets.Load()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,2500))`)
	if linkGets.Load() != gets {
		t.Error("the panel kept polling after the link finished")
	}
	for _, width := range []int{390, 1440} {
		page.MustSetViewport(width, 900, 1, width == 390)
		if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
			t.Errorf("config overflow at %d px", width)
		}
	}
}

// TestWhatsAppConfigSwitchMethodAndLeaveBrowser covers a running session: a
// refused link shows a translated message, the other method replaces the
// session with force, and leaving the section ends the polling.
func TestWhatsAppConfigSwitchMethodAndLeaveBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var linkGets atomic.Int32
	var posts atomic.Int32
	var postedMu sync.Mutex
	var second map[string]any
	origin := whatsAppFixtureOrigin(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/whatsapp/status":
			writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"status": "not_linked", "whatsapp": map[string]any{"enabled": true, "state": "not_linked", "linked": false}})
		case "/api/whatsapp/link":
			if r.Method != http.MethodPost {
				linkGets.Add(1)
				writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"state": "code", "pairing_code": "ABCD-EFGH"})
				return
			}
			switch posts.Add(1) {
			case 1: // the server refuses the first request
				writeWhatsAppFixtureJSON(w, http.StatusConflict, map[string]any{"error": "link_in_progress"})
			default:
				body := map[string]any{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				postedMu.Lock()
				second = body
				postedMu.Unlock()
				writeWhatsAppFixtureJSON(w, http.StatusAccepted, map[string]any{"state": "waiting"})
			}
		default:
			writeWhatsAppFixtureJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		}
	})
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin + "/config#whatsapp")
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `()=>{const b=document.getElementById('whatsapp-link-qr');return !!b&&!b.disabled}`)
	// The resume probe found a pairing-code session already running.
	waitForJSBool(t, page, `()=>document.getElementById('whatsapp-code').textContent==='ABCD-EFGH'&&!document.getElementById('whatsapp-code').hidden`)
	page.MustElement("#whatsapp-link-qr").MustClick()
	waitForJSBool(t, page, `()=>document.getElementById('whatsapp-result').textContent.includes('bereits')`)
	if page.MustEval(`()=>document.getElementById('whatsapp-result').textContent.includes('link_in_progress')`).Bool() {
		t.Error("the raw error code reached the page")
	}
	// Typing the pairing phone must not make the section dirty or lock the buttons.
	page.MustElement("#whatsapp-phone").MustInput("+491701234567")
	waitForJSBool(t, page, `()=>{const b=document.getElementById('whatsapp-link-phone');return !b.disabled&&!hasUnsavedConfigChanges()}`)
	page.MustElement("#whatsapp-link-phone").MustClick()
	waitForWhatsAppCount(t, &posts, 2, "link requests")
	postedMu.Lock()
	body := second
	postedMu.Unlock()
	if body["method"] != "phone" || body["phone"] != "+491701234567" || body["force"] != true {
		t.Errorf("switch request = %v; want phone with force", body)
	}
	waitForWhatsAppCount(t, &linkGets, 2, "link polls")
	page.MustEval(`async()=>{await selectSection('server',{scrollBehavior:'auto'})}`)
	time.Sleep(200 * time.Millisecond) // a request sent just before the abort may still arrive
	gets := linkGets.Load()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,2500))`)
	if linkGets.Load() != gets {
		t.Error("the panel kept polling after the section was left")
	}
}

// TestWhatsAppConfigStatesUnlinkAndTestBrowser renders every runtime state,
// then runs the test message (sent, rate limited) and unlink actions.
func TestWhatsAppConfigStatesUnlinkAndTestBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var de map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/config/whatsapp/de.json"), &de); err != nil {
		t.Fatal(err)
	}
	var stateMu sync.Mutex
	state, linked := "healthy", true
	var tests atomic.Int32
	origin := whatsAppFixtureOrigin(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/whatsapp/status":
			stateMu.Lock()
			current, isLinked := state, linked
			stateMu.Unlock()
			writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"status": current, "whatsapp": map[string]any{"enabled": current != "disabled", "state": current, "linked": isLinked, "linked_jid_masked": "***4567"}})
		case "/api/whatsapp/link":
			writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"state": "idle"})
		case "/api/whatsapp/test":
			if tests.Add(1) == 1 {
				writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"status": "sent"})
				return
			}
			writeWhatsAppFixtureJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited"})
		case "/api/whatsapp/logout":
			stateMu.Lock()
			state, linked = "not_linked", false
			stateMu.Unlock()
			writeWhatsAppFixtureJSON(w, http.StatusOK, map[string]any{"status": "unlinked", "remote_removed": false})
		default:
			writeWhatsAppFixtureJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		}
	})
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin + "/config#whatsapp")
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	resultHas := func(key string) string {
		quoted, _ := json.Marshal(de[key])
		return fmt.Sprintf(`()=>document.getElementById('whatsapp-result').textContent.includes(%s)`, quoted)
	}
	waitForJSBool(t, page, `()=>{const b=document.getElementById('whatsapp-test');return !!b&&!b.disabled}`)
	page.MustElement("#whatsapp-test").MustClick()
	waitForJSBool(t, page, resultHas("config.whatsapp.test_sent"))
	waitForJSBool(t, page, `()=>!document.getElementById('whatsapp-test').disabled`)
	page.MustElement("#whatsapp-test").MustClick()
	waitForJSBool(t, page, resultHas("config.whatsapp.err_rate_limited"))
	waitForJSBool(t, page, `()=>!document.getElementById('whatsapp-unlink').disabled`)
	page.MustElement("#whatsapp-unlink").MustClick()
	page.MustElement("#shared-modal-confirm").MustClick()
	// The handler answers {"status":"unlinked"}; the panel must count that as success.
	waitForJSBool(t, page, resultHas("config.whatsapp.unlinked_local"))
	if page.MustEval(`()=>document.getElementById('whatsapp-result').dataset.state`).Str() != "success" {
		t.Error("a completed unlink is not reported as success")
	}
	waitForJSBool(t, page, `()=>document.getElementById('whatsapp-unlink').disabled&&!!document.getElementById('whatsapp-status').dataset.state&&document.getElementById('whatsapp-status').dataset.state!=='checking'`)

	for _, want := range []struct{ state, banner, label, detail string }{
		{"disabled", "off", "", ""},
		{"not_configured", "unconfigured", "", "config.whatsapp.err_not_configured"},
		{"not_linked", "unconfigured", "", "config.whatsapp.not_linked"},
		{"healthy", "connected", "", ""},
		{"degraded", "degraded", "", "config.whatsapp.state_degraded"},
		{"logged_out", "error", "", "config.whatsapp.state_logged_out"},
		{"conflict", "error", "", "config.whatsapp.state_conflict"},
		{"temp_banned", "error", "", "config.whatsapp.state_temp_banned"},
		{"outdated", "error", "", "config.whatsapp.state_outdated"},
		{"linking", "checking", "config.whatsapp.state_linking", ""},
		{"connecting", "checking", "config.whatsapp.state_connecting", ""},
	} {
		stateMu.Lock()
		state, linked = want.state, want.state != "disabled" && want.state != "not_configured" && want.state != "not_linked" && want.state != "logged_out"
		stateMu.Unlock()
		page.MustEval(`async()=>{await selectSection('whatsapp',{scrollBehavior:'auto'})}`)
		got := page.MustEval(`()=>{const b=document.getElementById('whatsapp-status');return {state:b.dataset.state,label:b.querySelector('.cfg-state-label').textContent,detail:b.querySelector('.cfg-state-detail').textContent,test:document.getElementById('whatsapp-test').disabled,broken:!!document.querySelector('.cfg-error-state')}}`).Map()
		if got["broken"].Bool() || got["state"].Str() != want.banner {
			t.Errorf("state %s: banner %q (broken %v), want %q", want.state, got["state"].Str(), got["broken"].Bool(), want.banner)
		}
		if want.label != "" && got["label"].Str() != de[want.label] {
			t.Errorf("state %s: label %q, want %q", want.state, got["label"].Str(), de[want.label])
		}
		if got["detail"].Str() != de[want.detail] {
			t.Errorf("state %s: detail %q, want %q", want.state, got["detail"].Str(), de[want.detail])
		}
		if got["test"].Bool() != (want.state != "healthy") {
			t.Errorf("state %s: test button disabled = %v", want.state, got["test"].Bool())
		}
	}
}

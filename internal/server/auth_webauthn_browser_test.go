package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/i18n"
	"aurago/ui"

	"github.com/go-rod/rod/lib/proto"
)

func TestWebAuthnBrowserLoginAndRecovery(t *testing.T) {
	browser := personalRadioBrowser(t)
	s, owner := newWebAuthnTestServer(t, "passkey")
	i18n.Load(ui.Content, s.Logger)
	var recovery atomic.Bool
	var credentialReads, enrollmentFinishes atomic.Int64
	mux := http.NewServeMux()
	s.registerAssetRecoveryRoutes(mux)
	mux.HandleFunc("/api/auth/status", handleAuthStatus(s))
	mux.HandleFunc("/api/auth/activity", handleAuthActivity(s))
	mux.HandleFunc("/api/auth/logout", handleAuthLogoutAPI(s))
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handleAuthLogin(s)(w, r)
			return
		}
		if recovery.Load() {
			s.recoveryPage(w, r)
			return
		}
		handleAuthLoginPage(s, ui.Content)(w, r)
	})
	read := func(path string) string {
		b, err := ui.Content.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	css := strings.Join(regexp.MustCompile(`<link\b[^>]*rel="stylesheet"[^>]*>`).FindAllString(read("config.html"), -1), "\n")
	css = strings.ReplaceAll(css, "{{.BuildVersion}}", "")
	mux.HandleFunc("/fixture/settings", func(w http.ResponseWriter, r *http.Request) {
		a, _ := s.webAuthnAuthSnapshot()
		settings, _ := json.Marshal(a.Config)
		messages, _ := json.Marshal(webAuthnLocales["en"])
		fmt.Fprintf(w, `<!doctype html><html lang="en" data-theme="dark"><head>%s<style>.is-hidden{display:none!important}body{margin:24px auto;max-width:760px;padding:20px}input,select,textarea{max-width:100%%}#content{padding:20px}</style></head><body class="pw-page" data-workspace-page="config"><main id="content"></main>
<script src="/js/shared/shared-core.js"></script>
<script>window.configData={auth:{webauthn:%s},agent:{max_tokens:123}};const labels=%s;Object.assign(labels,{'common.enabled':'Enabled','common.disabled':'Disabled','common.btn_save':'Save','common.btn_delete':'Delete','common.btn_cancel':'Cancel','common.loading':'Loading…','login.password_label':'Password'});window.t=key=>labels[key]||key;window.escapeHtml=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));window.escapeAttr=escapeHtml;window.setHidden=(el,value)=>el.classList.toggle('is-hidden',value);window.showConfirm=async()=>true;</script>
<script src="/js/config/state.js"></script><script src="/auth/webauthn/client.js"></script><script src="/cfg/auth_webauthn.js"></script><script src="/cfg/auth.js"></script>
<script>(async()=>{AuraConfigState.init(configData);AuraConfigState.set('agent.max_tokens',456);_authStatus=await(await fetch('/api/auth/status')).json();document.getElementById('content').innerHTML='<div class="cfg-section active">'+renderWebAuthnSection(configData.auth.webauthn,_authStatus)+'</div>';initWebAuthnSection();})();</script></body></html>`, css, settings, messages)
	})
	files := http.FileServer(http.FS(ui.Content))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprint(w, "<!doctype html><html><body>Authenticated fixture</body></html>")
			return
		}
		files.ServeHTTP(w, r)
	})
	server := httptest.NewServer(authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/webauthn/credentials":
			credentialReads.Add(1)
		case "/api/auth/webauthn/register/finish":
			enrollmentFinishes.Add(1)
		}
		mux.ServeHTTP(w, r)
	})))
	defer server.Close()
	origin := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	s.Cfg.Auth.WebAuthn.Origins = []string{origin}
	page := browser.MustPage().Timeout(90 * time.Second)
	defer page.Close()
	if err := (proto.WebAuthnEnable{}).Call(page); err != nil {
		t.Fatal(err)
	}
	authenticator, err := (proto.WebAuthnAddVirtualAuthenticator{Options: &proto.WebAuthnVirtualAuthenticatorOptions{Protocol: proto.WebAuthnAuthenticatorProtocolCtap2, Transport: proto.WebAuthnAuthenticatorTransportInternal, HasResidentKey: true, HasUserVerification: true, IsUserVerified: true, AutomaticPresenceSimulation: true}}).Call(page)
	if err != nil {
		t.Fatal(err)
	}
	page.MustSetCookies(&proto.NetworkCookieParam{Name: owner.Name, Value: owner.Value, URL: origin, Path: "/", HTTPOnly: true})
	page.MustSetViewport(1100, 1000, 1, false)
	page.MustNavigate(origin + "/fixture/settings").MustWaitLoad()
	page.MustElement("#auth-webauthn-current-pw").MustInput(stepUpTestPassword)
	page.MustElement(`[data-wa-action="settings"]`).MustClick()
	page.MustWait(`()=>document.getElementById('wa-status').classList.contains('is-success')&&document.getElementById('auth-webauthn').getAttribute('aria-busy')==='false'`)
	page.MustElement("#wa-label").MustInput("Browser test key")
	page.MustElement("#auth-webauthn-current-pw").MustInput(stepUpTestPassword)
	page.MustElement(`[data-wa-action="add"]`).MustClick()
	page.MustWait(`()=>document.querySelectorAll('[data-credential]').length===1&&document.getElementById('auth-webauthn').getAttribute('aria-busy')==='false'`)
	page.MustEval(`()=>{document.getElementById('wa-enabled').value='true';}`)
	page.MustElement("#auth-webauthn-current-pw").MustInput(stepUpTestPassword)
	page.MustElement(`[data-wa-action="settings"]`).MustClick()
	page.MustWait(`()=>configData.auth.webauthn.enabled&&document.getElementById('auth-webauthn').getAttribute('aria-busy')==='false'`)
	if !page.MustEval(`()=>configData.agent.max_tokens===123&&AuraConfigState.get('agent.max_tokens')===456&&AuraConfigState.isDirty()&&AuraConfigState.get('auth.webauthn.enabled')===true`).Bool() {
		t.Fatal("protected settings save lost unrelated config draft")
	}
	artifactDir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if artifactDir != "" {
		if err := os.MkdirAll(artifactDir, 0o700); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(artifactDir, "webauthn-settings.png"))
	}
	logout := func() {
		t.Helper()
		page.MustEval(`async()=>{const r=await fetch('/api/auth/logout',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});if(!r.ok)throw Error('logout '+r.status+': '+await r.text());}`)
	}
	for _, mode := range []string{"passkey", "second_factor"} {
		s.CfgSaveMu.Lock()
		s.CfgMu.Lock()
		s.Cfg.Auth.WebAuthn.LoginMode = mode
		s.CfgMu.Unlock()
		s.CfgSaveMu.Unlock()
		logout()
		page.MustNavigate(origin + "/auth/login").MustWaitLoad()
		page.MustWait(`()=>typeof _webAuthnLoginStatus!=='undefined'&&_webAuthnLoginStatus?.webauthn_enabled&&!!window.AuraWebAuthn`)
		if artifactDir != "" {
			page.MustScreenshot(filepath.Join(artifactDir, "webauthn-"+mode+"-login.png"))
		}
		if mode == "second_factor" {
			page.MustElement("#password").MustInput(stepUpTestPassword)
		}
		page.MustElement("#btnPasskey").MustClick()
		page.MustWait(`()=>location.pathname==='/'`)
		if !page.MustEval(`async()=>{const r=await fetch('/api/auth/status');return(await r.json()).authenticated}`).Bool() {
			t.Fatalf("%s did not establish a session", mode)
		}
	}
	logout()
	recovery.Store(true)
	page.MustNavigate(origin + "/auth/login").MustWaitLoad()
	page.MustWait(`()=>!document.getElementById('passkeyButton').hidden&&!document.getElementById('passkeyButton').disabled`)
	if !page.MustEval(`()=>document.querySelectorAll('script[src]').length===0`).Bool() {
		t.Fatal("recovery relies on external UI scripts")
	}
	page.MustElement(`input[name="password"]`).MustInput(stepUpTestPassword)
	page.MustElement("#passkeyButton").MustClick()
	page.MustWait(`()=>location.pathname==='/'`)
	if !page.MustEval(`async()=>{const r=await fetch('/api/auth/status');return(await r.json()).authenticated}`).Bool() {
		t.Fatal("recovery did not establish a session")
	}
	logout()
	recovery.Store(false)
	page.MustSetViewport(390, 780, 1, false)
	page.MustNavigate(origin + "/auth/login").MustWaitLoad()
	page.MustWait(`()=>typeof _webAuthnLoginStatus!=='undefined'&&_webAuthnLoginStatus?.webauthn_enabled`)
	if artifactDir != "" {
		page.MustScreenshot(filepath.Join(artifactDir, "webauthn-mobile-login.png"))
	}
	if !page.MustEval(`()=>{const r=document.getElementById('btnPasskey').getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight}`).Bool() {
		t.Fatal("mobile passkey action outside viewport")
	}
	if err := (proto.WebAuthnSetAutomaticPresenceSimulation{AuthenticatorID: authenticator.AuthenticatorID, Enabled: false}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustElement("#password").MustInput(stepUpTestPassword)
	page.MustElement("#btnPasskey").MustClick()
	page.MustWait(`()=>_loginPending&&!document.getElementById('btnPasskeyCancel').classList.contains('is-hidden')`)
	page.MustElement("#btnPasskeyCancel").MustClick()
	page.MustWait(`()=>!_loginPending&&document.getElementById('loginError').textContent===t('webauthn.cancelled')`)
	if page.MustEval(`async()=>{const r=await fetch('/api/auth/status');return(await r.json()).authenticated}`).Bool() {
		t.Fatal("cancelled login issued session")
	}
	page.MustEval(`()=>{Object.defineProperty(window,'PublicKeyCredential',{value:undefined,configurable:true});renderWebAuthnLogin();}`)
	if !page.MustEval(`()=>document.getElementById('btnPasskey').disabled&&document.getElementById('btnLogin').classList.contains('is-hidden')&&document.getElementById('webauthnHint').textContent===t('webauthn.unsupported')`).Bool() {
		t.Fatal("unsupported browser weakened second-factor policy")
	}

	owner.Value = createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))
	page.MustSetCookies(&proto.NetworkCookieParam{Name: owner.Name, Value: owner.Value, URL: origin, Path: "/", HTTPOnly: true})
	page.MustNavigate(origin + "/fixture/settings").MustWaitLoad()
	page.MustWait(`()=>document.getElementById('auth-webauthn')?.getAttribute('aria-busy')==='false'`)
	page.MustEval(`()=>{const create=navigator.credentials.create.bind(navigator.credentials);navigator.credentials.create=options=>{window.fixtureEnrollmentSignal=options.signal;return create(options);};}`)
	page.MustElement("#wa-label").MustInput("Cancelled on navigation")
	page.MustElement("#auth-webauthn-current-pw").MustInput(stepUpTestPassword)
	finishesBefore := enrollmentFinishes.Load()
	page.MustElement(`[data-wa-action="add"]`).MustClick()
	page.MustWait(`()=>!!window.fixtureEnrollmentSignal`)
	page.MustEval(`()=>document.getElementById('content').replaceChildren()`)
	page.MustWait(`()=>window.fixtureEnrollmentSignal.aborted`)
	page.MustEval(`async()=>{await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);}`)
	if enrollmentFinishes.Load() != finishesBefore {
		t.Fatal("leaving the nested settings panel finished enrollment")
	}

	page.MustNavigate(origin + "/fixture/settings").MustWaitLoad()
	page.MustWait(`()=>document.getElementById('auth-webauthn')?.getAttribute('aria-busy')==='false'`)
	page.MustEval(`()=>AuraConfigState.set('agent.max_tokens',789)`)
	page.MustElement("#auth-webauthn-current-pw").MustInput("incorrect fixture password")
	page.MustElement(`[data-wa-action="settings"]`).MustClick()
	page.MustWait(`()=>document.getElementById('wa-status')?.classList.contains('is-error')&&document.getElementById('auth-webauthn').getAttribute('aria-busy')==='false'`)
	page.MustEval(`async()=>{await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);}`)
	if !page.MustEval(`()=>location.pathname==='/fixture/settings'&&AuraConfigState.get('agent.max_tokens')===789&&document.getElementById('wa-status').textContent===t('webauthn.invalid_credentials')`).Bool() {
		t.Fatal("a failed step-up discarded the draft or navigated away")
	}

	s.CfgMu.Lock()
	s.Cfg.Auth.Enabled = false
	s.CfgMu.Unlock()
	readsBefore := credentialReads.Load()
	page.MustNavigate(origin + "/fixture/settings").MustWaitLoad()
	page.MustWait(`()=>document.getElementById('auth-webauthn')?.getAttribute('aria-disabled')==='true'`)
	if credentialReads.Load() != readsBefore || !page.MustEval(`()=>location.pathname==='/fixture/settings'&&document.querySelector('#auth-webauthn [role="status"]').textContent===t('webauthn.owner_required')`).Bool() {
		t.Fatal("disabled owner auth requested protected key metadata or navigated away")
	}
}

func TestWebAuthnRecoveryLocalesMatchUI(t *testing.T) {
	if len(webAuthnLocales) != 16 {
		t.Fatal("missing recovery locales")
	}
	for locale, messages := range webAuthnLocales {
		data, err := ui.Content.ReadFile("lang/common/" + locale + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var source map[string]string
		if err := json.Unmarshal(data, &source); err != nil {
			t.Fatal(err)
		}
		if len(messages) != len(webAuthnLocales["en"]) {
			t.Fatalf("%s key count", locale)
		}
		for key, english := range webAuthnLocales["en"] {
			value := messages[key]
			if strings.TrimSpace(value) == "" || value != source[key] || locale != "en" && value == english {
				t.Fatalf("%s: missing, stale or untranslated %s", locale, key)
			}
		}
	}
}

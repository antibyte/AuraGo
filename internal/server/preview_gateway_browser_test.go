package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/gorilla/websocket"
)

type previewBrowserRequest struct {
	Path          string
	Cookie        string
	FetchSite     string
	Authorization string
}

type previewBrowserResult struct {
	Done                  bool   `json:"done"`
	RelativeAsset         bool   `json:"relativeAsset"`
	ParentDocumentBlocked bool   `json:"parentDocumentBlocked"`
	ParentStorageBlocked  bool   `json:"parentStorageBlocked"`
	ParentFetchBlocked    bool   `json:"parentFetchBlocked"`
	CookieProbe           string `json:"cookieProbe"`
	UploadStatus          int    `json:"uploadStatus"`
	UploadBody            string `json:"uploadBody"`
	DownloadBody          string `json:"downloadBody"`
	RedirectBody          string `json:"redirectBody"`
	WebSocketEcho         string `json:"webSocketEcho"`
}

// This browser test is opt-in because it launches an installed Chrome or Edge.
// Chrome's test flag blocks ordinary third-party cookies while retaining CHIPS.
func TestPreviewGatewayBrowserIsolationWithBlockedThirdPartyCookies(t *testing.T) {
	browser := previewGatewayBrowser(t)
	var mu sync.Mutex
	guestRequests := make([]previewBrowserRequest, 0, 16)
	mainAPICookies := make([]string, 0, 4)
	mainOrigin := ""
	recordGuest := func(r *http.Request) {
		mu.Lock()
		guestRequests = append(guestRequests, previewBrowserRequest{
			Path: r.URL.RequestURI(), Cookie: r.Header.Get("Cookie"),
			FetchSite: r.Header.Get("Sec-Fetch-Site"), Authorization: r.Header.Get("Authorization"),
		})
		mu.Unlock()
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordGuest(r)
		switch r.URL.Path {
		case "/":
			w.Header().Add("Set-Cookie", "app_login=app-session; Path=/; HttpOnly; SameSite=Lax")
			w.Header().Add("Set-Cookie", sessionCookieName+"=guest-must-not-set-aura-session; Path=/; Secure")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>preview guest</title><script src="assets/relative.js"></script><script src="assets/app.js"></script>`)
		case "/assets/relative.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = fmt.Fprint(w, `window.relativeAssetLoaded=true;`)
		case "/assets/app.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = fmt.Fprintf(w, previewGatewayBrowserScript, mainOrigin)
		case "/api/cookie-probe":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, r.Header.Get("Cookie"))
		case "/api/upload":
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "uploaded:%s", body)
		case "/api/download":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = fmt.Fprint(w, "download-through-preview")
		case "/relative-redirect":
			http.Redirect(w, r, "../redirect-final?from=relative", http.StatusFound)
		case "/redirect-final":
			_, _ = fmt.Fprint(w, "relative redirect reached guest")
		case "/socket":
			conn, err := previewBrowserWebSocketUpgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			kind, body, err := conn.ReadMessage()
			if err == nil {
				_ = conn.WriteMessage(kind, body)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	port := upstream.Listener.Addr().(*net.TCPAddr).Port
	store, _, _ := testInstalledStoreApp(t, "node-red", port)
	s := testDesktopStoreServerWithService(t, store)
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "preview-browser-session-secret-" + t.Name()
	s.Cfg.Server.HTTPS.Domain = "aura.test"
	s.Cfg.Server.PreviewDomain = "preview.test"
	s.Cfg.Server.PreviewEnabled = true
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Directories.DataDir = t.TempDir()
	session := createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))
	main := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/frame":
			http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: session, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.SetCookie(w, &http.Cookie{Name: "aurago_browser_marker", Value: "parent-only", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteNoneMode})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><script>window.previewMessages=[];window.addEventListener('message',e=>window.previewMessages.push({origin:e.origin,data:e.data}));</script><h1>AuraGo parent</h1><iframe id="guest" src="/launch"></iframe>`)
		case "/launch", "/top-launch":
			if cookie, err := r.Cookie(sessionCookieName); err != nil || cookie.Value != session {
				http.Error(w, "session missing", http.StatusUnauthorized)
				return
			}
			launch, err := s.issuePreviewLaunch(r, previewResource{kind: "store", id: "node-red"}, "/")
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			http.Redirect(w, r, launch, http.StatusSeeOther)
		case "/api/private":
			mu.Lock()
			mainAPICookies = append(mainAPICookies, r.Header.Get("Cookie"))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"secret":"parent-only"}`)
		default:
			http.NotFound(w, r)
		}
	})
	gateway := httptest.NewTLSServer(previewHostMiddleware(s, main))
	defer gateway.Close()
	mainOrigin = "https://aura.test:" + strings.TrimPrefix(gateway.URL, "https://127.0.0.1:")
	page := browser.MustPage().Timeout(90 * time.Second)
	defer page.Close()
	page.MustNavigate(mainOrigin + "/frame").MustWaitLoad()
	waitPreviewBrowser(t, page, `()=>window.previewMessages.some(message=>message.data?.test==='preview-app' && message.data.result?.done)`)
	frameRaw := page.MustEval(`()=>JSON.stringify(window.previewMessages.find(message=>message.data?.test==='preview-app'))`).Str()
	var frameEvent struct {
		Origin string `json:"origin"`
		Data   struct {
			Result previewBrowserResult `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(frameRaw), &frameEvent); err != nil {
		t.Fatalf("decode iframe message %q: %v", frameRaw, err)
	}
	assertPreviewBrowserResult(t, frameEvent.Data.Result, true)
	if !strings.Contains(frameEvent.Origin, ".preview.test:") || !strings.HasSuffix(frameEvent.Origin, ":"+strings.TrimPrefix(gateway.URL, "https://127.0.0.1:")) {
		t.Fatalf("iframe guest did not use the mapped HTTPS preview authority: %q", frameEvent.Origin)
	}

	mu.Lock()
	firstRequests := append([]previewBrowserRequest(nil), guestRequests...)
	firstMainAPICookies := append([]string(nil), mainAPICookies...)
	mu.Unlock()
	assertPreviewBrowserRequests(t, firstRequests, true)
	for _, cookie := range firstMainAPICookies {
		if strings.Contains(cookie, "aurago_browser_marker=") || strings.Contains(cookie, sessionCookieName+"=") {
			t.Fatalf("parent AuraGo cookie crossed into the guest's cross-site API fetch: %q", cookie)
		}
	}
	if len(firstMainAPICookies) == 0 {
		t.Fatal("guest did not attempt the parent API isolation probe")
	}

	// A fresh launch gets a separate partition when the preview is opened top-level.
	top := browser.MustPage().Timeout(90 * time.Second)
	defer top.Close()
	top.MustNavigate(mainOrigin + "/top-launch").MustWaitLoad()
	waitPreviewBrowser(t, top, `()=>window.__previewResult?.done`)
	topRaw := top.MustEval(`()=>JSON.stringify(window.__previewResult)`).Str()
	var topResult previewBrowserResult
	if err := json.Unmarshal([]byte(topRaw), &topResult); err != nil {
		t.Fatalf("decode top-level preview result %q: %v", topRaw, err)
	}
	assertPreviewBrowserResult(t, topResult, false)
	mu.Lock()
	allRequests := append([]previewBrowserRequest(nil), guestRequests...)
	topMainAPICookies := append([]string(nil), mainAPICookies[len(firstMainAPICookies):]...)
	mu.Unlock()
	assertPreviewBrowserRequests(t, allRequests, false)
	for _, cookie := range topMainAPICookies {
		if strings.Contains(cookie, "aurago_browser_marker=") || strings.Contains(cookie, sessionCookieName+"=") {
			t.Fatalf("parent AuraGo cookie crossed into the top-level guest's API probe: %q", cookie)
		}
	}
}

const previewGatewayBrowserScript = `(async()=>{
 const report={test:'preview-app',result:{done:false,relativeAsset:!!window.relativeAssetLoaded}};
 try{void parent.document.cookie;report.result.parentDocumentBlocked=false}catch(_){report.result.parentDocumentBlocked=true}
 try{void parent.localStorage;report.result.parentStorageBlocked=false}catch(_){report.result.parentStorageBlocked=true}
 document.cookie='third_party_probe=unpartitioned; Path=/; Secure; SameSite=None';
 report.result.cookieProbe=await(await fetch('/api/cookie-probe',{credentials:'same-origin'})).text();
 const upload=await fetch('/api/upload',{method:'POST',body:'browser-upload',credentials:'same-origin'});
 report.result.uploadStatus=upload.status;report.result.uploadBody=await upload.text();
 report.result.downloadBody=await(await fetch('/api/download',{credentials:'same-origin'})).text();
 report.result.redirectBody=await(await fetch('/relative-redirect',{credentials:'same-origin'})).text();
 try{await fetch(%q+'/api/private',{credentials:'include'});report.result.parentFetchBlocked=false}catch(_){report.result.parentFetchBlocked=true}
 report.result.webSocketEcho=await new Promise(resolve=>{
  let done=false;const finish=value=>{if(!done){done=true;resolve(value)}};
  const socket=new WebSocket('wss://'+location.host+'/socket');socket.binaryType='arraybuffer';
  socket.onopen=()=>socket.send(new Uint8Array([1,2,3]));
  socket.onmessage=e=>{finish(Array.from(new Uint8Array(e.data)).join(','));socket.close()};
  socket.onerror=()=>finish('error');setTimeout(()=>finish('timeout'),7000);
 });
 report.result.done=true;window.__previewResult=report.result;
 if(parent!==window)parent.postMessage(report,'*');
})().catch(error=>{window.__previewResult={done:true,error:String(error)};if(parent!==window)parent.postMessage({test:'preview-app',result:window.__previewResult},'*')});`

func previewGatewayBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to launch Chrome/Edge")
	}
	bin := ""
	for _, path := range []string{`C:\Program Files\Google\Chrome\Application\chrome.exe`, `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, "chromium", "google-chrome"} {
		if resolved, err := exec.LookPath(path); err == nil {
			bin = resolved
			break
		}
	}
	if bin == "" {
		t.Skip("Chrome/Edge unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	rules := "MAP *.preview.test 127.0.0.1,MAP aura.test 127.0.0.1,EXCLUDE localhost"
	controlURL, err := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).
		Set("disable-gpu").Set("ignore-certificate-errors").
		Set("host-resolver-rules", rules).Set("test-third-party-cookie-phaseout", "1").Launch()
	if err != nil {
		t.Fatal(err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = browser.Close() })
	return browser
}

func waitPreviewBrowser(t *testing.T, page *rod.Page, expression string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(expression).Bool() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	body := page.MustEval(`()=>document.body?.innerText||''`).Str()
	state := page.MustEval(`()=>JSON.stringify({url:location.href,messages:window.previewMessages||null,result:window.__previewResult||null})`).Str()
	t.Fatalf("browser condition timed out: %s; body=%q; state=%s", expression, body, state)
}

func assertPreviewBrowserResult(t *testing.T, got previewBrowserResult, expectCHIPS bool) {
	t.Helper()
	if !got.Done || !got.RelativeAsset || !got.ParentFetchBlocked {
		t.Fatalf("preview isolation/relative-asset result: %+v", got)
	}
	if expectCHIPS && (!got.ParentDocumentBlocked || !got.ParentStorageBlocked) {
		t.Fatalf("embedded guest can inspect the AuraGo parent: %+v", got)
	}
	if !expectCHIPS && (got.ParentDocumentBlocked || got.ParentStorageBlocked) {
		t.Fatalf("top-level guest unexpectedly has a cross-origin parent: %+v", got)
	}
	if got.UploadStatus != http.StatusOK || got.UploadBody != "uploaded:browser-upload" || got.DownloadBody != "download-through-preview" || got.RedirectBody != "relative redirect reached guest" || got.WebSocketEcho != "1,2,3" {
		t.Fatalf("preview HTTP/WebSocket flow: %+v", got)
	}
	if expectCHIPS && (!strings.Contains(got.CookieProbe, "app_login=app-session") || strings.Contains(got.CookieProbe, "third_party_probe=")) {
		t.Fatalf("iframe did not retain CHIPS while blocking ordinary third-party cookies: %q", got.CookieProbe)
	}
}

func assertPreviewBrowserRequests(t *testing.T, requests []previewBrowserRequest, requireIframeCookieIsolation bool) {
	t.Helper()
	seen := make(map[string]bool)
	for _, request := range requests {
		path := strings.Split(request.Path, "?")[0]
		seen[path] = true
		if strings.Contains(request.Cookie, previewCookieName+"=") || strings.Contains(request.Cookie, sessionCookieName+"=") || strings.Contains(request.Cookie, "aurago_browser_marker=") {
			t.Errorf("AuraGo credential reached the guest upstream on %s: %q", path, request.Cookie)
		}
		if strings.Contains(request.Authorization, "Bearer ") {
			t.Errorf("AuraGo bearer reached the guest upstream on %s", path)
		}
		if requireIframeCookieIsolation && path == "/api/cookie-probe" && strings.Contains(request.Cookie, "third_party_probe=") {
			t.Errorf("unpartitioned third-party cookie reached guest API: %q", request.Cookie)
		}
	}
	for _, request := range requests {
		path := strings.Split(request.Path, "?")[0]
		if (path == "/api/upload" || path == "/api/download" || path == "/socket") && !strings.Contains(request.Cookie, "app_login=app-session") {
			t.Errorf("guest app login cookie missing from %s: %q", path, request.Cookie)
		}
	}
	for _, path := range []string{"/", "/assets/relative.js", "/assets/app.js", "/api/cookie-probe", "/api/upload", "/api/download", "/relative-redirect", "/redirect-final", "/socket"} {
		if !seen[path] {
			t.Errorf("guest did not request %s; requests=%+v", path, requests)
		}
	}
}

var previewBrowserWebSocketUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

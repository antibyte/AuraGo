package server

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestPreviewGatewayRechecksOwnerAfterDelayedTargetResolution(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var released sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			close(started)
			<-release
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Error("revoked request reached guest")
	}))
	defer upstream.Close()
	defer released.Do(func() { close(release) })
	s, handler, owner, _, _ := testPreviewGateway(t, upstream)
	s.Cfg.VirtualComputers = virtualComputersTestConfig(upstream.URL).VirtualComputers
	s.Cfg.VirtualComputers.ControlPlane.Mode = "local_host"
	launch, err := s.issuePreviewLaunch(owner, previewResource{kind: "vm", id: "vm-1", port: "8080"}, "/")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, launch, nil))
	u, _ := url.Parse(launch)
	request := httptest.NewRequest(http.MethodPost, "https://"+u.Host+"/api", nil)
	request.AddCookie(response.Result().Cookies()[0])
	response = httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); handler.ServeHTTP(response, request) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		released.Do(func() { close(release) })
		t.Fatal("target lookup did not start")
	}
	if !revokeRequestSession(s, owner) {
		t.Fatal("logout failed")
	}
	released.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("revoked request did not finish")
	}
	if response.Code != http.StatusForbidden {
		t.Fatalf("late revoked request = %d", response.Code)
	}
}

func testPreviewGateway(t *testing.T, upstream *httptest.Server) (*Server, http.Handler, *http.Request, string, *http.Cookie) {
	t.Helper()
	port := upstream.Listener.Addr().(*net.TCPAddr).Port
	store, _, _ := testInstalledStoreApp(t, "node-red", port)
	s := testDesktopStoreServerWithService(t, store)
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "preview-test-session-secret-" + t.Name()
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.Server.PreviewDomain = "guest.example.net"
	s.Cfg.Server.HTTPS.Domain = "aurago.example.org"
	r := httptest.NewRequest(http.MethodGet, "https://aurago.example.org/api/desktop/store/apps/node-red/open-url?isolated=1", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	launch, err := s.issuePreviewLaunch(r, previewResource{kind: "store", id: "node-red"}, "/start?app=1")
	if err != nil {
		t.Fatal(err)
	}
	mainCalls := 0
	handler := previewHostMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mainCalls++; http.Error(w, "main", http.StatusTeapot) }))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, launch, nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/start?app=1" {
		t.Fatalf("launch = %d %s", response.Code, response.Body.String())
	}
	if mainCalls != 0 {
		t.Fatal("preview reached main router")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || !cookies[0].Partitioned || cookies[0].Domain != "" || cookies[0].SameSite != http.SameSiteNoneMode {
		t.Fatalf("unsafe preview cookie: %+v", cookies)
	}
	again := httptest.NewRecorder()
	handler.ServeHTTP(again, httptest.NewRequest(http.MethodGet, launch, nil))
	if again.Code != http.StatusUnauthorized {
		t.Fatalf("ticket replay = %d", again.Code)
	}
	u, _ := url.Parse(launch)
	return s, handler, r, u.Host, cookies[0]
}

func TestPreviewGatewayPreservesAppRequestsWithoutAuraGoCredentials(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || string(body) != "uploaded content" || r.URL.RequestURI() != "/api/app?value=1" {
			t.Errorf("app request changed: %s %s %q", r.Method, r.URL.RequestURI(), body)
		}
		if r.Header.Get("Cookie") != "app_login=mine" || r.Header.Get("Authorization") != "Basic app-credential" && r.Header.Get("Authorization") != "Bearer guest-session" || r.Header.Get("X-CSRF-Token") != "app-csrf" {
			t.Errorf("wrong guest credentials: cookies=%q auth=%q", r.Header.Get("Cookie"), r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-AuraGo-Agodesk-Dev-Token") != "" {
			t.Error("AgoDesk gateway credential reached guest")
		}
		w.Header().Add("Set-Cookie", "app_login=new; Domain=.example.org; Path=/; HttpOnly")
		w.Header().Add("Set-Cookie", sessionCookieName+"=evil; Path=/")
		w.Header().Add("Set-Cookie", previewCookieName+"=evil; Path=/; Secure")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	s, handler, _, host, cookie := testPreviewGateway(t, upstream)
	request := httptest.NewRequest(http.MethodPost, "https://"+host+"/api/app?value=1", strings.NewReader("uploaded content"))
	request.AddCookie(cookie)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "never-forward"})
	request.AddCookie(&http.Cookie{Name: "app_login", Value: "mine"})
	request.Header.Set("Authorization", "Basic app-credential")
	request.Header.Set("X-CSRF-Token", "app-csrf")
	request.Header.Set("X-AuraGo-Agodesk-Dev-Token", "private-development-credential")
	request.Header.Set("Origin", "https://"+host)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"ok":true}` {
		t.Fatalf("guest request = %d %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "app_login" || cookies[0].Domain != "" || !cookies[0].Partitioned || !cookies[0].HttpOnly {
		t.Fatalf("guest cookies = %+v", cookies)
	}
	guestBearer := request.Clone(request.Context())
	guestBearer.Header.Set("Authorization", "Bearer guest-session")
	guestBearer.Body = io.NopCloser(strings.NewReader("uploaded content"))
	bearerResponse := httptest.NewRecorder()
	handler.ServeHTTP(bearerResponse, guestBearer)
	if bearerResponse.Code != http.StatusOK {
		t.Fatalf("guest bearer rejected: %d", bearerResponse.Code)
	}
	wrongHost := request.Clone(request.Context())
	wrongHost.Host = "app-other.guest.example.net"
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, wrongHost)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("cross-resource grant accepted: %d", denied.Code)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	denied = httptest.NewRecorder()
	handler.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("readonly write = %d", denied.Code)
	}
}

func TestPreviewGatewayWebSocketClosesAfterOwnerLogout(t *testing.T) {
	upstreamClosed := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		defer close(upstreamClosed)
		for {
			kind, body, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(kind, body); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	s, handler, owner, host, cookie := testPreviewGateway(t, upstream)
	gateway := httptest.NewTLSServer(handler)
	defer gateway.Close()
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // httptest certificate only
	headers := http.Header{"Host": []string{host}, "Cookie": []string{cookie.String()}, "Origin": []string{"https://" + host}}
	conn, _, err := dialer.Dial("wss"+strings.TrimPrefix(gateway.URL, "https")+"/socket", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	_, body, err := conn.ReadMessage()
	if err != nil || string(body) != string([]byte{1, 2, 3}) {
		t.Fatalf("websocket echo: %v %v", body, err)
	}
	if !revokeRequestSession(s, owner) {
		t.Fatal("logout failed")
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("logout left socket open")
	}
	select {
	case <-upstreamClosed:
	case <-time.After(time.Second):
		t.Fatal("logout left upstream socket blocked")
	}
}

func TestPreviewGatewayDomainAndStagingAdmission(t *testing.T) {
	for _, domain := range []string{"example.org", "apps.example.org", "org", "bad/path", "https://guest.net"} {
		if validatePreviewDomain(domain, "aurago.example.org") == nil {
			t.Errorf("unsafe domain accepted: %s", domain)
		}
	}
	if err := validatePreviewDomain("guests.example.net", "aurago.example.org"); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("app")) }))
	defer upstream.Close()
	s, handler, owner, host, cookie := testPreviewGateway(t, upstream)
	if s.previewRequested(httptest.NewRequest("GET", "https://aurago.example.org/", nil)) {
		t.Fatal("staging activated by domain alone")
	}
	main := httptest.NewRecorder()
	handler.ServeHTTP(main, httptest.NewRequest("GET", "https://aurago.example.org/", nil))
	if main.Code != http.StatusTeapot {
		t.Fatal("main request intercepted")
	}
	guest := httptest.NewRequest("GET", "https://"+host+"/", nil)
	guest.AddCookie(cookie)
	if !revokeRequestSession(s, owner) {
		t.Fatal("logout failed")
	}
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, guest)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("revoked session allowed: %d", denied.Code)
	}
	owner.Header.Del("Cookie")
	owner.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(2*time.Hour))})
	if _, err := s.issuePreviewLaunch(owner, previewResource{kind: "store", id: "node-red"}, "/"); err != nil {
		t.Fatalf("other session revoked: %v", err)
	}
}

func TestPreviewGatewayInvalidConfigurationPreservesMainRouter(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	s, handler, owner, _, _ := testPreviewGateway(t, upstream)
	for _, tc := range []struct{ name, primary, domain string }{
		{"same host", "aurago.example.org", "aurago.example.org"},
		{"shared site", "aurago.example.org", "example.org"},
		{"missing primary", "", "guest.example.net"},
		{"IP primary", "127.0.0.1", "guest.example.net"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.Cfg.Server.HTTPS.Domain = tc.primary
			s.Cfg.Server.Host = "127.0.0.1"
			s.Cfg.Server.PreviewDomain = tc.domain
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, owner)
			if response.Code != http.StatusTeapot {
				t.Fatalf("bad configuration intercepted main site: %d", response.Code)
			}
			if _, err := s.issuePreviewLaunch(owner, previewResource{kind: "store", id: "node-red"}, "/"); err == nil {
				t.Fatal("bad configuration issued launch")
			}
		})
	}
	s.Cfg.Server.HTTPS.Domain = ""
	s.Cfg.Server.Host = "aurago.example.org"
	if _, err := s.issuePreviewLaunch(owner, previewResource{kind: "store", id: "node-red"}, "/"); err != nil {
		t.Fatalf("valid DNS server.host fallback rejected: %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://unknown.guest.example.net/", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unknown preview host reached main site: %d", response.Code)
	}
}

func TestPreviewGatewayStripsAuraGoTokensRegardlessOfScope(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("AuraGo credential reached guest")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	s, handler, _, host, cookie := testPreviewGateway(t, upstream)
	tokens, _, _ := testDesktopPermissionServer(t)
	s.TokenManager = tokens.TokenManager
	for _, scope := range []string{desktopScopeAdmin, "webhook"} {
		raw, _, err := s.TokenManager.Create("preview stripping test", []string{scope}, nil)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "https://"+host+"/", nil)
		request.AddCookie(cookie)
		request.Header.Set("Authorization", "Bearer "+raw)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("request with %s token = %d", scope, response.Code)
		}
	}
}

func TestPreviewGatewayVMPreservesAppPathAndGuestBearer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.RequestURI() != "/v1/machines/vm-1/web/8080/api/app?q=guest" {
			t.Errorf("unexpected VM app target: %s", r.URL.RequestURI())
		}
		if r.Header.Get("Authorization") != "Bearer guest-session" {
			t.Error("guest authentication changed or management bearer leaked")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	s, handler, owner, _, _ := testPreviewGateway(t, upstream)
	s.Cfg.VirtualComputers = virtualComputersTestConfig(upstream.URL).VirtualComputers
	launch, err := s.issuePreviewLaunch(owner, previewResource{kind: "vm", id: "vm-1", port: "8080"}, "/")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, launch, nil))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("VM launch failed: %d", response.Code)
	}
	u, _ := url.Parse(launch)
	request := httptest.NewRequest(http.MethodPost, "https://"+u.Host+"/api/app?q=guest", nil)
	request.AddCookie(response.Result().Cookies()[0])
	request.Header.Set("Authorization", "Bearer guest-session")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("VM app request failed: %d %s", response.Code, response.Body.String())
	}
}

func TestPreviewGatewayVMLaunchPreservesEscapedPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	s, handler, owner, _, _ := testPreviewGateway(t, upstream)
	s.Cfg.VirtualComputers.Enabled = true
	owner.URL.RawQuery = "isolated=1&app=value"
	response := httptest.NewRecorder()
	if !serveIsolatedVMPreviewLaunch(s, response, owner, "vm-1", 8080, "file?#.html") {
		t.Fatal("isolated launch not handled")
	}
	launch := response.Header().Get("Location")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, launch, nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/file%3F%23.html?app=value" {
		t.Fatalf("guest path/query changed: status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
}

func TestPreviewGatewayNativeDockerLoopbackTargets(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	s, _, _, _, _ := testPreviewGateway(t, upstream)
	for _, host := range []string{"tcp://127.0.0.1:2375", "tcp://[::1]:2375", "tcp://localhost:2375"} {
		s.Cfg.Docker.Host = host
		target, err := s.previewTarget(t.Context(), previewResource{kind: "store", id: "node-red"})
		if err != nil || target.String() != upstream.URL+"/" {
			t.Fatalf("local Docker %q target=%v err=%v", host, target, err)
		}
	}
	s.Cfg.Docker.Host = "tcp://docker.internal:2375"
	if _, err := s.previewTarget(t.Context(), previewResource{kind: "store", id: "node-red"}); err == nil {
		t.Fatal("remote Docker accepted unreachable loopback app binding")
	}
}

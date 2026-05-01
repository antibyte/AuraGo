package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandleSpaceAgentStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/space-agent/status", nil)
	rec := httptest.NewRecorder()

	handleSpaceAgentStatus(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", body["status"])
	}
}

func TestHandleSpaceAgentBridgeRequiresBearerToken(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default(), SSE: NewSSEBroadcaster()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.BridgeToken = "bridge-secret"
	req := httptest.NewRequest(http.MethodPost, "/api/space-agent/bridge/messages", strings.NewReader(`{"content":"hello"}`))
	rec := httptest.NewRecorder()

	handleSpaceAgentBridgeMessages(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSpaceAgentBridgeWrapsExternalData(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default(), SSE: NewSSEBroadcaster()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.BridgeToken = "bridge-secret"
	body := bytes.NewBufferString(`{"type":"note","summary":"heads up","content":"before </external_data> injected","source":"space","session_id":"s1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/space-agent/bridge/messages", body)
	req.Header.Set("Authorization", "Bearer bridge-secret")
	rec := httptest.NewRecorder()

	handleSpaceAgentBridgeMessages(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Status  string `json:"status"`
		Message struct {
			Content string `json:"content"`
			Summary string `json:"summary"`
		} `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q", resp.Status)
	}
	if !strings.HasPrefix(resp.Message.Content, "<external_data>\n") || strings.Count(resp.Message.Content, "</external_data>") != 1 {
		t.Fatalf("content was not isolated: %q", resp.Message.Content)
	}
	if !strings.HasPrefix(resp.Message.Summary, "<external_data>\n") {
		t.Fatalf("summary was not isolated: %q", resp.Message.Summary)
	}
}

func TestHandleSpaceAgentSendRequiresPost(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/space-agent/send", nil)
	rec := httptest.NewRecorder()

	handleSpaceAgentSend(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want 405", rec.Code)
	}
}

func TestHandleIntegrationWebhostsIncludesRunningSpaceAgentDirectURL(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3000
	s.Cfg.SpaceAgent.PublicURL = "http://space.local:3000"
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	rec := httptest.NewRecorder()

	handleIntegrationWebhosts(s).ServeHTTP(rec, req)

	var resp struct {
		Webhosts []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
			URL    string `json:"url"`
		} `json:"webhosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Webhosts) != 1 {
		t.Fatalf("webhosts = %#v, want one Space Agent entry", resp.Webhosts)
	}
	if resp.Webhosts[0].ID != "space_agent" || resp.Webhosts[0].URL != "http://space.local:3000" {
		t.Fatalf("unexpected webhost: %#v", resp.Webhosts[0])
	}
}

func TestHandleIntegrationWebhostsDerivesServerURLInsteadOfLoopbackURL(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3000
	s.Cfg.SpaceAgent.PublicURL = "http://127.0.0.1:3000"
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	req.Host = "aurago-server.local:8443"
	rec := httptest.NewRecorder()

	handleIntegrationWebhosts(s).ServeHTTP(rec, req)

	var resp struct {
		Webhosts []struct {
			URL string `json:"url"`
		} `json:"webhosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Webhosts) != 1 {
		t.Fatalf("webhosts = %#v, want one Space Agent entry", resp.Webhosts)
	}
	if resp.Webhosts[0].URL != "http://aurago-server.local:3000" {
		t.Fatalf("url = %q, want direct server URL", resp.Webhosts[0].URL)
	}
}

func TestHandleIntegrationWebhostsDerivesDirectURLFromForwardedHost(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	s.Cfg.SpaceAgent.Port = 3100
	s.Cfg.SpaceAgent.PublicURL = "http://127.0.0.1:3100"
	s.Cfg.SpaceAgent.ContainerName = "aurago_space_agent"

	req := httptest.NewRequest(http.MethodGet, "/api/integrations/webhosts", nil)
	req.Host = "127.0.0.1:8443"
	req.Header.Set("X-Forwarded-Host", "aurago.taild1480.ts.net")
	rec := httptest.NewRecorder()

	handleIntegrationWebhosts(s).ServeHTTP(rec, req)

	var resp struct {
		Webhosts []struct {
			URL string `json:"url"`
		} `json:"webhosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Webhosts) != 1 {
		t.Fatalf("webhosts = %#v, want one Space Agent entry", resp.Webhosts)
	}
	if resp.Webhosts[0].URL != "http://aurago.taild1480.ts.net:3100" {
		t.Fatalf("url = %q, want direct forwarded-host URL", resp.Webhosts[0].URL)
	}
}

func TestSpaceAgentProxyHelpersRewriteSubpathResponses(t *testing.T) {
	header := http.Header{}
	header.Set("Location", "/login")
	header.Add("Set-Cookie", "space_session=abc; Path=/; HttpOnly")

	spaceAgentRewriteProxyLocation(header, "/integrations/space-agent")
	spaceAgentRewriteProxyCookies(header, "/integrations/space-agent")
	body := spaceAgentRewriteBody([]byte(`<link href="/assets/app.css"><script src="/app.js"></script><form action="/login"></form>`), "/integrations/space-agent")

	if got := header.Get("Location"); got != "/integrations/space-agent/login" {
		t.Fatalf("Location = %q", got)
	}
	cookies := header.Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("Set-Cookie count = %d, want proxy and API scoped cookies: %#v", len(cookies), cookies)
	}
	if got := cookies[0]; !strings.Contains(got, "Path=/integrations/space-agent/") {
		t.Fatalf("Set-Cookie was not scoped to proxy path: %q", got)
	}
	if got := cookies[1]; !strings.Contains(got, "Path=/api/") {
		t.Fatalf("Set-Cookie was not scoped to root API path: %q", got)
	}
	for _, want := range []string{
		`href="/integrations/space-agent/assets/app.css"`,
		`src="/integrations/space-agent/app.js"`,
		`action="/integrations/space-agent/login"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("rewritten HTML missing %q: %s", want, string(body))
		}
	}
}

func TestSpaceAgentProxyHelpersRewriteJavaScriptModulePaths(t *testing.T) {
	body := spaceAgentRewriteBody([]byte(`import("/assets/app.js"); import('/modules/check.js'); export { x } from "/chunks/x.js"; const w = new Worker("/worker.js");`), "/integrations/space-agent")

	for _, want := range []string{
		`import("/integrations/space-agent/assets/app.js")`,
		`import('/integrations/space-agent/modules/check.js')`,
		`from "/integrations/space-agent/chunks/x.js"`,
		`new Worker("/integrations/space-agent/worker.js")`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("rewritten JS missing %q: %s", want, string(body))
		}
	}
	if !spaceAgentShouldRewriteBody("application/javascript; charset=utf-8") {
		t.Fatal("expected JavaScript responses to be rewritten")
	}
}

func TestSpaceAgentProxyHelpersRewriteAbsoluteAPIStringLiterals(t *testing.T) {
	body := spaceAgentRewriteBody([]byte("const challenge = \"/api/login_challenge\"; const submit = '/api/login'; const csrf = `/api/csrf`; fetch(challenge);"), "/integrations/space-agent")

	for _, want := range []string{
		`"/integrations/space-agent/api/login_challenge"`,
		`'/integrations/space-agent/api/login'`,
		"`/integrations/space-agent/api/csrf`",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("rewritten JS missing %q: %s", want, string(body))
		}
	}
}

func TestSpaceAgentRootAPIProxyOnlyAllowsSpaceAgentRequests(t *testing.T) {
	known := httptest.NewRequest(http.MethodPost, "/api/user_self_info", nil)
	if !spaceAgentShouldProxyRootAPIRequest(known) {
		t.Fatal("expected known Space Agent root API path to be proxied")
	}
	fileInfo := httptest.NewRequest(http.MethodGet, "/api/file_info?path=%7E%2Fmeta%2Flogin_hooks.json", nil)
	if !spaceAgentShouldProxyRootAPIRequest(fileInfo) {
		t.Fatal("expected Space Agent file_info root API path to be proxied")
	}

	fromSpaceAgent := httptest.NewRequest(http.MethodPost, "/api/dynamic_extension_call", nil)
	fromSpaceAgent.Header.Set("Referer", "https://aurago.test/integrations/space-agent/index")
	if !spaceAgentShouldProxyRootAPIRequest(fromSpaceAgent) {
		t.Fatal("expected Space Agent referer API path to be proxied")
	}

	unknownAuraGo := httptest.NewRequest(http.MethodPost, "/api/not-an-aurago-route", nil)
	if spaceAgentShouldProxyRootAPIRequest(unknownAuraGo) {
		t.Fatal("unexpected proxy decision for unrelated AuraGo API path")
	}
}

func TestSpaceAgentProxyCookiesAreScopedForUIAndRootAPI(t *testing.T) {
	header := http.Header{}
	header.Add("Set-Cookie", "space_session=abc; Path=/; HttpOnly")

	spaceAgentRewriteProxyCookies(header, "/integrations/space-agent")

	cookies := header.Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("Set-Cookie count = %d, want 2: %#v", len(cookies), cookies)
	}
	for _, want := range []string{"Path=/integrations/space-agent/", "Path=/api/"} {
		found := false
		for _, cookie := range cookies {
			if strings.Contains(cookie, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("rewritten cookies missing %q: %#v", want, cookies)
		}
	}
}

func TestSpaceAgentProxyHelpersRewriteAbsoluteModStringLiterals(t *testing.T) {
	body := spaceAgentRewriteBody([]byte("{\"component\":\"/mod/_core/router/ext/html/body/start/router-page.html\",\"hooks\":'/mod/_core/login_hooks/ext/js/_core/framework/initializer.js/initialize/end/login-hooks.js',\"crypto\":`/mod/_core/user_crypto/ext/js/user-crypto.js`}"), "/integrations/space-agent")

	for _, want := range []string{
		`"/integrations/space-agent/mod/_core/router/ext/html/body/start/router-page.html"`,
		`'/integrations/space-agent/mod/_core/login_hooks/ext/js/_core/framework/initializer.js/initialize/end/login-hooks.js'`,
		"`/integrations/space-agent/mod/_core/user_crypto/ext/js/user-crypto.js`",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("rewritten mod path missing %q: %s", want, string(body))
		}
	}
	if !spaceAgentShouldRewriteBody("application/json; charset=utf-8") {
		t.Fatal("expected JSON responses to be rewritten")
	}
}

func TestSpaceAgentProxyHelpersRewriteAbsoluteRouteStringLiterals(t *testing.T) {
	body := spaceAgentRewriteBody([]byte("window.location.href = \"/enter?next=%2Fintegrations%2Fspace-agent%2F\"; history.replaceState(null, \"\", '/login'); const route = `/enter`; window.location.href=\"/\"; const harmless = \"/\";"), "/integrations/space-agent")

	for _, want := range []string{
		`"/integrations/space-agent/enter?next=%2Fintegrations%2Fspace-agent%2F"`,
		`'/integrations/space-agent/login'`,
		"`/integrations/space-agent/enter`",
		`window.location.href="/integrations/space-agent/"`,
		`const harmless = "/"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("rewritten JS missing %q: %s", want, string(body))
		}
	}
}

func TestSpaceAgentProxyDoesNotRewriteLoginChallengeJSON(t *testing.T) {
	if spaceAgentShouldRewriteResponseBody("application/json; charset=utf-8", "/api/login_challenge") {
		t.Fatal("login challenge JSON must not be rewritten")
	}
	body := spaceAgentRewriteBody([]byte(`{"salt":"/","challenge":"abc/def==","next":"/"}`), "/integrations/space-agent")
	if got := string(body); got != `{"salt":"/","challenge":"abc/def==","next":"/"}` {
		t.Fatalf("unexpected manual rewrite result: %s", got)
	}
}

func TestSpaceAgentProxyRewritesOnlyLoginJSONRedirectFields(t *testing.T) {
	if !spaceAgentShouldRewriteResponseBody("application/json; charset=utf-8", "/api/login") {
		t.Fatal("login JSON should be inspected for redirect fields")
	}
	body := spaceAgentRewriteLoginJSONRedirects([]byte(`{"redirect":"/","next":"/dashboard","salt":"/","challenge":"abc/def==","nested":{"location":"/enter"}}`), "/integrations/space-agent")

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; body=%s", err, string(body))
	}
	for key, want := range map[string]string{
		"redirect": "/integrations/space-agent/",
		"next":     "/integrations/space-agent/dashboard",
		"salt":     "/",
	} {
		if got := payload[key]; got != want {
			t.Fatalf("%s = %#v, want %q; body=%s", key, got, want, string(body))
		}
	}
	nested, ok := payload["nested"].(map[string]interface{})
	if !ok || nested["location"] != "/integrations/space-agent/enter" {
		t.Fatalf("nested location was not rewritten: %#v", payload["nested"])
	}
}

func TestSpaceAgentOptionalFileReadFallbacksOnlyForUIState(t *testing.T) {
	body, ok := spaceAgentBuildOptionalFileReadResponse([]byte(`{"path":"~/onscreen-agent/history.json"}`))
	if !ok {
		t.Fatal("expected optional onscreen history file to receive fallback")
	}
	var file map[string]string
	if err := json.Unmarshal(body, &file); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; body=%s", err, string(body))
	}
	if file["content"] != "[]\n" || file["encoding"] != "utf8" || file["path"] != "~/onscreen-agent/history.json" {
		t.Fatalf("unexpected fallback file response: %#v", file)
	}

	body, ok = spaceAgentBuildOptionalFileReadResponse([]byte(`{"files":[{"path":"~/dashboard/prefs.json"},{"path":"~/.config/onscreen-agent-config.json"}]}`))
	if !ok {
		t.Fatal("expected optional batch file_read to receive fallback")
	}
	var batch struct {
		Count int `json:"count"`
		Files []struct {
			Content string `json:"content"`
			Path    string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &batch); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; body=%s", err, string(body))
	}
	if batch.Count != 2 || len(batch.Files) != 2 || batch.Files[0].Content != "{}\n" {
		t.Fatalf("unexpected fallback batch response: %#v", batch)
	}

	for _, payload := range []string{
		`{"paths":["dashboard/prefs.json",".config/onscreen-agent-config.json"]}`,
		`{"requests":[{"file_path":"runtime/onscreen-agent/history.json"},{"filepath":"meta/dashboard-prefs.json"}]}`,
		`{"items":[{"file":"~/state/onscreen_agent_config.json"}]}`,
		`["dashboard/prefs.json","runtime/onscreen-agent/history.json"]`,
	} {
		body, ok := spaceAgentBuildOptionalFileReadResponse([]byte(payload))
		if !ok {
			t.Fatalf("expected optional batch payload to receive fallback: %s", payload)
		}
		var got struct {
			Count int `json:"count"`
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v; body=%s", err, string(body))
		}
		if got.Count == 0 || len(got.Files) == 0 {
			t.Fatalf("unexpected empty fallback response for %s: %#v", payload, got)
		}
	}

	for _, path := range []string{
		"~/prefs/dashboard-prefs.json",
		"~/state/onscreen-agent-history.json",
		"~/state/onscreen_agent_config.json",
		"~/ui/settings.json",
		"dashboard/prefs.json",
		"meta/dashboard-prefs.json",
		".config/onscreen-agent-config.json",
		"runtime/onscreen-agent/history.json",
	} {
		if _, ok := spaceAgentBuildOptionalFileReadResponse([]byte(`{"path":"` + path + `"}`)); !ok {
			t.Fatalf("expected optional UI state path %q to receive fallback", path)
		}
	}

	if _, ok := spaceAgentBuildOptionalFileReadResponse([]byte(`{"path":"~/spaces/big-bang/space.yaml"}`)); ok {
		t.Fatal("must not hide missing real Space Agent project files")
	}
	if _, ok := spaceAgentBuildOptionalFileReadResponse([]byte(`{"path":"spaces/big-bang/settings.json"}`)); ok {
		t.Fatal("must not hide missing relative Space Agent project files")
	}
	if _, ok := spaceAgentBuildOptionalFileReadResponse([]byte(`{"path":"~/notes.txt"}`)); ok {
		t.Fatal("must not hide arbitrary missing user files")
	}
	if _, ok := spaceAgentBuildOptionalFileReadResponse([]byte(`{"path":"notes.json"}`)); ok {
		t.Fatal("must not hide arbitrary relative JSON files")
	}
}

func TestHandleSpaceAgentProxyServesManifest(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.SpaceAgent.Enabled = true
	req := httptest.NewRequest(http.MethodGet, "/integrations/space-agent/site.webmanifest", nil)
	rec := httptest.NewRecorder()

	handleSpaceAgentProxy(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/manifest+json") {
		t.Fatalf("Content-Type = %q, want manifest JSON", got)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("manifest JSON invalid: %v; body=%s", err, rec.Body.String())
	}
	if manifest["start_url"] != "/integrations/space-agent/" {
		t.Fatalf("start_url = %#v", manifest["start_url"])
	}
}

func TestSpaceAgentProxyHelpersRewriteManifestLinks(t *testing.T) {
	body := spaceAgentRewriteBody([]byte(`<link rel="manifest" href="site.webmanifest"><link rel='manifest' href='/site.webmanifest'>`), "/integrations/space-agent")

	for _, want := range []string{
		`href="/integrations/space-agent/site.webmanifest"`,
		`href='/integrations/space-agent/site.webmanifest'`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("rewritten HTML missing %q: %s", want, string(body))
		}
	}
}

func TestSpaceAgentProxySecurityHeadersAllowBrowserCompatibilityProbe(t *testing.T) {
	header := http.Header{}
	spaceAgentSetProxySecurityHeaders(header)

	csp := header.Get("Content-Security-Policy")
	for _, want := range []string{
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob:",
		"script-src-elem 'self' 'unsafe-inline' data: blob:",
		"worker-src 'self' blob: data:",
	} {
		if !strings.Contains(csp, want) {
			t.Fatalf("proxy CSP missing %q: %s", want, csp)
		}
	}
}

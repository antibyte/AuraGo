package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/evomap"
	"aurago/internal/testutil"
)

// Config UX wave 1, batch 1: the test buttons of these integrations send the
// unsaved form values as {"draft": ...}. Each test saves nothing, leaves the saved
// section empty and checks that the upstream is reached with the draft values.

// draftProbe records the requests an upstream received.
type draftProbe struct {
	mu      sync.Mutex
	auth    []string
	user    []string
	pass    []string
	hits    int
	handler func(w http.ResponseWriter, r *http.Request)
}

func (p *draftProbe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.hits++
	p.auth = append(p.auth, r.Header.Get("Authorization"))
	user, pass, _ := r.BasicAuth()
	p.user = append(p.user, user)
	p.pass = append(p.pass, pass)
	p.mu.Unlock()
	if p.handler != nil {
		p.handler(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func (p *draftProbe) sawBasic(user, pass string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.user {
		if p.user[i] == user && p.pass[i] == pass {
			return true
		}
	}
	return false
}

func (p *draftProbe) sawAuthorization(value string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, got := range p.auth {
		if got == value {
			return true
		}
	}
	return false
}

func newDraftProbe(t *testing.T) (*draftProbe, *httptest.Server) {
	t.Helper()
	probe := &draftProbe{}
	srv := testutil.NewHTTPServer(t, probe)
	t.Cleanup(srv.Close)
	return probe, srv
}

func TestAdGuardTestUsesDraftValues(t *testing.T) {
	s := newDraftTestServer(t)
	probe, srv := newDraftProbe(t)
	body := fmt.Sprintf(`{"draft":{"adguard":{"url":%q,"username":"draft-user","password":"draft-pass"}}}`, srv.URL)
	rec := postDraft(handleAdGuardTest(s), "/api/adguard/test", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !probe.sawBasic("draft-user", "draft-pass") {
		t.Fatalf("upstream did not see the draft credentials (hits=%d)", probe.hits)
	}
	if got := s.ConfigSnapshot().AdGuard.URL; got != "" {
		t.Fatalf("saved snapshot was changed: %q", got)
	}
}

func TestAdGuardTestWithoutDraftReportsMissingURL(t *testing.T) {
	s := newDraftTestServer(t)
	rec := postDraft(handleAdGuardTest(s), "/api/adguard/test", "")
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestGrafanaTestUsesDraftValues(t *testing.T) {
	s := newDraftTestServer(t)
	probe, srv := newDraftProbe(t)
	body := fmt.Sprintf(`{"draft":{"grafana":{"base_url":%q,"api_key":"draft-grafana-key"}}}`, srv.URL)
	rec := postDraft(handleGrafanaTest(s), "/api/grafana/test", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !probe.sawAuthorization("Bearer draft-grafana-key") {
		t.Fatalf("upstream did not see the draft API key (hits=%d, auth=%v)", probe.hits, probe.auth)
	}
}

func TestUptimeKumaTestUsesDraftValues(t *testing.T) {
	s := newDraftTestServer(t)
	probe, srv := newDraftProbe(t)
	probe.handler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testUptimeKumaMetrics)
	}
	body := fmt.Sprintf(`{"draft":{"uptime_kuma":{"base_url":%q,"api_key":"draft-uk-key"}}}`, srv.URL)
	rec := postDraft(handleUptimeKumaTest(s), "/api/uptime-kuma/test", body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !probe.sawBasic("", "draft-uk-key") {
		t.Fatalf("upstream did not see the draft API key (user=%v pass=%v)", probe.user, probe.pass)
	}
}

func TestKoofrTestUsesDraftValues(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	s := newDraftTestServer(t)
	probe, srv := newDraftProbe(t)
	body := fmt.Sprintf(`{"draft":{"koofr":{"enabled":true,"base_url":%q,"username":"draft@example.com","app_password":"draft-app-pass"}}}`, srv.URL)
	rec := postDraft(handleKoofrTest(s), "/api/koofr/test", body)
	if probe.hits == 0 {
		t.Fatalf("upstream not reached; status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !probe.sawBasic("draft@example.com", "draft-app-pass") {
		t.Fatalf("upstream did not see the draft credentials (user=%v)", probe.user)
	}
	if strings.Contains(rec.Body.String(), "no_credentials") || strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("draft credentials were not used: %s", rec.Body.String())
	}
}

func TestWebDAVTestUsesDraftValues(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	s := newDraftTestServer(t)
	probe, srv := newDraftProbe(t)
	probe.handler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:"><d:response><d:href>/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
	}
	body := fmt.Sprintf(`{"draft":{"webdav":{"enabled":true,"auth_type":"basic","url":%q,"username":"draft-dav","password":"draft-dav-pass"}}}`, srv.URL)
	rec := postDraft(handleWebDAVTest(s), "/api/webdav/test", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !probe.sawBasic("draft-dav", "draft-dav-pass") {
		t.Fatalf("upstream did not see the draft credentials (user=%v)", probe.user)
	}
}

func TestAgentMailTestUsesDraftValues(t *testing.T) {
	s := newDraftTestServer(t)
	probe, srv := newDraftProbe(t)
	probe.handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"inboxes":[],"count":0}`)
	}
	body := fmt.Sprintf(`{"draft":{"agentmail":{"base_url":%q,"api_key":"draft-am-key"}}}`, srv.URL)
	rec := postDraft(handleAgentMailTest(s), "/api/agentmail/test", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !probe.sawAuthorization("Bearer draft-am-key") {
		t.Fatalf("upstream did not see the draft API key (hits=%d auth=%v) body=%s", probe.hits, probe.auth, rec.Body.String())
	}
}

type fakeEvomapClient struct{}

func (fakeEvomapClient) Status(context.Context) (evomap.StatusResult, error) {
	return evomap.StatusResult{Raw: []byte(`{"ok":true}`)}, nil
}

func (fakeEvomapClient) RegisterNode(context.Context, evomap.RegisterRequest) (evomap.RegisterResponse, error) {
	return evomap.RegisterResponse{}, nil
}

func TestEvomapTestUsesDraftValues(t *testing.T) {
	s := newDraftTestServer(t)
	var got config.EvomapConfig
	previous := newEvomapServerClient
	newEvomapServerClient = func(cfg config.EvomapConfig) (evomapServerClient, error) {
		got = cfg
		return fakeEvomapClient{}, nil
	}
	t.Cleanup(func() { newEvomapServerClient = previous })

	body := `{"draft":{"evomap":{"enabled":true,"base_url":"https://draft.evomap.test","node_id":"node_draft","node_secret":"draft-node-secret","api_key":"draft-evo-key"}}}`
	rec := postDraft(handleEvomapTest(s), "/api/evomap/test", body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if got.BaseURL != "https://draft.evomap.test" || got.NodeID != "node_draft" || got.NodeSecret != "draft-node-secret" || got.APIKey != "draft-evo-key" {
		t.Fatalf("client built from %+v, want the draft values", got)
	}
}

func TestTelnyxTestUsesDraftEnabledFlag(t *testing.T) {
	// The saved config is enabled with a key, so only the draft can produce "disabled"
	// (and the handler must not dial Telnyx).
	s := newDraftTestServer(t)
	next := s.ConfigSnapshot().Clone()
	next.Telnyx.Enabled = true
	next.Telnyx.APIKey = "saved-telnyx-key"
	s.replaceConfigSnapshot(next)
	rec := postDraft(handleTelnyxTest(s), "/api/telnyx/test", `{"draft":{"telnyx":{"enabled":false}}}`)
	if !strings.Contains(rec.Body.String(), `"disabled"`) {
		t.Fatalf("draft enabled=false was ignored: %s", rec.Body.String())
	}
}

func TestTelnyxTestDraftWithoutKeyReportsMissingCredentials(t *testing.T) {
	s := newDraftTestServer(t)
	rec := postDraft(handleTelnyxTest(s), "/api/telnyx/test", `{"draft":{"telnyx":{"enabled":true}}}`)
	if !strings.Contains(rec.Body.String(), "no_credentials") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestMQTTTestUsesDraftBroker(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		close(accepted)
		defer conn.Close()
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf) // hold the socket without sending CONNACK
		time.Sleep(time.Second)
	}()

	s := newDraftTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	body := fmt.Sprintf(`{"draft":{"mqtt":{"enabled":true,"broker":"tcp://%s","password":"draft-mqtt-pass"}}}`, listener.Addr().String())
	req := draftRequest(body).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleMQTTTest(s).ServeHTTP(rec, req)
	select {
	case <-accepted:
	default:
		t.Fatalf("the draft broker was not dialed; body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "draft-mqtt-pass") {
		t.Fatalf("response leaked the draft password: %s", rec.Body.String())
	}
	if s.ConfigSnapshot().MQTT.Broker != "" {
		t.Fatal("saved snapshot was changed")
	}
}

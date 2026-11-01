package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/whatsapp"
)

type fakeWhatsAppController struct {
	linkReq   whatsapp.LinkRequest
	linkForce bool
	linkErr   error
	snapshot  whatsapp.LinkSnapshot
	logout    whatsapp.LogoutResult
	logoutErr error
	tests     int
	testErr   error
}

func (f *fakeWhatsAppController) Link(_ context.Context, req whatsapp.LinkRequest, force bool) (whatsapp.LinkSnapshot, error) {
	f.linkReq, f.linkForce = req, force
	if f.linkErr != nil {
		return whatsapp.LinkSnapshot{}, f.linkErr
	}
	return whatsapp.LinkSnapshot{State: whatsapp.LinkStateWaiting}, nil
}

func (f *fakeWhatsAppController) LinkSnapshot() whatsapp.LinkSnapshot { return f.snapshot }

func (f *fakeWhatsAppController) Logout(context.Context) (whatsapp.LogoutResult, error) {
	return f.logout, f.logoutErr
}

func (f *fakeWhatsAppController) SendTest(context.Context, time.Time) error {
	f.tests++
	return f.testErr
}

func whatsAppTestServer(ctl whatsAppController) *Server {
	return &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), whatsAppCtl: ctl}
}

func TestWhatsAppHandlersUseNoStoreAndMapErrors(t *testing.T) {
	fake := &fakeWhatsAppController{snapshot: whatsapp.LinkSnapshot{State: whatsapp.LinkStateQR, QRPNG: "data:image/png;base64,AAAA"}}
	s := whatsAppTestServer(fake)
	rec := httptest.NewRecorder()
	handleWhatsAppLink(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/link", strings.NewReader(`{"method":"phone","phone":"+49 170 1234567","force":true}`)))
	if rec.Code != http.StatusAccepted || rec.Header().Get("Cache-Control") != "no-store" || fake.linkReq.Phone != "+49 170 1234567" || !fake.linkForce {
		t.Fatalf("link: status=%d headers=%v req=%+v force=%v", rec.Code, rec.Header(), fake.linkReq, fake.linkForce)
	}
	rec = httptest.NewRecorder()
	handleWhatsAppLink(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/whatsapp/link", nil))
	var snap whatsapp.LinkSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil || snap.State != whatsapp.LinkStateQR || snap.QRPNG == "" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("snapshot: %s %v", rec.Body.String(), err)
	}
	for err, want := range map[error]struct {
		code   int
		reason string
	}{
		whatsapp.ErrAlreadyLinked:      {http.StatusConflict, "already_linked"},
		whatsapp.ErrLinkInProgress:     {http.StatusConflict, "link_in_progress"},
		whatsapp.ErrNotConfigured:      {http.StatusConflict, "not_configured"},
		whatsapp.ErrInvalidPhone:       {http.StatusBadRequest, "invalid_phone"},
		whatsapp.ErrInvalidLinkMethod:  {http.StatusBadRequest, "invalid_method"},
		errors.New("websocket closed"): {http.StatusInternalServerError, "whatsapp_failed"},
	} {
		fake.linkErr = err
		rec = httptest.NewRecorder()
		handleWhatsAppLink(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/link", strings.NewReader(`{"method":"qr"}`)))
		if rec.Code != want.code || !strings.Contains(rec.Body.String(), `"`+want.reason+`"`) || strings.Contains(rec.Body.String(), "websocket") {
			t.Errorf("%v: status=%d body=%s", err, rec.Code, rec.Body.String())
		}
	}
	rec = httptest.NewRecorder()
	handleWhatsAppLink(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/link", strings.NewReader(`{"method":"qr","number":"+491701234567"}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("unknown field: status=%d body=%s", rec.Code, rec.Body.String())
	}
	integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateNotLinked, config.WhatsAppModeDedicated)
	rec = httptest.NewRecorder()
	handleWhatsAppStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/whatsapp/status", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"status":"not_linked"`) {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	fake.logout = whatsapp.LogoutResult{RemoteRemoved: true}
	rec = httptest.NewRecorder()
	handleWhatsAppLogout(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/logout", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"remote_removed":true`) {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleWhatsAppLogout(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/whatsapp/logout", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET logout = %d, want 405", rec.Code)
	}
}

// Bot errors wrap whatsmeow errors (%w), and their text can carry a recipient
// JID. No handler echoes err.Error(), and the log keeps digits masked.
func TestWhatsAppHandlersNeverEchoWrappedErrors(t *testing.T) {
	const number = "491701234567"
	leak := fmt.Errorf("send whatsapp test message: %w", errors.New("websocket closed while sending to "+number+"@s.whatsapp.net"))
	wrappedSentinel := fmt.Errorf("resolve "+number+"@s.whatsapp.net: %w", whatsapp.ErrNotConnected)

	originalNow, originalLast := whatsAppTestNow, whatsAppTestLast
	t.Cleanup(func() { whatsAppTestNow, whatsAppTestLast = originalNow, originalLast })
	whatsAppTestNow = time.Now

	var logs bytes.Buffer
	fake := &fakeWhatsAppController{}
	s := whatsAppTestServer(fake)
	s.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	for _, tc := range []struct {
		name       string
		set        func(error)
		serve      func(*httptest.ResponseRecorder)
		err        error
		wantStatus int
		wantCode   string
	}{
		{"test, unexpected", func(e error) { fake.testErr = e }, func(rec *httptest.ResponseRecorder) {
			whatsAppTestLast = time.Time{}
			handleWhatsAppTest(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/test", nil))
		}, leak, http.StatusInternalServerError, "whatsapp_failed"},
		{"test, wrapped sentinel", func(e error) { fake.testErr = e }, func(rec *httptest.ResponseRecorder) {
			whatsAppTestLast = time.Time{}
			handleWhatsAppTest(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/test", nil))
		}, wrappedSentinel, http.StatusConflict, "not_connected"},
		{"logout", func(e error) { fake.logoutErr = e }, func(rec *httptest.ResponseRecorder) {
			handleWhatsAppLogout(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/logout", nil))
		}, leak, http.StatusInternalServerError, "whatsapp_failed"},
		{"link", func(e error) { fake.linkErr = e }, func(rec *httptest.ResponseRecorder) {
			handleWhatsAppLink(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/link", strings.NewReader(`{"method":"qr"}`)))
		}, leak, http.StatusInternalServerError, "whatsapp_failed"},
	} {
		tc.set(tc.err)
		rec := httptest.NewRecorder()
		tc.serve(rec)
		body := rec.Body.String()
		if rec.Code != tc.wantStatus || !strings.Contains(body, `"`+tc.wantCode+`"`) {
			t.Errorf("%s: status=%d body=%s", tc.name, rec.Code, body)
		}
		for _, secret := range []string{number, "4917", "s.whatsapp.net", "websocket", "send whatsapp"} {
			if strings.Contains(body, secret) {
				t.Errorf("%s: response leaks %q: %s", tc.name, secret, body)
			}
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: Cache-Control = %q", tc.name, rec.Header().Get("Cache-Control"))
		}
	}
	if strings.Contains(logs.String(), number) {
		t.Errorf("the log carries an unmasked number: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "***4567") {
		t.Errorf("the log does not carry the masked cause: %s", logs.String())
	}
}

func TestWhatsAppTestRateLimitAllowsOneSendPerMinute(t *testing.T) {
	originalNow, originalLast := whatsAppTestNow, whatsAppTestLast
	t.Cleanup(func() { whatsAppTestNow, whatsAppTestLast = originalNow, originalLast })
	fixed := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	whatsAppTestNow = func() time.Time { return fixed }
	whatsAppTestLast = time.Time{}
	fake := &fakeWhatsAppController{}
	s := whatsAppTestServer(fake)
	first, second := httptest.NewRecorder(), httptest.NewRecorder()
	handleWhatsAppTest(s).ServeHTTP(first, whatsAppPost("/api/whatsapp/test", nil))
	handleWhatsAppTest(s).ServeHTTP(second, whatsAppPost("/api/whatsapp/test", nil))
	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests || fake.tests != 1 || !strings.Contains(second.Body.String(), "rate_limited") {
		t.Fatalf("responses = %d/%d, sends = %d", first.Code, second.Code, fake.tests)
	}
	whatsAppTestNow = func() time.Time { return fixed.Add(time.Minute) }
	third := httptest.NewRecorder()
	handleWhatsAppTest(s).ServeHTTP(third, whatsAppPost("/api/whatsapp/test", nil))
	if third.Code != http.StatusOK || fake.tests != 2 {
		t.Fatalf("after 60 s: status=%d sends=%d, want 200 and 2", third.Code, fake.tests)
	}
}

func TestWhatsAppRoutesRequireAdmin(t *testing.T) {
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "whatsapp-route-test-session-secret"
	s := &Server{Cfg: cfg, Logger: slog.Default(), whatsAppCtl: &fakeWhatsAppController{}}
	mux := http.NewServeMux()
	registerWhatsAppRoutes(mux, s)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/whatsapp/status"},
		{http.MethodGet, "/api/whatsapp/link"},
		{http.MethodPost, "/api/whatsapp/link"},
		{http.MethodPost, "/api/whatsapp/logout"},
		{http.MethodPost, "/api/whatsapp/test"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", route.method, route.path, rec.Code)
		}
	}
}

// whatsAppPost builds a same-origin browser POST. httptest requests carry the
// host example.com.
func whatsAppPost(path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Origin", "http://example.com")
	return req
}

func TestWhatsAppLinkBodyIsBoundedAndSingle(t *testing.T) {
	fake := &fakeWhatsAppController{}
	s := whatsAppTestServer(fake)
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handleWhatsAppLink(s).ServeHTTP(rec, whatsAppPost("/api/whatsapp/link", strings.NewReader(body)))
		return rec
	}
	for name, tc := range map[string]struct {
		body string
		code int
		want string
	}{
		"second object":   {`{"method":"qr"}{"method":"phone","phone":"+491701234567"}`, http.StatusBadRequest, "invalid_request"},
		"trailing junk":   {`{"method":"qr"} x`, http.StatusBadRequest, "invalid_request"},
		"empty body":      {``, http.StatusBadRequest, "invalid_request"},
		"oversize":        {`{"method":"qr","phone":"` + strings.Repeat("1", whatsAppLinkBodyLimit+1) + `"}`, http.StatusRequestEntityTooLarge, "request_too_large"},
		"trailing spaces": {"{\"method\":\"qr\"}  \n", http.StatusAccepted, ""},
	} {
		fake.linkReq = whatsapp.LinkRequest{}
		rec := post(tc.body)
		if rec.Code != tc.code || (tc.want != "" && !strings.Contains(rec.Body.String(), `"`+tc.want+`"`)) {
			t.Errorf("%s: status=%d body=%s", name, rec.Code, rec.Body.String())
		}
		if tc.code != http.StatusAccepted && fake.linkReq != (whatsapp.LinkRequest{}) {
			t.Errorf("%s: a rejected body reached the controller: %+v", name, fake.linkReq)
		}
	}
}

// A browser mutation needs a matching Origin even with a valid admin session.
func TestWhatsAppMutationsRequireSameOrigin(t *testing.T) {
	originalNow, originalLast := whatsAppTestNow, whatsAppTestLast
	t.Cleanup(func() { whatsAppTestNow, whatsAppTestLast = originalNow, originalLast })
	whatsAppTestNow = time.Now
	whatsAppTestLast = time.Time{}

	const secret = "whatsapp-origin-test-session-secret"
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = secret
	fake := &fakeWhatsAppController{}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), whatsAppCtl: fake}
	mux := http.NewServeMux()
	registerWhatsAppRoutes(mux, s)
	session := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(secret, time.Now().Add(time.Hour))}
	serve := func(path, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"method":"qr"}`))
		req.AddCookie(session)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	for _, path := range []string{"/api/whatsapp/link", "/api/whatsapp/logout", "/api/whatsapp/test"} {
		for _, origin := range []string{"", "http://evil.example"} {
			rec := serve(path, origin)
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "csrf_check_failed") {
				t.Errorf("%s with Origin %q = %d %s, want 403 csrf_check_failed", path, origin, rec.Code, rec.Body.String())
			}
		}
	}
	if fake.tests != 0 || fake.linkReq != (whatsapp.LinkRequest{}) {
		t.Fatalf("a forged request reached the controller: tests=%d link=%+v", fake.tests, fake.linkReq)
	}
	for _, path := range []string{"/api/whatsapp/link", "/api/whatsapp/logout", "/api/whatsapp/test"} {
		if rec := serve(path, "http://example.com"); rec.Code >= 300 {
			t.Errorf("%s with the matching Origin = %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if fake.tests != 1 {
		t.Errorf("same-origin test send count = %d, want 1 (forged requests must not use up the rate limit)", fake.tests)
	}
}

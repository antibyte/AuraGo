package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestSpeechLabBrowserProxyRequiresFreshSessionAndProtectsUpstream(t *testing.T) {
	var upstreamCookie, upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCookie = r.Header.Get("Cookie")
		upstreamAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	cfg.SpeechLab.Enabled = true
	cfg.SpeechLab.BrowserBackendURL = upstream.URL
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	handler := handleSpeechLabBrowser(s)

	request := func(method string, withSession bool, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://aurago.test/speech-lab/api/v1/stack", nil)
		if withSession {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	if got := request(http.MethodGet, false, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("anonymous Browser Lab status = %d, want 401", got)
	}
	if got := request(http.MethodPost, true, "https://evil.test").Code; got != http.StatusForbidden {
		t.Fatalf("cross-origin stack write status = %d, want 403", got)
	}
	if got := request(http.MethodPost, true, "https://aurago.test").Code; got != http.StatusNoContent {
		t.Fatalf("same-origin stack write status = %d, want 204", got)
	}
	if upstreamCookie != "" || upstreamAuth != "" {
		t.Fatal("AuraGo credentials reached the Speech Lab backend")
	}
	cfg.Auth.Enabled = false
	cfg.Auth.AllowUnauthenticatedRemote = true
	if got := request(http.MethodGet, true, "").Code; got != http.StatusForbidden {
		t.Fatalf("auth-disabled Browser Lab status = %d, want 403", got)
	}
}

func TestSpeechLabBrowserProxyRejectsForeignWebSocketOrigin(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	cfg.SpeechLab.Enabled = true
	s := &Server{Cfg: cfg}
	req := httptest.NewRequest(http.MethodGet, "https://aurago.test/speech-lab/ws", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Origin", "https://evil.test")
	rec := httptest.NewRecorder()
	handleSpeechLabBrowser(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign WebSocket origin status = %d, want 403", rec.Code)
	}
}

func TestTrustedProxyRequiresConfiguredPeer(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.HTTPS.BehindProxy = true
	cfg.Server.HTTPS.TrustedProxyCIDRs = []string{"192.0.2.10/32"}
	s := &Server{Cfg: cfg}
	handler := trustedProxyMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Observed-Host", requestHost(r))
		w.Header().Set("Observed-Secure", strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))))
	}))
	for _, tt := range []struct{ peer, host, proto string }{
		{"192.0.2.11:1234", "local.test", ""},
		{"192.0.2.10:1234", "public.test", "https"},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://local.test/", nil)
		req.RemoteAddr = tt.peer
		req.Header.Set("X-Forwarded-Host", "public.test")
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Header().Get("Observed-Host") != tt.host || rec.Header().Get("Observed-Secure") != tt.proto {
			t.Fatalf("peer %s yielded host=%q proto=%q", tt.peer, rec.Header().Get("Observed-Host"), rec.Header().Get("Observed-Secure"))
		}
	}
}

func TestRemoteAuthExposureRequiresExplicitException(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	if err := validateRemoteAuthExposure(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Server.Host = "localhost"
	if err := validateRemoteAuthExposure(cfg); err != nil {
		t.Fatalf("localhost should be loopback-only: %v", err)
	}
	cfg.Tailscale.TsNet.Enabled = true
	if err := validateRemoteAuthExposure(cfg); err == nil {
		t.Fatal("Tailscale listener accepted without auth")
	}
	cfg.Auth.AllowUnauthenticatedRemote = true
	if err := validateRemoteAuthExposure(cfg); err != nil {
		t.Fatalf("explicit unsafe exception was ignored: %v", err)
	}
}

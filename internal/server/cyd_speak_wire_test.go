package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/httpstream"
)

// The glass speaks only when GET /api/cyd/speak/{id} returns raw PCM with a
// Content-Length. ESP32 HTTPClient ignores chunked bodies in this path.
func TestCYDSpeakWireKeepsIdentityPCMForESP32(t *testing.T) {
	t.Parallel()
	pcm := []byte{128, 140, 160, 180, 200, 180, 160, 140, 128, 100, 80, 60, 40, 60, 80, 100}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Sample-Rate", "8000")
		w.Header().Set("Content-Length", "16")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(pcm)
	})
	s := &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"

	chain := httpstream.WithWriteTimeout(
		gzipMiddleware(securityHeadersMiddleware(authMiddleware(s, inner), false, false)),
		40*time.Minute,
	)
	srv := httptest.NewServer(chain)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/cyd/speak/ntf_abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Same Accept-Encoding the ESP32 HTTPClient sends on every request.
	req.Header.Set("Accept-Encoding", "identity;q=1,chunked;q=0.1,*;q=0")
	req.Header.Set("Authorization", "Bearer aura_K7M2PQ9XH")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body %q", resp.StatusCode, body)
	}
	if resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("Content-Encoding = %q", resp.Header.Get("Content-Encoding"))
	}
	if len(resp.TransferEncoding) != 0 {
		t.Fatalf("Transfer-Encoding = %v", resp.TransferEncoding)
	}
	if resp.ContentLength != int64(len(pcm)) {
		t.Fatalf("Content-Length = %d, want %d", resp.ContentLength, len(pcm))
	}
	if string(body) != string(pcm) {
		t.Fatalf("body = %v", body)
	}
	var headerBytes int
	for k, vals := range resp.Header {
		for _, v := range vals {
			headerBytes += len(k) + len(v) + 4
		}
	}
	t.Logf("response header bytes ≈ %d", headerBytes)
	if headerBytes > 2048 {
		t.Fatalf("security headers are %d bytes; ESP32 HTTPClient drops long CYD responses", headerBytes)
	}
}

func TestCYDSpeakAuthBypassDoesNotDemandAdminScope(t *testing.T) {
	s, raw := testCYDServer(t)
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	admin, _, err := s.TokenManager.Create("admin-test", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := authMiddleware(s, handleCYDSpeak(s))
	for _, tc := range []struct {
		name, token string
		status      int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"invalid", "not-a-token", http.StatusForbidden},
		{"wrong-scope", admin, http.StatusForbidden},
		{"device", raw, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			// ID validation happens after device authorization, without waiting for TTS.
			req := httptest.NewRequest(http.MethodGet, "/api/cyd/speak/invalid/id", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.status || strings.Contains(rec.Body.String(), "invalid_bearer_scope") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

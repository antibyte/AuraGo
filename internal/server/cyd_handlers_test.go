package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/cyd"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func testCYDServer(t *testing.T) (*Server, string) {
	t.Helper()
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	tm, err := security.NewTokenManager(vault, filepath.Join(t.TempDir(), "tokens.bin"))
	if err != nil {
		t.Fatalf("tokens: %v", err)
	}
	raw, _, err := tm.Create("cyd-test", []string{"cyd"}, nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	cfg := &config.Config{}
	cfg.Cyd.Enabled = true
	cfg.Cyd.PollSeconds = 5
	s := &Server{Cfg: cfg, TokenManager: tm, CydHub: cyd.NewHub()}
	cyd.SetGlobal(s.CydHub)
	t.Cleanup(func() { cyd.SetGlobal(nil) })
	return s, raw
}

func TestCYDSnapshotAuth(t *testing.T) {
	s, raw := testCYDServer(t)
	h := handleCYDSnapshot(s)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cyd/snapshot", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/cyd/snapshot", nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("bad token status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/cyd/snapshot", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("good token status = %d body %s", rec.Code, rec.Body.String())
	}
	var snap cyd.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
}

func TestCYDTestNotificationAppearsOnSnapshot(t *testing.T) {
	s, raw := testCYDServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cyd/test", strings.NewReader(`{"title":"Backup failed","message":"disk 98%","priority":"critical"}`))
	req.Header.Set("Content-Type", "application/json")
	handleCYDTest(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("test status = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	get := httptest.NewRequest(http.MethodGet, "/api/cyd/snapshot", nil)
	get.Header.Set("Authorization", "Bearer "+raw)
	handleCYDSnapshot(s).ServeHTTP(rec, get)
	var snap cyd.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if snap.Notify == nil || snap.Notify.Title != "Backup failed" {
		t.Fatalf("notify = %+v", snap.Notify)
	}
}

func TestSendNotificationCYDRequiresDevice(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cyd.Enabled = true
	cyd.SetGlobal(cyd.NewHub())
	t.Cleanup(func() { cyd.SetGlobal(nil) })
	out := tools.SendNotification(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "cyd", "t", "m", "normal", nil)
	if !strings.Contains(out, "no cyd device") {
		t.Fatalf("got %s", out)
	}
}

func TestCYDStatusHandlesNilConfig(t *testing.T) {
	s := &Server{CydHub: cyd.NewHub()}
	rec := httptest.NewRecorder()
	handleCYDStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cyd/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["enabled"] != false {
		t.Fatalf("enabled = %v", body["enabled"])
	}
}

func TestAuthBypassesCYDDeviceRoutes(t *testing.T) {
	for _, path := range []string{"/api/cyd/snapshot", "/api/cyd/ws", "/api/cyd/ack", "/api/cyd/heartbeat"} {
		if !isAuthBypassed(path) {
			t.Fatalf("%s should bypass session auth", path)
		}
	}
	if isAuthBypassed("/api/cyd/status") {
		t.Fatal("status must stay session-protected")
	}
	if isAuthBypassed("/api/cyd/test") {
		t.Fatal("test must stay session-protected")
	}
}

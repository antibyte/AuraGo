package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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
	if rec.Header().Get("Content-Length") == "" {
		t.Fatal("snapshot must set Content-Length so ESP32 HTTPClient can read the full body")
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

func TestCYDDeviceURLUsesHTTPSPort(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "192.168.1.9"
	cfg.Server.Port = 8088
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	if got := cydDeviceURL(cfg); got != "https://192.168.1.9:8443" {
		t.Fatalf("device url = %q", got)
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

func TestCYDFirmwareStatusAndProvision(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cyd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bootloader.bin", "partitions.bin", "boot_app0.bin", "firmware.bin"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CYD_FIRMWARE_DIR", root)
	s, _ := testCYDServer(t)

	rec := httptest.NewRecorder()
	handleCYDFirmwareStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cyd/firmware/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["provision_offset"] == nil {
		t.Fatalf("missing provision_offset: %v", body)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cyd/firmware/provision", strings.NewReader(`{"url":"https://192.168.1.9:8443","token":"aura_ABCDEFGHJ"}`))
	req.Header.Set("Content-Type", "application/json")
	handleCYDFirmwareProvision(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("provision = %d %s", rec.Code, rec.Body.String())
	}
	url, token, ok := cyd.DecodeFactoryBlob(rec.Body.Bytes())
	if !ok || url != "https://192.168.1.9:8443" || token != "aura_ABCDEFGHJ" {
		t.Fatalf("blob url=%q token=%q ok=%v", url, token, ok)
	}

	rec = httptest.NewRecorder()
	handleCYDFirmwareFile(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cyd/firmware/cyd/firmware.bin", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("file = %d %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "firmware.bin" {
		t.Fatalf("body = %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handleCYDFirmwareFile(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cyd/firmware/cyd/../secret.bin", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d", rec.Code)
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
	if isAuthBypassed("/api/cyd/firmware/status") {
		t.Fatal("firmware must stay session-protected")
	}
}

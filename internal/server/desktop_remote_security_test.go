package server

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/inventory"
	"aurago/internal/security"
)

func TestWithDesktopRemoteGuardRateLimitsInvalidDeviceID(t *testing.T) {
	loginMu.Lock()
	loginRecords = make(map[string]*loginRecord)
	loginMu.Unlock()
	t.Cleanup(func() {
		loginMu.Lock()
		loginRecords = make(map[string]*loginRecord)
		loginMu.Unlock()
	})

	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("init inventory db: %v", err)
	}
	defer db.Close()
	if err := inventory.AddDevice(db, inventory.DeviceRecord{
		ID:       "valid-device",
		Name:     "valid-device",
		Type:     "server",
		Protocol: "ssh",
		Tags:     []string{},
	}); err != nil {
		t.Fatalf("add device: %v", err)
	}

	s := &Server{
		Cfg:         &config.Config{},
		InventoryDB: db,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	s.Cfg.Auth.MaxLoginAttempts = 1
	s.Cfg.Auth.LockoutMinutes = 10

	called := false
	handler := withDesktopRemoteGuard(s, "desktop_ssh_connect", "", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/ssh?device_id=missing-device", nil)
	req.RemoteAddr = "203.0.113.10:4567"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if called {
		t.Fatal("handler was called for invalid device")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("invalid device status = %d, want 404", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/desktop/ssh?device_id=valid-device", nil)
	req.RemoteAddr = "203.0.113.10:4567"
	rec = httptest.NewRecorder()
	handler(rec, req)
	if called {
		t.Fatal("handler was called while remote scope was locked")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked remote scope status = %d, want 429", rec.Code)
	}
}

func TestWithDesktopRemoteGuardAllowsDeviceScopedBearerToken(t *testing.T) {
	t.Parallel()

	s, token := testDesktopRemoteGuardServer(t, inventory.DeviceRecord{
		ID:       "device-a",
		Name:     "device-a",
		Type:     "server",
		Protocol: "ssh",
		Tags:     []string{"lab"},
	}, []string{"desktop:remote:device:device-a"})

	called := false
	handler := withDesktopRemoteGuard(s, "desktop_ssh_connect", "", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/ssh?device_id=device-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Fatalf("handler was not called: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWithDesktopRemoteGuardRejectsWrongDeviceScopedBearerToken(t *testing.T) {
	t.Parallel()

	s, token := testDesktopRemoteGuardServer(t, inventory.DeviceRecord{
		ID:       "device-a",
		Name:     "device-a",
		Type:     "server",
		Protocol: "ssh",
		Tags:     []string{"lab"},
	}, []string{"desktop:remote:device:device-b"})

	called := false
	handler := withDesktopRemoteGuard(s, "desktop_ssh_connect", "", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/ssh?device_id=device-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if called {
		t.Fatal("handler was called for wrong device-scoped token")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestWithDesktopRemoteGuardAllowsTagScopedBearerToken(t *testing.T) {
	t.Parallel()

	s, token := testDesktopRemoteGuardServer(t, inventory.DeviceRecord{
		ID:       "device-a",
		Name:     "device-a",
		Type:     "server",
		Protocol: "ssh",
		Tags:     []string{"lab", "gpu"},
	}, []string{"desktop:remote:tag:gpu"})

	called := false
	handler := withDesktopRemoteGuard(s, "desktop_ssh_connect", "", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/ssh?device_id=device-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Fatalf("handler was not called: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWithDesktopRemoteGuardAllowsAdminSessionWithoutDeviceScope(t *testing.T) {
	t.Parallel()

	s, _ := testDesktopRemoteGuardServer(t, inventory.DeviceRecord{
		ID:       "device-a",
		Name:     "device-a",
		Type:     "server",
		Protocol: "ssh",
		Tags:     []string{"lab"},
	}, nil)
	s.Cfg.Auth.Enabled = false

	called := false
	handler := withDesktopRemoteGuard(s, "desktop_ssh_connect", "", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/ssh?device_id=device-a", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Fatalf("handler was not called: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWithDesktopRemoteGuardWritesRequestAuditAttribution(t *testing.T) {
	t.Parallel()

	s, _ := testDesktopRemoteGuardServer(t, inventory.DeviceRecord{
		ID:       "device-a",
		Name:     "device-a",
		Type:     "server",
		Protocol: "ssh",
		Tags:     []string{"lab"},
	}, nil)
	s.Cfg.Auth.Enabled = false
	desktopDBPath := filepath.Join(t.TempDir(), "desktop.db")
	desktopSvc, err := desktop.NewService(desktop.Config{
		Enabled:            true,
		WorkspaceDir:       filepath.Join(t.TempDir(), "workspace"),
		DBPath:             desktopDBPath,
		MaxFileSizeMB:      1,
		AllowGeneratedApps: true,
		AllowAgentControl:  true,
		ControlLevel:       desktop.ControlConfirmDestructive,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := desktopSvc.Init(context.Background()); err != nil {
		t.Fatalf("Init desktop service: %v", err)
	}
	t.Cleanup(func() { _ = desktopSvc.Close() })
	s.DesktopService = desktopSvc

	handler := withDesktopRemoteGuard(s, "desktop_ssh_connect", "", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/ssh?device_id=device-a", nil)
	req.RemoteAddr = "203.0.113.20:1234"
	req.Header.Set("User-Agent", "desktop-audit-test")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-value"})
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	auditDB, err := sql.Open("sqlite", desktopDBPath)
	if err != nil {
		t.Fatalf("open audit db: %v", err)
	}
	defer auditDB.Close()
	var clientIP, sessionHash, userAgent string
	if err := auditDB.QueryRow(`SELECT client_ip, session_hash, user_agent FROM desktop_audit WHERE action = ? AND target = ?`, "desktop_ssh_connect", "device-a").Scan(&clientIP, &sessionHash, &userAgent); err != nil {
		t.Fatalf("query audit row: %v", err)
	}
	if clientIP != "203.0.113.20" || sessionHash != desktopRequestSessionHash(req) || userAgent != "desktop-audit-test" {
		t.Fatalf("audit attribution = %q/%q/%q", clientIP, sessionHash, userAgent)
	}
}

func testDesktopRemoteGuardServer(t *testing.T, device inventory.DeviceRecord, scopes []string) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := inventory.InitDB(filepath.Join(dir, "inventory.db"))
	if err != nil {
		t.Fatalf("init inventory db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := inventory.AddDevice(db, device); err != nil {
		t.Fatalf("add device: %v", err)
	}
	vault, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	token := ""
	if scopes != nil {
		token, _, err = tokens.Create("desktop remote", scopes, nil)
		if err != nil {
			t.Fatalf("Create token: %v", err)
		}
	}
	s := &Server{
		Cfg:          &config.Config{},
		InventoryDB:  db,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		TokenManager: tokens,
		Vault:        vault,
	}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "desktop-session-secret"
	s.Cfg.Auth.PasswordHash = "configured"
	s.StartedAt = time.Now()
	return s, token
}

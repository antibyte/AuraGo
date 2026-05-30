package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/inventory"
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

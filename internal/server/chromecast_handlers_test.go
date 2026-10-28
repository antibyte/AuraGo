package server

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/inventory"
)

func TestHandleChromecastTestGatesAndDeviceLookup(t *testing.T) {
	called := false
	orig := runChromecastTestAnnouncement
	runChromecastTestAnnouncement = func(cfg *config.Config, addr string, port int, inventoryDB *sql.DB, logger *slog.Logger) string {
		called = true
		if addr != "192.168.1.20" || port != 8009 {
			t.Fatalf("announcement target = %s:%d", addr, port)
		}
		return `{"status":"success","message":"sent"}`
	}
	defer func() { runChromecastTestAnnouncement = orig }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	server := &Server{Logger: logger, Cfg: cfg}
	body := strings.NewReader(`{"device_id":"cc1"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/chromecast/test", body)
	response := httptest.NewRecorder()
	handleChromecastTest(server).ServeHTTP(response, request)
	if response.Code != http.StatusConflict || called {
		t.Fatalf("disabled: %d called=%v", response.Code, called)
	}

	cfg.Chromecast.Enabled = true
	cfg.Chromecast.ReadOnly = true
	request = httptest.NewRequest(http.MethodPost, "/api/chromecast/test", strings.NewReader(`{"device_id":"cc1"}`))
	response = httptest.NewRecorder()
	handleChromecastTest(server).ServeHTTP(response, request)
	if response.Code != http.StatusConflict || called {
		t.Fatalf("read-only: %d called=%v", response.Code, called)
	}

	cfg.Chromecast.ReadOnly = false
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	server.InventoryDB = db
	request = httptest.NewRequest(http.MethodPost, "/api/chromecast/test", strings.NewReader(`{"device_id":"missing"}`))
	response = httptest.NewRecorder()
	handleChromecastTest(server).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || called {
		t.Fatalf("missing: %d called=%v", response.Code, called)
	}

	if err := inventory.AddDevice(db, inventory.DeviceRecord{
		ID:        "cc1",
		Name:      "Wohnzimmer",
		Type:      "chromecast",
		Protocol:  inventory.ProtocolNone,
		IPAddress: "192.168.1.20",
		Port:      8009,
		Tags:      []string{"chromecast"},
	}); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/chromecast/test", strings.NewReader(`{"device_id":"cc1"}`))
	response = httptest.NewRecorder()
	handleChromecastTest(server).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !called || !strings.Contains(response.Body.String(), `"status":"success"`) {
		t.Fatalf("found: %d called=%v body=%s", response.Code, called, response.Body.String())
	}
}

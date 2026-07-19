package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/bluetooth"
	"aurago/internal/config"
)

func testBluetoothServer() *Server {
	cfg := &config.Config{}
	cfg.Bluetooth.Enabled = true
	cfg.Bluetooth.ReadOnly = true
	cfg.Runtime.Bluetooth = bluetooth.Status{
		Supported: true,
		Usable:    true,
		Adapter:   bluetooth.AdapterStatus{Name: "Test Adapter", Powered: true},
		Audio:     bluetooth.AudioStatus{Usable: true, Backend: "pipewire"},
	}
	manager := bluetooth.NewManager(slog.Default())
	manager.Configure(config.BluetoothRuntimeOptions(cfg))
	manager.SeedStatus(cfg.Runtime.Bluetooth)
	server := &Server{Cfg: cfg, Logger: slog.Default(), Bluetooth: manager}
	server.initConfigSnapshot()
	return server
}

func TestBluetoothStatusHandlerContract(t *testing.T) {
	server := testBluetoothServer()
	request := httptest.NewRequest(http.MethodGet, "/api/bluetooth/status", nil)
	response := httptest.NewRecorder()
	handleBluetoothStatus(server).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", response.Code, response.Body.String())
	}
	for _, wanted := range []string{`"status"`, `"devices"`, `"playback"`, `"permissions"`, `"Test Adapter"`} {
		if !strings.Contains(response.Body.String(), wanted) {
			t.Fatalf("response missing %s: %s", wanted, response.Body.String())
		}
	}
}

func TestBluetoothMutatingHandlersRejectWrongMethods(t *testing.T) {
	server := testBluetoothServer()
	for name, handler := range map[string]http.Handler{
		"reprobe":  handleBluetoothReprobe(server),
		"discover": handleBluetoothDiscover(server),
		"action":   handleBluetoothDeviceAction(server),
		"test":     handleBluetoothAudioTest(server),
		"stop":     handleBluetoothAudioStop(server),
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/bluetooth/"+name, nil))
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestBluetoothDeviceActionHonorsReadOnly(t *testing.T) {
	server := testBluetoothServer()
	request := httptest.NewRequest(http.MethodPost, "/api/bluetooth/devices/action",
		strings.NewReader(`{"operation":"connect","address":"AA:BB:CC:DD:EE:FF"}`))
	response := httptest.NewRecorder()
	handleBluetoothDeviceAction(server).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), bluetooth.ErrorReadOnly) {
		t.Fatalf("response missing read-only code: %s", response.Body.String())
	}
}

func TestBluetoothRoutesRequireAdminSession(t *testing.T) {
	server := testBluetoothServer()
	server.Cfg.WebConfig.Enabled = true
	server.Cfg.Auth.Enabled = true
	server.Cfg.Auth.SessionSecret = "test-session-secret"
	mux := http.NewServeMux()
	server.registerConfigAPIRoutes(mux, nil)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/bluetooth/status", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !isAdminProtectedPath("/api/bluetooth/audio/test") {
		t.Fatal("Bluetooth route is missing from admin-protected path metadata")
	}
}

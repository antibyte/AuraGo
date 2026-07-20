package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/networkshares"
)

func testNetworkSharesServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.NetworkShares.Enabled = true
	cfg.NetworkShares.ReadOnly = true
	cfg.SQLite.NetworkSharesPath = filepath.Join(t.TempDir(), "network_shares.db")
	manager, err := networkshares.OpenManager(cfg.SQLite.NetworkSharesPath, slog.Default())
	if err != nil {
		t.Fatalf("OpenManager: %v", err)
	}
	manager.Configure(config.NetworkSharesOptions(cfg, ""))
	server := &Server{Cfg: cfg, Logger: slog.Default(), NetworkShares: manager}
	server.initConfigSnapshot()
	t.Cleanup(func() { _ = manager.Close() })
	return server
}

func TestNetworkSharesStatusHandlerContract(t *testing.T) {
	server := testNetworkSharesServer(t)
	response := httptest.NewRecorder()
	handleNetworkSharesStatus(server).ServeHTTP(response,
		httptest.NewRequest(http.MethodGet, "/api/network-shares/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, wanted := range []string{`"status"`, `"permissions"`, `"smb"`, `"nfs"`, `"allowed_roots"`} {
		if !strings.Contains(response.Body.String(), wanted) {
			t.Fatalf("response missing %s: %s", wanted, response.Body.String())
		}
	}
}

func TestNetworkSharesHandlersRejectWrongMethodsAndBadJSON(t *testing.T) {
	server := testNetworkSharesServer(t)
	for name, handler := range map[string]http.Handler{
		"status":   handleNetworkSharesStatus(server),
		"reprobe":  handleNetworkSharesReprobe(server),
		"validate": handleNetworkSharesValidate(server),
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/network-shares/"+name, nil))
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}

	response := httptest.NewRecorder()
	handleNetworkSharesCollection(server).ServeHTTP(response,
		httptest.NewRequest(http.MethodPost, "/api/network-shares", strings.NewReader(`{"unknown":true}`)))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), networkshares.ErrorInvalidArgument) {
		t.Fatalf("bad JSON status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestNetworkSharesRoutesRequireAdminSession(t *testing.T) {
	server := testNetworkSharesServer(t)
	server.Cfg.WebConfig.Enabled = true
	server.Cfg.Auth.Enabled = true
	server.Cfg.Auth.SessionSecret = "test-session-secret"
	mux := http.NewServeMux()
	server.registerConfigAPIRoutes(mux, nil)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/network-shares/status", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !isAdminProtectedPath("/api/network-shares/status") ||
		!isAdminProtectedPath("/api/network-shares/abc") {
		t.Fatal("network share routes are missing from admin-protected path metadata")
	}
}

func TestNetworkSharesValidateRequiresSupportedOperationAndUpdateID(t *testing.T) {
	server := testNetworkSharesServer(t)
	tests := []struct {
		name string
		body string
	}{
		{name: "missing operation", body: `{"share":{}}`},
		{name: "unknown operation", body: `{"operation":"delete","share":{}}`},
		{name: "update without id", body: `{"operation":"update","share":{}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handleNetworkSharesValidate(server).ServeHTTP(response,
				httptest.NewRequest(http.MethodPost, "/api/network-shares/validate", strings.NewReader(test.body)))
			if response.Code != http.StatusBadRequest ||
				!strings.Contains(response.Body.String(), networkshares.ErrorInvalidArgument) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

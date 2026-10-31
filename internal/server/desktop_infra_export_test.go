package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/infra"
	"aurago/internal/inventory"
)

func TestDesktopInfraExportPDFRoute(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("infra admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.InventoryDB = db
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.Infra.Enabled = true
	s.Cfg.VirtualDesktop.Infra.ReadOnly = true
	runtime, err := s.desktopInfraRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.store.CreateNode(context.Background(), infra.Node{Kind: infra.NodeDevice, Name: "Fritz!Box", Icon: "router"}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerDesktopInfraRoutes(mux, s)
	get := func(query, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/export/pdf"+query, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}

	response := get("?paper=a3&sections=map,inventory", adminToken)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("export = %d %q %.120s; want 200 application/pdf", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	if !strings.HasPrefix(response.Body.String(), "%PDF") {
		t.Fatal("export body is not a PDF")
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, `attachment; filename="homelab-map-`) || !strings.HasSuffix(disposition, `.pdf"`) {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	for _, query := range []string{"?paper=x", "?sections=map,bogus"} {
		if response := get(query, adminToken); response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_request") {
			t.Fatalf("export%s = %d %s; want 400 invalid_request", query, response.Code, response.Body.String())
		}
	}
	if response := get("", ""); response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
		t.Fatalf("export without a token = %d, want 401 or 403", response.Code)
	}
}

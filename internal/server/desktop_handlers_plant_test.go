package server

import (
	"aurago/internal/desktop"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestDesktopLeafyPermissionsAndActions(t *testing.T) {
	srv, readToken, writeToken := testDesktopPermissionServer(t)
	srv.Cfg.VirtualDesktop.Enabled = true
	srv.Cfg.Directories.DataDir = t.TempDir()
	srv.Cfg.VirtualDesktop.WorkspaceDir = filepath.Join(t.TempDir(), "workspace")
	srv.Cfg.SQLite.VirtualDesktopPath = filepath.Join(t.TempDir(), "desktop.db")
	defer func() {
		if srv.DesktopService != nil {
			_ = srv.DesktopService.Close()
		}
		if srv.DesktopHub != nil {
			srv.DesktopHub.Close()
		}
	}()
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handleDesktopPlant(srv)(rec, req)
		return rec
	}
	for _, tt := range []struct {
		method, path, token, body string
		status                    int
	}{
		{"GET", "/api/desktop/plant", "", "", 401},
		{"POST", "/api/desktop/plant/actions", readToken, `{"action":"replant","action_id":"plant_test_01","revision":0}`, 403},
		{"DELETE", "/api/desktop/plant", writeToken, "", 405},
		{"POST", "/api/desktop/plant/actions", writeToken, `{"action":"inject","action_id":"plant_test_01","revision":0}`, 400},
	} {
		r := call(tt.method, tt.path, tt.token, tt.body)
		if r.Code != tt.status {
			t.Fatalf("%s: %d %s", tt.method, r.Code, r.Body.String())
		}
	}
	before := call("GET", "/api/desktop/plant", readToken, "")
	if before.Code != 200 || before.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("read: %d %s", before.Code, before.Body.String())
	}
	var empty desktop.PlantSnapshot
	json.Unmarshal(before.Body.Bytes(), &empty)
	if empty.Plant != nil {
		t.Fatal("GET planted")
	}
	body := `{"action":"replant","action_id":"plant_test_01","revision":0}`
	first := call("POST", "/api/desktop/plant/actions", writeToken, body)
	if first.Code != 200 {
		t.Fatalf("plant: %d %s", first.Code, first.Body.String())
	}
	repeated := call("POST", "/api/desktop/plant/actions", writeToken, body)
	var a, b desktop.PlantSnapshot
	json.Unmarshal(first.Body.Bytes(), &a)
	json.Unmarshal(repeated.Body.Bytes(), &b)
	if repeated.Code != 200 || a.Plant.Revision != b.Plant.Revision || a.Plant.Seed != b.Plant.Seed {
		t.Fatal("replant retry duplicated")
	}
	conflict := call("POST", "/api/desktop/plant/actions", writeToken, `{"action":"water","action_id":"plant_test_02","revision":0}`)
	if conflict.Code != 409 {
		t.Fatalf("expected stale revision: %d", conflict.Code)
	}
	srv.Cfg.VirtualDesktop.ReadOnly = true
	denied := call("POST", "/api/desktop/plant/actions", writeToken, `{"action":"water","action_id":"plant_test_03","revision":1}`)
	if denied.Code != 403 {
		t.Fatalf("readonly: %d %s", denied.Code, denied.Body.String())
	}
}

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/inventory"
)

func fileAccessTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &Server{InventoryDB: db, Logger: slog.Default()}
}

func postDevice(t *testing.T, s *Server, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	handleCreateDevice(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader(raw)))
	return rec
}

func putDevice(t *testing.T, s *Server, id string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	handleUpdateDevice(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/devices/"+id, bytes.NewReader(raw)))
	return rec
}

func createdID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp["id"]
}

func TestCreateRDPDeviceStoresFileAccess(t *testing.T) {
	s := fileAccessTestServer(t)
	id := createdID(t, postDevice(t, s, map[string]any{
		"name": "win", "protocol": "rdp", "ip_address": "10.0.0.5",
		"file_access": map[string]any{"mode": "smb", "smb_share": " Data ", "sftp_port": 2222},
	}))
	fa, err := inventory.GetDeviceFileAccess(s.InventoryDB, id)
	if err != nil || fa.Mode != "smb" || fa.SMBShare != "Data" || fa.SFTPPort != 2222 {
		t.Fatalf("stored %+v %v", fa, err)
	}
}

func TestCreateDeviceRejectsInvalidFileAccessWithoutCreatingIt(t *testing.T) {
	s := fileAccessTestServer(t)
	for _, fa := range []map[string]any{
		{"mode": "weird"},
		{"mode": "sftp", "sftp_port": 70000},
		{"mode": "smb", "smb_share": "IPC$"},
		{"mode": "smb", "smb_share": "a/b"},
		{"mode": "auto", "credential_id": "does-not-exist"},
	} {
		rec := postDevice(t, s, map[string]any{"name": "bad", "protocol": "rdp", "ip_address": "10.0.0.6", "file_access": fa})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%v: status %d %s", fa, rec.Code, rec.Body.String())
		}
	}
	devices, _ := inventory.ListAllDevices(s.InventoryDB)
	if len(devices) != 0 {
		t.Fatalf("bad input created %d device(s)", len(devices))
	}
}

func TestUpdateDeviceFileAccessLifecycle(t *testing.T) {
	s := fileAccessTestServer(t)
	id := createdID(t, postDevice(t, s, map[string]any{
		"name": "win", "protocol": "rdp", "ip_address": "10.0.0.5",
		"file_access": map[string]any{"mode": "auto"},
	}))
	// Update without file_access keeps the row.
	if rec := putDevice(t, s, id, map[string]any{"name": "win2", "protocol": "rdp", "ip_address": "10.0.0.5"}); rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if fa, _ := inventory.GetDeviceFileAccess(s.InventoryDB, id); fa.Mode != "auto" {
		t.Fatalf("row lost on update without file_access: %+v", fa)
	}
	// Invalid input is a 400 and changes nothing.
	if rec := putDevice(t, s, id, map[string]any{"name": "renamed", "protocol": "rdp", "ip_address": "10.0.0.5", "file_access": map[string]any{"mode": "x"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid update: %d", rec.Code)
	}
	if d, _ := inventory.GetDeviceByID(s.InventoryDB, id); d.Name != "win2" {
		t.Fatalf("invalid update changed the device: %+v", d)
	}
	// Explicit update.
	if rec := putDevice(t, s, id, map[string]any{"name": "win2", "protocol": "rdp", "ip_address": "10.0.0.5", "file_access": map[string]any{"mode": "off"}}); rec.Code != http.StatusOK {
		t.Fatalf("update: %d", rec.Code)
	}
	if fa, _ := inventory.GetDeviceFileAccess(s.InventoryDB, id); fa.Mode != "off" {
		t.Fatalf("explicit update: %+v", fa)
	}
	// Switching to ssh drops the row.
	if err := inventory.SetDeviceFileAccess(s.InventoryDB, id, inventory.FileAccess{Mode: "sftp"}); err != nil {
		t.Fatal(err)
	}
	if rec := putDevice(t, s, id, map[string]any{"name": "win2", "protocol": "ssh", "ip_address": "10.0.0.5"}); rec.Code != http.StatusOK {
		t.Fatalf("protocol change: %d %s", rec.Code, rec.Body.String())
	}
	all, _ := inventory.ListDeviceFileAccess(s.InventoryDB)
	if _, ok := all[id]; ok {
		t.Fatal("row survived the switch to ssh")
	}
}

func TestQuickConnectListCarriesFileAccessOnlyForRDP(t *testing.T) {
	s := fileAccessTestServer(t)
	rdpID := createdID(t, postDevice(t, s, map[string]any{"name": "w", "protocol": "rdp", "file_access": map[string]any{"mode": "sftp", "sftp_port": 2200}}))
	rdpOffID := createdID(t, postDevice(t, s, map[string]any{"name": "w2", "protocol": "rdp"}))
	sshID := createdID(t, postDevice(t, s, map[string]any{"name": "l", "protocol": "ssh"}))
	devices, _ := inventory.ListAllDevices(s.InventoryDB)
	list := quickConnectDevices(context.Background(), devices)
	fillQuickConnectFileAccess(s, list)
	raw, _ := json.Marshal(list)
	var out []map[string]any
	_ = json.Unmarshal(raw, &out)
	byID := map[string]map[string]any{}
	for _, d := range out {
		byID[d["id"].(string)] = d
	}
	fa, _ := byID[rdpID]["file_access"].(map[string]any)
	if fa["mode"] != "sftp" || fa["sftp_port"] != float64(2200) {
		t.Fatalf("rdp file_access: %v", byID[rdpID])
	}
	off, _ := byID[rdpOffID]["file_access"].(map[string]any)
	if off["mode"] != "off" || off["smb_share"] != "C$" || off["credential_id"] != "" {
		t.Fatalf("default file_access: %v", byID[rdpOffID])
	}
	if _, has := byID[sshID]["file_access"]; has {
		t.Fatalf("ssh device must not carry file_access: %v", byID[sshID])
	}
}

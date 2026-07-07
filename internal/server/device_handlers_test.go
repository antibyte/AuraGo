package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"
)

func TestHandleCreateDeviceAcceptsSupportedProtocols(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	s := &Server{InventoryDB: db, Logger: slog.Default()}
	for _, protocol := range []string{"none", "ssh", "vnc"} {
		body := map[string]interface{}{
			"name":     "device-" + protocol,
			"type":     "generic",
			"protocol": protocol,
			"tags":     []string{},
		}
		reqBody, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		handleCreateDevice(s).ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("protocol %q status = %d, body = %s", protocol, rec.Code, rec.Body.String())
		}
		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		got, err := inventory.GetDeviceByID(db, resp["id"])
		if err != nil {
			t.Fatalf("GetDeviceByID: %v", err)
		}
		if got.Protocol != protocol {
			t.Fatalf("stored protocol = %q, want %q", got.Protocol, protocol)
		}
	}
}

func TestHandleCreateDeviceRejectsInvalidProtocolWithFlatJSONError(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	s := &Server{InventoryDB: db, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader([]byte(`{"name":"bad","protocol":"telnet"}`)))
	rec := httptest.NewRecorder()

	handleCreateDevice(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp["error"] != "protocol must be ssh, vnc, or none" {
		t.Fatalf("error = %q, want flat protocol error", resp["error"])
	}
}

func TestHandleUpdateDevicePreservesCredentialReferenceWhenLegacyFieldsOmitted(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	credID, err := credentials.Create(db, credentials.Record{
		Name:     "SSH Root",
		Type:     "ssh",
		Host:     "10.0.0.5",
		Username: "root",
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	deviceID, err := inventory.CreateDevice(db, "server-1", "server", "ssh", "192.168.1.10", 22, "", "", credID, "desc", []string{"prod"}, "")
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	s := &Server{InventoryDB: db, Logger: slog.Default()}
	reqBody := []byte(`{"name":"server-1","type":"server","protocol":"ssh","ip_address":"192.168.1.11","port":22,"tags":["prod"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/devices/"+deviceID, bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	handleUpdateDevice(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, err := inventory.GetDeviceByID(db, deviceID)
	if err != nil {
		t.Fatalf("GetDeviceByID: %v", err)
	}
	if got.CredentialID != credID {
		t.Fatalf("credential_id = %q, want %q", got.CredentialID, credID)
	}
	if got.Username != "" || got.VaultSecretID != "" {
		t.Fatalf("legacy auth fields should remain cleared for credential-linked device: %#v", got)
	}
}

func TestHandleRemoteDeviceDeleteMissingReturnsNotFound(t *testing.T) {
	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	defer db.Close()
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	s := &Server{
		Cfg:       &config.Config{},
		Logger:    slog.Default(),
		RemoteHub: remote.NewRemoteHub(db, nil, slog.Default()),
		Vault:     vault,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/remote/devices/missing", nil)
	rec := httptest.NewRecorder()
	handleRemoteDevice(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRemoteDeviceRevokeMissingReturnsNotFound(t *testing.T) {
	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	defer db.Close()
	s := &Server{
		Cfg:       &config.Config{},
		Logger:    slog.Default(),
		RemoteHub: remote.NewRemoteHub(db, nil, slog.Default()),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/remote/devices/missing/revoke", nil)
	rec := httptest.NewRecorder()
	handleRemoteDeviceRevoke(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/security"
)

func TestCredentialHandlersStoreSecretsInVault(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("1", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	s := &Server{InventoryDB: db, Vault: vault}

	body := map[string]string{
		"name":             "Root SSH",
		"type":             "ssh",
		"host":             "192.168.1.2",
		"username":         "root",
		"password":         "super-secret",
		"certificate_mode": "text",
		"certificate_text": "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/credentials", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handleCreateCredential(s)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/credentials status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var createResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("create response JSON error = %v", err)
	}
	id := createResp["id"]
	if id == "" {
		t.Fatalf("create response missing id")
	}

	item, err := credentials.GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if item.PasswordVaultID == "" || item.CertificateVaultID == "" {
		t.Fatalf("vault ids not stored on record: %+v", item)
	}

	password, err := vault.ReadSecret(item.PasswordVaultID)
	if err != nil {
		t.Fatalf("vault.ReadSecret(password) error = %v", err)
	}
	if password != "super-secret" {
		t.Fatalf("password in vault = %q, want %q", password, "super-secret")
	}

	cert, err := vault.ReadSecret(item.CertificateVaultID)
	if err != nil {
		t.Fatalf("vault.ReadSecret(certificate) error = %v", err)
	}
	if !strings.Contains(cert, "BEGIN CERTIFICATE") {
		t.Fatalf("certificate in vault = %q, want PEM text", cert)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/credentials/"+id, nil)
	getRec := httptest.NewRecorder()
	handleGetCredential(s)(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/credentials/{id} status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
	if strings.Contains(getRec.Body.String(), "super-secret") || strings.Contains(getRec.Body.String(), "BEGIN CERTIFICATE") {
		t.Fatalf("secret leaked in API response: %s", getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), "\"has_password\":true") || !strings.Contains(getRec.Body.String(), "\"has_certificate\":true") {
		t.Fatalf("secret presence flags missing in API response: %s", getRec.Body.String())
	}
}

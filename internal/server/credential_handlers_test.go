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

func TestCredentialHandlerTokenStorage(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("2", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	s := &Server{InventoryDB: db, Vault: vault}

	body := map[string]interface{}{
		"name":     "API Service",
		"type":     "token",
		"username": "svc-acct",
		"token":    "tok-abc-secret-123",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/credentials", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handleCreateCredential(s)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	id := resp["id"]

	item, err := credentials.GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if item.TokenVaultID == "" {
		t.Fatal("expected TokenVaultID to be set")
	}
	if !strings.HasPrefix(item.TokenVaultID, "credential_token_") {
		t.Fatalf("TokenVaultID = %q, want credential_token_ prefix", item.TokenVaultID)
	}
	if !item.HasToken {
		t.Fatal("expected HasToken = true")
	}

	tokenVal, err := vault.ReadSecret(item.TokenVaultID)
	if err != nil {
		t.Fatalf("vault.ReadSecret(token) error = %v", err)
	}
	if tokenVal != "tok-abc-secret-123" {
		t.Fatalf("token in vault = %q, want %q", tokenVal, "tok-abc-secret-123")
	}

	// GET should not leak the token
	getReq := httptest.NewRequest(http.MethodGet, "/api/credentials/"+id, nil)
	getRec := httptest.NewRecorder()
	handleGetCredential(s)(getRec, getReq)
	if strings.Contains(getRec.Body.String(), "tok-abc-secret-123") {
		t.Fatalf("token leaked in API response: %s", getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), "\"has_token\":true") {
		t.Fatalf("has_token flag missing in API response: %s", getRec.Body.String())
	}
}

func TestCredentialHandlerAllowPython(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("3", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	s := &Server{InventoryDB: db, Vault: vault}

	body := map[string]interface{}{
		"name":         "Python OK",
		"type":         "login",
		"username":     "pyuser",
		"password":     "pw123",
		"allow_python": true,
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/credentials", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handleCreateCredential(s)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)

	item, err := credentials.GetByID(db, resp["id"])
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !item.AllowPython {
		t.Fatal("expected AllowPython = true")
	}

	// Verify it appears in python-accessible list
	getReq := httptest.NewRequest(http.MethodGet, "/api/credentials/python-accessible", nil)
	getRec := httptest.NewRecorder()
	handleListPythonAccessibleCredentials(s)(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET python-accessible status = %d", getRec.Code)
	}
	if !strings.Contains(getRec.Body.String(), "Python OK") {
		t.Fatalf("expected credential in python-accessible list, got: %s", getRec.Body.String())
	}
}

func TestCredentialHandlerLoginTypeNoHost(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("4", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	s := &Server{InventoryDB: db, Vault: vault}

	// Login type without host should succeed
	body := map[string]string{
		"name":     "Web Panel",
		"type":     "login",
		"username": "admin",
		"password": "secret",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/credentials", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handleCreateCredential(s)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST login without host: status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCredentialHandlerInvalidType(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("5", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	s := &Server{InventoryDB: db, Vault: vault}

	body := map[string]string{
		"name":     "Bad Type",
		"type":     "ftp",
		"username": "user",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/credentials", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handleCreateCredential(s)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST invalid type: status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCredentialHandlerDeleteCleansVault(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("6", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	s := &Server{InventoryDB: db, Vault: vault}

	// Create with password and token
	body := map[string]interface{}{
		"name":     "Delete Me",
		"type":     "token",
		"username": "svc",
		"password": "pw-value",
		"token":    "tok-value",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/credentials", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handleCreateCredential(s)(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	id := resp["id"]

	item, _ := credentials.GetByID(db, id)
	pwVaultID := item.PasswordVaultID
	tokVaultID := item.TokenVaultID

	// Delete credential
	delReq := httptest.NewRequest(http.MethodDelete, "/api/credentials/"+id, nil)
	delRec := httptest.NewRecorder()
	handleDeleteCredential(s)(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d, body = %s", delRec.Code, delRec.Body.String())
	}

	// Vault secrets should be cleaned up
	if _, err := vault.ReadSecret(pwVaultID); err == nil {
		t.Fatal("password vault entry should have been deleted")
	}
	if _, err := vault.ReadSecret(tokVaultID); err == nil {
		t.Fatal("token vault entry should have been deleted")
	}
}

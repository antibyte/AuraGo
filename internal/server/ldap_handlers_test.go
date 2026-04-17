package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ldap"
	"aurago/internal/security"
)

type fakeLDAPHandlerClient struct {
	testErr error
}

func (f *fakeLDAPHandlerClient) TestConnection() error {
	return f.testErr
}

func TestHandleLDAPTestRejectsDisabledIntegration(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/api/ldap/test", nil)
	rec := httptest.NewRecorder()

	handleLDAPTest(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["status"] != "error" || payload["message"] != "LDAP integration is disabled" {
		t.Fatalf("payload = %v", payload)
	}
}

func TestHandleLDAPTestRequiresBindDN(t *testing.T) {
	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.Host = "ldap.example.com"

	s := &Server{Cfg: cfg}
	req := httptest.NewRequest(http.MethodPost, "/api/ldap/test", nil)
	rec := httptest.NewRecorder()

	handleLDAPTest(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["message"] != "LDAP bind_dn is not configured" {
		t.Fatalf("payload = %v", payload)
	}
}

func TestHandleLDAPTestUsesVaultPassword(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := vault.WriteSecret("ldap_bind_password", "vault-secret"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}

	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.Host = "ldap.example.com"
	cfg.LDAP.BindDN = "cn=svc,dc=example,dc=com"

	var captured ldap.LDAPConfig
	orig := newLDAPTestClient
	newLDAPTestClient = func(clientCfg ldap.LDAPConfig) ldapTestClient {
		captured = clientCfg
		return &fakeLDAPHandlerClient{}
	}
	defer func() { newLDAPTestClient = orig }()

	s := &Server{Cfg: cfg, Vault: vault}
	req := httptest.NewRequest(http.MethodPost, "/api/ldap/test", nil)
	rec := httptest.NewRecorder()

	handleLDAPTest(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["status"] != "success" {
		t.Fatalf("payload = %v", payload)
	}
	if captured.BindPassword != "vault-secret" {
		t.Fatalf("captured.BindPassword = %q, want vault-secret", captured.BindPassword)
	}
}

func TestHandleLDAPTestPropagatesConnectionFailure(t *testing.T) {
	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.Host = "ldap.example.com"
	cfg.LDAP.BindDN = "cn=svc,dc=example,dc=com"

	orig := newLDAPTestClient
	newLDAPTestClient = func(clientCfg ldap.LDAPConfig) ldapTestClient {
		return &fakeLDAPHandlerClient{testErr: fmt.Errorf("dial failed")}
	}
	defer func() { newLDAPTestClient = orig }()

	s := &Server{Cfg: cfg}
	req := httptest.NewRequest(http.MethodPost, "/api/ldap/test", nil)
	rec := httptest.NewRecorder()

	handleLDAPTest(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("payload = %v", payload)
	}
}

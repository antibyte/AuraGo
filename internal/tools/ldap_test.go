package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ldap"
	"aurago/internal/security"
)

type fakeLDAPToolClient struct {
	connectErr        error
	connectAndBindErr error
	testErr           error
	searchResult      *ldap.SearchResult
	lastConfig        ldap.LDAPConfig
}

func (f *fakeLDAPToolClient) Connect() error        { return f.connectErr }
func (f *fakeLDAPToolClient) ConnectAndBind() error { return f.connectAndBindErr }
func (f *fakeLDAPToolClient) Close()                {}
func (f *fakeLDAPToolClient) Search(baseDN, filter string, attributes []string) (*ldap.SearchResult, error) {
	if f.searchResult != nil {
		return f.searchResult, nil
	}
	return &ldap.SearchResult{}, nil
}
func (f *fakeLDAPToolClient) GetUser(username string) (*ldap.SearchEntry, error) { return nil, nil }
func (f *fakeLDAPToolClient) ListUsers() (*ldap.SearchResult, error) {
	return &ldap.SearchResult{}, nil
}
func (f *fakeLDAPToolClient) GetGroup(groupName string) (*ldap.SearchEntry, error) { return nil, nil }
func (f *fakeLDAPToolClient) ListGroups() (*ldap.SearchResult, error) {
	return &ldap.SearchResult{}, nil
}
func (f *fakeLDAPToolClient) Authenticate(userDN, password string) (bool, error) { return true, nil }
func (f *fakeLDAPToolClient) AddEntry(dn string, attributes map[string][]string) error {
	return nil
}
func (f *fakeLDAPToolClient) ModifyEntry(dn string, changes map[string][]string) error {
	return nil
}
func (f *fakeLDAPToolClient) DeleteEntry(dn string) error { return nil }
func (f *fakeLDAPToolClient) TestConnection() error       { return f.testErr }

func decodeLDAPJSON(t *testing.T, out string) map[string]interface{} {
	t.Helper()
	if strings.HasPrefix(out, "Tool Output: ") {
		t.Fatalf("LDAP tool returned a dispatch prefix unexpectedly: %q", out)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", out, err)
	}
	return payload
}

func TestLDAPRequiresBindDN(t *testing.T) {
	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.Host = "ldap.example.com"
	cfg.LDAP.BaseDN = "dc=example,dc=com"

	out := LDAP(cfg, nil, "search", map[string]interface{}{"filter": "(objectClass=*)"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	payload := decodeLDAPJSON(t, out)
	if payload["status"] != "error" {
		t.Fatalf("payload = %v, want error", payload)
	}
	if payload["message"] != "LDAP bind_dn is not configured." {
		t.Fatalf("message = %q", payload["message"])
	}
}

func TestLDAPUsesVaultPasswordAndReturnsPlainJSON(t *testing.T) {
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
	cfg.LDAP.BaseDN = "dc=example,dc=com"
	cfg.LDAP.BindDN = "cn=svc,dc=example,dc=com"

	fake := &fakeLDAPToolClient{}
	orig := newLDAPClient
	newLDAPClient = func(clientCfg ldap.LDAPConfig) ldapClient {
		fake.lastConfig = clientCfg
		return fake
	}
	defer func() { newLDAPClient = orig }()

	out := LDAP(cfg, vault, "test_connection", nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	payload := decodeLDAPJSON(t, out)
	if payload["status"] != "success" {
		t.Fatalf("payload = %v, want success", payload)
	}
	if fake.lastConfig.BindPassword != "vault-secret" {
		t.Fatalf("BindPassword = %q, want vault-secret", fake.lastConfig.BindPassword)
	}
}

func TestLDAPScrubsConnectionErrors(t *testing.T) {
	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.Host = "ldap.example.com"
	cfg.LDAP.BaseDN = "dc=example,dc=com"
	cfg.LDAP.BindDN = "cn=svc,dc=example,dc=com"
	cfg.LDAP.BindPassword = "super-secret"

	fake := &fakeLDAPToolClient{
		connectAndBindErr: fmt.Errorf("bind failed for password super-secret"),
	}
	orig := newLDAPClient
	newLDAPClient = func(clientCfg ldap.LDAPConfig) ldapClient { return fake }
	defer func() { newLDAPClient = orig }()

	out := LDAP(cfg, nil, "search", map[string]interface{}{"filter": "(objectClass=*)"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if strings.Contains(out, "super-secret") {
		t.Fatalf("expected output to scrub secret, got %q", out)
	}
	payload := decodeLDAPJSON(t, out)
	if payload["status"] != "error" {
		t.Fatalf("payload = %v, want error", payload)
	}
}

func TestLDAPAddUserRequiresEntryAttributes(t *testing.T) {
	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.Host = "ldap.example.com"
	cfg.LDAP.BindDN = "cn=svc,dc=example,dc=com"

	out := LDAP(cfg, nil, "add_user", map[string]interface{}{"dn": "cn=jane,dc=example,dc=com"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	payload := decodeLDAPJSON(t, out)
	if payload["status"] != "error" {
		t.Fatalf("payload = %v, want error", payload)
	}
	if payload["message"] != "'entry_attributes' is required for add_user operation." {
		t.Fatalf("message = %q", payload["message"])
	}
}

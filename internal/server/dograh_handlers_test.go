package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestDograhStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/dograh/status", nil)
	rec := httptest.NewRecorder()

	handleDograhStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", body["status"])
	}
}

func TestEnsureDograhSecretsCreatesManagedStackSecrets(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	vault, err := security.NewVault(masterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	cfg := &config.Config{}
	cfg.Dograh.Enabled = true
	cfg.Dograh.Mode = "managed"
	cfg.Dograh.APIURL = "http://127.0.0.1:8000"
	cfg.Dograh.UIURL = "http://127.0.0.1:3010"
	s := &Server{Cfg: cfg, Vault: vault}

	if err := s.ensureDograhSecrets(cfg); err != nil {
		t.Fatalf("ensureDograhSecrets() error = %v", err)
	}

	for key, value := range map[string]string{
		"dograh_oss_jwt_secret":      cfg.Dograh.OSSJWTSecret,
		"dograh_postgres_password":   cfg.Dograh.PostgresPassword,
		"dograh_redis_password":      cfg.Dograh.RedisPassword,
		"dograh_minio_root_password": cfg.Dograh.MinioRootPassword,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("%s was not generated", key)
		}
		got, err := vault.ReadSecret(key)
		if err != nil {
			t.Fatalf("vault.ReadSecret(%q) error = %v", key, err)
		}
		if got != value {
			t.Fatalf("vault secret %q = %q, want generated value", key, got)
		}
	}
	if _, err := tools.ResolveDograhStackConfig(cfg, false); err != nil {
		t.Fatalf("ResolveDograhStackConfig() after ensureDograhSecrets error = %v", err)
	}
}

func TestDograhRegisterAuraGoMCPToolRequiresAPIKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Dograh.Enabled = true
	cfg.Dograh.MCPServerToolEnabled = true
	cfg.Dograh.ReadOnly = false
	cfg.Dograh.AuraGoMCPCredentialUUID = "credential-uuid"
	cfg.MCPServer.Enabled = true
	s := &Server{Cfg: cfg}

	req := httptest.NewRequest(http.MethodPost, "/api/dograh/register-aurago-mcp-tool", nil)
	rec := httptest.NewRecorder()

	handleDograhRegisterAuraGoMCPTool(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Dograh API key") {
		t.Fatalf("body = %s, want missing API key message", rec.Body.String())
	}
}

func TestDograhRegisterAuraGoMCPToolBlocksReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Dograh.Enabled = true
	cfg.Dograh.MCPServerToolEnabled = true
	cfg.Dograh.ReadOnly = true
	cfg.Dograh.APIKey = "dograh-api-key"
	cfg.Dograh.AuraGoMCPCredentialUUID = "credential-uuid"
	cfg.MCPServer.Enabled = true
	s := &Server{Cfg: cfg}

	req := httptest.NewRequest(http.MethodPost, "/api/dograh/register-aurago-mcp-tool", nil)
	rec := httptest.NewRecorder()

	handleDograhRegisterAuraGoMCPTool(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status code = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "read-only") {
		t.Fatalf("body = %s, want read-only message", rec.Body.String())
	}
}

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tsnetnode"
)

func TestValidateTsNetAuthKey(t *testing.T) {
	valid := "tskey-auth-abcdefghijklmnopqrstuvwxyz"
	if err := validateTsNetAuthKey(valid); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	for _, key := range []string{
		"",
		"not-a-tsnet-key-abcdefghijklmnopqrstuvwxyz",
		"tskey-auth-invalid value with spaces",
		strings.Repeat("x", 513),
	} {
		if err := validateTsNetAuthKey(key); err == nil {
			t.Fatalf("invalid key %q accepted", key)
		}
	}
}

func TestTsNetCredentialHandlerStoresOnlyNodeVaultKey(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	key := "tskey-auth-main-abcdefghijklmnopqrstuvwxyz"
	sharedKey := "tskey-auth-shared-abcdefghijklmnopqrstuvwxyz"
	if err := vault.WriteSecret("tailscale_tsnet_authkey", sharedKey); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"node": "main", "auth_key": key})
	req := httptest.NewRequest(http.MethodPost, "/api/tsnet/credentials", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	handleTsNetCredentials(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), key) {
		t.Fatal("credential response exposed the auth key")
	}
	stored, err := vault.ReadSecret("tailscale_tsnet_authkey_main")
	if err != nil {
		t.Fatalf("ReadSecret() error = %v", err)
	}
	if stored != key {
		t.Fatalf("stored key = %q, want submitted key", stored)
	}
	if cfg.Tailscale.TsNet.AuthKeyMain != key {
		t.Fatal("runtime config was not updated with the Vault-backed credential")
	}
	if shared, _ := vault.ReadSecret("tailscale_tsnet_authkey"); shared != sharedKey {
		t.Fatal("node credential unexpectedly changed the legacy shared credential")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/tsnet/credentials?node=main", nil)
	deleteReq.RemoteAddr = "203.0.113.11:12345"
	deleteRec := httptest.NewRecorder()
	handleTsNetCredentials(s).ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}
	if nodeKey, _ := vault.ReadSecret("tailscale_tsnet_authkey_main"); nodeKey != "" {
		t.Fatal("node-specific credential was not removed")
	}
	if shared, _ := vault.ReadSecret("tailscale_tsnet_authkey"); shared != sharedKey {
		t.Fatal("deleting a node credential removed the shared fallback")
	}
}

func TestTsNetCredentialHandlerRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	s := &Server{Cfg: &config.Config{}, Vault: vault, Logger: slog.Default()}
	for i, body := range []string{
		`{"node":"main","auth_key":"tskey-auth-abcdefghijklmnopqrstuvwxyz","unexpected":true}`,
		`{"node":"main","auth_key":"tskey-auth-abcdefghijklmnopqrstuvwxyz"} {}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/tsnet/credentials", strings.NewReader(body))
		req.RemoteAddr = "203.0.113." + string(rune('2'+i)) + ":12345"
		rec := httptest.NewRecorder()
		handleTsNetCredentials(s).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q returned %d, want 400", body, rec.Code)
		}
	}
}

func TestTsNetBodylessActionsAcceptOnlyAnEmptyObject(t *testing.T) {
	for _, handler := range []http.HandlerFunc{
		handleTsNetStart(&Server{}),
		handleTsNetStop(&Server{}),
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/tsnet/action", strings.NewReader(`{"unexpected":true}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unknown field returned %d, want 400", rec.Code)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/tsnet/action", strings.NewReader(`{}`))
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("empty object returned %d, want handler-level 503", rec.Code)
		}
	}
}

func TestTsNetStatusWithoutManagerPreservesLegacyFields(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tsnet/status", nil)
	handleTsNetStatus(&Server{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"enabled", "running"} {
		if _, ok := response[field]; !ok {
			t.Fatalf("legacy field %q is missing", field)
		}
	}
}

func TestGenericVaultRouteRejectsMalformedTsNetAuthKeyBeforeStorage(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	s := &Server{Cfg: &config.Config{}, Vault: vault, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(
		`{"key":"tailscale_tsnet_authkey","value":"malformed secret"}`,
	))
	rec := httptest.NewRecorder()
	handleSetVaultSecret(s, rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if stored, _ := vault.ReadSecret("tailscale_tsnet_authkey"); stored != "" {
		t.Fatal("malformed tsnet key was written to the Vault")
	}
}

func TestTsNetStartReturnsSynchronouslyRegisteredOperationID(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.StateDir = t.TempDir()
	manager := tsnetnode.NewManager(cfg, slog.Default())
	s := &Server{
		Cfg:          cfg,
		Logger:       slog.Default(),
		TsNetManager: manager,
		tsNetHandler: http.NotFoundHandler(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tsnet/start", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	handleTsNetStart(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != "starting" || strings.TrimSpace(response["operation_id"]) == "" {
		t.Fatalf("response does not contain a registered operation: %v", response)
	}
	status := manager.GetStatus()
	if status.Operation == nil || status.Operation.ID != response["operation_id"] {
		t.Fatalf("manager operation = %+v, response = %v", status.Operation, response)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = manager.Shutdown(shutdownCtx)
}

func TestTsNetReauthRejectsUnconfiguredNodeWithConflict(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Enabled = true
	manager := tsnetnode.NewManager(cfg, slog.Default())
	s := &Server{
		Cfg:          cfg,
		Logger:       slog.Default(),
		TsNetManager: manager,
		tsNetHandler: http.NotFoundHandler(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tsnet/reauth", strings.NewReader(
		`{"node":"manifest","mode":"normal","confirm_new_identity":false}`,
	))
	rec := httptest.NewRecorder()

	handleTsNetReauth(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["error_code"] != tsnetnode.ErrorNodeNotConfigured {
		t.Fatalf("error_code = %q, want %s", response["error_code"], tsnetnode.ErrorNodeNotConfigured)
	}
}

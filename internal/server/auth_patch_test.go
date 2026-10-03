package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestPatchAuthConfigRestoresVaultWhenReloadFails(t *testing.T) {
	s := newBootstrapTestServer(t, "127.0.0.1")
	if err := s.Vault.WriteSecret("auth_password_hash", "old-hash"); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}
	before := s.Cfg
	if err := os.Remove(s.Cfg.ConfigPath); err != nil { // config.Load will fail
		t.Fatalf("Remove config: %v", err)
	}

	err := patchAuthConfig(s, map[string]interface{}{"password_hash": "new-hash", "session_secret": "new-secret"})
	if err == nil {
		t.Fatal("patchAuthConfig succeeded although the config reload failed")
	}
	if got, _ := s.Vault.ReadSecret("auth_password_hash"); got != "old-hash" {
		t.Fatalf("auth_password_hash = %q after failed update, want old-hash", got)
	}
	if _, err := s.Vault.ReadSecret("auth_session_secret"); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("session secret left behind after failed update: %v", err)
	}
	if s.Cfg != before {
		t.Fatal("live snapshot replaced after a failed update")
	}
}

func TestPatchAuthConfigLeavesVaultUntouchedWhenYAMLCannotBeRead(t *testing.T) {
	s := newBootstrapTestServer(t, "127.0.0.1")
	if err := s.Vault.WriteSecret("auth_totp_secret", "JBSWY3DPEHPK3PXP"); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}
	s.Cfg.ConfigPath = t.TempDir() // a directory: reading config.yaml fails

	if err := patchAuthConfig(s, map[string]interface{}{"totp_secret": "", "totp_enabled": false}); err == nil {
		t.Fatal("patchAuthConfig succeeded although config.yaml could not be read")
	}
	if got, _ := s.Vault.ReadSecret("auth_totp_secret"); got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("auth_totp_secret = %q after failed update, want the previous secret", got)
	}
}

func TestPatchAuthConfigPersistsYAMLAndVaultTogether(t *testing.T) {
	s := newBootstrapTestServer(t, "127.0.0.1")
	if err := patchAuthConfig(s, map[string]interface{}{"totp_secret": "JBSWY3DPEHPK3PXP", "totp_enabled": true}); err != nil {
		t.Fatalf("patchAuthConfig: %v", err)
	}
	if !s.Cfg.Auth.TOTPEnabled || s.Cfg.Auth.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("live snapshot TOTP = %v/%q", s.Cfg.Auth.TOTPEnabled, s.Cfg.Auth.TOTPSecret)
	}
	data, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil || !strings.Contains(string(data), "totp_enabled: true") {
		t.Fatalf("config.yaml does not contain totp_enabled: true (%v)", err)
	}
}

func TestAuthTOTPDeleteWaitsForConfigSaveLock(t *testing.T) {
	s, cookie := newStepUpTestServer(t)
	s.CfgSaveMu.Lock()
	done := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		handleAuthTOTPDelete(s).ServeHTTP(rec, stepUpRequest(http.MethodDelete, "/api/auth/totp", `{"current_password":"`+stepUpTestPassword+`"}`, cookie))
		done <- rec.Code
	}()
	select {
	case code := <-done:
		s.CfgSaveMu.Unlock()
		t.Fatalf("TOTP delete finished (status %d) while the config save lock was held", code)
	case <-time.After(1500 * time.Millisecond):
	}
	s.CfgSaveMu.Unlock()
	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Fatalf("TOTP delete after unlock: status %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("TOTP delete never completed after the lock was released")
	}
}

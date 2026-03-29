package server

import (
	"aurago/internal/config"
	"testing"
)

func TestNeedsSetupRequiresPasswordWhenAuthEnabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.LLM.APIKey = "configured"
	cfg.Auth.Enabled = true

	if !needsSetup(cfg) {
		t.Fatal("expected setup to remain required while auth is enabled and no password is set")
	}

	cfg.Auth.PasswordHash = "hash"
	if needsSetup(cfg) {
		t.Fatal("expected setup to be complete once provider and password are configured")
	}
}

func TestExtractSetupAdminPasswordStripsTemporaryField(t *testing.T) {
	t.Parallel()

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled":        true,
			"admin_password": "supersecret",
		},
	}

	password, authEnabled, err := extractSetupAdminPassword(patch, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !authEnabled {
		t.Fatal("expected auth to stay enabled")
	}
	if password != "supersecret" {
		t.Fatalf("unexpected password %q", password)
	}

	authPatch := patch["auth"].(map[string]interface{})
	if _, exists := authPatch["admin_password"]; exists {
		t.Fatal("expected temporary admin_password field to be removed before config merge")
	}
}

func TestExtractSetupAdminPasswordAllowsExistingPasswordToRemain(t *testing.T) {
	t.Parallel()

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled": true,
		},
	}

	password, authEnabled, err := extractSetupAdminPassword(patch, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !authEnabled {
		t.Fatal("expected auth to stay enabled")
	}
	if password != "" {
		t.Fatalf("expected empty password when keeping existing one, got %q", password)
	}
}

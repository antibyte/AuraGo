package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestMCPSecretsFailedSavePreservesVaultAndRuntime(t *testing.T) {
	for _, failure := range []string{"missing file", "invalid YAML", "reload failure"} {
		t.Run(failure, func(t *testing.T) {
			s := auditMCPServer(t)
			s.Cfg.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
			s.Cfg.MCP.Secrets = []config.MCPSecret{{Alias: "removed"}, {Alias: "rotated"}}
			original := []byte("mcp: [invalid")
			if failure == "reload failure" {
				original = []byte("server:\n  port: [invalid]\nmcp:\n  secrets:\n    - alias: removed\n    - alias: rotated\n")
			}
			if failure != "missing file" {
				if err := os.WriteFile(s.Cfg.ConfigPath, original, 0600); err != nil {
					t.Fatal(err)
				}
			}
			vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			s.Vault = vault
			if err := vault.WriteSecrets(map[string]string{mcpSecretVaultKey("removed"): "fixture-old", mcpSecretVaultKey("rotated"): "fixture-old"}, nil); err != nil {
				t.Fatal(err)
			}
			old := shutdownExternalMCPManager
			defer func() { shutdownExternalMCPManager = old }()
			stops := 0
			shutdownExternalMCPManager = func() uint64 { stops++; return 1 }
			cfg := s.Cfg
			w := httptest.NewRecorder()
			handlePutMCPSecrets(s, w, httptest.NewRequest(http.MethodPut, "/api/mcp-secrets", strings.NewReader(`{"secrets":[{"alias":"rotated","value":"fixture-new"},{"alias":"created","value":"fixture-new"}]}`)))
			if w.Code != http.StatusInternalServerError || stops != 0 || s.Cfg != cfg {
				t.Fatalf("failed transaction published: status=%d stops=%d", w.Code, stops)
			}
			for _, alias := range []string{"removed", "rotated"} {
				value, err := vault.ReadSecret(mcpSecretVaultKey(alias))
				if err != nil || value != "fixture-old" {
					t.Fatalf("previous secret %s not restored: %v", alias, err)
				}
			}
			if _, err := vault.ReadSecret(mcpSecretVaultKey("created")); !errors.Is(err, security.ErrSecretNotFound) {
				t.Fatalf("new secret survived failed save: %v", err)
			}
			if failure != "missing file" {
				actual, err := os.ReadFile(s.Cfg.ConfigPath)
				if err != nil || string(actual) != string(original) {
					t.Fatalf("configuration not restored: %v", err)
				}
			}
		})
	}
}

package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/evomap"
	"aurago/internal/security"
)

func TestIntegrationSectionReloadsPreserveProviderCredentials(t *testing.T) {
	for _, name := range []string{"LLM_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		t.Setenv(name, "")
	}
	withFakeEvomapServerClient(t, fakeEvomapServerClient{register: evomap.RegisterResponse{NodeID: "reload-fixture"}})
	for _, provider := range []string{"manifest", "omniroute"} {
		for _, update := range []struct {
			name string
			run  func(*Server) error
		}{
			{"composio", func(s *Server) error {
				return persistComposioSectionUpdate(s, func(section map[string]interface{}) error {
					section["enabled"] = false
					return nil
				})
			}},
			{"mcp", func(s *Server) error {
				return persistMCPSectionUpdate(s, func(section map[string]interface{}) error {
					section["enabled"] = false
					return nil
				})
			}},
			{"evomap", func(s *Server) error {
				_, _, _, err := s.registerEvomapNode(context.Background(), s.ConfigSnapshot().Evomap)
				return err
			}},
		} {
			t.Run(provider+"/"+update.name, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "config.yaml")
				data := fmt.Sprintf("providers:\n  - id: selected\n    type: %s\n    model: auto\nllm:\n  provider: selected\n%s:\n  mode: external\n  external_base_url: https://provider.example.invalid/v1\nevomap:\n  enabled: true\n  readonly: false\n  base_url: https://evomap.example.invalid\nmcp:\n  enabled: false\n  servers: []\n", provider, provider)
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				cfg, err := config.Load(path)
				if err != nil {
					t.Fatal(err)
				}
				vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(dir, "vault.bin"))
				if err != nil {
					t.Fatal(err)
				}
				if err := vault.WriteSecret(provider+"_api_key", "sectionFixture123456"); err != nil {
					t.Fatal(err)
				}
				cfg.ApplyVaultSecrets(vault)
				cfg.ResolveProviders()
				cfg.ApplyOAuthTokens(vault)
				s := &Server{Cfg: cfg, Vault: vault, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
				if err := update.run(s); err != nil {
					t.Fatal(err)
				}
				updated := s.ConfigSnapshot()
				if updated.LLM.APIKey != "sectionFixture123456" || updated.FindProvider("selected").APIKey != "sectionFixture123456" {
					t.Fatal("integration reload lost section-backed provider credentials")
				}
				if updated.LLM.BaseURL != "https://provider.example.invalid/v1" {
					t.Fatal("integration reload changed the provider route")
				}
				stored, err := os.ReadFile(path)
				if err != nil || strings.Contains(string(stored), "sectionFixture123456") {
					t.Fatal("runtime provider credentials reached YAML")
				}
			})
		}
	}
}

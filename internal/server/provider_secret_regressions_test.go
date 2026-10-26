package server

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

type providerOAuthSecretReader map[string]string

func (r providerOAuthSecretReader) ReadSecret(key string) (string, error) {
	if value, ok := r[key]; ok {
		return value, nil
	}
	return "", security.ErrSecretNotFound
}

func TestProviderOAuthOpaqueTokensAreMaskedAndReleased(t *testing.T) {
	for _, source := range []string{"runtime", "slot", "vault_sip_only"} {
		t.Run(source, func(t *testing.T) {
			// Neither value passes the heuristic used for local static placeholders.
			first := "oauthopaquefirstlowercase" + strings.ReplaceAll(source, "_", "")
			second := strings.Repeat("9", 32) + strings.Repeat("8", len(source))
			cfg := &config.Config{Providers: []config.ProviderEntry{{ID: "opaque-side", AuthType: "oauth2"}}}
			var scope llmSecretScope
			t.Cleanup(func() { scope.replaceAccessTokens(nil) })
			register := func(token string) {
				var readers []config.SecretReader
				switch source {
				case "runtime":
					cfg.Providers[0].RuntimeOAuthToken = token
				case "slot":
					cfg.LLM.Provider, cfg.LLM.APIKey = "opaque-side", token
				case "vault_sip_only":
					cfg.SIP.Voice.AgentProviderID = "opaque-side"
					readers = append(readers, providerOAuthSecretReader{"oauth_opaque-side": `{"access_token":"` + token + `"}`})
				}
				scope.register(cfg, readers...)
			}
			register(first)
			if strings.Contains(security.Scrub("bearer "+first), first) {
				t.Fatal("known opaque OAuth token was not masked")
			}
			register(second)
			if strings.Contains(security.Scrub("bearer "+second), second) {
				t.Fatal("rotated opaque OAuth token was not masked")
			}
			if !strings.Contains(security.Scrub("retired "+first), first) {
				t.Fatal("retired token registration was not released")
			}
			register(maskedKey)
			if len(scope.releases) != 0 {
				t.Fatal("UI mask was registered as an OAuth credential")
			}
		})
	}
}

func providerSetupSecretPatch() map[string]interface{} {
	return map[string]interface{}{
		"ai_gateway": map[string]interface{}{"token": "syntheticNewGatewayToken123"},
		"three_d_printers": map[string]interface{}{"klipper": map[string]interface{}{"printers": []interface{}{
			map[string]interface{}{"id": "fixture-printer", "api_key": "syntheticPrinterKey123"},
		}}},
		"go2rtc": map[string]interface{}{"streams": []interface{}{
			map[string]interface{}{"id": "fixture-camera", "source": "rtsp://fixture:syntheticPassword123@camera.example.invalid/live"},
		}},
		"a2a": map[string]interface{}{"client": map[string]interface{}{"remote_agents": []interface{}{
			map[string]interface{}{"id": "fixture-agent", "api_key": "syntheticAgentKey123", "bearer_token": "syntheticBearerToken123"},
		}}},
	}
}

func TestProviderSetupPreparesAllSecretsBeforeWriting(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig)
	patch := providerSetupSecretPatch()
	mutations, err := prepareConfigSecrets(patch, s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 5 {
		t.Fatalf("prepared %d secret writes, want 5", len(mutations))
	}
	for _, mutation := range mutations {
		if _, err := vault.ReadSecret(mutation.key); !errors.Is(err, security.ErrSecretNotFound) {
			t.Fatal("preparation wrote to the Vault")
		}
	}
	if _, present := patch["ai_gateway"].(map[string]interface{})["token"]; present {
		t.Fatal("prepared secret remained in YAML patch")
	}

	// The normal config writer retains its typed-nil Vault error behavior.
	var missing *security.Vault
	if err := extractSecretsToVault(providerSetupSecretPatch(), missing, s.Logger); err == nil {
		t.Fatal("missing Vault accepted credentials")
	}
}

func TestProviderSetupReloadFailureRollsBackAllSecrets(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig)
	patch := providerSetupSecretPatch()
	mutations, err := prepareConfigSecrets(patch, s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	mutations = append(mutations, vaultMutation{key: "provider_main_api_key", value: "syntheticNewProviderKey123"})
	for _, key := range []string{"ai_gateway_token", "provider_main_api_key"} {
		if err := vault.WriteSecret(key, "syntheticPreviousKey123"); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, revision := s.ConfigSnapshot(), s.providerETag()
	_, err = saveProviderConfig(s, s.Cfg.ConfigPath, before, []byte("providers: [invalid YAML"), mutations)
	if err == nil {
		t.Fatal("invalid persisted config was accepted")
	}
	after, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed reload changed the config file")
	}
	if s.ConfigSnapshot() != snapshot || s.providerETag() != revision {
		t.Fatal("failed reload published a new snapshot or revision")
	}
	for _, mutation := range mutations {
		value, err := vault.ReadSecret(mutation.key)
		if mutation.key == "ai_gateway_token" || mutation.key == "provider_main_api_key" {
			if err != nil || value != "syntheticPreviousKey123" {
				t.Fatal("failed reload did not restore an existing credential")
			}
		} else if !errors.Is(err, security.ErrSecretNotFound) {
			t.Fatal("failed reload retained a newly created credential")
		}
	}
}

func TestProviderSetupSuccessfulSaveKeepsSecretsOutOfYAML(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig+"\nai_gateway:\n  enabled: false\n")
	patch := map[string]interface{}{
		"providers":  []interface{}{map[string]interface{}{"id": "main", "type": "openai", "model": "gpt-4o-mini", "api_key": "syntheticSavedProviderKey123"}},
		"ai_gateway": map[string]interface{}{"token": "syntheticSavedGatewayToken123"},
	}
	if _, err := applyConfigPatch(s, patch); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"provider_main_api_key": "syntheticSavedProviderKey123", "ai_gateway_token": "syntheticSavedGatewayToken123"} {
		if bytes.Contains(data, []byte(want)) {
			t.Fatal("saved YAML contains a plaintext credential")
		}
		if got, err := vault.ReadSecret(key); err != nil || got != want {
			t.Fatal("successful save did not persist its prepared credential")
		}
	}
}

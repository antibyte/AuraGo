package config

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type failingPersonalityVault struct {
	testSecretVault
	failRead, failWrite bool
}

func (v *failingPersonalityVault) ReadSecret(key string) (string, error) {
	if v.failRead {
		return "", errors.New("fixture read failure")
	}
	return v.testSecretVault.ReadSecret(key)
}
func (v *failingPersonalityVault) WriteSecret(key, value string) error {
	if v.failWrite {
		return errors.New("fixture write failure")
	}
	return v.testSecretVault.WriteSecret(key, value)
}

func TestPersonalityLegacySecretsStaySeparateFromHelper(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(map[bool]string{false: "ambiguous", true: "explicit_provider"}[bound], func(t *testing.T) {
			personality := map[string]interface{}{"v2_api_key": "old-personality-credential"}
			if bound {
				personality["v2_provider"] = "analysis"
				personality["v2_url"] = "https://analysis.example/v1"
			}
			root := map[string]interface{}{"personality": personality, "agent": map[string]interface{}{"personality_v2_api_key": "old-agent-credential"}, "llm": map[string]interface{}{"helper_provider": "helper"}, "providers": []interface{}{map[string]interface{}{"id": "analysis", "base_url": "https://analysis.example/v1"}, map[string]interface{}{"id": "helper", "base_url": "https://helper.example/v1"}}}
			vault := &testSecretVault{data: map[string]string{"provider_helper_api_key": "active-helper-credential"}}
			if !migratePersonalityLegacySecrets(root, vault, slog.Default()) {
				t.Fatal("nothing migrated")
			}
			if vault.data["provider_helper_api_key"] != "active-helper-credential" {
				t.Fatal("overwrote active helper")
			}
			if (vault.data["provider_analysis_api_key"] == "old-personality-credential") != bound {
				t.Fatal("wrong provider identity")
			}
			slots := 0
			for key := range vault.data {
				if strings.HasPrefix(key, "personality_legacy_") {
					slots++
				}
			}
			want := 2
			if bound {
				want = 1
			}
			if slots != want {
				t.Fatalf("escrow slots=%d want %d", slots, want)
			}
			if _, exists := personality["v2_api_key"]; exists {
				t.Fatal("plaintext survived successful save")
			}
		})
	}
}

func TestPersonalityLegacyVaultFailurePreservesYAMLAndCredentials(t *testing.T) {
	for _, failRead := range []bool{false, true} {
		section := map[string]interface{}{"v2_api_key": "legacy-credential", "v2_provider": "helper"}
		root := map[string]interface{}{"personality": section, "providers": []interface{}{map[string]interface{}{"id": "helper"}}}
		vault := &failingPersonalityVault{testSecretVault: testSecretVault{data: map[string]string{"provider_helper_api_key": "active-credential"}}, failRead: failRead, failWrite: !failRead}
		if migratePersonalityLegacySecrets(root, vault, slog.Default()) {
			t.Fatal("claimed unsuccessful Vault migration")
		}
		if section["v2_api_key"] != "legacy-credential" || vault.data["provider_helper_api_key"] != "active-credential" {
			t.Fatal("lost credential on Vault failure")
		}
	}
}

func TestPersonalityLegacyPatchRejectsBothSpellings(t *testing.T) {
	for _, path := range [][2]string{{"personality", "v2_api_key"}, {"agent", "personality_v2_api_key"}} {
		for _, value := range []interface{}{"credential", "", nil} {
			if err := ValidatePersonalityLegacyPatch(map[string]interface{}{path[0]: map[string]interface{}{path[1]: value}}); err == nil {
				t.Fatal("legacy key accepted")
			}
		}
	}
}

func TestPersonalityDeprecatedErrorStreakRemainsLoadable(t *testing.T) {
	data := []byte("personality:\n  inner_voice:\n    error_streak_min: 9\n")
	if len(PersonalityMigrationNotices(data)) != 1 {
		t.Fatal("missing deprecation notice")
	}
}

package config

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestPersonalityLegacyNumericSecretsSurviveDiskMigration(t *testing.T) {
	for _, path := range [][2]string{{"personality", "v2_api_key"}, {"agent", "personality_v2_api_key"}} {
		for _, rewrite := range []bool{false, true} {
			name := path[0] + "/unchanged"
			if rewrite {
				name = path[0] + "/other_secret_migrated"
			}
			t.Run(name, func(t *testing.T) {
				data := []byte(path[0] + ":\n  " + path[1] + ": 123456789\n")
				if rewrite {
					data = append(data, []byte("providers:\n  - id: unrelated\n    api_key: unrelated-credential\n")...)
				}
				var legacy Config
				if err := yaml.Unmarshal(data, &legacy); err != nil {
					t.Fatal(err)
				}
				if legacy.Personality.V2APIKey != "123456789" && legacy.Agent.LegacyPersonalityV2APIKey != "123456789" {
					t.Fatal("numeric credential was not supported by the legacy decoder")
				}
				configPath := filepath.Join(t.TempDir(), "config.yaml")
				if err := os.WriteFile(configPath, data, 0o600); err != nil {
					t.Fatal(err)
				}
				vault := &testSecretVault{data: map[string]string{"provider_helper_api_key": "active-helper-credential"}}
				var warnings bytes.Buffer
				log := slog.New(slog.NewTextHandler(&warnings, nil))
				MigratePlaintextSecretsToVault(configPath, vault, log)
				after, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				if !rewrite && !bytes.Equal(after, data) {
					t.Fatal("unsupported credential rewrote the source YAML")
				}
				var raw map[string]interface{}
				if err := yaml.Unmarshal(after, &raw); err != nil {
					t.Fatal(err)
				}
				section, _ := raw[path[0]].(map[string]interface{})
				if section[path[1]] != 123456789 {
					t.Fatal("numeric credential was removed or changed without Vault storage")
				}
				wantSecrets := 1
				if rewrite {
					wantSecrets++
					if vault.data["provider_unrelated_api_key"] != "unrelated-credential" {
						t.Fatal("independent credential migration did not run")
					}
				}
				if len(vault.data) != wantSecrets || vault.data["provider_helper_api_key"] != "active-helper-credential" {
					t.Fatal("legacy migration changed unrelated credentials")
				}
				if !strings.Contains(warnings.String(), path[0]+"."+path[1]) || !strings.Contains(warnings.String(), "not a string") || strings.Contains(warnings.String(), "123456789") {
					t.Fatal("migration did not provide a content-free field diagnostic")
				}
			})
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

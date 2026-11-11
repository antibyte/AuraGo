package invasion

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

func TestEggPermissionsRevocationSurvivesSubsequentModelPatch(t *testing.T) {
	egg := EggRecord{AllowedTools: `["shell","python"]`, AllowFilesystemWrite: true, AllowNetworkRequests: true, AllowUnsandboxedShell: true}
	data, err := GenerateEggConfig(minimalMasterCfg(), egg, NestRecord{}, "private-shared-fixture", "ws://master", "private-vault-fixture")
	if err != nil {
		t.Fatal(err)
	}
	no := false
	patch := SafeConfigPatch{AllowedTools: []string{}, AllowFilesystemWrite: &no, AllowNetworkRequests: &no, AllowUnsandboxedShell: &no}
	revoked, err := ApplySafeConfigPatch(data, patch)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := EggPermissionsJSON(revoked)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ApplyEggPermissions(data, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	model := "next-model"
	updated, err := ApplySafeConfigPatch(restored, SafeConfigPatch{Model: &model})
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(updated, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.AllowShell || cfg.Agent.AllowPython || cfg.Agent.AllowUnsandboxedShell || cfg.Agent.AllowFilesystemWrite || cfg.Agent.AllowNetworkRequests {
		t.Fatal("a subsequent model patch restored revoked permissions")
	}
	encoded, err := PatchToJSON(patch)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &stored); err != nil {
		t.Fatal(err)
	}
	if string(stored["allowed_tools"]) != "[]" {
		t.Fatal("empty tool revocation was omitted from the revision")
	}
	if _, err := ApplyEggPermissions(data, ""); err == nil {
		t.Fatal("legacy permissions accepted")
	}
}

func TestEggConfigContainsNoBootstrapSecrets(t *testing.T) {
	master := minimalMasterCfg()
	master.LLM.APIKey = "private-provider-bootstrap-fixture"
	data, err := GenerateEggConfig(master, EggRecord{InheritLLM: true}, NestRecord{}, "private-shared-bootstrap-fixture", "ws://master", "private-master-bootstrap-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{master.LLM.APIKey, "private-shared-bootstrap-fixture", "private-master-bootstrap-fixture", "api_key:", "master_key:", "shared_key:"} {
		if strings.Contains(string(data), value) {
			t.Fatalf("bootstrap secret field or value reached configuration")
		}
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.AllowShell || cfg.Agent.AllowPython || cfg.Agent.AllowUnsandboxedShell || cfg.Agent.AllowFilesystemWrite || cfg.Agent.AllowNetworkRequests {
		t.Fatal("empty tool list granted capabilities")
	}
	if cfg.EggMode.SecurityPolicyVersion != 1 {
		t.Fatal("security version missing")
	}
}

func TestEggPolicyRejectsLegacyAndTLSExceptions(t *testing.T) {
	cfg := &config.Config{}
	cfg.EggMode.Enabled = true
	if ValidateEggPolicy(cfg) == nil {
		t.Fatal("legacy configuration accepted")
	}
	cfg.EggMode.SecurityPolicyVersion = 1
	cfg.EggMode.TLSSkipVerify = true
	cfg.EggMode.TLSPinSHA256 = strings.Repeat("a", 64)
	if ValidateEggPolicy(cfg) == nil {
		t.Fatal("legacy TLS exception accepted even with a pin")
	}
}

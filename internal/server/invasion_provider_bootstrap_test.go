package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/invasion"

	"gopkg.in/yaml.v3"
)

func TestEggSavedProviderBootstrapUsesMatchingVaultKey(t *testing.T) {
	s := newSharedKeyTestServer(t, "")
	manual := false
	cfg := &config.Config{Providers: []config.ProviderEntry{{ID: "worker-model", Type: "openrouter", BaseURL: "https://fixture.invalid/v1", Model: "saved-model", Capabilities: config.ProviderCapabilities{Auto: &manual, ToolCalling: true}}}}
	egg := invasion.EggRecord{Provider: "worker-model", Model: "override-model"}
	if err := s.Vault.WriteSecret("provider_worker-model_api_key", "fixture-provider-key"); err != nil {
		t.Fatal(err)
	}
	for _, override := range []bool{false, true} {
		want := "fixture-provider-key"
		if override {
			egg.APIKeyRef = "explicit-egg-key"
			want = "fixture-explicit-key"
			if err := s.Vault.WriteSecret(egg.APIKeyRef, want); err != nil {
				t.Fatal(err)
			}
		}
		data, err := s.eggBootstrap(cfg, egg, "", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		var bootstrap invasion.EggBootstrap
		if err := json.Unmarshal(data, &bootstrap); err != nil {
			t.Fatal(err)
		}
		if bootstrap.Secrets["provider_main_api_key"] != want {
			t.Fatal("wrong provider key in private bootstrap")
		}
		generated, err := invasion.GenerateEggConfig(cfg, egg, invasion.NestRecord{}, "", "wss://fixture.invalid", "")
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Providers []config.ProviderEntry `yaml:"providers"`
		}
		if err := yaml.Unmarshal(generated, &parsed); err != nil {
			t.Fatal(err)
		}
		p := parsed.Providers[0]
		if p.Type != "openrouter" || p.ID != "main" || p.Model != "override-model" || p.BaseURL != "https://fixture.invalid/v1" || !p.Capabilities.ToolCalling {
			t.Fatal("saved provider metadata lost")
		}
		if strings.Contains(string(generated), want) || strings.Contains(string(generated), "api_key:") {
			t.Fatal("provider key leaked into YAML")
		}
	}
	cfg.Providers[0].AuthType = "oauth2"
	if _, err := s.eggBootstrap(cfg, egg, "", "", nil); err == nil {
		t.Fatal("OAuth session accepted for static bootstrap")
	}
	cfg.Providers = []config.ProviderEntry{{ID: "keyless", Type: "ollama", BaseURL: "https://fixture.invalid"}}
	egg.Provider, egg.APIKeyRef = "keyless", ""
	for _, noVault := range []bool{false, true} {
		if noVault {
			s.Vault = nil
		}
		if _, err := s.eggBootstrap(cfg, egg, "", "", nil); err != nil {
			t.Fatalf("keyless provider rejected: %v", err)
		}
	}
}

func TestEggReconfigureAndRollbackKeepSelectedProvider(t *testing.T) {
	s := newSharedKeyTestServer(t, "")
	cfg := &config.Config{Providers: []config.ProviderEntry{
		{ID: "old", Type: "openai", BaseURL: "https://old.invalid", Model: "old-model", APIKey: "fixture-old-provider-key"},
		{ID: "selected", Type: "anthropic", BaseURL: "https://selected.invalid", Model: "selected-model", APIKey: "fixture-selected-provider-key"},
	}}
	egg := invasion.EggRecord{Provider: "old", InheritLLM: true, BaseURL: "https://old-override.invalid", Model: "old-override"}
	selectedID, model, grant := "selected", "later-model", true
	selected, applied, err := prepareEggConfigPatch(cfg, egg, invasion.SafeConfigPatch{}, invasion.SafeConfigPatch{Provider: &selectedID})
	if err != nil {
		t.Fatal(err)
	}
	if selected.InheritLLM || selected.BaseURL != "" || selected.Model != "" {
		t.Fatal("provider change retained the old endpoint/model override")
	}
	// Store and reopen the revision, then submit only a model edit. Previous grants are not replayed.
	applied.AllowNetworkRequests = &grant
	record, err := invasion.PatchToJSON(applied)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := invasion.JSONToPatch(record)
	if err != nil {
		t.Fatal(err)
	}
	selected, effective, err := prepareEggConfigPatch(cfg, egg, prior, invasion.SafeConfigPatch{Model: &model})
	if err != nil {
		t.Fatal(err)
	}
	if effective.AllowNetworkRequests != nil || selected.Provider != selectedID || selected.InheritLLM {
		t.Fatal("partial patch changed the binding or resurrected a grant")
	}
	for _, state := range []invasion.SafeConfigPatch{applied, effective} {
		worker, replay, err := prepareEggConfigPatch(cfg, egg, invasion.SafeConfigPatch{}, state)
		if err != nil {
			t.Fatal(err)
		}
		data, err := invasion.GenerateEggConfig(cfg, worker, invasion.NestRecord{}, "", "wss://fixture.invalid", "")
		if err != nil {
			t.Fatal(err)
		}
		replay.Provider, replay.InheritLLM = nil, nil
		data, err = invasion.ApplySafeConfigPatch(data, replay)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Providers []config.ProviderEntry `yaml:"providers"`
		}
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		if parsed.Providers[0].Type != "anthropic" || parsed.Providers[0].BaseURL != "https://selected.invalid" {
			t.Fatal("patch used an ID as a provider type or reused the old endpoint")
		}
		bootstrap, err := s.eggBootstrap(cfg, worker, "", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		var private invasion.EggBootstrap
		if err := json.Unmarshal(bootstrap, &private); err != nil {
			t.Fatal(err)
		}
		if private.Secrets["provider_main_api_key"] != "fixture-selected-provider-key" {
			t.Fatal("reconfiguration/rollback selected another provider's key")
		}
	}
	unknown := "missing-provider"
	if _, _, err := prepareEggConfigPatch(cfg, egg, prior, invasion.SafeConfigPatch{Provider: &unknown}); err == nil {
		t.Fatal("unknown provider ID accepted")
	}
}

func TestEggRevisionCannotTransferGrantsAcrossAssignments(t *testing.T) {
	db, err := invasion.InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	oldID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "old"})
	if err != nil {
		t.Fatal(err)
	}
	newID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "new"})
	if err != nil {
		t.Fatal(err)
	}
	nestID, err := invasion.CreateNest(db, invasion.NestRecord{Name: "fixture", EggID: newID})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.InvasionControl.Enabled = true
	base, err := invasion.GenerateEggConfig(cfg, invasion.EggRecord{ID: newID}, invasion.NestRecord{ID: nestID}, "", "wss://fixture.invalid", "")
	if err != nil {
		t.Fatal(err)
	}
	permissions, err := invasion.EggPermissionsJSON(base)
	if err != nil {
		t.Fatal(err)
	}
	revID, err := invasion.CreateSafeConfigRevision(db, nestID, oldID, "{}", "fixture", "safe_reconfigure", permissions)
	if err != nil {
		t.Fatal(err)
	}
	if err := invasion.UpdateSafeConfigRevisionStatus(db, revID, "applied", ""); err != nil {
		t.Fatal(err)
	}
	if err := invasion.UpdateNestConfigRev(db, nestID, revID, revID); err != nil {
		t.Fatal(err)
	}
	nest, err := invasion.GetNest(db, nestID)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, InvasionDB: db}
	if _, err := s.restoreEggPermissions(nest, base, invasion.SafeConfigPatch{}); err == nil {
		t.Fatal("another Egg's permission snapshot was restored")
	}
	deny := false
	reviewed := invasion.SafeConfigPatch{AllowedTools: []string{}, AllowFilesystemWrite: &deny, AllowNetworkRequests: &deny, AllowUnsandboxedShell: &deny}
	if data, err := s.restoreEggPermissions(nest, base, reviewed); err != nil || !bytes.Equal(data, base) {
		t.Fatal("explicit review could not start from the new Egg's own configuration")
	}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"revision_id": revID})
	handleInvasionNestConfigRollback(s)(rec, httptest.NewRequest(http.MethodPost, "/api/invasion/nests/"+nestID+"/config-rollback", bytes.NewReader(body)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("cross-Egg rollback status=%d", rec.Code)
	}
}

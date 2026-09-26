package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTaskRouterConfigDefaultsAndExplicitClears(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("llm_router:\n  enabled: true\n  helper_fallback: false\n  helper_max_calls_per_hour: 0\n  areas:\n    coding: {provider: '', model: ''}\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.LLMRouter.Enabled || cfg.LLMRouter.HelperFallback || cfg.LLMRouter.HelperMaxCallsPerHour != 0 || cfg.LLMRouter.HelperTimeoutMS != 1500 || len(cfg.LLMRouter.Areas) != 9 {
		t.Fatalf("defaults or explicit values lost: %+v", cfg.LLMRouter)
	}
	clone := cfg.Clone()
	clone.LLMRouter.Areas["coding"] = LLMRouterTarget{Provider: "changed"}
	if cfg.LLMRouter.Areas["coding"].Provider != "" {
		t.Fatal("clone shares router assignments")
	}
	data, err := yaml.Marshal(cfg.LLMRouter)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip LLMRouterConfig
	if err = yaml.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if roundtrip.HelperFallback || roundtrip.HelperMaxCallsPerHour != 0 {
		t.Fatal("explicit false/zero lost")
	}
}

func TestTaskRouterLoadLegacyAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  ui_language: de\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMRouter.Enabled || !cfg.LLMRouter.HelperFallback || len(cfg.LLMRouter.Areas) != 9 {
		t.Fatal("legacy config changed routing behavior")
	}
	cfg.LLMRouter.Enabled = true
	cfg.LLMRouter.HelperFallback = false
	cfg.LLMRouter.HelperMaxCallsPerHour = 0
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.LLMRouter.Enabled || reloaded.LLMRouter.HelperFallback || reloaded.LLMRouter.HelperMaxCallsPerHour != 0 {
		t.Fatal("runtime Save lost router settings")
	}
}

func TestTaskRouterConfigValidation(t *testing.T) {
	cfg := Config{}
	cfg.LLMRouter = DefaultLLMRouterConfig()
	cfg.LLMRouter.Areas["coding"] = LLMRouterTarget{Provider: "removed"}
	if err := ValidateLLMRouterConfig(&cfg, false); err != nil {
		t.Fatalf("stale reference must permit startup: %v", err)
	}
	if err := ValidateLLMRouterConfig(&cfg, true); err == nil {
		t.Fatal("save accepted missing provider")
	}
	cfg.LLMRouter.Areas["coding"] = LLMRouterTarget{Model: "orphan"}
	if err := ValidateLLMRouterConfig(&cfg, false); err == nil {
		t.Fatal("accepted orphan model")
	}
	cfg.LLMRouter.Areas["coding"] = LLMRouterTarget{}
	cfg.LLMRouter.HelperMaxCallsPerHour = -1
	if err := ValidateLLMRouterConfig(&cfg, true); err == nil {
		t.Fatal("accepted negative quota")
	}
	cfg.LLMRouter = DefaultLLMRouterConfig()
	cfg.LLMRouter.HelperTimeoutMS = 0
	if err := ValidateLLMRouterConfig(&cfg, true); err == nil {
		t.Fatal("accepted an explicit zero deadline")
	}
	if err := ValidateLLMRouterConfig(&Config{}, true); err != nil {
		t.Fatal("missing legacy section must remain valid", err)
	}
}

func TestTaskRouterZeroValueSnapshotReload(t *testing.T) {
	data, err := yaml.Marshal(LLMRouterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var restored LLMRouterConfig
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Enabled || restored.HelperFallback || restored.HelperMaxCallsPerHour != 0 || restored.HelperTimeoutMS != 1500 {
		t.Fatalf("legacy snapshot changed switches or lost default deadline: %+v", restored)
	}
	if err := ValidateLLMRouterConfig(&Config{LLMRouter: restored}, false); err != nil {
		t.Fatal(err)
	}
}

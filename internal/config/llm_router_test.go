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

func TestTaskRouterAgnesEffectiveModelValidation(t *testing.T) {
	for _, tc := range []struct {
		name, model, override string
		valid                 bool
	}{
		{"configured chat", "agnes-3.0-flash", "", true},
		{"chat override", "agnes-3.0-flash", "agnes-2.5-flash", true},
		{"explicit chat on empty provider", "", "agnes-3.0-flash", true},
		{"explicit chat on media provider", "agnes-image-2.1-flash", "agnes-3.0-flash", true},
		{"image default", "agnes-image-2.1-flash", "", false},
		{"video default", "agnes-video-v2.0", "", false},
		{"image override", "agnes-3.0-flash", "agnes-image-2.1-flash", false},
		{"video override", "agnes-3.0-flash", "agnes-video-v2.0", false},
		{"qualified media override", "agnes-3.0-flash", " AGNES/Agnes-Image-2.1-flash ", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Providers: []ProviderEntry{{ID: "agnesai", Type: "agnes", Model: tc.model}}, LLMRouter: DefaultLLMRouterConfig()}
			cfg.LLMRouter.Areas["coding"] = LLMRouterTarget{Provider: "agnesai", Model: tc.override}
			for _, strict := range []bool{false, true} {
				if err := ValidateLLMRouterConfig(&cfg, strict); (err == nil) != tc.valid {
					t.Fatalf("strict=%v: valid=%v, error=%v", strict, tc.valid, err)
				}
			}
			if cfg.Providers[0].Model != tc.model {
				t.Fatal("validation changed the provider's default model")
			}
		})
	}
}

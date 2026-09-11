package config

import "testing"

func TestLocalMusicProviderSwitchClearsCloudCredentials(t *testing.T) {
	c := &Config{}
	c.MusicGeneration.Enabled = true
	c.Providers = []ProviderEntry{{ID: "cloud", Type: "minimax", APIKey: "cloud-test-key", BaseURL: "https://example.test", Model: "music"}}
	c.MusicGeneration.Provider = "cloud"
	c.ResolveProviders()
	if c.MusicGeneration.APIKey == "" {
		t.Fatal("cloud not resolved")
	}
	c.MusicGeneration.Provider = LocalMusicProviderID
	c.ResolveProviders()
	if !c.MusicConfigured() || c.MusicGeneration.APIKey != "" || c.MusicGeneration.BaseURL != "" || c.MusicGeneration.ProviderType != "acestep" {
		t.Fatal("local inherited cloud configuration")
	}
	c.MusicGeneration.Provider = "cloud"
	c.ResolveProviders()
	if c.MusicGeneration.APIKey != "cloud-test-key" {
		t.Fatal("cloud switch regression")
	}
	c.LLM.Provider = LocalMusicProviderID
	if err := ValidateLocalMusicConfig(c); err == nil {
		t.Fatal("music provider accepted for LLM")
	}
}

func TestLocalMusicSettingsBounds(t *testing.T) {
	zero := 0.0
	c := LocalMusicConfig{VRAMReserveGB: &zero}.Defaults()
	if *c.VRAMReserveGB != 0 || c.TimeoutSeconds != 1800 || c.Backend != "auto" {
		t.Fatal("defaults lost explicit zero")
	}
	for _, c := range []LocalMusicConfig{{Backend: "metal"}, {Device: "../gpu"}, {TimeoutSeconds: 1801}, {TimeoutSeconds: 1}} {
		if c.Validate() == nil {
			t.Errorf("invalid configuration accepted: %+v", c)
		}
	}
	if (LocalMusicConfig{Backend: "cpu"}).Validate() != nil {
		t.Fatal("explicit CPU unsupported")
	}
}

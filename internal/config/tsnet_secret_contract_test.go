package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTsNetCredentialsAreNeverSerialized(t *testing.T) {
	cfg := &Config{}
	cfg.Tailscale.APIKey = "tailscale-api-secret"
	cfg.Tailscale.TsNet.AuthKey = "tskey-auth-shared-secret"
	cfg.Tailscale.TsNet.AuthKeyMain = "tskey-auth-main-secret"
	cfg.Tailscale.TsNet.AuthKeyManifest = "tskey-auth-manifest-secret"
	cfg.Tailscale.TsNet.AuthKeySpaceAgent = "tskey-auth-space-secret"

	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	jsonData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	for _, serialized := range []struct {
		name string
		data string
	}{
		{name: "YAML", data: string(yamlData)},
		{name: "JSON", data: string(jsonData)},
	} {
		for _, forbidden := range []string{
			"tailscale-api-secret",
			"tskey-auth-shared-secret",
			"tskey-auth-main-secret",
			"tskey-auth-manifest-secret",
			"tskey-auth-space-secret",
			"auth_key_main",
			"auth_key_manifest",
			"auth_key_space_agent",
		} {
			if strings.Contains(serialized.data, forbidden) {
				t.Fatalf("%s serialization leaked %q", serialized.name, forbidden)
			}
		}
	}
}

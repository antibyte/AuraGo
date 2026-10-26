package server

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/agodesk"
	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

func TestCloudflareProviderGatewaySurvivesAllYAMLAndJSONProjections(t *testing.T) {
	p := config.ProviderEntry{ID: "rest", Type: "cloudflare-rest", AccountID: "account", GatewayID: "gateway", Model: "google/gemini-2.5-flash"}
	node, err := marshalConfigWithProviderEntries([]byte("providers: []\n"), []config.ProviderEntry{p})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []any{buildProviderYAMLEntry(p), node} {
		var data []byte
		if raw, ok := input.([]byte); ok {
			data = raw
		} else {
			data, err = yaml.Marshal(map[string]any{"providers": []any{input}})
		}
		if err != nil {
			t.Fatal(err)
		}
		var cfg config.Config
		if err = yaml.Unmarshal(data, &cfg); err != nil || len(cfg.Providers) != 1 || cfg.Providers[0].GatewayID != p.GatewayID {
			t.Fatalf("gateway lost: %s, %v", data, err)
		}
	}
	for _, v := range []any{providerJSON{GatewayID: p.GatewayID}, agodesk.ConfigProviderEntryPayload{GatewayID: p.GatewayID}} {
		b, _ := json.Marshal(v)
		if !strings.Contains(string(b), `"gateway_id":"gateway"`) {
			t.Fatal("gateway missing from provider API")
		}
	}
}

package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTregDefaultsValidationAndSecrets(t *testing.T) {
	for _, raw := range []string{"{}", "enabled: true", "max_call_cost_micro: 0"} {
		var c TregConfig
		if err := yaml.Unmarshal([]byte(raw), &c); err != nil {
			t.Fatal(err)
		}
		if !c.ReadOnly || (raw != "max_call_cost_micro: 0" && c.MaxCallCostMicro != 1_000_000) {
			t.Fatalf("defaults lost: %+v", c)
		}
		c.Token = "fixture-secret"
		y, _ := yaml.Marshal(c)
		j, _ := json.Marshal(c)
		if strings.Contains(string(y)+string(j), c.Token) {
			t.Fatal("serialized token")
		}
	}
	for _, raw := range []string{"max_call_cost_micro: -1", "max_call_cost_micro: 0.5", "allowed_endpoints: [{endpoint_id: x, method: GET, path: /x}]", "allowed_endpoints: [{endpoint_id: '../x', method: GET, path: /x, operation: read}]"} {
		var c TregConfig
		if yaml.Unmarshal([]byte(raw), &c) == nil {
			t.Fatalf("accepted invalid configuration %s", raw)
		}
	}
}

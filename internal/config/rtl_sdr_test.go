package config

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestRTLSDRConfigBoundsAndDefaults(t *testing.T) {
	for _, raw := range []string{`{"quota_gb":-1}`, `{"quota_gb":1001}`, `{"device":"/dev/sda"}`} {
		var c RTLSDRConfig
		if json.Unmarshal([]byte(raw), &c) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	var c RTLSDRConfig
	if err := yaml.Unmarshal([]byte("device: 3-2.4\nquota_gb: 10\n"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Enabled || c.AllowAgent || c.QuotaGB != 10 {
		t.Fatalf("unsafe defaults: %+v", c)
	}
	if err := json.Unmarshal([]byte(`{"allow_agent":true}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.Device != "3-2.4" || c.QuotaGB != 10 {
		t.Fatal("partial config patch erased values")
	}
}

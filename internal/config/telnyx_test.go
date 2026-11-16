package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestTelnyxOwnerRequiresExplicitAllowedE164(t *testing.T) {
	for _, tc := range []struct {
		owner   string
		allowed []string
		valid   bool
	}{
		{"", []string{"+15550000001"}, true},
		{"+15550000001", []string{"+15550000001"}, true},
		{"+15550000001", nil, false},
		{"+15550000002", []string{"+15550000001"}, false},
		{"15550000001", []string{"15550000001"}, false},
		{" +15550000001", []string{"+15550000001"}, false},
	} {
		cfg := &Config{}
		cfg.Telnyx.OwnerNumber, cfg.Telnyx.AllowedNumbers = tc.owner, tc.allowed
		if err := ValidateTelnyxOwner(cfg); (err == nil) != tc.valid {
			t.Fatalf("owner validation: %v", err)
		}
	}
	var cfg Config
	if err := yaml.Unmarshal([]byte("telnyx:\n  allowed_numbers: ['+15550000001']\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Telnyx.OwnerNumber != "" {
		t.Fatal("old configuration gained owner rights")
	}
	cfg.Telnyx.OwnerNumber = "+15550000001"
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	var restored Config
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Telnyx.OwnerNumber != cfg.Telnyx.OwnerNumber || ValidateTelnyxOwner(&restored) != nil {
		t.Fatal("owner binding lost on round trip")
	}
}

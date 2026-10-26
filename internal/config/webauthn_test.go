package config

import "testing"

func TestValidateWebAuthnConfig(t *testing.T) {
	valid := WebAuthnConfig{Enabled: true, LoginMode: "passkey", RPID: "aurago.example.test", Origins: []string{"https://aurago.example.test"}}
	for _, tc := range []struct {
		name   string
		change func(*WebAuthnConfig)
		valid  bool
	}{
		{"passkey", func(c *WebAuthnConfig) {}, true},
		{"second factor", func(c *WebAuthnConfig) { c.LoginMode = "second_factor" }, true},
		{"disabled default", func(c *WebAuthnConfig) { *c = WebAuthnConfig{} }, true},
		{"localhost", func(c *WebAuthnConfig) { c.RPID = "localhost"; c.Origins = []string{"http://localhost:8123"} }, true},
		{"multiple ports", func(c *WebAuthnConfig) { c.Origins = append(c.Origins, "https://aurago.example.test:8443") }, true},
		{"missing mode", func(c *WebAuthnConfig) { c.LoginMode = "" }, false},
		{"unknown mode", func(c *WebAuthnConfig) { c.LoginMode = "both" }, false},
		{"public suffix", func(c *WebAuthnConfig) { c.RPID = "com"; c.Origins = []string{"https://com"} }, false},
		{"IP", func(c *WebAuthnConfig) { c.RPID = "192.168.1.1"; c.Origins = []string{"https://192.168.1.1"} }, false},
		{"scheme in RP", func(c *WebAuthnConfig) { c.RPID = "https://aurago.example.test" }, false},
		{"foreign origin", func(c *WebAuthnConfig) { c.Origins = []string{"https://evil.test"} }, false},
		{"suffix confusion", func(c *WebAuthnConfig) { c.Origins = []string{"https://evilaurago.example.test"} }, false},
		{"HTTP", func(c *WebAuthnConfig) { c.Origins = []string{"http://aurago.example.test"} }, false},
		{"wildcard", func(c *WebAuthnConfig) { c.Origins = []string{"https://*.aurago.example.test"} }, false},
		{"userinfo", func(c *WebAuthnConfig) { c.Origins = []string{"https://user@aurago.example.test"} }, false},
		{"path", func(c *WebAuthnConfig) { c.Origins = []string{"https://aurago.example.test/"} }, false},
		{"query", func(c *WebAuthnConfig) { c.Origins = []string{"https://aurago.example.test?"} }, false},
		{"fragment", func(c *WebAuthnConfig) { c.Origins = []string{"https://aurago.example.test#bad"} }, false},
		{"empty fragment", func(c *WebAuthnConfig) { c.Origins = []string{"https://aurago.example.test#"} }, false},
		{"empty port", func(c *WebAuthnConfig) { c.Origins = []string{"https://aurago.example.test:"} }, false},
		{"invalid port", func(c *WebAuthnConfig) { c.Origins = []string{"https://aurago.example.test:65536"} }, false},
		{"no origins", func(c *WebAuthnConfig) { c.Origins = nil }, false},
		{"duplicate", func(c *WebAuthnConfig) { c.Origins = append(c.Origins, c.Origins[0]) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			c.Origins = append([]string(nil), valid.Origins...)
			tc.change(&c)
			if err := ValidateWebAuthnConfig(c); (err == nil) != tc.valid {
				t.Fatalf("validation: %v; want valid %v", err, tc.valid)
			}
		})
	}
}

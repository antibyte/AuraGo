package config

import "testing"

func TestMigrateEmailAccountsPreservesLegacyDisabledState(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := &Config{}
		cfg.Email.Enabled = enabled
		cfg.Email.IMAPHost = "mail.example.test"
		cfg.Email.Username = "owner@example.test"
		cfg.Email.Password = "fixture-password"
		cfg.Email.WatchEnabled = true
		cfg.Email.ReadOnly = true
		cfg.MigrateEmailAccounts()
		if len(cfg.EmailAccounts) != 1 {
			t.Fatal("legacy account was not migrated")
		}
		got := cfg.EmailAccounts[0]
		if got.Disabled == enabled || !got.ReadOnly || !got.WatchEnabled || got.Password != cfg.Email.Password {
			t.Fatalf("legacy policy was not preserved for enabled=%t", enabled)
		}
		cfg.MigrateEmailAccounts()
		if len(cfg.EmailAccounts) != 1 {
			t.Fatal("migration repeated")
		}
	}
}

func TestMigrateEmailAccountsKeepsModernAccountPolicy(t *testing.T) {
	cfg := &Config{EmailAccounts: []EmailAccount{{ID: "modern", IMAPHost: "mail.example.test"}}}
	cfg.Email.Enabled = false
	cfg.Email.IMAPHost = "legacy.example.test"
	cfg.MigrateEmailAccounts()
	if len(cfg.EmailAccounts) != 1 || cfg.EmailAccounts[0].ID != "modern" || cfg.EmailAccounts[0].Disabled {
		t.Fatal("legacy switch changed a modern account")
	}
}

func TestValidateAgentMailBaseURL(t *testing.T) {
	for _, raw := range []string{"", "https://api.agentmail.to", "https://mail.example.test:8443/api/v0/"} {
		if err := ValidateAgentMailBaseURL(raw); err != nil {
			t.Fatalf("valid URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"http://mail.example.test", "https://user:password@mail.example.test", "https://", "https://mail.example.test?", "https://mail.example.test?key=private", "https://mail.example.test#", "https://mail.example.test#private", "//mail.example.test"} {
		if err := ValidateAgentMailBaseURL(raw); err == nil {
			t.Fatalf("accepted invalid base URL %q", raw)
		}
	}
}

func TestValidateAgentMailWebSocketURL(t *testing.T) {
	for _, raw := range []string{"", "wss://ws.agentmail.to/v0", "WSS://relay.example.test:8443/v0"} {
		if err := ValidateAgentMailWebSocketURL(raw); err != nil {
			t.Fatalf("valid URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"ws://ws.agentmail.to/v0", "https://ws.agentmail.to", "wss://", "wss://u:p@relay.example.test", "wss://relay.example.test?key=x", "wss://relay.example.test#f", "//relay.example.test"} {
		if err := ValidateAgentMailWebSocketURL(raw); err == nil {
			t.Fatalf("accepted invalid websocket URL %q", raw)
		}
	}
}

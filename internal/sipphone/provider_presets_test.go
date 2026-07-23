package sipphone

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestSIPProviderCatalogIsBroadSecretFreeAndDeterministic(t *testing.T) {
	presets := SIPProviderPresets()
	if len(presets) < 50 {
		t.Fatalf("provider catalog is unexpectedly small: %d", len(presets))
	}
	seen := make(map[string]struct{}, len(presets))
	for index, preset := range presets {
		if index > 0 && presets[index-1].ID >= preset.ID {
			t.Fatalf("catalog is not sorted at %q", preset.ID)
		}
		if _, exists := seen[preset.ID]; exists {
			t.Fatalf("duplicate provider ID %q", preset.ID)
		}
		seen[preset.ID] = struct{}{}
		if preset.Name == "" || preset.Category == "" || !strings.HasPrefix(preset.DocumentationURL, "https://") {
			t.Fatalf("incomplete provider preset: %+v", preset)
		}
		if len(preset.Fields) < 3 || len(preset.Fields) > 5 {
			t.Fatalf("%s asks for %d fields; guided setup permits three to five", preset.ID, len(preset.Fields))
		}
	}
	encoded, err := json.Marshal(presets)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{`"password":"`, "authorization", "private_key"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("provider catalog contains secret material marker %q", forbidden)
		}
	}
}

func TestApplySIPProviderPresetUsesSafeRegistrationOnlyDefaults(t *testing.T) {
	cfg, err := ApplySIPProviderPreset("fritzbox", map[string]string{
		"server":       "fritz.box",
		"username":     "aurago-phone",
		"display_name": "AuraGo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PresetID != "fritzbox" || cfg.Registrar != "fritz.box" || cfg.Domain != "fritz.box" {
		t.Fatalf("unexpected FRITZ!Box account values: %+v", cfg)
	}
	if cfg.BindHost != "0.0.0.0" || !cfg.Enabled || !cfg.ReadOnly {
		t.Fatalf("unexpected guided network defaults: %+v", cfg)
	}
	if cfg.Permissions.AnswerInbound || cfg.Permissions.OriginateOutbound || cfg.Permissions.SendDTMF {
		t.Fatalf("preset silently granted call permissions: %+v", cfg.Permissions)
	}
	if len(cfg.Inbound.TrustedPeerCIDRs) != 0 || len(cfg.Inbound.AllowedCallers) != 0 ||
		len(cfg.Outbound.AllowedDomains) != 0 || len(cfg.Outbound.AllowedUsers) != 0 ||
		len(cfg.Outbound.AllowedE164Prefixes) != 0 {
		t.Fatalf("preset silently broadened allowlists: %+v %+v", cfg.Inbound, cfg.Outbound)
	}
	if err := config.ValidateSIPConfig(cfg); err != nil {
		t.Fatalf("registration-only preset is invalid: %v", err)
	}
}

func TestApplySIPProviderPresetSupportsProviderSpecificAccounts(t *testing.T) {
	tests := []struct {
		id        string
		values    map[string]string
		registrar string
		username  string
		auth      string
		expires   int
	}{
		{
			id: "sipgate-de", values: map[string]string{"username": "abc123", "display_name": "Desk"},
			registrar: "sipgate.de", username: "abc123", auth: "abc123", expires: 600,
		},
		{
			id: "telekom-de", values: map[string]string{"phone_number": "+491234567", "auth_username": "name@t-online.de"},
			registrar: "tel.t-online.de", username: "+491234567", auth: "name@t-online.de", expires: 300,
		},
		{
			id: "voip-ms", values: map[string]string{"server": "london1.voip.ms", "username": "100000_sub", "display_name": "Desk"},
			registrar: "london1.voip.ms", username: "100000_sub", auth: "100000_sub", expires: 300,
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			cfg, err := ApplySIPProviderPreset(test.id, test.values)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Registrar != test.registrar || cfg.Username != test.username ||
				cfg.AuthUsername != test.auth || cfg.RegisterExpiresSeconds != test.expires {
				t.Fatalf("unexpected provider result: %+v", cfg)
			}
			if test.id == "telekom-de" && !cfg.PreferSRV {
				t.Fatal("Telekom preset must prefer DNS SRV targets")
			}
		})
	}
}

func TestApplySIPProviderPresetSeparatesOptionalServerPortFromDomain(t *testing.T) {
	tests := []struct {
		name      string
		server    string
		registrar string
		domain    string
	}{
		{name: "hostname", server: "pbx.example:5070", registrar: "pbx.example:5070", domain: "pbx.example"},
		{name: "IPv4", server: "192.0.2.8:5061", registrar: "192.0.2.8:5061", domain: "192.0.2.8"},
		{name: "IPv6 with port", server: "[2001:db8::8]:5070", registrar: "[2001:db8::8]:5070", domain: "2001:db8::8"},
		{name: "IPv6 without port", server: "2001:db8::8", registrar: "[2001:db8::8]", domain: "2001:db8::8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := ApplySIPProviderPreset("fritzbox", map[string]string{
				"server": test.server, "username": "desk",
			})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Registrar != test.registrar || cfg.Domain != test.domain {
				t.Fatalf("server=%q registrar=%q domain=%q", test.server, cfg.Registrar, cfg.Domain)
			}
		})
	}
}

func TestApplySIPProviderPresetRejectsUnknownOrInjectedValues(t *testing.T) {
	for _, test := range []struct {
		id     string
		values map[string]string
	}{
		{id: "missing", values: map[string]string{}},
		{id: "fritzbox", values: map[string]string{"server": "fritz.box", "username": "desk", "password": "must-not-be-here"}},
		{id: "fritzbox", values: map[string]string{"server": "fritz.box\r\nInjected", "username": "desk"}},
		{id: "fritzbox", values: map[string]string{"server": "https://pbx.example", "username": "desk"}},
		{id: "fritzbox", values: map[string]string{"server": "sip:desk@pbx.example", "username": "desk"}},
		{id: "fritzbox", values: map[string]string{"server": "pbx.example/path", "username": "desk"}},
		{id: "fritzbox", values: map[string]string{"server": "pbx.example:0", "username": "desk"}},
		{id: "fritzbox", values: map[string]string{"server": "pbx.example:65536", "username": "desk"}},
	} {
		if _, err := ApplySIPProviderPreset(test.id, test.values); err == nil {
			t.Fatalf("expected %q to be rejected", test.id)
		}
	}
}

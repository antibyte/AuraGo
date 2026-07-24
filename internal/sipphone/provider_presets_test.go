package sipphone

import (
	"context"
	"encoding/json"
	"net"
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

func TestSIPProviderCatalogExposesDomesticChoiceWithoutTechnicalPatterns(t *testing.T) {
	var fritz SIPProviderPreset
	var multiCountry SIPProviderPreset
	for _, preset := range SIPProviderPresets() {
		if preset.ID == "fritzbox" {
			fritz = preset
		}
		if preset.ID == "sipcall" {
			multiCountry = preset
		}
	}
	if fritz.DomesticRegion != "DE" {
		t.Fatalf("FRITZ!Box domestic region = %q", fritz.DomesticRegion)
	}
	if multiCountry.DomesticRegion != "" {
		t.Fatalf("ambiguous multi-country preset exposed domestic policy %q", multiCountry.DomesticRegion)
	}
	encoded, err := json.Marshal(fritz)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "domesticPrefix") || strings.Contains(string(encoded), "nationalPattern") {
		t.Fatalf("technical policy leaked into provider catalog: %s", encoded)
	}
}

func TestApplySetupActivationBuildsStrictOutboundPolicies(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		values   []string
		users    []string
		prefixes []string
	}{
		{name: "all account domain", scope: SetupScopeAll, users: []string{"*"}},
		{name: "German domestic", scope: SetupScopeDomestic, users: []string{"0*"}, prefixes: []string{"+49"}},
		{name: "individual values", scope: SetupScopeCustom, values: []string{"101", "+491701234567"}, users: []string{"101"}, prefixes: []string{"+491701234567"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := ApplySIPProviderPreset("fritzbox", map[string]string{"server": "192.0.2.10", "username": "desk"})
			if err != nil {
				t.Fatal(err)
			}
			err = ApplySetupActivation(context.Background(), &cfg, "fritzbox", SetupActivation{
				OutboundScope: test.scope, OutboundValues: test.values, InboundScope: SetupScopeDisabled,
			})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ReadOnly || !cfg.BrowserMedia.Enabled || !cfg.Permissions.OriginateOutbound || cfg.Permissions.AnswerInbound {
				t.Fatalf("unexpected guided permissions: %+v", cfg)
			}
			if strings.Join(cfg.Outbound.AllowedDomains, ",") != "192.0.2.10" ||
				strings.Join(cfg.Outbound.AllowedUsers, ",") != strings.Join(test.users, ",") ||
				strings.Join(cfg.Outbound.AllowedE164Prefixes, ",") != strings.Join(test.prefixes, ",") {
				t.Fatalf("unexpected outbound policy: %+v", cfg.Outbound)
			}
			if cfg.Inbound.Route != "reject" || len(cfg.Inbound.TrustedPeerCIDRs) != 0 {
				t.Fatalf("incoming calls were not kept disabled: %+v", cfg.Inbound)
			}
		})
	}
}

func TestApplySetupActivationRejectsWildcardsInGuidedCustomValues(t *testing.T) {
	cfg, err := ApplySIPProviderPreset("fritzbox", map[string]string{"server": "192.0.2.10", "username": "desk"})
	if err != nil {
		t.Fatal(err)
	}
	err = ApplySetupActivation(context.Background(), &cfg, "fritzbox", SetupActivation{
		OutboundScope: SetupScopeCustom, OutboundValues: []string{"premium-*"}, InboundScope: SetupScopeDisabled,
	})
	if err == nil {
		t.Fatal("guided custom values accepted a wildcard")
	}
}

func TestApplySetupActivationDetectsExactInboundPeer(t *testing.T) {
	cfg, err := ApplySIPProviderPreset("fritzbox", map[string]string{"server": "192.0.2.10", "username": "desk"})
	if err != nil {
		t.Fatal(err)
	}
	err = ApplySetupActivation(context.Background(), &cfg, "fritzbox", SetupActivation{
		OutboundScope: SetupScopeAll, InboundScope: SetupScopeAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Inbound.Route != "manual" || !cfg.Permissions.AnswerInbound ||
		strings.Join(cfg.Inbound.TrustedPeerCIDRs, ",") != "192.0.2.10" ||
		strings.Join(cfg.Inbound.AllowedCallers, ",") != "*" {
		t.Fatalf("unexpected inbound policy: %+v", cfg.Inbound)
	}
}

func TestSelectLocalSIPHostUsesRegistrarSubnetAndTailscaleRange(t *testing.T) {
	locals := []sipLocalAddress{
		{ip: net.ParseIP("172.18.0.1"), network: mustSIPNetwork(t, "172.18.0.1/16")},
		{ip: net.ParseIP("192.168.6.238"), network: mustSIPNetwork(t, "192.168.6.238/24")},
		{ip: net.ParseIP("100.100.60.102"), network: mustSIPNetwork(t, "100.100.60.102/32")},
	}
	if got := selectLocalSIPHost([]net.IP{net.ParseIP("192.168.6.1")}, locals); got != "192.168.6.238" {
		t.Fatalf("LAN advertised host = %q", got)
	}
	if got := selectLocalSIPHost([]net.IP{net.ParseIP("100.112.131.52")}, locals); got != "100.100.60.102" {
		t.Fatalf("Tailscale advertised host = %q", got)
	}
	if got := selectLocalSIPHost([]net.IP{net.ParseIP("203.0.113.10")}, locals); got != "" {
		t.Fatalf("public peer unexpectedly selected advertised host %q", got)
	}

	cfg := config.SIPConfig{Registrar: "192.168.6.1"}
	applyProviderNetworkDefaults(context.Background(), &cfg, "100.112.131.52:443", locals)
	if cfg.AdvertisedSignalingHost != "192.168.6.238" || cfg.Media.AdvertisedHost != "192.168.6.238" {
		t.Fatalf("SIP advertised hosts were not populated: %+v", cfg)
	}
	if cfg.BrowserMedia.AdvertisedIP != "100.100.60.102" {
		t.Fatalf("browser advertised IP = %q", cfg.BrowserMedia.AdvertisedIP)
	}
}

func mustSIPNetwork(t *testing.T, value string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		t.Fatal(err)
	}
	return network
}

package config

import "testing"

func TestSIPDefaultsAreDisabledAndReadOnly(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	if cfg.Enabled || !cfg.ReadOnly || cfg.Permissions.AnswerInbound || cfg.Permissions.OriginateOutbound || cfg.Permissions.SendDTMF {
		t.Fatalf("unsafe SIP defaults: %+v", cfg)
	}
	if cfg.Media.RTPPortStart != 30000 || cfg.Media.RTPPortEnd != 30099 || cfg.Media.JitterBufferMS != 60 {
		t.Fatalf("unexpected media defaults: %+v", cfg.Media)
	}
	if cfg.BrowserMedia.Enabled || cfg.BrowserMedia.UDPPort != DefaultSIPBrowserMediaUDPPort || cfg.BrowserMedia.BindHost != "" || cfg.BrowserMedia.AdvertisedIP != "" {
		t.Fatalf("unsafe browser media defaults: %+v", cfg.BrowserMedia)
	}
}

func TestNormalizeSIPConfigMovesE164UsersAndFillsDomain(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Domain = "fritz.box"
	cfg.Outbound.AllowedUsers = []string{"desk", "+4972311222949", "desk"}
	cfg.Outbound.AllowedE164Prefixes = []string{"+49"}
	cfg.Outbound.AllowedDomains = nil
	NormalizeSIPConfig(&cfg)
	if len(cfg.Outbound.AllowedUsers) != 1 || cfg.Outbound.AllowedUsers[0] != "desk" {
		t.Fatalf("unexpected users after normalize: %#v", cfg.Outbound.AllowedUsers)
	}
	foundFull := false
	foundPrefix := false
	for _, prefix := range cfg.Outbound.AllowedE164Prefixes {
		if prefix == "+4972311222949" {
			foundFull = true
		}
		if prefix == "+49" {
			foundPrefix = true
		}
	}
	if !foundFull || !foundPrefix {
		t.Fatalf("unexpected e164 prefixes after normalize: %#v", cfg.Outbound.AllowedE164Prefixes)
	}
	if len(cfg.Outbound.AllowedDomains) != 1 || cfg.Outbound.AllowedDomains[0] != "fritz.box" {
		t.Fatalf("expected domain allowlist to include account domain: %#v", cfg.Outbound.AllowedDomains)
	}
}

func TestValidateSIPConfigRequiresAllowlistsAndRuntimeSecret(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Enabled = true
	cfg.ReadOnly = false
	cfg.Registrar = "pbx.example"
	cfg.Domain = "pbx.example"
	cfg.Username = "aurago"
	cfg.Permissions.AnswerInbound = true
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("expected missing SIP allowlists to fail")
	}
	cfg.Inbound.TrustedPeerCIDRs = []string{"192.0.2.0/24"}
	cfg.Inbound.AllowedCallers = []string{"alice"}
	cfg.Outbound.AllowedDomains = []string{"pbx.example"}
	cfg.Outbound.AllowedUsers = []string{"alice"}
	if err := ValidateSIPConfig(cfg); err != nil {
		t.Fatalf("static validation failed: %v", err)
	}
	if err := ValidateSIPRuntimeConfig(cfg); err == nil {
		t.Fatal("expected missing Vault password to fail runtime validation")
	}
	cfg.Password = "secret"
	if err := ValidateSIPRuntimeConfig(cfg); err != nil {
		t.Fatalf("runtime validation failed: %v", err)
	}
}

func TestValidateSIPConfigAllowsRegistrationOnlyWithoutAllowlists(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Enabled = true
	cfg.Registrar = "pbx.example"
	cfg.Domain = "pbx.example"
	cfg.Username = "aurago"
	if err := ValidateSIPConfig(cfg); err != nil {
		t.Fatalf("registration-only SIP config should be valid: %v", err)
	}
	cfg.Permissions.OriginateOutbound = true
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("outbound permission without a destination allowlist must fail")
	}
}

func TestValidateSIPConfigAllowsAuthUsernameWithDomain(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Enabled = true
	cfg.Registrar = "tel.t-online.de"
	cfg.Domain = "tel.t-online.de"
	cfg.Username = "+49123456789"
	cfg.AuthUsername = "name@t-online.de"
	if err := ValidateSIPConfig(cfg); err != nil {
		t.Fatalf("domain-qualified digest username should be valid: %v", err)
	}
}

func TestValidateSIPConfigRejectsDomainInURIUsers(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Enabled = true
	cfg.Registrar = "pbx.example"
	cfg.Domain = "pbx.example"
	cfg.Username = "alice@pbx.example"
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("SIP URI username must not contain a domain")
	}

	cfg.Username = "alice"
	cfg.Outbound.AllowedUsers = []string{"bob@pbx.example"}
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("outbound allowed user must not contain a domain")
	}
}

func TestValidateSIPConfigRejectsBadMediaRange(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Media.RTPPortStart = 30001
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("expected odd RTP start port rejection")
	}
}

func TestValidateSIPConfigRejectsControlCharacterInjection(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.DisplayName = "AuraGo\r\nX-Injected: true"
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("expected SIP display-name injection to be rejected")
	}
}

func TestValidateSIPConfigRejectsBrowserMediaPortOverlap(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.BrowserMedia.UDPPort = cfg.BindPort
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("expected signaling port overlap to be rejected")
	}
	cfg.BrowserMedia.UDPPort = cfg.Media.RTPPortStart + 2
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("expected RTP port overlap to be rejected")
	}
	cfg.BrowserMedia.UDPPort = DefaultSIPBrowserMediaUDPPort
	cfg.BrowserMedia.AdvertisedIP = "not-an-ip"
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("expected invalid advertised browser IP to be rejected")
	}
}

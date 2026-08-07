package config

import (
	"strings"
	"testing"
)

func TestSIPDefaultsAreDisabledAndReadOnly(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	if cfg.Enabled || !cfg.ReadOnly || cfg.Permissions.AnswerInbound || cfg.Permissions.OriginateOutbound || cfg.Permissions.SendDTMF {
		t.Fatalf("unsafe SIP defaults: %+v", cfg)
	}
	if cfg.Media.RTPPortStart != 30000 || cfg.Media.RTPPortEnd != 30099 || cfg.Media.JitterBufferMS != 60 {
		t.Fatalf("unexpected media defaults: %+v", cfg.Media)
	}
	if cfg.Media.RTPIdleTimeoutSeconds != 60 || cfg.Inbound.RingTimeoutSeconds != 90 || cfg.Voice.TurnTimeoutSeconds != 60 || cfg.Voice.MaxResponseChars != 1200 {
		t.Fatalf("unexpected SIP safety defaults: %+v", cfg)
	}
	if cfg.BrowserMedia.Enabled || cfg.BrowserMedia.UDPPort != DefaultSIPBrowserMediaUDPPort || cfg.BrowserMedia.BindHost != "" || cfg.BrowserMedia.AdvertisedIP != "" {
		t.Fatalf("unsafe browser media defaults: %+v", cfg.BrowserMedia)
	}
	if cfg.Voice.Backend != "classic" || !cfg.Voice.Behavior.GreetingEnabled || cfg.Voice.IdleTimeoutSeconds != 120 || cfg.Voice.MaxOutboundCallsPerDay != DefaultSIPMaxOutboundCallsPerDay {
		t.Fatalf("unexpected telephone agent defaults: %+v", cfg.Voice)
	}
	if len(cfg.Voice.AllowedTools) != 0 || cfg.Voice.PersistTranscripts {
		t.Fatalf("unsafe telephone agent privacy/tool defaults: %+v", cfg.Voice)
	}
	if cfg.Voice.AgentProviderID != "" || cfg.Voice.Classic.ASRProviderID != "" || cfg.Voice.Classic.TTSProvider != "" {
		t.Fatalf("legacy provider inheritance markers were materialized too early: %+v", cfg.Voice)
	}
}

func TestSIPRejectsDomainAllowEntriesWithPorts(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Outbound.AllowedDomains = []string{"pbx.example:5060"}
	if err := ValidateSIPConfig(cfg); err == nil || !strings.Contains(err.Error(), "allowed_domains") {
		t.Fatalf("domain with port was not rejected: %v", err)
	}
}

func TestValidateSIPTelephoneAgentConfiguration(t *testing.T) {
	valid := func() SIPConfig {
		var cfg SIPConfig
		ApplySIPDefaults(&cfg)
		return cfg
	}
	tests := []struct {
		name   string
		mutate func(*SIPConfig)
		want   string
	}{
		{
			name: "legacy empty provider references",
		},
		{
			name: "unsupported ASR mode",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.Classic.ASRMode = "magic"
			},
			want: "asr_mode",
		},
		{
			name: "unsupported TTS provider",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.Classic.TTSProvider = "unknown"
			},
			want: "tts_provider",
		},
		{
			name: "Gemini profile required",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.Backend = "gemini_live"
			},
			want: "realtime_profile_id",
		},
		{
			name: "idle timeout bounded",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.IdleTimeoutSeconds = 14
			},
			want: "idle_timeout_seconds",
		},
		{
			name: "behavior enum bounded",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.Behavior.UnavailableRequestBehavior = "invent"
			},
			want: "unavailable_request_behavior",
		},
		{
			name: "greeting length bounded",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.Behavior.Greeting = strings.Repeat("a", 501)
			},
			want: "greeting exceeds 500",
		},
		{
			name: "provider reference length bounded",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.AgentProviderID = strings.Repeat("p", 129)
			},
			want: "agent_provider_id exceeds 128",
		},
		{
			name: "language length bounded",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.Language = strings.Repeat("l", 33)
			},
			want: "language exceeds 32",
		},
		{
			name: "tool count bounded",
			mutate: func(cfg *SIPConfig) {
				cfg.Voice.AllowedTools = make([]string, 129)
				for i := range cfg.Voice.AllowedTools {
					cfg.Voice.AllowedTools[i] = strings.Repeat("a", i+1)
				}
			},
			want: "at most 128",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid()
			if test.mutate != nil {
				test.mutate(&cfg)
			}
			err := ValidateSIPConfig(cfg)
			if test.want == "" {
				if err != nil {
					t.Fatalf("unexpected validation error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
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

func TestNormalizeSIPConfigMovesDeniedE164Users(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Outbound.DeniedUsers = []string{"sales-*", "+49900"}
	NormalizeSIPConfig(&cfg)
	if len(cfg.Outbound.DeniedUsers) != 1 || cfg.Outbound.DeniedUsers[0] != "sales-*" {
		t.Fatalf("unexpected denied users: %#v", cfg.Outbound.DeniedUsers)
	}
	if len(cfg.Outbound.DeniedE164Prefixes) != 1 || cfg.Outbound.DeniedE164Prefixes[0] != "+49900" {
		t.Fatalf("unexpected denied E.164 prefixes: %#v", cfg.Outbound.DeniedE164Prefixes)
	}
}

func TestNormalizeSIPConfigMigratesUniversalOutboundWildcard(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Domain = "pbx.example"
	cfg.Outbound.AllowedDomains = []string{"*"}
	cfg.Outbound.AllowedUsers = []string{"*", "desk"}
	NormalizeSIPConfig(&cfg)
	if len(cfg.Outbound.AllowedUsers) != 0 {
		t.Fatalf("universal user wildcard was not removed: %#v", cfg.Outbound.AllowedUsers)
	}
	if len(cfg.Outbound.AllowedDomains) != 1 || cfg.Outbound.AllowedDomains[0] != cfg.Domain {
		t.Fatalf("universal domain wildcard did not fall back to account domain: %#v", cfg.Outbound.AllowedDomains)
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
	if err := ValidateSIPConfig(cfg); err == nil || !strings.Contains(err.Error(), "readonly=false") {
		t.Fatalf("outbound permission in read-only mode error = %v", err)
	}
	cfg.ReadOnly = false
	if err := ValidateSIPConfig(cfg); err != nil {
		t.Fatalf("empty outbound destination allowlist should permit the account domain: %v", err)
	}
}

func TestValidateSIPConfigBoundsDailyAgentCallLimit(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	for _, invalid := range []int{-1, 1001} {
		cfg.Voice.MaxOutboundCallsPerDay = invalid
		if err := ValidateSIPConfig(cfg); err == nil || !strings.Contains(err.Error(), "max_outbound_calls_per_day") {
			t.Fatalf("limit %d error = %v", invalid, err)
		}
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

func TestValidateSIPConfigAllowsWildcardPolicies(t *testing.T) {
	var cfg SIPConfig
	ApplySIPDefaults(&cfg)
	cfg.Inbound.AllowedCallers = []string{"sip:+49*@*.example.com", "desk-??"}
	cfg.Inbound.DeniedCallers = []string{"sip:+49900*@*.example.com"}
	cfg.Outbound.AllowedDomains = []string{"*.example.com"}
	cfg.Outbound.DeniedDomains = []string{"premium.example.com"}
	cfg.Outbound.AllowedUsers = []string{"sales-*"}
	cfg.Outbound.DeniedUsers = []string{"sales-999"}
	if err := ValidateSIPConfig(cfg); err != nil {
		t.Fatalf("wildcard policies should be valid: %v", err)
	}
	cfg.Outbound.DeniedDomains = []string{"bad domain*"}
	if err := ValidateSIPConfig(cfg); err == nil {
		t.Fatal("invalid wildcard domain must be rejected")
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

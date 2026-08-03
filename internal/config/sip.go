package config

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
)

var (
	sipConfigUserPattern   = regexp.MustCompile(`^[A-Za-z0-9_.!~*?'()%+\-]+$`)
	sipAuthUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.!~*'()%+@\-]+$`)
	sipPresetIDPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	sipE164PrefixPattern   = regexp.MustCompile(`^\+[1-9][0-9]{0,14}$`)
)

const (
	// SIPPasswordVaultKey is the only supported storage location for the SIP digest password.
	SIPPasswordVaultKey = "sip_endpoint_password"

	DefaultSIPBindHost             = "127.0.0.1"
	DefaultSIPBindPort             = 5060
	DefaultSIPRegisterExpires      = 300
	DefaultSIPRTPPortStart         = 30000
	DefaultSIPRTPPortEnd           = 30099
	DefaultSIPBrowserMediaUDPPort  = 30100
	DefaultSIPJitterBufferMS       = 60
	DefaultSIPRTPIdleTimeout       = 60
	DefaultSIPAutoAnswerDelayMS    = 1000
	DefaultSIPRingTimeout          = 90
	DefaultSIPMaxCallDuration      = 3600
	DefaultSIPIdleTimeout          = 120
	DefaultSIPTurnTimeout          = 60
	DefaultSIPMaxResponseChars     = 1200
	DefaultSIPHistoryRetentionDays = 90
)

// SIPConfig configures AuraGo's single-account, single-call native SIP endpoint.
// Password is runtime-only and hydrated exclusively from the encrypted vault.
type SIPConfig struct {
	PresetID                string                `yaml:"preset_id,omitempty" json:"preset_id,omitempty"`
	Enabled                 bool                  `yaml:"enabled" json:"enabled"`
	ReadOnly                bool                  `yaml:"readonly" json:"readonly"`
	BindHost                string                `yaml:"bind_host" json:"bind_host"`
	BindPort                int                   `yaml:"bind_port" json:"bind_port"`
	Transport               string                `yaml:"transport" json:"transport"`
	PreferSRV               bool                  `yaml:"prefer_srv,omitempty" json:"prefer_srv"`
	Registrar               string                `yaml:"registrar" json:"registrar"`
	OutboundProxy           string                `yaml:"outbound_proxy" json:"outbound_proxy"`
	Domain                  string                `yaml:"domain" json:"domain"`
	Username                string                `yaml:"username" json:"username"`
	AuthUsername            string                `yaml:"auth_username" json:"auth_username"`
	DisplayName             string                `yaml:"display_name" json:"display_name"`
	RegisterExpiresSeconds  int                   `yaml:"register_expires_seconds" json:"register_expires_seconds"`
	AdvertisedSignalingHost string                `yaml:"advertised_signaling_host" json:"advertised_signaling_host"`
	Password                string                `yaml:"-" json:"-"`
	TLS                     SIPTLSConfig          `yaml:"tls" json:"tls"`
	Media                   SIPMediaConfig        `yaml:"media" json:"media"`
	BrowserMedia            SIPBrowserMediaConfig `yaml:"browser_media" json:"browser_media"`
	Inbound                 SIPInboundConfig      `yaml:"inbound" json:"inbound"`
	Outbound                SIPOutboundConfig     `yaml:"outbound" json:"outbound"`
	Permissions             SIPPermissionsConfig  `yaml:"permissions" json:"permissions"`
	Voice                   SIPVoiceConfig        `yaml:"voice" json:"voice"`
	HistoryRetentionDays    int                   `yaml:"history_retention_days" json:"history_retention_days"`
}

type SIPTLSConfig struct {
	CertFile   string `yaml:"cert_file" json:"cert_file"`
	KeyFile    string `yaml:"key_file" json:"key_file"`
	ServerName string `yaml:"server_name" json:"server_name"`
}

type SIPMediaConfig struct {
	RTPPortStart          int      `yaml:"rtp_port_start" json:"rtp_port_start"`
	RTPPortEnd            int      `yaml:"rtp_port_end" json:"rtp_port_end"`
	AdvertisedHost        string   `yaml:"advertised_host" json:"advertised_host"`
	SymmetricRTP          bool     `yaml:"symmetric_rtp" json:"symmetric_rtp"`
	JitterBufferMS        int      `yaml:"jitter_buffer_ms" json:"jitter_buffer_ms"`
	RTPIdleTimeoutSeconds int      `yaml:"rtp_idle_timeout_seconds" json:"rtp_idle_timeout_seconds"`
	Codecs                []string `yaml:"codecs" json:"codecs"`
}

// SIPBrowserMediaConfig controls the authenticated WebRTC media boundary used
// by the Virtual Desktop phone. It is opt-in and never exposes SIP credentials.
type SIPBrowserMediaConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	BindHost     string `yaml:"bind_host" json:"bind_host"`
	UDPPort      int    `yaml:"udp_port" json:"udp_port"`
	AdvertisedIP string `yaml:"advertised_ip" json:"advertised_ip"`
}

type SIPInboundConfig struct {
	Route              string   `yaml:"route" json:"route"`
	AutoAnswerDelayMS  int      `yaml:"auto_answer_delay_ms" json:"auto_answer_delay_ms"`
	RingTimeoutSeconds int      `yaml:"ring_timeout_seconds" json:"ring_timeout_seconds"`
	NumberRegion       string   `yaml:"number_region,omitempty" json:"number_region"`
	TrustedPeerCIDRs   []string `yaml:"trusted_peer_cidrs" json:"trusted_peer_cidrs"`
	AllowedCallers     []string `yaml:"allowed_callers" json:"allowed_callers"`
	DeniedCallers      []string `yaml:"denied_callers,omitempty" json:"denied_callers"`
}

type SIPOutboundConfig struct {
	AllowedDomains      []string `yaml:"allowed_domains" json:"allowed_domains"`
	DeniedDomains       []string `yaml:"denied_domains,omitempty" json:"denied_domains"`
	AllowedUsers        []string `yaml:"allowed_users" json:"allowed_users"`
	DeniedUsers         []string `yaml:"denied_users,omitempty" json:"denied_users"`
	AllowedE164Prefixes []string `yaml:"allowed_e164_prefixes" json:"allowed_e164_prefixes"`
	DeniedE164Prefixes  []string `yaml:"denied_e164_prefixes,omitempty" json:"denied_e164_prefixes"`
}

type SIPPermissionsConfig struct {
	AnswerInbound     bool `yaml:"answer_inbound" json:"answer_inbound"`
	OriginateOutbound bool `yaml:"originate_outbound" json:"originate_outbound"`
	SendDTMF          bool `yaml:"send_dtmf" json:"send_dtmf"`
	AgentHangup       bool `yaml:"agent_hangup" json:"agent_hangup"`
}

type SIPVoiceConfig struct {
	Backend                string                 `yaml:"backend" json:"backend"`
	RealtimeProfileID      string                 `yaml:"realtime_profile_id" json:"realtime_profile_id"`
	AgentProviderID        string                 `yaml:"agent_provider_id,omitempty" json:"agent_provider_id"`
	Classic                SIPVoiceClassicConfig  `yaml:"classic,omitempty" json:"classic"`
	Behavior               SIPVoiceBehaviorConfig `yaml:"behavior,omitempty" json:"behavior"`
	Language               string                 `yaml:"language" json:"language"`
	AllowedTools           []string               `yaml:"allowed_tools" json:"allowed_tools"`
	PersistTranscripts     bool                   `yaml:"persist_transcripts" json:"persist_transcripts"`
	MaxCallDurationSeconds int                    `yaml:"max_call_duration_seconds" json:"max_call_duration_seconds"`
	IdleTimeoutSeconds     int                    `yaml:"idle_timeout_seconds" json:"idle_timeout_seconds"`
	TurnTimeoutSeconds     int                    `yaml:"turn_timeout_seconds" json:"turn_timeout_seconds"`
	MaxResponseChars       int                    `yaml:"max_response_chars" json:"max_response_chars"`
}

// SIPVoiceClassicConfig selects the three independently managed stages of the
// classic telephone pipeline. Empty fields are legacy inheritance markers and
// are materialized by the Telefonagent API on first save.
type SIPVoiceClassicConfig struct {
	ASRProviderID string `yaml:"asr_provider_id,omitempty" json:"asr_provider_id"`
	ASRMode       string `yaml:"asr_mode,omitempty" json:"asr_mode"`
	TTSProvider   string `yaml:"tts_provider,omitempty" json:"tts_provider"`
}

// SIPVoiceBehaviorConfig contains only additive telephone guidance. AuraGo's
// identity, security rules, and prompt-injection defenses remain authoritative.
type SIPVoiceBehaviorConfig struct {
	GreetingEnabled            bool   `yaml:"greeting_enabled" json:"greeting_enabled"`
	Greeting                   string `yaml:"greeting,omitempty" json:"greeting"`
	Purpose                    string `yaml:"purpose,omitempty" json:"purpose"`
	SpeakingStyle              string `yaml:"speaking_style,omitempty" json:"speaking_style"`
	AdditionalProhibitions     string `yaml:"additional_prohibitions,omitempty" json:"additional_prohibitions"`
	UnavailableRequestBehavior string `yaml:"unavailable_request_behavior" json:"unavailable_request_behavior"`
	FailureMessage             string `yaml:"failure_message,omitempty" json:"failure_message"`
	GoodbyeMessage             string `yaml:"goodbye_message,omitempty" json:"goodbye_message"`
}

// ApplySIPDefaults sets safe defaults before YAML unmarshalling so absent booleans remain secure.
func ApplySIPDefaults(cfg *SIPConfig) {
	if cfg == nil {
		return
	}
	cfg.ReadOnly = true
	cfg.BindHost = DefaultSIPBindHost
	cfg.BindPort = DefaultSIPBindPort
	cfg.Transport = "udp"
	cfg.DisplayName = "AuraGo"
	cfg.RegisterExpiresSeconds = DefaultSIPRegisterExpires
	cfg.Media.RTPPortStart = DefaultSIPRTPPortStart
	cfg.Media.RTPPortEnd = DefaultSIPRTPPortEnd
	cfg.Media.SymmetricRTP = true
	cfg.Media.JitterBufferMS = DefaultSIPJitterBufferMS
	cfg.Media.RTPIdleTimeoutSeconds = DefaultSIPRTPIdleTimeout
	cfg.Media.Codecs = []string{"pcma", "pcmu"}
	cfg.BrowserMedia.UDPPort = DefaultSIPBrowserMediaUDPPort
	cfg.Inbound.Route = "agent"
	cfg.Inbound.AutoAnswerDelayMS = DefaultSIPAutoAnswerDelayMS
	cfg.Inbound.RingTimeoutSeconds = DefaultSIPRingTimeout
	cfg.Permissions.AgentHangup = true
	cfg.Voice.Backend = "classic"
	cfg.Voice.Language = "auto"
	cfg.Voice.MaxCallDurationSeconds = DefaultSIPMaxCallDuration
	cfg.Voice.IdleTimeoutSeconds = DefaultSIPIdleTimeout
	cfg.Voice.TurnTimeoutSeconds = DefaultSIPTurnTimeout
	cfg.Voice.MaxResponseChars = DefaultSIPMaxResponseChars
	cfg.Voice.Behavior.GreetingEnabled = true
	cfg.Voice.Behavior.UnavailableRequestBehavior = "explain"
	cfg.HistoryRetentionDays = DefaultSIPHistoryRetentionDays
}

// NormalizeSIPConfig canonicalizes non-secret SIP settings without enabling the endpoint.
func NormalizeSIPConfig(cfg *SIPConfig) {
	if cfg == nil {
		return
	}
	// Preserve compatibility with API clients and tests that construct partial
	// SIP values instead of unmarshalling on top of ApplySIPDefaults.
	if cfg.Media.RTPIdleTimeoutSeconds == 0 {
		cfg.Media.RTPIdleTimeoutSeconds = DefaultSIPRTPIdleTimeout
	}
	if cfg.Inbound.RingTimeoutSeconds == 0 {
		cfg.Inbound.RingTimeoutSeconds = DefaultSIPRingTimeout
	}
	if cfg.Voice.TurnTimeoutSeconds == 0 {
		cfg.Voice.TurnTimeoutSeconds = DefaultSIPTurnTimeout
	}
	if cfg.Voice.MaxResponseChars == 0 {
		cfg.Voice.MaxResponseChars = DefaultSIPMaxResponseChars
	}
	cfg.BindHost = strings.TrimSpace(cfg.BindHost)
	cfg.PresetID = strings.ToLower(strings.TrimSpace(cfg.PresetID))
	cfg.Transport = strings.ToLower(strings.TrimSpace(cfg.Transport))
	cfg.Registrar = strings.TrimSpace(cfg.Registrar)
	cfg.OutboundProxy = strings.TrimSpace(cfg.OutboundProxy)
	cfg.Domain = strings.ToLower(strings.TrimSpace(cfg.Domain))
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.AuthUsername = strings.TrimSpace(cfg.AuthUsername)
	cfg.DisplayName = strings.TrimSpace(cfg.DisplayName)
	cfg.AdvertisedSignalingHost = strings.TrimSpace(cfg.AdvertisedSignalingHost)
	cfg.TLS.CertFile = strings.TrimSpace(cfg.TLS.CertFile)
	cfg.TLS.KeyFile = strings.TrimSpace(cfg.TLS.KeyFile)
	cfg.TLS.ServerName = strings.TrimSpace(cfg.TLS.ServerName)
	cfg.Media.AdvertisedHost = strings.TrimSpace(cfg.Media.AdvertisedHost)
	cfg.Media.Codecs = normalizedUniqueLower(cfg.Media.Codecs)
	cfg.BrowserMedia.BindHost = strings.TrimSpace(cfg.BrowserMedia.BindHost)
	cfg.BrowserMedia.AdvertisedIP = strings.TrimSpace(cfg.BrowserMedia.AdvertisedIP)
	cfg.Inbound.Route = strings.ToLower(strings.TrimSpace(cfg.Inbound.Route))
	cfg.Inbound.NumberRegion = strings.ToUpper(strings.TrimSpace(cfg.Inbound.NumberRegion))
	cfg.Inbound.TrustedPeerCIDRs = normalizedUnique(cfg.Inbound.TrustedPeerCIDRs, false)
	cfg.Inbound.AllowedCallers = normalizedUnique(cfg.Inbound.AllowedCallers, true)
	cfg.Inbound.DeniedCallers = normalizedUnique(cfg.Inbound.DeniedCallers, true)
	cfg.Outbound.AllowedDomains = normalizedUnique(cfg.Outbound.AllowedDomains, true)
	cfg.Outbound.DeniedDomains = normalizedUnique(cfg.Outbound.DeniedDomains, true)
	cfg.Outbound.AllowedUsers = normalizedUnique(cfg.Outbound.AllowedUsers, false)
	cfg.Outbound.DeniedUsers = normalizedUnique(cfg.Outbound.DeniedUsers, false)
	cfg.Outbound.AllowedE164Prefixes = normalizedUnique(cfg.Outbound.AllowedE164Prefixes, false)
	cfg.Outbound.DeniedE164Prefixes = normalizedUnique(cfg.Outbound.DeniedE164Prefixes, false)
	// Exact E.164 values are destinations/prefixes, not SIP URI users. Keep them
	// in the prefix list so dial policy and validation stay consistent.
	if len(cfg.Outbound.AllowedUsers) > 0 {
		keptUsers := make([]string, 0, len(cfg.Outbound.AllowedUsers))
		for _, user := range cfg.Outbound.AllowedUsers {
			if sipE164PrefixPattern.MatchString(user) {
				cfg.Outbound.AllowedE164Prefixes = append(cfg.Outbound.AllowedE164Prefixes, user)
				continue
			}
			keptUsers = append(keptUsers, user)
		}
		cfg.Outbound.AllowedUsers = normalizedUnique(keptUsers, false)
		cfg.Outbound.AllowedE164Prefixes = normalizedUnique(cfg.Outbound.AllowedE164Prefixes, false)
	}
	if len(cfg.Outbound.DeniedUsers) > 0 {
		keptUsers := make([]string, 0, len(cfg.Outbound.DeniedUsers))
		for _, user := range cfg.Outbound.DeniedUsers {
			if sipE164PrefixPattern.MatchString(user) {
				cfg.Outbound.DeniedE164Prefixes = append(cfg.Outbound.DeniedE164Prefixes, user)
				continue
			}
			keptUsers = append(keptUsers, user)
		}
		cfg.Outbound.DeniedUsers = normalizedUnique(keptUsers, false)
		cfg.Outbound.DeniedE164Prefixes = normalizedUnique(cfg.Outbound.DeniedE164Prefixes, false)
	}
	// Dial requires an allowlisted domain. When destinations exist and the
	// account domain is known, include it so saved numbers are actually usable.
	if cfg.Domain != "" && (len(cfg.Outbound.AllowedUsers) > 0 || len(cfg.Outbound.AllowedE164Prefixes) > 0) {
		hasDomain := false
		for _, domain := range cfg.Outbound.AllowedDomains {
			if domain == cfg.Domain {
				hasDomain = true
				break
			}
		}
		if !hasDomain {
			cfg.Outbound.AllowedDomains = append(cfg.Outbound.AllowedDomains, cfg.Domain)
		}
	}
	cfg.Voice.Backend = strings.ToLower(strings.TrimSpace(cfg.Voice.Backend))
	cfg.Voice.RealtimeProfileID = strings.TrimSpace(cfg.Voice.RealtimeProfileID)
	cfg.Voice.AgentProviderID = strings.TrimSpace(cfg.Voice.AgentProviderID)
	cfg.Voice.Classic.ASRProviderID = strings.TrimSpace(cfg.Voice.Classic.ASRProviderID)
	cfg.Voice.Classic.ASRMode = strings.ToLower(strings.TrimSpace(cfg.Voice.Classic.ASRMode))
	cfg.Voice.Classic.TTSProvider = strings.ToLower(strings.TrimSpace(cfg.Voice.Classic.TTSProvider))
	cfg.Voice.Behavior.Greeting = strings.TrimSpace(cfg.Voice.Behavior.Greeting)
	cfg.Voice.Behavior.Purpose = strings.TrimSpace(cfg.Voice.Behavior.Purpose)
	cfg.Voice.Behavior.SpeakingStyle = strings.TrimSpace(cfg.Voice.Behavior.SpeakingStyle)
	cfg.Voice.Behavior.AdditionalProhibitions = strings.TrimSpace(cfg.Voice.Behavior.AdditionalProhibitions)
	cfg.Voice.Behavior.UnavailableRequestBehavior = strings.ToLower(strings.TrimSpace(cfg.Voice.Behavior.UnavailableRequestBehavior))
	cfg.Voice.Behavior.FailureMessage = strings.TrimSpace(cfg.Voice.Behavior.FailureMessage)
	cfg.Voice.Behavior.GoodbyeMessage = strings.TrimSpace(cfg.Voice.Behavior.GoodbyeMessage)
	cfg.Voice.Language = strings.TrimSpace(cfg.Voice.Language)
	cfg.Voice.AllowedTools = normalizedUnique(cfg.Voice.AllowedTools, true)
}

// ValidateSIPConfig checks invariants that can be decided without network access.
func ValidateSIPConfig(cfg SIPConfig) error {
	// Programmatic tests and partial config writers may construct Config without
	// running Load. An entirely absent, disabled SIP block is valid and receives
	// defaults the next time the configuration is loaded.
	if !cfg.Enabled && cfg.BindHost == "" && cfg.BindPort == 0 && cfg.Transport == "" {
		return nil
	}
	for name, value := range map[string]string{
		"preset_id": cfg.PresetID, "registrar": cfg.Registrar, "outbound_proxy": cfg.OutboundProxy, "domain": cfg.Domain,
		"username": cfg.Username, "auth_username": cfg.AuthUsername, "display_name": cfg.DisplayName,
		"advertised_signaling_host": cfg.AdvertisedSignalingHost, "media.advertised_host": cfg.Media.AdvertisedHost,
		"browser_media.bind_host": cfg.BrowserMedia.BindHost, "browser_media.advertised_ip": cfg.BrowserMedia.AdvertisedIP,
		"tls.server_name": cfg.TLS.ServerName, "tls.cert_file": cfg.TLS.CertFile, "tls.key_file": cfg.TLS.KeyFile,
		"voice.backend": cfg.Voice.Backend, "voice.realtime_profile_id": cfg.Voice.RealtimeProfileID,
		"voice.language":          cfg.Voice.Language,
		"inbound.number_region":   cfg.Inbound.NumberRegion,
		"voice.agent_provider_id": cfg.Voice.AgentProviderID, "voice.classic.asr_provider_id": cfg.Voice.Classic.ASRProviderID,
		"voice.classic.asr_mode": cfg.Voice.Classic.ASRMode, "voice.classic.tts_provider": cfg.Voice.Classic.TTSProvider,
	} {
		if strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("sip.%s contains forbidden control characters", name)
		}
	}
	for name, values := range map[string][]string{
		"inbound.trusted_peer_cidrs":     cfg.Inbound.TrustedPeerCIDRs,
		"inbound.allowed_callers":        cfg.Inbound.AllowedCallers,
		"inbound.denied_callers":         cfg.Inbound.DeniedCallers,
		"outbound.allowed_domains":       cfg.Outbound.AllowedDomains,
		"outbound.denied_domains":        cfg.Outbound.DeniedDomains,
		"outbound.allowed_users":         cfg.Outbound.AllowedUsers,
		"outbound.denied_users":          cfg.Outbound.DeniedUsers,
		"outbound.allowed_e164_prefixes": cfg.Outbound.AllowedE164Prefixes,
		"outbound.denied_e164_prefixes":  cfg.Outbound.DeniedE164Prefixes,
		"voice.allowed_tools":            cfg.Voice.AllowedTools,
	} {
		for _, value := range values {
			if strings.ContainsAny(value, "\r\n\x00") {
				return fmt.Errorf("sip.%s contains forbidden control characters", name)
			}
		}
	}
	if cfg.BindHost == "" || net.ParseIP(cfg.BindHost) == nil {
		return fmt.Errorf("sip.bind_host must be a concrete IP address")
	}
	if cfg.PresetID != "" && !sipPresetIDPattern.MatchString(cfg.PresetID) {
		return fmt.Errorf("sip.preset_id is invalid")
	}
	if cfg.BindPort < 1 || cfg.BindPort > 65535 {
		return fmt.Errorf("sip.bind_port must be between 1 and 65535")
	}
	switch cfg.Transport {
	case "udp", "tcp", "tls":
	default:
		return fmt.Errorf("sip.transport must be udp, tcp, or tls")
	}
	if cfg.Enabled {
		if cfg.Registrar == "" || cfg.Username == "" || cfg.Domain == "" {
			return fmt.Errorf("enabled SIP requires registrar, domain, and username")
		}
		if !sipConfigUserPattern.MatchString(cfg.Username) {
			return fmt.Errorf("sip.username must be a valid SIP URI user")
		}
		if cfg.AuthUsername != "" && !sipAuthUsernamePattern.MatchString(cfg.AuthUsername) {
			return fmt.Errorf("sip.auth_username must be a valid digest authentication user")
		}
		if !validSIPDomain(cfg.Domain) {
			return fmt.Errorf("sip.domain must be a host name or IP address without a port")
		}
		if cfg.Permissions.AnswerInbound && (len(cfg.Inbound.TrustedPeerCIDRs) == 0 || len(cfg.Inbound.AllowedCallers) == 0) {
			return fmt.Errorf("enabled SIP requires trusted peer and caller allowlists")
		}
		if cfg.Permissions.OriginateOutbound && (len(cfg.Outbound.AllowedDomains) == 0 || (len(cfg.Outbound.AllowedUsers) == 0 && len(cfg.Outbound.AllowedE164Prefixes) == 0)) {
			return fmt.Errorf("enabled SIP requires destination domain and user or E.164 allowlists")
		}
		if cfg.Transport == "tls" && (cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "") {
			return fmt.Errorf("sip.tls.cert_file and sip.tls.key_file are required for TLS listening")
		}
	}
	if cfg.RegisterExpiresSeconds < 60 || cfg.RegisterExpiresSeconds > 3600 {
		return fmt.Errorf("sip.register_expires_seconds must be between 60 and 3600")
	}
	if cfg.Media.RTPPortStart < 1024 || cfg.Media.RTPPortStart > 65534 || cfg.Media.RTPPortStart%2 != 0 {
		return fmt.Errorf("sip.media.rtp_port_start must be an even port between 1024 and 65534")
	}
	if cfg.Media.RTPPortEnd <= cfg.Media.RTPPortStart || cfg.Media.RTPPortEnd > 65535 {
		return fmt.Errorf("sip.media.rtp_port_end must be greater than rtp_port_start and at most 65535")
	}
	if cfg.BrowserMedia.UDPPort < 1024 || cfg.BrowserMedia.UDPPort > 65535 {
		return fmt.Errorf("sip.browser_media.udp_port must be between 1024 and 65535")
	}
	if cfg.BrowserMedia.BindHost != "" && net.ParseIP(cfg.BrowserMedia.BindHost) == nil {
		return fmt.Errorf("sip.browser_media.bind_host must be empty or a concrete IP address")
	}
	if cfg.BrowserMedia.AdvertisedIP != "" && net.ParseIP(cfg.BrowserMedia.AdvertisedIP) == nil {
		return fmt.Errorf("sip.browser_media.advertised_ip must be empty or a concrete IP address")
	}
	if cfg.BrowserMedia.UDPPort == cfg.BindPort {
		return fmt.Errorf("sip.browser_media.udp_port must not overlap sip.bind_port")
	}
	if cfg.BrowserMedia.UDPPort >= cfg.Media.RTPPortStart && cfg.BrowserMedia.UDPPort <= cfg.Media.RTPPortEnd {
		return fmt.Errorf("sip.browser_media.udp_port must not overlap the SIP RTP port range")
	}
	if cfg.Media.JitterBufferMS < 20 || cfg.Media.JitterBufferMS > 200 || cfg.Media.JitterBufferMS%20 != 0 {
		return fmt.Errorf("sip.media.jitter_buffer_ms must be a multiple of 20 between 20 and 200")
	}
	if cfg.Media.RTPIdleTimeoutSeconds < 15 || cfg.Media.RTPIdleTimeoutSeconds > 600 {
		return fmt.Errorf("sip.media.rtp_idle_timeout_seconds must be between 15 and 600")
	}
	if len(cfg.Media.Codecs) == 0 {
		return fmt.Errorf("sip.media.codecs must include pcma or pcmu")
	}
	for _, codec := range cfg.Media.Codecs {
		if codec != "pcma" && codec != "pcmu" {
			return fmt.Errorf("sip.media.codecs only supports pcma and pcmu")
		}
	}
	switch cfg.Inbound.Route {
	case "agent", "manual", "reject":
	default:
		return fmt.Errorf("sip.inbound.route must be agent, manual, or reject")
	}
	if cfg.Inbound.RingTimeoutSeconds < 15 || cfg.Inbound.RingTimeoutSeconds > 300 {
		return fmt.Errorf("sip.inbound.ring_timeout_seconds must be between 15 and 300")
	}
	if cfg.Inbound.NumberRegion != "" && !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(cfg.Inbound.NumberRegion) {
		return fmt.Errorf("sip.inbound.number_region must be an ISO 3166-1 alpha-2 country code")
	}
	for _, raw := range cfg.Inbound.TrustedPeerCIDRs {
		if net.ParseIP(raw) == nil {
			if _, _, err := net.ParseCIDR(raw); err != nil {
				return fmt.Errorf("invalid sip.inbound.trusted_peer_cidrs entry %q", raw)
			}
		}
	}
	for _, domain := range cfg.Outbound.AllowedDomains {
		if !validSIPDomainPattern(domain) {
			return fmt.Errorf("invalid sip.outbound.allowed_domains entry %q", domain)
		}
	}
	for _, domain := range cfg.Outbound.DeniedDomains {
		if !validSIPDomainPattern(domain) {
			return fmt.Errorf("invalid sip.outbound.denied_domains entry %q", domain)
		}
	}
	for _, caller := range append(append([]string(nil), cfg.Inbound.AllowedCallers...), cfg.Inbound.DeniedCallers...) {
		if !validSIPCallerPattern(caller) {
			return fmt.Errorf("invalid SIP caller pattern %q", caller)
		}
	}
	for _, user := range cfg.Outbound.AllowedUsers {
		if !sipConfigUserPattern.MatchString(user) {
			return fmt.Errorf("invalid sip.outbound.allowed_users entry %q", user)
		}
	}
	for _, user := range cfg.Outbound.DeniedUsers {
		if !sipConfigUserPattern.MatchString(user) {
			return fmt.Errorf("invalid sip.outbound.denied_users entry %q", user)
		}
	}
	for _, prefix := range cfg.Outbound.AllowedE164Prefixes {
		if !sipE164PrefixPattern.MatchString(prefix) {
			return fmt.Errorf("invalid sip.outbound.allowed_e164_prefixes entry %q", prefix)
		}
	}
	for _, prefix := range cfg.Outbound.DeniedE164Prefixes {
		if !sipE164PrefixPattern.MatchString(prefix) {
			return fmt.Errorf("invalid sip.outbound.denied_e164_prefixes entry %q", prefix)
		}
	}
	switch cfg.Voice.Backend {
	case "classic":
		switch cfg.Voice.Classic.ASRMode {
		case "", "whisper", "multimodal", "local", "speech_lab":
		default:
			return fmt.Errorf("sip.voice.classic.asr_mode must be whisper, multimodal, local, or speech_lab")
		}
		switch cfg.Voice.Classic.TTSProvider {
		case "", "google", "elevenlabs", "minimax", "mistral", "piper", "supertonic", "speech_lab":
		default:
			return fmt.Errorf("sip.voice.classic.tts_provider is unsupported")
		}
	case "gemini_live":
		if cfg.Voice.RealtimeProfileID == "" {
			return fmt.Errorf("sip.voice.realtime_profile_id is required for gemini_live")
		}
	default:
		return fmt.Errorf("sip.voice.backend must be classic or gemini_live")
	}
	if cfg.Voice.MaxCallDurationSeconds < 30 || cfg.Voice.MaxCallDurationSeconds > 86400 {
		return fmt.Errorf("sip.voice.max_call_duration_seconds must be between 30 and 86400")
	}
	if cfg.Voice.IdleTimeoutSeconds < 15 || cfg.Voice.IdleTimeoutSeconds > 3600 {
		return fmt.Errorf("sip.voice.idle_timeout_seconds must be between 15 and 3600")
	}
	if cfg.Voice.TurnTimeoutSeconds < 15 || cfg.Voice.TurnTimeoutSeconds > 300 {
		return fmt.Errorf("sip.voice.turn_timeout_seconds must be between 15 and 300")
	}
	if cfg.Voice.MaxResponseChars < 200 || cfg.Voice.MaxResponseChars > 5000 {
		return fmt.Errorf("sip.voice.max_response_chars must be between 200 and 5000")
	}
	for name, value := range map[string]string{
		"agent_provider_id":   cfg.Voice.AgentProviderID,
		"asr_provider_id":     cfg.Voice.Classic.ASRProviderID,
		"realtime_profile_id": cfg.Voice.RealtimeProfileID,
	} {
		if len([]rune(value)) > 128 {
			return fmt.Errorf("sip.voice.%s exceeds 128 characters", name)
		}
	}
	if len([]rune(cfg.Voice.Language)) > 32 {
		return fmt.Errorf("sip.voice.language exceeds 32 characters")
	}
	if len(cfg.Voice.AllowedTools) > 128 {
		return fmt.Errorf("sip.voice.allowed_tools must contain at most 128 entries")
	}
	for _, name := range cfg.Voice.AllowedTools {
		if len(name) > 128 {
			return fmt.Errorf("sip.voice.allowed_tools contains an entry longer than 128 characters")
		}
	}
	switch cfg.Voice.Behavior.UnavailableRequestBehavior {
	case "explain", "explain_and_end":
	default:
		return fmt.Errorf("sip.voice.behavior.unavailable_request_behavior must be explain or explain_and_end")
	}
	for name, value := range map[string]string{
		"greeting":                cfg.Voice.Behavior.Greeting,
		"purpose":                 cfg.Voice.Behavior.Purpose,
		"speaking_style":          cfg.Voice.Behavior.SpeakingStyle,
		"additional_prohibitions": cfg.Voice.Behavior.AdditionalProhibitions,
		"failure_message":         cfg.Voice.Behavior.FailureMessage,
		"goodbye_message":         cfg.Voice.Behavior.GoodbyeMessage,
	} {
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("sip.voice.behavior.%s contains forbidden control characters", name)
		}
		limit := 8000
		if name == "greeting" || name == "failure_message" || name == "goodbye_message" {
			limit = 500
		}
		if len([]rune(value)) > limit {
			return fmt.Errorf("sip.voice.behavior.%s exceeds %d characters", name, limit)
		}
	}
	if cfg.HistoryRetentionDays < 1 || cfg.HistoryRetentionDays > 3650 {
		return fmt.Errorf("sip.history_retention_days must be between 1 and 3650")
	}
	return nil
}

func validSIPDomain(value string) bool {
	value = strings.TrimSpace(strings.TrimSuffix(value, "."))
	if value == "" || len(value) > 253 || strings.ContainsAny(value, " \t\r\n/@;?") {
		return false
	}
	if net.ParseIP(strings.Trim(value, "[]")) != nil {
		return true
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func validSIPDomainPattern(value string) bool {
	value = strings.TrimSpace(strings.TrimSuffix(value, "."))
	if value == "" || len(value) > 253 || strings.ContainsAny(value, " \t\r\n/@;") {
		return false
	}
	if strings.Contains(value, ":") {
		return !strings.ContainsAny(value, "*?") && strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") && net.ParseIP(strings.Trim(value, "[]")) != nil
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' && char != '.' &&
			char != '*' && char != '?' {
			return false
		}
	}
	if !strings.ContainsAny(value, "*?") {
		return validSIPDomain(value)
	}
	return !strings.Contains(value, "..")
}

func validSIPCallerPattern(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if !strings.Contains(value, "@") {
		return sipConfigUserPattern.MatchString(value)
	}
	value = strings.TrimPrefix(strings.ToLower(value), "sip:")
	user, domain, ok := strings.Cut(value, "@")
	return ok && sipConfigUserPattern.MatchString(user) && validSIPDomainPattern(domain)
}

// ValidateSIPRuntimeConfig adds checks that require Vault-hydrated secrets.
func ValidateSIPRuntimeConfig(cfg SIPConfig) error {
	if err := ValidateSIPConfig(cfg); err != nil {
		return err
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Password) == "" {
		return fmt.Errorf("enabled SIP requires the Vault password")
	}
	return nil
}

func normalizedUniqueLower(values []string) []string {
	return normalizedUnique(values, true)
}

func normalizedUnique(values []string, lower bool) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

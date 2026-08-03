package sipphone

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"aurago/internal/config"
)

const (
	SetupScopeDisabled = "disabled"
	SetupScopeAll      = "all"
	SetupScopeDomestic = "domestic"
	SetupScopeCustom   = "custom"
)

// SetupActivation is the human-facing call policy selected by guided setup.
// Empty scopes preserve the legacy registration-only behavior.
type SetupActivation struct {
	OutboundScope  string
	OutboundValues []string
	InboundScope   string
	InboundValues  []string
}

// ApplySetupActivation converts guided choices into the strict SIP policy
// lists used by the runtime.
func ApplySetupActivation(ctx context.Context, cfg *config.SIPConfig, presetID string, activation SetupActivation) error {
	if cfg == nil {
		return fmt.Errorf("SIP configuration is required")
	}
	outboundScope := strings.ToLower(strings.TrimSpace(activation.OutboundScope))
	inboundScope := strings.ToLower(strings.TrimSpace(activation.InboundScope))
	if outboundScope == "" && inboundScope == "" {
		return nil
	}
	switch outboundScope {
	case "", SetupScopeDisabled:
		cfg.Outbound.AllowedDomains = nil
		cfg.Outbound.AllowedUsers = nil
		cfg.Outbound.AllowedE164Prefixes = nil
		cfg.Permissions.OriginateOutbound = false
	case SetupScopeAll:
		return fmt.Errorf("unrestricted outbound calling is no longer supported; enter exact destinations")
	case SetupScopeDomestic:
		preset, ok := sipProviderPreset(presetID)
		if !ok || preset.domesticPrefix == "" {
			return fmt.Errorf("domestic calling is unavailable for this SIP provider")
		}
		enableGuidedOutbound(cfg, nil, []string{preset.domesticPrefix})
	case SetupScopeCustom:
		users, prefixes, err := classifyGuidedTargets(activation.OutboundValues)
		if err != nil {
			return err
		}
		enableGuidedOutbound(cfg, users, prefixes)
	default:
		return fmt.Errorf("unsupported outbound calling scope")
	}

	switch inboundScope {
	case "", SetupScopeDisabled:
		cfg.Inbound.Route = "reject"
		cfg.Inbound.TrustedPeerCIDRs = nil
		cfg.Inbound.AllowedCallers = nil
		cfg.Permissions.AnswerInbound = false
	case SetupScopeAll, SetupScopeCustom:
		peers, err := ResolveRegistrarPeers(ctx, cfg.Registrar)
		if err != nil || len(peers) == 0 {
			return fmt.Errorf("trusted SIP peers could not be detected automatically")
		}
		callers := []string{"*"}
		if inboundScope == SetupScopeCustom {
			callers, err = classifyGuidedCallers(activation.InboundValues)
			if err != nil {
				return err
			}
		}
		cfg.Inbound.Route = "manual"
		cfg.Inbound.TrustedPeerCIDRs = peers
		cfg.Inbound.AllowedCallers = callers
		cfg.Permissions.AnswerInbound = true
	default:
		return fmt.Errorf("unsupported incoming calling scope")
	}

	if cfg.Permissions.OriginateOutbound || cfg.Permissions.AnswerInbound {
		cfg.Enabled = true
		cfg.ReadOnly = false
		cfg.BrowserMedia.Enabled = true
		cfg.Permissions.SendDTMF = true
		cfg.Permissions.AgentHangup = true
	}
	return nil
}

func enableGuidedOutbound(cfg *config.SIPConfig, users, prefixes []string) {
	cfg.Outbound.AllowedDomains = []string{cfg.Domain}
	cfg.Outbound.AllowedUsers = append([]string(nil), users...)
	cfg.Outbound.AllowedE164Prefixes = append([]string(nil), prefixes...)
	cfg.Permissions.OriginateOutbound = true
}

func classifyGuidedTargets(values []string) ([]string, []string, error) {
	if len(values) == 0 || len(values) > 64 {
		return nil, nil, fmt.Errorf("enter between 1 and 64 allowed destinations")
	}
	users := make([]string, 0, len(values))
	prefixes := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || strings.ContainsAny(value, "\r\n\x00*?") {
			return nil, nil, fmt.Errorf("guided destinations must be individual numbers or extensions")
		}
		switch {
		case e164Prefix.MatchString(value):
			prefixes = append(prefixes, value)
		case sipUserPattern.MatchString(value):
			users = append(users, value)
		default:
			return nil, nil, fmt.Errorf("invalid guided destination")
		}
	}
	users = uniqueSortedStrings(users)
	prefixes = uniqueSortedStrings(prefixes)
	if len(users) == 0 && len(prefixes) == 0 {
		return nil, nil, fmt.Errorf("at least one allowed destination is required")
	}
	return users, prefixes, nil
}

func classifyGuidedCallers(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 64 {
		return nil, fmt.Errorf("enter between 1 and 64 allowed callers")
	}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || strings.ContainsAny(value, "\r\n\x00*?") {
			return nil, fmt.Errorf("guided callers must be individual numbers or extensions")
		}
		if !sipUserPattern.MatchString(value) {
			return nil, fmt.Errorf("invalid guided caller")
		}
		result = append(result, value)
	}
	return uniqueSortedStrings(result), nil
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// ResolveRegistrarPeers returns exact registrar IPs suitable for the inbound
// network allowlist. Hostnames are resolved at setup time and never widened to
// arbitrary subnets.
func ResolveRegistrarPeers(ctx context.Context, registrar string) ([]string, error) {
	host := sipRegistrarHost(registrar)
	if host == "" {
		return nil, fmt.Errorf("SIP registrar is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		return []string{ip.String()}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve SIP registrar: %w", err)
	}
	peers := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address.IP == nil || address.IP.IsUnspecified() || address.IP.IsMulticast() {
			continue
		}
		peers = append(peers, address.IP.String())
	}
	return uniqueSortedStrings(peers), nil
}

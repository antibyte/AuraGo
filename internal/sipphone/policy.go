package sipphone

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"aurago/internal/config"

	"github.com/emiago/sipgo/sip"
)

var (
	sipUserPattern  = regexp.MustCompile(`^[A-Za-z0-9_.!~*'()%+\-]+$`)
	e164Pattern     = regexp.MustCompile(`^\+[1-9][0-9]{3,14}$`)
	e164Prefix      = regexp.MustCompile(`^\+[1-9][0-9]{0,14}$`)
	fritzInternal   = regexp.MustCompile(`^\*\*[0-9]{1,4}$`)
	phoneNumber     = regexp.MustCompile(`^(?:\+|00|0)[0-9]+$`)
	headerNameToken = regexp.MustCompile(`^[A-Za-z0-9.!%*_+~\-]+$`)
)

func NormalizeSIPURI(raw string) (sip.Uri, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\x00") {
		return sip.Uri{}, "", fmt.Errorf("invalid SIP URI")
	}
	var uri sip.Uri
	if err := sip.ParseUri(raw, &uri); err != nil {
		return sip.Uri{}, "", fmt.Errorf("parse SIP URI: %w", err)
	}
	if strings.ToLower(uri.Scheme) != "sip" || uri.User == "" || uri.Host == "" || uri.Password != "" || uri.Wildcard || uri.HierarhicalSlashes {
		return sip.Uri{}, "", fmt.Errorf("SIP URI must be canonical sip:user@domain")
	}
	if !sipUserPattern.MatchString(uri.User) || strings.ContainsAny(uri.Host, "\r\n\x00") {
		return sip.Uri{}, "", fmt.Errorf("invalid SIP user or host")
	}
	if uri.Headers != nil && uri.Headers.Length() != 0 {
		return sip.Uri{}, "", fmt.Errorf("SIP URI headers are not allowed")
	}
	if uri.UriParams != nil && uri.UriParams.Length() != 0 {
		return sip.Uri{}, "", fmt.Errorf("SIP URI parameters are not allowed")
	}
	uri.Scheme = "sip"
	uri.Host = strings.ToLower(strings.TrimSuffix(uri.Host, "."))
	return uri, uri.String(), nil
}

func DestinationAllowed(cfg config.SIPOutboundConfig, accountDomain string, uri sip.Uri) bool {
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(uri.Host), "."))
	for _, domain := range cfg.DeniedDomains {
		if wildcardMatch(strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), ".")), host) {
			return false
		}
	}
	for _, user := range cfg.DeniedUsers {
		if wildcardMatch(strings.TrimSpace(user), uri.User) {
			return false
		}
	}
	if e164Pattern.MatchString(uri.User) {
		for _, prefix := range cfg.DeniedE164Prefixes {
			prefix = strings.TrimSpace(prefix)
			if e164Prefix.MatchString(prefix) && strings.HasPrefix(uri.User, prefix) {
				return false
			}
		}
	}
	domainAllowed := false
	if len(cfg.AllowedDomains) == 0 {
		domainAllowed = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(accountDomain), ".")) == host
	} else {
		for _, domain := range cfg.AllowedDomains {
			if strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), ".")) == host {
				domainAllowed = true
				break
			}
		}
	}
	if !domainAllowed {
		return false
	}
	if len(cfg.AllowedUsers) == 0 && len(cfg.AllowedE164Prefixes) == 0 {
		return true
	}
	for _, user := range cfg.AllowedUsers {
		if strings.TrimSpace(user) == uri.User {
			return true
		}
	}
	if !e164Pattern.MatchString(uri.User) {
		return false
	}
	for _, prefix := range cfg.AllowedE164Prefixes {
		prefix = strings.TrimSpace(prefix)
		if e164Prefix.MatchString(prefix) && strings.HasPrefix(uri.User, prefix) {
			return true
		}
	}
	return false
}

// OutboundPolicyMigrationRequired reports legacy wildcard allow entries. They
// remain loadable for configuration compatibility but never grant access.
func OutboundPolicyMigrationRequired(cfg config.SIPOutboundConfig) bool {
	for _, values := range [][]string{cfg.AllowedDomains, cfg.AllowedUsers} {
		for _, value := range values {
			if strings.ContainsAny(value, "*?") {
				return true
			}
		}
	}
	return false
}

func CallerAllowed(cfg config.SIPInboundConfig, source string, from sip.Uri, regions ...string) bool {
	if len(cfg.AllowedCallers) == 0 || len(cfg.TrustedPeerCIDRs) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(source)
	if err != nil {
		host = source
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	trusted := false
	for _, rawCIDR := range cfg.TrustedPeerCIDRs {
		rawCIDR = strings.TrimSpace(rawCIDR)
		if trustedIP := net.ParseIP(rawCIDR); trustedIP != nil && trustedIP.Equal(ip) {
			trusted = true
			break
		}
		_, network, err := net.ParseCIDR(rawCIDR)
		if err == nil && network.Contains(ip) {
			trusted = true
			break
		}
	}
	if !trusted {
		return false
	}
	canonical := strings.ToLower((&sip.Uri{Scheme: "sip", User: from.User, Host: strings.ToLower(strings.TrimSuffix(from.Host, "."))}).String())
	user := strings.ToLower(from.User)
	region := cfg.NumberRegion
	if len(regions) > 0 && strings.TrimSpace(regions[0]) != "" {
		region = regions[0]
	}
	user = canonicalInboundNumber(user, region)
	for _, denied := range cfg.DeniedCallers {
		denied = strings.ToLower(strings.TrimSpace(denied))
		if callerPatternMatches(denied, user, canonical, region) {
			return false
		}
	}
	for _, allowed := range cfg.AllowedCallers {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if callerPatternMatches(allowed, user, canonical, region) {
			return true
		}
	}
	return false
}

func isFritzInternalNumber(value string) bool {
	return fritzInternal.MatchString(strings.TrimSpace(value))
}

func callerPatternMatches(pattern, user, canonical, region string) bool {
	if isFritzInternalNumber(pattern) {
		return pattern == user
	}
	if phoneNumber.MatchString(pattern) {
		return canonicalInboundNumber(pattern, region) == user
	}
	return wildcardMatch(pattern, user) || wildcardMatch(pattern, canonical)
}

func canonicalInboundNumber(value, region string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "00") && len(value) > 2 {
		return "+" + value[2:]
	}
	if strings.HasPrefix(value, "0") && !strings.HasPrefix(value, "00") && len(value) > 1 {
		prefixes := map[string]string{
			"DE": "+49", "CH": "+41", "AT": "+43", "GB": "+44", "UK": "+44",
			"FR": "+33", "NL": "+31", "BE": "+32", "IT": "+39", "AU": "+61", "NZ": "+64",
		}
		if prefix := prefixes[strings.ToUpper(strings.TrimSpace(region))]; prefix != "" {
			return prefix + value[1:]
		}
	}
	return value
}

// wildcardMatch supports the deliberately small policy syntax used by SIP lists:
// '*' matches any sequence and '?' matches exactly one character.
func wildcardMatch(pattern, value string) bool {
	patternRunes := []rune(pattern)
	valueRunes := []rune(value)
	patternIndex, valueIndex := 0, 0
	starIndex, starValueIndex := -1, 0
	for valueIndex < len(valueRunes) {
		if patternIndex < len(patternRunes) && (patternRunes[patternIndex] == '?' || patternRunes[patternIndex] == valueRunes[valueIndex]) {
			patternIndex++
			valueIndex++
			continue
		}
		if patternIndex < len(patternRunes) && patternRunes[patternIndex] == '*' {
			starIndex = patternIndex
			starValueIndex = valueIndex
			patternIndex++
			continue
		}
		if starIndex < 0 {
			return false
		}
		patternIndex = starIndex + 1
		starValueIndex++
		valueIndex = starValueIndex
	}
	for patternIndex < len(patternRunes) && patternRunes[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(patternRunes)
}

func ValidateRequest(req *sip.Request) error {
	if req == nil || req.Method == "" || strings.ContainsAny(string(req.Method), "\r\n\x00") {
		return fmt.Errorf("invalid request method")
	}
	if req.SipVersion != "SIP/2.0" || req.CallID() == nil || req.Via() == nil || req.From() == nil || req.To() == nil || req.CSeq() == nil {
		return fmt.Errorf("missing mandatory SIP header")
	}
	if len(req.Body()) > 64*1024 {
		return fmt.Errorf("SIP body exceeds 64 KiB")
	}
	if req.Recipient.Host == "" || strings.ContainsAny(req.Recipient.String(), "\r\n\x00") {
		return fmt.Errorf("invalid request URI")
	}
	if len(req.Headers()) > 128 {
		return fmt.Errorf("too many SIP headers")
	}
	for _, header := range req.Headers() {
		if header == nil {
			continue
		}
		if !headerNameToken.MatchString(header.Name()) || strings.ContainsAny(header.Value(), "\r\n\x00") || len(header.Value()) > 8192 {
			return fmt.Errorf("invalid SIP header")
		}
	}
	return nil
}

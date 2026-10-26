package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"golang.org/x/net/publicsuffix"
)

// WebAuthnConfig is operator-owned; request hosts never select the relying party.
type WebAuthnConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	LoginMode string   `yaml:"login_mode" json:"login_mode"`
	RPID      string   `yaml:"rp_id" json:"rp_id"`
	Origins   []string `yaml:"origins" json:"origins"`
}

func ValidateWebAuthnConfig(c WebAuthnConfig) error {
	if !c.Enabled && c.LoginMode == "" && c.RPID == "" && len(c.Origins) == 0 {
		return nil
	}
	if c.LoginMode != "passkey" && c.LoginMode != "second_factor" {
		return fmt.Errorf("webauthn login_mode must be passkey or second_factor")
	}
	if c.RPID != strings.ToLower(strings.TrimSpace(c.RPID)) || strings.HasSuffix(c.RPID, ".") {
		return fmt.Errorf("webauthn rp_id must be a lowercase domain without a trailing dot")
	}
	if err := protocol.ValidateRPID(c.RPID); err != nil {
		return fmt.Errorf("webauthn rp_id: %w", err)
	}
	if suffix, _ := publicsuffix.PublicSuffix(c.RPID); c.RPID != "localhost" && suffix == c.RPID {
		return fmt.Errorf("webauthn rp_id must not be a public suffix")
	}
	if len(c.Origins) == 0 || len(c.Origins) > 8 {
		return fmt.Errorf("webauthn requires between one and eight origins")
	}
	seen := make(map[string]bool)
	for _, origin := range c.Origins {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || origin != u.Scheme+"://"+u.Host || strings.HasSuffix(u.Host, ":") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
			return fmt.Errorf("webauthn origins must contain only scheme, host and optional port")
		}
		if origin != strings.ToLower(strings.TrimSpace(origin)) || strings.Contains(origin, "*") {
			return fmt.Errorf("webauthn origins must be lowercase and must not contain wildcards")
		}
		host := u.Hostname()
		if port := u.Port(); port != "" {
			number, err := strconv.Atoi(port)
			if err != nil || number < 1 || number > 65535 {
				return fmt.Errorf("webauthn origin port must be between 1 and 65535")
			}
		}
		if u.Scheme != "https" && !(u.Scheme == "http" && host == "localhost" && c.RPID == "localhost") {
			return fmt.Errorf("webauthn requires HTTPS, except explicit http://localhost development")
		}
		if host != c.RPID && !strings.HasSuffix(host, "."+c.RPID) {
			return fmt.Errorf("webauthn origin is outside the configured rp_id")
		}
		if seen[origin] {
			return fmt.Errorf("webauthn origins must be unique")
		}
		seen[origin] = true
	}
	return nil
}

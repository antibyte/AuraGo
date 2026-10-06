package proxy

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"aurago/internal/config"
	"aurago/internal/security"
)

// basicAuthHashCost is the bcrypt cost of the Basic Auth hash. Caddy caches
// verified credentials, so the cost is paid per credential, not per request.
const basicAuthHashCost = 12

// GenerateCaddyfile builds a Caddyfile from the SecurityProxy configuration.
// With basic auth enabled it embeds the bcrypt hash of the Vault password, so
// the result must be written with writeCaddyfile (mode 0600).
func GenerateCaddyfile(cfg *config.Config, upstream string) (string, error) {
	sp := cfg.SecurityProxy
	account, err := basicAuthAccount(cfg)
	if err != nil {
		return "", err
	}
	var sb strings.Builder

	// Global options
	sb.WriteString("{\n")
	if sp.Email != "" {
		sb.WriteString(fmt.Sprintf("\temail %s\n", sp.Email))
	}
	sb.WriteString("}\n\n")

	// Main AuraGo route
	writeRouteBlock(&sb, sp.Domain, sp.HTTPSPort, upstream, cfg, account)

	// Additional routes
	for _, route := range sp.AdditionalRoutes {
		if route.Domain == "" || route.Upstream == "" {
			continue
		}
		writeRouteBlock(&sb, route.Domain, 0, route.Upstream, cfg, account)
	}

	return sb.String(), nil
}

// basicAuthAccount returns the basic_auth account line for the Vault
// credentials, or "" while basic auth is off.
func basicAuthAccount(cfg *config.Config) (string, error) {
	ba := cfg.SecurityProxy.BasicAuth
	if !ba.Enabled {
		return "", nil
	}
	if ba.Username == "" || ba.Password == "" {
		return "", ErrBasicAuthCredentialsMissing
	}
	if err := validateBasicAuthUsername(ba.Username); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBasicAuthCredentialsInvalid, err)
	}
	// bcrypt only uses the first 72 bytes; a longer password would accept
	// anything that shares its prefix.
	if len(ba.Password) > 72 {
		return "", fmt.Errorf("%w: the password is longer than 72 bytes", ErrBasicAuthCredentialsInvalid)
	}
	security.RegisterSensitive(ba.Password)
	hash, err := bcrypt.GenerateFromPassword([]byte(ba.Password), basicAuthHashCost)
	if err != nil {
		return "", fmt.Errorf("%w: hash password: %v", ErrBasicAuthCredentialsInvalid, err)
	}
	return `"` + ba.Username + `" ` + string(hash), nil
}

// validateBasicAuthUsername rejects names that HTTP Basic Auth cannot carry
// (':') or that a quoted Caddyfile token would change: quotes, backslashes,
// braces (Caddyfile placeholders) and non-printable characters.
func validateBasicAuthUsername(user string) error {
	for _, r := range user {
		switch {
		case r == ':':
			return errors.New("the user name must not contain ':'")
		case r == '"' || r == '`' || r == '\\' || r == '{' || r == '}':
			return fmt.Errorf("the user name must not contain %q", r)
		case !unicode.IsPrint(r):
			return errors.New("the user name must not contain control characters")
		}
	}
	return nil
}

// writeRouteBlock writes a single site block for a domain/upstream pair.
func writeRouteBlock(sb *strings.Builder, domain string, httpsPort int, upstream string, cfg *config.Config, basicAuthAccount string) {
	sp := cfg.SecurityProxy

	// Site address
	if domain != "" {
		sb.WriteString(domain)
	} else {
		// No domain: listen on HTTPS port with automatic certs disabled
		sb.WriteString(fmt.Sprintf(":%d", httpsPort))
	}
	sb.WriteString(" {\n")

	// Security headers
	sb.WriteString("\theader {\n")
	sb.WriteString("\t\tX-Content-Type-Options nosniff\n")
	sb.WriteString("\t\tX-Frame-Options SAMEORIGIN\n")
	sb.WriteString("\t\tReferrer-Policy strict-origin-when-cross-origin\n")
	sb.WriteString("\t\t-Server\n")
	sb.WriteString("\t}\n\n")

	// IP filter
	if sp.IPFilter.Enabled && len(sp.IPFilter.Addresses) > 0 {
		writeIPFilter(sb, cfg)
	}

	// Basic Auth
	if basicAuthAccount != "" {
		sb.WriteString("\tbasic_auth * {\n")
		sb.WriteString("\t\t" + basicAuthAccount + "\n")
		sb.WriteString("\t}\n\n")
	}

	// Rate limiting (caddy-ratelimit module, see rateLimitImageName)
	if sp.RateLimiting.Enabled {
		writeRateLimiting(sb, cfg)
	}

	// Reverse proxy
	sb.WriteString(fmt.Sprintf("\treverse_proxy %s {\n", upstream))
	sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
	sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
	sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	// WebSocket support
	sb.WriteString("\t\theader_up Connection {>Connection}\n")
	sb.WriteString("\t\theader_up Upgrade {>Upgrade}\n")
	sb.WriteString("\t}\n")

	sb.WriteString("}\n\n")
}

func writeIPFilter(sb *strings.Builder, cfg *config.Config) {
	sp := cfg.SecurityProxy
	addrs := strings.Join(sp.IPFilter.Addresses, " ")
	if sp.IPFilter.Mode == "allowlist" {
		sb.WriteString("\t@blocked not remote_ip " + addrs + "\n")
		sb.WriteString("\trespond @blocked 403\n\n")
	} else {
		sb.WriteString("\t@blocked remote_ip " + addrs + "\n")
		sb.WriteString("\trespond @blocked 403\n\n")
	}
}

// writeRateLimiting emits one caddy-ratelimit zone keyed by client IP. Its
// sliding window admits `burst` requests per burst/requests_per_second
// seconds: bursts of up to `burst` requests and requests_per_second on
// average. The module orders rate_limit before basic_auth, so it also slows
// down password guessing.
func writeRateLimiting(sb *strings.Builder, cfg *config.Config) {
	rl := cfg.SecurityProxy.RateLimiting
	events := max(rl.Burst, 1)
	perSecond := max(rl.RequestsPerSecond, 1)
	windowMS := (int64(events)*1000 + int64(perSecond) - 1) / int64(perSecond)
	window := time.Duration(max(windowMS, 1)) * time.Millisecond

	sb.WriteString("\trate_limit {\n")
	sb.WriteString("\t\tzone aurago_client {\n")
	sb.WriteString("\t\t\tkey {http.request.remote.host}\n")
	sb.WriteString(fmt.Sprintf("\t\t\tevents %d\n", events))
	sb.WriteString(fmt.Sprintf("\t\t\twindow %s\n", window))
	sb.WriteString("\t\t}\n")
	sb.WriteString("\t}\n\n")
}

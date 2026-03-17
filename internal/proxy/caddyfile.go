package proxy

import (
	"fmt"
	"strings"

	"aurago/internal/config"
)

// GenerateCaddyfile builds a Caddyfile from the SecurityProxy configuration.
func GenerateCaddyfile(cfg *config.Config, upstream string) string {
	sp := cfg.SecurityProxy
	var sb strings.Builder

	// Global options
	sb.WriteString("{\n")
	if sp.Email != "" {
		sb.WriteString(fmt.Sprintf("\temail %s\n", sp.Email))
	}
	sb.WriteString("}\n\n")

	// Main AuraGo route
	writeRouteBlock(&sb, sp.Domain, sp.HTTPSPort, upstream, cfg)

	// Additional routes
	for _, route := range sp.AdditionalRoutes {
		if route.Domain == "" || route.Upstream == "" {
			continue
		}
		writeRouteBlock(&sb, route.Domain, 0, route.Upstream, cfg)
	}

	return sb.String()
}

// writeRouteBlock writes a single site block for a domain/upstream pair.
func writeRouteBlock(sb *strings.Builder, domain string, httpsPort int, upstream string, cfg *config.Config) {
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
	if sp.BasicAuth.Enabled {
		sb.WriteString("\tbasicauth * {\n")
		sb.WriteString("\t\t{$PROXY_BASIC_AUTH_USER} {$PROXY_BASIC_AUTH_HASH}\n")
		sb.WriteString("\t}\n\n")
	}

	// Rate limiting (using caddy-ratelimit plugin)
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

func writeRateLimiting(sb *strings.Builder, cfg *config.Config) {
	sp := cfg.SecurityProxy
	sb.WriteString(fmt.Sprintf("\trate_limit {remote.host} %d %d\n\n",
		sp.RateLimiting.RequestsPerSecond,
		sp.RateLimiting.Burst))
}

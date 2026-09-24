package server

import (
	"aurago/internal/config"
	"fmt"
	"net/netip"
	"strings"
)

// validateRemoteAuthExposure runs before any AuraGo listener or managed
// integration starts. A loopback bind is safe only when no remote ingress is
// configured through HTTPS, the reverse proxy, a tunnel, or Tailscale.
func validateRemoteAuthExposure(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("server configuration is required")
	}
	if cfg.Auth.Enabled || cfg.Auth.AllowUnauthenticatedRemote {
		return nil
	}
	host := strings.TrimSpace(cfg.Server.Host)
	ip, err := netip.ParseAddr(strings.Trim(host, "[]"))
	loopback := strings.EqualFold(host, "localhost") || (err == nil && ip.IsLoopback())
	remote := !loopback || cfg.Server.HTTPS.Enabled || cfg.Server.HTTPS.BehindProxy || cfg.SecurityProxy.Enabled ||
		(cfg.CloudflareTunnel.Enabled && cfg.CloudflareTunnel.ExposeWebUI) || cfg.Tailscale.TsNet.Enabled
	if remote {
		return fmt.Errorf("remote AuraGo listener requires auth.enabled or explicit auth.allow_unauthenticated_remote (unsafe)")
	}
	return nil
}

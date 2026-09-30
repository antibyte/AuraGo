package server

import (
	"aurago/internal/config"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

var errUnauthenticatedRemoteExposure = errors.New("remote AuraGo listener requires auth.enabled or explicit auth.allow_unauthenticated_remote (unsafe)")

// validateRemoteAuthExposure runs before any AuraGo listener or managed
// integration starts, and before config saves. A loopback bind is safe only
// when no remote ingress is configured through HTTPS, the reverse proxy, a
// tunnel, or Tailscale.
func validateRemoteAuthExposure(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("server configuration is required")
	}
	if cfg.Auth.Enabled || cfg.Auth.AllowUnauthenticatedRemote {
		return nil
	}
	if configAllowsRemoteIngress(cfg) {
		return errUnauthenticatedRemoteExposure
	}
	return nil
}

// configAllowsRemoteIngress reports whether anything other than a loopback
// peer can reach the AuraGo listener. The bind host honors the
// AURAGO_SERVER_HOST override, which config.yaml does not show.
func configAllowsRemoteIngress(cfg *config.Config) bool {
	host := strings.TrimSpace(config.EffectiveServerHost(cfg.Server.Host))
	ip, err := netip.ParseAddr(strings.Trim(host, "[]"))
	loopback := strings.EqualFold(host, "localhost") || (err == nil && ip.IsLoopback())
	return !loopback || cfg.Server.HTTPS.Enabled || cfg.Server.HTTPS.BehindProxy || cfg.SecurityProxy.Enabled ||
		(cfg.CloudflareTunnel.Enabled && cfg.CloudflareTunnel.ExposeWebUI) || cfg.Tailscale.TsNet.Enabled
}

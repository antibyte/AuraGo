package server

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func homepageDevAutoStartAllowed(cfg *config.Config) bool {
	return cfg != nil &&
		cfg.Docker.Enabled &&
		cfg.Homepage.Enabled &&
		strings.TrimSpace(cfg.Homepage.WorkspacePath) != ""
}

func homepageWebServerAutoStartAllowed(cfg *config.Config) bool {
	return cfg != nil &&
		cfg.Docker.Enabled &&
		cfg.Homepage.WebServerEnabled &&
		strings.TrimSpace(cfg.Homepage.WorkspacePath) != ""
}

func securityProxyAutoStartAllowed(cfg *config.Config) bool {
	return cfg != nil && cfg.Docker.Enabled && cfg.SecurityProxy.Enabled
}

func cloudflareTunnelRuntimeAllowed(cfg *config.Config) bool {
	if cfg == nil || !cfg.CloudflareTunnel.Enabled {
		return false
	}
	if cfg.Docker.Enabled {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(cfg.CloudflareTunnel.Mode), "docker")
}

func cloudflareTunnelAutoStartAllowed(cfg *config.Config) bool {
	return cloudflareTunnelRuntimeAllowed(cfg) && cfg.CloudflareTunnel.AutoStart
}

func cloudflareTunnelRuntimeConfig(cfg *config.Config) tools.CloudflareTunnelConfig {
	return tools.CloudflareTunnelConfigFromConfig(cfg)
}

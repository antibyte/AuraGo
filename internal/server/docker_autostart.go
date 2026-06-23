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
	if cfg == nil {
		return tools.CloudflareTunnelConfig{}
	}
	mode := strings.TrimSpace(cfg.CloudflareTunnel.Mode)
	if !cfg.Docker.Enabled && (mode == "" || strings.EqualFold(mode, "auto")) {
		mode = "native"
	}
	tunnelCfg := tools.CloudflareTunnelConfig{
		Enabled:        cfg.CloudflareTunnel.Enabled,
		ReadOnly:       cfg.CloudflareTunnel.ReadOnly,
		Mode:           mode,
		AutoStart:      cfg.CloudflareTunnel.AutoStart,
		AuthMethod:     cfg.CloudflareTunnel.AuthMethod,
		TunnelName:     cfg.CloudflareTunnel.TunnelName,
		AccountID:      cfg.CloudflareTunnel.AccountID,
		TunnelID:       cfg.CloudflareTunnel.TunnelID,
		LoopbackPort:   cfg.CloudflareTunnel.LoopbackPort,
		ExposeWebUI:    cfg.CloudflareTunnel.ExposeWebUI,
		ExposeHomepage: cfg.CloudflareTunnel.ExposeHomepage,
		MetricsPort:    cfg.CloudflareTunnel.MetricsPort,
		LogLevel:       cfg.CloudflareTunnel.LogLevel,
		DockerHost:     cfg.Docker.Host,
		WebUIPort:      cfg.Server.Port,
		HomepagePort:   cfg.Homepage.WebServerPort,
		DataDir:        cfg.Directories.DataDir,
		HTTPSEnabled:   cfg.Server.HTTPS.Enabled,
		HTTPSPort:      cfg.Server.HTTPS.HTTPSPort,
	}
	for _, r := range cfg.CloudflareTunnel.CustomIngress {
		tunnelCfg.CustomIngress = append(tunnelCfg.CustomIngress, tools.CloudflareIngress{
			Hostname: r.Hostname,
			Service:  r.Service,
			Path:     r.Path,
		})
	}
	return tunnelCfg
}

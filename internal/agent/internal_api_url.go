package agent

import (
	"fmt"

	"aurago/internal/config"
)

func internalAPIBaseURL(cfg *config.Config) string {
	if cfg == nil {
		return "http://127.0.0.1:8088"
	}
	if cfg.CloudflareTunnel.LoopbackPort > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d", cfg.CloudflareTunnel.LoopbackPort)
	}
	if cfg.Server.HTTPS.Enabled {
		port := cfg.Server.Port
		if port > 0 && port != cfg.Server.HTTPS.HTTPSPort && port != cfg.Server.HTTPS.HTTPPort {
			return fmt.Sprintf("http://127.0.0.1:%d", port)
		}
		httpsPort := cfg.Server.HTTPS.HTTPSPort
		if httpsPort <= 0 {
			httpsPort = 443
		}
		return fmt.Sprintf("https://127.0.0.1:%d", httpsPort)
	}
	port := cfg.Server.Port
	if port <= 0 {
		port = 8088
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

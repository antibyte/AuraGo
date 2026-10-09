package config

import (
	"fmt"
	"math"

	"gopkg.in/yaml.v3"
)

// ValidateCloudflareTunnelPortYAML prevents yaml.v3 from truncating fractions into ints.
func ValidateCloudflareTunnelPortYAML(data []byte) error {
	var document struct {
		Cloudflare map[string]interface{} `yaml:"cloudflare_tunnel"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("validate Cloudflare ports: %w", err)
	}
	for _, name := range []string{"loopback_port", "metrics_port"} {
		value, exists := document.Cloudflare[name]
		if !exists {
			continue
		}
		var number float64
		switch v := value.(type) {
		case int:
			number = float64(v)
		case float64:
			number = v
		default:
			return fmt.Errorf("cloudflare_tunnel.%s must be an integer", name)
		}
		if math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number < 0 || number > 65535 {
			return fmt.Errorf("cloudflare_tunnel.%s must be 0 or an integer between 1 and 65535", name)
		}
	}
	return nil
}

// ValidateCloudflareTunnelPorts validates optional ports against active listeners.
func ValidateCloudflareTunnelPorts(loopback, metrics int, listeners []int) error {
	for name, port := range map[string]int{"loopback_port": loopback, "metrics_port": metrics} {
		if port < 0 || port > 65535 {
			return fmt.Errorf("cloudflare_tunnel.%s must be 0 or a port between 1 and 65535", name)
		}
		if port == 0 {
			continue
		}
		for _, active := range listeners {
			if active > 0 && port == active {
				return fmt.Errorf("cloudflare_tunnel.%s conflicts with an active listener on port %d", name, port)
			}
		}
	}
	if loopback > 0 && loopback == metrics {
		return fmt.Errorf("Cloudflare loopback and metrics ports must differ")
	}
	return nil
}

func ValidateCloudflareTunnelConfig(c *Config) error {
	if c == nil {
		return nil
	}
	listeners := []int{}
	if c.Server.HTTPS.Enabled {
		port := c.Server.HTTPS.HTTPSPort
		if port <= 0 {
			port = 443
		}
		listeners = append(listeners, port, c.Server.HTTPS.HTTPPort)
	} else {
		listeners = append(listeners, c.Server.Port)
	}
	if c.Homepage.WebServerEnabled {
		port := c.Homepage.WebServerPort
		if port <= 0 {
			port = 8080
		}
		listeners = append(listeners, port)
	}
	if c.Server.HTTPS.Enabled && c.CloudflareTunnel.LoopbackPort == 0 && c.CloudflareTunnel.MetricsPort == c.Server.Port && c.Server.Port > 0 {
		return fmt.Errorf("Cloudflare metrics port conflicts with the internal loopback listener")
	}
	return ValidateCloudflareTunnelPorts(c.CloudflareTunnel.LoopbackPort, c.CloudflareTunnel.MetricsPort, listeners)
}

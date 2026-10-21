package config

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ManagedOllamaURL is shared by configuration, health checks and model pulls.
func ManagedOllamaURL(inDocker, embeddings bool, port int) string {
	name := "aurago_ollama_managed"
	if embeddings {
		name = "aurago_ollama_embeddings"
	}
	if inDocker {
		return "http://" + name + ":11434"
	}
	if port == 0 {
		port = 11434
		if embeddings {
			port = 11435
		}
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// ParseOllamaMemoryLimit accepts bytes or a binary k/m/g suffix, without overflow.
func ParseOllamaMemoryLimit(value string) (int64, bool) {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "" {
		return 0, false
	}
	multiplier := int64(1)
	for suffix, factor := range map[string]int64{"k": 1 << 10, "m": 1 << 20, "g": 1 << 30} {
		if strings.HasSuffix(s, suffix) {
			multiplier = factor
			s = strings.TrimSuffix(s, suffix)
			break
		}
	}
	if s == "" || strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n > math.MaxInt64/multiplier {
		return 0, false
	}
	return n * multiplier, true
}

// ValidateOllamaSettings validates saved inputs before any container mutation.
func ValidateOllamaSettings(c Config) error {
	mi, lo := c.Ollama.ManagedInstance, c.Embeddings.LocalOllama
	for path, port := range map[string]int{"ollama.managed_instance.container_port": mi.ContainerPort, "embeddings.local_ollama.container_port": lo.ContainerPort} {
		if port < 0 || port > 65535 {
			return fmt.Errorf("%s must be between 1 and 65535 (0 selects the default)", path)
		}
	}
	if strings.TrimSpace(mi.MemoryLimit) != "" {
		if _, ok := ParseOllamaMemoryLimit(mi.MemoryLimit); !ok {
			return fmt.Errorf("ollama.managed_instance.memory_limit must be positive bytes or use k/m/g, for example 8g")
		}
	}
	if !c.Runtime.IsDocker {
		host := strings.TrimSpace(c.Docker.Host)
		if host == "" {
			host = strings.TrimSpace(os.Getenv("DOCKER_HOST"))
		}
		if mi.Enabled || lo.Enabled {
			if target, err := url.Parse(host); err == nil && (target.Scheme == "tcp" || target.Scheme == "http" || target.Scheme == "https" || target.Scheme == "ssh") {
				ip := net.ParseIP(target.Hostname())
				if target.Scheme == "ssh" || target.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
					return fmt.Errorf("managed Ollama requires a local Docker Engine on native installations; configure an external Ollama endpoint for remote Engines")
				}
			}
		}
		mp, ep := mi.ContainerPort, lo.ContainerPort
		if mp == 0 {
			mp = 11434
		}
		if ep == 0 {
			ep = 11435
		}
		if mi.Enabled && lo.Enabled && mp == ep {
			return fmt.Errorf("ollama.managed_instance.container_port conflicts with embeddings.local_ollama.container_port")
		}
		for path, port := range map[string]int{"ollama.managed_instance.container_port": mp, "embeddings.local_ollama.container_port": ep} {
			enabled := mi.Enabled
			if strings.HasPrefix(path, "embeddings") {
				enabled = lo.Enabled
			}
			serverIP := net.ParseIP(strings.TrimSpace(c.Server.Host))
			overlapsLoopback := serverIP == nil || serverIP.IsUnspecified() || serverIP.IsLoopback()
			if enabled && overlapsLoopback && (port == c.Server.Port || c.Server.HTTPS.Enabled && (port == c.Server.HTTPS.HTTPSPort || port == c.Server.HTTPS.HTTPPort)) {
				return fmt.Errorf("%s conflicts with the AuraGo server port", path)
			}
		}
	}
	return nil
}

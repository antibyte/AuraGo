package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

var cloudflareDNSLabel = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

func validateCloudflareRuntime(cfg CloudflareTunnelConfig) error {
	if cfg.AuthMethod != "token" && cfg.AuthMethod != "named" && cfg.AuthMethod != "quick" {
		return fmt.Errorf("unknown Cloudflare auth_method; use token, named or quick")
	}
	listeners := []int{}
	if cfg.HTTPSEnabled {
		listeners = append(listeners, effectiveHTTPSPort(cfg), cfg.HTTPRedirectPort)
	} else {
		listeners = append(listeners, cfg.WebUIPort)
	}
	if cfg.HomepageServing {
		listeners = append(listeners, cfg.HomepagePort)
	}
	if cfg.HTTPSEnabled && cfg.LoopbackPort == 0 && cfg.WebUIPort != effectiveHTTPSPort(cfg) && cfg.WebUIPort != cfg.HTTPRedirectPort {
		listeners = append(listeners, cfg.WebUIPort)
	}
	if err := config.ValidateCloudflareTunnelPorts(cfg.LoopbackPort, cfg.MetricsPort, listeners); err != nil {
		return err
	}
	if cfg.AuthMethod == "named" {
		if strings.ContainsAny(cfg.TunnelName, "\r\n") {
			return fmt.Errorf("tunnel name must not contain newline characters")
		}
		if cfg.TunnelName == "" || len(cfg.CustomIngress) == 0 {
			return fmt.Errorf("named tunnels require a tunnel name and explicit custom_ingress routes")
		}
		return validateCustomIngress(cfg.CustomIngress)
	}
	return nil
}

func cloudflareRuntimeArgs(cfg CloudflareTunnelConfig, args []string) []string {
	if len(args) == 0 {
		return args
	}
	result := []string{args[0], "--no-autoupdate"}
	if cfg.LogLevel != "" {
		result = append(result, "--loglevel", cfg.LogLevel)
	}
	// Pin zero to loopback too; cloudflared's Docker default otherwise binds all interfaces.
	result = append(result, "--metrics", fmt.Sprintf("localhost:%d", cfg.MetricsPort))
	return append(result, args[1:]...)
}

func stopCloudflareDockerBeforeCreate(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	result := reconcileCloudflareLocked(cfg, logger, false)
	if !cloudflareTunnelToolResultOK(result) {
		return result
	}
	if cloudflareSnapshot().Mode == "docker" {
		return stopDockerTunnel(cfg, logger)
	}
	return okJSON("No previous Docker tunnel")
}

func cloudflareQuickDockerNetwork(cfg CloudflareTunnelConfig) (string, map[string]interface{}, error) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		hostname, err := os.Hostname()
		if err != nil || len(hostname) < 12 || len(hostname) > 64 || strings.Trim(hostname, "0123456789abcdef") != "" {
			return "", nil, fmt.Errorf("cannot identify this container for quick publication; use native mode")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		raw, code, err := DockerRequestContext(ctx, DockerConfig{Host: cfg.DockerHost}, "GET", "/containers/"+url.PathEscape(hostname)+"/json", "")
		var parent struct {
			ID    string `json:"Id"`
			State struct {
				Running bool `json:"Running"`
			} `json:"State"`
		}
		if err != nil || code != 200 || json.Unmarshal(raw, &parent) != nil || len(parent.ID) != 64 || strings.Trim(parent.ID, "0123456789abcdef") != "" || !strings.HasPrefix(parent.ID, hostname) || !parent.State.Running {
			return "", nil, fmt.Errorf("cannot verify parent container for quick publication; use native mode")
		}
		return "127.0.0.1", map[string]interface{}{"NetworkMode": "container:" + parent.ID, "RestartPolicy": map[string]string{"Name": "no"}}, nil
	}
	endpoint := dockerutil.NormalizeHost(cfg.DockerHost)
	if !strings.HasPrefix(endpoint, "unix://") && !strings.HasPrefix(endpoint, "npipe://") {
		return "", nil, fmt.Errorf("quick publication needs a local Docker socket or verified parent container; use native mode for a remote Docker daemon")
	}
	return "host.docker.internal", map[string]interface{}{"ExtraHosts": []string{"host.docker.internal:host-gateway"}, "RestartPolicy": map[string]string{"Name": "no"}}, nil
}

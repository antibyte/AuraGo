package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"

	"aurago/internal/config"
)

// ollamaNetwork proves same-daemon placement before relying on container DNS.
// A Docker proxy's address is never treated as the Ollama host.
func ollamaNetwork(cfg *config.Config, contexts ...context.Context) (string, error) {
	ctx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		ctx = contexts[0]
	}
	host := strings.TrimSpace(cfg.Docker.Host)
	if host == "" {
		host = strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	}
	if !cfg.Runtime.IsDocker {
		if u, err := url.Parse(host); err == nil && (u.Scheme == "tcp" || u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "ssh") {
			ip := net.ParseIP(u.Hostname())
			if u.Scheme == "ssh" || u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
				return "", fmt.Errorf("managed Ollama requires a local Docker Engine; configure an external Ollama URL for a remote Engine")
			}
		}
		return "", nil
	}
	self, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("resolve AuraGo container identity: %w", err)
	}
	data, code, err := dockerRequest(DockerConfig{Host: host, Context: ctx}, "GET", "/containers/"+url.PathEscape(self)+"/json", "")
	if err != nil {
		return "", fmt.Errorf("inspect AuraGo network: %w", err)
	}
	if code != 200 {
		return "", fmt.Errorf("cannot prove AuraGo placement on this Docker Engine (HTTP %d); use an external Ollama URL", code)
	}
	return ollamaNetworkFromInspect(ctx, DockerConfig{Host: host, Context: ctx}, self, data)
}

func ollamaNetworkFromInspect(ctx context.Context, docker DockerConfig, self string, data []byte) (string, error) {
	var info struct {
		ID              string                                        `json:"Id"`
		HostConfig      struct{ NetworkMode string }                  `json:"HostConfig"`
		NetworkSettings struct{ Networks map[string]json.RawMessage } `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("decode AuraGo placement: %w", err)
	}
	// Custom hostnames and shared/host network namespaces cannot prove identity
	// from a name. Refuse them rather than attaching to an unrelated daemon.
	if len(self) < 12 || !strings.HasPrefix(info.ID, self) || info.HostConfig.NetworkMode == "host" || strings.HasPrefix(info.HostConfig.NetworkMode, "container:") {
		return "", fmt.Errorf("managed Ollama requires a proven bridge container identity; custom hostname, host/shared network or remote Engine is unsupported; configure external Ollama")
	}
	names := make([]string, 0, len(info.NetworkSettings.Networks))
	for name := range info.NetworkSettings.Networks {
		if name != "bridge" && !strings.Contains(strings.ReplaceAll(name, "_", "-"), "docker-control") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		data, code, err := dockerRequest(docker, "GET", "/networks/"+url.PathEscape(name), "")
		var network struct {
			Internal bool
			Driver   string
		}
		if err == nil && code == 200 && json.Unmarshal(data, &network) == nil && !network.Internal && network.Driver == "bridge" {
			return name, nil
		}
	}
	return "", fmt.Errorf("managed Ollama needs a shared application bridge with download egress; docker-control and internal-only networks are excluded")
}

func ollamaPlacePayload(payload []byte, network string) ([]byte, error) {
	if network == "" {
		return payload, nil
	}
	var body ollamaCreateBody
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	body.HostConfig.NetworkMode = network
	body.HostConfig.PortBindings = nil
	return json.Marshal(body)
}

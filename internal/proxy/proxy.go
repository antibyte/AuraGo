package proxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const (
	containerName = "aurago-security-proxy"
	imageName     = "aurago-proxy:latest"
)

// Manager manages the Caddy reverse proxy Docker container lifecycle.
type Manager struct {
	cfg    *config.Config
	logger *slog.Logger
}

// NewManager creates a new proxy manager.
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	return &Manager{cfg: cfg, logger: logger}
}

// UpdateConfig updates the config reference (e.g. after hot-reload).
func (m *Manager) UpdateConfig(cfg *config.Config) {
	m.cfg = cfg
}

// dockerCfg returns the Docker config for API calls.
func (m *Manager) dockerCfg() tools.DockerConfig {
	host := m.cfg.SecurityProxy.DockerHost
	if host == "" {
		host = m.cfg.Docker.Host
	}
	return tools.DockerConfig{Host: host}
}

// dataDir returns the proxy data directory, creating it if needed.
func (m *Manager) dataDir() string {
	dir := filepath.Join(m.cfg.Directories.DataDir, "proxy")
	os.MkdirAll(dir, 0o750)
	return dir
}

// upstreamAddr returns the address Caddy should proxy to for the AuraGo backend.
func (m *Manager) upstreamAddr() string {
	port := m.cfg.Server.Port
	if port <= 0 {
		port = 8088
	}
	// When running inside Docker (compose), use the service name
	if isRunningInDocker() {
		return fmt.Sprintf("aurago:%d", port)
	}
	// Otherwise use host.docker.internal on Mac/Windows, 172.17.0.1 on Linux
	if runtime.GOOS == "linux" {
		return fmt.Sprintf("172.17.0.1:%d", port)
	}
	return fmt.Sprintf("host.docker.internal:%d", port)
}

func isRunningInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// Start builds the image (if needed), generates the Caddyfile, and starts the container.
func (m *Manager) Start() error {
	cfg := m.dockerCfg()

	// Verify Docker is available
	if err := tools.DockerPing(cfg.Host); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	proxyCfg := m.cfg.SecurityProxy

	// Ensure data directories
	dataDir := m.dataDir()
	for _, sub := range []string{"caddy_data", "caddy_config"} {
		os.MkdirAll(filepath.Join(dataDir, sub), 0o750)
	}

	// Generate and write Caddyfile
	caddyfile := GenerateCaddyfile(m.cfg, m.upstreamAddr())
	caddyfilePath := filepath.Join(dataDir, "Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0o644); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}
	m.logger.Info("Security proxy Caddyfile written", "path", caddyfilePath)

	// Build or pull image
	if err := m.ensureImage(); err != nil {
		return fmt.Errorf("ensure proxy image: %w", err)
	}

	// Stop existing container if any
	m.stopAndRemove()

	// Create container
	absCaddyfile, _ := filepath.Abs(caddyfilePath)
	absCaddyData, _ := filepath.Abs(filepath.Join(dataDir, "caddy_data"))
	absCaddyConfig, _ := filepath.Abs(filepath.Join(dataDir, "caddy_config"))

	payload := map[string]interface{}{
		"Image": imageName,
		"ExposedPorts": map[string]interface{}{
			"443/tcp": struct{}{},
			"80/tcp":  struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"Binds": []string{
				absCaddyfile + ":/etc/caddy/Caddyfile",
				absCaddyData + ":/data",
				absCaddyConfig + ":/config",
			},
			"PortBindings": map[string]interface{}{
				"443/tcp": []map[string]string{
					{"HostIp": "0.0.0.0", "HostPort": fmt.Sprintf("%d", proxyCfg.HTTPSPort)},
				},
				"80/tcp": []map[string]string{
					{"HostIp": "0.0.0.0", "HostPort": fmt.Sprintf("%d", proxyCfg.HTTPPort)},
				},
			},
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
			"ExtraHosts":    []string{"host.docker.internal:host-gateway"},
		},
	}

	body, _ := json.Marshal(payload)
	data, code, err := tools.DockerRequest(cfg, "POST", "/containers/create?name="+url.QueryEscape(containerName), string(body))
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	if code != 201 {
		return fmt.Errorf("create container: HTTP %d: %s", code, string(data))
	}

	// Start container
	_, startCode, startErr := tools.DockerRequest(cfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/start", "")
	if startErr != nil {
		return fmt.Errorf("start container: %w", startErr)
	}
	if startCode != 204 && startCode != 304 {
		return fmt.Errorf("start container: HTTP %d", startCode)
	}

	m.logger.Info("Security proxy started",
		"container", containerName,
		"https_port", proxyCfg.HTTPSPort,
		"http_port", proxyCfg.HTTPPort,
		"domain", proxyCfg.Domain)
	return nil
}

// Stop stops the running proxy container without removing it.
func (m *Manager) Stop() error {
	cfg := m.dockerCfg()
	_, code, err := tools.DockerRequest(cfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/stop?t=10", "")
	if err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	if code != 204 && code != 304 && code != 404 {
		return fmt.Errorf("stop container: HTTP %d", code)
	}
	m.logger.Info("Security proxy stopped")
	return nil
}

// Destroy stops and removes the container.
func (m *Manager) Destroy() error {
	m.stopAndRemove()
	m.logger.Info("Security proxy destroyed")
	return nil
}

func (m *Manager) stopAndRemove() {
	cfg := m.dockerCfg()
	tools.DockerRequest(cfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/stop?t=5", "")
	tools.DockerRequest(cfg, "DELETE", "/containers/"+url.QueryEscape(containerName)+"?force=true&v=true", "")
}

// Reload writes a new Caddyfile and reloads Caddy's config via the admin API.
func (m *Manager) Reload() error {
	dataDir := m.dataDir()

	caddyfile := GenerateCaddyfile(m.cfg, m.upstreamAddr())
	caddyfilePath := filepath.Join(dataDir, "Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0o644); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}

	// Exec caddy reload inside the container
	cfg := m.dockerCfg()
	execPayload := map[string]interface{}{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          []string{"caddy", "reload", "--config", "/etc/caddy/Caddyfile"},
	}
	body, _ := json.Marshal(execPayload)
	data, code, err := tools.DockerRequest(cfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/exec", string(body))
	if err != nil {
		return fmt.Errorf("exec create: %w", err)
	}
	if code != 201 {
		return fmt.Errorf("exec create: HTTP %d: %s", code, string(data))
	}

	var execResp struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &execResp); err != nil {
		return fmt.Errorf("parse exec response: %w", err)
	}

	startPayload, _ := json.Marshal(map[string]bool{"Detach": true})
	_, _, _ = tools.DockerRequest(cfg, "POST", "/exec/"+execResp.ID+"/start", string(startPayload))
	m.logger.Info("Security proxy configuration reloaded")
	return nil
}

// Status returns the container status.
type ContainerStatus struct {
	Running bool   `json:"running"`
	State   string `json:"state"`  // "running", "exited", "paused", etc.
	Status  string `json:"status"` // human-readable Docker status
	Image   string `json:"image"`
}

func (m *Manager) Status() (*ContainerStatus, error) {
	cfg := m.dockerCfg()
	data, code, err := tools.DockerRequest(cfg, "GET", "/containers/"+url.QueryEscape(containerName)+"/json", "")
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}
	if code == 404 {
		return &ContainerStatus{Running: false, State: "not_found"}, nil
	}
	if code != 200 {
		return nil, fmt.Errorf("inspect container: HTTP %d", code)
	}

	var inspect struct {
		State struct {
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
		} `json:"State"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &inspect); err != nil {
		return nil, fmt.Errorf("parse inspect: %w", err)
	}

	return &ContainerStatus{
		Running: inspect.State.Running,
		State:   inspect.State.Status,
		Status:  inspect.State.Status,
		Image:   inspect.Config.Image,
	}, nil
}

// Logs returns the last N lines of container logs.
func (m *Manager) Logs(tail int) (string, error) {
	if tail <= 0 {
		tail = 100
	}
	cfg := m.dockerCfg()
	endpoint := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%d&timestamps=true", url.QueryEscape(containerName), tail)
	data, code, err := tools.DockerRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}
	if code != 200 {
		return "", fmt.Errorf("get logs: HTTP %d", code)
	}
	// Strip Docker log header bytes (8 bytes per frame)
	return stripDockerLogHeaders(data), nil
}

// stripDockerLogHeaders removes the 8-byte Docker multiplexed log frame headers.
func stripDockerLogHeaders(raw []byte) string {
	var sb strings.Builder
	for len(raw) > 0 {
		if len(raw) < 8 {
			sb.Write(raw)
			break
		}
		size := int(raw[4])<<24 | int(raw[5])<<16 | int(raw[6])<<8 | int(raw[7])
		raw = raw[8:]
		if size > len(raw) {
			size = len(raw)
		}
		sb.Write(raw[:size])
		raw = raw[size:]
	}
	return sb.String()
}

// ensureImage checks if the proxy image exists; if not, pulls caddy:latest.
// For V1, we use the official Caddy image. Rate limiting uses Caddy's built-in
// reverse_proxy load balancing. Custom plugins (xcaddy) can be added in V2.
func (m *Manager) ensureImage() error {
	cfg := m.dockerCfg()
	_, code, _ := tools.DockerRequest(cfg, "GET", "/images/"+url.QueryEscape(imageName)+"/json", "")
	if code == 200 {
		return nil // image already exists
	}

	m.logger.Info("Pulling Caddy image for security proxy...")
	// Pull official caddy image and tag it as our image
	_, pullCode, pullErr := tools.DockerRequest(cfg, "POST", "/images/create?fromImage=caddy&tag=latest", "")
	if pullErr != nil {
		return fmt.Errorf("pull caddy image: %w", pullErr)
	}
	if pullCode != 200 {
		return fmt.Errorf("pull caddy image: HTTP %d", pullCode)
	}

	// Tag as our image name
	_, tagCode, tagErr := tools.DockerRequest(cfg, "POST", "/images/caddy:latest/tag?repo=aurago-proxy&tag=latest", "")
	if tagErr != nil {
		return fmt.Errorf("tag image: %w", tagErr)
	}
	if tagCode != 201 {
		return fmt.Errorf("tag image: HTTP %d", tagCode)
	}

	m.logger.Info("Security proxy image ready", "image", imageName)
	return nil
}

package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const containerName = "aurago-security-proxy"

// createTimeout bounds the container create like the 60 s client timeout of
// tools.DockerRequest, which created the container before.
const createTimeout = 60 * time.Second

// engine is the Docker Engine API surface the manager uses.
type engine struct {
	ping           func(host string) error
	request        func(cfg tools.DockerConfig, method, endpoint, body string) ([]byte, int, error)
	requestContext func(ctx context.Context, cfg tools.DockerConfig, method, endpoint, body string) ([]byte, int, error)
	// createTrusted posts a /containers/create body whose HostConfig.Binds
	// listed exactly in trusted skip the create bind policy; every other bind
	// is still checked.
	createTrusted func(ctx context.Context, cfg tools.DockerConfig, endpoint, body string, trusted []string) ([]byte, int, error)
	build         func(ctx context.Context, cfg tools.DockerConfig, image, dockerfileName string, dockerfile []byte, buildArgs map[string]string, logger *slog.Logger) error
	// pull always pulls the image and fails on an error event in the Engine's
	// progress stream.
	pull func(ctx context.Context, cfg tools.DockerConfig, image string, logger *slog.Logger) error
}

var dockerEngine = engine{
	ping:           tools.DockerPing,
	request:        tools.DockerRequest,
	requestContext: tools.DockerRequestContext,
	createTrusted:  tools.DockerCreateRequestContextWithTrustedBinds,
	build:          tools.BuildImageWait,
	pull:           tools.PullImageForce,
}

// Manager manages the Caddy reverse proxy Docker container lifecycle.
type Manager struct {
	mu     sync.RWMutex
	cfg    *config.Config
	logger *slog.Logger

	// lifecycle serializes Start, Reload, Stop and Destroy. A first image
	// build (up to rateLimitBuildTimeout) or pull holds it, and the others
	// wait; Status and Logs do not take it.
	lifecycle sync.Mutex
	engine    engine
	// settle is how long Start waits before checking that Caddy kept running.
	settle time.Duration
	// inDocker and selfIDs replace the container probes in tests.
	inDocker func() bool
	selfIDs  func() []string
	// native replaces nativePlacement in tests, e.g. with host paths under
	// /root that a test can neither write nor produce on Windows.
	native func(cfg *config.Config, proxyDir string) placement
}

// NewManager creates a new proxy manager.
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	return &Manager{cfg: cfg, logger: logger, engine: dockerEngine, settle: 2 * time.Second}
}

// UpdateConfig replaces the config the next Start or Reload uses. The server
// calls it whenever it publishes a new config snapshot.
func (m *Manager) UpdateConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}

// Config returns the config the manager currently uses.
func (m *Manager) Config() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) log() *slog.Logger {
	if m.logger != nil {
		return m.logger
	}
	return slog.Default()
}

// dockerConfigFor returns the Docker config for API calls.
func dockerConfigFor(cfg *config.Config) tools.DockerConfig {
	host := cfg.SecurityProxy.DockerHost
	if host == "" {
		host = cfg.Docker.Host
	}
	return tools.DockerConfig{Host: host}
}

// dockerCfg returns the Docker config of the current config.
func (m *Manager) dockerCfg() tools.DockerConfig {
	return dockerConfigFor(m.Config())
}

// dataDir returns the proxy data directory, creating it if needed.
func dataDir(cfg *config.Config) string {
	dir := filepath.Join(cfg.Directories.DataDir, "proxy")
	os.MkdirAll(dir, 0o750)
	return dir
}

// runsInDocker reports whether AuraGo itself runs in a container.
func (m *Manager) runsInDocker() bool {
	if m.inDocker != nil {
		return m.inDocker()
	}
	return isRunningInDocker()
}

func isRunningInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// writeCaddyfile rewrites the Caddyfile in place with mode 0600: it can hold
// the Basic Auth hash. It must keep the inode, because a native install
// bind-mounts the single file and a renamed replacement would stay invisible
// to Caddy.
func writeCaddyfile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	// Tighten files written by earlier versions with 0644 before they receive
	// the hash.
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if err := f.Truncate(0); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Start generates the Caddyfile, builds or pulls the image (if needed), then
// writes the Caddyfile and recreates the container.
func (m *Manager) Start() error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	return m.startLocked(m.Config())
}

func (m *Manager) startLocked(cfg *config.Config) error {
	dockerCfg := dockerConfigFor(cfg)

	// Verify Docker is available
	if err := m.engine.ping(dockerCfg.Host); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	proxyCfg := cfg.SecurityProxy

	// Ensure data directories
	dir := dataDir(cfg)
	for _, sub := range []string{"caddy_data", "caddy_config"} {
		os.MkdirAll(filepath.Join(dir, sub), 0o750)
	}

	place, err := m.resolvePlacement(cfg, dir)
	if err != nil {
		return err
	}

	// Generate the Caddyfile in memory first, so unusable credentials fail
	// before any image build or pull.
	caddyfile, err := GenerateCaddyfile(cfg, place.upstream)
	if err != nil {
		return err
	}

	// Build or pull the image before the Caddyfile is written: a failed build
	// or pull must leave the running container and the file it loads as they
	// are. Reload's recreate path comes through here too.
	image, err := m.ensureImage(cfg)
	if err != nil {
		return fmt.Errorf("ensure proxy image: %w", err)
	}

	caddyfilePath := filepath.Join(dir, "Caddyfile")
	if err := writeCaddyfile(caddyfilePath, []byte(caddyfile)); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}
	m.log().Info("Security proxy Caddyfile written", "path", caddyfilePath)

	// Stop existing container if any
	m.stopAndRemove(dockerCfg)

	// Create container. The binds of a native placement are the proxy's own
	// directory, so they are trusted: an install under /root, /mnt, /etc or
	// /hostfs (install.sh run as root uses /root/aurago) would fail the create
	// bind policy otherwise. Nothing else is trusted; the Docker placement uses
	// Mounts and has no binds.
	payload := securityProxyCreatePayload(image, place, proxyCfg.HTTPSPort, proxyCfg.HTTPPort)
	body, _ := json.Marshal(payload)
	createCtx, cancelCreate := context.WithTimeout(context.Background(), createTimeout)
	data, code, err := m.engine.createTrusted(createCtx, dockerCfg, "/containers/create?name="+url.QueryEscape(containerName), string(body), place.binds)
	cancelCreate()
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	if code != 201 {
		return fmt.Errorf("create container: HTTP %d: %s", code, string(data))
	}

	// Start container
	_, startCode, startErr := m.engine.request(dockerCfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/start", "")
	if startErr != nil {
		return fmt.Errorf("start container: %w", startErr)
	}
	if startCode != 204 && startCode != 304 {
		return fmt.Errorf("start container: HTTP %d", startCode)
	}

	if err := m.verifyRunning(dockerCfg); err != nil {
		return err
	}

	m.log().Info("Security proxy started",
		"container", containerName,
		"image", image,
		"https_port", proxyCfg.HTTPSPort,
		"http_port", proxyCfg.HTTPPort,
		"domain", proxyCfg.Domain)
	return nil
}

// securityProxyCreatePayload is the Docker create body for the Caddy proxy.
// A native install bind-mounts host paths. When AuraGo runs in Docker the
// proxy mounts AuraGo's own data volume or bind source and joins AuraGo's
// network, because AuraGo's paths and container name mean nothing outside it.
func securityProxyCreatePayload(image string, place placement, httpsPort, httpPort int) map[string]interface{} {
	hostConfig := map[string]interface{}{
		"PortBindings": map[string]interface{}{
			"443/tcp": []map[string]string{
				{"HostIp": "0.0.0.0", "HostPort": fmt.Sprintf("%d", httpsPort)},
			},
			"80/tcp": []map[string]string{
				{"HostIp": "0.0.0.0", "HostPort": fmt.Sprintf("%d", httpPort)},
			},
		},
		"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		"ExtraHosts":    []string{"host.docker.internal:host-gateway"},
	}
	payload := map[string]interface{}{
		"Image": image,
		"ExposedPorts": map[string]interface{}{
			"443/tcp": struct{}{},
			"80/tcp":  struct{}{},
		},
		"HostConfig": hostConfig,
	}
	if place.binds != nil {
		hostConfig["Binds"] = place.binds
	}
	if place.mounts != nil {
		hostConfig["Mounts"] = place.mounts
	}
	if place.network != "" {
		hostConfig["NetworkMode"] = place.network
		payload["NetworkingConfig"] = map[string]interface{}{
			"EndpointsConfig": map[string]interface{}{place.network: map[string]interface{}{}},
		}
	}
	return payload
}

// containerState is the part of the proxy container inspection Start and
// Reload use.
type containerState struct {
	RestartCount int `json:"RestartCount"`
	State        struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Restarting bool   `json:"Restarting"`
	} `json:"State"`
	Config struct {
		Image string `json:"Image"`
	} `json:"Config"`
}

func (m *Manager) inspectContainer(dockerCfg tools.DockerConfig) (containerState, bool, error) {
	data, code, err := m.engine.request(dockerCfg, "GET", "/containers/"+url.QueryEscape(containerName)+"/json", "")
	if err != nil {
		return containerState{}, false, fmt.Errorf("inspect container: %w", err)
	}
	if code == 404 {
		return containerState{}, false, nil
	}
	if code != 200 {
		return containerState{}, false, fmt.Errorf("inspect container: HTTP %d", code)
	}
	var state containerState
	if err := json.Unmarshal(data, &state); err != nil {
		return containerState{}, false, fmt.Errorf("parse inspect: %w", err)
	}
	return state, true, nil
}

// verifyRunning reports a Caddy that exits right after the start, e.g. on a
// Caddyfile it cannot load. The restart policy would otherwise hide the
// failure behind a restart loop.
func (m *Manager) verifyRunning(dockerCfg tools.DockerConfig) error {
	if m.settle > 0 {
		time.Sleep(m.settle)
	}
	state, found, err := m.inspectContainer(dockerCfg)
	if err != nil {
		m.log().Warn("Security proxy started but its state could not be checked", "error", err)
		return nil
	}
	if found && state.State.Running && !state.State.Restarting && state.RestartCount == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrCaddyExited, m.recentCaddyError(dockerCfg))
}

// recentCaddyError returns the last error Caddy logged.
func (m *Manager) recentCaddyError(dockerCfg tools.DockerConfig) string {
	data, code, err := m.engine.request(dockerCfg, "GET", "/containers/"+url.QueryEscape(containerName)+"/logs?stdout=true&stderr=true&tail=20", "")
	if err != nil || code != 200 {
		return "see the proxy logs"
	}
	if reason := lastCaddyError(stripDockerLogHeaders(data)); reason != "" {
		return reason
	}
	return "see the proxy logs"
}

// lastCaddyError picks the last error line of Caddy output (JSON log lines or
// plain "Error: ..." lines).
func lastCaddyError(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry struct {
			Level string `json:"level"`
			Msg   string `json:"msg"`
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil && entry.Level != "" {
			if entry.Level != "error" && entry.Level != "fatal" && entry.Level != "panic" {
				continue
			}
			line = strings.TrimSpace(strings.TrimSuffix(entry.Msg+": "+entry.Error, ": "))
		} else if !strings.Contains(strings.ToLower(line), "error") {
			continue
		}
		if len(line) > 400 {
			line = line[:400] + "..."
		}
		return line
	}
	return ""
}

// Stop stops the running proxy container without removing it.
func (m *Manager) Stop() error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	cfg := m.dockerCfg()
	_, code, err := m.engine.request(cfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/stop?t=10", "")
	if err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	if code != 204 && code != 304 && code != 404 {
		return fmt.Errorf("stop container: HTTP %d", code)
	}
	m.log().Info("Security proxy stopped")
	return nil
}

// Destroy stops and removes the container.
func (m *Manager) Destroy() error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.stopAndRemove(m.dockerCfg())
	m.log().Info("Security proxy destroyed")
	return nil
}

func (m *Manager) stopAndRemove(cfg tools.DockerConfig) {
	m.engine.request(cfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/stop?t=5", "")
	m.engine.request(cfg, "DELETE", "/containers/"+url.QueryEscape(containerName)+"?force=true&v=true", "")
}

// Reload writes a new Caddyfile and reloads Caddy's config via the admin API.
// A container whose image no longer fits the config (rate limiting toggled)
// is recreated instead. When Caddy rejects the new Caddyfile it keeps the old
// configuration, and the previous file is restored so a container restart
// loads the configuration that is actually running.
func (m *Manager) Reload() error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	cfg := m.Config()
	dockerCfg := dockerConfigFor(cfg)

	state, found, err := m.inspectContainer(dockerCfg)
	if err != nil {
		return err
	}
	if !found || !state.State.Running {
		return ErrNotRunning
	}
	if want := proxyImage(cfg); state.Config.Image != want {
		m.log().Info("Security proxy image changed; recreating the container", "from", state.Config.Image, "to", want)
		return m.startLocked(cfg)
	}

	dir := dataDir(cfg)
	place, err := m.resolvePlacement(cfg, dir)
	if err != nil {
		return err
	}
	caddyfile, err := GenerateCaddyfile(cfg, place.upstream)
	if err != nil {
		return err
	}
	caddyfilePath := filepath.Join(dir, "Caddyfile")
	previous, previousErr := os.ReadFile(caddyfilePath)
	restore := func() {
		if previousErr != nil {
			return
		}
		if err := writeCaddyfile(caddyfilePath, previous); err != nil {
			m.log().Warn("Failed to restore the previous security proxy Caddyfile", "error", err)
		}
	}
	if err := writeCaddyfile(caddyfilePath, []byte(caddyfile)); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}

	// Exec caddy reload inside the container and wait for its exit code.
	exitCode, output, started, err := m.execReload(dockerCfg)
	if err != nil {
		if !started {
			restore()
		}
		return err
	}
	if exitCode != 0 {
		restore()
		reason := lastCaddyError(output)
		if reason == "" {
			reason = fmt.Sprintf("caddy reload exited with code %d", exitCode)
		}
		return fmt.Errorf("%w: %s", ErrConfigRejected, reason)
	}
	m.log().Info("Security proxy configuration reloaded")
	return nil
}

// execReload runs `caddy reload` attached. started reports whether the
// command may have run; after that point a transport error leaves its outcome
// unknown.
func (m *Manager) execReload(dockerCfg tools.DockerConfig) (exitCode int, output string, started bool, err error) {
	execPayload := map[string]interface{}{
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          false,
		"Cmd":          []string{"caddy", "reload", "--config", "/etc/caddy/Caddyfile"},
	}
	body, _ := json.Marshal(execPayload)
	data, code, err := m.engine.request(dockerCfg, "POST", "/containers/"+url.QueryEscape(containerName)+"/exec", string(body))
	if err != nil {
		return -1, "", false, fmt.Errorf("exec create: %w", err)
	}
	if code == 404 || code == 409 {
		return -1, "", false, ErrNotRunning
	}
	if code != 201 {
		return -1, "", false, fmt.Errorf("exec create: HTTP %d: %s", code, string(data))
	}

	var execResp struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &execResp); err != nil || execResp.ID == "" {
		return -1, "", false, errors.New("parse exec response: no exec ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	startPayload, _ := json.Marshal(map[string]bool{"Detach": false, "Tty": false})
	out, startCode, err := m.engine.requestContext(ctx, dockerCfg, "POST", "/exec/"+url.PathEscape(execResp.ID)+"/start", string(startPayload))
	if err != nil {
		return -1, "", true, fmt.Errorf("exec start (outcome unknown, check the proxy logs): %w", err)
	}
	if startCode != 200 {
		return -1, "", false, fmt.Errorf("exec start: HTTP %d", startCode)
	}
	output = stripDockerLogHeaders(out)

	inspect, inspectCode, err := m.engine.requestContext(ctx, dockerCfg, "GET", "/exec/"+url.PathEscape(execResp.ID)+"/json", "")
	if err != nil || inspectCode != 200 {
		return -1, output, true, fmt.Errorf("exec inspect (outcome unknown, check the proxy logs): HTTP %d %v", inspectCode, err)
	}
	var result struct {
		ExitCode int `json:"ExitCode"`
	}
	if err := json.Unmarshal(inspect, &result); err != nil {
		return -1, output, true, fmt.Errorf("parse exec inspect: %w", err)
	}
	return result.ExitCode, output, true, nil
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
	data, code, err := m.engine.request(cfg, "GET", "/containers/"+url.QueryEscape(containerName)+"/json", "")
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
	data, code, err := m.engine.request(cfg, "GET", endpoint, "")
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

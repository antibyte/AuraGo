package tools

import (
	"aurago/internal/security"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CloudflareTunnelConfig holds the merged config for tunnel management.
type CloudflareTunnelConfig struct {
	Enabled        bool
	ReadOnly       bool
	Mode           string // "auto", "docker", "native"
	AutoStart      bool
	AuthMethod     string // "token", "named", "quick"
	TunnelName     string
	AccountID      string
	ExposeWebUI    bool
	ExposeHomepage bool
	CustomIngress  []CloudflareIngress
	MetricsPort    int
	LogLevel       string
	DockerHost     string // inherited from docker.host
	WebUIPort      int    // from server.port (plain HTTP port)
	HomepagePort   int    // from homepage.webserver_port
	DataDir        string // for storing config files
	// HTTPS fields: when HTTPS is enabled AuraGo no longer listens on WebUIPort.
	// The tunnel must connect to the HTTPS endpoint instead.
	HTTPSEnabled bool // from server.https.enabled
	HTTPSPort    int  // from server.https.https_port (default 443)
}

// CloudflareIngress mirrors config.CloudflareIngressRule for the tools package.
type CloudflareIngress struct {
	Hostname string
	Service  string
	Path     string
}

const (
	cfdContainerName = "aurago-cloudflared"
	cfdImageName     = "cloudflare/cloudflared:latest"
	cfdBinaryName    = "cloudflared"
)

// tunnelState tracks the running cloudflared process/container.
var (
	tunnelMu       sync.Mutex
	tunnelPID      int    // native mode: PID in registry
	tunnelMode     string // "docker", "native", "quick", or ""
	tunnelURL      string // quick tunnel: the public URL
	tunnelStarted  time.Time
	tunnelStopping bool
)

// ──────────────────────────────────────────────────────────────────────────
// Lifecycle
// ──────────────────────────────────────────────────────────────────────────

// CloudflareTunnelStart starts the cloudflared tunnel.
func CloudflareTunnelStart(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()

	// Check if already running
	if tunnelMode != "" {
		return errJSON("Tunnel already running (mode=%s). Stop it first.", tunnelMode)
	}

	switch cfg.AuthMethod {
	case "token":
		return startTokenTunnel(cfg, vault, registry, logger)
	case "named":
		return startNamedTunnel(cfg, vault, registry, logger)
	case "quick":
		return startQuickTunnel(cfg, registry, logger, cfg.WebUIPort)
	default:
		return errJSON("Unknown auth_method: %q. Use: token, named, quick", cfg.AuthMethod)
	}
}

// CloudflareTunnelStop stops the running tunnel.
func CloudflareTunnelStop(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()

	if tunnelMode == "" {
		return errJSON("No tunnel is running")
	}

	tunnelStopping = true
	defer func() { tunnelStopping = false }()

	switch tunnelMode {
	case "docker":
		return stopDockerTunnel(cfg, logger)
	case "native", "quick":
		return stopNativeTunnel(cfg, registry, logger)
	default:
		tunnelMode = ""
		return errJSON("Unknown tunnel mode: %s", tunnelMode)
	}
}

// CloudflareTunnelRestart stops and restarts the tunnel.
func CloudflareTunnelRestart(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	stopResult := CloudflareTunnelStop(cfg, registry, logger)
	// Allow a moment for cleanup
	time.Sleep(time.Second)
	startResult := CloudflareTunnelStart(cfg, vault, registry, logger)
	return fmt.Sprintf(`{"stop": %s, "start": %s}`, stopResult, startResult)
}

// CloudflareTunnelStatus returns the current tunnel status.
func CloudflareTunnelStatus(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()

	if tunnelMode == "" {
		out, _ := json.Marshal(map[string]interface{}{
			"status":  "ok",
			"running": false,
			"message": "No tunnel running",
		})
		return string(out)
	}

	result := map[string]interface{}{
		"status":      "ok",
		"running":     true,
		"mode":        tunnelMode,
		"auth_method": cfg.AuthMethod,
		"uptime":      fmt.Sprintf("%.0fs", time.Since(tunnelStarted).Seconds()),
		"started":     tunnelStarted.Format(time.RFC3339),
	}
	if tunnelURL != "" {
		result["tunnel_url"] = tunnelURL
	}

	// Check health
	if tunnelMode == "docker" {
		dockerCfg := DockerConfig{Host: cfg.DockerHost}
		data, code, _ := dockerRequest(dockerCfg, "GET", "/containers/"+cfdContainerName+"/json", "")
		if code == 200 {
			var info map[string]interface{}
			json.Unmarshal(data, &info)
			if state, ok := info["State"].(map[string]interface{}); ok {
				result["container_running"], _ = state["Running"].(bool)
			}
		}
	} else if tunnelPID > 0 {
		if info, ok := registry.Get(tunnelPID); ok {
			result["pid"] = tunnelPID
			result["process_alive"] = info.Alive
		}
	}

	out, _ := json.Marshal(result)
	return string(out)
}

// CloudflareTunnelQuickTunnel starts a temporary quick tunnel for a specific port.
// Quick tunnels use TryCloudflare and don't require any Cloudflare account.
func CloudflareTunnelQuickTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger, port int) string {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()

	if tunnelMode != "" {
		return errJSON("A tunnel is already running (mode=%s). Stop it first to start a quick tunnel.", tunnelMode)
	}

	if port <= 0 {
		port = cfg.WebUIPort
	}

	return startQuickTunnel(cfg, registry, logger, port)
}

// CloudflareTunnelLogs returns recent log output from the tunnel process.
func CloudflareTunnelLogs(registry *ProcessRegistry, logger *slog.Logger) string {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()

	if tunnelMode == "" {
		return errJSON("No tunnel running")
	}

	if tunnelMode == "docker" {
		// Docker logs are not captured in ProcessRegistry; hint user to use docker logs
		out, _ := json.Marshal(map[string]interface{}{
			"status":  "ok",
			"message": "Tunnel running in Docker mode. Use docker tool to view container logs.",
			"mode":    "docker",
		})
		return string(out)
	}

	if tunnelPID <= 0 {
		return errJSON("No process PID tracked")
	}
	info, ok := registry.Get(tunnelPID)
	if !ok {
		return errJSON("Process %d not found in registry", tunnelPID)
	}

	output := info.ReadOutput()
	// Truncate to last 4KB for readability
	if len(output) > 4096 {
		output = output[len(output)-4096:]
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"pid":    tunnelPID,
		"logs":   output,
	})
	return string(out)
}

// CloudflareTunnelListRoutes returns the currently configured ingress rules.
func CloudflareTunnelListRoutes(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	rules := buildIngressRules(cfg)
	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"rules":  rules,
	})
	return string(out)
}

// CloudflareTunnelInstall downloads the cloudflared binary for the current platform.
func CloudflareTunnelInstall(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	binPath := cfdBinaryPath(cfg.DataDir)
	return installCloudflaredBinary(binPath, logger)
}

// ──────────────────────────────────────────────────────────────────────────
// Token Tunnel (Connector Token via CF Dashboard)
// ──────────────────────────────────────────────────────────────────────────

// buildLocalURL returns the URL cloudflared should use to reach AuraGo locally.
//
// When HTTPS is enabled AuraGo no longer listens on the plain HTTP port.
// cloudflared must connect to the HTTPS listener instead. Because the TLS
// certificate is typically self-signed in a home-lab, --no-tls-verify must
// also be appended to the cloudflared args in that case.
//
// Returns (url, extraArgs) where extraArgs may contain "--no-tls-verify".
func buildLocalURL(cfg CloudflareTunnelConfig, host string) (string, []string) {
	if cfg.HTTPSEnabled {
		port := cfg.HTTPSPort
		if port <= 0 {
			port = 443
		}
		return fmt.Sprintf("https://%s:%d", host, port), []string{"--no-tls-verify"}
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.WebUIPort), nil
}

func startTokenTunnel(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	token, err := vault.ReadSecret("cloudflared_token")
	if err != nil || token == "" {
		return errJSON("Cloudflare connector token not found in vault. Store it with key 'cloudflared_token' via the Config UI.")
	}

	mode := resolveMode(cfg)
	logger.Info("[CloudflareTunnel] Starting token tunnel", "mode", mode)

	// Build the local URL.
	// Token tunnel Docker uses NetworkMode=host, so the container IS on the host network;
	// localhost resolves correctly. For native, localhost is also correct.
	localURL, noTLSArgs := buildLocalURL(cfg, "localhost")
	logger.Info("[CloudflareTunnel] Token tunnel local URL", "url", localURL, "https", cfg.HTTPSEnabled)

	// Base command – the --url flag overrides the origin service even for token tunnels.
	tunnelArgs := append([]string{"tunnel", "--url", localURL}, noTLSArgs...)
	tunnelArgs = append(tunnelArgs, "run")

	switch mode {
	case "docker":
		// For HTTPS origins with self-signed certificates, cloudflared must be told to
		// skip TLS verification for the local connection to AuraGo. The --no-tls-verify
		// CLI flag is not reliable for token tunnels when the tunnel config is managed
		// by the Cloudflare dashboard. Writing a config file is the only robust method.
		var extraBinds []string
		if cfg.HTTPSEnabled {
			configContent := "originRequest:\n  noTLSVerify: true\n"
			configDir := filepath.Join(cfg.DataDir, "cloudflared")
			if mkErr := os.MkdirAll(configDir, 0o700); mkErr == nil {
				configPath := filepath.Join(configDir, "origin-config.yml")
				if wErr := os.WriteFile(configPath, []byte(configContent), 0o600); wErr == nil {
					tunnelArgs = append([]string{"tunnel", "--config", "/etc/cloudflared/origin-config.yml", "--url", localURL}, "run")
					extraBinds = []string{configDir + ":/etc/cloudflared:ro"}
					logger.Info("[CloudflareTunnel] Origin config written with noTLSVerify", "path", configPath)
				} else {
					logger.Warn("[CloudflareTunnel] Failed to write origin config", "error", wErr)
				}
			}
		}
		// Pass token as env var to avoid exposure in process listings (ps aux / /proc)
		return startDockerTunnel(cfg, tunnelArgs, []string{"TUNNEL_TOKEN=" + token}, extraBinds, logger)
	case "native":
		// Pass token as env var to avoid exposure in process listings (ps aux / /proc)
		return startNativeTunnel(cfg, registry, tunnelArgs, []string{"TUNNEL_TOKEN=" + token}, logger)
	default:
		return errJSON("Could not determine runtime mode. Docker not available and native binary not found.")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Named Tunnel (credentials.json)
// ──────────────────────────────────────────────────────────────────────────

func startNamedTunnel(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	if cfg.TunnelName == "" {
		return errJSON("tunnel_name is required for named tunnel auth method")
	}

	// Read credentials from vault
	credJSON, err := vault.ReadSecret("cloudflared_credentials")
	if err != nil || credJSON == "" {
		return errJSON("Cloudflare tunnel credentials not found in vault. Store the credentials.json content with key 'cloudflared_credentials'.")
	}

	// Write credentials to a temp file
	credDir := filepath.Join(cfg.DataDir, "cloudflared")
	if err := os.MkdirAll(credDir, 0700); err != nil {
		return errJSON("Failed to create cloudflared config dir: %v", err)
	}
	credPath := filepath.Join(credDir, "credentials.json")
	if err := os.WriteFile(credPath, []byte(credJSON), 0600); err != nil {
		return errJSON("Failed to write credentials file: %v", err)
	}

	// Generate config with ingress rules
	configPath := filepath.Join(credDir, "config.yml")
	if err := writeNamedTunnelConfig(cfg, credPath, configPath); err != nil {
		return errJSON("Failed to write tunnel config: %v", err)
	}

	mode := resolveMode(cfg)
	logger.Info("[CloudflareTunnel] Starting named tunnel", "mode", mode, "tunnel", cfg.TunnelName)

	switch mode {
	case "docker":
		return startDockerNamedTunnel(cfg, credDir, configPath, logger)
	case "native":
		return startNativeTunnel(cfg, registry, []string{
			"tunnel", "--config", configPath, "run", cfg.TunnelName,
		}, nil, logger)
	default:
		return errJSON("Could not determine runtime mode. Docker not available and native binary not found.")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Quick Tunnel (TryCloudflare, no account)
// ──────────────────────────────────────────────────────────────────────────

func startQuickTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger, port int) string {
	mode := resolveMode(cfg)
	logger.Info("[CloudflareTunnel] Starting quick tunnel", "mode", mode, "port", port)

	// For a quick tunnel the caller passes an explicit port; respect HTTPS only
	// when the port matches the HTTPS port (i.e. caller did not override).
	var args []string
	if cfg.HTTPSEnabled && (port <= 0 || port == cfg.HTTPSPort) {
		httpsPort := cfg.HTTPSPort
		if httpsPort <= 0 {
			httpsPort = 443
		}
		args = []string{"tunnel", "--url", fmt.Sprintf("https://localhost:%d", httpsPort), "--no-tls-verify"}
	} else {
		if port <= 0 {
			port = cfg.WebUIPort
		}
		args = []string{"tunnel", "--url", fmt.Sprintf("http://localhost:%d", port)}
	}
	if cfg.MetricsPort > 0 {
		args = append(args, "--metrics", fmt.Sprintf("localhost:%d", cfg.MetricsPort))
	}

	switch mode {
	case "docker":
		return startDockerQuickTunnel(cfg, port, logger)
	case "native":
		return startNativeQuickTunnel(cfg, registry, args, logger)
	default:
		return errJSON("Could not determine runtime mode. Docker not available and native binary not found.")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Docker Backend
// ──────────────────────────────────────────────────────────────────────────

func startDockerTunnel(cfg CloudflareTunnelConfig, cmd []string, containerEnv []string, extraBinds []string, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	// Pull image if not present
	pullImage(dockerCfg, cfdImageName, logger)

	// Remove old container if exists
	removeContainer(dockerCfg, cfdContainerName)

	hostCfg := map[string]interface{}{
		"NetworkMode":   "host",
		"RestartPolicy": map[string]string{"Name": "unless-stopped"},
	}
	if len(extraBinds) > 0 {
		hostCfg["Binds"] = extraBinds
	}

	payload := map[string]interface{}{
		"Image":      cfdImageName,
		"Cmd":        cmd,
		"HostConfig": hostCfg,
	}
	if len(containerEnv) > 0 {
		payload["Env"] = containerEnv
	}
	if cfg.MetricsPort > 0 {
		payload["Cmd"] = append(cmd, "--metrics", fmt.Sprintf("localhost:%d", cfg.MetricsPort))
	}

	return createAndStartContainer(dockerCfg, cfdContainerName, payload, logger, "token")
}

func startDockerNamedTunnel(cfg CloudflareTunnelConfig, credDir, configPath string, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	pullImage(dockerCfg, cfdImageName, logger)
	removeContainer(dockerCfg, cfdContainerName)

	payload := map[string]interface{}{
		"Image": cfdImageName,
		"Cmd":   []string{"tunnel", "--config", "/etc/cloudflared/config.yml", "run", cfg.TunnelName},
		"HostConfig": map[string]interface{}{
			"NetworkMode":   "host",
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
			"Binds": []string{
				credDir + ":/etc/cloudflared:ro",
			},
		},
	}

	return createAndStartContainer(dockerCfg, cfdContainerName, payload, logger, "named")
}

func startDockerQuickTunnel(cfg CloudflareTunnelConfig, port int, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	pullImage(dockerCfg, cfdImageName, logger)
	removeContainer(dockerCfg, cfdContainerName)

	localURL, noTLSArgs := buildLocalURL(cfg, "host.docker.internal")
	cmd := append([]string{"tunnel", "--url", localURL}, noTLSArgs...)

	payload := map[string]interface{}{
		"Image": cfdImageName,
		"Cmd":   cmd,
		"HostConfig": map[string]interface{}{
			"ExtraHosts":    []string{"host.docker.internal:host-gateway"},
			"RestartPolicy": map[string]string{"Name": "no"},
		},
	}

	result := createAndStartContainer(dockerCfg, cfdContainerName, payload, logger, "quick")

	// Try to capture the quick tunnel URL from container logs after a few seconds
	go func() {
		time.Sleep(5 * time.Second)
		url := captureQuickTunnelURLDocker(dockerCfg, logger)
		if url != "" {
			tunnelMu.Lock()
			tunnelURL = url
			tunnelMu.Unlock()
			logger.Info("[CloudflareTunnel] Quick tunnel URL captured", "url", url)
		}
	}()

	return result
}

func createAndStartContainer(dockerCfg DockerConfig, name string, payload map[string]interface{}, logger *slog.Logger, authMethod string) string {
	body, _ := json.Marshal(payload)
	data, code, err := dockerRequest(dockerCfg, "POST", "/containers/create?name="+name, string(body))
	if err != nil {
		return errJSON("Failed to create cloudflared container: %v", err)
	}
	if code != 201 {
		return errJSON("Failed to create cloudflared container: HTTP %d — %s", code, string(data))
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+name+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		return errJSON("Failed to start cloudflared container: code=%d err=%v", startCode, startErr)
	}

	tunnelMode = "docker"
	tunnelStarted = time.Now()
	tunnelPID = 0
	logger.Info("[CloudflareTunnel] Container started", "container", name, "auth", authMethod)

	out, _ := json.Marshal(map[string]interface{}{
		"status":    "ok",
		"message":   "Cloudflare tunnel started (Docker)",
		"container": name,
		"mode":      "docker",
		"auth":      authMethod,
	})
	return string(out)
}

func stopDockerTunnel(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	_, stopCode, _ := dockerRequest(dockerCfg, "POST", "/containers/"+cfdContainerName+"/stop?t=10", "")
	if stopCode != 204 && stopCode != 304 {
		logger.Warn("[CloudflareTunnel] Container stop returned unexpected code", "code", stopCode)
	}

	removeContainer(dockerCfg, cfdContainerName)

	tunnelMode = ""
	tunnelURL = ""
	tunnelPID = 0

	// Clean up credential file written for named tunnel auth
	credPath := filepath.Join(cfg.DataDir, "cloudflared", "credentials.json")
	if _, err := os.Stat(credPath); err == nil {
		if err := os.Remove(credPath); err != nil {
			logger.Warn("[CloudflareTunnel] Failed to remove credential file", "path", credPath, "error", err)
		} else {
			logger.Info("[CloudflareTunnel] Credential file removed", "path", credPath)
		}
	}

	logger.Info("[CloudflareTunnel] Docker tunnel stopped")

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Cloudflare tunnel stopped",
	})
	return string(out)
}

// ──────────────────────────────────────────────────────────────────────────
// Native Binary Backend
// ──────────────────────────────────────────────────────────────────────────

func startNativeTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, args []string, extraEnv []string, logger *slog.Logger) string {
	binPath := findCloudflaredBinary(cfg.DataDir)
	if binPath == "" {
		// Try auto-install
		logger.Info("[CloudflareTunnel] Binary not found, attempting auto-install")
		installResult := installCloudflaredBinary(cfdBinaryPath(cfg.DataDir), logger)
		var ir map[string]interface{}
		if json.Unmarshal([]byte(installResult), &ir) == nil {
			if s, _ := ir["status"].(string); s == "error" {
				return installResult
			}
		}
		binPath = findCloudflaredBinary(cfg.DataDir)
		if binPath == "" {
			return errJSON("cloudflared binary not found after install attempt")
		}
	}

	if cfg.LogLevel != "" {
		args = append(args, "--loglevel", cfg.LogLevel)
	}
	if cfg.MetricsPort > 0 {
		args = append(args, "--metrics", fmt.Sprintf("localhost:%d", cfg.MetricsPort))
	}

	cmd := exec.Command(binPath, args...)
	info := &ProcessInfo{
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
	}
	cmd.Stdout = info
	cmd.Stderr = info
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	if err := cmd.Start(); err != nil {
		return errJSON("Failed to start cloudflared: %v", err)
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
	registry.Register(info)

	tunnelMode = "native"
	tunnelPID = cmd.Process.Pid
	tunnelStarted = time.Now()
	logger.Info("[CloudflareTunnel] Native process started", "pid", cmd.Process.Pid)

	// Wait for process exit in background
	go func() {
		_ = cmd.Wait()
		info.mu.Lock()
		info.Alive = false
		info.mu.Unlock()

		tunnelMu.Lock()
		if !tunnelStopping && tunnelPID == cmd.Process.Pid {
			tunnelMode = ""
			tunnelPID = 0
			tunnelURL = ""
		}
		tunnelMu.Unlock()
		logger.Info("[CloudflareTunnel] Native process exited", "pid", cmd.Process.Pid)
	}()

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Cloudflare tunnel started (native)",
		"pid":     cmd.Process.Pid,
		"mode":    "native",
	})
	return string(out)
}

func startNativeQuickTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, args []string, logger *slog.Logger) string {
	result := startNativeTunnel(cfg, registry, args, nil, logger)

	// Parse to check success
	var r map[string]interface{}
	if json.Unmarshal([]byte(result), &r) == nil && r["status"] == "ok" {
		// Try to capture quick tunnel URL from process output
		go func() {
			pid := tunnelPID
			if pid <= 0 {
				return
			}
			info, ok := registry.Get(pid)
			if !ok {
				return
			}
			// Poll output for the URL (appears within ~5 seconds)
			for i := 0; i < 20; i++ {
				time.Sleep(500 * time.Millisecond)
				output := info.ReadOutput()
				if url := extractQuickTunnelURL(output); url != "" {
					tunnelMu.Lock()
					tunnelURL = url
					tunnelMu.Unlock()
					logger.Info("[CloudflareTunnel] Quick tunnel URL captured", "url", url)
					return
				}
			}
			logger.Warn("[CloudflareTunnel] Could not capture quick tunnel URL within timeout")
		}()
	}

	return result
}

func stopNativeTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
	if tunnelPID > 0 {
		if err := registry.Terminate(tunnelPID); err != nil {
			logger.Warn("[CloudflareTunnel] Failed to terminate process", "pid", tunnelPID, "error", err)
		}
	}

	tunnelMode = ""
	tunnelURL = ""
	tunnelPID = 0

	// Clean up credential file written for named tunnel auth
	credPath := filepath.Join(cfg.DataDir, "cloudflared", "credentials.json")
	if _, err := os.Stat(credPath); err == nil {
		if err := os.Remove(credPath); err != nil {
			logger.Warn("[CloudflareTunnel] Failed to remove credential file", "path", credPath, "error", err)
		} else {
			logger.Info("[CloudflareTunnel] Credential file removed", "path", credPath)
		}
	}

	logger.Info("[CloudflareTunnel] Native tunnel stopped")

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Cloudflare tunnel stopped",
	})
	return string(out)
}

// ──────────────────────────────────────────────────────────────────────────
// Config Generation (Named Tunnel)
// ──────────────────────────────────────────────────────────────────────────

func buildIngressRules(cfg CloudflareTunnelConfig) []map[string]string {
	var rules []map[string]string

	if cfg.ExposeWebUI && cfg.WebUIPort > 0 {
		rules = append(rules, map[string]string{
			"service":  fmt.Sprintf("http://localhost:%d", cfg.WebUIPort),
			"hostname": "(auto — from CF dashboard)",
			"note":     "AuraGo Web UI",
		})
	}
	if cfg.ExposeHomepage && cfg.HomepagePort > 0 {
		rules = append(rules, map[string]string{
			"service":  fmt.Sprintf("http://localhost:%d", cfg.HomepagePort),
			"hostname": "(auto — from CF dashboard)",
			"note":     "Homepage Web Server",
		})
	}
	for _, r := range cfg.CustomIngress {
		entry := map[string]string{
			"hostname": r.Hostname,
			"service":  r.Service,
		}
		if r.Path != "" {
			entry["path"] = r.Path
		}
		rules = append(rules, entry)
	}
	// Catch-all is always required for named tunnel config
	rules = append(rules, map[string]string{
		"service": "http_status:404",
		"note":    "catch-all (required)",
	})
	return rules
}

func validateCustomIngress(rules []CloudflareIngress) error {
	for _, r := range rules {
		if r.Service == "" {
			return fmt.Errorf("ingress rule is missing a service URL")
		}
		u, err := url.Parse(r.Service)
		if err != nil {
			return fmt.Errorf("invalid service URL %q: %w", r.Service, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("service URL %q must use http or https scheme", r.Service)
		}
		if u.Host == "" {
			return fmt.Errorf("service URL %q is missing a host", r.Service)
		}
		// Block well-known sensitive ports to prevent accidental direct exposure
		switch u.Port() {
		case "22", "23", "3389", "5900", "5901":
			return fmt.Errorf("service URL %q targets sensitive port %s; expose such services via a reverse proxy instead", r.Service, u.Port())
		}
	}
	return nil
}

func writeNamedTunnelConfig(cfg CloudflareTunnelConfig, credPath, configPath string) error {
	if err := validateCustomIngress(cfg.CustomIngress); err != nil {
		return fmt.Errorf("invalid ingress configuration: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("tunnel: " + cfg.TunnelName + "\n")
	sb.WriteString("credentials-file: " + credPath + "\n")
	if cfg.LogLevel != "" {
		sb.WriteString("loglevel: " + cfg.LogLevel + "\n")
	}
	if cfg.MetricsPort > 0 {
		sb.WriteString(fmt.Sprintf("metrics: localhost:%d\n", cfg.MetricsPort))
	}
	sb.WriteString("\ningress:\n")

	if cfg.ExposeWebUI && cfg.WebUIPort > 0 {
		// For named tunnels, hostname must be set in CF dashboard; use the first custom rule
		// or let the user configure it. We create a catch-all that routes to web UI.
	}

	for _, r := range cfg.CustomIngress {
		sb.WriteString("  - hostname: " + r.Hostname + "\n")
		if r.Path != "" {
			sb.WriteString("    path: " + r.Path + "\n")
		}
		sb.WriteString("    service: " + r.Service + "\n")
	}

	// Auto-add web UI and homepage as ingress (user should set hostname in CF dashboard for token mode,
	// but for named tunnel we can use wildcard or specific rules)
	if cfg.ExposeWebUI && cfg.WebUIPort > 0 && !hasIngressForService(cfg.CustomIngress, cfg.WebUIPort) {
		// If no custom rule already maps to the web UI port, add a fallback
	}

	// Required catch-all
	sb.WriteString("  - service: http_status:404\n")

	// When AuraGo uses HTTPS with a self-signed certificate, tell cloudflared to skip
	// TLS verification for the local connection to the origin service.
	if cfg.HTTPSEnabled {
		sb.WriteString("\noriginRequest:\n  noTLSVerify: true\n")
	}

	return os.WriteFile(configPath, []byte(sb.String()), 0600)
}

func hasIngressForService(rules []CloudflareIngress, port int) bool {
	target := fmt.Sprintf(":%d", port)
	for _, r := range rules {
		if strings.Contains(r.Service, target) {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────────────────────────────────
// Binary Management
// ──────────────────────────────────────────────────────────────────────────

func cfdBinaryPath(dataDir string) string {
	binDir := filepath.Join(filepath.Dir(dataDir), "bin")
	if runtime.GOOS == "windows" {
		return filepath.Join(binDir, "cloudflared.exe")
	}
	return filepath.Join(binDir, "cloudflared")
}

func findCloudflaredBinary(dataDir string) string {
	// Check in AuraGo bin/ dir first
	binPath := cfdBinaryPath(dataDir)
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}
	// Check system PATH
	if p, err := exec.LookPath(cfdBinaryName); err == nil {
		return p
	}
	return ""
}

func installCloudflaredBinary(destPath string, logger *slog.Logger) string {
	arch := runtime.GOARCH
	goos := runtime.GOOS

	var downloadURL, checksumURL string
	switch {
	case goos == "linux" && arch == "amd64":
		downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
		checksumURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.sha256sum"
	case goos == "linux" && arch == "arm64":
		downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64"
		checksumURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64.sha256sum"
	case goos == "darwin" && arch == "amd64":
		downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-amd64.tgz"
	case goos == "darwin" && arch == "arm64":
		downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-arm64.tgz"
	case goos == "windows" && arch == "amd64":
		downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"
	default:
		return errJSON("Unsupported platform: %s/%s", goos, arch)
	}

	logger.Info("[CloudflareTunnel] Downloading cloudflared", "url", downloadURL, "dest", destPath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return errJSON("Failed to create bin directory: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return errJSON("Failed to download cloudflared: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errJSON("Download failed: HTTP %d", resp.StatusCode)
	}

	// Write to temp file first, then rename (atomic-ish)
	tmpPath := destPath + ".tmp." + randomHex(4)
	f, err := os.Create(tmpPath)
	if err != nil {
		return errJSON("Failed to create temp file: %v", err)
	}

	n, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return errJSON("Failed to write binary: %v", err)
	}

	// Verify integrity before installing (Linux only — sha256sum files available)
	if checksumURL != "" {
		if err := verifyCloudflaredChecksum(client, checksumURL, tmpPath, logger); err != nil {
			os.Remove(tmpPath)
			return errJSON("Binary integrity check failed: %v", err)
		}
	}

	// Make executable
	if goos != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			os.Remove(tmpPath)
			return errJSON("Failed to set permissions: %v", err)
		}
	}

	// Replace existing
	os.Remove(destPath)
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return errJSON("Failed to install binary: %v", err)
	}

	logger.Info("[CloudflareTunnel] Binary installed", "path", destPath, "bytes", n)

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("cloudflared installed (%d bytes)", n),
		"path":    destPath,
	})
	return string(out)
}

// ──────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────

// verifyCloudflaredChecksum downloads the .sha256sum file for the given URL and validates
// the already-downloaded binary at filePath. Best-effort: logs a warning and proceeds if
// the checksum file cannot be fetched (e.g. network issue or missing file for the platform).
func verifyCloudflaredChecksum(client *http.Client, checksumURL, filePath string, logger *slog.Logger) error {
	resp, err := client.Get(checksumURL)
	if err != nil {
		logger.Warn("[CloudflareTunnel] Could not download checksum file — skipping integrity check", "error", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logger.Warn("[CloudflareTunnel] Checksum file unavailable — skipping integrity check", "status", resp.StatusCode)
		return nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("[CloudflareTunnel] Failed to read checksum file — skipping integrity check", "error", err)
		return nil
	}
	// Format: "<sha256hash>  <filename>"
	parts := strings.Fields(string(data))
	if len(parts) == 0 {
		logger.Warn("[CloudflareTunnel] Checksum file empty or unparseable — skipping integrity check")
		return nil
	}
	expectedHash := strings.ToLower(strings.TrimSpace(parts[0]))

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open downloaded file for checksum verification: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash downloaded file: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch (expected %s, got %s) — possible tampering detected", expectedHash, actualHash)
	}
	logger.Info("[CloudflareTunnel] Binary checksum verified", "sha256", actualHash)
	return nil
}

// resolveMode determines whether to use Docker or native binary.
func resolveMode(cfg CloudflareTunnelConfig) string {
	switch cfg.Mode {
	case "docker":
		return "docker"
	case "native":
		return "native"
	default: // "auto"
		if checkDockerAvailable(cfg.DockerHost) {
			return "docker"
		}
		if findCloudflaredBinary(cfg.DataDir) != "" {
			return "native"
		}
		// Try to auto-install native binary
		return "native"
	}
}

func pullImage(dockerCfg DockerConfig, image string, logger *slog.Logger) {
	// Check if image exists
	filterURL := fmt.Sprintf("/images/json?filters=%%7B%%22reference%%22%%3A%%5B%%22%s%%22%%5D%%7D", image)
	data, code, err := dockerRequest(dockerCfg, "GET", filterURL, "")
	if err == nil && code == 200 {
		var images []interface{}
		if json.Unmarshal(data, &images) == nil && len(images) > 0 {
			return // Image already exists
		}
	}

	logger.Info("[CloudflareTunnel] Pulling image", "image", image)
	_, _, _ = dockerRequest(dockerCfg, "POST", "/images/create?fromImage="+image, "")
	// Wait a bit for pull to complete
	time.Sleep(3 * time.Second)
}

func removeContainer(dockerCfg DockerConfig, name string) {
	// Stop if running
	dockerRequest(dockerCfg, "POST", "/containers/"+name+"/stop?t=5", "")
	// Remove
	dockerRequest(dockerCfg, "DELETE", "/containers/"+name+"?force=true", "")
}

func captureQuickTunnelURLDocker(dockerCfg DockerConfig, logger *slog.Logger) string {
	// Read container logs
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		data, code, _ := dockerRequest(dockerCfg, "GET", "/containers/"+cfdContainerName+"/logs?stdout=true&stderr=true&tail=50", "")
		if code == 200 {
			output := stripDockerLogHeaders(data)
			if url := extractQuickTunnelURL(output); url != "" {
				return url
			}
		}
	}
	return ""
}

// extractQuickTunnelURL finds the trycloudflare.com URL in cloudflared output.
func extractQuickTunnelURL(output string) string {
	// cloudflared prints: "Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):"
	// followed by: "https://xxx-xxx-xxx.trycloudflare.com"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "trycloudflare.com") {
			// Extract the URL
			for _, word := range strings.Fields(line) {
				if strings.HasPrefix(word, "https://") && strings.Contains(word, "trycloudflare.com") {
					return word
				}
			}
		}
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// IsTunnelRunning returns true if a cloudflared tunnel is currently active.
func IsTunnelRunning() bool {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	return tunnelMode != ""
}

// GetTunnelURL returns the current tunnel URL (if any, mainly for quick tunnels).
func GetTunnelURL() string {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	return tunnelURL
}

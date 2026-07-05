package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
)

const (
	omniRouteDefaultImage         = "diegosouzapw/omniroute:3.8.39"
	omniRouteDefaultContainerName = "aurago_omniroute"
	omniRouteDefaultPort          = 20128
	omniRouteDefaultNetworkName   = "aurago_omniroute"
	omniRouteDefaultDataVolume    = "aurago_omniroute_data"
	omniRouteContainerDataPath    = "/app/data"
	omniRouteDefaultHealthPath    = "/api/monitoring/health"
	omniRouteDefaultMemoryMB      = 512
	omniRouteDefaultAlias         = "omniroute"
	omniRouteHealthProbeTimeout   = 3 * time.Second
)

var omniRouteHealthProbePaths = []string{omniRouteDefaultHealthPath, "/v1/models"}

// OmniRouteSidecarConfig is the resolved runtime configuration for the managed OmniRoute sidecar.
type OmniRouteSidecarConfig struct {
	Mode            string
	InternalBaseURL string
	BrowserBaseURL  string
	ProviderBaseURL string
	ContainerName   string
	Image           string
	Host            string
	Port            int
	HostPort        int
	NetworkName     string
	DataVolume      string
	HealthPath      string
	MemoryMB        int
	APIKey          string
	InitialPassword string
	JWTSecret       string
	APIKeySecret    string
	WSBridgeSecret  string
	RunningInDocker bool
}

// OmniRouteStatus reports the managed sidecar status in a UI-friendly shape.
type OmniRouteStatus struct {
	Enabled            bool   `json:"enabled"`
	Mode               string `json:"mode"`
	Status             string `json:"status"`
	Running            bool   `json:"running"`
	URL                string `json:"url"`
	ProviderBaseURL    string `json:"provider_base_url"`
	ContainerName      string `json:"container_name"`
	AdminSetupRequired bool   `json:"admin_setup_required"`
	Message            string `json:"message,omitempty"`
}

// OmniRouteManagedURLHost returns the host that AuraGo can manage for an OmniRoute URL.
func OmniRouteManagedURLHost(raw, containerName string, runningInDocker bool) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return host
	}
	if !runningInDocker {
		return ""
	}
	if host == omniRouteDefaultAlias {
		return host
	}
	if name := strings.ToLower(strings.TrimSpace(containerName)); name != "" && host == name {
		return host
	}
	return ""
}

// ResolveOmniRouteSidecarConfig resolves the managed sidecar URL, Docker names, and secrets.
func ResolveOmniRouteSidecarConfig(cfg *config.Config, runningInDocker bool) (OmniRouteSidecarConfig, error) {
	if cfg == nil {
		return OmniRouteSidecarConfig{}, fmt.Errorf("config is required")
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.OmniRoute.Mode))
	if mode == "" {
		mode = "managed"
	}
	if mode != "managed" {
		return OmniRouteSidecarConfig{Mode: mode, ProviderBaseURL: cfg.OmniRouteProviderBaseURL()}, nil
	}
	initialPassword := strings.TrimSpace(cfg.OmniRoute.InitialPassword)
	if initialPassword == "" {
		return OmniRouteSidecarConfig{}, fmt.Errorf("omniroute initial password is required in the vault before first managed start")
	}
	jwtSecret := strings.TrimSpace(cfg.OmniRoute.JWTSecret)
	if jwtSecret == "" {
		return OmniRouteSidecarConfig{}, fmt.Errorf("omniroute JWT secret is required in the vault")
	}
	apiKeySecret := strings.TrimSpace(cfg.OmniRoute.APIKeySecret)
	if apiKeySecret == "" {
		return OmniRouteSidecarConfig{}, fmt.Errorf("omniroute API key secret is required in the vault")
	}
	wsBridgeSecret := strings.TrimSpace(cfg.OmniRoute.WSBridgeSecret)
	if wsBridgeSecret == "" {
		return OmniRouteSidecarConfig{}, fmt.Errorf("omniroute websocket bridge secret is required in the vault")
	}
	port := cfg.OmniRoute.Port
	if port <= 0 {
		port = omniRouteDefaultPort
	}
	hostPort := cfg.OmniRoute.HostPort
	if hostPort <= 0 {
		hostPort = port
	}
	internalBaseURL := strings.TrimRight(strings.TrimSpace(cfg.OmniRoute.URL), "/")
	if internalBaseURL == "" {
		internalBaseURL = defaultOmniRouteBaseURL(runningInDocker, port)
	}
	host := strings.TrimSpace(cfg.OmniRoute.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	browserHost := host
	if browserHost == "" || browserHost == "0.0.0.0" || browserHost == "::" {
		browserHost = "127.0.0.1"
	}
	browserBaseURL := fmt.Sprintf("http://%s:%d", browserHost, hostPort)
	containerName := strings.TrimSpace(cfg.OmniRoute.ContainerName)
	if containerName == "" {
		containerName = omniRouteDefaultContainerName
	}
	networkName := strings.TrimSpace(cfg.OmniRoute.NetworkName)
	if networkName == "" {
		networkName = omniRouteDefaultNetworkName
	}
	healthPath := strings.TrimSpace(cfg.OmniRoute.HealthPath)
	if healthPath == "" {
		healthPath = omniRouteDefaultHealthPath
	}
	memoryMB := cfg.OmniRoute.MemoryMB
	if memoryMB <= 0 {
		memoryMB = omniRouteDefaultMemoryMB
	}
	return OmniRouteSidecarConfig{
		Mode:            "managed",
		InternalBaseURL: internalBaseURL,
		BrowserBaseURL:  browserBaseURL,
		ProviderBaseURL: strings.TrimRight(internalBaseURL, "/") + "/v1",
		ContainerName:   containerName,
		Image:           defaultString(cfg.OmniRoute.Image, omniRouteDefaultImage),
		Host:            host,
		Port:            port,
		HostPort:        hostPort,
		NetworkName:     networkName,
		DataVolume:      defaultString(cfg.OmniRoute.DataVolume, omniRouteDefaultDataVolume),
		HealthPath:      healthPath,
		MemoryMB:        memoryMB,
		APIKey:          strings.TrimSpace(cfg.OmniRoute.APIKey),
		InitialPassword: initialPassword,
		JWTSecret:       jwtSecret,
		APIKeySecret:    apiKeySecret,
		WSBridgeSecret:  wsBridgeSecret,
		RunningInDocker: runningInDocker,
	}, nil
}

func defaultOmniRouteBaseURL(runningInDocker bool, port int) string {
	if runningInDocker {
		return fmt.Sprintf("http://%s:%d", omniRouteDefaultAlias, port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func omniRouteRunsInDocker(cfg *config.Config) bool {
	return (cfg != nil && cfg.Runtime.IsDocker) || browserAutomationRunsInDocker()
}

func buildOmniRouteCreatePayload(sidecar OmniRouteSidecarConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(sidecar.ContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(sidecar.Image); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = sidecar.NetworkName
	}
	port := sidecar.Port
	if port <= 0 {
		port = omniRouteDefaultPort
	}
	hostPort := sidecar.HostPort
	if hostPort <= 0 {
		hostPort = port
	}
	containerPort := fmt.Sprintf("%d/tcp", port)
	env := []string{
		"DATA_DIR=" + omniRouteContainerDataPath,
		"PORT=" + strconv.Itoa(port),
		"DASHBOARD_PORT=" + strconv.Itoa(port),
		"HOSTNAME=0.0.0.0",
		"NODE_ENV=production",
		"OMNIROUTE_MEMORY_MB=" + strconv.Itoa(defaultOmniRouteMemoryMB(sidecar.MemoryMB)),
		"JWT_SECRET=" + sidecar.JWTSecret,
		"API_KEY_SECRET=" + sidecar.APIKeySecret,
		"INITIAL_PASSWORD=" + sidecar.InitialPassword,
		"OMNIROUTE_WS_BRIDGE_SECRET=" + sidecar.WSBridgeSecret,
		"REQUIRE_API_KEY=true",
		"ALLOW_API_KEY_REVEAL=false",
		"AUTH_COOKIE_SECURE=false",
		"NEXT_PUBLIC_BASE_URL=" + normalizeManifestBrowserBaseURL(sidecar.BrowserBaseURL),
		"BASE_URL=" + strings.TrimRight(strings.TrimSpace(sidecar.InternalBaseURL), "/"),
	}
	hostConfig := map[string]interface{}{
		"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
		"Binds":         []string{defaultString(sidecar.DataVolume, omniRouteDefaultDataVolume) + ":" + omniRouteContainerDataPath},
		"NetworkMode":   networkName,
		"PortBindings": map[string]interface{}{
			containerPort: []map[string]string{{"HostIp": manifestPublishHost(sidecar.Host), "HostPort": strconv.Itoa(hostPort)}},
		},
	}
	if memoryMB := defaultOmniRouteMemoryMB(sidecar.MemoryMB); memoryMB > 0 {
		hostConfig["Memory"] = int64(memoryMB) * 1024 * 1024
	}
	payload := map[string]interface{}{
		"Image": sidecar.Image,
		"Env":   env,
		"ExposedPorts": map[string]interface{}{
			containerPort: struct{}{},
		},
		"HostConfig":       hostConfig,
		"NetworkingConfig": manifestNetworkingConfig(networkName, omniRouteDefaultAlias),
	}
	return json.Marshal(payload)
}

func defaultOmniRouteMemoryMB(value int) int {
	if value <= 0 {
		return omniRouteDefaultMemoryMB
	}
	return value
}

// EnsureOmniRouteSidecarRunning creates and starts OmniRoute when managed mode is enabled.
func EnsureOmniRouteSidecarRunning(ctx context.Context, dockerHost string, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	return EnsureOmniRouteSidecarRunningWithBrowserURL(ctx, dockerHost, cfg, "", logger)
}

// EnsureOmniRouteSidecarRunningWithBrowserURL creates and starts managed OmniRoute and configures its public URL.
func EnsureOmniRouteSidecarRunningWithBrowserURL(ctx context.Context, dockerHost string, cfg *config.Config, browserBaseURL string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if cfg == nil || !cfg.OmniRoute.Enabled || strings.EqualFold(strings.TrimSpace(cfg.OmniRoute.Mode), "external") {
		return nil
	}
	sidecar, err := ResolveOmniRouteSidecarConfig(cfg, omniRouteRunsInDocker(cfg))
	if err != nil {
		return err
	}
	if normalized := normalizeManifestBrowserBaseURL(browserBaseURL); normalized != "" {
		sidecar.BrowserBaseURL = normalized
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	networkName, err := ensureOmniRouteDockerNetwork(dockerCfg, sidecar)
	if err != nil {
		if logger != nil {
			logger.Warn("[OmniRoute] Failed to ensure Docker network", "error", err)
		}
		return err
	}
	if err := ensureManifestContainerWithRecreate(ctx, dockerCfg, sidecar.ContainerName, sidecar.Image, func() ([]byte, error) {
		return buildOmniRouteCreatePayload(sidecar, networkName)
	}, func(data []byte) bool {
		return omniRouteContainerNeedsRecreate(data, sidecar, networkName)
	}); err != nil {
		if logger != nil {
			logger.Error("[OmniRoute] Failed to ensure sidecar", "error", err)
		}
		return err
	}
	if logger != nil {
		logger.Info("[OmniRoute] Sidecar is running", "container", sidecar.ContainerName)
	}
	return nil
}

func ensureOmniRouteDockerNetwork(dockerCfg DockerConfig, sidecar OmniRouteSidecarConfig) (string, error) {
	if sidecar.RunningInDocker {
		networkName, err := browserAutomationCurrentContainerNetwork(dockerCfg)
		if err == nil && strings.TrimSpace(networkName) != "" {
			return networkName, nil
		}
		return "", err
	}
	networkName := sidecar.NetworkName
	if networkName == "" {
		networkName = omniRouteDefaultNetworkName
	}
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/networks/"+url.PathEscape(networkName), "")
	if err != nil {
		return "", err
	}
	if code == http.StatusOK {
		_ = data
		return networkName, nil
	}
	if code != http.StatusNotFound {
		return "", fmt.Errorf("inspect Docker network %q returned HTTP %d", networkName, code)
	}
	body, _ := json.Marshal(map[string]interface{}{"Name": networkName, "Driver": "bridge"})
	_, createCode, createErr := dockerRequest(dockerCfg, http.MethodPost, "/networks/create", string(body))
	if createErr != nil {
		return "", createErr
	}
	if createCode != http.StatusCreated && createCode != http.StatusOK {
		return "", fmt.Errorf("create Docker network %q returned HTTP %d", networkName, createCode)
	}
	return networkName, nil
}

func omniRouteContainerNeedsRecreate(data []byte, sidecar OmniRouteSidecarConfig, networkName string) bool {
	var info struct {
		Config struct {
			Env []string `json:"Env"`
		} `json:"Config"`
		HostConfig struct {
			Binds        []string `json:"Binds"`
			Memory       int64    `json:"Memory"`
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
		NetworkSettings struct {
			Networks map[string]interface{} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return false
	}
	if got, want := manifestEnvValue(info.Config.Env, "NEXT_PUBLIC_BASE_URL"), normalizeManifestBrowserBaseURL(sidecar.BrowserBaseURL); got != want {
		return true
	}
	if !manifestContainerAttachedToNetwork(info.NetworkSettings.Networks, networkName) {
		return true
	}
	wantBind := defaultString(sidecar.DataVolume, omniRouteDefaultDataVolume) + ":" + omniRouteContainerDataPath
	foundBind := false
	for _, bind := range info.HostConfig.Binds {
		if strings.TrimSpace(bind) == wantBind {
			foundBind = true
			break
		}
	}
	if !foundBind {
		return true
	}
	wantMemory := int64(defaultOmniRouteMemoryMB(sidecar.MemoryMB)) * 1024 * 1024
	if wantMemory > 0 && info.HostConfig.Memory != wantMemory {
		return true
	}
	port := sidecar.Port
	if port <= 0 {
		port = omniRouteDefaultPort
	}
	hostPort := sidecar.HostPort
	if hostPort <= 0 {
		hostPort = port
	}
	containerPort := fmt.Sprintf("%d/tcp", port)
	bindings := info.HostConfig.PortBindings[containerPort]
	if len(bindings) == 0 {
		return true
	}
	wantHost := manifestPublishHost(sidecar.Host)
	wantPort := strconv.Itoa(hostPort)
	for _, binding := range bindings {
		hostIP := strings.TrimSpace(binding.HostIP)
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		if hostIP == wantHost && strings.TrimSpace(binding.HostPort) == wantPort {
			return false
		}
	}
	return true
}

// StopOmniRouteSidecar stops the managed OmniRoute container without deleting the data volume.
func StopOmniRouteSidecar(ctx context.Context, dockerHost string, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !cfg.OmniRoute.Enabled {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.OmniRoute.Mode))
	if mode == "" {
		mode = "managed"
	}
	if mode != "managed" {
		return nil
	}
	containerName := strings.TrimSpace(cfg.OmniRoute.ContainerName)
	if containerName == "" {
		containerName = omniRouteDefaultContainerName
	}
	if err := validateDockerName(containerName); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	_, _, _ = dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(containerName)+"/stop?t=5", "")
	_, _, _ = dockerRequest(dockerCfg, http.MethodDelete, "/containers/"+url.PathEscape(containerName)+"?force=true", "")
	if logger != nil {
		logger.Info("[OmniRoute] Sidecar stopped", "container", containerName)
	}
	return nil
}

// OmniRouteSidecarStatus returns a best-effort Docker and health status.
func OmniRouteSidecarStatus(ctx context.Context, dockerHost string, cfg *config.Config) (OmniRouteStatus, error) {
	if cfg == nil {
		return OmniRouteStatus{Status: manifestStatusUnknown, Message: "config is required"}, fmt.Errorf("config is required")
	}
	if !cfg.OmniRoute.Enabled {
		return OmniRouteStatus{Enabled: false, Mode: cfg.OmniRoute.Mode, Status: "disabled"}, nil
	}
	sidecar, err := ResolveOmniRouteSidecarConfig(cfg, omniRouteRunsInDocker(cfg))
	if err != nil {
		return OmniRouteStatus{Enabled: true, Mode: cfg.OmniRoute.Mode, Status: "setup_required", Message: err.Error(), AdminSetupRequired: true}, nil
	}
	status := OmniRouteStatus{
		Enabled:            true,
		Mode:               sidecar.Mode,
		Status:             manifestStatusStarting,
		URL:                sidecar.BrowserBaseURL,
		ProviderBaseURL:    sidecar.ProviderBaseURL,
		ContainerName:      sidecar.ContainerName,
		AdminSetupRequired: strings.TrimSpace(cfg.OmniRoute.APIKey) == "",
	}
	if sidecar.Mode != "managed" {
		status.Status = manifestStatusUnknown
		status.URL = sidecar.ProviderBaseURL
		status.ProviderBaseURL = sidecar.ProviderBaseURL
		return status, nil
	}
	data, code, err := dockerRequest(DockerConfig{Host: dockerHost}, http.MethodGet, "/containers/"+url.PathEscape(sidecar.ContainerName)+"/json", "")
	if err != nil {
		status.Message = err.Error()
		return status, nil
	}
	if code == http.StatusNotFound {
		status.Status = manifestStatusStopped
		return status, nil
	}
	if code != http.StatusOK {
		status.Status = manifestStatusUnknown
		status.Message = fmt.Sprintf("docker inspect returned HTTP %d", code)
		return status, nil
	}
	if manifestDockerContainerRunning(data) {
		status.Status = manifestDockerRunningStatus
		status.Running = true
		if ok, msg := ProbeOmniRouteHealth(ctx, sidecar); !ok {
			status.Status = manifestStatusStarting
			status.Running = false
			status.Message = msg
		}
	}
	return status, nil
}

// ProbeOmniRouteHealth checks HTTP health endpoints, then falls back to TCP reachability.
func ProbeOmniRouteHealth(ctx context.Context, sidecar OmniRouteSidecarConfig) (bool, string) {
	base := strings.TrimRight(sidecar.InternalBaseURL, "/")
	if base == "" {
		return false, "OmniRoute URL is not configured"
	}
	paths := []string{}
	if strings.TrimSpace(sidecar.HealthPath) != "" {
		paths = append(paths, strings.TrimSpace(sidecar.HealthPath))
	}
	paths = append(paths, omniRouteHealthProbePaths...)
	client := &http.Client{Timeout: omniRouteHealthProbeTimeout}
	for _, path := range paths {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return true, ""
			}
		}
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return false, "OmniRoute health endpoint was not confirmed"
	}
	addr := parsed.Host
	if !strings.Contains(addr, ":") {
		addr += ":80"
	}
	conn, err := (&net.Dialer{Timeout: omniRouteHealthProbeTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, "OmniRoute HTTP health endpoint and TCP fallback were not reachable"
	}
	_ = conn.Close()
	return true, "OmniRoute TCP port is reachable, but no HTTP health endpoint was confirmed"
}

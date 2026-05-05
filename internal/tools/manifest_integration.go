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
	manifestDefaultImage                 = "manifestdotbuild/manifest:5"
	manifestDefaultContainerName         = "aurago_manifest"
	manifestDefaultPort                  = 2099
	manifestDefaultNetworkName           = "aurago_manifest"
	manifestDefaultPostgresContainerName = "aurago_manifest_postgres"
	manifestDefaultPostgresAlias         = "manifest-postgres"
	manifestDefaultPostgresImage         = "postgres:15-alpine"
	manifestDefaultPostgresUser          = "manifest"
	manifestDefaultPostgresDatabase      = "manifest"
	manifestDefaultPostgresVolume        = "aurago_manifest_pgdata"
	manifestPostgresContainerDataPath    = "/var/lib/postgresql/data"
	manifestDockerRunningStatus          = "running"
	manifestStatusStarting               = "starting"
	manifestStatusStopped                = "stopped"
	manifestStatusUnknown                = "unknown"
	manifestHealthProbeTimeout           = 3 * time.Second
)

var manifestHealthProbePaths = []string{"/health", "/api/health"}

// ManifestSidecarConfig is the resolved runtime configuration for Manifest and its Postgres sidecar.
type ManifestSidecarConfig struct {
	Mode                  string
	InternalBaseURL       string
	BrowserBaseURL        string
	ProviderBaseURL       string
	ContainerName         string
	Image                 string
	Host                  string
	Port                  int
	HostPort              int
	NetworkName           string
	PostgresContainerName string
	PostgresAlias         string
	PostgresImage         string
	PostgresUser          string
	PostgresDatabase      string
	PostgresVolume        string
	PostgresPassword      string
	BetterAuthSecret      string
	HealthPath            string
	RunningInDocker       bool
}

// ManifestStatus reports the managed sidecar status in a UI-friendly shape.
type ManifestStatus struct {
	Enabled            bool   `json:"enabled"`
	Mode               string `json:"mode"`
	Status             string `json:"status"`
	Running            bool   `json:"running"`
	URL                string `json:"url"`
	ProviderBaseURL    string `json:"provider_base_url"`
	ContainerName      string `json:"container_name"`
	PostgresContainer  string `json:"postgres_container"`
	AdminSetupRequired bool   `json:"admin_setup_required"`
	Message            string `json:"message,omitempty"`
}

// ManifestManagedURLHost returns the host that AuraGo can manage for a Manifest URL.
func ManifestManagedURLHost(raw, containerName string, runningInDocker bool) string {
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
	if host == "manifest" {
		return host
	}
	if name := strings.ToLower(strings.TrimSpace(containerName)); name != "" && host == name {
		return host
	}
	return ""
}

// ResolveManifestSidecarConfig resolves the managed sidecar URLs, names, and secrets.
func ResolveManifestSidecarConfig(cfg *config.Config, runningInDocker bool) (ManifestSidecarConfig, error) {
	if cfg == nil {
		return ManifestSidecarConfig{}, fmt.Errorf("config is required")
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Manifest.Mode))
	if mode == "" {
		mode = "managed"
	}
	if mode != "managed" {
		return ManifestSidecarConfig{Mode: mode, ProviderBaseURL: strings.TrimRight(strings.TrimSpace(cfg.Manifest.ExternalBaseURL), "/")}, nil
	}
	postgresPassword := strings.TrimSpace(cfg.Manifest.PostgresPassword)
	if postgresPassword == "" {
		return ManifestSidecarConfig{}, fmt.Errorf("manifest postgres password is required in the vault")
	}
	betterAuthSecret := strings.TrimSpace(cfg.Manifest.BetterAuthSecret)
	if betterAuthSecret == "" {
		return ManifestSidecarConfig{}, fmt.Errorf("manifest better auth secret is required in the vault")
	}
	port := cfg.Manifest.Port
	if port <= 0 {
		port = manifestDefaultPort
	}
	hostPort := cfg.Manifest.HostPort
	if hostPort <= 0 {
		hostPort = port
	}
	internalBaseURL := strings.TrimRight(strings.TrimSpace(cfg.Manifest.URL), "/")
	if internalBaseURL == "" {
		internalBaseURL = defaultManifestBaseURL(runningInDocker, port)
	}
	host := strings.TrimSpace(cfg.Manifest.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	browserHost := host
	if browserHost == "" || browserHost == "0.0.0.0" || browserHost == "::" {
		browserHost = "127.0.0.1"
	}
	browserBaseURL := fmt.Sprintf("http://%s:%d", browserHost, hostPort)
	containerName := strings.TrimSpace(cfg.Manifest.ContainerName)
	if containerName == "" {
		containerName = manifestDefaultContainerName
	}
	networkName := strings.TrimSpace(cfg.Manifest.NetworkName)
	if networkName == "" {
		networkName = manifestDefaultNetworkName
	}
	return ManifestSidecarConfig{
		Mode:                  "managed",
		InternalBaseURL:       internalBaseURL,
		BrowserBaseURL:        browserBaseURL,
		ProviderBaseURL:       strings.TrimRight(internalBaseURL, "/") + "/v1",
		ContainerName:         containerName,
		Image:                 defaultString(cfg.Manifest.Image, manifestDefaultImage),
		Host:                  host,
		Port:                  port,
		HostPort:              hostPort,
		NetworkName:           networkName,
		PostgresContainerName: defaultString(cfg.Manifest.PostgresContainerName, manifestDefaultPostgresContainerName),
		PostgresAlias:         manifestDefaultPostgresAlias,
		PostgresImage:         defaultString(cfg.Manifest.PostgresImage, manifestDefaultPostgresImage),
		PostgresUser:          defaultString(cfg.Manifest.PostgresUser, manifestDefaultPostgresUser),
		PostgresDatabase:      defaultString(cfg.Manifest.PostgresDatabase, manifestDefaultPostgresDatabase),
		PostgresVolume:        defaultString(cfg.Manifest.PostgresVolume, manifestDefaultPostgresVolume),
		PostgresPassword:      postgresPassword,
		BetterAuthSecret:      betterAuthSecret,
		HealthPath:            strings.TrimSpace(cfg.Manifest.HealthPath),
		RunningInDocker:       runningInDocker,
	}, nil
}

func defaultManifestBaseURL(runningInDocker bool, port int) string {
	if runningInDocker {
		return fmt.Sprintf("http://manifest:%d", port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func manifestDatabaseURL(sidecar ManifestSidecarConfig) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:5432/%s",
		url.QueryEscape(sidecar.PostgresUser),
		url.QueryEscape(sidecar.PostgresPassword),
		sidecar.PostgresAlias,
		url.QueryEscape(sidecar.PostgresDatabase),
	)
}

func buildManifestPostgresCreatePayload(sidecar ManifestSidecarConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(sidecar.PostgresContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(sidecar.PostgresImage); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = sidecar.NetworkName
	}
	env := []string{
		"POSTGRES_USER=" + sidecar.PostgresUser,
		"POSTGRES_PASSWORD=" + sidecar.PostgresPassword,
		"POSTGRES_DB=" + sidecar.PostgresDatabase,
	}
	payload := map[string]interface{}{
		"Image": sidecar.PostgresImage,
		"Env":   env,
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"Binds":         []string{sidecar.PostgresVolume + ":" + manifestPostgresContainerDataPath},
			"NetworkMode":   networkName,
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, sidecar.PostgresAlias),
	}
	return json.Marshal(payload)
}

func buildManifestCreatePayload(sidecar ManifestSidecarConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(sidecar.ContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(sidecar.Image); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = sidecar.NetworkName
	}
	containerPort := fmt.Sprintf("%d/tcp", sidecar.Port)
	env := []string{
		"PORT=" + strconv.Itoa(sidecar.Port),
		"DATABASE_URL=" + manifestDatabaseURL(sidecar),
		"BETTER_AUTH_SECRET=" + sidecar.BetterAuthSecret,
		"BETTER_AUTH_URL=" + sidecar.BrowserBaseURL,
		"MANIFEST_TELEMETRY_DISABLED=1",
	}
	payload := map[string]interface{}{
		"Image": sidecar.Image,
		"Env":   env,
		"ExposedPorts": map[string]interface{}{
			containerPort: struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"NetworkMode":   networkName,
			"PortBindings": map[string]interface{}{
				containerPort: []map[string]string{{"HostIp": sidecar.Host, "HostPort": strconv.Itoa(sidecar.HostPort)}},
			},
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, "manifest"),
	}
	return json.Marshal(payload)
}

func manifestNetworkingConfig(networkName, alias string) map[string]interface{} {
	if strings.TrimSpace(networkName) == "" {
		return nil
	}
	aliases := []string{}
	if strings.TrimSpace(alias) != "" {
		aliases = append(aliases, strings.TrimSpace(alias))
	}
	return map[string]interface{}{
		"EndpointsConfig": map[string]interface{}{
			networkName: map[string]interface{}{
				"Aliases": aliases,
			},
		},
	}
}

// EnsureManifestSidecarsRunning creates and starts Postgres and Manifest when managed mode is enabled.
func EnsureManifestSidecarsRunning(ctx context.Context, dockerHost string, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if cfg == nil || !cfg.Manifest.Enabled || strings.ToLower(strings.TrimSpace(cfg.Manifest.Mode)) == "external" {
		return nil
	}
	sidecar, err := ResolveManifestSidecarConfig(cfg, browserAutomationRunsInDocker())
	if err != nil {
		return err
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	networkName, err := ensureManifestDockerNetwork(dockerCfg, sidecar)
	if err != nil {
		logger.Warn("[Manifest] Failed to ensure Docker network", "error", err)
		return err
	}
	if err := ensureManifestContainer(ctx, dockerCfg, sidecar.PostgresContainerName, sidecar.PostgresImage, func() ([]byte, error) {
		return buildManifestPostgresCreatePayload(sidecar, networkName)
	}); err != nil {
		logger.Error("[Manifest] Failed to ensure Postgres sidecar", "error", err)
		return err
	}
	if err := ensureManifestContainer(ctx, dockerCfg, sidecar.ContainerName, sidecar.Image, func() ([]byte, error) {
		return buildManifestCreatePayload(sidecar, networkName)
	}); err != nil {
		logger.Error("[Manifest] Failed to ensure Manifest sidecar", "error", err)
		return err
	}
	logger.Info("[Manifest] Sidecars are running", "container", sidecar.ContainerName, "postgres", sidecar.PostgresContainerName)
	return nil
}

func ensureManifestDockerNetwork(dockerCfg DockerConfig, sidecar ManifestSidecarConfig) (string, error) {
	if sidecar.RunningInDocker {
		networkName, err := browserAutomationCurrentContainerNetwork(dockerCfg)
		if err == nil && strings.TrimSpace(networkName) != "" {
			return networkName, nil
		}
		return "", err
	}
	networkName := sidecar.NetworkName
	if networkName == "" {
		networkName = manifestDefaultNetworkName
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

func ensureManifestContainer(ctx context.Context, dockerCfg DockerConfig, containerName, image string, payload func() ([]byte, error)) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/containers/"+url.PathEscape(containerName)+"/json", "")
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		if manifestDockerContainerRunning(data) {
			return nil
		}
		_, startCode, startErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(containerName)+"/start", "")
		if startErr != nil {
			return startErr
		}
		if startCode != http.StatusNoContent && startCode != http.StatusNotModified {
			return fmt.Errorf("start container %q returned HTTP %d", containerName, startCode)
		}
		return nil
	}
	if code != http.StatusNotFound {
		return fmt.Errorf("inspect container %q returned HTTP %d", containerName, code)
	}
	if _, imgCode, imgErr := dockerRequest(dockerCfg, http.MethodGet, "/images/"+url.PathEscape(image)+"/json", ""); imgErr != nil || imgCode != http.StatusOK {
		_, _, _ = dockerRequest(dockerCfg, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(image), "")
	}
	body, err := payload()
	if err != nil {
		return err
	}
	_, createCode, createErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/create?name="+url.QueryEscape(containerName), string(body))
	if createErr != nil {
		return createErr
	}
	if createCode != http.StatusCreated {
		return fmt.Errorf("create container %q returned HTTP %d", containerName, createCode)
	}
	_, startCode, startErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(containerName)+"/start", "")
	if startErr != nil {
		return startErr
	}
	if startCode != http.StatusNoContent && startCode != http.StatusNotModified {
		return fmt.Errorf("start container %q returned HTTP %d", containerName, startCode)
	}
	return nil
}

func manifestDockerContainerRunning(data []byte) bool {
	var info struct {
		State struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
		} `json:"State"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return false
	}
	return info.State.Running || strings.EqualFold(info.State.Status, manifestDockerRunningStatus)
}

// StopManifestSidecars stops the managed Manifest containers without deleting the Postgres volume.
func StopManifestSidecars(ctx context.Context, dockerHost string, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	sidecar, err := ResolveManifestSidecarConfig(cfg, browserAutomationRunsInDocker())
	if err != nil {
		return err
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	for _, name := range []string{sidecar.ContainerName, sidecar.PostgresContainerName} {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, _, _ = dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(name)+"/stop?t=5", "")
		_, _, _ = dockerRequest(dockerCfg, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=true", "")
	}
	logger.Info("[Manifest] Sidecars stopped", "container", sidecar.ContainerName, "postgres", sidecar.PostgresContainerName)
	return nil
}

// ManifestSidecarStatus returns a best-effort Docker and health status.
func ManifestSidecarStatus(ctx context.Context, dockerHost string, cfg *config.Config) (ManifestStatus, error) {
	if cfg == nil {
		return ManifestStatus{Status: manifestStatusUnknown, Message: "config is required"}, fmt.Errorf("config is required")
	}
	if !cfg.Manifest.Enabled {
		return ManifestStatus{Enabled: false, Mode: cfg.Manifest.Mode, Status: "disabled"}, nil
	}
	sidecar, err := ResolveManifestSidecarConfig(cfg, browserAutomationRunsInDocker())
	if err != nil {
		return ManifestStatus{Enabled: true, Mode: cfg.Manifest.Mode, Status: "setup_required", Message: err.Error(), AdminSetupRequired: true}, nil
	}
	status := ManifestStatus{
		Enabled:            true,
		Mode:               sidecar.Mode,
		Status:             manifestStatusStarting,
		URL:                sidecar.BrowserBaseURL,
		ProviderBaseURL:    sidecar.ProviderBaseURL,
		ContainerName:      sidecar.ContainerName,
		PostgresContainer:  sidecar.PostgresContainerName,
		AdminSetupRequired: strings.TrimSpace(cfg.Manifest.APIKey) == "",
	}
	if sidecar.Mode != "managed" {
		status.Status = manifestStatusUnknown
		status.URL = strings.TrimRight(strings.TrimSpace(cfg.Manifest.ExternalBaseURL), "/")
		status.ProviderBaseURL = status.URL
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
		if ok, msg := ProbeManifestHealth(ctx, sidecar); !ok {
			status.Status = manifestStatusStarting
			status.Running = false
			status.Message = msg
		}
	}
	return status, nil
}

// ProbeManifestHealth checks HTTP health endpoints, then falls back to TCP reachability.
func ProbeManifestHealth(ctx context.Context, sidecar ManifestSidecarConfig) (bool, string) {
	base := strings.TrimRight(sidecar.InternalBaseURL, "/")
	if base == "" {
		return false, "Manifest URL is not configured"
	}
	paths := []string{}
	if strings.TrimSpace(sidecar.HealthPath) != "" {
		paths = append(paths, strings.TrimSpace(sidecar.HealthPath))
	}
	paths = append(paths, manifestHealthProbePaths...)
	client := &http.Client{Timeout: manifestHealthProbeTimeout}
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
		return false, "Manifest health endpoint was not confirmed"
	}
	addr := parsed.Host
	if !strings.Contains(addr, ":") {
		addr += ":80"
	}
	conn, err := (&net.Dialer{Timeout: manifestHealthProbeTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, "Manifest HTTP health endpoint and TCP fallback were not reachable"
	}
	_ = conn.Close()
	return true, "Manifest TCP port is reachable, but no HTTP health endpoint was confirmed"
}

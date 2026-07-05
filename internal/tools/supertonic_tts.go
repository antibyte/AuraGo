package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
)

const (
	defaultSupertonicContainerName = "aurago-supertonic-tts"
	defaultSupertonicImage         = "ghcr.io/antibyte/aurago-supertonic:latest"
	defaultSupertonicPort          = 7788
	defaultSupertonicModel         = "supertonic-3"
)

var supertonicHTTPClient = &http.Client{Timeout: 10 * time.Second}

// SupertonicStyle describes one built-in or imported voice style returned by /v1/styles.
type SupertonicStyle struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
}

// EnsureSupertonicRunning ensures the managed Supertonic HTTP sidecar is running.
func EnsureSupertonicRunning(cfg *config.Config, logger *slog.Logger) {
	if cfg == nil {
		return
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.TTS.Provider))
	if provider != "supertonic" {
		if logger != nil {
			logger.Info("[Supertonic TTS] Skipping auto-start (provider is not supertonic)")
		}
		return
	}
	st := cfg.TTS.Supertonic
	if !st.AutoStart {
		if logger != nil {
			logger.Info("[Supertonic TTS] Skipping auto-start (disabled in config)")
		}
		return
	}

	dockerCfg := DockerConfig{Host: cfg.Docker.Host}
	port := st.ContainerPort
	if port <= 0 {
		port = defaultSupertonicPort
	}
	portStr := fmt.Sprintf("%d", port)

	image := strings.TrimSpace(st.Image)
	if image == "" {
		image = defaultSupertonicImage
	}
	model := strings.TrimSpace(st.Model)
	if model == "" {
		model = defaultSupertonicModel
	}
	containerName := supertonicContainerName(st.ContainerName)

	dataPath := strings.TrimSpace(st.DataPath)
	if dataPath == "" {
		dataPath = "data/supertonic"
	}
	absData, err := filepath.Abs(dataPath)
	if err != nil {
		logSupertonic(logger, slog.LevelError, "Failed to resolve data path", "path", dataPath, "error", err)
		return
	}
	if err := os.MkdirAll(absData, 0o755); err != nil {
		logSupertonic(logger, slog.LevelError, "Failed to create data path", "path", absData, "error", err)
		return
	}

	listData, listCode, listErr := dockerRequest(dockerCfg, "GET",
		fmt.Sprintf(`/containers/json?filters={"status":["running"],"ancestor":[%q]}`, image), "")
	if listErr == nil && listCode == http.StatusOK {
		var containers []map[string]interface{}
		if json.Unmarshal(listData, &containers) == nil && len(containers) > 0 {
			logSupertonic(logger, slog.LevelInfo, "Container already running (external)", "count", len(containers))
			waitForSupertonicReady(supertonicBaseURL(st.URL, port), logger)
			return
		}
	}

	data, code, inspectErr := dockerRequest(dockerCfg, "GET", "/containers/"+url.PathEscape(containerName)+"/json", "")
	if inspectErr != nil {
		logSupertonic(logger, slog.LevelWarn, "Docker unavailable, skipping auto-start", "error", inspectErr)
		return
	}

	if code == http.StatusOK {
		var info map[string]interface{}
		if json.Unmarshal(data, &info) == nil {
			if state, ok := info["State"].(map[string]interface{}); ok {
				if running, _ := state["Running"].(bool); running {
					logSupertonic(logger, slog.LevelInfo, "Container already running")
					waitForSupertonicReady(supertonicBaseURL(st.URL, port), logger)
					return
				}
			}
		}
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+url.PathEscape(containerName)+"/start", "")
		if startErr != nil || (startCode != http.StatusNoContent && startCode != http.StatusNotModified) {
			logSupertonic(logger, slog.LevelError, "Failed to start existing container", "code", startCode, "error", startErr)
			return
		}
		logSupertonic(logger, slog.LevelInfo, "Container started")
		waitForSupertonicReady(supertonicBaseURL(st.URL, port), logger)
		return
	}
	if code != http.StatusNotFound {
		logSupertonic(logger, slog.LevelWarn, "Unexpected Docker inspect response, skipping auto-start", "code", code)
		return
	}

	logSupertonic(logger, slog.LevelInfo, "Pulling image", "image", image)
	_, pullCode, pullErr := dockerRequest(dockerCfg, "POST", "/images/create?fromImage="+url.QueryEscape(image), "")
	if pullErr != nil {
		logSupertonic(logger, slog.LevelWarn, "Image pull failed, trying to create container anyway", "error", pullErr)
	} else if pullCode != http.StatusOK {
		logSupertonic(logger, slog.LevelWarn, "Image pull returned unexpected status", "code", pullCode)
	}

	payload := map[string]interface{}{
		"Image": image,
		"Cmd":   []string{"supertonic", "serve", "--host", "0.0.0.0", "--port", fmt.Sprintf("%d", defaultSupertonicPort), "--model", model},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"PortBindings": map[string]interface{}{
				fmt.Sprintf("%d/tcp", defaultSupertonicPort): []map[string]string{{"HostIp": "127.0.0.1", "HostPort": portStr}},
			},
			"Binds": []string{absData + ":/root/.cache"},
		},
		"ExposedPorts": map[string]interface{}{
			fmt.Sprintf("%d/tcp", defaultSupertonicPort): struct{}{},
		},
	}
	body, _ := json.Marshal(payload)
	_, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+url.QueryEscape(containerName), string(body))
	if createErr != nil || createCode != http.StatusCreated {
		logSupertonic(logger, slog.LevelError, "Failed to create container", "code", createCode, "error", createErr)
		return
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+url.PathEscape(containerName)+"/start", "")
	if startErr != nil || (startCode != http.StatusNoContent && startCode != http.StatusNotModified) {
		logSupertonic(logger, slog.LevelError, "Failed to start new container", "code", startCode, "error", startErr)
		return
	}
	logSupertonic(logger, slog.LevelInfo, "Container created and started", "image", image, "port", port, "model", model)
	waitForSupertonicReady(supertonicBaseURL(st.URL, port), logger)
}

// SupertonicHealth checks the local Supertonic HTTP server and returns UI-friendly status data.
func SupertonicHealth(baseURL string) map[string]interface{} {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return map[string]interface{}{
			"status":  "unconfigured",
			"message": "Supertonic URL is not configured",
		}
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/health", nil)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	resp, err := supertonicHTTPClient.Do(req)
	if err != nil {
		return map[string]interface{}{"status": "stopped", "message": err.Error()}
	}
	defer resp.Body.Close()

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}
	}

	var health map[string]interface{}
	if err := json.Unmarshal(body, &health); err != nil {
		return map[string]interface{}{"status": "error", "message": "invalid health response: " + err.Error()}
	}
	upstreamStatus, _ := health["status"].(string)
	if strings.EqualFold(upstreamStatus, "ok") {
		health["upstream_status"] = upstreamStatus
		health["status"] = "running"
	} else if strings.TrimSpace(upstreamStatus) == "" {
		health["status"] = "running"
	}
	health["url"] = baseURL
	return health
}

// SupertonicListStyles returns available built-in and imported voice styles.
func SupertonicListStyles(baseURL string) ([]SupertonicStyle, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("Supertonic URL is not configured")
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/styles", nil)
	if err != nil {
		return nil, fmt.Errorf("create Supertonic styles request: %w", err)
	}
	resp, err := supertonicHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Supertonic styles request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("read Supertonic styles response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Supertonic styles returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var wrapped struct {
		Styles []SupertonicStyle `json:"styles"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Styles != nil {
		return wrapped.Styles, nil
	}
	var styles []SupertonicStyle
	if err := json.Unmarshal(body, &styles); err != nil {
		return nil, fmt.Errorf("parse Supertonic styles response: %w", err)
	}
	return styles, nil
}

func waitForSupertonicReady(baseURL string, logger *slog.Logger) {
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		health := SupertonicHealth(baseURL)
		status, _ := health["status"].(string)
		if status == "running" || status == "loading" {
			logSupertonic(logger, slog.LevelInfo, "HTTP server is ready", "url", baseURL, "status", status)
			return
		}
		time.Sleep(3 * time.Second)
	}
	logSupertonic(logger, slog.LevelWarn, "Timed out waiting for HTTP server", "url", baseURL)
}

func supertonicBaseURL(raw string, port int) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw != "" {
		return raw
	}
	if port <= 0 {
		port = defaultSupertonicPort
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func supertonicContainerName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return defaultSupertonicContainerName
	}
	return name
}

func logSupertonic(logger *slog.Logger, level slog.Level, msg string, args ...any) {
	if logger == nil {
		return
	}
	logger.Log(context.Background(), level, "[Supertonic TTS] "+msg, args...)
}

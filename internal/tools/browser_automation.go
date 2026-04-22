package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
)

const (
	browserAutomationContainerName = "aurago_browser_automation"
	browserAutomationImage         = "aurago-browser-automation:latest"
	browserAutomationContainerPort = 7331
	browserAutomationWorkspaceDir  = "/workspace"
	browserAutomationDownloadsDir  = "/downloads"
)

type BrowserAutomationRequest struct {
	Operation    string
	SessionID    string
	URL          string
	Selector     string
	Text         string
	Value        string
	Key          string
	WaitFor      string
	TimeoutMs    int
	OutputPath   string
	FullPage     bool
	FilePath     string
	DownloadName string
	DOMSnippet   bool
	MaxElements  int
}

type BrowserAutomationSidecarConfig struct {
	URL            string
	Image          string
	ContainerName  string
	AutoBuild      bool
	DockerfileDir  string
	SessionTTL     int
	MaxSessions    int
	Headless       bool
	AllowUploads   bool
	AllowDownloads bool
	ReadOnly       bool
	WorkspaceDir   string
	DownloadDir    string
	ViewportWidth  int
	ViewportHeight int
}

var browserAutomationHTTPClient = &http.Client{Timeout: 60 * time.Second}

var browserAutomationRetryDelays = []time.Duration{
	200 * time.Millisecond,
	500 * time.Millisecond,
}

func browserAutomationJSON(result map[string]interface{}) string {
	data, err := json.Marshal(result)
	if err != nil {
		return `{"status":"error","message":"failed to encode browser automation result"}`
	}
	return string(data)
}

func browserAutomationReadOnlyBlocked(op string) bool {
	switch op {
	case "click", "type", "select", "press", "upload_file":
		return true
	default:
		return false
	}
}

func browserAutomationNeedsSession(op string) bool {
	return op != "" && op != "create_session"
}

func browserAutomationResolveWorkspaceRoot(cfg *config.Config) (string, error) {
	return filepath.Abs(cfg.Directories.WorkspaceDir)
}

func browserAutomationResolveDownloadsRoot(cfg *config.Config, workspaceRoot string) (string, error) {
	target := strings.TrimSpace(cfg.BrowserAutomation.AllowedDownloadDir)
	if target == "" {
		target = "browser_downloads"
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(workspaceRoot, target)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func browserAutomationResolveScreenshotsRoot(cfg *config.Config, workspaceRoot string) (string, error) {
	target := strings.TrimSpace(cfg.BrowserAutomation.ScreenshotsDir)
	if target == "" {
		target = "browser_screenshots"
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(workspaceRoot, target)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func browserAutomationRelFromWorkspace(workspaceRoot, candidate string) (string, string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", "", fmt.Errorf("path is required")
	}
	abs := candidate
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspaceRoot, candidate)
	}
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(workspaceRoot, abs)
	if err != nil {
		return "", "", err
	}
	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("path must stay inside workspace")
	}
	return filepath.ToSlash(rel), abs, nil
}

func browserAutomationRelFromRoot(root, candidate string) (string, string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", "", fmt.Errorf("path is required")
	}
	abs := candidate
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, candidate)
	}
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", err
	}
	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("path must stay inside allowed directory")
	}
	return filepath.ToSlash(rel), abs, nil
}

func browserAutomationWebPath(workspaceRoot, localPath string) string {
	rel, err := filepath.Rel(workspaceRoot, localPath)
	if err != nil {
		return ""
	}
	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return "/files/" + filepath.ToSlash(rel)
}

func browserAutomationManagedURLHost(raw, containerName string, runningInDocker bool) string {
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
	if host == "browser-automation" {
		return host
	}
	if name := strings.ToLower(strings.TrimSpace(containerName)); name != "" && host == name {
		return host
	}
	return ""
}

func browserAutomationIsLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func browserAutomationRunsInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func browserAutomationEffectiveContainerName(sidecarCfg BrowserAutomationSidecarConfig, managedHost string) string {
	name := strings.TrimSpace(sidecarCfg.ContainerName)
	if name == "" {
		name = browserAutomationContainerName
	}
	if browserAutomationIsLoopbackHost(managedHost) {
		return name
	}
	if name == "" || strings.EqualFold(name, browserAutomationContainerName) {
		return managedHost
	}
	return name
}

func browserAutomationCurrentContainerNetwork(dockerCfg DockerConfig) (string, error) {
	if !browserAutomationRunsInDocker() {
		return "", fmt.Errorf("current process is not running inside Docker")
	}
	selfID, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("resolve current container hostname: %w", err)
	}
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+url.PathEscape(selfID)+"/json", "")
	if err != nil {
		return "", fmt.Errorf("inspect current container %q: %w", selfID, err)
	}
	if code != 200 {
		return "", fmt.Errorf("inspect current container %q returned status %d", selfID, code)
	}
	var info struct {
		NetworkSettings struct {
			Networks map[string]struct{} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("decode current container networks: %w", err)
	}
	for networkName := range info.NetworkSettings.Networks {
		if strings.TrimSpace(networkName) != "" {
			return networkName, nil
		}
	}
	return "", fmt.Errorf("current container has no attached Docker network")
}

func browserAutomationDefaultScreenshotRel(req BrowserAutomationRequest) string {
	sessionPart := strings.TrimSpace(req.SessionID)
	if sessionPart == "" {
		sessionPart = "session"
	}
	return filepath.ToSlash(filepath.Join("browser_screenshots", fmt.Sprintf("%s_%d.png", sessionPart, time.Now().UnixMilli())))
}

func browserAutomationSidecarRequest(ctx context.Context, cfg BrowserAutomationSidecarConfig, payload map[string]interface{}) (map[string]interface{}, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("browser automation URL is not configured")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal sidecar payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/automation", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create sidecar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var (
		resp   *http.Response
		reqErr error
	)
	for attempt := 0; attempt <= len(browserAutomationRetryDelays); attempt++ {
		clonedReq := req.Clone(ctx)
		clonedReq.Body = io.NopCloser(bytes.NewReader(data))
		resp, reqErr = browserAutomationHTTPClient.Do(clonedReq)
		if reqErr == nil {
			break
		}
		if attempt == len(browserAutomationRetryDelays) || !browserAutomationRetryableError(reqErr) {
			return nil, fmt.Errorf("browser automation request failed: %w", reqErr)
		}
		if sleepErr := browserAutomationSleepWithContext(ctx, browserAutomationRetryDelays[attempt]); sleepErr != nil {
			return nil, fmt.Errorf("browser automation request failed: %w", reqErr)
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read sidecar response: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode sidecar response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if result == nil {
			result = map[string]interface{}{}
		}
		if _, ok := result["status"]; !ok {
			result["status"] = "error"
		}
		if _, ok := result["message"]; !ok {
			result["message"] = fmt.Sprintf("sidecar returned HTTP %d", resp.StatusCode)
		}
		return result, nil
	}
	return result, nil
}

func BrowserAutomationHealth(ctx context.Context, cfg *config.Config) map[string]interface{} {
	sidecarCfg, err := browserAutomationSidecarConfig(cfg)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	baseURL := strings.TrimRight(sidecarCfg.URL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	resp, err := browserAutomationHTTPClient.Do(req)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseSize))
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{"status": "error", "message": "invalid sidecar health response"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if result == nil {
			result = map[string]interface{}{}
		}
		if _, ok := result["status"]; !ok {
			result["status"] = "error"
		}
		if _, ok := result["message"]; !ok {
			result["message"] = fmt.Sprintf("sidecar health returned HTTP %d", resp.StatusCode)
		}
	}
	return result
}

func browserAutomationRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errText := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"eof",
		"no such host",
		"server misbehaving",
		"timeout",
		"temporarily unavailable",
	} {
		if strings.Contains(errText, marker) {
			return true
		}
	}
	return false
}

func browserAutomationSleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func ResolveBrowserAutomationSidecarConfig(cfg *config.Config) (BrowserAutomationSidecarConfig, error) {
	return browserAutomationSidecarConfig(cfg)
}

func browserAutomationSidecarConfig(cfg *config.Config) (BrowserAutomationSidecarConfig, error) {
	workspaceRoot, err := browserAutomationResolveWorkspaceRoot(cfg)
	if err != nil {
		return BrowserAutomationSidecarConfig{}, fmt.Errorf("resolve workspace dir: %w", err)
	}
	downloadsRoot, err := browserAutomationResolveDownloadsRoot(cfg, workspaceRoot)
	if err != nil {
		return BrowserAutomationSidecarConfig{}, fmt.Errorf("resolve download dir: %w", err)
	}
	return BrowserAutomationSidecarConfig{
		URL:            cfg.BrowserAutomation.URL,
		Image:          cfg.BrowserAutomation.Image,
		ContainerName:  cfg.BrowserAutomation.ContainerName,
		AutoBuild:      cfg.BrowserAutomation.AutoBuild,
		DockerfileDir:  cfg.BrowserAutomation.DockerfileDir,
		SessionTTL:     cfg.BrowserAutomation.SessionTTLMinutes,
		MaxSessions:    cfg.BrowserAutomation.MaxSessions,
		Headless:       cfg.BrowserAutomation.Headless,
		AllowUploads:   cfg.BrowserAutomation.AllowFileUploads,
		AllowDownloads: cfg.BrowserAutomation.AllowFileDownloads,
		ReadOnly:       cfg.BrowserAutomation.ReadOnly,
		WorkspaceDir:   workspaceRoot,
		DownloadDir:    downloadsRoot,
		ViewportWidth:  cfg.BrowserAutomation.Viewport.Width,
		ViewportHeight: cfg.BrowserAutomation.Viewport.Height,
	}, nil
}

func ExecuteBrowserAutomation(ctx context.Context, cfg *config.Config, req BrowserAutomationRequest, logger *slog.Logger) string {
	if cfg == nil {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "message": "config is required"})
	}
	if !cfg.BrowserAutomation.Enabled || !cfg.Tools.BrowserAutomation.Enabled {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "message": "browser_automation is disabled"})
	}

	op := strings.TrimSpace(req.Operation)
	if op == "" {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "message": "operation is required"})
	}
	if browserAutomationNeedsSession(op) && strings.TrimSpace(req.SessionID) == "" {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "message": "session_id is required"})
	}
	if cfg.BrowserAutomation.ReadOnly && browserAutomationReadOnlyBlocked(op) {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": "browser_automation is in read-only mode"})
	}
	if op == "upload_file" && !cfg.BrowserAutomation.AllowFileUploads {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": "file uploads are disabled"})
	}
	if (op == "list_downloads" || op == "get_download") && !cfg.BrowserAutomation.AllowFileDownloads {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": "file downloads are disabled"})
	}

	sidecarCfg, err := browserAutomationSidecarConfig(cfg)
	if err != nil {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": err.Error()})
	}
	workspaceRoot := sidecarCfg.WorkspaceDir
	downloadsRoot := sidecarCfg.DownloadDir

	payload := map[string]interface{}{
		"operation":    op,
		"session_id":   req.SessionID,
		"url":          req.URL,
		"selector":     req.Selector,
		"text":         req.Text,
		"value":        req.Value,
		"key":          req.Key,
		"wait_for":     req.WaitFor,
		"timeout_ms":   req.TimeoutMs,
		"full_page":    req.FullPage,
		"dom_snippet":  req.DOMSnippet,
		"max_elements": req.MaxElements,
	}

	if op == "screenshot" {
		outputPath := strings.TrimSpace(req.OutputPath)
		if outputPath == "" {
			outputPath = browserAutomationDefaultScreenshotRel(req)
		}
		relPath, _, relErr := browserAutomationRelFromWorkspace(workspaceRoot, outputPath)
		if relErr != nil {
			return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": relErr.Error()})
		}
		payload["output_path"] = relPath
	}

	if op == "upload_file" {
		relPath, _, relErr := browserAutomationRelFromWorkspace(workspaceRoot, req.FilePath)
		if relErr != nil {
			return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": relErr.Error()})
		}
		payload["file_path"] = relPath
	}

	if op == "get_download" {
		payload["download_name"] = req.DownloadName
	}

	resp, err := browserAutomationSidecarRequest(ctx, sidecarCfg, payload)
	if err != nil {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": err.Error()})
	}

	if screenshotRel, ok := resp["screenshot_rel_path"].(string); ok && screenshotRel != "" {
		abs := filepath.Join(workspaceRoot, filepath.FromSlash(screenshotRel))
		resp["screenshot_path"] = abs
		if webPath := browserAutomationWebPath(workspaceRoot, abs); webPath != "" {
			resp["screenshot_web_path"] = webPath
		}
	}

	if downloadRel, ok := resp["download_rel_path"].(string); ok && downloadRel != "" {
		abs := filepath.Join(downloadsRoot, filepath.FromSlash(downloadRel))
		resp["downloaded_file"] = abs
		if webPath := browserAutomationWebPath(workspaceRoot, abs); webPath != "" {
			resp["downloaded_file_web_path"] = webPath
		}
	}

	if rawDownloads, ok := resp["downloads"].([]interface{}); ok {
		for _, raw := range rawDownloads {
			entry, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			relPath, _ := entry["rel_path"].(string)
			if relPath == "" {
				continue
			}
			abs := filepath.Join(downloadsRoot, filepath.FromSlash(relPath))
			entry["local_path"] = abs
			if webPath := browserAutomationWebPath(workspaceRoot, abs); webPath != "" {
				entry["web_path"] = webPath
			}
		}
	}

	if status, _ := resp["status"].(string); status == "" {
		resp["status"] = "success"
	}
	if _, ok := resp["operation"]; !ok {
		resp["operation"] = op
	}
	return browserAutomationJSON(resp)
}

func EnsureBrowserAutomationSidecarRunning(dockerHost string, sidecarCfg BrowserAutomationSidecarConfig, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	managedHost := browserAutomationManagedURLHost(sidecarCfg.URL, sidecarCfg.ContainerName, browserAutomationRunsInDocker())
	if managedHost == "" {
		logger.Info("[BrowserAutomation] Skipping auto-start because sidecar URL points to an external/container service", "url", sidecarCfg.URL)
		return
	}

	dockerCfg := DockerConfig{Host: dockerHost}

	image := sidecarCfg.Image
	if image == "" {
		image = browserAutomationImage
	}
	containerName := browserAutomationEffectiveContainerName(sidecarCfg, managedHost)

	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+containerName+"/json", "")
	if err != nil {
		logger.Warn("[BrowserAutomation] Docker unavailable, skipping auto-start", "error", err)
		return
	}
	if code == 200 {
		var info map[string]interface{}
		if json.Unmarshal(data, &info) == nil {
			if state, ok := info["State"].(map[string]interface{}); ok {
				if running, _ := state["Running"].(bool); running {
					logger.Info("[BrowserAutomation] Sidecar container already running")
					return
				}
			}
		}
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/start", "")
		if startErr != nil || (startCode != 204 && startCode != 304) {
			logger.Error("[BrowserAutomation] Failed to start existing sidecar container", "code", startCode, "error", startErr)
			return
		}
		logger.Info("[BrowserAutomation] Sidecar container started")
		return
	}
	if code != 404 {
		logger.Warn("[BrowserAutomation] Unexpected Docker inspect response, skipping auto-start", "code", code)
		return
	}

	if _, imgCode, imgErr := dockerRequest(dockerCfg, "GET", "/images/"+image+"/json", ""); imgErr != nil || imgCode != 200 {
		if sidecarCfg.AutoBuild {
			if err := buildBrowserAutomationImage(image, sidecarCfg.DockerfileDir, logger); err != nil {
				logger.Error("[BrowserAutomation] Auto-build failed", "image", image, "error", err)
				return
			}
		} else {
			logger.Warn("[BrowserAutomation] Image not found locally", "image", image)
			return
		}
	}

	_ = os.MkdirAll(sidecarCfg.WorkspaceDir, 0o750)
	_ = os.MkdirAll(sidecarCfg.DownloadDir, 0o750)
	env := []string{
		"PORT=7331",
		fmt.Sprintf("SESSION_TTL_MINUTES=%d", max(1, sidecarCfg.SessionTTL)),
		fmt.Sprintf("MAX_SESSIONS=%d", max(1, sidecarCfg.MaxSessions)),
		fmt.Sprintf("HEADLESS=%t", sidecarCfg.Headless),
		fmt.Sprintf("ALLOW_FILE_UPLOADS=%t", sidecarCfg.AllowUploads),
		fmt.Sprintf("ALLOW_FILE_DOWNLOADS=%t", sidecarCfg.AllowDownloads),
		fmt.Sprintf("READ_ONLY=%t", sidecarCfg.ReadOnly),
		"WORKSPACE_ROOT=" + browserAutomationWorkspaceDir,
		"DOWNLOAD_ROOT=" + browserAutomationDownloadsDir,
		fmt.Sprintf("VIEWPORT_WIDTH=%d", max(320, sidecarCfg.ViewportWidth)),
		fmt.Sprintf("VIEWPORT_HEIGHT=%d", max(240, sidecarCfg.ViewportHeight)),
	}

	payload := map[string]interface{}{
		"Image": image,
		"Env":   env,
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"Memory":        int64(1024 * 1024 * 1024),
			"NanoCpus":      int64(1_000_000_000),
			"Binds": []string{
				sidecarCfg.WorkspaceDir + ":" + browserAutomationWorkspaceDir,
				sidecarCfg.DownloadDir + ":" + browserAutomationDownloadsDir,
			},
		},
		"ExposedPorts": map[string]interface{}{
			fmt.Sprintf("%d/tcp", browserAutomationContainerPort): struct{}{},
		},
	}
	hostConfig, _ := payload["HostConfig"].(map[string]interface{})
	if browserAutomationIsLoopbackHost(managedHost) {
		hostConfig["PortBindings"] = map[string]interface{}{
			fmt.Sprintf("%d/tcp", browserAutomationContainerPort): []map[string]string{{"HostIp": "127.0.0.1", "HostPort": fmt.Sprintf("%d", browserAutomationContainerPort)}},
		}
	} else {
		networkName, networkErr := browserAutomationCurrentContainerNetwork(dockerCfg)
		if networkErr != nil {
			logger.Error("[BrowserAutomation] Failed to resolve current Docker network for managed sidecar", "error", networkErr, "url", sidecarCfg.URL)
			return
		}
		hostConfig["NetworkMode"] = networkName
		payload["NetworkingConfig"] = map[string]interface{}{
			"EndpointsConfig": map[string]interface{}{
				networkName: map[string]interface{}{
					"Aliases": []string{managedHost},
				},
			},
		}
	}
	body, _ := json.Marshal(payload)
	_, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+url.QueryEscape(containerName), string(body))
	if createErr != nil || createCode != 201 {
		logger.Error("[BrowserAutomation] Failed to create sidecar container", "code", createCode, "error", createErr)
		return
	}
	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		logger.Error("[BrowserAutomation] Failed to start new sidecar container", "code", startCode, "error", startErr)
		return
	}
	logger.Info("[BrowserAutomation] Sidecar container created and started", "image", image, "container", containerName)
}

// StopBrowserAutomationSidecar stops and removes the managed sidecar container.
// This is used when config changes require a container restart with new environment
// variables (e.g. viewport, session TTL, read-only mode).
func StopBrowserAutomationSidecar(dockerHost string, sidecarCfg BrowserAutomationSidecarConfig, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	managedHost := browserAutomationManagedURLHost(sidecarCfg.URL, sidecarCfg.ContainerName, browserAutomationRunsInDocker())
	if managedHost == "" {
		return
	}
	containerName := browserAutomationEffectiveContainerName(sidecarCfg, managedHost)
	dockerCfg := DockerConfig{Host: dockerHost}

	// Stop the container (ignore errors if it's not running).
	_, _, _ = dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/stop?t=5", "")

	// Remove the container so the next EnsureBrowserAutomationSidecarRunning call
	// recreates it with updated env vars and binds.
	_, removeCode, removeErr := dockerRequest(dockerCfg, "DELETE", "/containers/"+containerName+"?force=true", "")
	if removeErr != nil {
		logger.Warn("[BrowserAutomation] Failed to remove old sidecar container", "error", removeErr)
	} else if removeCode == 204 || removeCode == 404 {
		logger.Info("[BrowserAutomation] Old sidecar container removed", "container", containerName)
	}
}

func buildBrowserAutomationImage(image, dockerfileDir string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if dockerfileDir == "" {
		dockerfileDir = "."
	}
	logger.Info("[BrowserAutomation] Building sidecar image (this may take a few minutes)…", "image", image, "context", dockerfileDir)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build",
		"-f", filepath.Join(dockerfileDir, "Dockerfile.browser_automation"),
		"-t", image,
		dockerfileDir,
	)
	dockerCfgDir := filepath.Join(dockerfileDir, "data", ".docker")
	_ = os.MkdirAll(dockerCfgDir, 0o700)
	cmd.Env = append(os.Environ(), "DOCKER_CONFIG="+dockerCfgDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	logger.Info("[BrowserAutomation] Image built successfully", "image", image)
	return nil
}

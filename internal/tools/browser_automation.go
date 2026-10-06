package tools

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"aurago/internal/dockerutil"
	"aurago/internal/sandbox"
	"aurago/internal/security"
)

const (
	browserAutomationContainerName = "aurago_browser_automation"
	browserAutomationImage         = "aurago-browser-automation:latest"
	browserAutomationContainerPort = 7331
	browserAutomationWorkspaceDir  = "/workspace"
	browserAutomationDownloadsDir  = "/downloads"
)

type BrowserAutomationRequest struct {
	Operation            string
	SessionID            string
	URL                  string
	Selector             string
	Text                 string
	Value                string
	Key                  string
	WaitFor              string
	TimeoutMs            int
	OutputPath           string
	FullPage             bool
	FilePath             string
	DownloadName         string
	DOMSnippet           bool
	MaxElements          int
	CloakHumanize        bool
	CloakProxy           string
	CloakFingerprintSeed string
}

type BrowserAutomationSidecarConfig struct {
	URL                   string
	Image                 string
	ContainerName         string
	AuthToken             string
	HTTPClient            *http.Client
	AutoBuild             bool
	DockerfileDir         string
	SessionTTL            int
	MaxSessions           int
	Headless              bool
	AllowUploads          bool
	AllowDownloads        bool
	ReadOnly              bool
	WorkspaceDir          string
	DownloadDir           string
	ViewportWidth         int
	ViewportHeight        int
	CloakHumanize         bool
	CloakHumanPreset      string
	CloakProxy            string
	EgressNetwork         string
	AllowedPrivateOrigins []string
	CloakFingerprintSeed  string
	RuntimeIsDocker       bool // cfg.Runtime.IsDocker or /.dockerenv; only words the remote-build warning
}

var browserAutomationDefaultHTTPClient = &http.Client{Timeout: 60 * time.Second}

var browserAutomationRetryDelays = []time.Duration{
	250 * time.Millisecond,
	750 * time.Millisecond,
	1500 * time.Millisecond,
	3000 * time.Millisecond,
}

func browserAutomationHTTPClientFor(cfg BrowserAutomationSidecarConfig) *http.Client {
	base := browserAutomationDefaultHTTPClient
	if cfg.HTTPClient != nil {
		base = cfg.HTTPClient
	}
	client := *base
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &client
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

func browserAutomationValidateRequest(req BrowserAutomationRequest) error {
	op := strings.TrimSpace(req.Operation)
	switch op {
	case "create_session", "close_session", "extract", "current_state", "screenshot", "list_downloads", "get_download":
		return nil
	case "navigate":
		if strings.TrimSpace(req.URL) == "" {
			return fmt.Errorf("url is required for navigate")
		}
		return nil
	case "click", "type", "upload_file":
		if strings.TrimSpace(req.Selector) == "" {
			return fmt.Errorf("selector is required for %s", op)
		}
		return nil
	case "select":
		if strings.TrimSpace(req.Selector) == "" {
			return fmt.Errorf("selector is required for select")
		}
		if strings.TrimSpace(req.Value) == "" {
			return fmt.Errorf("value is required for select")
		}
		return nil
	case "press":
		return nil
	case "wait_for":
		waitFor := strings.TrimSpace(req.WaitFor)
		switch waitFor {
		case "visible", "hidden", "attached", "detached":
			if strings.TrimSpace(req.Selector) == "" {
				return fmt.Errorf("selector is required for wait_for state %s", waitFor)
			}
			return nil
		case "load", "networkidle":
			return nil
		case "":
			return fmt.Errorf("wait_for is required for wait_for")
		default:
			return fmt.Errorf("unsupported wait_for state: %s", waitFor)
		}
	default:
		return fmt.Errorf("unsupported operation: %s", op)
	}
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
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return nil, fmt.Errorf("browser automation requires a sidecar token")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("browser automation URL is not configured")
	}
	httpClient := browserAutomationHTTPClientFor(cfg)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal sidecar payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/automation", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create sidecar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(cfg.AuthToken); token != "" {
		req.Header.Set("X-AuraGo-Sidecar-Token", token)
	}

	var (
		resp   *http.Response
		reqErr error
	)
	for attempt := 0; attempt <= len(browserAutomationRetryDelays); attempt++ {
		clonedReq := req.Clone(ctx)
		clonedReq.Body = io.NopCloser(bytes.NewReader(data))
		resp, reqErr = httpClient.Do(clonedReq)
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
	httpClient := browserAutomationHTTPClientFor(sidecarCfg)
	baseURL := strings.TrimRight(sidecarCfg.URL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	if token := strings.TrimSpace(sidecarCfg.AuthToken); token != "" {
		req.Header.Set("X-AuraGo-Sidecar-Token", token)
	}
	resp, err := httpClient.Do(req)
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
	if result["policy_version"] != "egress-v1" || result["egress_isolated"] != true {
		return map[string]interface{}{"status": "error", "message": "sidecar does not attest the required egress policy"}
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

func browserAutomationAuthToken(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if token := strings.TrimSpace(os.Getenv("AURAGO_BROWSER_AUTOMATION_TOKEN")); token != "" {
		security.RegisterSensitive(token)
		return token
	}
	masterKey := strings.TrimSpace(cfg.Server.MasterKey)
	if masterKey == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(masterKey))
	_, _ = mac.Write([]byte("aurago/browser-automation-sidecar/v1"))
	token := hex.EncodeToString(mac.Sum(nil))
	security.RegisterSensitive(token)
	return token
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
	runningInDocker := cfg.Runtime.IsDocker || browserAutomationRunsInDocker()
	return BrowserAutomationSidecarConfig{
		URL:                   config.NormalizeLegacySidecarURL(cfg.BrowserAutomation.URL, runningInDocker, "browser-automation", browserAutomationContainerPort),
		Image:                 cfg.BrowserAutomation.Image,
		ContainerName:         cfg.BrowserAutomation.ContainerName,
		AuthToken:             browserAutomationAuthToken(cfg),
		AutoBuild:             cfg.BrowserAutomation.AutoBuild,
		DockerfileDir:         cfg.BrowserAutomation.DockerfileDir,
		SessionTTL:            cfg.BrowserAutomation.SessionTTLMinutes,
		MaxSessions:           cfg.BrowserAutomation.MaxSessions,
		Headless:              cfg.BrowserAutomation.Headless,
		AllowUploads:          cfg.BrowserAutomation.AllowFileUploads,
		AllowDownloads:        cfg.BrowserAutomation.AllowFileDownloads,
		ReadOnly:              cfg.BrowserAutomation.ReadOnly,
		WorkspaceDir:          workspaceRoot,
		DownloadDir:           downloadsRoot,
		ViewportWidth:         cfg.BrowserAutomation.Viewport.Width,
		ViewportHeight:        cfg.BrowserAutomation.Viewport.Height,
		CloakHumanize:         cfg.BrowserAutomation.CloakHumanize,
		CloakHumanPreset:      cfg.BrowserAutomation.CloakHumanPreset,
		CloakProxy:            cfg.BrowserAutomation.CloakProxy,
		EgressNetwork:         cfg.BrowserAutomation.EgressNetwork,
		AllowedPrivateOrigins: cfg.BrowserAutomation.AllowedPrivateOrigins,
		CloakFingerprintSeed:  cfg.BrowserAutomation.CloakFingerprintSeed,
		RuntimeIsDocker:       runningInDocker,
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
	if validationErr := browserAutomationValidateRequest(req); validationErr != nil {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": validationErr.Error()})
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

	if health := BrowserAutomationHealth(ctx, cfg); health["status"] != "success" {
		return browserAutomationJSON(map[string]interface{}{"status": "error", "operation": op, "message": "browser automation egress policy is unavailable"})
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
	if sidecarCfg.AuthToken == "" || sidecarCfg.CloakProxy == "" || sidecarCfg.EgressNetwork == "" {
		logger.Error("[BrowserAutomation] Managed sidecar requires a token, filtering proxy, and internal egress network")
		return
	}

	dockerCfg := DockerConfig{Host: dockerHost}
	if !browserAutomationInternalNetwork(dockerCfg, sidecarCfg.EgressNetwork) {
		logger.Error("[BrowserAutomation] Egress network is missing or is not Docker-internal", "network", sidecarCfg.EgressNetwork)
		return
	}

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
			host, _ := info["HostConfig"].(map[string]interface{})
			if host["NetworkMode"] != sidecarCfg.EgressNetwork {
				logger.Error("[BrowserAutomation] Existing sidecar uses an untrusted network")
				return
			}
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
			if err := buildBrowserAutomationImage(image, sidecarCfg.DockerfileDir, dockerHost, sidecarCfg.RuntimeIsDocker, logger); err != nil {
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
		"AURAGO_BROWSER_AUTOMATION_TOKEN=" + sidecarCfg.AuthToken,
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
		fmt.Sprintf("CLOAK_HUMANIZE=%t", sidecarCfg.CloakHumanize),
		fmt.Sprintf("CLOAK_HUMAN_PRESET=%s", sidecarCfg.CloakHumanPreset),
	}
	allowedPrivateOrigins, _ := json.Marshal(sidecarCfg.AllowedPrivateOrigins)
	env = append(env,
		"AURAGO_BROWSER_EGRESS_ISOLATED=true",
		"AURAGO_BROWSER_EGRESS_PROXY="+sidecarCfg.CloakProxy,
		"AURAGO_BROWSER_ALLOWED_PRIVATE_ORIGINS="+string(allowedPrivateOrigins),
	)
	if sidecarCfg.CloakFingerprintSeed != "" {
		env = append(env, "CLOAK_FINGERPRINT_SEED="+sidecarCfg.CloakFingerprintSeed)
	}

	hostConfig := browserAutomationManagedHostConfig(sidecarCfg)
	payload := map[string]interface{}{
		"Image":      image,
		"Env":        env,
		"HostConfig": hostConfig,
		"ExposedPorts": map[string]interface{}{
			fmt.Sprintf("%d/tcp", browserAutomationContainerPort): struct{}{},
		},
	}
	hostConfig["NetworkMode"] = sidecarCfg.EgressNetwork
	if browserAutomationIsLoopbackHost(managedHost) {
		hostConfig["PortBindings"] = map[string]interface{}{
			fmt.Sprintf("%d/tcp", browserAutomationContainerPort): []map[string]string{{"HostIp": "127.0.0.1", "HostPort": fmt.Sprintf("%d", browserAutomationContainerPort)}},
		}
	} else {
		networkName := sidecarCfg.EgressNetwork
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

func browserAutomationInternalNetwork(dockerCfg DockerConfig, name string) bool {
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/networks/"+url.PathEscape(name), "")
	if err != nil || code != http.StatusOK {
		return false
	}
	var info struct {
		Internal bool `json:"Internal"`
	}
	return json.Unmarshal(data, &info) == nil && info.Internal
}

func browserAutomationManagedHostConfig(sidecarCfg BrowserAutomationSidecarConfig) map[string]interface{} {
	return hardenManagedSidecarHostConfig(map[string]interface{}{
		"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
		"Memory":        int64(1024 * 1024 * 1024),
		"NanoCpus":      int64(1_000_000_000),
		"Binds": []string{
			dockerutil.FormatBindMount(sidecarCfg.WorkspaceDir, browserAutomationWorkspaceDir),
			dockerutil.FormatBindMount(sidecarCfg.DownloadDir, browserAutomationDownloadsDir),
		},
	})
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

// browserAutomationDockerfileName is the sidecar Dockerfile expected inside
// browser_automation.dockerfile_dir.
const browserAutomationDockerfileName = "Dockerfile.browser_automation"

// browserAutomationBuildInvocation is the docker CLI call that builds the
// managed sidecar image.
type browserAutomationBuildInvocation struct {
	Args       []string
	Env        []string
	ContextDir string
	Dockerfile string
	ConfigDir  string // DOCKER_CONFIG of the child
	DockerHost string // the engine this invocation targets
	// Fallback is the pre-K19 invocation: the inherited environment with only
	// DOCKER_CONFIG overridden. It is set only when that environment names a
	// different engine than DockerHost through DOCKER_HOST or the platform
	// default (a DOCKER_CONTEXT cannot be resolved with the DOCKER_CONFIG
	// override, so it never gets one), and only used when docker.host refuses the
	// build (see browserAutomationBuildFallback).
	Fallback *browserAutomationBuildInvocation
}

// browserAutomationBuildCommand derives the docker CLI arguments and
// environment. DOCKER_HOST is set to the endpoint that the image check and the
// container create use (docker.host, which falls back to the DOCKER_HOST
// environment at config load), so the image is built on the engine that runs
// it; an inherited DOCKER_HOST or DOCKER_CONTEXT is dropped. TLS variables stay
// inherited. Paths are absolute, so a directory name starting with "-" cannot
// be read as a flag. DOCKER_CONFIG stays under <dir>/data/.docker like the
// Ansible and Space Agent builds: systemd's ProtectHome makes the service
// user's home read-only, and AuraGo never logs in to a registry, so no
// credentials are written there.
//
// The returned invocation also carries Fallback, the pre-K19 call, for a socket
// proxy that serves docker.host with BUILD=0 while the CLI's own default engine
// can still build (see browserAutomationBuildFallback).
func browserAutomationBuildCommand(baseEnv []string, image, dockerfileDir, dockerHost string) (browserAutomationBuildInvocation, error) {
	dir := dockerfileDir
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	contextDir, err := filepath.Abs(dir)
	if err != nil {
		return browserAutomationBuildInvocation{}, fmt.Errorf("resolve browser_automation.dockerfile_dir %q: %w", dockerfileDir, err)
	}
	host := dockerutil.NormalizeHost(dockerHost)
	configDir := filepath.Join(contextDir, "data", ".docker")
	env := make([]string, 0, len(baseEnv)+2)
	legacyEnv := make([]string, 0, len(baseEnv)+1)
	for _, kv := range baseEnv {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(strings.TrimSpace(name)) {
		case "DOCKER_HOST", "DOCKER_CONTEXT":
			legacyEnv = append(legacyEnv, kv)
			continue
		case "DOCKER_CONFIG":
			continue
		}
		env = append(env, kv)
		legacyEnv = append(legacyEnv, kv)
	}
	env = append(env, "DOCKER_HOST="+host, "DOCKER_CONFIG="+configDir)
	legacyEnv = append(legacyEnv, "DOCKER_CONFIG="+configDir)
	dockerfile := filepath.Join(contextDir, browserAutomationDockerfileName)
	args := []string{"build", "-f", dockerfile, "-t", image, contextDir}
	inv := browserAutomationBuildInvocation{
		Args:       args,
		Env:        env,
		ContextDir: contextDir,
		Dockerfile: dockerfile,
		ConfigDir:  configDir,
		DockerHost: host,
	}
	if inherited, ok := browserAutomationInheritedEngine(baseEnv); ok && browserAutomationEngineURL(inherited) != browserAutomationEngineURL(host) {
		inv.Fallback = &browserAutomationBuildInvocation{
			Args:       args,
			Env:        legacyEnv,
			ContextDir: contextDir,
			Dockerfile: dockerfile,
			ConfigDir:  configDir,
			DockerHost: inherited,
		}
	}
	return inv, nil
}

// browserAutomationInheritedEngine returns the engine that the docker CLI
// reaches with the inherited environment: DOCKER_HOST, else the platform
// default socket. ok is false when DOCKER_CONTEXT selects a context other than
// "default" (the CLI lets it override DOCKER_HOST); the context store is looked
// up in DOCKER_CONFIG, which the build overrides, so that engine has no
// endpoint to retry on. Variable names match case-insensitively, as on Windows.
func browserAutomationInheritedEngine(env []string) (engine string, ok bool) {
	var host, contextName string
	for _, kv := range env {
		name, value, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(name)) {
		case "DOCKER_HOST":
			host = strings.TrimSpace(value)
		case "DOCKER_CONTEXT":
			contextName = strings.TrimSpace(value)
		}
	}
	if contextName != "" && !strings.EqualFold(contextName, "default") {
		return "", false
	}
	return dockerutil.NormalizeHost(host), true
}

// browserAutomationEngineURL normalizes an engine endpoint for comparison and
// parsing: surrounding space is trimmed, an empty value becomes the platform
// default, and a bare host:port gets the tcp:// scheme the docker CLI assumes.
func browserAutomationEngineURL(host string) string {
	host = dockerutil.NormalizeHost(host)
	if !strings.Contains(host, "://") {
		host = "tcp://" + host
	}
	return host
}

// browserAutomationRefusalMarkers are the texts of a refusal by a socket proxy
// that denies the build API (HAProxy in tecnativa/docker-socket-proxy with
// BUILD=0), e.g. `Error response from daemon: <html><body><h1>403
// Forbidden</h1>\nRequest forbidden by administrative rules.\n</body></html>`.
var browserAutomationRefusalMarkers = []string{"403 forbidden", "forbidden by administrative rules"}

// browserAutomationStepFailureMarkers show that the build ran and one of its
// own steps failed (BuildKit and the classic builder), so a 403 in the output
// came from a mirror or registry and not from the engine endpoint.
var browserAutomationStepFailureMarkers = []string{"did not complete successfully", "returned a non-zero code"}

// browserAutomationBuildRefused reports whether a failed build's output is a
// refusal of the build API by the engine endpoint and not the failure of a
// build step.
func browserAutomationBuildRefused(output string) bool {
	lower := strings.ToLower(output)
	for _, marker := range browserAutomationStepFailureMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	for _, marker := range browserAutomationRefusalMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// browserAutomationBuildFallback returns the invocation to retry once after a
// failed build, or nil. Before K19 the build ignored docker.host and used the
// CLI's inherited environment. A native install can point docker.host at a
// socket proxy with BUILD=0 (the sidecar is created through it) and still
// build through the default local socket, so a refused build, and only a
// refused build, is retried on that old path when it reaches another engine.
func browserAutomationBuildFallback(inv browserAutomationBuildInvocation, buildErr error, output string) *browserAutomationBuildInvocation {
	if buildErr == nil || inv.Fallback == nil || !browserAutomationBuildRefused(output) {
		return nil
	}
	return inv.Fallback
}

// browserAutomationCheckBuildContext stops before docker runs when the
// Dockerfile is missing, which the docker CLI rejects anyway. Release archives
// ship neither the Dockerfile nor browser_automation_sidecar/, so the error
// names the cause and then the fix instead of the CLI's context error.
func browserAutomationCheckBuildContext(inv browserAutomationBuildInvocation, image string) error {
	info, err := os.Stat(inv.Dockerfile)
	var cause string
	switch {
	case err == nil && info.Mode().IsRegular():
		return nil
	case err == nil:
		cause = fmt.Sprintf("%s is not a regular file", inv.Dockerfile)
	case errors.Is(err, os.ErrNotExist):
		cause = fmt.Sprintf("%s was not found", inv.Dockerfile)
	default:
		cause = fmt.Sprintf("%s cannot be read: %v", inv.Dockerfile, err)
	}
	return fmt.Errorf("browser automation auto-build cannot continue because %s. Release installs do not ship the sidecar sources, so set browser_automation.dockerfile_dir to an AuraGo source checkout or build the image %s manually", cause, image)
}

// browserAutomationRemoteBuildEndpoint reports a native install whose build
// context would cross the network in plain text: a tcp:// engine that is not
// on loopback. Inside Docker, tcp:// normally names the socket proxy of the
// local engine (and the AuraGo image ships no docker CLI), so no warning.
func browserAutomationRemoteBuildEndpoint(dockerHost string, runtimeIsDocker bool) bool {
	if runtimeIsDocker {
		return false
	}
	host := browserAutomationEngineURL(dockerHost)
	if !strings.HasPrefix(host, "tcp://") {
		return false
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return true
	}
	return !isLoopbackHostname(parsed.Hostname())
}

func buildBrowserAutomationImage(image, dockerfileDir, dockerHost string, runtimeIsDocker bool, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	inv, err := browserAutomationBuildCommand(sandbox.FilterEnv(os.Environ()), image, dockerfileDir, dockerHost)
	if err != nil {
		return err
	}
	if err := browserAutomationCheckBuildContext(inv, image); err != nil {
		return err
	}
	if browserAutomationRemoteBuildEndpoint(inv.DockerHost, runtimeIsDocker) {
		logger.Warn("[BrowserAutomation] Building on a remote Docker engine over plain TCP; the build context is sent unencrypted", "docker_host", inv.DockerHost, "context", inv.ContextDir)
	}
	logger.Info("[BrowserAutomation] Building sidecar image (this may take a few minutes)…", "image", image, "context", inv.ContextDir, "docker_host", inv.DockerHost)
	ctx, cancel := context.WithTimeout(context.Background(), browserAutomationBuildTimeout)
	defer cancel()

	if err := os.MkdirAll(inv.ConfigDir, 0o700); err != nil {
		logger.Warn("[BrowserAutomation] Could not create the docker CLI config directory; the build may fail on a read-only home", "dir", inv.ConfigDir, "error", err)
	}
	out, err := browserAutomationRunBuild(ctx, inv)
	if err != nil {
		firstOutput := strings.TrimSpace(string(out))
		retry := browserAutomationBuildFallback(inv, err, firstOutput)
		if retry == nil {
			return fmt.Errorf("docker build: %w\n%s", err, firstOutput)
		}
		if left := browserAutomationBuildTimeLeft(ctx); left < browserAutomationRetryMinRemaining {
			return fmt.Errorf("docker build: docker.host %s refused the build, but only %s of the build time is left, too little to retry on %s: %w\n%s", inv.DockerHost, left.Round(time.Second), retry.DockerHost, err, firstOutput)
		}
		if browserAutomationRemoteBuildEndpoint(retry.DockerHost, runtimeIsDocker) {
			logger.Warn("[BrowserAutomation] Retrying on a remote Docker engine over plain TCP; the build context is sent unencrypted", "docker_host", retry.DockerHost, "context", inv.ContextDir)
		}
		retryOut, retryErr := browserAutomationRunBuild(ctx, *retry)
		if retryErr != nil {
			return fmt.Errorf("docker build: docker.host %s refused the build and the retry on %s failed too: %w\n%s\nfirst attempt on docker.host:\n%s", inv.DockerHost, retry.DockerHost, retryErr, strings.TrimSpace(string(retryOut)), firstOutput)
		}
		// The image now exists on the fallback engine. Ensure creates the
		// container through docker.host, so the image has to be visible there.
		if _, code, reqErr := dockerRequest(DockerConfig{Host: dockerHost}, http.MethodGet, "/images/"+image+"/json", ""); reqErr != nil || code != http.StatusOK {
			return fmt.Errorf("the fallback built on %s, but docker.host %s still has no image %s (status %d, error: %v): it is a different engine, so build the image there or point docker.host at the engine that docker uses by default", retry.DockerHost, inv.DockerHost, image, code, reqErr)
		}
		logger.Warn(fmt.Sprintf("[BrowserAutomation] docker.host refused the build with a 403-style response; built through %s instead", retry.DockerHost), "docker_host", inv.DockerHost, "built_on", retry.DockerHost)
	}
	logger.Info("[BrowserAutomation] Image built successfully", "image", image)
	return nil
}

// browserAutomationBuildTimeout bounds one auto-build including its single
// retry. The two variables are package-level so tests can shrink them.
var (
	browserAutomationBuildTimeout = 15 * time.Minute
	// browserAutomationRetryMinRemaining is the build time that must be left for
	// the fallback retry to be worth starting.
	browserAutomationRetryMinRemaining = 2 * time.Minute
)

// browserAutomationBuildTimeLeft is the time until the build context expires.
func browserAutomationBuildTimeLeft(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Duration(1<<63 - 1)
	}
	return time.Until(deadline)
}

// browserAutomationRunBuild runs one docker build invocation and returns its
// combined output.
func browserAutomationRunBuild(ctx context.Context, inv browserAutomationBuildInvocation) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", inv.Args...)
	cmd.Env = inv.Env
	return cmd.CombinedOutput()
}

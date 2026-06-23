package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/sandbox"
)

const (
	spaceAgentDefaultRepoURL       = "https://github.com/agent0ai/space-agent"
	spaceAgentDefaultGitRef        = "main"
	spaceAgentDefaultImage         = "aurago-space-agent:main"
	spaceAgentDefaultContainerName = "aurago_space_agent"
	spaceAgentDefaultPort          = 3100
	spaceAgentImageBuildRevision   = "20260502-aurago-onscreen-reset"
	spaceAgentDataContainerPath    = "/app/.space-agent"
	spaceAgentHomePath             = "/app/home"
	spaceAgentSupervisorPath       = "/app/supervisor"
	spaceAgentCustomwarePath       = "/app/customware"
	spaceAgentBridgeEndpoint       = "/api/space-agent/bridge/messages"
)

var spaceAgentInstructionEndpoints = []string{"/api/message_async", "/api/message"}

// SpaceAgentSidecarConfig is the resolved runtime configuration for the managed sidecar.
type SpaceAgentSidecarConfig struct {
	RepoURL        string
	GitRef         string
	Image          string
	ContainerName  string
	Host           string
	Port           int
	SourcePath     string
	DataPath       string
	CustomwarePath string
	AdminUser      string
	AdminPassword  string
	BridgeURL      string
	BridgeToken    string
	PublicURL      string
}

// SpaceAgentInstruction is sent from AuraGo to Space Agent.
type SpaceAgentInstruction struct {
	Instruction string `json:"instruction"`
	Information string `json:"information,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

// ResolveSpaceAgentSidecarConfig resolves the managed Space Agent sidecar paths and URLs.
func ResolveSpaceAgentSidecarConfig(cfg *config.Config, bridgeBaseURL string) (SpaceAgentSidecarConfig, error) {
	if cfg == nil {
		return SpaceAgentSidecarConfig{}, fmt.Errorf("config is required")
	}
	dataDir := cfg.Directories.DataDir
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}
	sourcePath := filepath.Join(dataDir, "sidecars", "space-agent", "source")
	port := cfg.SpaceAgent.Port
	if port <= 0 {
		port = spaceAgentDefaultPort
	}
	repoURL := strings.TrimSpace(cfg.SpaceAgent.RepoURL)
	if repoURL == "" {
		repoURL = spaceAgentDefaultRepoURL
	}
	gitRef := strings.TrimSpace(cfg.SpaceAgent.GitRef)
	if gitRef == "" {
		gitRef = spaceAgentDefaultGitRef
	}
	image := strings.TrimSpace(cfg.SpaceAgent.Image)
	if image == "" {
		image = spaceAgentDefaultImage
	}
	containerName := strings.TrimSpace(cfg.SpaceAgent.ContainerName)
	if containerName == "" {
		containerName = spaceAgentDefaultContainerName
	}
	host := strings.TrimSpace(cfg.SpaceAgent.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	bridgeBaseURL = strings.TrimRight(strings.TrimSpace(bridgeBaseURL), "/")
	bridgeURL := ""
	if bridgeBaseURL != "" {
		bridgeURL = bridgeBaseURL + spaceAgentBridgeEndpoint
	}
	return SpaceAgentSidecarConfig{
		RepoURL:        repoURL,
		GitRef:         gitRef,
		Image:          image,
		ContainerName:  containerName,
		Host:           host,
		Port:           port,
		SourcePath:     sourcePath,
		DataPath:       cfg.SpaceAgent.DataPath,
		CustomwarePath: cfg.SpaceAgent.CustomwarePath,
		AdminUser:      cfg.SpaceAgent.AdminUser,
		AdminPassword:  cfg.SpaceAgent.AdminPassword,
		BridgeURL:      bridgeURL,
		BridgeToken:    cfg.SpaceAgent.BridgeToken,
		PublicURL:      cfg.SpaceAgent.PublicURL,
	}, nil
}

func buildSpaceAgentCreatePayload(cfg SpaceAgentSidecarConfig) ([]byte, error) {
	if err := validateDockerName(effectiveSpaceAgentContainerName(cfg)); err != nil {
		return nil, err
	}
	image := strings.TrimSpace(cfg.Image)
	if image == "" {
		image = spaceAgentDefaultImage
	}
	if err := validateDockerName(image); err != nil {
		return nil, err
	}
	port := cfg.Port
	if port <= 0 {
		port = spaceAgentDefaultPort
	}
	publishHost := spaceAgentPublishHost(cfg.Host)
	env := []string{
		"HOST=0.0.0.0",
		"PORT=" + strconv.Itoa(port),
		"HOME=" + spaceAgentHomePath,
		"XDG_CONFIG_HOME=" + spaceAgentHomePath + "/.config",
		"XDG_DATA_HOME=" + spaceAgentHomePath + "/.local/share",
		"CUSTOMWARE_PATH=" + spaceAgentCustomwarePath,
		"SPACE_AGENT_ADMIN_USER=" + strings.TrimSpace(cfg.AdminUser),
		"SPACE_AGENT_ADMIN_PASSWORD=" + cfg.AdminPassword,
		"AURAGO_BRIDGE_URL=" + strings.TrimSpace(cfg.BridgeURL),
		"AURAGO_BRIDGE_TOKEN=" + cfg.BridgeToken,
	}
	containerPort := fmt.Sprintf("%d/tcp", port)
	payload := map[string]interface{}{
		"Image":  image,
		"Env":    env,
		"Labels": map[string]string{"org.aurago.space-agent.build-revision": spaceAgentImageBuildRevision},
		"ExposedPorts": map[string]interface{}{
			containerPort: struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"Binds": []string{
				dockerutil.FormatBindMount(cfg.DataPath, spaceAgentDataContainerPath),
				dockerutil.FormatBindMount(filepath.Join(cfg.DataPath, "home"), spaceAgentHomePath),
				dockerutil.FormatBindMount(filepath.Join(cfg.DataPath, "supervisor"), spaceAgentSupervisorPath),
				dockerutil.FormatBindMount(cfg.CustomwarePath, spaceAgentCustomwarePath),
			},
			"PortBindings": map[string]interface{}{
				containerPort: []map[string]string{{"HostIp": publishHost, "HostPort": strconv.Itoa(port)}},
			},
		},
	}
	return json.Marshal(payload)
}

func spaceAgentPublishHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" {
		return "0.0.0.0"
	}
	return host
}

func effectiveSpaceAgentContainerName(cfg SpaceAgentSidecarConfig) string {
	name := strings.TrimSpace(cfg.ContainerName)
	if name == "" {
		return spaceAgentDefaultContainerName
	}
	return name
}

// EnsureSpaceAgentSidecarRunning creates and starts the managed Space Agent sidecar when needed.
func EnsureSpaceAgentSidecarRunning(dockerHost string, cfg SpaceAgentSidecarConfig, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	if strings.TrimSpace(cfg.AdminPassword) == "" || strings.TrimSpace(cfg.BridgeToken) == "" {
		logger.Warn("[SpaceAgent] Missing vault secrets, skipping auto-start")
		return
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	containerName := effectiveSpaceAgentContainerName(cfg)
	if err := writeSpaceAgentBridgeCustomware(cfg.CustomwarePath, cfg.AdminUser, cfg.BridgeURL, cfg.BridgeToken); err != nil {
		logger.Warn("[SpaceAgent] Host-side bridge customware seed skipped; container bootstrap will retry", "error", err)
	}
	if data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/containers/"+containerName+"/json", ""); err == nil && code == 200 {
		if spaceAgentContainerNeedsRecreate(data, cfg) {
			logger.Warn("[SpaceAgent] Existing sidecar container has outdated network settings; recreating")
			_, _, _ = dockerRequest(dockerCfg, http.MethodPost, "/containers/"+containerName+"/stop?t=5", "")
			_, _, _ = dockerRequest(dockerCfg, http.MethodDelete, "/containers/"+containerName+"?force=true", "")
		} else {
			_, startCode, startErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/"+containerName+"/start", "")
			if startErr != nil || (startCode != http.StatusNoContent && startCode != http.StatusNotModified) {
				logger.Error("[SpaceAgent] Failed to start existing sidecar container", "code", startCode, "error", startErr)
				return
			}
			logger.Info("[SpaceAgent] Sidecar container is running")
			return
		}
	} else if err != nil {
		logger.Warn("[SpaceAgent] Docker unavailable, skipping auto-start", "error", err)
		return
	} else if code != http.StatusNotFound {
		logger.Warn("[SpaceAgent] Docker inspect returned unexpected status; skipping auto-start", "code", code)
		return
	}

	if err := ensureSpaceAgentSourceAndImage(cfg, logger); err != nil {
		logger.Error("[SpaceAgent] Failed to prepare sidecar image", "error", err)
		return
	}
	body, err := buildSpaceAgentCreatePayload(cfg)
	if err != nil {
		logger.Error("[SpaceAgent] Invalid Docker create payload", "error", err)
		return
	}
	_, createCode, createErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/create?name="+url.QueryEscape(containerName), string(body))
	if createErr != nil || createCode != http.StatusCreated {
		logger.Error("[SpaceAgent] Failed to create sidecar container", "code", createCode, "error", createErr)
		return
	}
	_, startCode, startErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/"+containerName+"/start", "")
	if startErr != nil || (startCode != http.StatusNoContent && startCode != http.StatusNotModified) {
		logger.Error("[SpaceAgent] Failed to start new sidecar container", "code", startCode, "error", startErr)
		return
	}
	logger.Info("[SpaceAgent] Sidecar container created and started", "image", cfg.Image, "container", containerName)
}

func spaceAgentContainerNeedsRecreate(data []byte, cfg SpaceAgentSidecarConfig) bool {
	var info struct {
		Config struct {
			Env    []string          `json:"Env"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		HostConfig struct {
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return false
	}
	port := cfg.Port
	if port <= 0 {
		port = spaceAgentDefaultPort
	}
	if !spaceAgentEnvContains(info.Config.Env, "HOST=0.0.0.0") {
		return true
	}
	if !spaceAgentEnvContains(info.Config.Env, "CUSTOMWARE_PATH="+spaceAgentCustomwarePath) {
		return true
	}
	if !spaceAgentEnvContains(info.Config.Env, "HOME="+spaceAgentHomePath) {
		return true
	}
	if info.Config.Labels["org.aurago.space-agent.build-revision"] != spaceAgentImageBuildRevision {
		return true
	}
	if wantBridgeURL := strings.TrimSpace(cfg.BridgeURL); wantBridgeURL != "" && !spaceAgentEnvContains(info.Config.Env, "AURAGO_BRIDGE_URL="+wantBridgeURL) {
		return true
	}
	if wantBridgeToken := strings.TrimSpace(cfg.BridgeToken); wantBridgeToken != "" && !spaceAgentEnvContains(info.Config.Env, "AURAGO_BRIDGE_TOKEN="+wantBridgeToken) {
		return true
	}
	containerPort := fmt.Sprintf("%d/tcp", port)
	bindings := info.HostConfig.PortBindings[containerPort]
	if len(bindings) == 0 {
		return true
	}
	wantHost := spaceAgentPublishHost(cfg.Host)
	wantPort := strconv.Itoa(port)
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

func spaceAgentEnvContains(env []string, want string) bool {
	for _, value := range env {
		if value == want {
			return true
		}
	}
	return false
}

// RecreateSpaceAgentSidecar removes and recreates the managed sidecar.
func RecreateSpaceAgentSidecar(dockerHost string, cfg SpaceAgentSidecarConfig, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	containerName := effectiveSpaceAgentContainerName(cfg)
	dockerCfg := DockerConfig{Host: dockerHost}
	_, _, _ = dockerRequest(dockerCfg, http.MethodPost, "/containers/"+containerName+"/stop?t=5", "")
	_, _, _ = dockerRequest(dockerCfg, http.MethodDelete, "/containers/"+containerName+"?force=true", "")
	EnsureSpaceAgentSidecarRunning(dockerHost, cfg, logger)
}

// SpaceAgentDockerStatus inspects the managed sidecar container.
func SpaceAgentDockerStatus(dockerHost string, cfg SpaceAgentSidecarConfig) map[string]interface{} {
	out := map[string]interface{}{
		"status":         "starting",
		"enabled":        true,
		"container_name": effectiveSpaceAgentContainerName(cfg),
		"image":          cfg.Image,
		"url":            cfg.PublicURL,
		"port":           cfg.Port,
	}
	data, code, err := dockerRequest(DockerConfig{Host: dockerHost}, http.MethodGet, "/containers/"+effectiveSpaceAgentContainerName(cfg)+"/json", "")
	if err != nil {
		out["status"] = "starting"
		out["message"] = err.Error()
		return out
	}
	if code == http.StatusNotFound {
		out["status"] = "stopped"
		return out
	}
	if code != http.StatusOK {
		out["status"] = "unknown"
		out["message"] = fmt.Sprintf("docker inspect returned HTTP %d", code)
		return out
	}
	var info map[string]interface{}
	if err := json.Unmarshal(data, &info); err != nil {
		out["status"] = "unknown"
		out["message"] = err.Error()
		return out
	}
	if state, ok := info["State"].(map[string]interface{}); ok {
		if running, _ := state["Running"].(bool); running {
			out["status"] = "running"
			out["running"] = true
			out["local_url"] = spaceAgentLocalURL(cfg)
			if !spaceAgentLocalPortReachable(cfg) {
				out["status"] = "starting"
				out["message"] = "container is running, but the Space Agent HTTP port is not reachable from AuraGo yet"
				out["running"] = false
			}
			return out
		}
		if status, _ := state["Status"].(string); status != "" {
			out["status"] = status
		}
	}
	return out
}

func spaceAgentLocalURL(cfg SpaceAgentSidecarConfig) string {
	port := cfg.Port
	if port <= 0 {
		port = spaceAgentDefaultPort
	}
	return "http://" + net.JoinHostPort(spaceAgentLocalTargetHost(cfg.Host), strconv.Itoa(port))
}

func spaceAgentLocalTargetHost(host string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || host == "0.0.0.0" || host == "::" || strings.EqualFold(host, "localhost") || host == "::1" {
		return "127.0.0.1"
	}
	return host
}

func spaceAgentLocalPortReachable(cfg SpaceAgentSidecarConfig) bool {
	port := cfg.Port
	if port <= 0 {
		port = spaceAgentDefaultPort
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(spaceAgentLocalTargetHost(cfg.Host), strconv.Itoa(port)), 750*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// SendSpaceAgentInstruction forwards an AuraGo instruction to the managed Space Agent instruction endpoint.
func SendSpaceAgentInstruction(ctx context.Context, cfg *config.Config, req SpaceAgentInstruction) map[string]interface{} {
	if cfg == nil || !cfg.SpaceAgent.Enabled {
		return map[string]interface{}{"status": "disabled", "message": "Space Agent integration is disabled"}
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		return map[string]interface{}{"status": "error", "message": "instruction is required"}
	}
	base := strings.TrimRight(spaceAgentLocalURL(SpaceAgentSidecarConfig{
		Host: cfg.SpaceAgent.Host,
		Port: cfg.SpaceAgent.Port,
	}), "/")
	var last map[string]interface{}
	var tried []string
	for _, endpoint := range spaceAgentInstructionEndpoints {
		tried = append(tried, endpoint)
		body, _ := json.Marshal(req)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+endpoint, bytes.NewReader(body))
		if err != nil {
			return map[string]interface{}{"status": "error", "message": err.Error()}
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if cfg.SpaceAgent.BridgeToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+cfg.SpaceAgent.BridgeToken)
		}
		httpReq.Header.Set("X-AuraGo-Instruction", "1")
		resp, err := spaceAgentHTTPClient.Do(httpReq)
		if err != nil {
			return map[string]interface{}{"status": "error", "message": err.Error(), "endpoint": endpoint, "tried_endpoints": tried}
		}
		rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()
		parsed := parseSpaceAgentInstructionResponseBody(resp.StatusCode, rawBody)
		parsed["endpoint"] = endpoint
		parsed["tried_endpoints"] = append([]string(nil), tried...)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return parsed
		}
		parsed["status"] = "error"
		parsed["http_status"] = resp.StatusCode
		last = parsed
		if resp.StatusCode != http.StatusNotFound {
			return parsed
		}
	}
	annotateSpaceAgentInstructionHTTPError(last, http.StatusNotFound)
	if mailbox := writeSpaceAgentInstructionMailbox(cfg, req, last); mailbox != nil {
		return mailbox
	}
	return last
}

var spaceAgentHTTPClient = &http.Client{Timeout: 30 * time.Second}

func parseSpaceAgentInstructionResponseBody(statusCode int, body []byte) map[string]interface{} {
	var parsed map[string]interface{}
	trimmed := strings.TrimSpace(string(body))
	if trimmed != "" {
		if err := json.Unmarshal(body, &parsed); err == nil && parsed != nil {
			if _, ok := parsed["http_status"]; !ok {
				parsed["http_status"] = statusCode
			}
			if statusCode >= 200 && statusCode < 300 {
				if _, ok := parsed["status"]; !ok {
					parsed["status"] = "ok"
				}
			}
			return parsed
		}
	}
	parsed = map[string]interface{}{"status": "ok", "http_status": statusCode}
	if trimmed != "" {
		parsed["body"] = trimmed
		parsed["message"] = trimmed
	}
	return parsed
}

func annotateSpaceAgentInstructionHTTPError(result map[string]interface{}, statusCode int) {
	if result == nil || statusCode != http.StatusNotFound {
		return
	}
	result["message"] = "Space Agent is reachable, but none of AuraGo's instruction endpoints are available. This is not an offline/network error. Recreate the managed Space Agent sidecar from the current AuraGo build so the message_async bridge is injected. If this persists after a fresh recreate, the running Space Agent image does not expose the expected message API and only the Space-Agent-to-AuraGo bridge fast path is currently available."
	result["requires_recreate"] = true
	if endpoint, _ := result["endpoint"].(string); endpoint != "" {
		result["missing_endpoint"] = endpoint
	}
}

func writeSpaceAgentInstructionMailbox(cfg *config.Config, req SpaceAgentInstruction, endpointResult map[string]interface{}) map[string]interface{} {
	if cfg == nil {
		return nil
	}
	dataPath := strings.TrimSpace(cfg.SpaceAgent.DataPath)
	if dataPath == "" {
		return nil
	}
	record := map[string]interface{}{
		"type":              "aurago_instruction",
		"instruction":       strings.TrimSpace(req.Instruction),
		"information":       strings.TrimSpace(req.Information),
		"session_id":        strings.TrimSpace(req.SessionID),
		"source":            "aurago",
		"created_at":        time.Now().UTC().Format(time.RFC3339Nano),
		"delivery":          "mailbox",
		"delivery_target":   "space_agent_onscreen_prompt",
		"auto_execution":    false,
		"endpoint_result":   endpointResult,
		"pickup_hint":       "Open ~/aurago_inbox/latest_instruction.json in Space Agent and execute the instruction.",
		"processed_by_user": false,
	}
	inboxDirs := []string{filepath.Join(dataPath, "home", "aurago_inbox")}
	if userInboxDir := spaceAgentCustomwareInboxDir(cfg); userInboxDir != "" {
		inboxDirs = append(inboxDirs, userInboxDir)
	}
	written := make([]string, 0, len(inboxDirs))
	for _, inboxDir := range inboxDirs {
		if err := writeSpaceAgentInstructionMailboxRecord(inboxDir, record); err != nil {
			continue
		}
		written = append(written, filepath.Join(inboxDir, "latest_instruction.json"))
	}
	if len(written) == 0 {
		return nil
	}
	return map[string]interface{}{
		"status":                      "ok",
		"accepted":                    true,
		"delivered":                   "mailbox",
		"auto_execution":              false,
		"requires_space_agent_pickup": true,
		"message":                     "Space Agent has no inbound HTTP instruction API. AuraGo wrote the instruction to the managed Space Agent mailbox.",
		"mailbox_path":                written[0],
		"mailbox_paths":               written,
		"endpoint_result":             endpointResult,
	}
}

func spaceAgentCustomwareInboxDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	customwarePath := strings.TrimSpace(cfg.SpaceAgent.CustomwarePath)
	if customwarePath == "" {
		return ""
	}
	username := strings.TrimSpace(cfg.SpaceAgent.AdminUser)
	if username == "" {
		username = "admin"
	}
	normalizedUser, err := normalizeSpaceAgentEntityID(username)
	if err != nil {
		return ""
	}
	return filepath.Join(customwarePath, "L2", normalizedUser, "aurago_inbox")
}

func writeSpaceAgentInstructionMailboxRecord(inboxDir string, record map[string]interface{}) error {
	if err := os.MkdirAll(inboxDir, 0o700); err != nil {
		return err
	}
	serialized, err := json.Marshal(record)
	if err != nil {
		return err
	}
	pretty, _ := json.MarshalIndent(record, "", "  ")
	if err := os.WriteFile(filepath.Join(inboxDir, "latest_instruction.json"), append(pretty, '\n'), 0o600); err != nil {
		return err
	}
	instruction, _ := record["instruction"].(string)
	if err := os.WriteFile(filepath.Join(inboxDir, "latest_instruction.txt"), []byte(strings.TrimSpace(instruction)+"\n"), 0o600); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(inboxDir, "instructions.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	if _, err := logFile.Write(append(serialized, '\n')); err != nil {
		return err
	}
	if err := appendSpaceAgentOnscreenHistory(filepath.Dir(inboxDir), record); err != nil {
		return err
	}
	return nil
}

func appendSpaceAgentOnscreenHistory(userRoot string, record map[string]interface{}) error {
	historyDir := filepath.Join(userRoot, "hist")
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		return err
	}
	historyPath := filepath.Join(historyDir, "onscreen-agent.json")
	var history []map[string]interface{}
	if data, err := os.ReadFile(historyPath); err == nil && len(bytes.TrimSpace(data)) > 0 {
		_ = json.Unmarshal(data, &history)
	}
	messageID, _ := record["message_id"].(string)
	if strings.TrimSpace(messageID) == "" {
		messageID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	content, _ := record["message"].(string)
	if strings.TrimSpace(content) == "" {
		content, _ = record["instruction"].(string)
	}
	history = append(history, map[string]interface{}{
		"attachments": []interface{}{},
		"content":     strings.TrimSpace(content),
		"id":          "user-" + strconv.FormatInt(time.Now().UnixMilli(), 10) + "-aurago-" + strings.TrimSpace(messageID),
		"kind":        "aurago-instruction",
		"role":        "user",
	})
	pretty, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath, append(pretty, '\n'), 0o600)
}

// ExecuteSpaceAgent is the agent-facing wrapper for Space Agent communication.
func ExecuteSpaceAgent(ctx context.Context, cfg *config.Config, req SpaceAgentInstruction) string {
	result := SendSpaceAgentInstruction(ctx, cfg, req)
	raw, _ := json.Marshal(result)
	return string(raw)
}

func ensureSpaceAgentSourceAndImage(cfg SpaceAgentSidecarConfig, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.SourcePath), 0o750); err != nil {
		return fmt.Errorf("create sidecar dir: %w", err)
	}
	if err := os.MkdirAll(cfg.DataPath, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataPath, "supervisor"), 0o750); err != nil {
		return fmt.Errorf("create supervisor state dir: %w", err)
	}
	if err := ensureSpaceAgentHome(filepath.Join(cfg.DataPath, "home")); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.CustomwarePath, 0o750); err != nil {
		return fmt.Errorf("create customware dir: %w", err)
	}
	if err := writeSpaceAgentBridgeCustomware(cfg.CustomwarePath, cfg.AdminUser, cfg.BridgeURL, cfg.BridgeToken); err != nil {
		logger.Warn("[SpaceAgent] Host-side bridge customware seed skipped; container bootstrap will retry", "error", err)
	}
	if err := ensureSpaceAgentCustomwareUserHome(cfg.CustomwarePath, cfg.AdminUser); err != nil {
		logger.Info("[SpaceAgent] Host-side customware workspace seed skipped; container bootstrap will retry", "error", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.SourcePath, ".git")); os.IsNotExist(err) {
		if err := runSpaceAgentCommand(logger, filepath.Dir(cfg.SourcePath), "git", "clone", "--depth", "1", "--branch", cfg.GitRef, cfg.RepoURL, cfg.SourcePath); err != nil {
			return err
		}
	} else {
		_ = runSpaceAgentCommand(logger, cfg.SourcePath, "git", "fetch", "--depth", "1", "origin", cfg.GitRef)
		_ = runSpaceAgentCommand(logger, cfg.SourcePath, "git", "checkout", cfg.GitRef)
	}
	dockerfilePath := filepath.Join(cfg.SourcePath, "Dockerfile.aurago")
	if err := os.WriteFile(filepath.Join(cfg.SourcePath, "aurago_space_bootstrap.mjs"), []byte(spaceAgentBootstrapScript()), 0o600); err != nil {
		return fmt.Errorf("write aurago_space_bootstrap.mjs: %w", err)
	}
	if err := writeSpaceAgentInstructionsAPIEndpoint(cfg.SourcePath); err != nil {
		return err
	}
	if err := os.WriteFile(dockerfilePath, []byte(spaceAgentDockerfile()), 0o600); err != nil {
		return fmt.Errorf("write Dockerfile.aurago: %w", err)
	}
	return runSpaceAgentCommand(logger, cfg.SourcePath, "docker", "build", "-f", dockerfilePath, "-t", cfg.Image, cfg.SourcePath)
}

func writeSpaceAgentInstructionsAPIEndpoint(sourcePath string) error {
	stalePaths := []string{
		filepath.Join(sourcePath, "server", "api", "aurago_instructions.js"),
		filepath.Join(sourcePath, "api", "aurago", "instructions.py"),
		filepath.Join(sourcePath, "api", "aurago_instructions.py"),
		filepath.Join(sourcePath, "python", "api", "aurago_instructions.py"),
		filepath.Join(sourcePath, "python", "api", "message_async.py"),
	}
	for _, stalePath := range stalePaths {
		if err := os.Remove(stalePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale AuraGo instructions endpoint %s: %w", stalePath, err)
		}
	}
	instructionsAPIPath := filepath.Join(sourcePath, "server", "api", "message_async.js")
	if err := os.MkdirAll(filepath.Dir(instructionsAPIPath), 0o750); err != nil {
		return fmt.Errorf("create AuraGo instructions api dir: %w", err)
	}
	if err := os.WriteFile(instructionsAPIPath, []byte(spaceAgentInstructionsAPIEndpoint()), 0o600); err != nil {
		return fmt.Errorf("write AuraGo instructions api endpoint: %w", err)
	}
	return nil
}

func ensureSpaceAgentHome(homePath string) error {
	return ensureSpaceAgentWorkspaceFiles(homePath)
}

func ensureSpaceAgentCustomwareUserHome(customwarePath string, adminUser string) error {
	if strings.TrimSpace(customwarePath) == "" {
		return nil
	}
	username := strings.TrimSpace(adminUser)
	if username == "" {
		username = "admin"
	}
	normalizedUser, err := normalizeSpaceAgentEntityID(username)
	if err != nil {
		return fmt.Errorf("normalize Space Agent admin user: %w", err)
	}
	return ensureSpaceAgentWorkspaceFiles(filepath.Join(customwarePath, "L2", normalizedUser))
}

func normalizeSpaceAgentEntityID(value string) (string, error) {
	raw := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if raw == "" {
		return "", fmt.Errorf("entity id must not be empty")
	}
	if strings.Contains(raw, "/") {
		return "", fmt.Errorf("entity id must be a single path segment")
	}
	normalized := path.Clean(raw)
	if normalized == "" || normalized == "." {
		return "", fmt.Errorf("entity id must not be empty")
	}
	if normalized == ".." || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/") {
		return "", fmt.Errorf("entity id must be a single path segment")
	}
	return normalized, nil
}

func ensureSpaceAgentWorkspaceFiles(homePath string) error {
	for _, dir := range []string{
		homePath,
		filepath.Join(homePath, "meta"),
		filepath.Join(homePath, "spaces"),
		filepath.Join(homePath, "conf"),
		filepath.Join(homePath, "hist"),
		filepath.Join(homePath, "docs"),
		filepath.Join(homePath, "dashboard"),
		filepath.Join(homePath, "onscreen-agent"),
		filepath.Join(homePath, ".config"),
		filepath.Join(homePath, ".local", "share"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create Space Agent home dir %s: %w", dir, err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(homePath, "AGENTS.md"):                        spaceAgentAuraGoAgentsMarkdown(),
		filepath.Join(homePath, "conf", "aurago.system.include.md"): spaceAgentAuraGoSystemInclude(),
		filepath.Join(homePath, "docs", "aurago-bridge.md"):         spaceAgentAuraGoBridgeReadme(),
		spaceAgentInboxPollerPath(homePath):                         spaceAgentInboxPollerJS(),
	} {
		if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750); mkdirErr != nil {
			return fmt.Errorf("create Space Agent managed file dir %s: %w", filepath.Dir(path), mkdirErr)
		}
		if writeErr := os.WriteFile(path, []byte(content), 0o600); writeErr != nil {
			return fmt.Errorf("write Space Agent managed file %s: %w", path, writeErr)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(homePath, "meta", "login_hooks.json"):               "[]\n",
		filepath.Join(homePath, "conf", "dashboard.yaml"):                 "{}\n",
		filepath.Join(homePath, "conf", "onscreen-agent.yaml"):            "{}\n",
		filepath.Join(homePath, "hist", "onscreen-agent.json"):            "[]\n",
		filepath.Join(homePath, "dashboard", "prefs.json"):                "{}\n",
		filepath.Join(homePath, "dashboard", "dashboard-prefs.json"):      "{}\n",
		filepath.Join(homePath, "onscreen-agent", "config.json"):          "{}\n",
		filepath.Join(homePath, "onscreen-agent", "history.json"):         "[]\n",
		filepath.Join(homePath, "meta", "onscreen-agent-config.json"):     "{}\n",
		filepath.Join(homePath, "meta", "onscreen-agent-history.json"):    "[]\n",
		filepath.Join(homePath, "meta", "dashboard-prefs.json"):           "{}\n",
		filepath.Join(homePath, ".config", "dashboard-prefs.json"):        "{}\n",
		filepath.Join(homePath, ".config", "onscreen-agent-config.json"):  "{}\n",
		filepath.Join(homePath, ".config", "onscreen-agent-history.json"): "[]\n",
	} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if writeErr := os.WriteFile(path, []byte(content), 0o600); writeErr != nil {
				return fmt.Errorf("seed Space Agent home file %s: %w", path, writeErr)
			}
		} else if err != nil {
			return fmt.Errorf("stat Space Agent home file %s: %w", path, err)
		}
	}
	return nil
}

func runSpaceAgentCommand(logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}, dir string, name string, args ...string) error {
	logger.Info("[SpaceAgent] Running command", "command", name, "args", args, "dir", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	dockerCfgDir := filepath.Join(dir, "data", ".docker")
	_ = os.MkdirAll(dockerCfgDir, 0o700)
	cmd.Env = append(sandbox.FilterEnv(os.Environ()), "DOCKER_CONFIG="+dockerCfgDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
		logger.Info("[SpaceAgent] Command completed", "output", trimmed)
	}
	return nil
}

func writeSpaceAgentBridgeCustomware(dir string, adminUser string, bridgeURL string, bridgeToken string) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	rootFiles := map[string]string{
		filepath.Join(dir, "aurago_bridge.md"):          spaceAgentBridgeHelperReadme(),
		filepath.Join(dir, "aurago_bridge.js"):          spaceAgentBridgeHelperESM(bridgeURL, bridgeToken),
		filepath.Join(dir, "aurago_bridge.cjs"):         spaceAgentBridgeHelperCJS(bridgeURL, bridgeToken),
		filepath.Join(dir, "aurago_bridge_config.json"): spaceAgentBridgeConfigJSON(bridgeURL, bridgeToken),
	}
	if err := writeSpaceAgentBridgeFiles(rootFiles); err != nil {
		return err
	}

	if userDir, err := spaceAgentCustomwareUserDir(dir, adminUser); err == nil && userDir != "" {
		userFiles := map[string]string{
			filepath.Join(userDir, "aurago_bridge.md"):          spaceAgentBridgeHelperReadme(),
			filepath.Join(userDir, "aurago_bridge.js"):          spaceAgentBridgeHelperESM(bridgeURL, bridgeToken),
			filepath.Join(userDir, "aurago_bridge.cjs"):         spaceAgentBridgeHelperCJS(bridgeURL, bridgeToken),
			filepath.Join(userDir, "aurago_bridge_config.json"): spaceAgentBridgeConfigJSON(bridgeURL, bridgeToken),
		}
		_ = writeSpaceAgentBridgeFiles(userFiles)
	} else if err != nil {
		return err
	}
	return nil
}

func writeSpaceAgentBridgeFiles(files map[string]string) error {
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func spaceAgentCustomwareUserDir(customwarePath string, adminUser string) (string, error) {
	username := strings.TrimSpace(adminUser)
	if username == "" {
		username = "admin"
	}
	normalizedUser, err := normalizeSpaceAgentEntityID(username)
	if err != nil {
		return "", fmt.Errorf("normalize Space Agent admin user: %w", err)
	}
	return filepath.Join(customwarePath, "L2", normalizedUser), nil
}

func spaceAgentBridgeHelperReadme() string {
	return `# AuraGo Space Agent Bridge

This directory is mounted into the managed Space Agent container.

Use aurago_bridge.js from browser-style Space Agent code, or aurago_bridge.cjs from Node/CommonJS code, to send structured messages back to AuraGo.

Fast path: import the helper, call sendToAuraGo, and use response.answer immediately when present. A separate callback is not required for bridge questions.

ES module example:

` + "```js" + `
const { sendToAuraGo } = await import("/L2/admin/aurago_bridge.js");
const response = await sendToAuraGo({
  type: "question",
  summary: "Short title",
  content: "Question for AuraGo",
  source: "space-agent"
});
return response.answer || response;
` + "```" + `

CommonJS code can use /app/customware/aurago_bridge.cjs.
`
}

func spaceAgentBridgeConfigJSON(bridgeURL string, bridgeToken string) string {
	browserBridgeURL := strings.TrimSpace(bridgeURL)
	if spaceAgentBridgeURLUsesLoopback(browserBridgeURL) {
		browserBridgeURL = ""
	}
	payload := map[string]string{
		"bridge_url":                  browserBridgeURL,
		"bridge_token":                strings.TrimSpace(bridgeToken),
		"browser_bridge_url_strategy": "Import aurago_bridge.js; it derives https://aurago.../api/space-agent/bridge/messages from https://aurago-space-agent... at runtime.",
		"note":                        "Browser contexts should import aurago_bridge.js instead of reading process.env directly.",
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(data) + "\n"
}

func spaceAgentBridgeURLUsesLoopback(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.")
}

func spaceAgentBridgeHelperESM(bridgeURL string, bridgeToken string) string {
	return `const EMBEDDED_BRIDGE_URL = ` + strconv.Quote(strings.TrimSpace(bridgeURL)) + `;
const EMBEDDED_BRIDGE_TOKEN = ` + strconv.Quote(strings.TrimSpace(bridgeToken)) + `;

function envValue(name) {
  if (typeof process !== "undefined" && process.env && process.env[name]) {
    return process.env[name];
  }
  if (typeof globalThis !== "undefined" && globalThis[name]) {
    return globalThis[name];
  }
  return "";
}

function deriveBrowserAuraGoBridgeURL() {
  if (typeof window === "undefined" || !window.location || !window.location.hostname) {
    return "";
  }
  const host = String(window.location.hostname || "");
  const labels = host.split(".");
  if (!labels[0] || !labels[0].endsWith("-space-agent")) {
    return "";
  }
  labels[0] = labels[0].slice(0, -"-space-agent".length) || "aurago";
  return window.location.protocol + "//" + labels.join(".") + "/api/space-agent/bridge/messages";
}

function uniqueNonEmpty(values) {
  return [...new Set(values.filter((value) => typeof value === "string" && value.trim() !== ""))];
}

function isLoopbackBridgeURL(value) {
  try {
    const url = new URL(value);
    const host = url.hostname.toLowerCase().replace(/^\[|\]$/g, "");
    return host === "localhost" || host === "::1" || host === "127.0.0.1" || host.startsWith("127.");
  } catch {
    return false;
  }
}

function bridgeUrlCandidates(options = {}) {
  const candidates = uniqueNonEmpty([
    options.bridgeUrl,
    deriveBrowserAuraGoBridgeURL(),
    envValue("AURAGO_BRIDGE_URL"),
    EMBEDDED_BRIDGE_URL
  ]);
  if (typeof window === "undefined") {
    return candidates;
  }
  return candidates.filter((candidate) => !isLoopbackBridgeURL(candidate));
}

function bridgeConfig(options = {}) {
  return {
    bridgeUrlCandidates: bridgeUrlCandidates(options),
    bridgeToken: options.bridgeToken || envValue("AURAGO_BRIDGE_TOKEN") || EMBEDDED_BRIDGE_TOKEN
  };
}

function buildAuraGoBridgePayload(message = {}) {
  return {
    type: message.type || "message",
    summary: message.summary || "",
    content: message.content || "",
    source: message.source || "space-agent",
    timestamp: message.timestamp || new Date().toISOString(),
    session_id: message.session_id || undefined
  };
}

export async function sendToAuraGo(message = {}, options = {}) {
  const { bridgeUrlCandidates, bridgeToken } = bridgeConfig(options);
  if (!bridgeUrlCandidates.length || !bridgeToken) {
    throw new Error("AuraGo bridge is not configured. Pass { bridgeUrl, bridgeToken } or recreate the managed Space Agent sidecar.");
  }
  const payload = JSON.stringify(buildAuraGoBridgePayload(message));
  let lastError;
  for (const bridgeUrl of bridgeUrlCandidates) {
    try {
      const res = await fetch(bridgeUrl, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": "Bearer " + bridgeToken
        },
        body: payload
      });
      if (res.ok) {
        return res.json();
      }
      lastError = new Error("AuraGo bridge returned HTTP " + res.status + " for " + bridgeUrl);
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError || new Error("AuraGo bridge request failed.");
}

export default { sendToAuraGo };
`
}

func spaceAgentBridgeHelperCJS(bridgeURL string, bridgeToken string) string {
	return `'use strict';

const EMBEDDED_BRIDGE_URL = ` + strconv.Quote(strings.TrimSpace(bridgeURL)) + `;
const EMBEDDED_BRIDGE_TOKEN = ` + strconv.Quote(strings.TrimSpace(bridgeToken)) + `;

function envValue(name) {
  if (typeof process !== 'undefined' && process.env && process.env[name]) {
    return process.env[name];
  }
  if (typeof globalThis !== 'undefined' && globalThis[name]) {
    return globalThis[name];
  }
  return '';
}

function deriveBrowserAuraGoBridgeURL() {
  if (typeof window === 'undefined' || !window.location || !window.location.hostname) {
    return '';
  }
  const host = String(window.location.hostname || '');
  const labels = host.split('.');
  if (!labels[0] || !labels[0].endsWith('-space-agent')) {
    return '';
  }
  labels[0] = labels[0].slice(0, -'-space-agent'.length) || 'aurago';
  return window.location.protocol + '//' + labels.join('.') + '/api/space-agent/bridge/messages';
}

function uniqueNonEmpty(values) {
  return [...new Set(values.filter((value) => typeof value === 'string' && value.trim() !== ''))];
}

function isLoopbackBridgeURL(value) {
  try {
    const url = new URL(value);
    const host = url.hostname.toLowerCase().replace(/^\[|\]$/g, '');
    return host === 'localhost' || host === '::1' || host === '127.0.0.1' || host.startsWith('127.');
  } catch {
    return false;
  }
}

function bridgeUrlCandidates(options = {}) {
  const candidates = uniqueNonEmpty([
    options.bridgeUrl,
    deriveBrowserAuraGoBridgeURL(),
    envValue('AURAGO_BRIDGE_URL'),
    EMBEDDED_BRIDGE_URL
  ]);
  if (typeof window === 'undefined') {
    return candidates;
  }
  return candidates.filter((candidate) => !isLoopbackBridgeURL(candidate));
}

function bridgeConfig(options = {}) {
  return {
    bridgeUrlCandidates: bridgeUrlCandidates(options),
    bridgeToken: options.bridgeToken || envValue('AURAGO_BRIDGE_TOKEN') || EMBEDDED_BRIDGE_TOKEN
  };
}

function buildAuraGoBridgePayload(message = {}) {
  return {
    type: message.type || 'message',
    summary: message.summary || '',
    content: message.content || '',
    source: message.source || 'space-agent',
    timestamp: message.timestamp || new Date().toISOString(),
    session_id: message.session_id || undefined
  };
}

async function sendToAuraGo(message = {}, options = {}) {
  const { bridgeUrlCandidates, bridgeToken } = bridgeConfig(options);
  if (!bridgeUrlCandidates.length || !bridgeToken) {
    throw new Error('AuraGo bridge is not configured. Pass { bridgeUrl, bridgeToken } or recreate the managed Space Agent sidecar.');
  }
  const payload = JSON.stringify(buildAuraGoBridgePayload(message));
  let lastError;
  for (const bridgeUrl of bridgeUrlCandidates) {
    try {
      const res = await fetch(bridgeUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + bridgeToken
        },
        body: payload
      });
      if (res.ok) {
        return res.json();
      }
      lastError = new Error('AuraGo bridge returned HTTP ' + res.status + ' for ' + bridgeUrl);
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError || new Error('AuraGo bridge request failed.');
}

module.exports = { sendToAuraGo };
`
}

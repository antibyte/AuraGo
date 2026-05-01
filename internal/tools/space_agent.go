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
	spaceAgentImageBuildRevision   = "20260501-password-crypto-guard"
	spaceAgentDataContainerPath    = "/app/.space-agent"
	spaceAgentHomePath             = "/app/home"
	spaceAgentSupervisorPath       = "/app/supervisor"
	spaceAgentCustomwarePath       = "/app/customware"
	spaceAgentBridgeEndpoint       = "/api/space-agent/bridge/messages"
	spaceAgentInstructionEndpoint  = "/api/aurago/instructions"
)

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
	if err := writeSpaceAgentBridgeCustomware(cfg.CustomwarePath); err != nil {
		logger.Error("[SpaceAgent] Failed to write bridge customware", "error", err)
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

// SendSpaceAgentInstruction forwards an AuraGo instruction to the Space Agent bridge customware endpoint.
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
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+spaceAgentInstructionEndpoint, bytes.NewReader(body))
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.SpaceAgent.BridgeToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.SpaceAgent.BridgeToken)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	defer resp.Body.Close()
	var parsed map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&parsed); err != nil {
		parsed = map[string]interface{}{"status": "ok", "http_status": resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		parsed["status"] = "error"
		parsed["http_status"] = resp.StatusCode
	}
	return parsed
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
	if err := ensureSpaceAgentCustomwareUserHome(cfg.CustomwarePath, cfg.AdminUser); err != nil {
		logger.Warn("[SpaceAgent] Host-side customware workspace seed skipped; container bootstrap will retry", "error", err)
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
	if err := os.WriteFile(dockerfilePath, []byte(spaceAgentDockerfile()), 0o600); err != nil {
		return fmt.Errorf("write Dockerfile.aurago: %w", err)
	}
	return runSpaceAgentCommand(logger, cfg.SourcePath, "docker", "build", "-f", dockerfilePath, "-t", cfg.Image, cfg.SourcePath)
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

func spaceAgentDockerfile() string {
	return `FROM node:22-bookworm-slim
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates openssh-client \
    && rm -rf /var/lib/apt/lists/*
COPY package*.json ./
RUN npm ci --omit=dev || npm install --omit=dev
COPY . .
EXPOSE 3000 3100
CMD ["sh", "-lc", "node aurago_space_bootstrap.mjs && node space supervise --state-dir /app/supervisor HOST=${HOST:-0.0.0.0} PORT=${PORT:-3000}"]
`
}

func spaceAgentBootstrapScript() string {
	return `import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

import { loadSupervisorAuthEnv } from "./commands/lib/supervisor/auth_keys.js";
import { createUser, setUserPassword } from "./server/lib/auth/user_manage.js";

const username = String(process.env.SPACE_AGENT_ADMIN_USER || "").trim();
const password = String(process.env.SPACE_AGENT_ADMIN_PASSWORD || "");
const projectRoot = process.cwd();
const stateDir = "/app/supervisor";
const managedStatePath = path.join(stateDir, "auth", "aurago_managed_user.json");

function normalizeEntityId(value) {
  const raw = String(value || "").trim().replaceAll("\\", "/");
  if (!raw || raw.includes("/")) {
    throw new Error("Managed Space Agent username must be a single path segment.");
  }
  const normalized = path.posix.normalize(raw);
  if (!normalized || normalized === "." || normalized === ".." || normalized.includes("/")) {
    throw new Error("Managed Space Agent username must be a single path segment.");
  }
  return normalized;
}

function digestPassword(value) {
  return createHash("sha256").update(String(value || ""), "utf8").digest("hex");
}

function readManagedState() {
  try {
    const parsed = JSON.parse(fs.readFileSync(managedStatePath, "utf8"));
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function writeManagedState(usernameValue, passwordDigest) {
  fs.mkdirSync(path.dirname(managedStatePath), { recursive: true, mode: 0o700 });
  fs.writeFileSync(
    managedStatePath,
    JSON.stringify({
      password_sha256: passwordDigest,
      updated_at: new Date().toISOString(),
      username: usernameValue
    }, null, 2) + "\n",
    { mode: 0o600 }
  );
}

function clearInvalidatedUserCrypto(usernameValue) {
  const customwarePath = process.env.CUSTOMWARE_PATH || "/app/customware";
  fs.rmSync(path.join(customwarePath, "L2", usernameValue, "meta", "user_crypto.json"), {
    force: true
  });
}

function seedFile(filePath, content) {
  if (!fs.existsSync(filePath)) {
    fs.writeFileSync(filePath, content, { mode: 0o600 });
  }
}

function seedWorkspaceFiles(rootPath) {
  for (const dir of [
    rootPath,
    path.join(rootPath, "meta"),
    path.join(rootPath, "spaces"),
    path.join(rootPath, "conf"),
    path.join(rootPath, "hist"),
    path.join(rootPath, "dashboard"),
    path.join(rootPath, "onscreen-agent"),
    path.join(rootPath, ".config"),
    path.join(rootPath, ".local", "share")
  ]) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o750 });
  }
  seedFile(path.join(rootPath, "meta", "login_hooks.json"), "[]\n");
  seedFile(path.join(rootPath, "conf", "dashboard.yaml"), "{}\n");
  seedFile(path.join(rootPath, "conf", "onscreen-agent.yaml"), "{}\n");
  seedFile(path.join(rootPath, "hist", "onscreen-agent.json"), "[]\n");
  seedFile(path.join(rootPath, "dashboard", "prefs.json"), "{}\n");
  seedFile(path.join(rootPath, "dashboard", "dashboard-prefs.json"), "{}\n");
  seedFile(path.join(rootPath, "onscreen-agent", "config.json"), "{}\n");
  seedFile(path.join(rootPath, "onscreen-agent", "history.json"), "[]\n");
  seedFile(path.join(rootPath, "meta", "onscreen-agent-config.json"), "{}\n");
  seedFile(path.join(rootPath, "meta", "onscreen-agent-history.json"), "[]\n");
  seedFile(path.join(rootPath, "meta", "dashboard-prefs.json"), "{}\n");
  seedFile(path.join(rootPath, ".config", "dashboard-prefs.json"), "{}\n");
  seedFile(path.join(rootPath, ".config", "onscreen-agent-config.json"), "{}\n");
  seedFile(path.join(rootPath, ".config", "onscreen-agent-history.json"), "[]\n");
}

if (username && password) {
  process.env.CUSTOMWARE_PATH = process.env.CUSTOMWARE_PATH || "/app/customware";
  const normalizedUsername = normalizeEntityId(username);
  const passwordDigest = digestPassword(password);
  seedWorkspaceFiles(path.join(process.env.CUSTOMWARE_PATH, "L2", normalizedUsername));
  const auth = await loadSupervisorAuthEnv({ env: process.env, stateDir });
  Object.assign(process.env, auth.env);

  try {
    createUser(projectRoot, username, password, { fullName: username });
    writeManagedState(normalizedUsername, passwordDigest);
    console.log("[aurago-bootstrap] Created managed Space Agent user " + username + ".");
  } catch (error) {
    if (!String(error?.message || "").startsWith("User already exists:")) {
      throw error;
    }
    const managedState = readManagedState();
    if (
      managedState.username === normalizedUsername &&
      managedState.password_sha256 === passwordDigest
    ) {
      console.log("[aurago-bootstrap] Managed Space Agent user " + username + " already current.");
    } else {
      setUserPassword(projectRoot, username, password);
      clearInvalidatedUserCrypto(normalizedUsername);
      writeManagedState(normalizedUsername, passwordDigest);
      console.log("[aurago-bootstrap] Updated managed Space Agent user " + username + ".");
    }
  }
}
`
}

func writeSpaceAgentBridgeCustomware(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	content := `# AuraGo Space Agent Bridge

This directory is mounted into the managed Space Agent container.

Use the environment variables AURAGO_BRIDGE_URL and AURAGO_BRIDGE_TOKEN to send structured messages back to AuraGo:

{
  "type": "note",
  "summary": "Short title",
  "content": "External content from Space Agent",
  "source": "space-agent",
  "timestamp": "2026-05-01T12:00:00Z",
  "session_id": "optional"
}
`
	if err := os.WriteFile(filepath.Join(dir, "aurago_bridge.md"), []byte(content), 0o600); err != nil {
		return err
	}
	helper := `'use strict';

async function sendToAuraGo(message) {
  const bridgeUrl = process.env.AURAGO_BRIDGE_URL;
  const bridgeToken = process.env.AURAGO_BRIDGE_TOKEN;
  if (!bridgeUrl || !bridgeToken) {
    throw new Error('AuraGo bridge environment is not configured');
  }
  const payload = {
    type: message.type || 'message',
    summary: message.summary || '',
    content: message.content || '',
    source: message.source || 'space-agent',
    timestamp: message.timestamp || new Date().toISOString(),
    session_id: message.session_id || undefined
  };
  const res = await fetch(bridgeUrl, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer ' + bridgeToken
    },
    body: JSON.stringify(payload)
  });
  if (!res.ok) {
    throw new Error('AuraGo bridge returned HTTP ' + res.status);
  }
  return res.json();
}

module.exports = { sendToAuraGo };
`
	return os.WriteFile(filepath.Join(dir, "aurago_bridge.js"), []byte(helper), 0o600)
}

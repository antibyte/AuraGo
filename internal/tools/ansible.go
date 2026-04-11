package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// AnsibleConfig holds the connection parameters for the Ansible sidecar API.
type AnsibleConfig struct {
	URL     string // e.g. "http://ansible:5001"
	Token   string // Bearer token (matches ANSIBLE_API_TOKEN in the sidecar)
	Timeout int    // HTTP client timeout in seconds (default 360)
}

// ansibleHTTPClient is intentionally generous — playbook runs can take minutes.
var ansibleHTTPClient = &http.Client{Timeout: 360 * time.Second}

// ansibleRequest executes an authenticated HTTP request against the Ansible sidecar.
func ansibleRequest(cfg AnsibleConfig, method, endpoint string, body interface{}) ([]byte, int, error) {
	url := strings.TrimRight(cfg.URL, "/") + endpoint

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	client := ansibleHTTPClient
	if cfg.Timeout > 0 {
		client = &http.Client{Timeout: time.Duration(cfg.Timeout+60) * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// ansibleResult wraps the raw API response, adding a status field in error cases.
func ansibleResult(data []byte, code int, err error) string {
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Cannot reach Ansible sidecar: %v"}`, err)
	}
	if code == 401 {
		return `{"status":"error","message":"Ansible sidecar rejected the token. Check ansible.token config."}`
	}
	return string(data)
}

// ── Public API ────────────────────────────────────────────────────────────────

// AnsibleStatus returns the health check of the Ansible sidecar (ansible version, config).
func AnsibleStatus(cfg AnsibleConfig) string {
	data, code, err := ansibleRequest(cfg, "GET", "/status", nil)
	return ansibleResult(data, code, err)
}

// AnsibleListPlaybooks returns the list of playbook files available on the sidecar.
func AnsibleListPlaybooks(cfg AnsibleConfig) string {
	data, code, err := ansibleRequest(cfg, "GET", "/playbooks", nil)
	return ansibleResult(data, code, err)
}

// AnsibleListInventory parses the inventory and returns the host list.
// inventoryPath overrides the sidecar's default inventory when non-empty.
func AnsibleListInventory(cfg AnsibleConfig, inventoryPath string) string {
	endpoint := "/inventory"
	if inventoryPath != "" {
		endpoint += "?inventory=" + inventoryPath
	}
	data, code, err := ansibleRequest(cfg, "GET", endpoint, nil)
	return ansibleResult(data, code, err)
}

// AnsiblePing executes `ansible <hosts> -m ping`.
func AnsiblePing(cfg AnsibleConfig, hosts, inventoryPath string) string {
	if hosts == "" {
		hosts = "all"
	}
	body := map[string]interface{}{"hosts": hosts}
	if inventoryPath != "" {
		body["inventory"] = inventoryPath
	}
	data, code, err := ansibleRequest(cfg, "POST", "/run/ping", body)
	return ansibleResult(data, code, err)
}

// AnsibleAdhoc runs an ad-hoc ansible module command.
//   - hosts:      target host pattern (e.g. "all", "webservers", "192.168.1.10")
//   - module:     ansible module name (e.g. "ping", "shell", "copy", "service")
//   - moduleArgs: module arguments string (e.g. "cmd='uptime'" or "name=nginx state=started")
//   - inventory:  optional path override
//   - extraVars:  optional extra variables
func AnsibleAdhoc(cfg AnsibleConfig, hosts, module, moduleArgs, inventoryPath string, extraVars map[string]interface{}) string {
	if hosts == "" {
		hosts = "all"
	}
	if module == "" {
		module = "ping"
	}
	body := map[string]interface{}{
		"hosts":  hosts,
		"module": module,
	}
	if moduleArgs != "" {
		body["args"] = moduleArgs
	}
	if inventoryPath != "" {
		body["inventory"] = inventoryPath
	}
	if len(extraVars) > 0 {
		body["extra_vars"] = extraVars
	}
	data, code, err := ansibleRequest(cfg, "POST", "/run/adhoc", body)
	return ansibleResult(data, code, err)
}

// AnsibleRunPlaybook executes an ansible-playbook command.
//   - playbook:   filename relative to the sidecar's PLAYBOOKS_DIR (e.g. "site.yml")
//   - inventory:  optional path override
//   - limit:      optional --limit pattern (e.g. "webservers" or "192.168.1.10")
//   - tags:       optional --tags (comma-separated)
//   - skipTags:   optional --skip-tags (comma-separated)
//   - extraVars:  optional extra variables (Go map, marshalled to JSON)
//   - check:      true = --check (dry-run, no changes applied)
//   - diff:       true = --diff (show file diffs)
func AnsibleRunPlaybook(cfg AnsibleConfig, playbook, inventoryPath, limit, tags, skipTags string, extraVars map[string]interface{}, check, diff bool) string {
	if playbook == "" {
		return `{"status":"error","message":"playbook name is required"}`
	}
	body := map[string]interface{}{
		"playbook": playbook,
	}
	if inventoryPath != "" {
		body["inventory"] = inventoryPath
	}
	if limit != "" {
		body["limit"] = limit
	}
	if tags != "" {
		body["tags"] = tags
	}
	if skipTags != "" {
		body["skip_tags"] = skipTags
	}
	if len(extraVars) > 0 {
		body["extra_vars"] = extraVars
	}
	if check {
		body["check"] = true
	}
	if diff {
		body["diff"] = true
	}
	data, code, err := ansibleRequest(cfg, "POST", "/run/playbook", body)
	return ansibleResult(data, code, err)
}

// AnsibleGatherFacts runs `ansible <hosts> -m setup` to collect system facts.
func AnsibleGatherFacts(cfg AnsibleConfig, hosts, inventoryPath string) string {
	if hosts == "" {
		hosts = "all"
	}
	body := map[string]interface{}{"hosts": hosts}
	if inventoryPath != "" {
		body["inventory"] = inventoryPath
	}
	data, code, err := ansibleRequest(cfg, "POST", "/run/facts", body)
	return ansibleResult(data, code, err)
}

// ── Sidecar container management ──────────────────────────────────────────────

const ansibleContainerName = "aurago_ansible"
const ansibleSidecarImage = "aurago-ansible:latest"

// AnsibleSidecarConfig carries the fields needed to auto-manage the
// Ansible sidecar container via the Docker API.
type AnsibleSidecarConfig struct {
	Token         string // ANSIBLE_API_TOKEN inside the container
	Timeout       int    // ANSIBLE_TIMEOUT (seconds, default 300)
	Image         string // Docker image (default: aurago-ansible:latest)
	ContainerName string // Container name (default: aurago_ansible)
	PlaybooksDir  string // Host path to playbooks dir (mounted into /playbooks)
	InventoryDir  string // Host path to inventory dir (mounted into /inventory)
	AutoBuild     bool   // Build image automatically if not found locally
	DockerfileDir string // Directory containing Dockerfile.ansible (default: ".")
}

// EnsureAnsibleSidecarRunning ensures the Ansible sidecar container is running.
// It checks the container state via the Docker API and, if missing or stopped,
// creates/starts it. The image must be pre-built locally:
//
//	docker build -f Dockerfile.ansible -t aurago-ansible .
//
// Safe to call multiple times.
func EnsureAnsibleSidecarRunning(dockerHost string, sidecarCfg AnsibleSidecarConfig, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	dockerCfg := DockerConfig{Host: dockerHost}

	image := sidecarCfg.Image
	if image == "" {
		image = ansibleSidecarImage
	}
	containerName := sidecarCfg.ContainerName
	if containerName == "" {
		containerName = ansibleContainerName
	}

	// Check if ANY ansible sidecar (any name) is already running with the same image.
	listData, listCode, listErr := dockerRequest(dockerCfg, "GET",
		`/containers/json?filters={"status":["running"],"ancestor":["`+image+`"]}`, "")
	if listErr == nil && listCode == 200 {
		var containers []map[string]interface{}
		if json.Unmarshal(listData, &containers) == nil && len(containers) > 0 {
			logger.Info("[Ansible] Sidecar container already running (external)", "count", len(containers))
			return
		}
	}

	// Inspect our managed container
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+containerName+"/json", "")
	if err != nil {
		logger.Warn("[Ansible] Docker unavailable, skipping sidecar auto-start", "error", err)
		return
	}

	if code == 200 {
		// Container exists — check if it's running
		var info map[string]interface{}
		if json.Unmarshal(data, &info) == nil {
			if state, ok := info["State"].(map[string]interface{}); ok {
				if running, _ := state["Running"].(bool); running {
					logger.Info("[Ansible] Sidecar container already running")
					return
				}
			}
		}
		// Exists but stopped — start it
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/start", "")
		if startErr != nil || (startCode != 204 && startCode != 304) {
			logger.Error("[Ansible] Failed to start existing sidecar container", "code", startCode, "error", startErr)
			return
		}
		logger.Info("[Ansible] Sidecar container started")
		return
	}

	if code != 404 {
		logger.Warn("[Ansible] Unexpected Docker inspect response, skipping sidecar auto-start", "code", code)
		return
	}

	// Container does not exist — check if image is available
	_, imgCode, imgErr := dockerRequest(dockerCfg, "GET", "/images/"+image+"/json", "")
	if imgErr != nil || imgCode != 200 {
		if sidecarCfg.AutoBuild {
			if err := buildAnsibleImage(image, sidecarCfg.DockerfileDir, logger); err != nil {
				logger.Error("[Ansible] Auto-build failed. Start the sidecar manually after building the image.",
					"image", image,
					"hint", "docker build -f Dockerfile.ansible -t "+image+" .",
					"error", err)
				return
			}
		} else {
			logger.Warn("[Ansible] Image not found locally. Build it first.",
				"image", image,
				"hint", "docker build -f Dockerfile.ansible -t "+image+" .")
			return
		}
	}

	// Build environment variables
	env := []string{
		"PORT=5001",
		"ANSIBLE_HOST_KEY_CHECKING=False",
	}
	if sidecarCfg.Token != "" {
		env = append(env, "ANSIBLE_API_TOKEN="+sidecarCfg.Token)
	}
	if sidecarCfg.Timeout > 0 {
		env = append(env, fmt.Sprintf("ANSIBLE_TIMEOUT=%d", sidecarCfg.Timeout))
	}

	// Build volume binds
	var binds []string
	// Mount SSH keys (read-only) so Ansible can reach managed hosts
	sshDir := ansibleSSHDir()
	if sshDir != "" {
		binds = append(binds, sshDir+":/root/.ssh:ro")
	}
	if sidecarCfg.PlaybooksDir != "" {
		binds = append(binds, sidecarCfg.PlaybooksDir+":/playbooks")
	}
	if sidecarCfg.InventoryDir != "" {
		binds = append(binds, sidecarCfg.InventoryDir+":/inventory")
	}

	// Create and start the container
	hostConfig := map[string]interface{}{
		"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
		"PortBindings": map[string]interface{}{
			"5001/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": "5001"}},
		},
	}
	if len(binds) > 0 {
		hostConfig["Binds"] = binds
	}

	payload := map[string]interface{}{
		"Image": image,
		"Env":   env,
		"ExposedPorts": map[string]interface{}{
			"5001/tcp": struct{}{},
		},
		"HostConfig": hostConfig,
	}
	body, _ := json.Marshal(payload)
	_, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+containerName, string(body))
	if createErr != nil || createCode != 201 {
		logger.Error("[Ansible] Failed to create sidecar container", "code", createCode, "error", createErr)
		return
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		logger.Error("[Ansible] Failed to start new sidecar container", "code", startCode, "error", startErr)
		return
	}
	logger.Info("[Ansible] Sidecar container created and started", "image", image, "container", containerName)
}

// buildAnsibleImage runs `docker build -f Dockerfile.ansible -t <image> <dir>` to build
// the Ansible sidecar image. This is called automatically when auto_build is enabled and
// the image is not found locally. The build can take several minutes on first run.
func buildAnsibleImage(image, dockerfileDir string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if dockerfileDir == "" {
		dockerfileDir = "."
	}

	logger.Info("[Ansible] Building sidecar image (this may take a few minutes)…",
		"image", image,
		"context", dockerfileDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build",
		"-f", filepath.Join(dockerfileDir, "Dockerfile.ansible"),
		"-t", image,
		dockerfileDir,
	)
	// Ensure Docker CLI can write its config/cache without needing /root/.docker.
	// Under systemd with ProtectSystem=strict + ProtectHome=read-only the user
	// home (/root) is read-only, causing "mkdir /root/.docker: read-only file system".
	// Use a path inside the working directory which is in ReadWritePaths.
	dockerCfgDir := filepath.Join(dockerfileDir, "data", ".docker")
	os.MkdirAll(dockerCfgDir, 0o700)
	cmd.Env = append(os.Environ(), "DOCKER_CONFIG="+dockerCfgDir)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	logger.Info("[Ansible] Image built successfully", "image", image)
	return nil
}

// ReapplyAnsibleToken stops and removes the managed Ansible sidecar container, then
// recreates it with the updated token in AnsibleSidecarConfig. This is called whenever
// the token changes so the running container picks up the new value immediately.
func ReapplyAnsibleToken(dockerHost string, sidecarCfg AnsibleSidecarConfig, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	containerName := sidecarCfg.ContainerName
	if containerName == "" {
		containerName = ansibleContainerName
	}
	dockerCfg := DockerConfig{Host: dockerHost}

	logger.Info("[Ansible] Reapplying token — recreating sidecar container", "container", containerName)

	// Stop container (best-effort, errors expected if already stopped)
	if _, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/stop?t=5", ""); err != nil {
		logger.Warn("[Ansible] Failed to stop container (may already be stopped)", "container", containerName, "code", code, "error", err)
	}
	// Remove container (best-effort)
	if _, code, err := dockerRequest(dockerCfg, "DELETE", "/containers/"+containerName, ""); err != nil {
		logger.Warn("[Ansible] Failed to remove container (may already be removed)", "container", containerName, "code", code, "error", err)
	}

	// Recreate with updated token
	EnsureAnsibleSidecarRunning(dockerHost, sidecarCfg, logger)
}

// ansibleSSHDir returns the host path to the SSH directory (~/.ssh).
func ansibleSSHDir() string {
	if runtime.GOOS == "windows" {
		return "" // Volume mounts from Windows to Linux containers are unreliable
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".ssh")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

// ── Local CLI mode ────────────────────────────────────────────────────────────
// Used when ansible.mode = "local". Runs the ansible / ansible-playbook
// binary directly via os/exec — no sidecar container needed.

// AnsibleLocalConfig holds settings for direct (non-sidecar) ansible execution.
type AnsibleLocalConfig struct {
	PlaybooksDir     string // directory containing playbook files
	DefaultInventory string // default inventory file path
	Timeout          int    // max seconds per command (default 300)
}

// ansibleLocalResult marshals a CLI result into a consistent JSON string.
func ansibleLocalResult(stdout, stderr string, err error) string {
	status := "ok"
	msg := ""
	if err != nil {
		status = "error"
		msg = err.Error()
	}
	out := map[string]interface{}{
		"status": status,
		"stdout": stdout,
		"stderr": stderr,
	}
	if msg != "" {
		out["message"] = msg
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// ansibleRunCmd executes a command with a timeout and returns stdout, stderr, and error.
func ansibleRunCmd(timeout int, name string, args ...string) (string, string, error) {
	if timeout <= 0 {
		timeout = 300
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return outBuf.String(), errBuf.String(), fmt.Errorf("command timed out after %ds", timeout)
	}
	return outBuf.String(), errBuf.String(), err
}

// AnsibleLocalStatus returns ansible and ansible-playbook version information.
func AnsibleLocalStatus(cfg AnsibleLocalConfig) string {
	stdout, stderr, err := ansibleRunCmd(30, "ansible", "--version")
	return ansibleLocalResult(stdout, stderr, err)
}

// AnsibleLocalListPlaybooks lists *.yml and *.yaml files in PlaybooksDir.
func AnsibleLocalListPlaybooks(cfg AnsibleLocalConfig) string {
	dir := cfg.PlaybooksDir
	if dir == "" {
		return `{"status":"error","message":"ansible.playbooks_dir is not configured"}`
	}
	patterns := []string{
		filepath.Join(dir, "*.yml"),
		filepath.Join(dir, "*.yaml"),
	}
	var files []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err == nil {
			for _, m := range matches {
				files = append(files, filepath.Base(m))
			}
		}
	}
	b, _ := json.Marshal(map[string]interface{}{"status": "ok", "playbooks": files, "directory": dir})
	return string(b)
}

// AnsibleLocalListInventory runs `ansible-inventory --list` for the given (or default) inventory.
func AnsibleLocalListInventory(cfg AnsibleLocalConfig, inventoryPath string) string {
	inv := inventoryPath
	if inv == "" {
		inv = cfg.DefaultInventory
	}
	args := []string{"--list"}
	if inv != "" {
		args = append(args, "-i", inv)
	}
	stdout, stderr, err := ansibleRunCmd(cfg.Timeout, "ansible-inventory", args...)
	return ansibleLocalResult(stdout, stderr, err)
}

// AnsibleLocalPing runs `ansible <hosts> -m ping`.
func AnsibleLocalPing(cfg AnsibleLocalConfig, hosts, inventoryPath string) string {
	if hosts == "" {
		hosts = "all"
	}
	inv := inventoryPath
	if inv == "" {
		inv = cfg.DefaultInventory
	}
	args := []string{hosts, "-m", "ping"}
	if inv != "" {
		args = append(args, "-i", inv)
	}
	stdout, stderr, err := ansibleRunCmd(cfg.Timeout, "ansible", args...)
	return ansibleLocalResult(stdout, stderr, err)
}

// AnsibleLocalAdhoc runs `ansible <hosts> -m <module> [-a <args>]`.
func AnsibleLocalAdhoc(cfg AnsibleLocalConfig, hosts, module, moduleArgs, inventoryPath string, extraVars map[string]interface{}) string {
	if hosts == "" {
		hosts = "all"
	}
	if module == "" {
		module = "ping"
	}
	inv := inventoryPath
	if inv == "" {
		inv = cfg.DefaultInventory
	}
	args := []string{hosts, "-m", module}
	if moduleArgs != "" {
		args = append(args, "-a", moduleArgs)
	}
	if inv != "" {
		args = append(args, "-i", inv)
	}
	if len(extraVars) > 0 {
		b, _ := json.Marshal(extraVars)
		args = append(args, "--extra-vars", string(b))
	}
	stdout, stderr, err := ansibleRunCmd(cfg.Timeout, "ansible", args...)
	return ansibleLocalResult(stdout, stderr, err)
}

// AnsibleLocalRunPlaybook runs `ansible-playbook <playbook>` with optional flags.
func AnsibleLocalRunPlaybook(cfg AnsibleLocalConfig, playbook, inventoryPath, limit, tags, skipTags string, extraVars map[string]interface{}, check, diff bool) string {
	if playbook == "" {
		return `{"status":"error","message":"playbook name is required"}`
	}
	// Resolve playbook path
	playbookPath := playbook
	if cfg.PlaybooksDir != "" && !filepath.IsAbs(playbook) {
		playbookPath = filepath.Join(cfg.PlaybooksDir, playbook)
		base := filepath.Clean(cfg.PlaybooksDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(playbookPath)+string(os.PathSeparator), base) {
			return `{"status":"error","message":"playbook path escapes the configured playbooks directory"}`
		}
	}
	if _, statErr := os.Stat(playbookPath); statErr != nil {
		return fmt.Sprintf(`{"status":"error","message":"playbook not found: %s"}`, playbookPath)
	}
	inv := inventoryPath
	if inv == "" {
		inv = cfg.DefaultInventory
	}
	args := []string{playbookPath}
	if inv != "" {
		args = append(args, "-i", inv)
	}
	if limit != "" {
		args = append(args, "--limit", limit)
	}
	if tags != "" {
		args = append(args, "--tags", tags)
	}
	if skipTags != "" {
		args = append(args, "--skip-tags", skipTags)
	}
	if len(extraVars) > 0 {
		b, _ := json.Marshal(extraVars)
		args = append(args, "--extra-vars", string(b))
	}
	if check {
		args = append(args, "--check")
	}
	if diff {
		args = append(args, "--diff")
	}
	stdout, stderr, err := ansibleRunCmd(cfg.Timeout, "ansible-playbook", args...)
	return ansibleLocalResult(stdout, stderr, err)
}

// AnsibleLocalGatherFacts runs `ansible <hosts> -m setup` to collect system facts.
func AnsibleLocalGatherFacts(cfg AnsibleLocalConfig, hosts, inventoryPath string) string {
	if hosts == "" {
		hosts = "all"
	}
	inv := inventoryPath
	if inv == "" {
		inv = cfg.DefaultInventory
	}
	args := []string{hosts, "-m", "setup"}
	if inv != "" {
		args = append(args, "-i", inv)
	}
	stdout, stderr, err := ansibleRunCmd(cfg.Timeout, "ansible", args...)
	// Truncate large fact output to avoid overwhelming the context window
	if len(stdout) > 16384 {
		stdout = truncateUTF8Safe(stdout, 16384) + "\n... [truncated]"
	}
	return ansibleLocalResult(stdout, stderr, err)
}

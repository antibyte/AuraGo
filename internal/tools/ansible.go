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

	data, err := io.ReadAll(resp.Body)
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
		stdout = stdout[:16384] + "\n... [truncated]"
	}
	return ansibleLocalResult(stdout, stderr, err)
}

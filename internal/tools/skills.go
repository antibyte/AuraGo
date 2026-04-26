package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

// maxSkillArgsBytes limits the serialized size of skill arguments to prevent
// denial of service via excessively large JSON payloads.
const maxSkillArgsBytes = 10 * 1024 * 1024   // 10 MB
const maxSkillOutputBytes = 10 * 1024 * 1024 // 10 MB
const skillDependencyInstallTimeout = 10 * time.Minute

// skillsCacheTTL is the time-to-live for the ListSkills cache.
const skillsCacheTTL = 30 * time.Second

// skillsCacheEntry holds cached skill manifests and their expiration time.
type skillsCacheEntry struct {
	skills    []SkillManifest
	expiresAt time.Time
}

// listSkillsCache is a simple TTL cache for skill manifests.
var listSkillsCache = struct {
	mu      sync.RWMutex
	entries map[string]skillsCacheEntry
}{
	entries: make(map[string]skillsCacheEntry),
}

// SkillManifest represents the structure of a skill config file (.json).
type SkillManifest struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Executable    string                 `json:"executable"` // e.g., "scan.py" or "custom_tool.exe"
	Category      string                 `json:"category,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`     // parameter schema (legacy flat map or JSON Schema)
	Returns       string                 `json:"returns,omitempty"`        // describes expected output format
	Dependencies  []string               `json:"dependencies,omitempty"`   // pip packages required by this skill
	VaultKeys     []string               `json:"vault_keys,omitempty"`     // vault secret keys this skill needs at runtime
	InternalTools []string               `json:"internal_tools,omitempty"` // AuraGo native tools this skill may call via tool bridge
	Documentation string                 `json:"documentation,omitempty"`  // optional path to a Markdown manual relative to skills_dir; defaults to <name>.md when present
	CheatsheetIDs []string               `json:"cheatsheet_ids,omitempty"` // optional cheatsheet IDs that complement this skill's manual
	Daemon        *DaemonManifest        `json:"daemon,omitempty"`         // if set, skill can run as a background daemon
}

// ExtractSkillParameterNames returns the list of parameter names from a skill
// manifest. It supports both the legacy flat map (key = param name) and JSON
// Schema objects that declare parameters under "properties".
func ExtractSkillParameterNames(params map[string]interface{}) []string {
	if params == nil {
		return nil
	}
	if props, ok := params["properties"].(map[string]interface{}); ok {
		names := make([]string, 0, len(props))
		for k := range props {
			names = append(names, k)
		}
		sort.Strings(names)
		return names
	}
	names := make([]string, 0, len(params))
	for k := range params {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// DaemonManifest holds configuration for a daemon skill.
// A nil DaemonManifest means the skill is not a daemon.
type DaemonManifest struct {
	Enabled                    bool              `json:"enabled"`
	WakeAgent                  bool              `json:"wake_agent"`
	WakeRateLimitSeconds       int               `json:"wake_rate_limit_seconds,omitempty"` // min seconds between wake-ups (default: 3000)
	MaxRuntimeHours            int               `json:"max_runtime_hours,omitempty"`       // hard kill after N hours (0 = unlimited)
	RestartOnCrash             bool              `json:"restart_on_crash"`
	MaxRestartAttempts         int               `json:"max_restart_attempts,omitempty"`          // max restarts within cooldown (default: 3)
	RestartCooldownSeconds     int               `json:"restart_cooldown_seconds,omitempty"`      // cooldown window for restart counting (default: 300)
	HealthCheckIntervalSeconds int               `json:"health_check_interval_seconds,omitempty"` // process liveness check interval (default: 60)
	Env                        map[string]string `json:"env,omitempty"`                           // extra environment variables
	TriggerMissionID           string            `json:"trigger_mission_id,omitempty"`            // mission to trigger on daemon event
	TriggerMissionName         string            `json:"trigger_mission_name,omitempty"`          // display name for UI
	CheatsheetID               string            `json:"cheatsheet_id,omitempty"`                 // cheatsheet to inject as working instructions
	CheatsheetName             string            `json:"cheatsheet_name,omitempty"`               // display name for UI
}

// DaemonManifestDefaults returns a DaemonManifest with sensible defaults applied.
func DaemonManifestDefaults() DaemonManifest {
	return DaemonManifest{
		WakeRateLimitSeconds:       60, // 1 minute; templates with wake_agent write this explicitly
		RestartOnCrash:             true,
		MaxRestartAttempts:         3,
		RestartCooldownSeconds:     300,
		HealthCheckIntervalSeconds: 60,
	}
}

// ApplyDefaults fills zero-value fields with sensible defaults.
// Note: RestartOnCrash is always set to the default (true) regardless of
// previous value, as daemon skills should always restart on crash unless
// explicitly disabled via configuration.
func (d *DaemonManifest) ApplyDefaults() {
	defaults := DaemonManifestDefaults()
	if d.WakeRateLimitSeconds <= 0 {
		d.WakeRateLimitSeconds = defaults.WakeRateLimitSeconds
	}
	// RestartOnCrash is intentionally always applied from defaults
	// to ensure daemons are resilient by default
	d.RestartOnCrash = defaults.RestartOnCrash
	if d.MaxRestartAttempts <= 0 {
		d.MaxRestartAttempts = defaults.MaxRestartAttempts
	}
	if d.RestartCooldownSeconds <= 0 {
		d.RestartCooldownSeconds = defaults.RestartCooldownSeconds
	}
	if d.HealthCheckIntervalSeconds <= 0 {
		d.HealthCheckIntervalSeconds = defaults.HealthCheckIntervalSeconds
	}
}

// ListSkills scans the skills directory for .json manifest files and returns them.
// Results are cached for skillsCacheTTL to avoid repeated filesystem reads.
func ListSkills(skillsDir string) ([]SkillManifest, error) {
	absDir, err := filepath.Abs(skillsDir)
	if err != nil {
		absDir = skillsDir
	}

	// Check cache first
	listSkillsCache.mu.RLock()
	if entry, ok := listSkillsCache.entries[absDir]; ok && time.Now().Before(entry.expiresAt) {
		listSkillsCache.mu.RUnlock()
		return entry.skills, nil
	}
	listSkillsCache.mu.RUnlock()

	// Cache miss - read from disk
	var skills []SkillManifest

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return skills, nil // Empty but not an error if directory doesn't exist yet
		}
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(skillsDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue // Skip unreadable files
			}

			var manifest SkillManifest
			if err := json.Unmarshal(data, &manifest); err == nil && manifest.Name != "" && manifest.Executable != "" {
				skills = append(skills, manifest)
			}
		}
	}

	// Update cache with defensive copy to prevent race conditions
	listSkillsCache.mu.Lock()
	skillsCopy := make([]SkillManifest, len(skills))
	copy(skillsCopy, skills)
	listSkillsCache.entries[absDir] = skillsCacheEntry{
		skills:    skillsCopy,
		expiresAt: time.Now().Add(skillsCacheTTL),
	}
	listSkillsCache.mu.Unlock()

	return skills, nil
}

// InvalidateSkillsCache clears the cache for a specific skills directory.
// Call this when skills are created, updated, or deleted.
func InvalidateSkillsCache(skillsDir string) {
	absDir, err := filepath.Abs(skillsDir)
	if err != nil {
		absDir = skillsDir
	}
	listSkillsCache.mu.Lock()
	delete(listSkillsCache.entries, absDir)
	listSkillsCache.mu.Unlock()
}

type skillExecutionOptions struct {
	injectEnv   func(*exec.Cmd)
	scrubOutput bool
	logInput    bool
}

func resolveSkillExecution(skillsDir, skillName string, argsJSON map[string]interface{}) (SkillManifest, string, string, error) {
	skills, err := ListSkills(skillsDir)
	if err != nil {
		return SkillManifest{}, "", "", fmt.Errorf("failed to scan skills: %w", err)
	}

	var manifest *SkillManifest
	for _, s := range skills {
		if s.Name == skillName {
			manifest = &s
			break
		}
	}
	if manifest == nil {
		return SkillManifest{}, "", "", fmt.Errorf("skill '%s' not found", skillName)
	}

	if manifest.Executable == "__builtin__" {
		return SkillManifest{}, "", "", fmt.Errorf("skill '%s' is built-in and cannot be executed via execute_skill", skillName)
	}
	if err := validateSkillExecutable(manifest.Executable); err != nil {
		return SkillManifest{}, "", "", fmt.Errorf("skill '%s' has invalid executable path '%s': %w", skillName, manifest.Executable, err)
	}

	absSkillsDir, err := filepath.Abs(skillsDir)
	if err != nil {
		return SkillManifest{}, "", "", fmt.Errorf("invalid skills directory: %w", err)
	}
	absExecPath, err := filepath.Abs(filepath.Join(skillsDir, manifest.Executable))
	if err != nil {
		return SkillManifest{}, "", "", fmt.Errorf("failed to resolve absolute path for skill '%s': %w", skillName, err)
	}
	rel, err := filepath.Rel(absSkillsDir, absExecPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return SkillManifest{}, "", "", fmt.Errorf("skill '%s' has invalid executable path '%s': skill path traversal detected", skillName, manifest.Executable)
	}
	fi, err := os.Lstat(absExecPath)
	if err != nil {
		return SkillManifest{}, "", "", fmt.Errorf("skill executable not accessible: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return SkillManifest{}, "", "", fmt.Errorf("symlinks are not allowed in skills directory")
	}
	if _, err := os.Stat(absExecPath); err != nil {
		if os.IsNotExist(err) {
			return SkillManifest{}, "", "", fmt.Errorf("skill executable '%s' not found at %s", manifest.Executable, absExecPath)
		}
		return SkillManifest{}, "", "", fmt.Errorf("skill executable '%s' not accessible: %w", manifest.Executable, err)
	}

	if argsJSON == nil {
		argsJSON = make(map[string]interface{})
	}
	argsBytes, err := json.Marshal(argsJSON)
	if err != nil {
		return SkillManifest{}, "", "", fmt.Errorf("failed to serialize args JSON: %w", err)
	}
	if len(argsBytes) > maxSkillArgsBytes {
		return SkillManifest{}, "", "", fmt.Errorf("skill args too large: %d bytes (max %d)", len(argsBytes), maxSkillArgsBytes)
	}

	return *manifest, absExecPath, string(argsBytes), nil
}

func buildSkillCommand(ctx context.Context, workspaceDir string, manifest SkillManifest, absExecPath string) *exec.Cmd {
	if strings.HasSuffix(manifest.Executable, ".py") {
		cfgPythonBin := GetPythonBin(workspaceDir)
		return exec.CommandContext(ctx, cfgPythonBin, "-u", absExecPath)
	}
	if strings.HasSuffix(manifest.Executable, ".sh") && runtime.GOOS != "windows" {
		return exec.CommandContext(ctx, "bash", absExecPath)
	}
	if strings.HasSuffix(manifest.Executable, ".ps1") && runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", absExecPath)
	}
	return exec.CommandContext(ctx, absExecPath)
}

// limitWriter captures stdout/stderr up to a byte limit.
type limitWriter struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.limit {
		w.overflow = true
		remaining := w.limit - w.buf.Len()
		if remaining > 0 {
			w.buf.Write(p[:remaining])
		}
		return len(p), nil
	}
	return w.buf.Write(p)
}

func executePreparedSkill(ctx context.Context, workspaceDir, skillName string, manifest SkillManifest, absExecPath, argsString string, opts skillExecutionOptions) (string, error) {
	if opts.logInput {
		slog.Debug(
			"[ExecuteSkill] Prepared JSON input",
			"skill", skillName,
			"arg_bytes", len(argsString),
			"arg_keys", extractTopLevelJSONKeysForLog(argsString),
		)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, GetSkillTimeout())
	defer cancel()

	cmd := buildSkillCommand(ctx, workspaceDir, manifest, absExecPath)
	cmd.Dir = workspaceDir
	SetSkillLimits(cmd, 1024, int(GetSkillTimeout().Seconds()))
	if opts.injectEnv != nil {
		opts.injectEnv(cmd)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	outBuf := &limitWriter{limit: maxSkillOutputBytes}
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start skill execution: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			KillProcessTree(cmd.Process.Pid)
		}
		cmd.Wait()
	}()
	ApplySkillLimits(cmd.Process.Pid, 1024, int(GetSkillTimeout().Seconds()))

	if opts.logInput {
		slog.Debug("[ExecuteSkill] Writing to Stdin...", "length", len(argsString))
	}
	if _, err := fmt.Fprint(stdin, argsString); err != nil {
		_ = stdin.Close()
		return "", fmt.Errorf("failed to write skill input: %w", err)
	}
	if err := stdin.Close(); err != nil {
		if opts.logInput {
			slog.Error("[ExecuteSkill] Failed to close stdin pipe", "error", err)
		}
		return "", fmt.Errorf("failed to close skill stdin: %w", err)
	}
	if opts.logInput {
		slog.Debug("[ExecuteSkill] Stdin closed (EOF sent)")
	}

	err = cmd.Wait()
	output := outBuf.buf.String()
	if outBuf.overflow {
		output += fmt.Sprintf("\n[OUTPUT TRUNCATED: exceeded %d MB limit]", maxSkillOutputBytes/(1024*1024))
	}
	if opts.scrubOutput {
		output = security.Scrub(output)
	}
	if ctx.Err() == context.DeadlineExceeded {
		timeout := GetSkillTimeout()
		return output, fmt.Errorf("TIMEOUT: skill '%s' exceeded %s limit and was killed", skillName, timeout.Round(time.Second))
	}
	if err != nil {
		return output, fmt.Errorf("execution failed: %w", err)
	}

	return output, nil
}

func extractTopLevelJSONKeysForLog(argsString string) []string {
	if strings.TrimSpace(argsString) == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsString), &m); err != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 20 {
		return append(keys[:20], fmt.Sprintf("…(+%d)", len(keys)-20))
	}
	return keys
}

// ExecuteSkill dynamically executes the requested skill script, routing Python scripts to the venv.
func ExecuteSkill(ctx context.Context, skillsDir, workspaceDir, skillName string, argsJSON map[string]interface{}) (string, error) {
	manifest, absExecPath, argsString, err := resolveSkillExecution(skillsDir, skillName, argsJSON)
	if err != nil {
		return "", err
	}

	return executePreparedSkill(ctx, workspaceDir, skillName, manifest, absExecPath, argsString, skillExecutionOptions{logInput: true})
}

// ExecuteSkillWithSecrets is like ExecuteSkill but injects vault secrets and credential secrets
// as environment variables and scrubs secrets from the output.
func ExecuteSkillWithSecrets(ctx context.Context, skillsDir, workspaceDir, skillName string, argsJSON map[string]interface{}, secrets map[string]string, creds []CredentialFields, bridgeURL, bridgeToken string, bridgeTools []string) (string, error) {
	manifest, absExecPath, argsString, err := resolveSkillExecution(skillsDir, skillName, argsJSON)
	if err != nil {
		return "", err
	}

	return executePreparedSkill(ctx, workspaceDir, skillName, manifest, absExecPath, argsString, skillExecutionOptions{
		injectEnv: func(cmd *exec.Cmd) {
			InjectSecretsEnv(cmd, secrets)
			InjectCredentialEnv(cmd, creds)
			if len(bridgeTools) > 0 && bridgeURL != "" && bridgeToken != "" {
				InjectToolBridgeEnv(cmd, bridgeURL, bridgeToken, bridgeTools)
			}
		},
		scrubOutput: true,
	})
}

// ExecuteSkillInSandbox executes a skill inside the sandboxed container environment.
// It reads the skill code, injects secrets/creds, prepends the args, and runs via SandboxExecuteCode.
// Returns error if sandbox is unavailable or skill not found.
func ExecuteSkillInSandbox(skillsDir, skillName string, argsJSON map[string]interface{}, secrets map[string]string, creds []CredentialFields, timeoutSeconds int, logger *slog.Logger, bridgeURL, bridgeToken string, bridgeTools []string) (string, error) {
	if _, err := validateSkillName(skillName); err != nil {
		return "", fmt.Errorf("invalid skill name: %w", err)
	}
	// Path traversal check: ensure the resolved path stays within skillsDir.
	absSkillsDir, err := filepath.Abs(skillsDir)
	if err != nil {
		return "", fmt.Errorf("invalid skills directory: %w", err)
	}
	absExecPath, err := filepath.Abs(filepath.Join(skillsDir, skillName+".py"))
	if err != nil {
		return "", fmt.Errorf("invalid skill path: %w", err)
	}
	rel, err := filepath.Rel(absSkillsDir, absExecPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("skill path traversal detected for '%s'", skillName)
	}
	fi, err := os.Lstat(absExecPath)
	if err != nil {
		return "", fmt.Errorf("skill executable not accessible: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("symlinks are not allowed in skills directory")
	}
	// Read skill code
	data, err := os.ReadFile(absExecPath)
	if err != nil {
		return "", fmt.Errorf("skill '%s' not found: %w", skillName, err)
	}
	skillCode := string(data)

	// Serialize args
	if argsJSON == nil {
		argsJSON = make(map[string]interface{})
	}
	argsBytes, err := json.Marshal(argsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to serialize args: %w", err)
	}
	if len(argsBytes) > maxSkillArgsBytes {
		return "", fmt.Errorf("skill args too large: %d bytes (max %d)", len(argsBytes), maxSkillArgsBytes)
	}

	// Build the execution script:
	// 1. Prepend secrets/creds injection
	// 2. Prepend args as a dict
	// 3. The skill's if __name__ == "__main__" block will be skipped since we're executing as a module
	// 4. We need to explicitly call the skill function

	// Build secrets/creds/bridge prelude
	prelude := ""
	if len(secrets) > 0 {
		prelude += BuildSecretPrelude(secrets)
	}
	if len(creds) > 0 {
		prelude += BuildCredentialPrelude(creds)
	}
	if len(bridgeTools) > 0 && bridgeURL != "" && bridgeToken != "" {
		prelude += BuildToolBridgePrelude(bridgeURL, bridgeToken, bridgeTools)
	}

	fullCode, err := buildSandboxSkillExecCode(skillName, skillCode, argsBytes, prelude)
	if err != nil {
		return "", err
	}

	// Execute via sandbox
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	result, err := SandboxExecuteCode(fullCode, "python", nil, timeoutSeconds, logger)
	if err != nil {
		return "", fmt.Errorf("sandbox execution failed: %w", err)
	}
	return result, nil
}

func buildSandboxSkillExecCode(skillName, skillCode string, argsBytes []byte, prelude string) (string, error) {
	loc := mainBlockRe.FindStringIndex(skillCode)
	if loc != nil {
		skillCode = skillCode[:loc[0]]
	}
	funcName := toFunctionName(skillName)
	funcPattern := regexp.MustCompile(`\bdef\s+` + regexp.QuoteMeta(funcName) + `\s*\(`)
	if !funcPattern.MatchString(skillCode) {
		return "", fmt.Errorf("skill '%s' does not define the expected function '%s'", skillName, funcName)
	}
	argsB64 := base64.StdEncoding.EncodeToString(argsBytes)
	execCode := fmt.Sprintf(`import base64
import json
%s
args = json.loads(base64.b64decode(%q).decode("utf-8"))
result = %s(**args)
print(json.dumps(result, ensure_ascii=False))
`, prelude, argsB64, funcName)
	return skillCode + "\n" + execCode, nil
}

// ProvisionSkillDependencies scans all skills and installs their pip dependencies into the venv.
func ProvisionSkillDependencies(skillsDir, workspaceDir string, logger *slog.Logger) {
	skills, err := ListSkills(skillsDir)
	if err != nil {
		logger.Warn("Failed to scan skills for dependency provisioning", "error", err)
		return
	}

	// Aggregate unique dependencies
	seen := make(map[string]bool)
	var deps []string
	for _, s := range skills {
		for _, dep := range s.Dependencies {
			dep = strings.TrimSpace(dep)
			if dep != "" && !seen[dep] {
				seen[dep] = true
				deps = append(deps, dep)
			}
		}
	}

	if len(deps) == 0 {
		logger.Info("No skill dependencies to provision.")
		return
	}

	logger.Info("Provisioning skill dependencies", "packages", strings.Join(deps, ", "))

	// Ensure venv exists before installing
	if err := EnsureVenv(workspaceDir, logger); err != nil {
		logger.Error("Failed to ensure Python virtual environment", "error", err)
		return
	}

	pipBin := GetPipBin(workspaceDir)
	args := append([]string{"install"}, deps...)
	output, err := runTimedCommand(workspaceDir, skillDependencyInstallTimeout, pipBin, args...)
	if err != nil {
		logger.Error("Failed to provision skill dependencies", "error", err, "output", string(output))
		return
	}
	logger.Info("Skill dependencies provisioned successfully.")
}

// ──────────────────────────────────────────────────────────────────────────────
// Auto-dependency detection
// ──────────────────────────────────────────────────────────────────────────────

// importRe matches Python import statements at the start of a line.
var importRe = regexp.MustCompile(`^(?:import\s+(\w+)|from\s+(\w+)[\s.])`)

// mainBlockRe matches the standard Python main guard block start.
var mainBlockRe = regexp.MustCompile(`(?m)^if\s+__name__\s*==\s*['"]__main__['"]\s*:`)

// funcDefRe matches a Python function definition.
var funcDefRe = regexp.MustCompile(`\bdef\s+(%s)\s*\(`)

// pythonStdlib contains common Python stdlib module names that never need pip install.
var pythonStdlib = map[string]bool{
	"abc": true, "argparse": true, "ast": true, "asyncio": true, "base64": true,
	"binascii": true, "builtins": true, "bz2": true, "calendar": true, "cgi": true,
	"codecs": true, "collections": true, "colorsys": true, "concurrent": true,
	"configparser": true, "contextlib": true, "copy": true, "csv": true,
	"ctypes": true, "dataclasses": true, "datetime": true, "decimal": true,
	"difflib": true, "dis": true, "email": true, "enum": true, "errno": true,
	"fcntl": true, "fileinput": true, "fnmatch": true, "fractions": true,
	"ftplib": true, "functools": true, "getpass": true, "glob": true, "gzip": true,
	"hashlib": true, "heapq": true, "hmac": true, "html": true, "http": true,
	"imaplib": true, "importlib": true, "inspect": true, "io": true, "ipaddress": true,
	"itertools": true, "json": true, "keyword": true, "linecache": true,
	"locale": true, "logging": true, "lzma": true, "math": true, "mimetypes": true,
	"msvcrt": true, "multiprocessing": true, "netrc": true, "numbers": true,
	"operator": true, "os": true, "pathlib": true, "pickle": true, "platform": true,
	"plistlib": true, "pprint": true, "queue": true, "quopri": true, "random": true,
	"re": true, "readline": true, "reprlib": true, "resource": true, "rlcompleter": true,
	"sched": true, "secrets": true, "select": true, "selectors": true, "shelve": true,
	"shlex": true, "shutil": true, "signal": true, "site": true, "smtplib": true,
	"socket": true, "socketserver": true, "sqlite3": true, "ssl": true, "stat": true,
	"statistics": true, "string": true, "struct": true, "subprocess": true,
	"sys": true, "sysconfig": true, "syslog": true, "tarfile": true, "tempfile": true,
	"termios": true, "textwrap": true, "threading": true, "time": true, "timeit": true,
	"tkinter": true, "token": true, "tokenize": true, "tomllib": true, "trace": true,
	"traceback": true, "tracemalloc": true, "tty": true, "turtle": true, "types": true,
	"typing": true, "unicodedata": true, "unittest": true, "urllib": true, "uuid": true,
	"venv": true, "warnings": true, "wave": true, "webbrowser": true, "winreg": true,
	"xml": true, "xmlrpc": true, "zipfile": true, "zipimport": true, "zlib": true,
	"_thread": true, "__future__": true,
}

// importToPyPI maps Python import names to their PyPI package names
// when they differ from the import name.
var importToPyPI = map[string]string{
	"PIL":                "Pillow",
	"cv2":                "opencv-python",
	"bs4":                "beautifulsoup4",
	"yaml":               "pyyaml",
	"sklearn":            "scikit-learn",
	"dotenv":             "python-dotenv",
	"Crypto":             "pycryptodome",
	"serial":             "pyserial",
	"usb":                "pyusb",
	"gi":                 "PyGObject",
	"attr":               "attrs",
	"dateutil":           "python-dateutil",
	"jose":               "python-jose",
	"magic":              "python-magic",
	"docx":               "python-docx",
	"pptx":               "python-pptx",
	"lxml":               "lxml",
	"wx":                 "wxPython",
	"skimage":            "scikit-image",
	"fitz":               "PyMuPDF",
	"telegram":           "python-telegram-bot",
	"discord":            "discord.py",
	"flask":              "Flask",
	"django":             "Django",
	"fastapi":            "fastapi",
	"pydantic":           "pydantic",
	"sqlalchemy":         "SQLAlchemy",
	"toml":               "toml",
	"zmq":                "pyzmq",
	"nacl":               "PyNaCl",
	"git":                "GitPython",
	"paramiko":           "paramiko",
	"socks":              "PySocks",
	"chardet":            "chardet",
	"certifi":            "certifi",
	"idna":               "idna",
	"charset_normalizer": "charset-normalizer",
}

// extractImports scans a Python file and returns the set of top-level imported module names.
func extractImports(pyFilePath string) (map[string]bool, error) {
	f, err := os.Open(pyFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	imports := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024) // 1 MB max token size
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		matches := importRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		// matches[1] is from "import X", matches[2] is from "from X import ..."
		mod := matches[1]
		if mod == "" {
			mod = matches[2]
		}
		if mod != "" {
			imports[mod] = true
		}
	}
	return imports, scanner.Err()
}

// detectAndInstallMissingDeps scans a Python file for imports and auto-installs
// any missing third-party packages. Designed to fail gracefully.
func detectAndInstallMissingDeps(pyFilePath, workspaceDir string) {
	imports, err := extractImports(pyFilePath)
	if err != nil {
		slog.Debug("[AutoDeps] Failed to scan imports", "file", pyFilePath, "error", err)
		return
	}

	packages := pythonPackagesForImports(imports)
	if len(packages) == 0 {
		return
	}

	pipBin := GetPipBin(workspaceDir)
	installed, err := pipShowInstalledPackages(workspaceDir, pipBin, packages)
	if err != nil && len(installed) == 0 {
		slog.Debug("[AutoDeps] Failed to inspect installed packages", "packages", packages, "error", err)
		return
	}
	missing := missingPythonPackages(packages, installed)

	if len(missing) == 0 {
		return
	}

	slog.Info("[AutoDeps] Installing missing packages", "packages", strings.Join(missing, ", "), "file", filepath.Base(pyFilePath))
	args := append([]string{"install", "--quiet"}, missing...)
	if output, err := runTimedCommand(workspaceDir, skillDependencyInstallTimeout, pipBin, args...); err != nil {
		slog.Warn("[AutoDeps] Failed to install packages", "packages", missing, "error", err, "output", string(output))
	}
}

func pythonPackagesForImports(imports map[string]bool) []string {
	packageSet := make(map[string]struct{})
	for mod := range imports {
		if pythonStdlib[mod] {
			continue
		}
		pkg := mod
		if pypi, ok := importToPyPI[mod]; ok {
			pkg = pypi
		}
		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			packageSet[pkg] = struct{}{}
		}
	}
	packages := make([]string, 0, len(packageSet))
	for pkg := range packageSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages
}

func pipShowInstalledPackages(workspaceDir, pipBin string, packages []string) (map[string]bool, error) {
	if len(packages) == 0 {
		return map[string]bool{}, nil
	}
	args := append([]string{"show"}, packages...)
	output, err := runTimedCommand(workspaceDir, 45*time.Second, pipBin, args...)
	installed := parsePipShowInstalledPackages(output)
	return installed, err
}

func parsePipShowInstalledPackages(output []byte) map[string]bool {
	installed := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Name:") {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "Name:")))
		if name != "" {
			installed[name] = true
		}
	}
	return installed
}

func missingPythonPackages(packages []string, installed map[string]bool) []string {
	missing := make([]string, 0)
	for _, pkg := range packages {
		if !installed[strings.ToLower(pkg)] {
			missing = append(missing, pkg)
		}
	}
	return missing
}

func runTimedCommand(workdir string, timeout time.Duration, command string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %s", timeout)
	}
	return output, err
}

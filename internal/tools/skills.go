package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

// maxSkillArgsBytes limits the serialized size of skill arguments to prevent
// denial of service via excessively large JSON payloads.
const maxSkillArgsBytes = 10 * 1024 * 1024 // 10 MB
const skillDependencyInstallTimeout = 10 * time.Minute

// skillsCacheTTL is the time-to-live for the ListSkills cache.
const skillsCacheTTL = 5 * time.Second

// skillsCacheEntry holds cached skill manifests and their expiration time.
type skillsCacheEntry struct {
	skills    []SkillManifest
	expiresAt time.Time
}

// listSkillsCache is a simple TTL cache for skill manifests.
var listSkillsCache = struct {
	mu    sync.RWMutex
	entries map[string]skillsCacheEntry
}{
	entries: make(map[string]skillsCacheEntry),
}

// SkillManifest represents the structure of a skill config file (.json).
type SkillManifest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Executable   string            `json:"executable"`             // e.g., "scan.py" or "custom_tool.exe"
	Category     string            `json:"category,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Parameters   map[string]string `json:"parameters,omitempty"`   // map of arg name to description
	Returns      string            `json:"returns,omitempty"`      // describes expected output format
	Dependencies []string          `json:"dependencies,omitempty"` // pip packages required by this skill
	VaultKeys    []string          `json:"vault_keys,omitempty"`   // vault secret keys this skill needs at runtime
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

	// Update cache
	listSkillsCache.mu.Lock()
	listSkillsCache.entries[absDir] = skillsCacheEntry{
		skills:    skills,
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

// ExecuteSkill dynamically executes the requested skill script, routing Python scripts to the venv.
func ExecuteSkill(skillsDir, workspaceDir, skillName string, argsJSON map[string]interface{}) (string, error) {
	// First, lookup the skill manifest to find its executable
	skills, err := ListSkills(skillsDir)
	if err != nil {
		return "", fmt.Errorf("failed to scan skills: %v", err)
	}

	var manifest *SkillManifest
	for _, s := range skills {
		if s.Name == skillName {
			manifest = &s
			break
		}
	}

	if manifest == nil {
		return "", fmt.Errorf("skill '%s' not found", skillName)
	}

	// Validate manifest executable — must be a relative path within skillsDir (no traversal or absolute paths).
	if filepath.IsAbs(manifest.Executable) || strings.Contains(manifest.Executable, "..") {
		return "", fmt.Errorf("skill '%s' has invalid executable path '%s': must be a relative filename inside the skills directory", skillName, manifest.Executable)
	}

	// Ensure the skill executable path is absolute.
	// This is CRITICAL because cmd.Dir is set to workspaceDir, which would break relative paths.
	absExecPath, err := filepath.Abs(filepath.Join(skillsDir, manifest.Executable))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for skill '%s': %v", skillName, err)
	}

	if _, err := os.Stat(absExecPath); os.IsNotExist(err) {
		return "", fmt.Errorf("skill executable '%s' not found at %s", manifest.Executable, absExecPath)
	}

	if argsJSON == nil {
		argsJSON = make(map[string]interface{})
	}
	argsBytes, err := json.Marshal(argsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to serialize args JSON: %v", err)
	}
	if len(argsBytes) > maxSkillArgsBytes {
		return "", fmt.Errorf("skill args too large: %d bytes (max %d)", len(argsBytes), maxSkillArgsBytes)
	}
	argsString := string(argsBytes)
	slog.Debug("[ExecuteSkill] Prepared JSON input", "skill", skillName, "input", argsString)

	// Route based on extension
	ctx, cancel := context.WithTimeout(context.Background(), SkillTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if strings.HasSuffix(manifest.Executable, ".py") {
		cfgPythonBin := GetPythonBin(workspaceDir)
		cmd = exec.CommandContext(ctx, cfgPythonBin, "-u", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".sh") && runtime.GOOS != "windows" {
		cmd = exec.CommandContext(ctx, "bash", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".ps1") && runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", absExecPath)
	} else {
		// Attempt to run directly (e.g., .exe or native binary)
		cmd = exec.CommandContext(ctx, absExecPath)
	}

	cmd.Dir = workspaceDir
	SetSkillLimits(cmd, 1024, int(SkillTimeout.Seconds()))

	// Manual Stdin pipe management for maximum synchronization on Windows.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start skill execution: %v", err)
	}
	ApplySkillLimits(cmd.Process.Pid, 1024, int(SkillTimeout.Seconds()))

	// Write and CLOSE immediately to send EOF
	slog.Debug("[ExecuteSkill] Writing to Stdin...", "length", len(argsString))
	fmt.Fprint(stdin, argsString)
	if err := stdin.Close(); err != nil {
		slog.Error("[ExecuteSkill] Failed to close stdin pipe", "error", err)
	} else {
		slog.Debug("[ExecuteSkill] Stdin closed (EOF sent)")
	}

	err = cmd.Wait()
	output := outBuf.String()
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("TIMEOUT: skill '%s' exceeded 2-minute limit and was killed", skillName)
	}
	if err != nil {
		return output, fmt.Errorf("execution failed: %v", err)
	}

	return output, nil
}

// ExecuteSkillWithSecrets is like ExecuteSkill but injects vault secrets and credential secrets
// as environment variables and scrubs secrets from the output.
func ExecuteSkillWithSecrets(skillsDir, workspaceDir, skillName string, argsJSON map[string]interface{}, secrets map[string]string, creds []CredentialFields) (string, error) {
	skills, err := ListSkills(skillsDir)
	if err != nil {
		return "", fmt.Errorf("failed to scan skills: %v", err)
	}

	var manifest *SkillManifest
	for _, s := range skills {
		if s.Name == skillName {
			manifest = &s
			break
		}
	}
	if manifest == nil {
		return "", fmt.Errorf("skill '%s' not found", skillName)
	}

	absExecPath, err := filepath.Abs(filepath.Join(skillsDir, manifest.Executable))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for skill '%s': %v", skillName, err)
	}
	if _, err := os.Stat(absExecPath); os.IsNotExist(err) {
		return "", fmt.Errorf("skill executable '%s' not found at %s", manifest.Executable, absExecPath)
	}

	if argsJSON == nil {
		argsJSON = make(map[string]interface{})
	}
	argsBytes, err := json.Marshal(argsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to serialize args JSON: %v", err)
	}
	if len(argsBytes) > maxSkillArgsBytes {
		return "", fmt.Errorf("skill args too large: %d bytes (max %d)", len(argsBytes), maxSkillArgsBytes)
	}
	argsString := string(argsBytes)

	ctx, cancel := context.WithTimeout(context.Background(), SkillTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if strings.HasSuffix(manifest.Executable, ".py") {
		cfgPythonBin := GetPythonBin(workspaceDir)
		cmd = exec.CommandContext(ctx, cfgPythonBin, "-u", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".sh") && runtime.GOOS != "windows" {
		cmd = exec.CommandContext(ctx, "bash", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".ps1") && runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", absExecPath)
	} else {
		cmd = exec.CommandContext(ctx, absExecPath)
	}

	cmd.Dir = workspaceDir
	SetSkillLimits(cmd, 1024, int(SkillTimeout.Seconds()))
	InjectSecretsEnv(cmd, secrets)
	InjectCredentialEnv(cmd, creds)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start skill execution: %v", err)
	}
	ApplySkillLimits(cmd.Process.Pid, 1024, int(SkillTimeout.Seconds()))

	fmt.Fprint(stdin, argsString)
	stdin.Close()

	err = cmd.Wait()
	output := security.Scrub(outBuf.String())
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("TIMEOUT: skill '%s' exceeded 2-minute limit and was killed", skillName)
	}
	if err != nil {
		return output, fmt.Errorf("execution failed: %v", err)
	}

	return output, nil
}

// ExecuteSkillInSandbox executes a skill inside the sandboxed container environment.
// It reads the skill code, injects secrets/creds, prepends the args, and runs via SandboxExecuteCode.
// Returns error if sandbox is unavailable or skill not found.
func ExecuteSkillInSandbox(skillsDir, skillName string, argsJSON map[string]interface{}, secrets map[string]string, creds []CredentialFields, timeoutSeconds int, logger *slog.Logger) (string, error) {
	// Read skill code
	absExecPath := filepath.Join(skillsDir, skillName+".py")
	data, err := os.ReadFile(absExecPath)
	if err != nil {
		return "", fmt.Errorf("skill '%s' not found: %v", skillName, err)
	}
	skillCode := string(data)

	// Serialize args
	if argsJSON == nil {
		argsJSON = make(map[string]interface{})
	}
	argsBytes, err := json.Marshal(argsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to serialize args: %v", err)
	}
	if len(argsBytes) > maxSkillArgsBytes {
		return "", fmt.Errorf("skill args too large: %d bytes (max %d)", len(argsBytes), maxSkillArgsBytes)
	}

	// Build the execution script:
	// 1. Prepend secrets/creds injection
	// 2. Prepend args as a dict
	// 3. The skill's if __name__ == "__main__" block will be skipped since we're executing as a module
	// 4. We need to explicitly call the skill function

	// Build secrets/creds prelude
	prelude := ""
	if len(secrets) > 0 {
		prelude += BuildSecretPrelude(secrets)
	}
	if len(creds) > 0 {
		prelude += BuildCredentialPrelude(creds)
	}

	// Build the call: we need to extract the function call from the if __name__ block
	// For simplicity, we inject args and then call the function directly
	// The skill templates have a consistent pattern: they call the function with kwargs
	funcName := toFunctionName(skillName)
	execCode := fmt.Sprintf(`import json
%s
args = json.loads('%s')
result = %s(**args)
print(json.dumps(result, ensure_ascii=False))
`, prelude, strings.ReplaceAll(string(argsBytes), `'`, `\'`), funcName)

	// Prepend the skill code (without the if __name__ block)
	// Split on "if __name__" and only keep the function definition
	mainIdx := strings.Index(skillCode, "if __name__")
	if mainIdx > 0 {
		skillCode = skillCode[:mainIdx]
	}

	// Full code: skill code + exec code
	fullCode := skillCode + "\n" + execCode

	// Execute via sandbox
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	result, err := SandboxExecuteCode(fullCode, "python", nil, timeoutSeconds, logger)
	if err != nil {
		return "", fmt.Errorf("sandbox execution failed: %v", err)
	}
	return result, nil
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

	// Determine which imports are third-party and need pip packages
	var missing []string
	pipBin := GetPipBin(workspaceDir)

	for mod := range imports {
		if pythonStdlib[mod] {
			continue
		}
		// Resolve PyPI package name
		pkg := mod
		if pypi, ok := importToPyPI[mod]; ok {
			pkg = pypi
		}
		// Check if already installed
		if _, err := runTimedCommand(workspaceDir, 45*time.Second, pipBin, "show", pkg); err != nil {
			missing = append(missing, pkg)
		}
	}

	if len(missing) == 0 {
		return
	}

	slog.Info("[AutoDeps] Installing missing packages", "packages", strings.Join(missing, ", "), "file", filepath.Base(pyFilePath))
	args := append([]string{"install", "--quiet"}, missing...)
	if output, err := runTimedCommand(workspaceDir, skillDependencyInstallTimeout, pipBin, args...); err != nil {
		slog.Warn("[AutoDeps] Failed to install packages", "packages", missing, "error", err, "output", string(output))
	}
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

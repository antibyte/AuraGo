package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/sandbox"
)

const DefaultGitHubRepo = "antibyte/AuraGo"

var (
	ErrSelfUpdateDisabled = errors.New("self-update disabled")
	ErrDockerRuntime      = errors.New("self-update unavailable in docker")
	ErrUnsupportedOS      = errors.New("self-update unsupported on this OS")
	ErrMissingScript      = errors.New("update script missing")
	ErrMissingBash        = errors.New("bash missing")
)

// CheckResult is the shared payload returned by update checks.
type CheckResult struct {
	Mode            string `json:"mode"` // "git" or "binary"
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	CommitCount     int    `json:"commit_count,omitempty"`
	Changelog       string `json:"changelog,omitempty"`
	Message         string `json:"message"`
	Error           string `json:"error,omitempty"`
}

type CommandRunner func(dir string, name string, args ...string) ([]byte, error)

type CheckOptions struct {
	InstallDir    string
	GitHubRepo    string
	ReleaseAPIURL string
	HTTPClient    *http.Client
	RunCommand    CommandRunner
}

type ScriptLaunch struct {
	Dir        string
	BashPath   string
	ScriptPath string
	LogPath    string
	Env        []string
}

type StartInstallOptions struct {
	Cfg         *config.Config
	InstallDir  string
	GOOS        string
	LookPath    func(string) (string, error)
	StartScript func(ScriptLaunch) error
	Env         []string
}

type InstallStartResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	LogPath string `json:"log_path,omitempty"`
}

// InstallDir returns the installation directory from config path, falling back to cwd.
func InstallDir(configPath string) string {
	if strings.TrimSpace(configPath) != "" {
		return filepath.Dir(configPath)
	}
	d, _ := os.Getwd()
	return d
}

// IsBinaryInstall returns true when the install dir has no .git directory.
func IsBinaryInstall(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return os.IsNotExist(err)
}

// ReadInstalledVersion reads the .version file written by install.sh / update.sh.
func ReadInstalledVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".version"))
	if err != nil {
		return "unknown"
	}
	if version := strings.TrimSpace(string(data)); version != "" {
		return version
	}
	return "unknown"
}

func CheckUpdates(ctx context.Context, opts CheckOptions) CheckResult {
	if opts.InstallDir == "" {
		opts.InstallDir = InstallDir("")
	}
	if IsBinaryInstall(opts.InstallDir) {
		return checkUpdatesBinary(ctx, opts)
	}
	return checkUpdatesGit(ctx, opts)
}

func checkUpdatesBinary(ctx context.Context, opts CheckOptions) CheckResult {
	current := ReadInstalledVersion(opts.InstallDir)
	latest, _, err := fetchLatestRelease(ctx, opts)
	if err != nil {
		return CheckResult{
			Mode:    "binary",
			Error:   "Could not reach GitHub",
			Message: "Update check failed.",
		}
	}

	available := latest != "" && current != latest
	msg := "AuraGo is up to date."
	if current == "unknown" {
		msg = "Installed version could not be determined. Latest available: " + latest
	}
	if available && current != "unknown" {
		msg = fmt.Sprintf("Update available: %s -> %s", current, latest)
	}

	return CheckResult{
		Mode:            "binary",
		UpdateAvailable: available,
		CurrentVersion:  current,
		LatestVersion:   latest,
		Message:         msg,
	}
}

func checkUpdatesGit(ctx context.Context, opts CheckOptions) CheckResult {
	run := opts.RunCommand
	if run == nil {
		run = runCommand
	}
	dir := opts.InstallDir
	safeDir := "safe.directory=" + dir

	current := "unknown"
	if out, err := run(dir, "git", "-c", safeDir, "describe", "--tags", "--always"); err == nil {
		if described := strings.TrimSpace(string(out)); described != "" {
			current = described
		}
	}

	if _, err := run(dir, "git", "-c", safeDir, "fetch", "origin", "main", "--quiet"); err != nil {
		return CheckResult{Mode: "git", Error: "Could not reach GitHub", Message: "Update check failed."}
	}

	countOut, err := run(dir, "git", "-c", safeDir, "rev-list", "HEAD..origin/main", "--count")
	if err != nil {
		return CheckResult{Mode: "git", Error: "Failed to determine pending updates", Message: "Update check failed."}
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(countOut)))
	latest, _, _ := fetchLatestRelease(ctx, opts)

	if count == 0 {
		return CheckResult{
			Mode:            "git",
			UpdateAvailable: false,
			CurrentVersion:  current,
			LatestVersion:   latest,
			Message:         "AuraGo is up to date.",
		}
	}

	logOut, _ := run(dir, "git", "-c", safeDir, "log", "HEAD..origin/main", "--oneline", "-n", "10")
	return CheckResult{
		Mode:            "git",
		UpdateAvailable: true,
		CurrentVersion:  current,
		LatestVersion:   latest,
		CommitCount:     count,
		Changelog:       strings.TrimSpace(string(logOut)),
		Message:         fmt.Sprintf("%d update(s) available.", count),
	}
}

func fetchLatestRelease(ctx context.Context, opts CheckOptions) (tag string, body string, err error) {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	url := opts.ReleaseAPIURL
	if url == "" {
		repo := opts.GitHubRepo
		if repo == "" {
			repo = DefaultGitHubRepo
		}
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AuraGo-Updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", string(raw), fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(raw, &release); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return "", release.Body, errors.New("GitHub release response missing tag_name")
	}
	return strings.TrimSpace(release.TagName), release.Body, nil
}

func runCommand(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = updateEnv(nil)
	return cmd.CombinedOutput()
}

func ValidateInstall(cfg *config.Config, installDir, goos string, lookPath func(string) (string, error)) error {
	if cfg == nil || !cfg.Agent.AllowSelfUpdate {
		return fmt.Errorf("%w: Self-update is disabled in the agent safety settings.", ErrSelfUpdateDisabled)
	}
	if cfg.Runtime.IsDocker {
		return fmt.Errorf("%w: Self-updates are disabled in Docker installations. Update the container image and recreate the container instead.", ErrDockerRuntime)
	}
	if goos != "linux" {
		return fmt.Errorf("%w: Self-updates from the app are only supported on Linux installations.", ErrUnsupportedOS)
	}
	if _, err := os.Stat(filepath.Join(installDir, "update.sh")); err != nil {
		return fmt.Errorf("%w: update.sh not found in installation directory. Download it from: https://raw.githubusercontent.com/%s/main/update.sh", ErrMissingScript, DefaultGitHubRepo)
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath("bash"); err != nil {
		return fmt.Errorf("%w: bash is required to run update.sh", ErrMissingBash)
	}
	return nil
}

func StartInstall(opts StartInstallOptions) (InstallStartResult, error) {
	if opts.InstallDir == "" && opts.Cfg != nil {
		opts.InstallDir = InstallDir(opts.Cfg.ConfigPath)
	}
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	if err := ValidateInstall(opts.Cfg, opts.InstallDir, opts.GOOS, opts.LookPath); err != nil {
		return InstallStartResult{}, err
	}
	bashPath, err := opts.LookPath("bash")
	if err != nil {
		return InstallStartResult{}, fmt.Errorf("%w: bash is required to run update.sh", ErrMissingBash)
	}

	logDir := filepath.Join(opts.InstallDir, "log")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return InstallStartResult{}, fmt.Errorf("create update log directory: %w", err)
	}
	logPath := filepath.Join(logDir, "update.log")
	start := opts.StartScript
	if start == nil {
		start = DefaultStartScript
	}
	env := opts.Env
	if env == nil {
		env = updateEnv(nil)
	}
	if err := start(ScriptLaunch{
		Dir:        opts.InstallDir,
		BashPath:   bashPath,
		ScriptPath: filepath.Join(opts.InstallDir, "update.sh"),
		LogPath:    logPath,
		Env:        env,
	}); err != nil {
		return InstallStartResult{}, err
	}

	return InstallStartResult{
		Status:  "started",
		Message: "Update started. Progress is logged to log/update.log. AuraGo will restart automatically once the update is complete.",
		LogPath: logPath,
	}, nil
}

func DefaultStartScript(launch ScriptLaunch) error {
	wrapper := fmt.Sprintf(`nohup %q %q --yes < /dev/null >> %q 2>&1 &`,
		launch.BashPath, launch.ScriptPath, launch.LogPath)
	cmd := exec.Command(launch.BashPath, "-c", wrapper)
	cmd.Dir = launch.Dir
	cmd.Env = launch.Env
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func updateEnv(extra []string) []string {
	env := sandbox.FilterEnv(os.Environ())
	if masterKey := os.Getenv("AURAGO_MASTER_KEY"); strings.TrimSpace(masterKey) != "" {
		env = append(env, "AURAGO_MASTER_KEY="+masterKey)
	}
	if home, _ := os.UserHomeDir(); home != "" {
		env = append(env, "HOME="+home)
	}
	return append(env, extra...)
}

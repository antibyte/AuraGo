//go:build linux

package sandbox

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var executablePath = os.Executable

// defaultReadOnlyPaths are always mounted read-only in the sandbox.
var defaultReadOnlyPaths = []string{
	"/bin",
	"/usr/bin",
	"/usr/lib",
	"/usr/lib64",
	"/usr/libexec",
	"/usr/share/python-wheels", // Debian/Ubuntu keep ensurepip's bundled wheels here.
	"/lib",
	"/lib64",
	"/etc/alternatives",
	"/etc/ld.so.cache",
	"/etc/ld.so.conf",
	"/etc/ld.so.conf.d",
	"/etc/passwd",         // many tools need this for user name lookups
	"/etc/group",          // many tools need this for group name lookups
	"/etc/debian_version", // pip's distro metadata probe reads this on Debian/Ubuntu.
	"/etc/mime.types",     // Python's MIME database is used while pip selects wheels.
	"/etc/nsswitch.conf",
	"/etc/resolv.conf",
	"/etc/hosts",
	"/etc/ssl",
	"/etc/ca-certificates",
	"/etc/pki",
}

// defaultReadWritePaths are always mounted read-write (if they exist).
var defaultReadWritePaths = []string{
	"/tmp",
	"/dev/null",
	"/dev/zero",
	"/dev/urandom",
	"/dev/random",
}

// LandlockSandbox implements ShellSandbox using the Landlock LSM and prlimit.
type LandlockSandbox struct {
	cfg          ShellSandboxConfig
	caps         Capabilities
	workspaceDir string
	logger       *slog.Logger
}

// NewLandlockSandbox creates a new Landlock-based sandbox.
func NewLandlockSandbox(cfg ShellSandboxConfig, caps Capabilities, workspaceDir string, logger *slog.Logger) *LandlockSandbox {
	return &LandlockSandbox{
		cfg:          cfg,
		caps:         caps,
		workspaceDir: workspaceDir,
		logger:       logger,
	}
}

func (s *LandlockSandbox) Available() bool { return s.caps.LandlockABI >= 3 }
func (s *LandlockSandbox) Name() string    { return "landlock" }

// PrepareCommand returns an exec.Cmd that re-invokes the AuraGo binary in
// sandbox-exec mode. The helper process applies Landlock restrictions and
// resource limits to itself before exec'ing the actual shell command.
func (s *LandlockSandbox) PrepareCommand(command, workDir string) *exec.Cmd {
	selfBin, err := executablePath()
	if err != nil {
		s.logger.Error("Cannot determine own executable path for sandbox", "error", err)
		cmd := blockedCommand("shell sandbox could not determine AuraGo executable: " + err.Error())
		cmd.Dir = workDir
		return cmd
	}

	// Pass sandbox config via environment variables — avoids complex arg parsing
	// and doesn't expose config in /proc/PID/cmdline beyond what's necessary.
	env, err := s.buildEnv(workDir, nil)
	if err != nil {
		s.logger.Error("Cannot build sandbox environment", "error", err)
		cmd := blockedCommand("shell sandbox rejected a path: " + err.Error())
		cmd.Dir = workDir
		return cmd
	}

	cmd := exec.Command(selfBin, "--sandbox-exec", command)
	cmd.Dir = workDir
	cmd.Env = env
	return cmd
}

// PrepareExecCommand returns an exec.Cmd that re-invokes the AuraGo binary in
// sandbox-exec-bin mode and then execs the target binary directly.
func (s *LandlockSandbox) PrepareExecCommand(binary string, args []string, workDir string) *exec.Cmd {
	selfBin, err := executablePath()
	if err != nil {
		s.logger.Error("Cannot determine own executable path for sandbox", "error", err)
		cmd := blockedCommand("shell sandbox could not determine AuraGo executable: " + err.Error())
		cmd.Dir = workDir
		return cmd
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		s.logger.Error("Cannot encode sandbox exec args", "error", err)
		cmd := blockedCommand("shell sandbox could not encode exec args: " + err.Error())
		cmd.Dir = workDir
		return cmd
	}

	env, err := s.buildEnv(workDir, []string{
		"AURAGO_SBX_EXEC_BIN=" + binary,
		"AURAGO_SBX_EXEC_ARGS=" + string(argsJSON),
	})
	if err != nil {
		s.logger.Error("Cannot build sandbox environment", "error", err)
		cmd := blockedCommand("shell sandbox rejected a path: " + err.Error())
		cmd.Dir = workDir
		return cmd
	}

	cmd := exec.Command(selfBin, "--sandbox-exec-bin")
	cmd.Dir = workDir
	cmd.Env = env
	return cmd
}

// buildEnv creates the environment for the sandboxed helper process.
// The work directory and every configured allowed path are canonicalized and
// listed as required. Built-in defaults are canonicalized and stay optional.
// An empty work directory is omitted: it is not a path, and the helper already
// treats a missing work-directory variable as ".".
func (s *LandlockSandbox) buildEnv(workDir string, extra []string) ([]string, error) {
	var rwPaths, roPaths, rwRequired, roRequired []string
	home := workDir
	if workDir != "" {
		canon, err := canonicalSandboxPath(workDir)
		if err != nil {
			return nil, err
		}
		home = canon
		rwPaths = append(rwPaths, canon)
		rwRequired = append(rwRequired, canon)
	}
	for _, path := range defaultReadWritePaths {
		if err := appendSandboxPath(&rwPaths, &rwRequired, path, false); err != nil {
			return nil, err
		}
	}
	for _, path := range defaultReadOnlyPaths {
		if err := appendSandboxPath(&roPaths, &roRequired, path, false); err != nil {
			return nil, err
		}
	}
	for _, rule := range s.cfg.AllowedPaths {
		if rule.ReadOnly {
			if err := appendSandboxPath(&roPaths, &roRequired, rule.Path, true); err != nil {
				return nil, err
			}
			continue
		}
		if err := appendSandboxPath(&rwPaths, &rwRequired, rule.Path, true); err != nil {
			return nil, err
		}
	}

	env := []string{
		// Minimal environment for shell commands
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + home,
		"TERM=dumb",
		"LANG=C.UTF-8",
	}

	// Sandbox configuration. Path lists stay colon-separated, and only after
	// every entry has been rejected or canonicalized.
	env = append(env, "AURAGO_SBX_WORKDIR="+home)
	env = append(env, "AURAGO_SBX_MEM="+strconv.Itoa(s.cfg.MaxMemoryMB))
	env = append(env, "AURAGO_SBX_CPU="+strconv.Itoa(s.cfg.MaxCPUSeconds))
	env = append(env, "AURAGO_SBX_PROCS="+strconv.Itoa(s.cfg.MaxProcesses))
	env = append(env, "AURAGO_SBX_FSIZE="+strconv.Itoa(s.cfg.MaxFileSizeMB))
	env = append(env, "AURAGO_SBX_ABI="+strconv.Itoa(s.caps.LandlockABI))
	env = append(env, "AURAGO_SBX_RW="+strings.Join(rwPaths, ":"))
	env = append(env, "AURAGO_SBX_RO="+strings.Join(roPaths, ":"))
	env = append(env, "AURAGO_SBX_RW_REQUIRED="+strings.Join(rwRequired, ":"))
	env = append(env, "AURAGO_SBX_RO_REQUIRED="+strings.Join(roRequired, ":"))

	// Additional env vars requested by the caller (e.g. DOCKER_HOST).
	// The Docker socket is not granted here.
	env = append(env, s.cfg.ExtraEnv...)
	env = append(env, extra...)

	return env, nil
}

// canonicalSandboxPath rejects an empty path and any path that cannot be
// stored in a colon-separated list. An existing path is made absolute and
// then resolved through filepath.EvalSymlinks. The helper changes directory
// before it opens rules, so the Landlock list must hold that absolute
// result. A missing path keeps its cleaned absolute form so an optional
// built-in default can be skipped later on ENOENT. The string placed in the
// list is rejected again when it contains ':', a newline, or NUL.
func canonicalSandboxPath(path string) (string, error) {
	if path == "" || strings.ContainsAny(path, ":\n\x00") {
		return "", fmt.Errorf("rejected sandbox path %q", path)
	}
	var resolved string
	if _, err := os.Lstat(path); err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("sandbox path %q: %w", path, err)
		}
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return "", fmt.Errorf("sandbox path %q: %w", path, absErr)
		}
		resolved = filepath.Clean(abs)
	} else {
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return "", fmt.Errorf("sandbox path %q: %w", path, absErr)
		}
		var err error
		resolved, err = filepath.EvalSymlinks(abs)
		if err != nil {
			return "", fmt.Errorf("sandbox path %q: %w", path, err)
		}
	}
	if strings.ContainsAny(resolved, ":\n\x00") {
		return "", fmt.Errorf("rejected sandbox path %q", resolved)
	}
	return resolved, nil
}

func appendSandboxPath(paths *[]string, required *[]string, path string, isRequired bool) error {
	canon, err := canonicalSandboxPath(path)
	if err != nil {
		return err
	}
	*paths = append(*paths, canon)
	if isRequired {
		*required = append(*required, canon)
	}
	return nil
}

// FormatHelperEnvDebug returns a debug string showing the sandbox environment.
// Only used for logging; never expose in production.
func FormatHelperEnvDebug(env []string) string {
	var sb strings.Builder
	for _, e := range env {
		if strings.HasPrefix(e, "AURAGO_SBX_") {
			fmt.Fprintln(&sb, e)
		}
	}
	return sb.String()
}

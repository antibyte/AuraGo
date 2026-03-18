//go:build linux

package sandbox

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// defaultReadOnlyPaths are always mounted read-only in the sandbox.
var defaultReadOnlyPaths = []string{
	"/bin",
	"/usr/bin",
	"/usr/lib",
	"/usr/lib64",
	"/usr/libexec",
	"/lib",
	"/lib64",
	"/etc/alternatives",
	"/etc/ld.so.cache",
	"/etc/ld.so.conf",
	"/etc/ld.so.conf.d",
	"/etc/passwd", // many tools need this for user name lookups
	"/etc/group",  // many tools need this for group name lookups
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

func (s *LandlockSandbox) Available() bool { return s.caps.LandlockABI >= 1 }
func (s *LandlockSandbox) Name() string    { return "landlock" }

// PrepareCommand returns an exec.Cmd that re-invokes the AuraGo binary in
// sandbox-exec mode. The helper process applies Landlock restrictions and
// resource limits to itself before exec'ing the actual shell command.
func (s *LandlockSandbox) PrepareCommand(command, workDir string) *exec.Cmd {
	selfBin, err := os.Executable()
	if err != nil {
		s.logger.Error("Cannot determine own executable path for sandbox", "error", err)
		// Fallback to direct execution
		cmd := exec.Command("/bin/sh", "-c", command)
		cmd.Dir = workDir
		return cmd
	}

	cmd := exec.Command(selfBin, "--sandbox-exec", command)
	cmd.Dir = workDir

	// Pass sandbox config via environment variables — avoids complex arg parsing
	// and doesn't expose config in /proc/PID/cmdline beyond what's necessary.
	cmd.Env = s.buildEnv(workDir)

	return cmd
}

// buildEnv creates the environment for the sandboxed helper process.
func (s *LandlockSandbox) buildEnv(workDir string) []string {
	env := []string{
		// Minimal environment for shell commands
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + workDir,
		"TERM=dumb",
		"LANG=C.UTF-8",
	}

	// Sandbox configuration
	env = append(env, "AURAGO_SBX_WORKDIR="+workDir)
	env = append(env, "AURAGO_SBX_MEM="+strconv.Itoa(s.cfg.MaxMemoryMB))
	env = append(env, "AURAGO_SBX_CPU="+strconv.Itoa(s.cfg.MaxCPUSeconds))
	env = append(env, "AURAGO_SBX_PROCS="+strconv.Itoa(s.cfg.MaxProcesses))
	env = append(env, "AURAGO_SBX_FSIZE="+strconv.Itoa(s.cfg.MaxFileSizeMB))

	// Build allowed paths: rw and ro separated
	var rwPaths, roPaths []string

	// Workspace is always read-write
	rwPaths = append(rwPaths, workDir)
	// Default read-write paths
	rwPaths = append(rwPaths, defaultReadWritePaths...)
	// Default read-only paths
	roPaths = append(roPaths, defaultReadOnlyPaths...)

	// User-configured additional paths
	for _, rule := range s.cfg.AllowedPaths {
		if rule.ReadOnly {
			roPaths = append(roPaths, rule.Path)
		} else {
			rwPaths = append(rwPaths, rule.Path)
		}
	}

	env = append(env, "AURAGO_SBX_RW="+strings.Join(rwPaths, ":"))
	env = append(env, "AURAGO_SBX_RO="+strings.Join(roPaths, ":"))

	return env
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

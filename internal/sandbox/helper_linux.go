//go:build linux

package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// RunHelper is the entry point for the sandbox helper mode (--sandbox-exec).
// It applies Landlock filesystem restrictions and resource limits to the
// current process, then replaces itself with /bin/sh -c <command>.
// This function never returns on success (syscall.Exec replaces the process).
func RunHelper(command string) {
	workDir := os.Getenv("AURAGO_SBX_WORKDIR")
	if workDir == "" {
		workDir = "."
	}

	if err := os.Chdir(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: chdir %s: %v\n", workDir, err)
		os.Exit(126)
	}

	// Apply resource limits (before Landlock, so /proc is still accessible)
	applyRlimits()

	// Apply Landlock filesystem restrictions
	if err := applyLandlock(); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: landlock: %v\n", err)
		os.Exit(126)
	}

	// Replace this process with the shell command.
	// Landlock restrictions and rlimits are inherited.
	shell := "/bin/sh"
	argv := []string{shell, "-c", command}
	env := FilterEnv(os.Environ())

	err := syscall.Exec(shell, argv, env)
	// If we get here, exec failed
	fmt.Fprintf(os.Stderr, "sandbox: exec %s: %v\n", shell, err)
	os.Exit(126)
}

// RunExecHelper is the entry point for direct binary execution in sandbox helper mode.
func RunExecHelper() {
	workDir := os.Getenv("AURAGO_SBX_WORKDIR")
	if workDir == "" {
		workDir = "."
	}

	if err := os.Chdir(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: chdir %s: %v\n", workDir, err)
		os.Exit(126)
	}

	applyRlimits()

	if err := applyLandlock(); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: landlock: %v\n", err)
		os.Exit(126)
	}

	binary := os.Getenv("AURAGO_SBX_EXEC_BIN")
	if binary == "" {
		fmt.Fprintln(os.Stderr, "sandbox: missing exec binary")
		os.Exit(126)
	}

	argsJSON := os.Getenv("AURAGO_SBX_EXEC_ARGS")
	var args []string
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox: decode exec args: %v\n", err)
			os.Exit(126)
		}
	}

	resolvedBinary, err := exec.LookPath(binary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: lookpath %s: %v\n", binary, err)
		os.Exit(126)
	}

	argv := append([]string{binary}, args...)
	env := FilterEnv(os.Environ())
	err = syscall.Exec(resolvedBinary, argv, env)
	fmt.Fprintf(os.Stderr, "sandbox: exec %s: %v\n", resolvedBinary, err)
	os.Exit(126)
}

// ── Landlock ───────────────────────────────────────────────────────────────

// applyLandlock creates a Landlock ruleset, adds path rules, and restricts
// the current process. Requires no privileges (Linux 5.13+).
func applyLandlock() error {
	// Set no_new_privs — required before landlock_restrict_self
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("prctl(NO_NEW_PRIVS): %w", err)
	}

	// Filesystem access flags handled by the ruleset.
	// Anything not explicitly allowed is denied.
	handledAccess := uint64(
		unix.LANDLOCK_ACCESS_FS_EXECUTE |
			unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
			unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
			unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
			unix.LANDLOCK_ACCESS_FS_MAKE_REG |
			unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
			unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_SYM,
	)

	attr := unix.LandlockRulesetAttr{
		Access_fs: handledAccess,
	}

	rulesetFd, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)),
		unsafe.Sizeof(attr),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_create_ruleset: %v", errno)
	}
	defer unix.Close(int(rulesetFd))

	// Read-write paths
	rwPaths := splitPaths(os.Getenv("AURAGO_SBX_RW"))
	rwAccess := handledAccess // full access

	for _, p := range rwPaths {
		if err := landlockAddPath(int(rulesetFd), p, rwAccess); err != nil {
			// Non-existent paths are silently skipped (e.g. /usr/lib64 on some distros)
			continue
		}
	}

	// Read-only paths
	roPaths := splitPaths(os.Getenv("AURAGO_SBX_RO"))
	roAccess := uint64(
		unix.LANDLOCK_ACCESS_FS_EXECUTE |
			unix.LANDLOCK_ACCESS_FS_READ_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_DIR,
	)

	for _, p := range roPaths {
		if err := landlockAddPath(int(rulesetFd), p, roAccess); err != nil {
			continue
		}
	}

	// Restrict self
	_, _, errno = unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		rulesetFd,
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %v", errno)
	}

	return nil
}

// landlockAddPath adds a path-beneath rule to the ruleset.
func landlockAddPath(rulesetFd int, path string, access uint64) error {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	pathAttr := unix.LandlockPathBeneathAttr{
		Allowed_access: access,
		Parent_fd:      int32(fd),
	}

	_, _, errno := unix.Syscall6(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(rulesetFd),
		unix.LANDLOCK_RULE_PATH_BENEATH,
		uintptr(unsafe.Pointer(&pathAttr)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_add_rule(%s): %v", path, errno)
	}
	return nil
}

// ── Resource Limits ────────────────────────────────────────────────────────

// applyRlimits sets resource limits on the current process based on env vars.
// Errors are written to stderr rather than silently ignored so that sandbox
// security misconfigurations are visible in the process output.
func applyRlimits() {
	if v := envInt("AURAGO_SBX_MEM"); v > 0 {
		lim := &unix.Rlimit{
			Cur: uint64(v) * 1024 * 1024,
			Max: uint64(v) * 1024 * 1024,
		}
		if err := unix.Setrlimit(unix.RLIMIT_AS, lim); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox: setrlimit(RLIMIT_AS, %dMB): %v\n", v, err)
		}
	}

	if v := envInt("AURAGO_SBX_CPU"); v > 0 {
		lim := &unix.Rlimit{
			Cur: uint64(v),
			Max: uint64(v),
		}
		if err := unix.Setrlimit(unix.RLIMIT_CPU, lim); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox: setrlimit(RLIMIT_CPU, %ds): %v\n", v, err)
		}
	}

	if v := envInt("AURAGO_SBX_PROCS"); v > 0 {
		lim := &unix.Rlimit{
			Cur: uint64(v),
			Max: uint64(v),
		}
		if err := unix.Setrlimit(unix.RLIMIT_NPROC, lim); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox: setrlimit(RLIMIT_NPROC, %d): %v\n", v, err)
		}
	}

	if v := envInt("AURAGO_SBX_FSIZE"); v > 0 {
		lim := &unix.Rlimit{
			Cur: uint64(v) * 1024 * 1024,
			Max: uint64(v) * 1024 * 1024,
		}
		if err := unix.Setrlimit(unix.RLIMIT_FSIZE, lim); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox: setrlimit(RLIMIT_FSIZE, %dMB): %v\n", v, err)
		}
	}
}

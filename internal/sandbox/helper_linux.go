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

	// Apply resource limits (before Landlock, so /proc is still accessible).
	// A failed limit exits before Landlock, same as a Landlock error.
	if err := applyRlimits(); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: %v\n", err)
		os.Exit(126)
	}

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

	if err := applyRlimits(); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: %v\n", err)
		os.Exit(126)
	}

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
	if envInt("AURAGO_SBX_ABI") < 3 {
		return fmt.Errorf("Landlock ABI 3 or newer is required for truncate protection")
	}
	// Set no_new_privs — required before landlock_restrict_self
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("prctl(NO_NEW_PRIVS): %w", err)
	}

	handledAccess := landlockHandledAccessForABI(envInt("AURAGO_SBX_ABI"))

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

func landlockHandledAccessForABI(abi int) uint64 {
	// Filesystem access flags handled by the ruleset.
	// Anything not explicitly handled is not mediated by Landlock.
	access := uint64(
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
	if abi >= 2 {
		access |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		access |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	if abi >= 5 {
		access |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
	}
	return access
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

// plannedRlimits returns the resource ids applyRlimits will set.
// RLIMIT_NPROC is never included: Linux counts that limit per user, including
// AuraGo's own threads, so AURAGO_SBX_PROCS is ignored.
func plannedRlimits() []int {
	var resources []int
	if envInt("AURAGO_SBX_MEM") > 0 {
		resources = append(resources, unix.RLIMIT_AS)
	}
	if envInt("AURAGO_SBX_CPU") > 0 {
		resources = append(resources, unix.RLIMIT_CPU)
	}
	if envInt("AURAGO_SBX_FSIZE") > 0 {
		resources = append(resources, unix.RLIMIT_FSIZE)
	}
	return resources
}

// applyRlimits sets memory, CPU, and file-size limits on the current process.
// A failed setrlimit is returned so the helper exits before Landlock.
func applyRlimits() error {
	for _, resource := range plannedRlimits() {
		var lim unix.Rlimit
		var label string
		switch resource {
		case unix.RLIMIT_AS:
			v := envInt("AURAGO_SBX_MEM")
			bytes := uint64(v) * 1024 * 1024
			lim = unix.Rlimit{Cur: bytes, Max: bytes}
			label = fmt.Sprintf("RLIMIT_AS, %dMB", v)
		case unix.RLIMIT_CPU:
			v := envInt("AURAGO_SBX_CPU")
			seconds := uint64(v)
			lim = unix.Rlimit{Cur: seconds, Max: seconds}
			label = fmt.Sprintf("RLIMIT_CPU, %ds", v)
		case unix.RLIMIT_FSIZE:
			v := envInt("AURAGO_SBX_FSIZE")
			bytes := uint64(v) * 1024 * 1024
			lim = unix.Rlimit{Cur: bytes, Max: bytes}
			label = fmt.Sprintf("RLIMIT_FSIZE, %dMB", v)
		default:
			return fmt.Errorf("unsupported rlimit %d", resource)
		}
		if err := unix.Setrlimit(resource, &lim); err != nil {
			return fmt.Errorf("setrlimit(%s): %w", label, err)
		}
	}
	return nil
}

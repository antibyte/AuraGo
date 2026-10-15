//go:build linux

package sandbox

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestLandlockRulesAcceptDirectoryFileDeviceAndSocket(t *testing.T) {
	abi := Detect().LandlockABI
	if abi < 3 {
		t.Skip("Landlock ABI 3 or newer is required")
	}
	access := landlockHandledAccessForABI(abi)
	attr := unix.LandlockRulesetAttr{Handled_access_fs: access}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		t.Fatalf("create ruleset: %v", errno)
	}
	defer unix.Close(int(fd))
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "socket")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	for _, path := range []string{dir, file, "/dev/null", socket} {
		for _, rights := range []uint64{access, unix.LANDLOCK_ACCESS_FS_EXECUTE | unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR} {
			if err := landlockAddPath(int(fd), path, rights); err != nil {
				t.Errorf("add path %s with rights %#x: %v", path, rights, err)
			}
		}
	}
}

func TestRlimitPlanSkipsProcessCount(t *testing.T) {
	t.Setenv("AURAGO_SBX_PROCS", "50")
	t.Setenv("AURAGO_SBX_MEM", "1024")
	t.Setenv("AURAGO_SBX_CPU", "30")
	t.Setenv("AURAGO_SBX_FSIZE", "1")
	resources := plannedRlimits()
	for _, resource := range resources {
		if resource == unix.RLIMIT_NPROC {
			t.Fatal("RLIMIT_NPROC must not be applied")
		}
	}
	if !containsResource(resources, unix.RLIMIT_AS) {
		t.Fatal("expected RLIMIT_AS")
	}
	if resources[len(resources)-1] != unix.RLIMIT_AS {
		t.Fatal("address-space limit must be applied last")
	}
}

func TestSandboxHelperEnforcesIsolationAndLowMemoryLimit(t *testing.T) {
	if mode := os.Getenv("AURAGO_TEST_HELPER_MODE"); mode != "" {
		if mode == "shell" {
			RunHelper(os.Getenv("AURAGO_TEST_HELPER_COMMAND"))
		} else {
			RunExecHelper()
		}
		return
	}
	caps := Detect()
	if caps.LandlockABI < 3 {
		t.Skip("Landlock ABI 3 or newer is required")
	}
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	if err := os.Mkdir(allowed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	sb := NewLandlockSandbox(ShellSandboxConfig{MaxMemoryMB: 64, MaxCPUSeconds: 10, MaxFileSizeMB: 1}, caps, allowed, nil)
	command := `test "$(ulimit -v)" = 65536 && printf allowed > inside && test "$(cat inside)" = allowed && ! cat ../outside 2>/dev/null && ! touch ../denied 2>/dev/null && echo SANDBOX_OK`
	for _, mode := range []string{"shell", "exec"} {
		t.Run(mode, func(t *testing.T) {
			env, err := sb.buildEnv(allowed, []string{
				"AURAGO_TEST_HELPER_MODE=" + mode,
				"AURAGO_TEST_HELPER_COMMAND=" + command,
				"AURAGO_SBX_EXEC_BIN=/bin/sh",
				`AURAGO_SBX_EXEC_ARGS=["-c",` + strconv.Quote(command) + `]`,
			})
			if err != nil {
				t.Fatal(err)
			}
			// Keep the default shared /tmp grant from including the outside fixture.
			for i, entry := range env {
				if strings.HasPrefix(entry, "AURAGO_SBX_RW=") {
					env[i] = "AURAGO_SBX_RW=" + allowed + ":/dev/null"
				}
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestSandboxHelperEnforcesIsolationAndLowMemoryLimit$")
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "SANDBOX_OK") {
				t.Fatalf("helper failed: %v\n%s", err, out)
			}
		})
	}
}

func containsResource(resources []int, want int) bool {
	for _, resource := range resources {
		if resource == want {
			return true
		}
	}
	return false
}

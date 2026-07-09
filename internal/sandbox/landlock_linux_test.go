//go:build linux

package sandbox

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLandlockHandledAccessForABIIncludesVersionedRights(t *testing.T) {
	base := uint64(
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

	tests := []struct {
		name string
		abi  int
		want uint64
	}{
		{name: "abi1", abi: 1, want: base},
		{name: "abi2", abi: 2, want: base | unix.LANDLOCK_ACCESS_FS_REFER},
		{name: "abi3", abi: 3, want: base | unix.LANDLOCK_ACCESS_FS_REFER | unix.LANDLOCK_ACCESS_FS_TRUNCATE},
		{name: "abi5", abi: 5, want: base | unix.LANDLOCK_ACCESS_FS_REFER | unix.LANDLOCK_ACCESS_FS_TRUNCATE | unix.LANDLOCK_ACCESS_FS_IOCTL_DEV},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := landlockHandledAccessForABI(tt.abi); got != tt.want {
				t.Fatalf("landlockHandledAccessForABI(%d) = %#x, want %#x", tt.abi, got, tt.want)
			}
		})
	}
}

func TestLandlockPrepareCommandBlocksWhenSelfExecutableUnavailable(t *testing.T) {
	old := executablePath
	executablePath = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { executablePath = old })

	sb := NewLandlockSandbox(ShellSandboxConfig{}, Capabilities{LandlockABI: 1}, t.TempDir(), testLogger())
	cmd := sb.PrepareCommand("echo should-not-run", t.TempDir())

	if !strings.Contains(strings.Join(cmd.Args, " "), "AuraGo refused unsandboxed shell execution") {
		t.Fatalf("PrepareCommand() args = %#v, want blocking command", cmd.Args)
	}
}

func TestLandlockPrepareExecCommandBlocksWhenSelfExecutableUnavailable(t *testing.T) {
	old := executablePath
	executablePath = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { executablePath = old })

	sb := NewLandlockSandbox(ShellSandboxConfig{}, Capabilities{LandlockABI: 1}, t.TempDir(), testLogger())
	cmd := sb.PrepareExecCommand("echo", []string{"should-not-run"}, t.TempDir())

	if !strings.Contains(strings.Join(cmd.Args, " "), "AuraGo refused unsandboxed shell execution") {
		t.Fatalf("PrepareExecCommand() args = %#v, want blocking command", cmd.Args)
	}
}

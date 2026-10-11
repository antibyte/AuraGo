//go:build linux

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
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

func TestWritableLandlockRequiresTruncateABI(t *testing.T) {
	for _, abi := range []int{0, 1, 2, 3, 5} {
		sb := NewLandlockSandbox(ShellSandboxConfig{}, Capabilities{LandlockABI: abi}, t.TempDir(), testLogger())
		if got := sb.Available(); got != (abi >= 3) {
			t.Fatalf("Landlock ABI %d available = %v", abi, got)
		}
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

func TestCanonicalSandboxPathRejectsColonAndFollowsSymlink(t *testing.T) {
	if _, err := canonicalSandboxPath("/tmp/has:colon"); err == nil {
		t.Fatal("expected colon rejection")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, err := canonicalSandboxPath(link)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("canonical = %q, want %q", got, want)
	}
}

func TestLandlockOpenUsesNoFollow(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	fd, err := openLandlockPath(link)
	if err == nil {
		unix.Close(fd)
		t.Fatal("symlink path must fail with O_NOFOLLOW")
	}
}

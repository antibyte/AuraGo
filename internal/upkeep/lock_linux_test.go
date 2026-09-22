//go:build linux

package upkeep

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
	"golang.org/x/sys/unix"
)

func TestInheritedUpdateLock(t *testing.T) {
	if root := os.Getenv("UPKEEP_TEST_LOCK_ROOT"); root != "" {
		close, err := lock(root)
		if err != nil {
			t.Fatal(err)
		}
		close()
		return
	}
	f := newFixture(t)
	name := filepath.Join(f.root, StateDir, "update.lock")
	fd, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	if err := unix.Flock(int(fd.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer unix.Flock(int(fd.Fd()), unix.LOCK_UN)
	c := exec.Command(os.Args[0], "-test.run=^TestInheritedUpdateLock$")
	c.Env = append(os.Environ(), "UPKEEP_TEST_LOCK_ROOT="+f.root, "AURAGO_UPDATE_LOCK_FD=3")
	c.ExtraFiles = []*os.File{fd}
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("inherited child lock: %v %s", err, out)
	}
	other := flock.New(name)
	defer other.Close()
	if ok, err := other.TryLock(); err != nil || ok {
		t.Fatalf("child released parent's lock: %v %v", ok, err)
	}
}

func TestInheritedLockRejectsDifferentInstallation(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	fd, err := os.OpenFile(filepath.Join(other.root, StateDir, "update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	c := exec.Command(os.Args[0], "-test.run=^TestInheritedUpdateLock$")
	c.Env = append(os.Environ(), "UPKEEP_TEST_LOCK_ROOT="+f.root, "AURAGO_UPDATE_LOCK_FD=3")
	c.ExtraFiles = []*os.File{fd}
	if err := c.Run(); err == nil {
		t.Fatal("foreign inherited descriptor accepted")
	}
}

func TestPermissionFailureIsPartialAndRecoverable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks require non-root")
	}
	f := newFixture(t)
	for i := 1; i <= 4; i++ {
		f.update(i, false)
	}
	blocked := filepath.Join(f.root, StateDir, "transactions", "txn-000001", "bin")
	if e := os.Chmod(blocked, 0500); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0700) })
	if _, e := f.clean(); e == nil {
		t.Fatal("permission failure reported as success")
	}
	blocked = filepath.Join(f.root, StateDir, "transactions", retiredName(f.root, "txn-000001"), "bin")
	state, e := ReadResult(f.root)
	if e != nil || state.Success || state.Failures != 1 {
		t.Fatal(state, e)
	}
	if e := os.Chmod(blocked, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := f.clean(); e != nil {
		t.Fatal("cleanup did not recover", e)
	}
	state, _ = ReadResult(f.root)
	if !state.Success {
		t.Fatal("recovery not recorded")
	}
}

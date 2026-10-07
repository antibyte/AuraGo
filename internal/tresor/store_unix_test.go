//go:build unix

package tresor

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenCreatesPrivateDatabaseFiles(t *testing.T) {
	// A common umask would leave a freshly created SQLite file world-readable.
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	path := filepath.Join(t.TempDir(), "tresor.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := Header{Salt: make([]byte, 32), PasswordEnvelope: make([]byte, 60), RecoveryEnvelope: make([]byte, 60)}
	if err := store.Setup(context.Background(), h); err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(name)
		if os.IsNotExist(err) && name != path {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		checked++
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("%s mode = %04o, want 0600", filepath.Base(name), mode)
		}
	}
	if checked < 2 {
		t.Fatalf("checked %d files, want the database plus at least one WAL sidecar", checked)
	}
}

func TestOpenKeepsDatabasePrivateWhenInitializationFails(t *testing.T) {
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	for name, existing := range map[string]bool{"new database": false, "existing wide database": true} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tresor.db")
			if existing {
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// A directory in place of the WAL file makes the PRAGMA block fail,
			// so Open returns before its final chmod.
			if err := os.Mkdir(path+"-wal", 0o700); err != nil {
				t.Fatal(err)
			}
			if store, err := Open(path); err == nil {
				store.Close()
				t.Fatal("Open succeeded with a directory as WAL file, want an initialization error")
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if mode := info.Mode().Perm(); mode != 0o600 {
				t.Fatalf("tresor.db mode after failed Open = %04o, want 0600", mode)
			}
		})
	}
}

func TestOpenTightensExistingWideSidecars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tresor.db")
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(name, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(name)
		if os.IsNotExist(err) && name != path {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("%s mode = %04o, want 0600", filepath.Base(name), mode)
		}
	}
}

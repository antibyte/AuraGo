//go:build windows

package desktop

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func setSecureOpenAfterLstat(t *testing.T, hook func(path string)) {
	t.Helper()
	old := secureOpenAfterLstat
	secureOpenAfterLstat = hook
	t.Cleanup(func() { secureOpenAfterLstat = old })
}

func TestOpenFileNoFollowReadsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openFileNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("openFileNoFollow: %v", err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(data) != "hello" {
		t.Fatalf("read = %q, %v; want hello", data, err)
	}
}

func TestOpenFileNoFollowTruncatesExistingFileInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("old content"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openFileNoFollow(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("openFileNoFollow: %v", err)
	}
	if _, err := f.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "new" {
		t.Fatalf("content = %q, %v; want new", data, err)
	}
}

func TestOpenFileNoFollowCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.txt")
	f, err := openFileNoFollow(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("openFileNoFollow: %v", err)
	}
	if _, err := f.WriteString("created"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "created" {
		t.Fatalf("content = %q, %v; want created", data, err)
	}
}

func TestOpenFileNoFollowRejectsAppendWithTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openFileNoFollow(path, os.O_WRONLY|os.O_APPEND|os.O_TRUNC, 0o600)
	if err == nil {
		f.Close()
		t.Fatal("openFileNoFollow accepted O_APPEND|O_TRUNC")
	}
	if !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("err = %v, want os.ErrInvalid", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep me" {
		t.Fatalf("content = %q, %v; want the file left untouched", data, err)
	}
}

func TestOpenFileNoFollowRefusesEntrySwappedAfterLstat(t *testing.T) {
	cases := map[string]func(t *testing.T, path, other string){
		"renamed over": func(t *testing.T, path, other string) {
			if err := os.Rename(other, path); err != nil {
				t.Errorf("rename over path: %v", err)
			}
		},
		"deleted and recreated": func(t *testing.T, path, _ string) {
			if err := os.Remove(path); err != nil {
				t.Errorf("remove path: %v", err)
			}
			if err := os.WriteFile(path, []byte("swapped"), 0o600); err != nil {
				t.Errorf("recreate path: %v", err)
			}
		},
	}
	for name, swap := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "note.txt")
			other := filepath.Join(dir, "other.txt")
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(other, []byte("swapped"), 0o600); err != nil {
				t.Fatal(err)
			}
			setSecureOpenAfterLstat(t, func(p string) {
				if p == path {
					swap(t, path, other)
				}
			})

			for _, flag := range []int{os.O_RDONLY, os.O_WRONLY | os.O_CREATE | os.O_TRUNC} {
				f, err := openFileNoFollow(path, flag, 0o600)
				if err == nil {
					f.Close()
					t.Fatalf("flag %#x: openFileNoFollow succeeded on a swapped entry", flag)
				}
				if !errors.Is(err, os.ErrPermission) {
					t.Fatalf("flag %#x: err = %v, want the symlink refusal (ErrPermission)", flag, err)
				}
				// The refused open must not have truncated the swapped-in file.
				if data, err := os.ReadFile(path); err != nil || string(data) != "swapped" {
					t.Fatalf("flag %#x: swapped-in file = %q, %v; want its content kept", flag, data, err)
				}
				// Re-arm the next round with a fresh file to swap in.
				if err := os.WriteFile(other, []byte("swapped"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// A leaked handle (opened without FILE_SHARE_DELETE) would block removal.
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove after refused open: %v (leaked handle?)", err)
			}
		})
	}
}

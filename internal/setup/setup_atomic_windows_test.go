//go:build windows

package setup

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestEnsureMasterKeyAtomicFailurePreservesEnv(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	original := []byte("AURAGO_MASTER_KEY=broken\nOTHER_SETTING=keep\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	// Allow in-place writes but deny replacement, reproducing a Windows reader race.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	t.Setenv("AURAGO_MASTER_KEY", strings.Repeat("a", 64))
	if err := ensureMasterKey(root, slog.Default()); err == nil {
		t.Fatal("replacement unexpectedly succeeded")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("failed replacement changed the original .env")
	}
	temps, err := filepath.Glob(filepath.Join(root, ".aurago-publish-*"))
	if err != nil || len(temps) > 0 {
		t.Fatal("failed publication left private temporary files")
	}
}

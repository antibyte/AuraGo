//go:build windows

package security

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestVaultAtomicReplaceHandlesWindowsReaders(t *testing.T) {
	for _, mode := range []string{"background", "context", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.bin")
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			name, err := windows.UTF16PtrFromString(path)
			if err != nil {
				t.Fatal(err)
			}
			// A scanner/reader can allow reads and writes while temporarily denying replacement.
			handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			if err != nil {
				t.Fatal(err)
			}
			unlock := func() {
				if handle != windows.InvalidHandle {
					_ = windows.CloseHandle(handle)
					handle = windows.InvalidHandle
				}
			}
			defer unlock()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if mode == "background" {
					done <- writeVaultFileAtomic(path, []byte("replacement"), 0o600)
				} else {
					done <- writeVaultFileAtomicContext(ctx, path, []byte("replacement"), 0o600)
				}
			}()
			select {
			case err := <-done:
				t.Fatalf("atomic replacement did not wait for reader: %v", err)
			case <-time.After(60 * time.Millisecond):
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != "original" {
				t.Fatalf("original changed before publish: %q %v", got, err)
			}
			if mode == "cancel" {
				cancel()
			} else {
				unlock()
			}
			select {
			case err := <-done:
				if mode == "cancel" {
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancellation lost: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("atomic replacement did not finish")
			}
			want := "replacement"
			if mode == "cancel" {
				want = "original"
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != want {
				t.Fatalf("published bytes: %q %v", got, err)
			}
			if files, err := filepath.Glob(path + ".tmp-*"); err != nil || len(files) != 0 {
				t.Fatalf("temporary files retained: %v %v", files, err)
			}
		})
	}
}

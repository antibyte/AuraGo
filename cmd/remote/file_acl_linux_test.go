package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/remote"
	"golang.org/x/sys/unix"
)

func testPOSIXACL(namedPerm uint16) []byte {
	entries := [][3]uint32{{1, 6, ^uint32(0)}, {2, uint32(namedPerm), 65534}, {4, 4, ^uint32(0)}, {16, 4, ^uint32(0)}, {32, 0, ^uint32(0)}}
	data := make([]byte, 4+8*len(entries))
	binary.LittleEndian.PutUint32(data, 2)
	for i, entry := range entries {
		o := 4 + i*8
		binary.LittleEndian.PutUint16(data[o:], uint16(entry[0]))
		binary.LittleEndian.PutUint16(data[o+2:], uint16(entry[1]))
		binary.LittleEndian.PutUint32(data[o+4:], entry[2])
	}
	return data
}

func TestAtomicFileWritePreservesPOSIXACL(t *testing.T) {
	for _, name := range []string{"deny", "grant", "no ACL"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			inherited := uint16(4)
			if name == "grant" {
				inherited = 0
			}
			if err := unix.Setxattr(dir, "system.posix_acl_default", testPOSIXACL(inherited), 0); err != nil {
				if errors.Is(err, unix.ENOTSUP) {
					t.Skip("filesystem does not support POSIX ACLs")
				}
				t.Fatal(err)
			}
			path := filepath.Join(dir, "private.txt")
			if err := os.WriteFile(path, []byte("original"), 0640); err != nil {
				t.Fatal(err)
			}
			var want []byte
			if name == "no ACL" {
				if err := unix.Removexattr(path, "system.posix_acl_access"); err != nil {
					t.Fatal(err)
				}
			} else {
				want = testPOSIXACL(4 - inherited)
				if err := unix.Setxattr(path, "system.posix_acl_access", want, 0); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			executor := NewExecutor(slog.Default(), 1)
			result := executor.Execute(remote.CommandPayload{Operation: remote.OpFileWrite, Args: map[string]interface{}{"path": path, "content": base64.StdEncoding.EncodeToString([]byte("replacement"))}}, false, []string{dir})
			if result.Status != "ok" {
				t.Fatalf("write failed: %+v", result)
			}
			got := make([]byte, 256)
			n, err := unix.Getxattr(path, "system.posix_acl_access", got)
			if want == nil {
				if !errors.Is(err, unix.ENODATA) {
					t.Fatalf("replacement retained an inherited ACL: %v", err)
				}
			} else if err != nil || !bytes.Equal(got[:n], want) {
				t.Fatalf("replacement changed the original ACL: %v", err)
			}
			after, err := os.Stat(path)
			if err != nil || after.Mode() != before.Mode() {
				t.Fatal("replacement changed the original mode")
			}
			content, err := os.ReadFile(path)
			if err != nil || string(content) != "replacement" {
				t.Fatal("replacement data was not published")
			}
		})
	}
}

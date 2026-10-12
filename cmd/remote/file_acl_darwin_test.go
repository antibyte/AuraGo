package main

import (
	"encoding/base64"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/remote"
)

func TestAtomicFileWritePreservesDarwinACL(t *testing.T) {
	for _, acl := range []string{"user:nobody deny read", "user:nobody allow read", ""} {
		t.Run(acl, func(t *testing.T) {
			dir := t.TempDir()
			if output, err := exec.Command("/bin/chmod", "+a", "user:nobody allow read,file_inherit", dir).CombinedOutput(); err != nil {
				t.Fatalf("set inherited ACL: %v %s", err, output)
			}
			path := filepath.Join(dir, "private.txt")
			if err := os.WriteFile(path, []byte("original"), 0640); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("/bin/chmod", "-N", path).CombinedOutput(); err != nil {
				t.Fatalf("clear inherited ACL: %v %s", err, output)
			}
			if acl != "" {
				if output, err := exec.Command("/bin/chmod", "+a", acl, path).CombinedOutput(); err != nil {
					t.Fatalf("set file ACL: %v %s", err, output)
				}
			}
			readACL := func() string {
				output, err := exec.Command("/bin/ls", "-le", path).CombinedOutput()
				if err != nil {
					t.Fatalf("read ACL: %v %s", err, output)
				}
				_, entries, _ := strings.Cut(string(output), "\n")
				return entries
			}
			before := readACL()
			executor := NewExecutor(slog.Default(), 1)
			result := executor.Execute(remote.CommandPayload{Operation: remote.OpFileWrite, Args: map[string]interface{}{"path": path, "content": base64.StdEncoding.EncodeToString([]byte("replacement"))}}, false, []string{dir})
			if result.Status != "ok" {
				t.Fatalf("write failed: %+v", result)
			}
			if after := readACL(); before != after {
				t.Fatalf("replacement changed ACL: before=%q after=%q", before, after)
			}
			content, err := os.ReadFile(path)
			if err != nil || string(content) != "replacement" {
				t.Fatal("replacement data was not published")
			}
		})
	}
}

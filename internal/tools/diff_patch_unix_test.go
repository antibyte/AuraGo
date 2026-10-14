//go:build !windows

package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileApplyPatchFallbackStaysInStaging(t *testing.T) {
	patchBinary, err := exec.LookPath("patch")
	if err != nil {
		t.Fatal("patch is required for the Unix fallback regression")
	}
	for _, tc := range []struct{ name, prefix, old, status string }{
		{"strip-prefix", "a/", "old", "success"},
		{"without-prefix", "", "old", "success"},
		{"invalid-hunk", "a/", "missing", "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace, bin := t.TempDir(), t.TempDir()
			selected := filepath.Join(workspace, "file.txt")
			if err := os.WriteFile(selected, []byte("old\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(workspace, "patch.log")
			for name, script := range map[string]string{
				"git":   "#!/bin/sh\nexit 1\n",
				"patch": "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" >> \"$PATCH_TEST_LOG\"\nexec \"$PATCH_TEST_BINARY\" \"$@\"\n",
			} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			t.Setenv("PATCH_TEST_LOG", logPath)
			t.Setenv("PATCH_TEST_BINARY", patchBinary)
			patch := "--- " + tc.prefix + "file.txt\n+++ " + tc.prefix + "file.txt\n@@ -1 +1 @@\n-" + tc.old + "\n+new\n"
			var result FileEditorResult
			encode := func(value FileEditorResult) string { data, _ := json.Marshal(value); return string(data) }
			if err := json.Unmarshal([]byte(fileApplyPatch(selected, patch, workspace, encode)), &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status {
				t.Fatalf("fallback result: %+v, want %s", result, tc.status)
			}
			want := "old\n"
			if tc.status == "success" {
				want = "new\n"
			}
			if data, err := os.ReadFile(selected); err != nil || string(data) != want {
				t.Fatalf("selected file: %q, %v", data, err)
			}
			log, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(log)), "\n")
			calls := 0
			for i, line := range lines {
				if strings.HasPrefix(filepath.Base(line), "aurago-patch-") && filepath.IsAbs(line) && !strings.HasSuffix(line, ".diff") {
					calls++
					if line == workspace || i+2 >= len(lines) || lines[i+1] != "--batch" || lines[i+2] != "--forward" {
						t.Fatalf("unsafe fallback invocation: %s", log)
					}
				}
			}
			wantCalls := 2
			if tc.name == "strip-prefix" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("fallback calls = %d, want %d: %s", calls, wantCalls, log)
			}
		})
	}
}

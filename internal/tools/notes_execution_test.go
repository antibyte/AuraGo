package tools

import (
	"aurago/internal/sandbox"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellWithNotesAndDisabledIsolation(t *testing.T) {
	workspace := t.TempDir()
	notes := filepath.Join(workspace, "Notes")
	if err := os.Mkdir(notes, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sandbox.SetForTest(&sandbox.FallbackSandbox{}))
	previous, configured := currentRuntimePermissions()
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})
	perms := RuntimePermissions{AllowShell: true, AllowFilesystemWrite: true, ProtectedNotesRoots: []string{notes}}
	ConfigureRuntimePermissions(perms)
	out, stderr, err := ExecuteShell("echo notes-policy-ok", workspace)
	if err != nil || !strings.Contains(out, "notes-policy-ok") {
		t.Fatalf("permitted shell failed: stdout=%q stderr=%q err=%v", out, stderr, err)
	}
	if err := writeFileAtomic(filepath.Join(notes, "note.md"), []byte("blocked")); err == nil {
		t.Fatal("native Notes write bypassed protection")
	}
	perms.AllowShell = false
	ConfigureRuntimePermissions(perms)
	if _, _, err := ExecuteShell("echo must-not-run", workspace); err == nil {
		t.Fatal("disabled shell permission was bypassed")
	}
}

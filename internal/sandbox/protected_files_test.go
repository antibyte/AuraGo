package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestProtectedNotesRefuseUnsafeProcesses(t *testing.T) {
	root := filepath.Join(t.TempDir(), "notes")
	cmd := exec.Command("never-run-this-command")
	if got, err := ProtectFilesCommand(cmd, []string{root}); err != nil || got != cmd {
		t.Fatal("absent notes changed execution", err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	restore := SetForTest(&FallbackSandbox{})
	defer restore()
	if _, err := ProtectFilesCommand(cmd, []string{root}); err == nil {
		t.Fatal("unrestricted process could modify protected notes")
	}
	for _, test := range []struct {
		a, b string
		want bool
	}{{root, root, true}, {filepath.Dir(root), root, true}, {root, filepath.Join(root, "file"), true}, {root, root + "-other", false}} {
		if pathsOverlap(test.a, test.b) != test.want {
			t.Fatal(test)
		}
	}
}

package sandbox

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestProtectedNotesRespectExecutionPolicy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "notes")
	cmd := exec.Command("never-run-this-command")
	if got, err := ProtectFilesCommand(cmd, []string{root}); err != nil || got != cmd {
		t.Fatal("absent notes changed execution", err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		cfg         ShellSandboxConfig
		wantBlocked bool
	}{
		{"disabled", ShellSandboxConfig{}, false},
		{"explicit unsafe fallback", ShellSandboxConfig{Enabled: true, AllowUnsafeFallback: true}, false},
		{"enabled but unavailable", ShellSandboxConfig{Enabled: true}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Docker makes the unavailable-backend case deterministic on every OS.
			sb := selectSandboxForCaps(test.cfg, Capabilities{InDocker: true}, t.TempDir(), slog.Default())
			t.Cleanup(SetForTest(sb))
			got, err := ProtectFilesCommand(cmd, []string{root})
			if (err != nil) != test.wantBlocked || !test.wantBlocked && got != cmd {
				t.Fatalf("command = %v, error = %v, want blocked = %v", got, err, test.wantBlocked)
			}
			if !ProtectedFilePath(filepath.Join(root, "note.md"), []string{root}, false) {
				t.Fatal("execution policy disabled native Notes protection")
			}
		})
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

//go:build linux

package sandbox

import (
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtectedNotesKeepActiveLandlockPolicy(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(SetForTest(NewLandlockSandbox(ShellSandboxConfig{Enabled: true}, Capabilities{LandlockABI: 4}, "/work", slog.Default())))
	for _, test := range []struct {
		name, writable string
		blocked        bool
	}{
		{"separate workspace", "/work", false},
		{"notes", root, true},
		{"ancestor", filepath.Dir(root), true},
		{"descendant", filepath.Join(root, "attachments"), true},
		{"missing policy", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command("aurago", "--sandbox-exec", "true")
			cmd.Env = []string{"AURAGO_SBX_RW=" + test.writable}
			got, err := ProtectFilesCommand(cmd, []string{root})
			if (err != nil) != test.blocked || !test.blocked && got != cmd {
				t.Fatalf("command = %v, error = %v, want blocked = %v", got, err, test.blocked)
			}
		})
	}
	// A raw command must still be wrapped in the actual Landlock helper.
	cmd := exec.Command("/bin/true")
	got, err := ProtectFilesCommand(cmd, []string{"/etc"})
	if err != nil || got == cmd || len(got.Args) < 2 || got.Args[1] != "--sandbox-exec-bin" {
		t.Fatalf("missing Landlock wrapper: command = %v, error = %v", got, err)
	}
	if !strings.Contains(strings.Join(got.Env, "\n"), "AURAGO_SBX_RW=") {
		t.Fatal("Landlock wrapper missing write policy")
	}
}

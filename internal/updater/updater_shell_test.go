//go:build !windows

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQuoteShellPathRoundTrips(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("Bash is required for the Linux update launcher")
	}
	for _, value := range []string{
		"", "/ordinary/path", "path with spaces", "quote'and\"double",
		"$HOME $(printf expanded) \x60printf expanded\x60", "back\\slash\nnewline;&",
	} {
		output, err := exec.Command(bash, "-c", "printf '%s' "+quoteShellPath(value)).Output()
		if err != nil {
			t.Fatalf("round trip %q: %v", value, err)
		}
		if string(output) != value {
			t.Fatalf("round trip = %q, want %q", output, value)
		}
	}
}

func TestDefaultStartScriptPreservesLiteralPathsAndLaunchContract(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("Bash is required for the Linux update launcher")
	}
	dir := filepath.Join(t.TempDir(), "literal $HOME $(printf expanded) \x60printf expanded\x60 ' spaced")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	bashPath := filepath.Join(dir, "bash $HOME ' \x60printf expanded\x60")
	if err := os.Symlink(bash, bashPath); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "fixture $(printf expanded) ' .sh")
	script := "printf '%s\\n' \"$PWD\" \"$#\" \"$1\"\nprintf 'fixture-stderr\\n' >&2\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "log $HOME \x60printf expanded\x60 ' .txt")
	if err := os.WriteFile(logPath, []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := DefaultStartScript(ScriptLaunch{
		Dir: dir, BashPath: bashPath, ScriptPath: scriptPath, LogPath: logPath, Env: os.Environ(),
	}); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{"existing", dir, "1", "--yes", "fixture-stderr", ""}, "\n")
	var output []byte
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		output, err = os.ReadFile(logPath)
		if err == nil && string(output) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture log = %q, want %q (read error: %v)", output, want, err)
}

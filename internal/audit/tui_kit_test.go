package audit

import (
	"bytes"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

var (
	tuiKitBegin = regexp.MustCompile(`(?m)^# >>> AURAGO-TUI-KIT[^\n]*\n`)
	tuiKitEnd   = regexp.MustCompile(`(?m)^# <<< AURAGO-TUI-KIT[^\n]*\n`)
)

// extractTUIKit returns the text from the opening kit marker through the
// closing marker line (inclusive), failing unless exactly one pair exists.
func extractTUIKit(t *testing.T, name, script string) string {
	t.Helper()
	begins := tuiKitBegin.FindAllStringIndex(script, -1)
	ends := tuiKitEnd.FindAllStringIndex(script, -1)
	if len(begins) != 1 || len(ends) != 1 {
		t.Fatalf("%s must contain exactly one AURAGO-TUI-KIT marker pair, found %d begin / %d end", name, len(begins), len(ends))
	}
	if ends[0][0] < begins[0][0] {
		t.Fatalf("%s has its AURAGO-TUI-KIT end marker before the begin marker", name)
	}
	return script[begins[0][0]:ends[0][1]]
}

// install.sh and update.sh stay single-file (curl | bash, release assets), so
// the kit is embedded. The copies must never drift from scripts/aurago-tui.sh;
// fix with: bash scripts/sync-tui-kit.sh
func TestTUIKitIsEmbeddedVerbatim(t *testing.T) {
	t.Parallel()

	source := extractTUIKit(t, "scripts/aurago-tui.sh", readRepoFile(t, "scripts/aurago-tui.sh"))
	for _, path := range []string{"install.sh", "update.sh"} {
		if got := extractTUIKit(t, path, readRepoFile(t, path)); got != source {
			t.Fatalf("%s embeds a stale TUI kit; run `bash scripts/sync-tui-kit.sh`", path)
		}
	}
}

// Output must stay machine-friendly: no terminal, no escape codes. The in-app
// updater appends update.sh output to log/update.log.
func TestTUIKitPlainModeEmitsNoEscapeCodes(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("bash behaviour differs on Windows hosts")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}

	command := exec.Command("bash", repoPath("scripts", "aurago-tui.sh"), "--plain")
	command.Env = []string{"PATH=/usr/bin:/bin:/usr/local/bin", "TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8"}
	done := make(chan struct{})
	var output []byte
	var runErr error
	go func() {
		output, runErr = command.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("TUI kit demo did not finish in plain mode")
	}
	if runErr != nil {
		t.Fatalf("TUI kit demo failed: %v\n%s", runErr, output)
	}
	if bytes.ContainsRune(output, 0x1b) {
		t.Fatalf("plain mode must not emit escape codes:\n%q", output)
	}
	for _, want := range []string{"[INFO]", "[ OK ]", "[WARN]", "[FAIL]", "==> [1/4] System scan"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("plain mode output missing %q:\n%s", want, output)
		}
	}
}

// The scripts must render through the kit instead of hand-rolled escapes.
func TestSetupScriptsUseTUIKitHelpers(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"install.sh", "update.sh"} {
		script := readRepoFile(t, path)
		for _, required := range []string{
			`info() `, `ok() `, `warn() `, `die() `, `section() `,
			`tui_banner "`, `tui_cleanup`,
		} {
			if !strings.Contains(script, required) {
				t.Fatalf("%s must route output through the TUI kit; missing %q", path, required)
			}
		}
		for _, legacy := range []string{`RED='\033`, `G1='\033`, `ICO_OK=`, `echo -e " ${G`} {
			if strings.Contains(script, legacy) {
				t.Fatalf("%s still contains legacy hand-rolled styling %q", path, legacy)
			}
		}
	}
}

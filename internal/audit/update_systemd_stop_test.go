package audit

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUpdaterSystemdStopDeadlineAndFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("behavioral systemd updater test requires Unix")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	update := readRepoFile(t, "update.sh")
	start := strings.Index(update, "SYSTEMD_STOP_FORCED=false\n")
	end := strings.Index(update, "\ninfo \"Removing stale lock files...\"")
	if start < 0 || end < start {
		t.Fatal("could not extract the systemd stop stage")
	}
	for _, test := range []struct {
		name        string
		foreground  bool
		sudo        bool
		authCode    int
		timeoutCode int
		stopCode    int
		result      string
		want        string
	}{
		{name: "GNU timeout", foreground: true},
		{name: "GNU timeout with sudo", foreground: true, sudo: true},
		{name: "BusyBox timeout"},
		{name: "BusyBox authenticates first", sudo: true},
		{name: "authentication denied", sudo: true, authCode: 1, want: "exit code 1"},
		{name: "timeout", foreground: true, timeoutCode: 124, want: "did not complete within 75 seconds"},
		{name: "command failed", foreground: true, stopCode: 5, want: "exit code 5"},
		{name: "forced stop still aborts", foreground: true, result: "timeout", want: "systemd reported a forced timeout stop"},
	} {
		t.Run(test.name, func(t *testing.T) {
			harness := `set -eu
stopped=false
SUDO=""
if $AURAGO_TEST_SUDO; then SUDO=sudo; fi
info() { :; }
ok() { :; }
has_interactive_tty() { return 0; }
_kill_proc() { return 0; }
abort_before_file_changes() { printf '%s\n' "$1"; exit 41; }
sudo() {
    if [ "${1:-}" = -v ]; then return "$AURAGO_TEST_AUTH_CODE"; fi
    if [ "${1:-}" = -n ]; then shift; fi
    "$@"
}
systemctl() {
    case "$1" in
        is-active) ! $stopped ;;
        stop) stopped=true; return "$AURAGO_TEST_STOP_CODE" ;;
        show)
            if [ "$*" = 'show aurago.service -p Result --value' ]; then
                printf '%s\n' "$AURAGO_TEST_RESULT"
            else
                printf 'ActiveState=deactivating\nSubState=stop-sigterm\nResult=success\nTimeoutStopUSec=1min\n'
            fi ;;
        *) exit 90 ;;
    esac
}
timeout() {
    if [ "$1" = --help ]; then
        if $AURAGO_TEST_FOREGROUND; then printf '%s\n' --foreground; fi
        return 0
    fi
    if $AURAGO_TEST_FOREGROUND; then
        [ "$1" = --foreground ] || exit 91
        shift
    fi
    [ "$1" = 75s ] || exit 92
    shift
    if [ "$AURAGO_TEST_TIMEOUT_CODE" -ne 0 ]; then return "$AURAGO_TEST_TIMEOUT_CODE"; fi
    "$@"
}
` + update[start:end] + "\nprintf 'completed\\n'\n"
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "bash", "-c", harness)
			command.Env = append(os.Environ(),
				"AURAGO_TEST_FOREGROUND="+strconv.FormatBool(test.foreground),
				"AURAGO_TEST_SUDO="+strconv.FormatBool(test.sudo),
				"AURAGO_TEST_AUTH_CODE="+strconv.Itoa(test.authCode),
				"AURAGO_TEST_TIMEOUT_CODE="+strconv.Itoa(test.timeoutCode),
				"AURAGO_TEST_STOP_CODE="+strconv.Itoa(test.stopCode),
				"AURAGO_TEST_RESULT="+test.result,
			)
			output, err := command.CombinedOutput()
			if test.want == "" {
				if err != nil || !strings.Contains(string(output), "completed") {
					t.Fatalf("stop failed: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(string(output), test.want) || strings.Contains(string(output), "completed") {
				t.Fatalf("stop error = %v, output = %s, want abort with %q", err, output, test.want)
			}
			if test.authCode != 0 || test.timeoutCode != 0 || test.stopCode != 0 {
				if !strings.Contains(string(output), "ActiveState=") || !strings.Contains(string(output), "journalctl") {
					t.Fatalf("stop failure did not provide diagnostics: %s", output)
				}
			}
		})
	}
}

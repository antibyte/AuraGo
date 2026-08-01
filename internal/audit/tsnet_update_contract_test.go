package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTsNetUpdaterLifecycleContract(t *testing.T) {
	update := readRepoFile(t, "update.sh")
	for _, required := range []string{
		"--rebuild",
		`[ "$INSTALLED_RELEASE" = "$RELEASE_TAG" ] && ! $REBUILD`,
		`[ "$LOCAL_HASH" = "$REMOTE_HASH" ]`,
		"--healthcheck-timeout 60s",
		"--healthcheck-timeout 180s --healthcheck-require-tsnet",
		"--print-tsnet-state-dir",
		"TSNET_WAS_READY",
		"cp -a --",
		"restore_tsnet_state_backup",
		"AuraGo required SIGKILL",
		"Update incomplete: tsnet readiness failed",
		"node-specific reauthentication",
		`PRE_START_MODE="stopped"`,
		"abort_before_file_changes",
		"systemd-analyze verify aurago.service",
		"20-aurago-stop-timeout.conf",
		"restore_service_stop_timeout_dropin",
	} {
		if !strings.Contains(update, required) {
			t.Fatalf("update.sh is missing lifecycle contract marker %q", required)
		}
	}
	if strings.Contains(update, "TSNET_FORCE_LOGIN") {
		t.Fatal("update.sh must not use process-wide TSNET_FORCE_LOGIN")
	}
	if strings.Contains(update, `sed -i 's/^TimeoutStopSec=`) ||
		strings.Contains(update, `sed -i '/^RestartSec=/a TimeoutStopSec=`) {
		t.Fatal("update.sh must use the verified systemd drop-in instead of editing TimeoutStopSec in the main unit")
	}
}

func TestGeneratedSystemdUnitsHaveBoundedStopTimeout(t *testing.T) {
	for _, path := range []string{
		"install.sh",
		"install_service_linux.sh",
		"internal/setup/setup.go",
	} {
		source := readRepoFile(t, path)
		if !strings.Contains(source, "TimeoutStopSec=60s") {
			t.Fatalf("%s does not generate TimeoutStopSec=60s", path)
		}
	}
}

func TestTsNetImplementationDoesNotUseGlobalForcedLogin(t *testing.T) {
	for _, path := range []string{
		"cmd/aurago/main.go",
		"cmd/aurago/healthcheck.go",
		"internal/tsnetnode/tsnetnode.go",
		"internal/server/tsnet_handlers.go",
	} {
		if strings.Contains(readRepoFile(t, path), "TSNET_FORCE_LOGIN") {
			t.Fatalf("%s uses forbidden process-wide forced login", path)
		}
	}
}

func TestUpdaterNoOpDoesNotInvokeSystemctlOrSudo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("behavioral updater test requires a POSIX shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}

	root := t.TempDir()
	script := filepath.Join(root, "update.sh")
	if err := os.WriteFile(script, []byte(readRepoFile(t, "update.sh")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module updater-test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "privileged-command-used")
	writeExecutable := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fakeBin, name), []byte("#!/usr/bin/env bash\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable("git", `
case "${1:-}" in
  fetch) exit 0 ;;
  rev-parse) echo "same-commit"; exit 0 ;;
  log) echo "same-commit already current"; exit 0 ;;
  remote) echo "https://example.invalid/AuraGo.git"; exit 0 ;;
esac
exit 0`)
	writeExecutable("sudo", `printf 'sudo\n' >> "$AURAGO_TEST_MARKER"; exit 99`)
	writeExecutable("systemctl", `printf 'systemctl\n' >> "$AURAGO_TEST_MARKER"; exit 99`)

	command := exec.Command("bash", script, "--yes")
	command.Dir = root
	command.Env = append(os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"AURAGO_TEST_MARKER="+marker,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("no-op updater failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "no files or services were changed") {
		t.Fatalf("no-op updater did not report an unchanged installation:\n%s", output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("no-op updater invoked a privileged/service command; stat error = %v", err)
	}
}

func TestUpdaterFailedStopRestartHonorsPreviousStartModeAndFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("behavioral updater test requires a POSIX shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}

	update := readRepoFile(t, "update.sh")
	start := strings.Index(update, "restart_unchanged_after_failed_stop() {")
	if start < 0 {
		t.Fatal("could not find restart_unchanged_after_failed_stop in update.sh")
	}
	end := strings.Index(update[start:], "\n# Stop systemd first.")
	if end < 0 {
		t.Fatal("could not extract restart_unchanged_after_failed_stop from update.sh")
	}
	functions := update[start : start+end]

	for _, test := range []struct {
		name          string
		mode          string
		systemdFails  bool
		wantSuccess   bool
		wantSystemctl bool
	}{
		{name: "stopped stays stopped", mode: "stopped", wantSuccess: true},
		{name: "systemd restarts through systemd", mode: "systemd", wantSuccess: true, wantSystemctl: true},
		{name: "systemd start failure is returned", mode: "systemd", systemdFails: true, wantSuccess: false, wantSystemctl: true},
		{name: "direct restarts directly", mode: "direct", wantSuccess: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			fakeBin := filepath.Join(root, "fake-bin")
			if err := os.MkdirAll(fakeBin, 0o700); err != nil {
				t.Fatal(err)
			}
			serviceState := filepath.Join(root, "systemd-active")
			systemctlLog := filepath.Join(root, "systemctl.log")
			processPID := filepath.Join(root, "direct.pid")
			auragoBinary := filepath.Join(root, "aurago")
			systemctlBody := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$AURAGO_TEST_SYSTEMCTL_LOG"
case "${1:-}" in
  start)
    [ "${AURAGO_TEST_SYSTEMD_FAIL:-0}" != "1" ] || exit 1
    : > "$AURAGO_TEST_SERVICE_STATE"
    exit 0
    ;;
  is-active)
    [ -f "$AURAGO_TEST_SERVICE_STATE" ]
    exit
    ;;
esac
exit 0
`
			if err := os.WriteFile(filepath.Join(fakeBin, "systemctl"), []byte(systemctlBody), 0o700); err != nil {
				t.Fatal(err)
			}
			auragoBody := `#!/usr/bin/env bash
for arg in "$@"; do
  [ "$arg" != "--healthcheck" ] || exit "${AURAGO_TEST_HEALTH_FAIL:-0}"
done
printf '%s\n' "$$" > "$AURAGO_TEST_DIRECT_PID"
trap 'exit 0' TERM INT
while :; do sleep 1; done
`
			if err := os.WriteFile(auragoBinary, []byte(auragoBody), 0o700); err != nil {
				t.Fatal(err)
			}
			harness := `#!/usr/bin/env bash
set -u
SUDO=""
NO_RESTART=false
CORE_WAS_READY="ready"
DIR="$AURAGO_TEST_ROOT"
CURRENT_AURAGO_BIN="$AURAGO_TEST_BINARY"
PRE_START_MODE="$AURAGO_TEST_MODE"
info() { :; }
die() { return 1; }
read_master_key_from_env() { printf ''; }
binary_supports_option() { return 0; }
` + functions + `
restart_unchanged_after_failed_stop
`
			harnessPath := filepath.Join(root, "harness.sh")
			if err := os.WriteFile(harnessPath, []byte(harness), 0o700); err != nil {
				t.Fatal(err)
			}
			failValue := "0"
			if test.systemdFails {
				failValue = "1"
			}
			command := exec.Command("bash", harnessPath)
			command.Env = append(os.Environ(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"AURAGO_TEST_ROOT="+root,
				"AURAGO_TEST_BINARY="+auragoBinary,
				"AURAGO_TEST_MODE="+test.mode,
				"AURAGO_TEST_SYSTEMD_FAIL="+failValue,
				"AURAGO_TEST_SYSTEMCTL_LOG="+systemctlLog,
				"AURAGO_TEST_SERVICE_STATE="+serviceState,
				"AURAGO_TEST_DIRECT_PID="+processPID,
			)
			output, err := command.CombinedOutput()
			if test.wantSuccess && err != nil {
				t.Fatalf("restart helper failed: %v\n%s", err, output)
			}
			if !test.wantSuccess && err == nil {
				t.Fatalf("restart helper unexpectedly succeeded:\n%s", output)
			}
			_, logErr := os.Stat(systemctlLog)
			if test.wantSystemctl && logErr != nil {
				t.Fatalf("systemctl was not invoked: %v", logErr)
			}
			if !test.wantSystemctl && !os.IsNotExist(logErr) {
				t.Fatalf("systemctl was invoked for %s mode", test.mode)
			}
			if rawPID, readErr := os.ReadFile(processPID); readErr == nil {
				pid := strings.TrimSpace(string(rawPID))
				_ = exec.Command("kill", "-TERM", pid).Run()
			}
		})
	}
}

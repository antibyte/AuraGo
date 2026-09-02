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
		"refresh_git_relationship",
		`[ "$GIT_REMOTE_AHEAD" -eq 0 ]`,
		"integrate_origin_main",
		"--healthcheck-timeout 60s",
		"--healthcheck-timeout 210s --healthcheck-require-tsnet",
		"--print-tsnet-state-dir",
		"TSNET_WAS_READY",
		"cp -a --",
		"restore_tsnet_state_backup",
		"AuraGo required SIGKILL",
		"Update incomplete: tsnet readiness failed",
		"node-specific reauthentication",
		"do not reauthenticate unless /api/tsnet/status reports a login or node-key error",
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

func TestUpdaterTsNetFailureGuidanceDistinguishesTimeoutFromAuthentication(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("behavioral updater test requires a POSIX shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}

	update := readRepoFile(t, "update.sh")
	start := strings.Index(update, "tsnet_failure_guidance() {")
	if start < 0 {
		t.Fatal("could not find tsnet_failure_guidance")
	}
	end := strings.Index(update[start:], "\nconfigured_tsnet_state_dir_fallback() {")
	if end < 0 {
		t.Fatal("could not extract tsnet_failure_guidance")
	}
	harness := update[start:start+end] + `
timeout_guidance="$(tsnet_failure_guidance 'TSNET_TIMEOUT')"
auth_guidance="$(tsnet_failure_guidance 'TSNET_LOGIN_REQUIRED')"
[[ "$timeout_guidance" == *"do not reauthenticate"* ]]
[[ "$timeout_guidance" != *"use node-specific reauthentication"* ]]
[[ "$auth_guidance" == *"use node-specific reauthentication"* ]]
[[ "$auth_guidance" != *"do not reauthenticate"* ]]
`
	command := exec.Command("bash", "-c", harness)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("tsnet guidance contract failed: %v\n%s", err, output)
	}
}

func TestUpdaterSudoStrategyUsesControllingTTY(t *testing.T) {
	update := readRepoFile(t, "update.sh")
	for _, required := range []string{
		"has_interactive_tty() {",
		"[ -r /dev/tty ] && [ -w /dev/tty ]",
		": </dev/tty",
		"if has_interactive_tty; then",
		`SUDO="sudo -n"`,
		"timeout --foreground 60s $SUDO systemctl stop aurago",
		"timeout --help 2>&1 | grep -q -- '--foreground'",
	} {
		if !strings.Contains(update, required) {
			t.Fatalf("update.sh is missing TTY-aware sudo strategy marker %q", required)
		}
	}
	if strings.Contains(update, "if [ -t 0 ]; then") {
		t.Fatal("update.sh must not decide sudo interactivity from stdin alone")
	}
	if strings.Contains(update, "\n        if ! timeout 60s $SUDO systemctl stop aurago; then") {
		t.Fatal("update.sh must keep sudo in timeout's foreground process group")
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

	for _, test := range []struct {
		name   string
		counts string
		want   string
	}{
		{name: "identical checkout", counts: "0 0", want: "already at the latest version"},
		{name: "local rollout commits ahead", counts: "3 0", want: "preserves 3 local commit(s)"},
	} {
		t.Run(test.name, func(t *testing.T) {
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
if [ "${1:-}" = "-C" ]; then shift 2; fi
case "${1:-}" in
  fetch) exit 0 ;;
  rev-list) printf '%s\n' "$AURAGO_TEST_COUNTS"; exit 0 ;;
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
				"AURAGO_TEST_COUNTS="+test.counts,
			)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("no-op updater failed: %v\n%s", err, output)
			}
			if !strings.Contains(string(output), test.want) || !strings.Contains(string(output), "no files or services were changed") {
				t.Fatalf("no-op updater did not report the expected relationship:\n%s", output)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("no-op updater invoked a privileged/service command; stat error = %v", err)
			}
		})
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

func TestUpdaterPreparesOnlyIdenticalUntrackedMergeCollisions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("behavioral updater test requires a POSIX shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}

	update := readRepoFile(t, "update.sh")
	start := strings.Index(update, "prepare_untracked_merge_collisions() {")
	if start < 0 {
		t.Fatal("could not find prepare_untracked_merge_collisions in update.sh")
	}
	end := strings.Index(update[start:], "\n# ── Files & directories")
	if end < 0 {
		t.Fatal("could not extract prepare_untracked_merge_collisions from update.sh")
	}
	function := update[start : start+end]

	for _, test := range []struct {
		name         string
		localContent string
		wantSuccess  bool
	}{
		{name: "identical file is backed up and removed", localContent: "new manual\n", wantSuccess: true},
		{name: "different file is preserved and rejected", localContent: "local customization\n", wantSuccess: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			backup := filepath.Join(root, "backup")
			privateRuntime := filepath.Join(root, ".cache", "rod", "browser", "extensions")
			if err := os.MkdirAll(privateRuntime, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(privateRuntime, "private.txt"), []byte("runtime\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			runGit := func(args ...string) {
				t.Helper()
				command := exec.Command("git", args...)
				command.Dir = root
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("git %v failed: %v\n%s", args, err, output)
				}
			}
			runGit("init", "-q")
			runGit("config", "user.email", "updater-test@example.invalid")
			runGit("config", "user.name", "Updater Test")
			if err := os.WriteFile(filepath.Join(root, "base.txt"), []byte("base\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			runGit("add", "base.txt")
			runGit("commit", "-qm", "base")
			baseCommand := exec.Command("git", "rev-parse", "HEAD")
			baseCommand.Dir = root
			baseRaw, err := baseCommand.Output()
			if err != nil {
				t.Fatal(err)
			}
			baseRef := strings.TrimSpace(string(baseRaw))

			manual := filepath.Join(root, "prompts", "tools_manuals", "new_tool.md")
			if err := os.MkdirAll(filepath.Dir(manual), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manual, []byte("new manual\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			runGit("add", "prompts/tools_manuals/new_tool.md")
			runGit("commit", "-qm", "add manual")
			runGit("branch", "-f", "origin/main", "HEAD")
			runGit("reset", "--hard", "-q", baseRef)
			if err := os.MkdirAll(filepath.Dir(manual), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manual, []byte(test.localContent), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(privateRuntime, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(privateRuntime, 0o700) })

			harness := "#!/usr/bin/env bash\nset -euo pipefail\n" +
				"DIR=\"$AURAGO_TEST_ROOT\"\nBACKUP_DIR=\"$AURAGO_TEST_BACKUP\"\n" +
				"info() { :; }\nok() { :; }\nwarn() { :; }\n" + function +
				"\nprepare_untracked_merge_collisions\n"
			harnessPath := filepath.Join(root, "harness.sh")
			if err := os.WriteFile(harnessPath, []byte(harness), 0o700); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("bash", harnessPath)
			command.Env = append(os.Environ(), "AURAGO_TEST_ROOT="+root, "AURAGO_TEST_BACKUP="+backup)
			output, err := command.CombinedOutput()
			if test.wantSuccess && err != nil {
				t.Fatalf("collision preparation failed: %v\n%s", err, output)
			}
			if !test.wantSuccess && err == nil {
				t.Fatalf("different collision was accepted:\n%s", output)
			}

			_, fileErr := os.Stat(manual)
			backupPath := filepath.Join(backup, "untracked_merge_collisions", "prompts", "tools_manuals", "new_tool.md")
			_, backupErr := os.Stat(backupPath)
			if test.wantSuccess {
				if !os.IsNotExist(fileErr) {
					t.Fatalf("identical collision still exists: %v", fileErr)
				}
				if backupErr != nil {
					t.Fatalf("identical collision was not backed up: %v", backupErr)
				}
			} else {
				if fileErr != nil {
					t.Fatalf("different collision was not preserved: %v", fileErr)
				}
				if !os.IsNotExist(backupErr) {
					t.Fatalf("different collision unexpectedly created a replacement backup: %v", backupErr)
				}
			}
		})
	}
}

func TestUpdaterIntegratesOriginMainWithoutDiscardingLocalCommits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("behavioral updater test requires a POSIX shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}

	update := readRepoFile(t, "update.sh")
	start := strings.Index(update, "GIT_LOCAL_AHEAD=0")
	if start < 0 {
		t.Fatal("could not find updater git integration helpers")
	}
	end := strings.Index(update[start:], "\nprepare_untracked_merge_collisions() {")
	if end < 0 {
		t.Fatal("could not extract updater git integration helpers")
	}
	functions := update[start : start+end]

	for _, test := range []struct {
		name              string
		localChange       bool
		remoteChange      bool
		conflict          bool
		forceReset        bool
		wantSuccess       bool
		wantMerge         bool
		wantError         string
		wantLocalPreserve bool
	}{
		{name: "remote only fast forwards", remoteChange: true, wantSuccess: true},
		{name: "local only is already current", localChange: true, wantSuccess: true, wantLocalPreserve: true},
		{name: "clean divergence creates merge", localChange: true, remoteChange: true, wantSuccess: true, wantMerge: true, wantLocalPreserve: true},
		{name: "real conflict aborts and restores checkout", localChange: true, remoteChange: true, conflict: true, wantError: "merge_conflict"},
		{name: "explicit force reset still discards local commits", localChange: true, remoteChange: true, forceReset: true, wantSuccess: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			runGit := func(args ...string) string {
				t.Helper()
				command := exec.Command("git", args...)
				command.Dir = root
				output, err := command.CombinedOutput()
				if strings.Contains(strings.ToLower(string(output)), "permission denied") {
					t.Fatalf("collision preparation scanned an unrelated private runtime directory:\n%s", output)
				}
				if err != nil {
					t.Fatalf("git %v failed: %v\n%s", args, err, output)
				}
				return strings.TrimSpace(string(output))
			}
			writeFile := func(name, content string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			runGit("init", "-q", "-b", "local")
			runGit("config", "user.email", "fixture@example.invalid")
			runGit("config", "user.name", "Updater Fixture")
			writeFile("shared.txt", "base\n")
			runGit("add", "shared.txt")
			runGit("commit", "-qm", "base")
			baseRef := runGit("rev-parse", "HEAD")

			if test.localChange {
				if test.conflict {
					writeFile("shared.txt", "local\n")
				} else {
					writeFile("local.txt", "local rollout\n")
				}
				runGit("add", ".")
				runGit("commit", "-qm", "local rollout")
			}
			preUpdateRef := runGit("rev-parse", "HEAD")

			runGit("switch", "-q", "-c", "incoming", baseRef)
			if test.remoteChange {
				if test.conflict {
					writeFile("shared.txt", "remote\n")
				} else {
					writeFile("remote.txt", "remote update\n")
				}
				runGit("add", ".")
				runGit("commit", "-qm", "remote update")
			}
			remoteRef := runGit("rev-parse", "HEAD")
			runGit("branch", "-f", "origin/main", remoteRef)
			runGit("switch", "-q", "local")

			resultPath := filepath.Join(root, "integration-result")
			harness := `#!/usr/bin/env bash
set -euo pipefail
DIR="$AURAGO_TEST_ROOT"
PRE_UPDATE_REF="$AURAGO_TEST_PRE_UPDATE_REF"
FORCE_RESET=false
[ "${AURAGO_TEST_FORCE_RESET:-0}" != "1" ] || FORCE_RESET=true
info() { :; }
warn() { :; }
ok() { :; }
` + functions + `
if integrate_origin_main; then
    printf 'ok:%s\n' "$GIT_INTEGRATION_ERROR" > "$AURAGO_TEST_RESULT"
else
    rc=$?
    printf 'error:%s\n' "$GIT_INTEGRATION_ERROR" > "$AURAGO_TEST_RESULT"
    exit "$rc"
fi
`
			harnessPath := filepath.Join(root, "harness.sh")
			if err := os.WriteFile(harnessPath, []byte(harness), 0o700); err != nil {
				t.Fatal(err)
			}
			forceReset := "0"
			if test.forceReset {
				forceReset = "1"
			}
			command := exec.Command("bash", harnessPath)
			command.Env = append(os.Environ(),
				"AURAGO_TEST_ROOT="+root,
				"AURAGO_TEST_PRE_UPDATE_REF="+preUpdateRef,
				"AURAGO_TEST_FORCE_RESET="+forceReset,
				"AURAGO_TEST_RESULT="+resultPath,
			)
			output, err := command.CombinedOutput()
			if test.wantSuccess && err != nil {
				t.Fatalf("integration failed: %v\n%s", err, output)
			}
			if !test.wantSuccess && err == nil {
				t.Fatalf("integration unexpectedly succeeded:\n%s", output)
			}
			result, readErr := os.ReadFile(resultPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if test.wantError != "" && !strings.Contains(string(result), test.wantError) {
				t.Fatalf("integration result = %q, want error %q", result, test.wantError)
			}

			head := runGit("rev-parse", "HEAD")
			if !test.wantSuccess {
				if head != preUpdateRef {
					t.Fatalf("conflicted merge left HEAD at %s, want %s", head, preUpdateRef)
				}
				if status := runGit("status", "--porcelain", "--untracked-files=no"); status != "" {
					t.Fatalf("conflicted merge left tracked changes: %s", status)
				}
				verify := exec.Command("git", "rev-parse", "-q", "--verify", "MERGE_HEAD")
				verify.Dir = root
				if verify.Run() == nil {
					t.Fatal("conflicted merge left MERGE_HEAD behind")
				}
				return
			}

			if test.forceReset {
				if head != remoteRef {
					t.Fatalf("force reset left HEAD at %s, want %s", head, remoteRef)
				}
				return
			}
			if test.wantLocalPreserve {
				ancestor := exec.Command("git", "merge-base", "--is-ancestor", preUpdateRef, head)
				ancestor.Dir = root
				if err := ancestor.Run(); err != nil {
					t.Fatalf("local commit %s was not preserved by %s", preUpdateRef, head)
				}
			}
			if test.remoteChange {
				ancestor := exec.Command("git", "merge-base", "--is-ancestor", remoteRef, head)
				ancestor.Dir = root
				if err := ancestor.Run(); err != nil {
					t.Fatalf("remote commit %s was not integrated by %s", remoteRef, head)
				}
			}
			parents := strings.Fields(runGit("rev-list", "--parents", "-n", "1", "HEAD"))
			if test.wantMerge && len(parents) != 3 {
				t.Fatalf("clean divergence produced %d parent(s), want 2: %v", len(parents)-1, parents)
			}
			if !test.wantMerge && len(parents) > 2 {
				t.Fatalf("non-diverged update unexpectedly created a merge: %v", parents)
			}
		})
	}
}

func TestUpdaterGitFailuresAfterShutdownUseRollback(t *testing.T) {
	update := readRepoFile(t, "update.sh")
	for _, required := range []string{
		`abort_update "Update aborted safely because conflicting local and remote edits require review."`,
		`abort_update "Update aborted safely; local commits were not discarded."`,
		`abort_update "Cannot continue update while tracked files are locked/unwritable. Fix permissions or run with sudo."`,
		`abort_update "Could not restore config.yaml (permission denied)."`,
		"restore_untracked_merge_collisions_after_failure",
	} {
		if !strings.Contains(update, required) {
			t.Fatalf("post-shutdown git update failure must roll back and restart; missing %q", required)
		}
	}
}

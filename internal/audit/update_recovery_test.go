package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// recoveryScript builds a bash harness from the retention helpers (which
// contain update_manifest, update_recover_interrupted and update_exit_cleanup)
// plus extra extracted functions and the fixture body.
func runRecoveryBash(t *testing.T, extract []string, body string) (root, out string, runErr error) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	s := readRepoFile(t, "update.sh")
	var funcs strings.Builder
	funcs.WriteString(retentionHelpers(t))
	funcs.WriteString("\n")
	for i := 0; i+1 < len(extract); i += 2 {
		a := strings.Index(s, extract[i])
		if a < 0 {
			t.Fatalf("missing %q", extract[i])
		}
		b := strings.Index(s[a:], extract[i+1])
		if b < 0 {
			t.Fatalf("missing end %q", extract[i+1])
		}
		funcs.WriteString(s[a : a+b])
		funcs.WriteString("\n")
	}
	root = t.TempDir()
	slashRoot := filepath.ToSlash(root)
	scriptPath := filepath.Join(root, "harness.sh")
	if e := os.WriteFile(scriptPath, []byte("set -euo pipefail\n"+funcs.String()+"\n"+body), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(bash, filepath.ToSlash(scriptPath))
	cmd.Env = append(os.Environ(), "FIXTURE_ROOT="+slashRoot)
	b, e := cmd.CombinedOutput()
	return root, string(b), e
}

const recoveryPrelude = `
DIR="$FIXTURE_ROOT"
UPDATE_STATE="$DIR/.aurago-update"
BACKUP_DIR="$UPDATE_STATE/transactions/txn-AbC123"
mkdir -p "$BACKUP_DIR"
UPDATE_CREATED=1234
UPDATE_PREVIOUS_VERSION=$(printf 'a%.0s' {1..64})
UPDATE_PREVIOUS_ASSET=$(printf 'b%.0s' {1..64})
_AU_LOCK="$UPDATE_STATE/pid"
NO_RESTART=false
remove_regular_file_if_present() { return 0; }
tui_cleanup() { :; }
warn() { echo "WARN $*"; }
ok() { echo "OK $*"; }
update_manifest pending
`

func readManifestStatus(t *testing.T, root string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(root, ".aurago-update", "transactions", "txn-AbC123", "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func readCalls(root string) string {
	b, _ := os.ReadFile(filepath.Join(root, "calls"))
	return string(b)
}

func TestUpdateInterruptAfterStopRollsBackOnceWithoutRecursion(t *testing.T) {
	root, out, err := runRecoveryBash(t, nil, recoveryPrelude+`
UPDATE_BACKUP_COMPLETE=true
UPDATE_OUTCOME=pending
# Mirrors the real abort_update contract: marks itself handled, rolls back, dies.
abort_update() {
    echo "ABORT:$1" >> "$FIXTURE_ROOT/calls"
    if [ "$UPDATE_RECOVERY" = running ]; then :; else echo "BAD-STATE" >> "$FIXTURE_ROOT/calls"; fi
    UPDATE_RECOVERY=done
    update_manifest rolled_back
    exit 1
}
restart_unchanged_after_failed_stop() { echo "RESTART" >> "$FIXTURE_ROOT/calls"; }
trap update_exit_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
UPDATE_RECOVERY=armed
kill -INT $$
echo NOT-REACHED >> "$FIXTURE_ROOT/calls"
`)
	if err == nil {
		t.Fatalf("expected the interrupted script to exit non-zero\n%s", out)
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 130 {
		t.Fatalf("exit code must stay 130: %v\n%s", err, out)
	}
	calls := readCalls(root)
	if strings.Count(calls, "ABORT:Update interrupted") != 1 || strings.Contains(calls, "BAD-STATE") || strings.Contains(calls, "NOT-REACHED") || strings.Contains(calls, "RESTART") {
		t.Fatalf("unexpected calls: %q\n%s", calls, out)
	}
	if m := readManifestStatus(t, root); !strings.Contains(m, `"status":"rolled_back"`) {
		t.Fatalf("manifest should be rolled_back, got %s", m)
	}
}

func TestUpdateInterruptBeforeBackupCompleteRestartsPreviousInstance(t *testing.T) {
	root, out, err := runRecoveryBash(t, nil, recoveryPrelude+`
UPDATE_BACKUP_COMPLETE=false
UPDATE_OUTCOME=pending
abort_update() { echo "ABORT" >> "$FIXTURE_ROOT/calls"; exit 1; }
restart_unchanged_after_failed_stop() { echo "RESTART" >> "$FIXTURE_ROOT/calls"; return 0; }
trap update_exit_cleanup EXIT
trap 'exit 143' TERM
UPDATE_RECOVERY=armed
kill -TERM $$
`)
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 143 {
		t.Fatalf("exit code must stay 143: %v\n%s", err, out)
	}
	calls := readCalls(root)
	if calls != "RESTART\n" {
		t.Fatalf("expected exactly one restart and no rollback: %q", calls)
	}
	if m := readManifestStatus(t, root); !strings.Contains(m, `"status":"rolled_back"`) {
		t.Fatalf("untouched install should resolve as rolled_back, got %s", m)
	}
}

func TestUpdateExitRecoveryStaysOutOfUnarmedAndSettledStates(t *testing.T) {
	for _, state := range []string{"off", "done", "running"} {
		root, out, err := runRecoveryBash(t, nil, recoveryPrelude+`
UPDATE_BACKUP_COMPLETE=true
UPDATE_OUTCOME=pending
abort_update() { echo "ABORT" >> "$FIXTURE_ROOT/calls"; exit 1; }
restart_unchanged_after_failed_stop() { echo "RESTART" >> "$FIXTURE_ROOT/calls"; }
trap update_exit_cleanup EXIT
UPDATE_RECOVERY=`+state+`
exit 7
`)
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 7 {
			t.Fatalf("%s: exit code must stay 7: %v\n%s", state, err, out)
		}
		if c := readCalls(root); c != "" {
			t.Fatalf("%s: recovery must not run, got %q", state, c)
		}
	}
	// A clean exit after arming (rc 0) is not an interruption either.
	root, out, err := runRecoveryBash(t, nil, recoveryPrelude+`
UPDATE_BACKUP_COMPLETE=true
UPDATE_OUTCOME=confirmed
abort_update() { echo "ABORT" >> "$FIXTURE_ROOT/calls"; exit 1; }
trap update_exit_cleanup EXIT
UPDATE_RECOVERY=armed
`)
	if err != nil || readCalls(root) != "" {
		t.Fatalf("rc 0 must not recover: %v %q %s", err, readCalls(root), out)
	}
}

const abortUpdateStart = "abort_update() {"
const abortUpdateEnd = "\nbackup_current_aurago_binary\nbackup_tsnet_state"

const abortStubs = `
PRE_UPDATE_REF=""
TSNET_WAS_READY=no
SUDO=""
systemctl() { return 0; }
timeout() { shift; "$@"; }
_kill_proc() { return 0; }
restore_previous_aurago_binary() { return 0; }
restore_untracked_merge_collisions_after_failure() { return 0; }
restore_binary_update_resources_after_failure() { return 0; }
restore_critical_user_data_after_failure() { return 0; }
restore_tsnet_state_backup() { return 0; }
restore_service_stop_timeout_dropin() { return 0; }
restart_previous_after_rollback() { return 0; }
binary_supports_option() { return 1; }
die() { echo "DIE $*"; exit 1; }
printf '#!/bin/sh\nexit 0\n' > "$FIXTURE_ROOT/aurago_stub"
chmod +x "$FIXTURE_ROOT/aurago_stub"
CURRENT_AURAGO_BIN="$FIXTURE_ROOT/aurago_stub"
UPDATE_BACKUP_COMPLETE=true
UPDATE_OUTCOME=pending
`

func TestUpdateAbortRestoresUserEditsBeforeMarkingRolledBack(t *testing.T) {
	// Restore failure must keep the transaction unresolved (not rolled_back).
	root, out, _ := runRecoveryBash(t, []string{abortUpdateStart, abortUpdateEnd}, recoveryPrelude+abortStubs+`
restore_user_edits_after_failure() { echo "EDITS" >> "$FIXTURE_ROOT/calls"; return 1; }
abort_update "boom"
`)
	if readCalls(root) != "EDITS\n" {
		t.Fatalf("edits restore must run: %q\n%s", readCalls(root), out)
	}
	if m := readManifestStatus(t, root); strings.Contains(m, "rolled_back") {
		t.Fatalf("failed edit restore must not mark rolled_back: %s", m)
	}

	root, out, _ = runRecoveryBash(t, []string{abortUpdateStart, abortUpdateEnd}, recoveryPrelude+abortStubs+`
restore_user_edits_after_failure() { echo "EDITS" >> "$FIXTURE_ROOT/calls"; return 0; }
update_manifest_orig=$(declare -f update_manifest)
update_manifest() { echo "MANIFEST:$1" >> "$FIXTURE_ROOT/calls"; }
abort_update "boom"
`)
	if got := readCalls(root); got != "EDITS\nMANIFEST:rolled_back\n" {
		t.Fatalf("edits must be restored before rolled_back: %q\n%s", got, out)
	}

	// --no-restart: a successful file rollback is still a resolved transaction.
	root, out, _ = runRecoveryBash(t, []string{abortUpdateStart, abortUpdateEnd}, recoveryPrelude+abortStubs+`
NO_RESTART=true
restore_user_edits_after_failure() { return 0; }
update_manifest() { echo "MANIFEST:$1" >> "$FIXTURE_ROOT/calls"; }
abort_update "boom"
`)
	if got := readCalls(root); got != "MANIFEST:rolled_back\n" {
		t.Fatalf("--no-restart rollback should resolve: %q\n%s", got, out)
	}
}

func TestUpdateRestoreUserEditsReappliesPromptsAndProtectedDirs(t *testing.T) {
	root, out, err := runRecoveryBash(t, []string{"restore_user_edits_after_failure() {", "\nbackup_binary_update_resources() {"}, `
DIR="$FIXTURE_ROOT/install"
PROMPTS_DIR="$DIR/prompts"
BACKUP_DIR="$FIXTURE_ROOT/backup"
PROTECTED_DIRS=(agent_workspace/tools agent_workspace/skills)
warn() { echo "WARN $*"; }
copy_tree_merge() { mkdir -p "$2"; cp -a "$1/." "$2/"; }
mkdir -p "$DIR/prompts" "$DIR/agent_workspace/tools" "$BACKUP_DIR/prompts__custom/sub" "$BACKUP_DIR/agent_workspace__tools" "$BACKUP_DIR/agent_workspace__skills"
echo upstream > "$DIR/prompts/a.md"
echo mine > "$BACKUP_DIR/prompts__custom/a.md"
echo mine2 > "$BACKUP_DIR/prompts__custom/sub/b.md"
echo tool > "$BACKUP_DIR/agent_workspace__tools/t.py"
echo skill > "$BACKUP_DIR/agent_workspace__skills/s.md"
restore_user_edits_after_failure
test "$(cat "$DIR/prompts/a.md")" = mine
test "$(cat "$DIR/prompts/sub/b.md")" = mine2
test "$(cat "$DIR/agent_workspace/tools/t.py")" = tool
test "$(cat "$DIR/agent_workspace/skills/s.md")" = skill
# A failing copy must be reported to the caller.
copy_tree_merge() { return 1; }
if restore_user_edits_after_failure; then echo "expected failure"; exit 9; fi
echo EDITS-OK
`)
	_ = root
	if err != nil || !strings.Contains(out, "EDITS-OK") {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestUpdateRecoveryContract(t *testing.T) {
	s := readRepoFile(t, "update.sh")
	body := func(start, end string) string {
		a := strings.Index(s, start)
		if a < 0 {
			t.Fatalf("missing %s", start)
		}
		b := strings.Index(s[a:], end)
		if b < 0 {
			t.Fatalf("missing %s", end)
		}
		return s[a : a+b]
	}
	if !strings.Contains(body("abort_update() {", "\n    warn"), "UPDATE_RECOVERY=done") {
		t.Error("abort_update must mark recovery handled before it can exit")
	}
	if !strings.Contains(body("abort_before_file_changes() {", "\n    if restart_unchanged"), "UPDATE_RECOVERY=done") {
		t.Error("abort_before_file_changes must mark recovery handled before it can exit")
	}
	abort := body("abort_update() {", "\nbackup_current_aurago_binary\nbackup_tsnet_state")
	edits := strings.Index(abort, "restore_user_edits_after_failure || rollback_ok=false")
	rolled := strings.Index(abort, "update_manifest rolled_back")
	cleanup := strings.Index(abort, "update_retention_cleanup")
	if edits < 0 || rolled < edits || cleanup < edits {
		t.Error("user edits must be restored before the transaction is marked rolled_back / retention runs")
	}
	arm := strings.Index(s, "UPDATE_RECOVERY=armed\n\n# Stop systemd first")
	stop := strings.Index(s, "_kill_proc \"aurago\" \"bin/aurago_linux\"")
	settled := strings.LastIndex(s, "UPDATE_RECOVERY=off   # update settled")
	confirmed := strings.LastIndex(s, "update_manifest confirmed")
	if arm < 0 || stop < arm || settled < 0 || confirmed < settled {
		t.Error("recovery must be armed before the shutdown and disarmed before the update is confirmed")
	}
	if !strings.Contains(s, "UPDATE_RECOVERY=off   # the updated core is healthy and intentionally kept") {
		t.Error("tsnet-incomplete exit must not roll back a healthy core")
	}
}

func TestUpdateInterruptWithFailingBinaryRestoreIsNotRolledBack(t *testing.T) {
	stubs := strings.Replace(abortStubs, "restore_previous_aurago_binary() { return 0; }\n", "", 1)
	for _, noRestart := range []string{"true", "false"} {
		root, out, err := runRecoveryBash(t, []string{abortUpdateStart, abortUpdateEnd, "restore_previous_aurago_binary() {", "\nrestore_critical_user_data_after_failure() {"}, recoveryPrelude+stubs+`
NO_RESTART=`+noRestart+`
mark_executable_if_present() { :; }
apply_aurago_setcap_if_available() { :; }
mkdir -p "$BACKUP_DIR/bin" "$DIR/bin"
echo old > "$BACKUP_DIR/bin/aurago_linux"
echo new > "$DIR/bin/aurago_linux"
restore_user_edits_after_failure() { return 0; }
# The new binary is busy: every copy of the old one fails.
cp() { return 1; }
update_manifest_real=1
trap update_exit_cleanup EXIT
trap 'exit 130' INT
UPDATE_RECOVERY=armed
kill -INT $$
`)
		if err == nil {
			t.Fatalf("noRestart=%s: expected non-zero exit\n%s", noRestart, out)
		}
		if m := readManifestStatus(t, root); strings.Contains(m, "rolled_back") {
			t.Fatalf("noRestart=%s: failed binary restore must not be rolled_back: %s\n%s", noRestart, m, out)
		}
	}
}

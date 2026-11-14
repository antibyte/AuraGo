package audit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func retentionHelpers(t *testing.T) string {
	t.Helper()
	s := readRepoFile(t, "update.sh")
	a := strings.Index(s, "update_json_string() {")
	b := strings.Index(s, "\nmark_executable_if_present() {")
	if a < 0 || b < a {
		t.Fatal("missing retention functions")
	}
	return s[a:b]
}

func runRetentionBash(t *testing.T, script string) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX behavioral test")
	}
	root := t.TempDir()
	scriptPath := filepath.Join(root, "harness.sh")
	if e := os.WriteFile(scriptPath, []byte("set -euo pipefail\n"+retentionHelpers(t)+"\n"+script), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(), "FIXTURE_ROOT="+root)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("bash fixture: %v\n%s", e, out)
	}
	return root, string(out)
}

func TestUpdateManifestEscapesPathsAndRecordsOutcome(t *testing.T) {
	root, _ := runRetentionBash(t, `
DIR="$FIXTURE_ROOT/install with \"quotes\" and \\ slash"
BACKUP_DIR="$FIXTURE_ROOT/txn-AbC123"
mkdir -p "$BACKUP_DIR"
UPDATE_CREATED=1234
UPDATE_PREVIOUS_VERSION=$(printf 'a%.0s' {1..64})
UPDATE_PREVIOUS_ASSET=$(printf 'b%.0s' {1..64})
UPDATE_NEW_ASSET=$(printf 'c%.0s' {1..64})
UPDATE_BACKUP_COMPLETE=true
update_manifest pending
update_manifest confirmed
`)
	b, e := os.ReadFile(filepath.Join(root, "txn-AbC123", "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var manifest map[string]any
	if e := json.Unmarshal(b, &manifest); e != nil {
		t.Fatal(e)
	}
	if manifest["status"] != "confirmed" || manifest["backup_complete"] != true || !strings.Contains(manifest["root"].(string), `"quotes"`) {
		t.Fatalf("invalid manifest %s", b)
	}
}

func TestUpdateExitRemovesOnlyOwnedScratchAndKeepsUncertainBackup(t *testing.T) {
	root, _ := runRetentionBash(t, `
DIR="$FIXTURE_ROOT"
UPDATE_STATE="$DIR/.aurago-update"
BACKUP_DIR="$UPDATE_STATE/transactions/txn-AbC123"
mkdir -p "$BACKUP_DIR" "$DIR/data/models"
echo keep > "$DIR/data/models/model"
UPDATE_WORK=$(mktemp -d "$UPDATE_STATE/work.XXXXXX")
echo build > "$UPDATE_WORK/build.tar.gz"
UPDATE_CREATED=1234
UPDATE_PREVIOUS_VERSION=$(printf 'a%.0s' {1..64})
UPDATE_PREVIOUS_ASSET=$(printf 'b%.0s' {1..64})
UPDATE_BACKUP_COMPLETE=true
UPDATE_OUTCOME=pending
_AU_LOCK="$UPDATE_STATE/pid"
remove_regular_file_if_present() { return 0; }
tui_cleanup() { :; }
warn() { echo "$*"; }
if update_safe_remove_work "$DIR/data/models"; then exit 3; fi
trap update_exit_cleanup EXIT
`)
	if _, e := os.Stat(filepath.Join(root, "data", "models", "model")); e != nil {
		t.Fatal(e)
	}
	work, _ := filepath.Glob(filepath.Join(root, ".aurago-update", "work.*"))
	if len(work) != 0 {
		t.Fatal("scratch survived exit")
	}
	b, e := os.ReadFile(filepath.Join(root, ".aurago-update", "transactions", "txn-AbC123", "manifest.json"))
	if e != nil || !strings.Contains(string(b), `"status":"uncertain"`) {
		t.Fatal(string(b), e)
	}
}

func TestUpdaterRetentionLifecycleContract(t *testing.T) {
	s := readRepoFile(t, "update.sh")
	for _, required := range []string{`flock -n 9`, `export GOCACHE="$UPDATE_STATE/go-cache"`, `-out "$UPDATE_WORK/packed"`, `--apply --adopt-legacy`, `update_manifest rolled_back`, `update_manifest uncertain`, `9>&-`, `update_space_check`, `UPDATE_BACKUP_COMPLETE=true`} {
		if !strings.Contains(s, required) {
			t.Errorf("missing %s", required)
		}
	}
	if strings.Contains(s, `mktemp -d /tmp/aurago-backup-`) || strings.Contains(s, `assetpack -out deploy`) {
		t.Fatal("unbounded artifact creation returned")
	}
	confirmed := strings.LastIndex(s, "update_manifest confirmed")
	ready := strings.LastIndex(s, "ok \"tsnet readiness verified.\"")
	if confirmed < ready || ready < 0 {
		t.Fatal("retention runs before readiness")
	}
	queued := strings.LastIndex(s, `UPDATE_RETENTION_BINARY="$LAUNCH_BIN"`)
	if queued < confirmed || strings.Contains(s[confirmed:], `update_retention_cleanup "$LAUNCH_BIN"`) {
		t.Fatal("successful restart still waits for retention or schedules it before confirmation")
	}
	start := strings.Index(s, "backup_binary_update_resources() {")
	end := strings.Index(s[start:], "\nrestore_binary_update_resources_after_failure() {")
	block := s[start : start+end]
	if strings.Contains(block, "agent_workspace assets ui") || !strings.Contains(block, `[ "${asset##*/}" != web ]`) {
		t.Fatal("binary backup copies all historic web sets")
	}
}

func TestUpdateSpaceFailureLeavesRuntimeUntouched(t *testing.T) {
	_, out := runRetentionBash(t, `
DIR="$FIXTURE_ROOT"
df() { printf 'Filesystem blocks used available capacity mount\nfixture 100 99 1 99%% /\n'; }
die() { echo "space refused"; return 17; }
if update_space_check 100; then exit 1; fi
echo "still running"
`)
	if !strings.Contains(out, "Not enough free space") || !strings.Contains(out, "still running") {
		t.Fatal(out)
	}
}

func TestUpdateRetentionCleanupUsesCompactOutputAndPreservesFailures(t *testing.T) {
	_, out := runRetentionBash(t, `
DIR="$FIXTURE_ROOT"
ok() { echo "OK $*"; }
warn() { echo "WARN $*"; }
MODE=modern
mock_binary() {
    if [ "$2" = --help ]; then
        if [ "$MODE" = modern ]; then echo '  -summary'; fi
        return 0
    fi
    if [ "$MODE" = modern ]; then
        [ "${@: -1}" = --summary ] || return 90
        echo 'Artifact cleanup: 3 removed, 20 kept; 2.07 GiB freed.'
    elif [ "$MODE" = legacy ]; then
        [ "${@: -1}" = --adopt-legacy ] || return 91
        echo '{"entries":[{"path":"private-legacy-inventory"}],"freed_bytes":42}'
    else
        echo 'cleanup error detail' >&2
        echo '{"error":"cleanup blocked"}'
        return 1
    fi
}
update_retention_cleanup mock_binary
MODE=legacy
update_retention_cleanup mock_binary
MODE=failure
update_retention_cleanup mock_binary
echo 'healthy update continues'
`)
	for _, want := range []string{"OK Artifact cleanup: 3 removed, 20 kept; 2.07 GiB freed.", "OK Artifact cleanup completed.", "cleanup error detail", "cleanup blocked", "WARN Update is healthy", "healthy update continues"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "private-legacy-inventory") || strings.Count(out, "OK ") != 2 {
		t.Fatalf("inventory leaked or failed cleanup reported success: %s", out)
	}
}

func TestUpdateBackgroundRetentionDetachesTerminalAndKeepsLock(t *testing.T) {
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("flock not available")
	}
	root, out := runRetentionBash(t, `
DIR="$FIXTURE_ROOT"
UPDATE_STATE="$DIR/.aurago-update"
mkdir -p "$UPDATE_STATE"
UPDATE_WORK=$(mktemp -d "$UPDATE_STATE/work.XXXXXX")
_AU_LOCK="$DIR/pid"
exec 9>"$UPDATE_STATE/update.lock"
flock -n 9
export AURAGO_UPDATE_LOCK_FD=9
UPDATE_OUTCOME=confirmed
UPDATE_RETENTION_BINARY=mock_binary
_svc_user=fixture
_svc_group=fixture
SUDO=sudo
sudo() {
    if [ -e "$DIR/release-cleanup" ]; then
        test "$1" = -n || return 94
        shift
    fi
    "$@"
}
chown() {
    if [ "$1" = -R ] && [ -e "$DIR/release-cleanup" ]; then
        echo 'ownership restored after cleanup'
    fi
}
remove_regular_file_if_present() { return 0; }
tui_cleanup() { :; }
info() { echo "INFO $*"; }
ok() { echo "OK $*"; }
warn() { echo "WARN $*"; }
mock_binary() {
    if [ "$2" = --help ]; then echo '  -summary'; return 0; fi
    test ! -e "$UPDATE_WORK" || return 90
    test "$AURAGO_UPDATE_LOCK_FD" = 9 || return 91
    : >&9
    if read -r line; then return 92; fi
    kill -HUP "$BASHPID"
    echo 'cleanup started' >&2
    for attempt in {1..100}; do
        [ ! -e "$DIR/release-cleanup" ] || break
        sleep 0.05
    done
    test -e "$DIR/release-cleanup" || return 93
    echo 'Artifact cleanup: 0 removed, 2 kept; 0 B freed.'
}
trap update_exit_cleanup EXIT
echo 'update finished'
`)
	if !strings.Contains(out, "update finished") || !strings.Contains(out, "cleanup continues in the background") || strings.Contains(out, "cleanup started") {
		t.Fatalf("terminal output was not detached: %s", out)
	}
	logPath := filepath.Join(root, ".aurago-update", "cleanup.log")
	waitForLog := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			data, _ := os.ReadFile(logPath)
			if strings.Contains(string(data), want) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		data, _ := os.ReadFile(logPath)
		t.Fatalf("background cleanup did not log %q: %s", want, data)
	}
	waitForLog("cleanup started")
	if err := exec.Command("flock", "-n", filepath.Join(root, ".aurago-update", "update.lock"), "true").Run(); err == nil {
		t.Fatal("background cleanup released the update lock")
	}
	if err := os.WriteFile(filepath.Join(root, "release-cleanup"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitForLog("OK Artifact cleanup:")
	waitForLog("ownership restored after cleanup")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("flock", "-n", filepath.Join(root, ".aurago-update", "update.lock"), "true").Run() == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("finished background cleanup kept the update lock")
}

func TestUpdateBackgroundRetentionSkipsFailedOrDeferredUpdatesAndUnsafeLog(t *testing.T) {
	_, out := runRetentionBash(t, `
DIR="$FIXTURE_ROOT"
UPDATE_STATE="$DIR/.aurago-update"
mkdir -p "$UPDATE_STATE"
_AU_LOCK="$DIR/pid"
remove_regular_file_if_present() { return 0; }
tui_cleanup() { :; }
warn() { echo "WARN $*"; }
for mode in failed deferred; do
    (
        UPDATE_OUTCOME=confirmed
        UPDATE_RETENTION_BINARY=""
        trap update_exit_cleanup EXIT
        if [ "$mode" = failed ]; then
            UPDATE_RETENTION_BINARY=must_not_run
            exit 1
        fi
    ) || test "$?" = 1
    test ! -e "$UPDATE_STATE/cleanup.log"
done
echo keep > "$DIR/keep"
ln -s "$DIR/keep" "$UPDATE_STATE/cleanup.log"
update_start_retention_cleanup must_not_run
test "$(cat "$DIR/keep")" = keep
`)
	if !strings.Contains(out, "background cleanup could not create its log") {
		t.Fatal(out)
	}
}

func TestBinaryResourceRollbackDoesNotCopyOrReplaceWebSets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX behavioral test")
	}
	s := readRepoFile(t, "update.sh")
	start := strings.Index(s, "backup_binary_update_resources() {")
	end := strings.Index(s[start:], "\nrestart_previous_after_rollback() {")
	functions := s[start : start+end]
	_, _ = runRetentionBash(t, functions+`
DIR="$FIXTURE_ROOT/install"
BACKUP_DIR="$FIXTURE_ROOT/backup"
BINARY_ONLY=true
mkdir -p "$DIR/assets/web/pinned" "$DIR/assets/icons" "$DIR/prompts" "$BACKUP_DIR"
echo web > "$DIR/assets/web/pinned/manifest"
echo old > "$DIR/assets/icons/icon"
echo prompt > "$DIR/prompts/test"
copy_tree_merge() { mkdir -p "$2"; cp -a "$1/." "$2/"; }
abort_before_file_changes() { echo "$*"; exit 1; }
backup_binary_update_resources
test ! -e "$BACKUP_DIR/binary_update_resources/assets/web"
echo changed > "$DIR/assets/icons/icon"
mkdir -p "$DIR/assets/web/new-version"
restore_binary_update_resources_after_failure
test "$(cat "$DIR/assets/icons/icon")" = old
test "$(cat "$DIR/assets/web/pinned/manifest")" = web
test -d "$DIR/assets/web/new-version"
`)
}

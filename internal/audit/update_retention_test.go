package audit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

package audit

import (
	"fmt"
	"strings"
	"testing"
)

func TestUpdateConfiguredDatabaseRollbackPreservesSidecars(t *testing.T) {
	helper := updateScriptSpan(t, "safe_restore_file() {", "\nfile_owner_uid() {")
	script := `
DIR="$(mktemp -d)"
trap 'rm -rf -- "$DIR"' EXIT
BACKUP_DIR="$DIR/transaction"; SUDO=""
mkdir -p "$DIR/custom space" "$BACKUP_DIR"
printf old > "$DIR/custom space/planner.sqlite"
printf wal > "$DIR/custom space/planner.sqlite-wal"
printf shm > "$DIR/custom space/planner.sqlite-shm"
printf old2 > "$DIR/custom space/launchpad.data"
fixture_binary() { printf '%s\0' "$DIR/custom space/planner.sqlite" "$DIR/custom space/launchpad.data"; }
update_space_check() { return 0; }
backup_configured_databases fixture_binary
[ -f "$BACKUP_DIR/sqlite-complete" ]
# A harmless failed-start fixture changes databases and leaves stale sidecars.
printf new > "$DIR/custom space/planner.sqlite"
printf newwal > "$DIR/custom space/planner.sqlite-wal"
rm -f "$DIR/custom space/planner.sqlite-shm"
printf new2 > "$DIR/custom space/launchpad.data"
printf stale > "$DIR/custom space/launchpad.data-wal"
printf stale > "$DIR/custom space/launchpad.data-shm"
restore_configured_databases
[ "$(cat "$DIR/custom space/planner.sqlite")" = old ]
[ "$(cat "$DIR/custom space/planner.sqlite-wal")" = wal ]
[ "$(cat "$DIR/custom space/planner.sqlite-shm")" = shm ]
[ "$(cat "$DIR/custom space/launchpad.data")" = old2 ]
[ ! -e "$DIR/custom space/launchpad.data-wal" ]
[ ! -e "$DIR/custom space/launchpad.data-shm" ]
# A failed copy or disk check cannot publish the completion receipt.
rm -f "$BACKUP_DIR/sqlite-complete"
update_space_check() { return 1; }
if backup_configured_databases fixture_binary; then exit 90; fi
[ ! -e "$BACKUP_DIR/sqlite-complete" ]
update_space_check() { return 0; }
backup_data_file_with_sidecars() { return 1; }
if backup_configured_databases fixture_binary; then exit 91; fi
[ ! -e "$BACKUP_DIR/sqlite-complete" ]
`
	if output, err := runUpdateScript(t, helper+script); err != nil {
		t.Fatalf("rollback fixture failed: %v\n%s", err, output)
	}
	source := readRepoFile(t, "update.sh")
	backup := strings.Index(source, `backup_configured_databases "$LAUNCH_BIN" || abort_update`)
	start := strings.Index(source, "start_updated_directly() {")
	if backup < 0 || start < backup {
		t.Fatal("new binary can start before configured backups")
	}
}

func TestUpdateForeignChecksumManifestCannotBypassVerification(t *testing.T) {
	helper := updateScriptSpan(t, "fetch_release_checksums() {", "\nverify_release_asset() {")
	for _, valid := range []bool{true, false} {
		script := fmt.Sprintf(`
DIR="$(mktemp -d)"
trap 'rm -rf -- "$DIR"' EXIT
RELEASE_BASE=https://fixture.invalid
RELEASE_CHECKSUMS_FILE="$DIR/foreign"
printf foreign > "$RELEASE_CHECKSUMS_FILE"
release_verifier_fetch() { printf downloaded > "$2"; }
verify_release_checksums_signature() { printf verified >> "$DIR/calls"; %t; }
die() { return 1; }
if fetch_release_checksums; then
    %t || exit 90
    [ "$RELEASE_CHECKSUMS_FILE" != "$DIR/foreign" ]
    fetch_release_checksums
    [ "$(cat "$DIR/calls")" = verified ]
    rm -f "$VERIFIED_RELEASE_CHECKSUMS_FILE"
else
    %t && exit 91
fi
[ "$(cat "$DIR/foreign")" = foreign ]
[ -f "$DIR/calls" ]
`, valid, valid, valid)
		if output, err := runUpdateScript(t, helper+script); err != nil {
			t.Fatalf("valid=%v: %v\n%s", valid, err, output)
		}
	}
	initial := updateScriptSpan(t, "set -euo pipefail", "# ── UI & Typography")
	if !strings.Contains(initial, `RELEASE_CHECKSUMS_FILE=""`) || !strings.Contains(initial, `VERIFIED_RELEASE_CHECKSUMS_FILE=""`) {
		t.Fatal("inherited verification receipts were not cleared")
	}
	cleanup := updateScriptSpan(t, "update_exit_cleanup() {", "\nupdate_retention_cleanup() {")
	if strings.Contains(cleanup, `remove_regular_file_if_present "$RELEASE_CHECKSUMS_FILE"`) {
		t.Fatal("cleanup can delete a caller-owned manifest")
	}
}

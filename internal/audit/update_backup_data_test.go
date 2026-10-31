package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// U3: SQLite -wal/-shm sidecars travel with their DB, and a restored DB never
// keeps a stale sidecar of a newer build.
func TestUpdateDataFilesBackupAndRestoreSidecars(t *testing.T) {
	root, out, err := runRecoveryBash(t, []string{"safe_restore_file() {", "copy_tree_merge() {"}, `
SUDO=""
DIR="$FIXTURE_ROOT/inst"; BK="$FIXTURE_ROOT/bk"
mkdir -p "$DIR/data" "$BK"
# a.db has sidecars at backup time, b.db has none.
echo A > "$DIR/data/a.db"; echo AW > "$DIR/data/a.db-wal"; echo AS > "$DIR/data/a.db-shm"
echo B > "$DIR/data/b.db"
backup_data_file_with_sidecars "$DIR/data/a.db" "$BK/a.db"
backup_data_file_with_sidecars "$DIR/data/b.db" "$BK/b.db"
# newer build changes everything and leaves WALs behind.
echo A2 > "$DIR/data/a.db"; echo A2W > "$DIR/data/a.db-wal"; rm -f "$DIR/data/a.db-shm"
echo B2 > "$DIR/data/b.db"; echo B2W > "$DIR/data/b.db-wal"; echo B2S > "$DIR/data/b.db-shm"
restore_data_file_with_sidecars "$BK/a.db" "$DIR/data/a.db"
restore_data_file_with_sidecars "$BK/b.db" "$DIR/data/b.db"
`)
	if err != nil {
		t.Fatalf("harness failed: %v\n%s", err, out)
	}
	read := func(p string) string {
		b, e := os.ReadFile(filepath.Join(root, "inst", "data", p))
		if e != nil {
			return "<missing>"
		}
		return strings.TrimSpace(string(b))
	}
	if read("a.db") != "A" || read("a.db-wal") != "AW" || read("a.db-shm") != "AS" {
		t.Fatalf("a.db sidecars not restored together: %s %s %s", read("a.db"), read("a.db-wal"), read("a.db-shm"))
	}
	if read("b.db") != "B" || read("b.db-wal") != "<missing>" || read("b.db-shm") != "<missing>" {
		t.Fatalf("b.db must lose stale sidecars: %s %s %s", read("b.db"), read("b.db-wal"), read("b.db-shm"))
	}
}

// U4: a failing prompts copy must keep the old directory.
func TestUpdatePromptsMigrationKeepsOldDirOnCopyFailure(t *testing.T) {
	script := `
DIR="$FIXTURE_ROOT/inst"
mkdir -p "$DIR/agent_workspace/prompts" "$DIR/prompts"
echo custom > "$DIR/agent_workspace/prompts/mine.md"
subsection() { :; }; info() { :; }
warn() { echo "WARN $*"; }; ok() { echo "OK $*"; }
`
	src := readRepoFile(t, "update.sh")
	a := strings.Index(src, `OLD_PROMPTS="$DIR/agent_workspace/prompts"`)
	b := strings.Index(src, "# No stash to re-apply")
	if a < 0 || b < a {
		t.Fatal("prompts migration block not found")
	}
	block := src[a:b]
	var extract []string

	root, out, err := runRecoveryBash(t, extract, script+"rsync() { return 23; }\n"+block)
	if err != nil {
		t.Fatalf("harness failed: %v\n%s", err, out)
	}
	if _, e := os.Stat(filepath.Join(root, "inst", "agent_workspace", "prompts", "mine.md")); e != nil {
		t.Fatalf("old prompts must be kept when rsync fails: %v\n%s", e, out)
	}
	if !strings.Contains(out, "keeping the old directory") {
		t.Fatalf("expected a keep warning:\n%s", out)
	}

	// Success path still migrates and removes (cp fallback, no rsync).
	root, out, err = runRecoveryBash(t, extract, script+"command() { if [ \"$1\" = -v ] && [ \"$2\" = rsync ]; then return 1; fi; builtin command \"$@\"; }\n"+block)
	if err != nil {
		t.Fatalf("harness failed: %v\n%s", err, out)
	}
	if _, e := os.Stat(filepath.Join(root, "inst", "prompts", "mine.md")); e != nil {
		t.Fatalf("prompt not migrated: %v\n%s", e, out)
	}
	if _, e := os.Stat(filepath.Join(root, "inst", "agent_workspace", "prompts")); !os.IsNotExist(e) {
		t.Fatalf("old dir should be removed after a full copy\n%s", out)
	}
}

// U5: never fall back to /tmp; only trusted prior transaction backups count.
func TestUpdateConfigFallbackOnlyTrustedTransactionBackups(t *testing.T) {
	root, out, err := runRecoveryBash(t, []string{"file_owner_uid() {", "copy_tree_merge() {"}, `
T="$FIXTURE_ROOT/transactions"
mkdir -p "$T/txn-cur" "$T/txn-old" "$T/txn-bad" "$T/txn-link"
chmod 700 "$T" "$T/txn-cur" "$T/txn-old" "$T/txn-link"
echo old > "$T/txn-old/config.yaml"; chmod 600 "$T/txn-old/config.yaml"
echo bad > "$T/txn-bad/config.yaml"; chmod 770 "$T/txn-bad"
echo real > "$FIXTURE_ROOT/real.yaml"; ln -s "$FIXTURE_ROOT/real.yaml" "$T/txn-link/config.yaml"
touch -d '2 hours ago' "$T/txn-old"; touch -d '1 hour ago' "$T/txn-bad" "$T/txn-link"
# /tmp lookalike must be ignored entirely.
TMPB=$(mktemp -d /tmp/aurago-backup-XXXXXX); echo evil > "$TMPB/config.yaml"
got=$(find_trusted_previous_config "$T" "$T/txn-cur")
echo "GOT=$got"
rm -rf "$TMPB"
chmod 777 "$T"
if find_trusted_previous_config "$T" "$T/txn-cur" >/dev/null; then echo "WRITABLE-ROOT-ACCEPTED"; fi
`)
	if err != nil {
		t.Fatalf("harness failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "GOT="+filepath.ToSlash(root)+"/transactions/txn-old/config.yaml") {
		t.Fatalf("expected only the trusted txn-old config:\n%s", out)
	}
	if strings.Contains(out, "WRITABLE-ROOT-ACCEPTED") {
		t.Fatalf("a group/other-writable transactions dir must be rejected:\n%s", out)
	}
}

// U4 review: a find failure or a differing destination file keeps the old dir.
func TestUpdatePromptsMigrationKeepsOldDirOnFindFailureOrConflict(t *testing.T) {
	src := readRepoFile(t, "update.sh")
	a := strings.Index(src, `OLD_PROMPTS="$DIR/agent_workspace/prompts"`)
	b := strings.Index(src, "# No stash to re-apply")
	if a < 0 || b < a {
		t.Fatal("prompts migration block not found")
	}
	block := src[a:b]
	setup := `
DIR="$FIXTURE_ROOT/inst"
mkdir -p "$DIR/agent_workspace/prompts" "$DIR/prompts"
echo old > "$DIR/agent_workspace/prompts/mine.md"
subsection() { :; }; info() { :; }
warn() { echo "WARN $*"; }; ok() { echo "OK $*"; }
`
	cases := []struct{ name, pre, want string }{
		{"find failure", "find() { command find \"$@\"; return 1; }\n", "Could not fully migrate"},
		{"conflict", "echo new > \"$DIR/prompts/mine.md\"\n", "differ from the same file"},
	}
	for _, c := range cases {
		root, out, err := runRecoveryBash(t, nil, setup+c.pre+block)
		if err != nil {
			t.Fatalf("%s: harness failed: %v\n%s", c.name, err, out)
		}
		if _, e := os.Stat(filepath.Join(root, "inst", "agent_workspace", "prompts", "mine.md")); e != nil {
			t.Fatalf("%s: old prompts must be kept: %v\n%s", c.name, e, out)
		}
		if !strings.Contains(out, c.want) {
			t.Fatalf("%s: expected %q:\n%s", c.name, c.want, out)
		}
		if b, _ := os.ReadFile(filepath.Join(root, "inst", "prompts", "mine.md")); c.name == "conflict" && strings.TrimSpace(string(b)) != "new" {
			t.Fatalf("destination must not be overwritten: %q", b)
		}
	}
}

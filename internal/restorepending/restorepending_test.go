package restorepending

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(path string) bool { _, err := os.Lstat(path); return err == nil }

// stageFiles builds a staging tree from rel->content and returns entries.
func stageFiles(t *testing.T, root string, files map[string]string, sqlite map[string]bool) (string, []Entry) {
	t.Helper()
	staging, err := os.MkdirTemp(root, ".ago-import-*")
	if err != nil {
		t.Fatal(err)
	}
	var entries []Entry
	for rel, content := range files {
		write(t, StageFilePath(staging, rel), content)
		entries = append(entries, Entry{Clean: rel, Mode: 0o644, SQLite: sqlite[rel]})
	}
	return staging, entries
}

func TestApplyReplacesFilesRemovesSidecarsAndWritesResult(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config.yaml"), "old")
	write(t, filepath.Join(root, "data", "a.db"), "olddb")
	write(t, filepath.Join(root, "data", "a.db-wal"), "wal")
	write(t, filepath.Join(root, "data", "a.db-shm"), "shm")
	staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "new", "data/a.db": "newdb"}, map[string]bool{"data/a.db": true})
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	if st := ReadState(root); !st.Pending || st.PendingCreatedAt == "" {
		t.Fatalf("state = %+v", st)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "old" {
		t.Fatal("stage must not touch live files")
	}
	res, err := Apply(root, Options{})
	if err != nil || res.Status != StatusApplied {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "new" || read(t, filepath.Join(root, "data", "a.db")) != "newdb" {
		t.Fatal("files not replaced")
	}
	if exists(filepath.Join(root, "data", "a.db-wal")) || exists(filepath.Join(root, "data", "a.db-shm")) {
		t.Fatal("stale sidecars must be removed")
	}
	st := ReadState(root)
	if st.Pending || st.Last == nil || st.Last.Status != StatusApplied {
		t.Fatalf("state = %+v", st)
	}
	if res2, err := Apply(root, Options{}); res2 != nil || err != nil {
		t.Fatal("second apply must be a no-op")
	}
}

func TestApplyFailureRollsEverythingBack(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config.yaml"), "old")
	write(t, filepath.Join(root, "data", "a.db"), "olddb")
	write(t, filepath.Join(root, "data", "a.db-wal"), "wal")
	staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "new", "data/a.db": "newdb", "data/b.json": "b"}, map[string]bool{"data/a.db": true})
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	calls := 0
	res, err := Apply(root, Options{afterPublish: func(i int, e Entry) error {
		calls++
		if calls == 3 {
			return errors.New("injected")
		}
		return nil
	}})
	if err == nil || res == nil || res.Status != StatusRolledBack {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(res.Message, "nichts übernommen") {
		t.Fatalf("message must say nothing was applied: %q", res.Message)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "old" || read(t, filepath.Join(root, "data", "a.db")) != "olddb" || read(t, filepath.Join(root, "data", "a.db-wal")) != "wal" {
		t.Fatal("old state not restored")
	}
	if exists(filepath.Join(root, "data", "b.json")) {
		t.Fatal("newly created file must be removed on rollback")
	}
	if ReadState(root).Pending {
		t.Fatal("pending must be cleared after failure")
	}
}

func TestApplyAfterCrashKeepsTrueOriginalsForRollback(t *testing.T) {
	setup := func() string {
		root := t.TempDir()
		write(t, filepath.Join(root, "config.yaml"), "old")
		write(t, filepath.Join(root, "data", "a.json"), "olda")
		staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "new", "data/a.json": "newa"}, nil)
		if _, err := Stage(root, staging, entries); err != nil {
			t.Fatal(err)
		}
		// First run crashes after one published entry: no rollback, no finish.
		if _, err := Apply(root, Options{simulateCrash: 1}); !errors.Is(err, errSimulatedCrash) {
			t.Fatalf("crash err = %v", err)
		}
		if !ReadState(root).Pending {
			t.Fatal("pending must survive the crash")
		}
		return root
	}

	// Second run fails: the rollback must restore the TRUE originals, not the
	// half-published state.
	root := setup()
	res, err := Apply(root, Options{afterPublish: func(i int, e Entry) error { return errors.New("injected") }})
	if err == nil || res.Status != StatusRolledBack {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "old" || read(t, filepath.Join(root, "data", "a.json")) != "olda" {
		t.Fatal("originals lost after crash + failure")
	}

	// Second run succeeds: everything applied.
	root = setup()
	res, err = Apply(root, Options{})
	if err != nil || res.Status != StatusApplied {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "new" || read(t, filepath.Join(root, "data", "a.json")) != "newa" {
		t.Fatal("second apply did not publish everything")
	}
}

func TestApplyInvalidRestoredConfigRollsBack(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config.yaml"), "old")
	staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "broken"}, nil)
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(root, Options{Validate: func() error { return errors.New("bad yaml") }})
	if err == nil || res.Status != StatusRolledBack || read(t, filepath.Join(root, "config.yaml")) != "old" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestApplyRejectsUntrustedManifest(t *testing.T) {
	cases := map[string]func(t *testing.T, root string) []Entry{
		"traversal": func(t *testing.T, root string) []Entry { return []Entry{{Clean: "data/../../evil.txt"}} },
		"not allowed": func(t *testing.T, root string) []Entry {
			return []Entry{{Clean: "start.bat"}}
		},
		"reserved": func(t *testing.T, root string) []Entry { return []Entry{{Clean: "data/.restore-last.json"}} },
		"symlinked parent": func(t *testing.T, root string) []Entry {
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, "prompts")); err != nil {
				t.Skip("symlinks unavailable: " + err.Error())
			}
			return []Entry{{Clean: "prompts/x.md"}}
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "data", "keep.json"), "keep")
			entries := mk(t, root)
			staging, _ := stageFiles(t, root, map[string]string{"data/ok.json": "ok"}, nil)
			// The legitimate file is part of the same restore and must not be published.
			entries = append([]Entry{{Clean: "data/ok.json", Mode: 0o644}}, entries...)
			write(t, StageFilePath(staging, "prompts/x.md"), "x")
			if _, err := Stage(root, staging, entries); err != nil {
				t.Fatal(err)
			}
			res, err := Apply(root, Options{})
			if err == nil || res.Status != StatusFailed {
				t.Fatalf("res=%+v err=%v", res, err)
			}
			if exists(filepath.Join(root, "data", "ok.json")) {
				t.Fatal("nothing may be published when any entry is rejected")
			}
			if ReadState(root).Pending {
				t.Fatal("rejected restore must be cleared")
			}
		})
	}
}

func TestStageReplacesOlderPending(t *testing.T) {
	root := t.TempDir()
	s1, e1 := stageFiles(t, root, map[string]string{"data/one.json": "1"}, nil)
	id1, err := Stage(root, s1, e1)
	if err != nil {
		t.Fatal(err)
	}
	s2, e2 := stageFiles(t, root, map[string]string{"data/two.json": "2"}, nil)
	id2, err := Stage(root, s2, e2)
	if err != nil || id1 == id2 {
		t.Fatalf("id1=%s id2=%s err=%v", id1, id2, err)
	}
	if exists(StagedPath(root, "data/one.json")) || !exists(StagedPath(root, "data/two.json")) {
		t.Fatal("newer pending must replace the older one")
	}
	if _, err := Apply(root, Options{}); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(root, "data", "one.json")) || read(t, filepath.Join(root, "data", "two.json")) != "2" {
		t.Fatal("only the newer restore may be applied")
	}
	left, _ := filepath.Glob(filepath.Join(root, "data", ".restore-pending*"))
	if len(left) != 0 {
		t.Fatalf("leftovers: %v", left)
	}
}

func TestDiscardRemovesPendingAndStagedVault(t *testing.T) {
	root := t.TempDir()
	staging, entries := stageFiles(t, root, map[string]string{"data/one.json": "1"}, nil)
	write(t, StagedVaultPath(staging), "blob")
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	if ok, err := Discard(root); err != nil || !ok {
		t.Fatalf("discard = %v %v", ok, err)
	}
	if ReadState(root).Pending || exists(pendingDir(root)) {
		t.Fatal("pending not removed")
	}
	if ok, _ := Discard(root); ok {
		t.Fatal("second discard must report false")
	}
}

func newTestVault(t *testing.T, dir string) *security.Vault {
	t.Helper()
	v, err := security.NewVault(strings.Repeat("ab", 32), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func stageWithVault(t *testing.T, root string, v *security.Vault) {
	t.Helper()
	staging, entries := stageFiles(t, root, map[string]string{"data/one.json": "1"}, nil)
	plain, _ := json.Marshal(map[string]string{"restored_key": "restored_value"})
	enc, err := v.EncryptBytes(plain)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(StagedVaultPath(staging), enc, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
}

func TestStagedVaultIsMergedOnlyAfterAppliedFiles(t *testing.T) {
	// Applied: secrets merged and staged file removed.
	root := t.TempDir()
	v := newTestVault(t, root)
	stageWithVault(t, root, v)
	if _, err := Apply(root, Options{}); err != nil {
		t.Fatal(err)
	}
	ApplyStagedVault(root, v, slog.Default())
	if got, err := v.ReadSecret("restored_key"); err != nil || got != "restored_value" {
		t.Fatalf("secret = %q err=%v", got, err)
	}
	if m, _ := filepath.Glob(filepath.Join(root, "data", ".restore-vault-*")); len(m) != 0 {
		t.Fatalf("staged vault left behind: %v", m)
	}

	// Rolled back: secrets are never applied.
	root = t.TempDir()
	v = newTestVault(t, root)
	stageWithVault(t, root, v)
	_, _ = Apply(root, Options{afterPublish: func(int, Entry) error { return errors.New("injected") }})
	ApplyStagedVault(root, v, slog.Default())
	if _, err := v.ReadSecret("restored_key"); err == nil {
		t.Fatal("secrets of a rolled-back restore must not be applied")
	}
}

func TestApplyWaitsForConfirmationOfSecurityChanges(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config.yaml"), "old")
	staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "new"}, nil)
	id, err := StageWithChanges(root, staging, entries, &SecurityChanges{AuthDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if st := ReadState(root); !st.Pending || !st.AwaitingConfirmation || st.PendingID != id || !st.SecurityChanges.AuthDisabled {
		t.Fatalf("state = %+v", st)
	}
	if res, err := Apply(root, Options{}); res != nil || err != nil {
		t.Fatalf("unconfirmed apply must be a silent no-op: res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "old" || !ReadState(root).Pending || ReadState(root).Last != nil {
		t.Fatal("unconfirmed restore must stay pending without a result")
	}
	if err := Confirm(root, "other"); !errors.Is(err, ErrIDMismatch) {
		t.Fatalf("Confirm wrong id = %v", err)
	}
	if err := Confirm(root, id); err != nil {
		t.Fatal(err)
	}
	if res, err := Apply(root, Options{}); err != nil || res == nil || res.Status != StatusApplied {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "new" {
		t.Fatal("confirmed restore not applied")
	}
	if err := Confirm(root, id); !errors.Is(err, ErrNoPending) {
		t.Fatalf("Confirm without pending = %v", err)
	}
}

func TestApplyWritesConfiguredDirectoriesAndKeepsSymlinkCheck(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	toolsDir := filepath.Join(other, "mytools")
	staging, entries := stageFiles(t, root, map[string]string{
		"agent_workspace/tools/a.py":      "tool",
		"agent_workspace/workdir/w.txt":   "work",
		"prompts/p.md":                    "prompt",
		"agent_workspace/skills/s/run.py": "skill",
	}, nil)
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	dirs := Dirs{Tools: toolsDir, Workspace: filepath.Join(other, "work")}
	if res, err := Apply(root, Options{ResolveDirs: func() (Dirs, error) { return dirs, nil }}); err != nil || res.Status != StatusApplied {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(toolsDir, "a.py")) != "tool" || read(t, filepath.Join(other, "work", "w.txt")) != "work" {
		t.Fatal("configured directories not used")
	}
	if read(t, filepath.Join(root, "prompts", "p.md")) != "prompt" || read(t, filepath.Join(root, "agent_workspace", "skills", "s", "run.py")) != "skill" {
		t.Fatal("unconfigured prefixes must stay below the config root")
	}
	if exists(filepath.Join(root, "agent_workspace", "tools", "a.py")) {
		t.Fatal("tool leaked into the config root")
	}

	// A symlinked configured root is refused and rolls nothing out.
	link := filepath.Join(other, "link")
	if err := os.Symlink(toolsDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	staging, entries = stageFiles(t, root, map[string]string{"agent_workspace/tools/b.py": "x"}, nil)
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(root, Options{ResolveDirs: func() (Dirs, error) { return Dirs{Tools: link}, nil }})
	if err == nil || res == nil || res.Status != StatusFailed {
		t.Fatalf("symlinked root must be rejected: res=%+v err=%v", res, err)
	}
}

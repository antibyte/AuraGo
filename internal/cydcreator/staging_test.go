package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyTreeCopiesProjectFilesOnly(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	writeTestFile(t, filepath.Join(src, "cyd.project.json"), "{}")
	writeTestFile(t, filepath.Join(src, "src", "app", "app_main.cpp"), "int a;")
	writeTestFile(t, filepath.Join(src, "src", "app", ".cyd-write-abc"), "partial")
	dst := filepath.Join(dir, "staging", "ccj_1")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "src", "app", "app_main.cpp")); err != nil || string(data) != "int a;" {
		t.Fatalf("copied file = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "src", "app", ".cyd-write-abc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a temporary write file was copied")
	}
	if err := copyTree(src, dst); err == nil {
		t.Fatal("copying onto an existing directory succeeded")
	}
}

// stageCopy prepares a staging copy of the project with one changed file.
func stageCopy(t *testing.T, env *testEnv, p ProjectSummary, name string) string {
	t.Helper()
	stage := filepath.Join(env.svc.stagingRoot(), name)
	if err := copyTree(env.svc.projectDir(p.Slug), stage); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(stage, "src", "app", "app_main.cpp"), "// staged\n")
	return stage
}

func TestPublishStageSwapsAndRollsBack(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	original := readProjectFile(t, env, p, "src/app/app_main.cpp")
	stage := stageCopy(t, env, p, "ccj_fail")
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	_, err := env.svc.publishStage(ctx, row, stage, receiptPublish, "publish_ccj_fail", func(tx *sql.Tx, version int64) error {
		return errors.New("commit refused")
	})
	if err == nil || !strings.Contains(err.Error(), "commit refused") {
		t.Fatalf("publish error = %v", err)
	}
	if got := readProjectFile(t, env, p, "src/app/app_main.cpp"); got != original {
		t.Fatalf("project after rollback = %q", got)
	}
	if !dirExists(stage) {
		t.Fatal("the stage was not put back")
	}
	if entries, _ := os.ReadDir(env.svc.receiptsDir()); len(entries) != 0 {
		t.Fatalf("receipts left: %v", entries)
	}
	after, _ := env.svc.loadProjectRow(ctx, p.ID)
	if after.Version != row.Version {
		t.Fatalf("version changed to %d", after.Version)
	}
	version, err := env.svc.publishStage(ctx, row, stage, receiptPublish, "publish_ccj_ok", nil)
	if err != nil || version != row.Version+1 {
		t.Fatalf("publish = %d, %v", version, err)
	}
	if got := readProjectFile(t, env, p, "src/app/app_main.cpp"); got != "// staged\n" || dirExists(stage) {
		t.Fatalf("published file = %q, stage exists %v", got, dirExists(stage))
	}
	matches, _ := filepath.Glob(env.svc.projectDir(p.Slug) + ".cyd-backup-*")
	if len(matches) != 0 {
		t.Fatalf("backups left: %v", matches)
	}
	if _, err := env.svc.publishStage(ctx, row, filepath.Join(env.dir, "elsewhere"), receiptPublish, "publish_ccj_x", nil); err == nil {
		t.Fatal("a stage outside the staging folder was published")
	}
}

// A receipt marked committed (its backup could not be removed) is finished
// even after later saves moved the version on.
func TestRecoverReceiptFinishesACommittedReceiptAfterLaterSaves(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	r := publishReceipt{Version: receiptFormat, ID: "publish_ccj_late", Kind: receiptPublish, ProjectID: row.ID, Slug: row.Slug,
		Stage: "ccj_late", PreviousVersion: row.Version, NewVersion: row.Version + 1}
	receipt, err := env.svc.writeReceipt(r)
	if err != nil {
		t.Fatal(err)
	}
	env.svc.markReceiptCommitted(receipt, r)
	_, backup, _, _ := env.svc.receiptPaths(r)
	writeTestFile(t, filepath.Join(backup, ProjectFile), "{}")
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET version=version+4 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	env.svc.recoverReceipts(ctx)
	if dirExists(backup) {
		t.Fatal("the backup of a committed receipt was kept")
	}
	if entries, _ := os.ReadDir(env.svc.receiptsDir()); len(entries) != 0 {
		t.Fatalf("receipts left: %v", entries)
	}
}

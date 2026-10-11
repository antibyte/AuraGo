package cydcreator

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/fileutil"
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

// makeDirLink links link to the directory target: a junction on Windows
// (mklink /J needs no privilege), a symlink elsewhere. The link is removed
// before the temporary directories.
func makeDirLink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Skipf("mklink /J: %v %s", err, out)
		}
	} else if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
}

// swapWithoutCommit leaves the state of a crash after both renames of a
// publication; committed also performs the commit (version bump and receipt
// id in cc_projects.publish_receipt).
func swapWithoutCommit(t *testing.T, env *testEnv, p ProjectSummary, name string, committed bool) publishReceipt {
	t.Helper()
	ctx := context.Background()
	stage := stageCopy(t, env, p, name)
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	r := publishReceipt{Version: receiptFormat, ID: receiptPublish + "_" + name, Kind: receiptPublish, ProjectID: row.ID, Slug: row.Slug,
		Stage: filepath.Base(stage), PreviousVersion: row.Version, NewVersion: row.Version + 1}
	if _, err := env.svc.writeReceipt(r); err != nil {
		t.Fatal(err)
	}
	target, backup, _, _ := env.svc.receiptPaths(r)
	if err := fileutil.Rename(target, backup); err != nil {
		t.Fatal(err)
	}
	if err := fileutil.Rename(stage, target); err != nil {
		t.Fatal(err)
	}
	if committed {
		if _, err := env.svc.db.Exec(`UPDATE cc_projects SET version=?, publish_receipt=? WHERE id=?`, r.NewVersion, r.ID, row.ID); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func receiptsLeft(t *testing.T, env *testEnv) int {
	t.Helper()
	entries, _ := os.ReadDir(env.svc.receiptsDir())
	return len(entries)
}

func TestPublishStageRecordsTheReceiptInTheCommit(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	stageCopy(t, env, p, "ccj_mark")
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	if _, err := env.svc.publishStage(ctx, row, filepath.Join(env.svc.stagingRoot(), "ccj_mark"), receiptPublish, "publish_ccj_mark", nil); err != nil {
		t.Fatal(err)
	}
	var marker string
	if err := env.svc.db.QueryRow(`SELECT publish_receipt FROM cc_projects WHERE id=?`, p.ID).Scan(&marker); err != nil || marker != "publish_ccj_mark" {
		t.Fatalf("publish_receipt = %q, %v", marker, err)
	}
}

func TestRecoveryFinishesACommittedSwapAfterLaterSaves(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	r := swapWithoutCommit(t, env, p, "ccj_done", true)
	// Saves after the commit move the version on; the receipt id still decides.
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET version=version+3 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	env.svc.recoverReceipts(ctx)
	_, backup, _, _ := env.svc.receiptPaths(r)
	if got := readProjectFile(t, env, p, "src/app/app_main.cpp"); got != "// staged\n" || dirExists(backup) || receiptsLeft(t, env) != 0 {
		t.Fatalf("project = %q, backup kept %v, %d receipts", got, dirExists(backup), receiptsLeft(t, env))
	}
	if row, _ := env.svc.loadProjectRow(ctx, p.ID); row.PublishReview != "" {
		t.Fatal("a resolved receipt fenced the project")
	}
}

func TestRecoveryRollsBackAnUncommittedSwap(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	original := readProjectFile(t, env, p, "src/app/app_main.cpp")
	r := swapWithoutCommit(t, env, p, "ccj_half", false)
	env.svc.recoverReceipts(ctx)
	_, backup, stage, _ := env.svc.receiptPaths(r)
	if got := readProjectFile(t, env, p, "src/app/app_main.cpp"); got != original || dirExists(backup) || receiptsLeft(t, env) != 0 {
		t.Fatalf("project = %q, backup kept %v, %d receipts", got, dirExists(backup), receiptsLeft(t, env))
	}
	if !dirExists(stage) {
		t.Fatal("the new tree was not moved back to its stage")
	}
}

func TestAnUnresolvedReceiptFencesTheProject(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	r := swapWithoutCommit(t, env, p, "ccj_odd", false)
	// Not committed, yet the version moved: ambiguous, kept for review.
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET version=version+5 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	env.svc.recoverReceipts(ctx)
	if receiptsLeft(t, env) != 1 {
		t.Fatal("the ambiguous receipt was not kept")
	}
	_, backup, _, _ := env.svc.receiptPaths(r)
	if !dirExists(backup) {
		t.Fatal("the backup of an unresolved receipt was removed")
	}
	review := func(err error) {
		t.Helper()
		if e := wantCode(t, err, CodeBusy); e.Details["reason"] != "publication_review" {
			t.Fatalf("details = %v", e.Details)
		}
	}
	layout, version, _, err := env.svc.GetLayout(ctx, p.ID)
	if err != nil {
		t.Fatalf("reads must keep working: %v", err)
	}
	_, _, err = env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser)
	review(err)
	_, err = env.svc.WriteFile(ctx, p.ID, "src/app/x.cpp", []byte("int x;\n"), "", OriginUser)
	review(err)
	_, err = env.svc.RenameProject(ctx, p.ID, "Other")
	review(err)
	review(env.svc.DeleteProject(ctx, p.ID))
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	stage := stageCopy(t, env, p, "ccj_next")
	_, err = env.svc.publishStage(ctx, row, stage, receiptPublish, "publish_ccj_next", nil)
	review(err)
}

func TestRecoverySweepsBackupsWithoutReceipt(t *testing.T) {
	env, p := newProjectEnv(t)
	orphan := env.svc.projectDir(p.Slug) + ".cyd-backup-publish_ccj_gone"
	writeTestFile(t, filepath.Join(orphan, ProjectFile), "{}")
	pending := swapWithoutCommit(t, env, p, "ccj_pending", true)
	_ = os.Remove(filepath.Join(env.svc.receiptsDir(), pending.ID+".json"))
	unrelated := filepath.Join(env.svc.projectsDir, "notes.cyd-backup-x")
	writeTestFile(t, filepath.Join(unrelated, "a"), "a")
	env.svc.recoverReceipts(context.Background())
	_, pendingBackup, _, _ := env.svc.receiptPaths(pending)
	if dirExists(orphan) || dirExists(pendingBackup) || !dirExists(unrelated) {
		t.Fatalf("orphan kept %v, committed backup kept %v, unrelated removed %v", dirExists(orphan), dirExists(pendingBackup), !dirExists(unrelated))
	}
}

func TestReceiptPathsRefuseLinkedDirectories(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	elsewhere := filepath.Join(env.dir, "elsewhere")
	if err := os.MkdirAll(filepath.Join(elsewhere, "ccj_link"), 0o750); err != nil {
		t.Fatal(err)
	}
	makeDirLink(t, elsewhere, env.svc.stagingRoot())
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	r := publishReceipt{Version: receiptFormat, ID: "publish_ccj_link", Kind: receiptPublish, ProjectID: row.ID, Slug: row.Slug,
		Stage: "ccj_link", PreviousVersion: row.Version, NewVersion: row.Version + 1}
	if _, _, _, err := env.svc.receiptPaths(r); err == nil || !strings.Contains(err.Error(), "link") {
		t.Fatalf("receiptPaths through a linked staging folder = %v", err)
	}
	if _, err := env.svc.publishStage(ctx, row, filepath.Join(env.svc.stagingRoot(), "ccj_link"), receiptPublish, r.ID, nil); err == nil {
		t.Fatal("a stage behind a link was published")
	}
	if receiptsLeft(t, env) != 0 {
		t.Fatal("a refused publication wrote a receipt")
	}
}

func TestCopyTreeLimitsAndLinks(t *testing.T) {
	dir := t.TempDir()
	many := filepath.Join(dir, "many")
	for i := 0; i <= maxStagingFiles; i++ {
		writeTestFile(t, filepath.Join(many, "src", "app", fmt.Sprintf("f%04d.h", i)), "")
	}
	if err := copyTree(many, filepath.Join(dir, "out1")); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("copy of %d files = %v", maxStagingFiles+1, err)
	}
	linked := filepath.Join(dir, "linked")
	writeTestFile(t, filepath.Join(linked, ProjectFile), "{}")
	writeTestFile(t, filepath.Join(dir, "outside", "secret.h"), "x")
	makeDirLink(t, filepath.Join(dir, "outside"), filepath.Join(linked, "src"))
	if err := copyTree(linked, filepath.Join(dir, "out2")); err == nil {
		t.Fatal("a linked folder was copied")
	}
}

// OpenAsset hands out memory, so a preview being served never holds a file
// of the project open (on Windows that would block the directory swap).
func TestOpenAssetDoesNotBlockAPublication(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	data := pngBytes(t, 8, 8, false)
	asset, _, err := env.svc.PutAsset(ctx, p.ID, "logo.png", data)
	if err != nil {
		t.Fatal(err)
	}
	reader, _, _, err := env.svc.OpenAsset(ctx, p.ID, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	stage := stageCopy(t, env, p, "ccj_asset")
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	if _, err := env.svc.publishStage(ctx, row, stage, receiptPublish, "publish_ccj_asset", nil); err != nil {
		t.Fatalf("publication while an asset is served: %v", err)
	}
	if served, err := io.ReadAll(reader); err != nil || !bytes.Equal(served, data) {
		t.Fatalf("served %d bytes, %v", len(served), err)
	}
}

func TestPublishStageReportsAnOpenFileAsBusy(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to rename a directory with an open file inside")
	}
	env, p := newProjectEnv(t)
	ctx := context.Background()
	original := readProjectFile(t, env, p, "src/app/app_main.cpp")
	stage := stageCopy(t, env, p, "ccj_open")
	f, err := os.Open(filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "app_main.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	_, err = env.svc.publishStage(ctx, row, stage, receiptPublish, "publish_ccj_open", nil)
	_ = f.Close()
	if e := wantCode(t, err, CodeBusy); e.Details["reason"] != "file_open" {
		t.Fatalf("details = %v", e.Details)
	}
	if got := readProjectFile(t, env, p, "src/app/app_main.cpp"); got != original || !dirExists(stage) || receiptsLeft(t, env) != 0 {
		t.Fatalf("project = %q, stage kept %v, %d receipts", got, dirExists(stage), receiptsLeft(t, env))
	}
	if after, _ := env.svc.loadProjectRow(ctx, p.ID); after.Version != row.Version || after.PublishReview != "" {
		t.Fatalf("after a busy publication: %+v", after)
	}
}

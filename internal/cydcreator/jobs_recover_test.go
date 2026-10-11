package cydcreator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/fileutil"
)

// interruptSwap leaves the state of a crash after both renames; committed
// also bumps the version and records the receipt id (the commit point).
func interruptSwap(t *testing.T, env *testEnv, p ProjectSummary, id string, committed bool) (stage string) {
	t.Helper()
	ctx := context.Background()
	stage = stageCopy(t, env, p, strings.TrimPrefix(id, "publish_"))
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	r := publishReceipt{Version: receiptFormat, ID: id, Kind: receiptPublish, ProjectID: row.ID, Slug: row.Slug,
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
		// publish_receipt is part of the commit point (plan 3 Task 2 review fix ec18305d4).
		if _, err := env.svc.db.Exec(`UPDATE cc_projects SET version=?, publish_receipt=? WHERE id=?`, r.NewVersion, r.ID, row.ID); err != nil {
			t.Fatal(err)
		}
	}
	return stage
}

func TestRecoveryFinishesACommittedSwap(t *testing.T) {
	env, _, p := newJobEnv(t)
	interruptSwap(t, env, p, "publish_ccj_done", true)
	_ = env.svc.Close()
	env2 := reopenService(t, env)
	if got := readProjectFile(t, env2, p, "src/app/app_main.cpp"); got != "// staged\n" {
		t.Fatalf("project after recovery = %q", got)
	}
	matches, _ := filepath.Glob(env2.svc.projectDir(p.Slug) + ".cyd-backup-*")
	entries, _ := os.ReadDir(env2.svc.receiptsDir())
	if len(matches) != 0 || len(entries) != 0 {
		t.Fatalf("backups %v, receipts %v", matches, entries)
	}
}

// TestRestartRollsBackAnUncommittedSwap is the plan's
// TestRecoveryRollsBackAnUncommittedSwap, renamed: staging_test.go already has
// a test of that name (recoverReceipts called directly).
func TestRestartRollsBackAnUncommittedSwap(t *testing.T) {
	env, _, p := newJobEnv(t)
	original := readProjectFile(t, env, p, "src/app/app_main.cpp")
	interruptSwap(t, env, p, "publish_ccj_half", false)
	_ = env.svc.Close()
	env2 := reopenService(t, env)
	if got := readProjectFile(t, env2, p, "src/app/app_main.cpp"); got != original {
		t.Fatalf("project after rollback = %q", got)
	}
	entries, _ := os.ReadDir(env2.svc.receiptsDir())
	if len(entries) != 0 {
		t.Fatalf("receipts left: %v", entries)
	}
}

func TestRecoveryKeepsAnAmbiguousReceipt(t *testing.T) {
	env, _, p := newJobEnv(t)
	interruptSwap(t, env, p, "publish_ccj_odd", false)
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET version=version+5 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	_ = env.svc.Close()
	env2 := reopenService(t, env)
	entries, _ := os.ReadDir(env2.svc.receiptsDir())
	if len(entries) != 1 {
		t.Fatalf("receipts = %v, want the ambiguous one kept", entries)
	}
}

func TestRestartTurnsAnInterruptedJobIntoADraft(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	job := Job{ID: "ccj_crashed", ProjectID: p.ID, Mode: JobModeChange, Status: JobRunning, Phase: PhaseCoding, Prompt: "x",
		StagingDir: stagingRel("ccj_crashed"), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET locked_by_job=? WHERE id=?`, job.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.createRevision(ctx, p.ID, root, "x", RevisionBeforeJob, job.ID, ""); err != nil {
		t.Fatal(err)
	}
	_ = root.Close()
	stageCopy(t, env, p, job.ID)
	orphan := filepath.Join(env.svc.stagingRoot(), "ccj_orphan")
	writeTestFile(t, filepath.Join(orphan, "cyd.project.json"), "{}")
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	got, _ := env2.svc.GetJob(ctx, job.ID)
	summary, _ := env2.svc.GetProject(ctx, p.ID)
	if got.Status != JobInterrupted || summary.LockedByJob != "" || !summary.HasDraft {
		t.Fatalf("job = %+v, project = %+v", got, summary)
	}
	if dirExists(orphan) || !dirExists(env2.svc.stagingPath(job.ID)) {
		t.Fatal("orphan kept or draft removed")
	}
	events, _ := env2.svc.EventsAfter(ctx, p.ID, 0, 500)
	var sawStatus bool
	for _, ev := range events {
		sawStatus = sawStatus || (ev.Type == EventJobStatus && ev.JobID == job.ID && strings.Contains(string(ev.Data), `"interrupted"`))
	}
	msgs, _ := env2.svc.ListMessages(ctx, p.ID)
	if !sawStatus || len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1].Text, "restarted") {
		t.Fatalf("status event %v, messages %+v", sawStatus, msgs)
	}
	runner := newFakeRunner()
	env2.svc.SetRunner(runner)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		return env2.svc.FinishJob(ctx, run.Job.ID, "resumed after restart")
	})
	resumed, err := env2.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, env2, resumed.ID, JobSucceeded, JobFailed); done.Status != JobSucceeded {
		t.Fatalf("resumed job = %+v", done)
	}
	if got := readProjectFile(t, env2, p, "src/app/app_main.cpp"); got != "// staged\n" {
		t.Fatalf("published after restart = %q", got)
	}
}

// wantPublicationReview checks that a write is refused by the publication
// fence.
func wantPublicationReview(t *testing.T, err error) {
	t.Helper()
	if e := wantCode(t, err, CodeBusy); e.Details["reason"] != "publication_review" {
		t.Fatalf("details = %v", e.Details)
	}
}

// A receipt recovery cannot resolve fences only its own project and keeps its
// stage for review; another project's interrupted job still becomes a draft.
func TestRestartFencesOnlyTheProjectWhoseRecoveryFailed(t *testing.T) {
	env, _, a := newJobEnv(t)
	ctx := context.Background()
	b := env.createProject(t, "Zweites")
	// Uncommitted swap with the stage AND the new directory present: recovery
	// cannot tell which tree to keep.
	interruptSwap(t, env, a, "publish_ccj_both", false)
	stageCopy(t, env, a, "ccj_both")
	job := Job{ID: "ccj_other", ProjectID: b.ID, Mode: JobModeChange, Status: JobRunning, Phase: PhaseCoding, Prompt: "x",
		StagingDir: stagingRel("ccj_other"), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	stageCopy(t, env, b, job.ID)
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	if receiptsLeft(t, env2) != 1 || !dirExists(env2.svc.stagingPath("ccj_both")) {
		t.Fatalf("%d receipts, stage of the unresolved receipt kept %v", receiptsLeft(t, env2), dirExists(env2.svc.stagingPath("ccj_both")))
	}
	_, err := env2.svc.RenameProject(ctx, a.ID, "Anders")
	wantPublicationReview(t, err)
	summary, err := env2.svc.GetProject(ctx, b.ID)
	if err != nil || !summary.HasDraft || !dirExists(env2.svc.stagingPath(job.ID)) {
		t.Fatalf("project b = %+v, %v", summary, err)
	}
	if _, err := env2.svc.RenameProject(ctx, b.ID, "Weiter"); err != nil {
		t.Fatalf("an unaffected project was fenced: %v", err)
	}
}

// When recoverReceipts skips the receipts (unreadable folder or, here, more
// than maxReceipts entries), no project is known to be consistent: every
// project is fenced and no working copy is removed.
func TestRestartFencesEveryProjectWhenReceiptsCannotBeChecked(t *testing.T) {
	env, _, p := newJobEnv(t)
	for i := 0; i <= maxReceipts; i++ {
		writeTestFile(t, filepath.Join(env.svc.receiptsDir(), fmt.Sprintf("publish_ccj_%04d.json", i)), "{}")
	}
	orphan := filepath.Join(env.svc.stagingRoot(), "ccj_orphan")
	writeTestFile(t, filepath.Join(orphan, ProjectFile), "{}")
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	_, err := env2.svc.RenameProject(context.Background(), p.ID, "Anders")
	wantPublicationReview(t, err)
	if !dirExists(orphan) {
		t.Fatal("a working copy was removed although the receipts could not be checked")
	}
}

func TestDeleteProjectRemovesItsWorkingCopies(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	job := Job{ID: "ccj_draft", ProjectID: p.ID, Mode: JobModeChange, Status: JobFailed, Phase: PhaseCoding, Prompt: "x",
		StagingDir: stagingRel("ccj_draft"), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	stage := stageCopy(t, env, p, job.ID)
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET draft_job=? WHERE id=?`, job.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if dirExists(stage) {
		t.Fatal("the draft working copy of a deleted project was kept")
	}
}

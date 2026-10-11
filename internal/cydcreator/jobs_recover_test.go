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
	job := crashedJob(t, env, b, "ccj_other", PhaseCoding)
	stageCopy(t, env, b, job.ID)
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	if receiptsLeft(t, env2) != 1 || !dirExists(env2.svc.stagingPath("ccj_both")) {
		t.Fatalf("%d receipts, stage of the unresolved receipt kept %v", receiptsLeft(t, env2), dirExists(env2.svc.stagingPath("ccj_both")))
	}
	_, err := env2.svc.RenameProject(ctx, a.ID, "Anders")
	wantPublicationReview(t, err)
	if fenced, _ := env2.svc.GetProject(ctx, a.ID); !fenced.PublishReview {
		t.Fatal("the fenced project does not report publish_review")
	}
	summary, err := env2.svc.GetProject(ctx, b.ID)
	if err != nil || !summary.HasDraft || summary.PublishReview || !dirExists(env2.svc.stagingPath(job.ID)) {
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

// beforeJobRevision records the "before_job" revision of jobID from the
// project as it is.
func beforeJobRevision(t *testing.T, env *testEnv, p ProjectSummary, jobID string) {
	t.Helper()
	ctx := context.Background()
	row, _ := env.svc.loadProjectRow(ctx, p.ID)
	root, err := env.svc.openRoot(row, "")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := env.svc.createRevision(ctx, p.ID, root, "x", RevisionBeforeJob, jobID, ""); err != nil {
		t.Fatal(err)
	}
}

// crashedJob stores a job that was running in phase when the server stopped,
// with its project lock and its "before_job" revision (no working copy).
func crashedJob(t *testing.T, env *testEnv, p ProjectSummary, id, phase string) Job {
	t.Helper()
	job := Job{ID: id, ProjectID: p.ID, Mode: JobModeChange, Status: JobRunning, Phase: phase, Prompt: "x",
		StagingDir: stagingRel(id), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET locked_by_job=? WHERE id=?`, id, p.ID); err != nil {
		t.Fatal(err)
	}
	beforeJobRevision(t, env, p, id)
	return job
}

// finishedDraft stores a failed job whose working copy is the project's draft.
func finishedDraft(t *testing.T, env *testEnv, p ProjectSummary, id string) {
	t.Helper()
	job := Job{ID: id, ProjectID: p.ID, Mode: JobModeChange, Status: JobFailed, Phase: PhaseCoding, Prompt: "x",
		StagingDir: stagingRel(id), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	beforeJobRevision(t, env, p, id)
	stageCopy(t, env, p, id)
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET draft_job=? WHERE id=?`, id, p.ID); err != nil {
		t.Fatal(err)
	}
}

func draftOf(t *testing.T, env *testEnv, projectID string) string {
	t.Helper()
	row, err := env.svc.loadProjectRow(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	return row.DraftJob
}

// A job that crashed while it was being prepared (still queued, or without
// its "before_job" revision) holds a partial copy: the older draft stays and
// can still be resumed.
func TestRestartKeepsTheDraftWhenAJobCrashedWhilePreparing(t *testing.T) {
	for _, tc := range []struct {
		name, phase string
		revision    bool
	}{{"queued", PhaseQueued, true}, {"no before_job revision", PhaseCoding, false}} {
		t.Run(tc.name, func(t *testing.T) {
			env, _, p := newJobEnv(t)
			ctx := context.Background()
			finishedDraft(t, env, p, "ccj_old")
			next := Job{ID: "ccj_new", ProjectID: p.ID, Mode: JobModeChange, Status: JobQueued, Phase: tc.phase, Prompt: "y",
				StagingDir: stagingRel("ccj_new"), ResumeOf: "ccj_old", CreatedAt: formatTime(env.svc.now())}
			if err := env.svc.insertJob(ctx, next); err != nil {
				t.Fatal(err)
			}
			if tc.revision {
				beforeJobRevision(t, env, p, next.ID)
			}
			if _, err := env.svc.db.Exec(`UPDATE cc_projects SET locked_by_job=? WHERE id=?`, next.ID, p.ID); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(env.svc.stagingPath(next.ID), ProjectFile), "{}") // partial copy
			_ = env.svc.Close()

			env2 := reopenService(t, env)
			if got := draftOf(t, env2, p.ID); got != "ccj_old" || !dirExists(env2.svc.stagingPath("ccj_old")) || dirExists(env2.svc.stagingPath(next.ID)) {
				t.Fatalf("draft %q, old kept %v, partial copy kept %v", got, dirExists(env2.svc.stagingPath("ccj_old")), dirExists(env2.svc.stagingPath(next.ID)))
			}
			runner := newFakeRunner()
			env2.svc.SetRunner(runner)
			runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
				return env2.svc.FinishJob(ctx, run.Job.ID, "resumed")
			})
			resumed, err := env2.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Resume: true})
			if err != nil {
				t.Fatalf("resume of the older draft: %v", err)
			}
			if done := waitJob(t, env2, resumed.ID, JobSucceeded, JobFailed); done.Status != JobSucceeded {
				t.Fatalf("resumed job = %+v", done)
			}
		})
	}
}

// A draft that could not be recorded fences the project for this start and
// keeps the working copy; the next start records it.
func TestRestartRetriesADraftThatCouldNotBeRecorded(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	job := crashedJob(t, env, p, "ccj_retry", PhaseCoding)
	stageCopy(t, env, p, job.ID)
	if _, err := env.svc.db.Exec(`CREATE TRIGGER test_refuse_draft BEFORE UPDATE OF draft_job ON cc_projects
		BEGIN SELECT RAISE(ABORT, 'draft writes refused'); END`); err != nil {
		t.Fatal(err)
	}
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	summary, _ := env2.svc.GetProject(ctx, p.ID)
	if summary.HasDraft || !summary.PublishReview || !dirExists(env2.svc.stagingPath(job.ID)) {
		t.Fatalf("after the failed draft write: %+v, working copy kept %v", summary, dirExists(env2.svc.stagingPath(job.ID)))
	}
	if _, err := env2.svc.db.Exec(`DROP TRIGGER test_refuse_draft`); err != nil {
		t.Fatal(err)
	}
	_ = env2.svc.Close()

	env3 := reopenService(t, env2)
	summary, _ = env3.svc.GetProject(ctx, p.ID)
	if !summary.HasDraft || summary.PublishReview || draftOf(t, env3, p.ID) != job.ID {
		t.Fatalf("after the retry: %+v", summary)
	}
	// Every reopenService clock starts at the same time, so look at all
	// messages instead of the last one.
	msgs, _ := env3.svc.ListMessages(ctx, p.ID)
	var retried bool
	for _, m := range msgs {
		retried = retried || strings.Contains(m.Text, "now kept as a draft")
	}
	if !retried {
		t.Fatalf("messages %+v", msgs)
	}
}

// A receipt that names no project (here: misnamed) could belong to any
// project: every project is fenced and reports publish_review.
func TestRestartFencesEveryProjectForAnUnattributableReceipt(t *testing.T) {
	env, _, p := newJobEnv(t)
	writeTestFile(t, filepath.Join(env.svc.receiptsDir(), "publish_ccj_bad.json"), "{}")
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	_, err := env2.svc.RenameProject(context.Background(), p.ID, "Anders")
	wantPublicationReview(t, err)
	list, err := env2.svc.ListProjects(context.Background())
	if err != nil || len(list) != 1 || !list[0].PublishReview {
		t.Fatalf("projects = %+v, %v", list, err)
	}
}

// The job's own publication was cut short after both renames: the restart
// rolls it back and the working copy becomes the draft.
func TestRestartTurnsTheJobsRolledBackPublicationIntoADraft(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	original := readProjectFile(t, env, p, "src/app/app_main.cpp")
	job := crashedJob(t, env, p, "ccj_pub", PhaseBuilding)
	interruptSwap(t, env, p, "publish_ccj_pub", false)
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	if got := readProjectFile(t, env2, p, "src/app/app_main.cpp"); got != original {
		t.Fatalf("project = %q", got)
	}
	summary, _ := env2.svc.GetProject(ctx, p.ID)
	if !summary.HasDraft || summary.PublishReview || !dirExists(env2.svc.stagingPath(job.ID)) || receiptsLeft(t, env2) != 0 {
		t.Fatalf("project %+v, working copy %v, %d receipts", summary, dirExists(env2.svc.stagingPath(job.ID)), receiptsLeft(t, env2))
	}
}

// A crash between the commit of a job's publication and its cleanup: the
// restart records the "job" revision and ends the private conversation.
func TestRestartCompletesACommittedJobPublication(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	job := Job{ID: "ccj_won", ProjectID: p.ID, Mode: JobModeChange, Status: JobSucceeded, Phase: PhaseDone, Prompt: "make it blue",
		StagingDir: stagingRel("ccj_won"), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`INSERT INTO cc_agent_context(job_id,project_id,data,updated_at) VALUES(?,?,?,?)`,
		job.ID, p.ID, []byte(`{}`), formatTime(env.svc.now())); err != nil {
		t.Fatal(err)
	}
	interruptSwap(t, env, p, "publish_ccj_won", true)
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	var revisions, contexts int
	_ = env2.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_revisions WHERE job_id=? AND reason=?`, job.ID, RevisionJob).Scan(&revisions)
	_ = env2.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_agent_context WHERE project_id=?`, p.ID).Scan(&contexts)
	if revisions != 1 || contexts != 0 || receiptsLeft(t, env2) != 0 {
		t.Fatalf("%d job revisions, %d conversations, %d receipts", revisions, contexts, receiptsLeft(t, env2))
	}
	msgs, _ := env2.svc.ListMessages(ctx, p.ID)
	if len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1].Text, "published") {
		t.Fatalf("messages %+v", msgs)
	}
}

func TestRestartKeepsTheWorkingCopyOfAJobAwaitingConfirmation(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	job := Job{ID: "ccj_wait", ProjectID: p.ID, Mode: JobModeNew, Status: JobAwaitingConfirmation, Phase: PhasePlanning, Prompt: "x",
		StagingDir: stagingRel("ccj_wait"), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET locked_by_job=? WHERE id=?`, job.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	stageCopy(t, env, p, job.ID)
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	got, _ := env2.svc.GetJob(ctx, job.ID)
	summary, _ := env2.svc.GetProject(ctx, p.ID)
	if got.Status != JobAwaitingConfirmation || summary.LockedByJob != job.ID || !dirExists(env2.svc.stagingPath(job.ID)) {
		t.Fatalf("job %+v, project %+v, working copy %v", got, summary, dirExists(env2.svc.stagingPath(job.ID)))
	}
}

func TestRestartReplacesTheOlderDraft(t *testing.T) {
	env, _, p := newJobEnv(t)
	finishedDraft(t, env, p, "ccj_older")
	job := crashedJob(t, env, p, "ccj_newer", PhaseCoding)
	stageCopy(t, env, p, job.ID)
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	if got := draftOf(t, env2, p.ID); got != job.ID || dirExists(env2.svc.stagingPath("ccj_older")) || !dirExists(env2.svc.stagingPath(job.ID)) {
		t.Fatalf("draft %q, older kept %v, newer kept %v", got, dirExists(env2.svc.stagingPath("ccj_older")), dirExists(env2.svc.stagingPath(job.ID)))
	}
}

func TestRestartClearsADraftWhoseWorkingCopyIsGone(t *testing.T) {
	env, _, p := newJobEnv(t)
	finishedDraft(t, env, p, "ccj_gone")
	if err := os.RemoveAll(env.svc.stagingPath("ccj_gone")); err != nil {
		t.Fatal(err)
	}
	_ = env.svc.Close()

	env2 := reopenService(t, env)
	if summary, _ := env2.svc.GetProject(context.Background(), p.ID); summary.HasDraft {
		t.Fatalf("project = %+v", summary)
	}
}

// A staging folder that is a link (junction on Windows) is never written or
// swept through: job start, restore, removeStaging and the startup sweep
// refuse.
func TestLinkedStagingFolderIsNeverFollowed(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	rev, err := env.svc.RecordRevision(ctx, p.ID, "x", RevisionImport)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(env.dir, "elsewhere")
	writeTestFile(t, filepath.Join(elsewhere, "ccj_orphan", ProjectFile), "{}")
	makeDirLink(t, elsewhere, env.svc.stagingRoot())
	env.svc.removeStaging("ccj_orphan")
	if _, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"}); err == nil {
		t.Fatal("a job started with a linked staging folder")
	}
	if _, _, err := env.svc.RestoreRevision(ctx, rev.ID); err == nil {
		t.Fatal("a restore ran through a linked staging folder")
	}
	_ = env.svc.Close()
	reopenService(t, env)
	entries, _ := os.ReadDir(elsewhere)
	if len(entries) != 1 || entries[0].Name() != "ccj_orphan" {
		t.Fatalf("the link target changed: %v", entries)
	}
}

func TestConfirmJobRefusesAFencedProject(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	job := Job{ID: "ccj_plan", ProjectID: p.ID, Mode: JobModeNew, Status: JobAwaitingConfirmation, Phase: PhasePlanning, Prompt: "x",
		StagingDir: stagingRel("ccj_plan"), CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_projects SET locked_by_job=? WHERE id=?`, job.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	stageCopy(t, env, p, job.ID)
	env.svc.setPublicationFence(p.ID, "publish_ccj_x")
	_, err := env.svc.ConfirmJob(ctx, job.ID, "")
	wantPublicationReview(t, err)
}

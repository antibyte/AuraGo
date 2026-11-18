package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"aurago/internal/fileutil"
)

// The diagnostic is emitted after the failed transaction has rolled back.
type completionRetryHandler struct {
	slog.Handler
	notify func()
}

func (h completionRetryHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "CYD Creator retrying job completion" {
		h.notify()
	}
	return h.Handler.Handle(ctx, r)
}

func holdCompletionRetry(t *testing.T, env *testEnv) (<-chan struct{}, func()) {
	t.Helper()
	retry, repaired := make(chan struct{}), make(chan struct{})
	resume := sync.OnceFunc(func() { close(repaired) })
	t.Cleanup(resume)
	env.svc.logger = slog.New(completionRetryHandler{env.svc.logger.Handler(), sync.OnceFunc(func() {
		close(retry)
		<-repaired
	})})
	return retry, resume
}

func TestResumeWaitsForAConcurrentSourceSave(t *testing.T) {
	env, _, p := newJobEnv(t)
	const draft, path = "ccj_saved", "src/app/user.cpp"
	finishedDraft(t, env, p, draft)
	entered, release := make(chan struct{}), make(chan struct{})
	releaseWriter := sync.OnceFunc(func() { close(release) })
	defer releaseWriter()
	ctx := fileutil.WithPublicationGate(context.Background(), func(commit func() error) error {
		close(entered)
		<-release
		return commit()
	})
	wrote := make(chan error, 1)
	go func() {
		_, err := env.svc.WriteFile(ctx, p.ID, path, []byte("int saved;"), "*", OriginUser)
		wrote <- err
	}()
	waitClosed(t, entered, "source save before its rename")
	resumed := make(chan error, 1)
	go func() {
		_, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Resume: true})
		resumed <- err
	}()
	// A premature reservation is visible in SQLite while the editor still
	// owns the project; a correctly serialized resume cannot insert that row.
	deadline := time.Now().Add(500 * time.Millisecond)
	var queued int
	for time.Now().Before(deadline) {
		if err := env.svc.db.QueryRow(`SELECT count(*) FROM cc_jobs WHERE resume_of=?`, draft).Scan(&queued); err != nil {
			t.Fatal(err)
		}
		if queued != 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	releaseWriter()
	if err := <-wrote; err != nil {
		t.Fatal(err)
	}
	if err := <-resumed; ErrorCode(err) != CodeJobState || queued != 0 {
		t.Fatalf("resume = %v, queued before save = %d", err, queued)
	}
	if got := readProjectFile(t, env, p, path); got != "int saved;" || draftOf(t, env, p.ID) != draft {
		t.Fatalf("saved source = %q, draft = %q", got, draftOf(t, env, p.ID))
	}
}

func TestAbortedResumeKeepsThePreviousDraft(t *testing.T) {
	env, _, p := newJobEnv(t)
	const old = "ccj_previous"
	finishedDraft(t, env, p, old)
	if _, err := env.svc.db.Exec(`CREATE TRIGGER refuse_revision BEFORE INSERT ON cc_revisions
		BEGIN SELECT RAISE(ABORT,'revision unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Resume: true})
	wantCode(t, err, CodeInternal)
	var id, status, lock, draft string
	var messages int
	err = env.svc.db.QueryRow(`SELECT j.id,j.status,p.locked_by_job,p.draft_job,(SELECT count(*) FROM cc_messages WHERE job_id=j.id)
		FROM cc_jobs j JOIN cc_projects p ON j.project_id=p.id WHERE j.resume_of=?`, old).Scan(&id, &status, &lock, &draft, &messages)
	if err != nil || status != JobFailed || lock != "" || draft != old || messages != 0 {
		t.Fatalf("aborted resume: status=%q lock=%q draft=%q messages=%d, error=%v", status, lock, draft, messages, err)
	}
	if dirExists(env.svc.stagingPath(id)) || !dirExists(env.svc.stagingPath(old)) {
		t.Fatal("aborted resume did not preserve the previous working copy")
	}
}

func TestJobCompletionRetriesTheWholeTransaction(t *testing.T) {
	for _, tc := range []struct{ name, fail, repair string }{
		{"project read", `ALTER TABLE cc_projects RENAME COLUMN name TO unavailable_name`,
			`ALTER TABLE cc_projects RENAME COLUMN unavailable_name TO name`},
		{"project write", `CREATE TRIGGER refuse_unlock BEFORE UPDATE OF locked_by_job ON cc_projects
			BEGIN SELECT RAISE(ABORT,'project write unavailable'); END`, `DROP TRIGGER refuse_unlock`},
		{"job write", `CREATE TRIGGER refuse_end BEFORE UPDATE ON cc_jobs
			BEGIN SELECT RAISE(ABORT,'job write unavailable'); END`, `DROP TRIGGER refuse_end`},
		{"commit", `CREATE TABLE completion_parent(id INTEGER PRIMARY KEY);
			CREATE TABLE completion_child(id INTEGER REFERENCES completion_parent(id) DEFERRABLE INITIALLY DEFERRED);
			CREATE TRIGGER refuse_commit AFTER UPDATE ON cc_jobs BEGIN INSERT INTO completion_child VALUES(1); END`,
			`DROP TRIGGER refuse_commit`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, _, p := newJobEnv(t)
			const old = "ccj_old"
			finishedDraft(t, env, p, old)
			job := crashedJob(t, env, p, "ccj_new", PhaseCoding)
			stageCopy(t, env, p, job.ID)
			for _, id := range []string{old, job.ID} {
				if _, err := env.svc.db.Exec(`INSERT INTO cc_agent_context VALUES(?,?,?,?)`, id, p.ID, []byte("{}"), job.CreatedAt); err != nil {
					t.Fatal(err)
				}
			}
			retry, resume := holdCompletionRetry(t, env)
			if _, err := env.svc.db.Exec(tc.fail); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- env.svc.finishJob(job, jobEnding{status: JobFailed, msg: "job_failed_internal"}) }()
			waitClosed(t, retry, "retry after database fault")
			var lock, draft, status string
			var contexts int
			check := func(wantLock, wantDraft, wantStatus string, wantContexts int) {
				t.Helper()
				err := env.svc.db.QueryRow(`SELECT locked_by_job,draft_job,status,
					(SELECT COUNT(*) FROM cc_agent_context WHERE project_id=p.id)
					FROM cc_projects p JOIN cc_jobs j ON j.project_id=p.id WHERE j.id=?`, job.ID).Scan(&lock, &draft, &status, &contexts)
				if err != nil || lock != wantLock || draft != wantDraft || status != wantStatus || contexts != wantContexts {
					t.Fatalf("lock=%q draft=%q job=%q contexts=%d, error=%v", lock, draft, status, contexts, err)
				}
			}
			check(job.ID, old, JobRunning, 2)
			if !dirExists(env.svc.stagingPath(old)) || !dirExists(env.svc.stagingPath(job.ID)) {
				t.Fatal("a failed completion removed a working copy")
			}
			if _, err := env.svc.db.Exec(tc.repair); err != nil {
				t.Fatal(err)
			}
			resume()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			check("", job.ID, JobFailed, 1)
			if dirExists(env.svc.stagingPath(old)) || !dirExists(env.svc.stagingPath(job.ID)) {
				t.Fatal("the committed draft was not retained")
			}
		})
	}
}

func TestRestartKeepsTheWorkingCopyOfAFailedPublication(t *testing.T) {
	env, _, p := newJobEnv(t)
	original := readProjectFile(t, env, p, "src/app/app_main.cpp")
	const old = "ccj_previous"
	finishedDraft(t, env, p, old)
	job := crashedJob(t, env, p, "ccj_green", PhaseBuilding)
	receipt := receiptPublish + "_" + job.ID
	var stage string
	if runtime.GOOS == "windows" {
		stage = stageCopy(t, env, p, job.ID)
		row, err := env.svc.loadProjectRow(context.Background(), p.ID)
		if err != nil {
			t.Fatal(err)
		}
		var held *os.File
		_, err = env.svc.publishStage(context.Background(), row, stage, receiptPublish, receipt, func(*sql.Tx, int64) error {
			var err error
			held, err = os.Open(filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "app_main.cpp"))
			if err != nil {
				return err
			}
			return errors.New("publication commit unavailable")
		})
		if held != nil {
			held.Close()
		}
		if held == nil || ErrorCode(err) != CodeBusy || env.svc.publicationFence(p.ID) != receipt || dirExists(stage) {
			t.Fatalf("failed put-back: error=%v, receipt=%q, stage exists=%v", err, env.svc.publicationFence(p.ID), dirExists(stage))
		}
	} else {
		// Unix can rename open directories. Restore the same persisted state.
		stage = interruptSwap(t, env, p, receipt, false)
		env.svc.setPublicationFence(p.ID, receipt)
	}
	if _, err := env.svc.db.Exec(`INSERT INTO cc_agent_context VALUES(?,?,?,?)`, job.ID, p.ID, []byte("{}"), job.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.finishJob(job, jobEnding{status: JobFailed, msg: "job_failed_internal"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`CREATE TRIGGER refuse_recovered_draft BEFORE UPDATE OF draft_job ON cc_projects
		BEGIN SELECT RAISE(ABORT, 'draft unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	_ = env.svc.Close()
	fenced := reopenService(t, env)
	if receiptsLeft(t, fenced) != 1 || draftOf(t, fenced, p.ID) != old || !dirExists(stage) || !dirExists(fenced.svc.stagingPath(old)) {
		t.Fatal("recovery discarded evidence before the new draft could be recorded")
	}
	if _, err := fenced.svc.db.Exec(`DROP TRIGGER refuse_recovered_draft`); err != nil {
		t.Fatal(err)
	}
	_ = fenced.svc.Close()
	recovered := reopenService(t, fenced)
	if got := readProjectFile(t, recovered, p, "src/app/app_main.cpp"); got != original {
		t.Fatalf("original project = %q", got)
	}
	data, err := os.ReadFile(filepath.Join(stage, "src", "app", "app_main.cpp"))
	if err != nil || string(data) != "// staged\n" || draftOf(t, recovered, p.ID) != job.ID || receiptsLeft(t, recovered) != 0 {
		t.Fatalf("recovered draft = %q, %v", data, err)
	}
	var status string
	var contexts int
	if err := recovered.svc.db.QueryRow(`SELECT status,(SELECT COUNT(*) FROM cc_agent_context WHERE job_id=?) FROM cc_jobs WHERE id=?`,
		job.ID, job.ID).Scan(&status, &contexts); err != nil || status != JobFailed || contexts != 1 || dirExists(recovered.svc.stagingPath(old)) {
		t.Fatalf("final status=%q contexts=%d, error=%v", status, contexts, err)
	}
}

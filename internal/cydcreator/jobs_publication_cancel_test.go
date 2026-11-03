package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

type publicationCheckContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *publicationCheckContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

func newPublicationRun(t *testing.T) (*testEnv, ProjectSummary, *jobRun) {
	t.Helper()
	env, p := newProjectEnv(t)
	job := Job{ID: "ccj_publication", ProjectID: p.ID, Mode: JobModeChange, Status: JobRunning, Phase: PhaseBuilding,
		CreatedAt: formatTime(env.svc.now()), StagingDir: stagingRel("ccj_publication")}
	if err := env.svc.insertJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	env.lockForJob(t, p.ID, job.ID)
	ctx, cancel := context.WithCancelCause(env.svc.ctx)
	t.Cleanup(func() { cancel(nil) })
	run := &jobRun{jobID: job.ID, projectID: p.ID, job: job, stage: stageCopy(t, env, p, job.ID), ctx: ctx, cancel: cancel}
	env.svc.jobs.mu.Lock()
	env.svc.jobs.active = run
	env.svc.jobs.mu.Unlock()
	return env, p, run
}

func TestPublicationDoesNotOverwriteAnAcceptedCancel(t *testing.T) {
	env, p, run := newPublicationRun(t)
	if err := env.svc.CancelJob(context.Background(), run.jobID); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.publishOnce(run, Build{ID: "ccb_ready"}); !errors.Is(err, errJobCancelled) {
		t.Fatalf("publication after cancel = %v", err)
	}
	row, _ := env.svc.loadProjectRow(context.Background(), p.ID)
	job, _ := env.svc.loadJob(context.Background(), run.jobID)
	if row.Version != p.Version || job.Status == JobSucceeded || !dirExists(run.stage) {
		t.Fatalf("cancelled publication changed state: row=%+v job=%+v", row, job)
	}
}

func TestPublicationWinsBeforeWaitingForTheProjectLock(t *testing.T) {
	env, p, run := newPublicationRun(t)
	observed := &publicationCheckContext{Context: run.ctx, checked: make(chan struct{})}
	run.ctx = observed
	unlock := sync.OnceFunc(env.svc.lockProject(p.ID))
	t.Cleanup(unlock)
	published := make(chan error, 1)
	go func() { published <- env.svc.publishOnce(run, Build{ID: "ccb_ready"}) }()
	waitClosed(t, observed.checked, "publication entry")
	cancelled := make(chan error, 1)
	go func() { cancelled <- env.svc.CancelJob(context.Background(), run.jobID) }()
	select {
	case err := <-cancelled:
		unlock()
		<-published
		t.Fatalf("cancel returned while publication held the gate: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	wantCode(t, <-cancelled, CodeJobState)
	job, _ := env.svc.loadJob(context.Background(), run.jobID)
	if job.Status != JobSucceeded || run.ctx.Err() != nil {
		t.Fatalf("committed job was cancelled: job=%+v cause=%v", job, context.Cause(run.ctx))
	}
}

func TestPublicationTimeoutBeforeTheSwapKeepsTheStage(t *testing.T) {
	env, p, run := newPublicationRun(t)
	observed := &publicationCheckContext{Context: run.ctx, checked: make(chan struct{})}
	run.ctx = observed
	unlock := sync.OnceFunc(env.svc.lockProject(p.ID))
	t.Cleanup(unlock)
	published := make(chan error, 1)
	go func() { published <- env.svc.publishOnce(run, Build{ID: "ccb_ready"}) }()
	waitClosed(t, observed.checked, "publication entry")
	run.cancel(errJobTimeout)
	unlock()
	if err := <-published; err == nil {
		t.Fatal("a timed-out job was published")
	}
	row, _ := env.svc.loadProjectRow(context.Background(), p.ID)
	job, _ := env.svc.loadJob(context.Background(), run.jobID)
	if row.Version != p.Version || job.Status == JobSucceeded || !dirExists(run.stage) {
		t.Fatalf("timeout changed the project: row=%+v job=%+v", row, job)
	}
}

func TestPublishStageCancellationBoundary(t *testing.T) {
	for _, beforeSwap := range []bool{true, false} {
		name := "after swap"
		if beforeSwap {
			name = "before swap"
		}
		t.Run(name, func(t *testing.T) {
			env, p := newProjectEnv(t)
			stage := stageCopy(t, env, p, "ccj_boundary")
			row, _ := env.svc.loadProjectRow(context.Background(), p.ID)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if beforeSwap {
				cancel()
			}
			_, err := env.svc.publishStage(ctx, row, stage, receiptPublish, "publish_ccj_boundary", func(tx *sql.Tx, _ int64) error {
				cancel()
				_, err := tx.Exec(`UPDATE cc_projects SET name='Published' WHERE id=?`, p.ID)
				return err
			})
			if beforeSwap {
				if !errors.Is(err, context.Canceled) || !dirExists(stage) || receiptsLeft(t, env) != 0 {
					t.Fatalf("cancel before swap = %v, stage=%v receipts=%d", err, dirExists(stage), receiptsLeft(t, env))
				}
				row, _ := env.svc.loadProjectRow(context.Background(), p.ID)
				if row.Version != p.Version {
					t.Fatalf("cancelled publication bumped version to %d", row.Version)
				}
			} else if err != nil || dirExists(stage) {
				t.Fatalf("cancel after swap interrupted commit: %v", err)
			}
		})
	}
}

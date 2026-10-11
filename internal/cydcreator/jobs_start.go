package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

const (
	continuePrompt     = "Continue the previous task. Keep its goal and the completed work; finish what is missing and fix what still fails."
	generateCodePrompt = "Write the app logic for this layout: implement every logic hook and app_setup/app_loop."
	attachedLogPrompt  = "Look at the attached log and fix the problem it shows."
)

// StartJob validates the request, takes the global agent slot and the project
// lock, copies the project (or the saved draft when resuming) into the job's
// working copy, records the "before_job" revision and starts the phases in
// the background. Modes: new (planning + confirmation), change and
// generate_code (straight to coding). A project locked by another job or
// fenced by an unresolved publication is CYD_BUSY.
func (s *Service) StartJob(ctx context.Context, projectID string, in StartJobInput) (Job, error) {
	switch in.Mode {
	case JobModeNew, JobModeChange, JobModeGenerateCode:
	default:
		return Job{}, NewError(CodeBadRequest, fmt.Sprintf("unknown job mode %q", in.Mode), map[string]any{"mode": in.Mode})
	}
	prompt, err := validPrompt(in.Prompt)
	if err != nil {
		return Job{}, err
	}
	attachments, err := validAttachments(in.Attachments)
	if err != nil {
		return Job{}, err
	}
	switch {
	case prompt != "":
	case in.Resume:
		prompt = continuePrompt
	case in.Mode == JobModeGenerateCode:
		prompt = generateCodePrompt
	case len(attachments) > 0:
		prompt = attachedLogPrompt
	default:
		return Job{}, NewError(CodeBadRequest, "describe what the agent should do", nil)
	}
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		return Job{}, s.internalError("start job", err)
	}
	if in.Mode == JobModeGenerateCode && !in.Resume {
		if err := s.requireValidLayout(row); err != nil {
			return Job{}, err
		}
	}

	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	if s.jobs.runner == nil {
		return Job{}, NewError(CodeDisabled, "the CYD Creator agent is not available", nil)
	}
	if s.jobs.closing || s.ctx.Err() != nil {
		return Job{}, NewError(CodeBusy, "CYD Creator is shutting down", nil)
	}
	active, err := s.activeJob(ctx)
	if err != nil {
		// A failed read must not open a second slot.
		return Job{}, NewError(CodeBusy, "the agent slot could not be checked; try again", nil)
	}
	if active != nil || s.jobs.active != nil {
		details := map[string]any{}
		if active != nil {
			details["job_id"], details["project_id"] = active.JobID, active.ProjectID
		}
		return Job{}, NewError(CodeBusy, "another agent job is active", details)
	}
	// A fresh row: the lock and the publication fence may have changed since
	// the validation above. setProjectLock has no fence check of its own.
	if row, err = s.loadProjectRow(ctx, projectID); err != nil {
		return Job{}, s.internalError("start job", err)
	}
	if row.LockedByJob != "" {
		return Job{}, NewError(CodeBusy, "the project is locked by another job", map[string]any{"job_id": row.LockedByJob})
	}
	if err := checkLock(row, OriginUser); err != nil {
		return Job{}, err
	}
	job := Job{ID: s.ids.NewID("ccj"), ProjectID: projectID, Mode: in.Mode, Status: JobQueued, Phase: PhaseQueued,
		Prompt: prompt, CreatedAt: formatTime(s.now())}
	job.StagingDir = stagingRel(job.ID)
	startPhase := PhaseCoding
	if in.Mode == JobModeNew {
		startPhase = PhasePlanning
	}
	copyFrom := s.projectDir(row.Slug)
	if in.Resume {
		draft, phase, err := s.resumableDraft(ctx, row)
		if err != nil {
			return Job{}, err
		}
		job.ResumeOf, copyFrom = draft.ID, s.stagingPath(draft.ID)
		if draft.Plan != nil && phase != "" {
			job.Plan, startPhase = draft.Plan, phase
		}
	}
	if err := s.insertJob(ctx, job); err != nil {
		return Job{}, s.internalError("start job", err)
	}
	// setProjectLock calls are serialized by jobs.mu (held here).
	if err := s.setProjectLock(ctx, projectID, job.ID); err != nil {
		s.abortStart(job, err)
		return Job{}, s.internalError("start job", err)
	}
	// setProjectLock waited for every write that held the project mutex (a
	// restore whose put-back failed sets the fence), and the job lock now
	// refuses new ones: a fence seen here is final.
	if s.publicationFence(projectID) != "" {
		s.abortStart(job, errors.New("the project is fenced by an unresolved publication"))
		return Job{}, publicationReviewError()
	}
	stage := s.stagingPath(job.ID)
	if err := s.checkStagingRoot(); err != nil {
		s.abortStart(job, err)
		return Job{}, s.internalError("start job", err)
	}
	_ = os.RemoveAll(stage)
	userChanges, err := s.prepareJob(ctx, row, job, copyFrom, stage)
	if err != nil {
		_ = os.RemoveAll(stage)
		s.abortStart(job, err)
		return Job{}, s.internalError("start job", err)
	}
	if _, err := s.AddUserMessage(ctx, projectID, job.ID, prompt, attachments); err != nil {
		s.logger.Warn("CYD Creator job message could not be stored", "job_id", job.ID, "error", err)
	}
	run := &jobRun{jobID: job.ID, projectID: projectID, job: job, startPhase: startPhase, stage: stage,
		attachments: attachments, userChanges: userChanges, resumed: in.Resume, phase: PhaseQueued,
		budget: s.Limits().JobTimeout, plan: job.Plan}
	s.Emit(projectID, EventJobStatus, job.ID, "", map[string]any{"job": job})
	s.launchRun(run)
	return job, nil
}

// requireValidLayout refuses generate_code for a layout with errors: the
// coding agent cannot repair the designer's layout problems.
func (s *Service) requireValidLayout(row projectRow) error {
	root, err := s.openRoot(row, "")
	if err != nil {
		return s.internalError("start job", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return s.internalError("start job", err)
	}
	var errs []Issue
	for _, is := range validateLayout(p) {
		if is.Severity == SeverityError {
			errs = append(errs, is)
		}
	}
	if len(errs) > 0 {
		return NewError(CodeInvalidLayout, "fix the layout problems in the designer before generating code",
			map[string]any{"issues": errs})
	}
	return nil
}

// resumableDraft returns the project's saved draft and the phase a resume
// starts in ("" = the mode's first phase). A draft whose project changed since
// the draft's job started is stale: publishing it would undo those changes.
func (s *Service) resumableDraft(ctx context.Context, row projectRow) (Job, string, error) {
	if row.DraftJob == "" || !dirExists(s.stagingPath(row.DraftJob)) {
		return Job{}, "", NewError(CodeJobState, "this project has no draft to resume", nil)
	}
	draft, err := s.loadJob(ctx, row.DraftJob)
	if err != nil {
		return Job{}, "", s.internalError("resume job", err)
	}
	current, err := treeFingerprint(s.projectDir(row.Slug))
	if err != nil {
		return Job{}, "", s.internalError("resume job", err)
	}
	base, ok, err := s.jobBaseFingerprint(ctx, draft.ID)
	if err != nil {
		return Job{}, "", s.internalError("resume job", err)
	}
	if !ok || base != current {
		return Job{}, "", NewError(CodeJobState, "the project changed after this draft was saved; discard the draft or start a new job",
			map[string]any{"job_id": draft.ID})
	}
	switch draft.Phase {
	case PhaseQueued, PhasePlanning:
		return draft, "", nil
	case PhaseLayout:
		return draft, PhaseLayout, nil
	}
	return draft, PhaseCoding, nil
}

// jobBaseFingerprint is the fingerprint of the job's "before_job" revision.
func (s *Service) jobBaseFingerprint(ctx context.Context, jobID string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM cc_revisions WHERE job_id=? AND reason=? ORDER BY created_at, id LIMIT 1`,
		jobID, RevisionBeforeJob).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	fp, err := s.revisionFingerprint(ctx, id)
	return fp, err == nil, err
}

// prepareJob copies the working copy, records the "before_job" revision and
// computes what the user changed since the last successful job.
func (s *Service) prepareJob(ctx context.Context, row projectRow, job Job, copyFrom, stage string) (*UserChanges, error) {
	if err := copyTree(copyFrom, stage); err != nil {
		return nil, fmt.Errorf("copy the project into the working copy: %w", err)
	}
	root, err := s.openRoot(row, "")
	if err != nil {
		return nil, err
	}
	defer root.Close()
	snap, err := snapshotTree(root)
	if err != nil {
		return nil, err
	}
	changes, err := s.userChangesSinceJob(ctx, row.ID, snap)
	if err != nil {
		return nil, err
	}
	if _, err := s.createRevision(ctx, row.ID, root, job.Prompt, RevisionBeforeJob, job.ID, ""); err != nil {
		return nil, err
	}
	return changes, nil
}

// userChangesSinceJob compares the project with its newest "job" revision; nil
// when no job ever succeeded.
func (s *Service) userChangesSinceJob(ctx context.Context, projectID string, snap treeSnapshot) (*UserChanges, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM cc_revisions WHERE project_id=? AND reason=? ORDER BY created_at DESC, id DESC LIMIT 1`,
		projectID, RevisionJob).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r, err := s.loadRevision(ctx, id)
	if err != nil {
		return nil, err
	}
	files, err := s.revisionFiles(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &UserChanges{LayoutChanged: sha256Hex(snap.Layout) != r.LayoutSHA, Files: []UserFileChange{}}
	for p, data := range snap.Files {
		old, ok := files[p]
		switch {
		case !ok:
			out.Files = append(out.Files, UserFileChange{Path: p, Status: "added"})
		case old != sha256Hex(data):
			out.Files = append(out.Files, UserFileChange{Path: p, Status: "modified"})
		}
	}
	for p := range files {
		if _, ok := snap.Files[p]; !ok {
			out.Files = append(out.Files, UserFileChange{Path: p, Status: "deleted"})
		}
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	return out, nil
}

// abortStart marks a job that could not start as failed and frees its lock.
// The lock is released and announced under the project mutex (like
// setProjectLock), so lock events keep the order of the lock changes.
func (s *Service) abortStart(job Job, cause error) {
	s.logger.Warn("CYD Creator job could not start", "job_id", job.ID, "error", cause)
	job.Status, job.ErrorMessage, job.FinishedAt = JobFailed, "The job could not be prepared.", formatTime(s.now())
	unlock := s.lockProject(job.ProjectID)
	res, err := s.db.Exec(`UPDATE cc_projects SET locked_by_job='' WHERE id=? AND locked_by_job=?`, job.ProjectID, job.ID)
	if err == nil {
		if n, _ := res.RowsAffected(); n == 1 {
			s.Emit(job.ProjectID, EventLockChanged, job.ID, "", map[string]any{"locked_by_job": ""})
		}
	}
	unlock()
	_ = s.saveTerminalJob(job)
}

// launchRun creates the run context and starts the orchestrator. Callers hold
// s.jobs.mu, which orders the WaitGroup Add before Close's Wait.
func (s *Service) launchRun(run *jobRun) {
	run.segmentStart = time.Now()
	run.ctx, run.cancel = newJobContext(s.ctx, &s.wg, s.jobs.newTimer, run.budget, func() bool {
		run.mu.Lock()
		defer run.mu.Unlock()
		switch run.phase {
		case PhaseCoding, PhaseBuilding, PhaseRepairing:
		default:
			return false
		}
		window := min(run.budget/3, 10*time.Minute)
		return !run.lastChange.IsZero() && time.Since(run.lastChange) <= window
	}, func(grace time.Duration) {
		s.emitStep(run.projectID, run.jobID, StepInfo, fmt.Sprintf("Recent changes are still being built; the job gets %s more to finish.", formatBudget(grace)))
	})
	s.jobs.active = run
	s.wg.Add(1)
	go s.executeJob(run)
}

// ConfirmJob continues a job waiting in awaiting_confirmation: without a
// prompt it builds the accepted plan, with an edited prompt it plans again.
// The waiting time does not count against the job timeout.
func (s *Service) ConfirmJob(ctx context.Context, jobID, prompt string) (Job, error) {
	prompt, err := validPrompt(prompt)
	if err != nil {
		return Job{}, err
	}
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	job, err := s.loadJob(ctx, jobID)
	if err != nil {
		return Job{}, s.internalError("confirm job", err)
	}
	if job.Status != JobAwaitingConfirmation {
		return Job{}, NewError(CodeJobState, "the job is not waiting for confirmation", map[string]any{"status": job.Status})
	}
	if s.jobs.runner == nil {
		return Job{}, NewError(CodeDisabled, "the CYD Creator agent is not available", nil)
	}
	if s.jobs.closing || s.ctx.Err() != nil {
		return Job{}, NewError(CodeBusy, "CYD Creator is shutting down", nil)
	}
	// A fresh row: a project fenced by an unresolved publication (or a failed
	// startup recovery) refuses the confirmation like any other write.
	row, err := s.loadProjectRow(ctx, job.ProjectID)
	if err != nil {
		return Job{}, s.internalError("confirm job", err)
	}
	if err := checkLock(row, JobOrigin(job.ID)); err != nil {
		return Job{}, err
	}
	stage := s.stagingPath(job.ID)
	if !dirExists(stage) {
		return Job{}, NewError(CodeJobState, "the job's working copy is missing; cancel the job and start again", nil)
	}
	startPhase := PhaseLayout
	if prompt != "" {
		job.Prompt, job.Plan, startPhase = prompt, nil, PhasePlanning
	} else if job.Plan == nil {
		return Job{}, NewError(CodeJobState, "the job has no plan to build", nil)
	}
	budget := s.Limits().JobTimeout
	if s.jobs.planningUsed == nil {
		s.jobs.planningUsed = map[string]time.Duration{}
	}
	if used := s.jobs.planningUsed[job.ID]; used > 0 {
		budget = max(budget-used, budget/4)
	}
	job.Status = JobRunning
	// Stored before the run starts: a lost write would leave the row waiting
	// for a confirmation that already happened.
	if err := s.saveJob(job); err != nil {
		return Job{}, s.internalError("confirm job", err)
	}
	if prompt != "" {
		if _, err := s.AddUserMessage(ctx, job.ProjectID, job.ID, prompt, nil); err != nil {
			s.logger.Warn("CYD Creator job message could not be stored", "job_id", job.ID, "error", err)
		}
	}
	run := &jobRun{jobID: job.ID, projectID: job.ProjectID, job: job, startPhase: startPhase, stage: stage,
		phase: job.Phase, budget: budget, plan: job.Plan}
	s.launchRun(run)
	return job, nil
}

// CancelJob stops a queued or running job (its working copy becomes the
// draft when it has changes) or ends a job waiting for confirmation. A stored
// terminal status is final, also while the finishing run still holds the
// agent slot (the status is written before the slot is released).
func (s *Service) CancelJob(ctx context.Context, jobID string) error {
	s.jobs.mu.Lock()
	job, err := s.loadJob(ctx, jobID)
	if err != nil {
		s.jobs.mu.Unlock()
		return s.internalError("cancel job", err)
	}
	if !isActiveJobStatus(job.Status) {
		s.jobs.mu.Unlock()
		return NewError(CodeJobState, "the job has already finished", map[string]any{"status": job.Status})
	}
	if run := s.jobs.active; run != nil && run.jobID == jobID {
		run.cancel(errJobCancelled)
		run.mu.Lock()
		buildID := run.buildID
		run.mu.Unlock()
		s.jobs.mu.Unlock()
		if buildID != "" {
			_ = s.CancelBuild(context.WithoutCancel(ctx), buildID)
		}
		return nil
	}
	// Waiting for confirmation (or left over without a run): finish it while
	// holding jobs.mu, so a concurrent ConfirmJob cannot start a run of a job
	// that is being cancelled.
	delete(s.jobs.planningUsed, jobID)
	defer s.jobs.mu.Unlock()
	return s.internalError("cancel job", s.finishJob(job, jobEnding{status: JobCancelled}))
}

// DiscardDraft deletes the project's saved draft (working copy and private
// agent conversation). A project without a draft is left unchanged.
func (s *Service) DiscardDraft(ctx context.Context, projectID string) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	unlock := s.lockProject(projectID)
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		unlock()
		return s.internalError("discard draft", err)
	}
	if row.DraftJob == "" {
		unlock()
		return nil
	}
	// The job's staging_dir goes too: a working copy whose removal fails (an
	// open file on Windows) must not be settled as the draft again at the next
	// start (unsettledInterruptedJobs).
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		unlock()
		return s.internalError("discard draft", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE cc_projects SET draft_job='' WHERE id=?`, projectID); err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE cc_jobs SET staging_dir='' WHERE id=?`, row.DraftJob)
	}
	if err == nil {
		err = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	if err != nil {
		unlock()
		return s.internalError("discard draft", err)
	}
	unlock()
	s.removeStaging(row.DraftJob)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM cc_agent_context WHERE job_id=?`, row.DraftJob)
	s.Emit(projectID, EventProjectChanged, "", "", projectChange(row.Version, OriginUser, "meta"))
	return nil
}

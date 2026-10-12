package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// runOutcome is how one orchestrator segment ended: waiting for the user's
// confirmation, published (err == nil) or failed/cancelled (err != nil).
type runOutcome struct {
	awaiting bool
	err      error
}

// executeJob runs the phases of a job segment and finishes it.
func (s *Service) executeJob(run *jobRun) {
	defer s.wg.Done()
	defer run.cancel(nil)
	outcome := s.driveJob(run)
	if outcome.awaiting {
		s.awaitConfirmation(run)
		return
	}
	if outcome.err != nil {
		_ = s.finishJob(run.job, s.classifyJobError(run, outcome.err))
	}
	s.jobs.mu.Lock()
	delete(s.jobs.planningUsed, run.jobID)
	if s.jobs.active == run {
		s.jobs.active = nil
	}
	s.jobs.mu.Unlock()
}

// awaitConfirmation ends a planning segment with an accepted plan. It runs
// under jobs.mu, which ConfirmJob and CancelJob take too: they see either
// the planning run or the stored waiting job with its plan message and the
// released slot, never a mix. A cancel or timeout that arrived after the
// round ended still wins.
func (s *Service) awaitConfirmation(run *jobRun) {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	if s.jobs.active == run {
		s.jobs.active = nil
	}
	if run.ctx.Err() != nil {
		delete(s.jobs.planningUsed, run.jobID)
		_ = s.finishJob(run.job, s.classifyJobError(run, context.Cause(run.ctx)))
		return
	}
	run.job.Status = JobAwaitingConfirmation
	if err := s.saveJob(run.job); err != nil {
		delete(s.jobs.planningUsed, run.jobID)
		_ = s.finishJob(run.job, jobEnding{status: JobFailed, message: "The plan could not be stored.", msg: "plan_not_stored"})
		return
	}
	if s.jobs.planningUsed == nil {
		s.jobs.planningUsed = map[string]time.Duration{}
	}
	s.jobs.planningUsed[run.jobID] += time.Since(run.segmentStart)
	s.postMessage(run.projectID, run.jobID, MessageRoleAgent, run.job.Plan.Summary)
}

// setJobPhase moves the job to a phase and announces it.
func (s *Service) setJobPhase(run *jobRun, phase string) {
	run.setPhase(phase)
	run.job.Phase = phase
	if run.job.Status != JobRunning {
		run.job.Status = JobRunning
		if run.job.StartedAt == "" {
			run.job.StartedAt = formatTime(s.now())
		}
	}
	_ = s.saveJob(run.job) // logged; the terminal state is checked (finishJob, publishJob)
}

// driveJob is the phase state machine: planning → (awaiting_confirmation) →
// layout → coding → building → (repairing → building)* → done.
func (s *Service) driveJob(run *jobRun) runOutcome {
	phase := run.startPhase
	limits := s.Limits()
	for {
		if err := run.ctx.Err(); err != nil {
			return runOutcome{err: context.Cause(run.ctx)}
		}
		switch phase {
		case PhasePlanning:
			run.mu.Lock()
			run.planAccepted, run.plan = false, nil
			run.mu.Unlock()
			s.setJobPhase(run, PhasePlanning)
			if err := s.runAgent(run, PhasePlanning); err != nil {
				return runOutcome{err: err}
			}
			run.mu.Lock()
			plan, accepted := run.plan, run.planAccepted
			run.mu.Unlock()
			if !accepted || plan == nil {
				return runOutcome{err: agentStopped{errors.New("the agent ended planning without submitting a plan")}}
			}
			run.job.Plan = plan // stored with the waiting status by awaitConfirmation
			return runOutcome{awaiting: true}
		case PhaseLayout:
			s.setJobPhase(run, PhaseLayout)
			if err := s.runAgent(run, PhaseLayout); err != nil {
				return runOutcome{err: err}
			}
			phase = PhaseCoding
		case PhaseCoding, PhaseRepairing:
			s.setJobPhase(run, phase)
			if phase == PhaseRepairing {
				s.emitStep(run.projectID, run.jobID, StepRepair, "repair_started", map[string]any{"attempt": run.job.RepairAttempt, "max": limits.MaxRepairAttempts})
			}
			if err := s.runAgent(run, phase); err != nil {
				return runOutcome{err: err}
			}
			phase = PhaseBuilding
		case PhaseBuilding:
			s.setJobPhase(run, PhaseBuilding)
			b, err := s.ensureJobBuild(run)
			if err != nil {
				return runOutcome{err: err}
			}
			run.job.BuildID = b.ID
			if b.Status == BuildSucceeded {
				return runOutcome{err: s.publishJob(run, b)}
			}
			diags := agentDiagnostics(b.Diagnostics)
			if !repairableBuild(b, diags) {
				return runOutcome{err: buildFailure(b, "The build failed with a problem the agent cannot fix.")}
			}
			if run.job.RepairAttempt >= limits.MaxRepairAttempts {
				return runOutcome{err: buildFailure(b, fmt.Sprintf("The build still fails after %d repair attempts.", run.job.RepairAttempt))}
			}
			run.job.RepairAttempt++
			run.mu.Lock()
			run.diagnostics = diags
			run.mu.Unlock()
			phase = PhaseRepairing
		default:
			return runOutcome{err: fmt.Errorf("unknown job phase %q", phase)}
		}
	}
}

// runAgent hands one phase to the Runner.
func (s *Service) runAgent(run *jobRun, phase string) error {
	s.jobs.mu.Lock()
	runner := s.jobs.runner
	s.jobs.mu.Unlock()
	if runner == nil {
		return NewError(CodeDisabled, "the CYD Creator agent is not available", nil)
	}
	summary, err := s.GetProject(run.ctx, run.projectID)
	if err != nil {
		return err
	}
	run.mu.Lock()
	diags := append([]Diagnostic(nil), run.diagnostics...)
	lastCode := ""
	if run.lastBuild != nil {
		lastCode = run.lastBuild.ErrorCode
	}
	run.mu.Unlock()
	in := JobRun{Job: run.job, Project: summary, Phase: phase, Prompt: run.job.Prompt, Attachments: run.attachments,
		Plan: run.job.Plan, RepairAttempt: run.job.RepairAttempt, MaxRepairAttempts: s.Limits().MaxRepairAttempts,
		Model: s.Limits().Model, UserChanges: run.userChanges, Resumed: run.resumed, Hooks: []LogicHook{}}
	if run.budget > 0 && !run.segmentStart.IsZero() {
		in.TimeRemaining = max(0, run.budget-time.Since(run.segmentStart))
	}
	if phase == PhaseRepairing {
		in.Diagnostics, in.BuildErrorCode = diags, lastCode
	}
	if root, err := os.OpenRoot(run.stage); err == nil {
		if p, err := readLayout(root); err == nil {
			in.Hooks = logicHooks(p)
		}
		_ = root.Close()
	}
	err = runner.RunCYDJob(WithJobContext(run.ctx, run.jobID), in)
	if run.ctx.Err() != nil {
		return context.Cause(run.ctx)
	}
	if err != nil {
		return agentStopped{err}
	}
	return nil
}

// agentStopped marks an error of the agent run itself (the Runner): its text
// is the agent's reason and goes to the chat ("The agent stopped: ...").
// Other untyped orchestrator errors stay in the server log (CYD_INTERNAL).
type agentStopped struct{ err error }

func (e agentStopped) Error() string { return e.err.Error() }
func (e agentStopped) Unwrap() error { return e.err }

// logicHooks lists every `logic` event binding of named widgets.
func logicHooks(p Project) []LogicHook {
	hooks := []LogicHook{}
	p.WalkWidgets(func(screen *Screen, _ *Widget, w *Widget) bool {
		if w.Name == "" {
			return true
		}
		for _, ev := range w.Events {
			if ev.Action.Type == "logic" {
				hooks = append(hooks, LogicHook{Hook: "on_" + w.Name + "_" + ev.On, Widget: w.Name, Event: ev.On,
					Screen: screen.Name, Hint: ev.Action.Hint})
			}
		}
		return true
	})
	return hooks
}

// agentDiagnostics drops findings in generated code: those are generator bugs
// (CYD_GENERATOR_ERROR) and never go to the agent.
func agentDiagnostics(list []Diagnostic) []Diagnostic {
	out := []Diagnostic{}
	for _, d := range list {
		if !d.Generated {
			out = append(out, d)
		}
	}
	return out
}

// repairableBuild reports a failure the agent can fix: compile errors in app
// code or an exceeded flash budget. classifyBuild demotes the errors of
// tooling failures (cyd-build exit 2, unknown exit codes, a failed Builder)
// to notes, so only real compiler/linker error diagnostics count here.
func repairableBuild(b Build, diags []Diagnostic) bool {
	switch b.ErrorCode {
	case CodeFlashBudgetExceeded:
		return true
	case CodeBuildFailed:
		for _, d := range diags {
			if d.Severity == SeverityError {
				return true
			}
		}
	}
	return false
}

func buildFailure(b Build, message string) error {
	code := b.ErrorCode
	if code == "" {
		code = CodeBuildFailed
	}
	return NewError(code, message, map[string]any{"build_id": b.ID})
}

// ensureJobBuild returns a finished build of the current working copy: the
// agent's last build when nothing changed since, else a new one.
func (s *Service) ensureJobBuild(run *jobRun) (Build, error) {
	fp, err := treeFingerprint(run.stage)
	if err != nil {
		return Build{}, err
	}
	run.mu.Lock()
	last, lastFP := run.lastBuild, run.lastBuildFP
	run.mu.Unlock()
	if last != nil && lastFP == fp {
		return *last, nil
	}
	return s.jobBuild(run.ctx, run)
}

// jobBuild builds the working copy as an agent build and waits for it. A
// cancelled or expired job also cancels the build.
func (s *Service) jobBuild(ctx context.Context, run *jobRun) (Build, error) {
	fp, err := treeFingerprint(run.stage)
	if err != nil {
		return Build{}, err
	}
	b, err := s.StartBuild(ctx, run.projectID, BuildOptions{Origin: JobOrigin(run.jobID), Dir: run.stage})
	if err != nil {
		return Build{}, err
	}
	run.mu.Lock()
	run.buildID = b.ID
	run.builds++
	n := run.builds
	run.mu.Unlock()
	final, err := s.WaitBuild(ctx, b.ID)
	run.mu.Lock()
	run.buildID = ""
	run.mu.Unlock()
	if err != nil {
		if ctx.Err() != nil {
			_ = s.CancelBuild(context.WithoutCancel(ctx), b.ID)
		}
		return Build{}, err
	}
	run.mu.Lock()
	run.lastBuild, run.lastBuildFP = &final, fp
	run.mu.Unlock()
	s.emitBuildStep(run, n, final)
	return final, nil
}

// emitBuildStep is the job_step of the job's n-th build: success (with the
// flash share when known), else the first error outside generated files
// (with its place when it has a file), else the build's error code.
func (s *Service) emitBuildStep(run *jobRun, n int, b Build) {
	if b.Status == BuildSucceeded {
		if b.Usage.FlashTotal > 0 {
			s.emitStep(run.projectID, run.jobID, StepBuild, "build_succeeded_flash", map[string]any{"n": n, "percent": b.Usage.FlashUsed * 100 / b.Usage.FlashTotal})
			return
		}
		s.emitStep(run.projectID, run.jobID, StepBuild, "build_succeeded", map[string]any{"n": n})
		return
	}
	for _, d := range b.Diagnostics {
		if d.Severity != SeverityError || d.Generated {
			continue
		}
		if d.File == "" {
			s.emitStep(run.projectID, run.jobID, StepBuild, "build_failed_error", map[string]any{"n": n, "message": d.Message})
			return
		}
		location := d.File
		if d.Line > 0 {
			location = fmt.Sprintf("%s:%d", d.File, d.Line)
		}
		s.emitStep(run.projectID, run.jobID, StepBuild, "build_failed_at", map[string]any{"n": n, "location": location, "message": d.Message})
		return
	}
	s.emitStep(run.projectID, run.jobID, StepBuild, "build_failed", map[string]any{"n": n, "error_code": b.ErrorCode, "error": b.ErrorCode})
}

// publishRetryDelays are the pauses before a publication is tried again after
// a directory rename was refused because a project file is open (CYD_BUSY
// reason file_open: on Windows a file manager, editor or virus scanner may
// hold a handle for a moment). The working copy stays in place meanwhile.
var publishRetryDelays = []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second}

// isFileOpenBusy reports publishStage's retryable CYD_BUSY (reason file_open).
func isFileOpenBusy(err error) bool {
	var typed *Error
	return errors.As(err, &typed) && typed.Code == CodeBusy && typed.Details["reason"] == "file_open"
}

// publishJob makes the green working copy the project state (crash-safe
// swap), records the "job" revision and announces the result. A swap refused
// by an open file is retried after publishRetryDelays; a cancel or timeout
// during the wait ends the job (the working copy becomes the draft).
func (s *Service) publishJob(run *jobRun, b Build) error {
	for attempt := 0; ; attempt++ {
		err := s.publishOnce(run, b)
		if !isFileOpenBusy(err) || attempt >= len(publishRetryDelays) {
			return err
		}
		delay := publishRetryDelays[attempt]
		s.emitStep(run.projectID, run.jobID, StepInfo, "publish_retry", map[string]any{"milliseconds": delay.Milliseconds()})
		timer := time.NewTimer(delay)
		select {
		case <-run.ctx.Done():
			timer.Stop()
			return context.Cause(run.ctx)
		case <-timer.C:
		}
	}
}

// publishOnce is one publication attempt under the project mutex. The
// commit point is publishStage's transaction (version bump and
// publish_receipt); everything after it runs without the job's cancellation
// and announces the result even when the "job" revision cannot be stored.
func (s *Service) publishOnce(run *jobRun, b Build) error {
	if err := run.ctx.Err(); err != nil {
		return context.Cause(run.ctx)
	}
	ctx := context.WithoutCancel(run.ctx)
	unlock := s.lockProject(run.projectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, run.projectID)
	if err != nil {
		return err
	}
	stageRoot, err := os.OpenRoot(run.stage)
	if err != nil {
		return err
	}
	p, err := readLayout(stageRoot)
	_ = stageRoot.Close()
	if err != nil {
		return err
	}
	// Publishing writes the layout: the asset entries follow the layout save
	// rule against the project's stored list (a damaged stored layout lists
	// none, like PutLayout). Every handle is closed before the swap.
	var storedAssets []Asset
	if root, err := s.openRoot(row, ""); err == nil {
		if stored, err := readLayout(root); err == nil {
			storedAssets = stored.Assets
		}
		_ = root.Close()
	}
	if err := checkLayoutAssets(storedAssets, p.Assets); err != nil {
		return err
	}
	name, board, theme := layoutRowMeta(row, p)
	job := run.job
	job.Status, job.Phase, job.BuildID, job.FinishedAt = JobSucceeded, PhaseDone, b.ID, formatTime(s.now())
	version, err := s.publishStage(ctx, row, run.stage, receiptPublish, receiptPublish+"_"+run.jobID, func(tx *sql.Tx, version int64) error {
		if _, err := tx.Exec(`UPDATE cc_projects SET name=?, board=?, theme=?, locked_by_job='', draft_job='', app_changed_since_job=0
			WHERE id=?`, name, board, theme, row.ID); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE cc_jobs SET status=?, phase=?, plan=?, repair_attempt=?, build_id=?, started_at=?, finished_at=?,
			error_code='', error_message='' WHERE id=?`, job.Status, job.Phase, planJSON(job.Plan), job.RepairAttempt, job.BuildID,
			job.StartedAt, job.FinishedAt, job.ID)
		return err
	})
	if err != nil {
		return err
	}
	run.job = job
	if root, err := s.openRoot(row, ""); err == nil {
		if _, err := s.createRevision(ctx, row.ID, root, job.Prompt, RevisionJob, job.ID, b.ID); err != nil {
			s.logger.Warn("CYD Creator job revision could not be stored", "job_id", job.ID, "error", err)
		}
		_ = root.Close()
	} else {
		s.logger.Warn("CYD Creator job revision could not be stored", "job_id", job.ID, "error", err)
	}
	if row.DraftJob != "" && row.DraftJob != job.ID {
		s.removeStaging(row.DraftJob)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM cc_agent_context WHERE project_id=?`, row.ID)
	origin := JobOrigin(job.ID)
	s.Emit(row.ID, EventProjectChanged, job.ID, "", projectChange(version, origin, "layout"))
	s.Emit(row.ID, EventProjectChanged, job.ID, "", projectChange(version, origin, "files"))
	s.Emit(row.ID, EventLockChanged, job.ID, "", map[string]any{"locked_by_job": ""})
	s.Emit(row.ID, EventJobStatus, job.ID, "", map[string]any{"job": job})
	run.mu.Lock()
	summary := strings.TrimSpace(run.summary)
	if summary == "" {
		summary = strings.TrimSpace(run.reply)
	}
	run.mu.Unlock()
	if summary == "" {
		summary = "The app was built successfully."
	}
	s.postMessage(row.ID, job.ID, MessageRoleAgent, summary)
	return nil
}

// jobEnding is how a job that did not publish ends: its status, the typed
// error's code and details.reason (empty for untyped endings), the English
// job error message and the code and vars of its system chat line (msg, a
// messageTexts key with a draft variant; finishJob posts msg or msg+"_draft").
type jobEnding struct {
	status, code, reason, message string
	msg                           string
	vars                          map[string]any
}

// jobErrorReasonPattern accepts the identifier-shaped details.reason values
// of service errors (publication_review, file_open, too_large, ...); free-text
// reasons such as ParseJobPlan's are not stored on the job.
var jobErrorReasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// classifyJobError maps the end of a failed segment to a jobEnding.
//
// When the run's context has ended, its cause decides: a step that was
// waiting (a build, a database read) returns the bare context error, which
// would otherwise read as a failure. Shutdown cancels the parent, so its
// cause is context.Canceled and the s.ctx check makes it "interrupted".
func (s *Service) classifyJobError(run *jobRun, err error) jobEnding {
	if run.ctx.Err() != nil {
		if cause := context.Cause(run.ctx); cause != nil {
			err = cause
		}
	}
	var typed *Error
	var agent agentStopped
	switch {
	case errors.Is(err, errJobCancelled):
		return jobEnding{status: JobCancelled, msg: "job_cancelled"}
	case errors.Is(err, errJobTimeout):
		message := fmt.Sprintf("The job exceeded its time limit of %s.", formatBudget(run.budget))
		if secs := textSeconds(run.budget); secs >= 60 {
			return jobEnding{status: JobFailed, message: message, msg: "job_timeout_minutes", vars: map[string]any{"minutes": secs / 60, "seconds": secs % 60}}
		}
		return jobEnding{status: JobFailed, message: message, msg: "job_timeout", vars: map[string]any{"seconds": textSeconds(run.budget)}}
	case s.ctx.Err() != nil:
		return jobEnding{status: JobInterrupted, message: "The server stopped while the job was running.", msg: "job_interrupted"}
	case errors.As(err, &typed):
		reason, _ := typed.Details["reason"].(string)
		if !jobErrorReasonPattern.MatchString(reason) {
			reason = ""
		}
		// The client shows its own sentence for the code (and reason); the
		// English message is the fallback.
		vars := map[string]any{"error_code": typed.Code, "error": typed.Message}
		if reason != "" {
			vars["error_reason"] = reason
		}
		return jobEnding{status: JobFailed, code: typed.Code, reason: reason, message: typed.Message, msg: "job_failed", vars: vars}
	case errors.As(err, &agent):
		s.logger.Warn("CYD Creator agent run failed", "job_id", run.jobID, "error", err)
		reason := err.Error()
		if s.scrub != nil {
			// Second safeguard: the runner already reports provider failures
			// generically; registered secrets never reach the chat.
			reason = s.scrub(reason)
		}
		message := "The agent stopped: " + strings.Join(strings.Fields(reason), " ")
		if utf8.RuneCountInString(message) > 300 {
			message = string([]rune(message)[:299]) + "…"
		}
		return jobEnding{status: JobFailed, message: message, msg: "agent_stopped",
			vars: map[string]any{"reason": strings.TrimPrefix(message, "The agent stopped: ")}}
	}
	// An orchestrator failure (file system, database): the cause can name
	// host paths, so it stays in the server log.
	s.logger.Warn("CYD Creator job failed", "job_id", run.jobID, "error", err)
	return jobEnding{status: JobFailed, code: CodeInternal, message: "The job failed because of an internal error; the server log has the details.", msg: "job_failed_internal"}
}

// terminalSaveDelays are the pauses between further attempts to store a
// job's final state (saveJob itself retries once). A lost terminal write
// would leave the cc_jobs row active, and activeJob would refuse every
// StartJob until CancelJob or a restart repairs it.
var terminalSaveDelays = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, time.Second}

// saveTerminalJob stores a finished job, retrying with backoff. A row that
// no longer exists (CYD_JOB_NOT_FOUND: the project was deleted) blocks
// nothing and is not retried.
func (s *Service) saveTerminalJob(job Job) error {
	err := s.saveJob(job)
	for _, delay := range terminalSaveDelays {
		if err == nil || ErrorCode(err) == CodeJobNotFound {
			return err
		}
		time.Sleep(delay)
		err = s.saveJob(job)
	}
	if err != nil && ErrorCode(err) != CodeJobNotFound {
		s.logger.Error("CYD Creator could not store the end of a job; cancel the job or restart AuraGo to free the agent slot",
			"job_id", job.ID, "status", job.Status, "error", err)
	}
	return err
}

// finishJob ends a job that did not publish: the project stays unchanged, a
// working copy with changes becomes the project's draft (replacing an older
// one), the lock is released and the chat gets a status line. It returns the
// error of the terminal job write (logged).
func (s *Service) finishJob(job Job, end jobEnding) error {
	ctx := context.Background()
	stage := s.stagingPath(job.ID)
	unlock := s.lockProject(job.ProjectID)
	row, err := s.loadProjectRow(ctx, job.ProjectID)
	if err != nil {
		unlock()
		s.logger.Warn("CYD Creator job could not be finished", "job_id", job.ID, "error", err)
		job.Status, job.ErrorCode, job.ErrorReason, job.ErrorMessage, job.FinishedAt = end.status, end.code, end.reason, end.message, formatTime(s.now())
		if ErrorCode(err) == CodeNotFound {
			return s.saveTerminalJob(job) // the project is gone; the row must not stay active
		}
		return err
	}
	keep := false
	if dirExists(stage) {
		staged, err1 := treeFingerprint(stage)
		current, err2 := treeFingerprint(s.projectDir(row.Slug))
		keep = err1 != nil || err2 != nil || staged != current
	}
	draft := row.DraftJob
	if keep {
		draft = job.ID
	}
	job.Status, job.ErrorCode, job.ErrorReason, job.ErrorMessage, job.FinishedAt = end.status, end.code, end.reason, end.message, formatTime(s.now())
	released := row.LockedByJob == job.ID
	if _, err := s.db.Exec(`UPDATE cc_projects SET locked_by_job=CASE WHEN locked_by_job=? THEN '' ELSE locked_by_job END,
		draft_job=? WHERE id=?`, job.ID, draft, row.ID); err != nil {
		s.logger.Warn("CYD Creator project state could not be stored", "job_id", job.ID, "error", err)
		released = false
	}
	if released {
		// Under the project mutex like setProjectLock: lock events keep the
		// order of the lock changes.
		s.Emit(row.ID, EventLockChanged, job.ID, "", map[string]any{"locked_by_job": ""})
	}
	unlock()
	if keep && row.DraftJob != "" && row.DraftJob != job.ID {
		s.removeStaging(row.DraftJob)
		_, _ = s.db.Exec(`DELETE FROM cc_agent_context WHERE job_id=?`, row.DraftJob)
	}
	if !keep {
		s.removeStaging(job.ID)
		_, _ = s.db.Exec(`DELETE FROM cc_agent_context WHERE job_id=?`, job.ID)
	}
	saveErr := s.saveTerminalJob(job)
	code := end.msg
	if keep {
		code += draftCodeSuffix
	}
	s.postSystemMessage(row.ID, job.ID, code, end.vars)
	return saveErr
}

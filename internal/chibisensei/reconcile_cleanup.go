package chibisensei

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"strings"
)

const (
	cleanupStaged   = "staged"
	cleanupDataset  = "dataset"
	cleanupNotebook = "notebook"
	cleanDeleted    = "deleted"
	cleanAbsent     = "absent"
	cleanKept       = "kept"
	cleanFailed     = "failed"

	stepCleanStaged = "cleanup_staged" // key of the retry counter in Remote.Attempts
	maxStagedRounds = 5
)

func markCleanup(x *Run, res, outcome string) {
	if x.Remote.Cleanup == nil {
		x.Remote.Cleanup = map[string]string{}
	}
	x.Remote.Cleanup[res] = outcome
}

// cleaning (every run passes here): wait for this run's local Kaggle jobs, remove every staged r<run12>- file,
// delete dataset then notebook when the run's delete flag and kaggle.allow_delete allow it, record leftovers, end
// in Remote.EndState. It never waits for Kaggle: a refused or failed delete becomes a leftover.
func (o *Orchestrator) stepCleaning(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	for _, rec := range rem.Jobs {
		if rec.JobID == "" || !jobActive(rec.Status) || rec.Step == StepDeleteDataset || rec.Step == StepDeleteKernel {
			continue
		}
		// A failed read means the job may still be active: wait, never remove files under a running transfer.
		if job, err := o.drv.Job(ctx, rec.JobID); err != nil || jobActive(job.Status) {
			return stepResult{wait: o.iv.localJob}, nil
		}
	}
	if rem.Cleanup[cleanupStaged] == "" {
		return o.removeStaged(ctx, r)
	}
	for _, res := range []string{cleanupDataset, cleanupNotebook} {
		if rem.Cleanup[res] == "" {
			return o.cleanResource(ctx, r, res)
		}
	}
	return o.finishRun(ctx, r)
}

// removeStaged removes the run's staged project files (local files of the Kaggle integration). A failed removal
// is retried after the error wait; after maxStagedRounds failed rounds the files still there are named in a
// staged_left warning (names only) and cleaning goes on, so a run can always end.
func (o *Orchestrator) removeStaged(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	removed := map[string]bool{}
	var left []string
	if rem.KaggleProject != "" {
		var names []string
		for _, f := range rem.Staged {
			names = append(names, f.Name)
		}
		for _, out := range []string{OutStatus, OutManifest, OutMetrics, OutEvalSamples, OutModelGGUF, OutBaseGGUF, OutAdapter, OutAdapterConfig} {
			names = append(names, stagedOutputName(r.ID, out))
		}
		for _, n := range names {
			if err := o.drv.RemoveFile(rem.KaggleProject, n); err != nil {
				o.log.Warn("Chibi Sensei staged file not removed", "run", r.ID, "file", n, "error", err)
				left = append(left, n)
				continue
			}
			removed[n] = true
		}
	}
	markRemoved := func(x *Run) {
		for i := range x.Remote.Staged {
			if rem.KaggleProject == "" || removed[x.Remote.Staged[i].Name] {
				x.Remote.Staged[i].Removed = true
			}
		}
	}
	if len(left) == 0 {
		_, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { markCleanup(x, cleanupStaged, "done"); markRemoved(x) })
		return stepResult{progressed: true}, err
	}
	round := rem.Attempts[stepCleanStaged] + 1
	if round < maxStagedRounds {
		_, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { markRemoved(x); setAttempt(x, stepCleanStaged, round) })
		return stepResult{wait: o.iv.errorWait}, err
	}
	ev := NewRunEvent("warn", map[string]any{"code": "staged_left", "message": strings.Join(left, ", ")})
	_, err := o.commit(ctx, r, ev, func(x *Run) { markRemoved(x); markCleanup(x, cleanupStaged, cleanFailed) })
	return stepResult{progressed: true}, err
}

func (o *Orchestrator) cleanResource(ctx context.Context, r Run, res string) (stepResult, error) {
	rem := r.Remote
	step, op, read := StepDeleteDataset, "delete_dataset", "get_dataset"
	params := map[string]any{"ownerSlug": rem.Owner, "datasetSlug": rem.DatasetSlug}
	exists := datasetMayExist(rem)
	if res == cleanupNotebook {
		step, op, read = StepDeleteKernel, "delete_kernel", "get_kernel"
		params = map[string]any{"userName": rem.Owner, "kernelSlug": rem.NotebookSlug}
		exists = notebookMayExist(rem)
	}
	if !exists {
		return o.cleanupDone(ctx, r, res, cleanAbsent, kaggleTurn{})
	}
	if !(o.cfg().AllowDelete && rem.DeleteRemoteAfterRun) {
		return o.cleanupDone(ctx, r, res, cleanKept, kaggleTurn{})
	}
	if rec := rem.Step(step); rec == nil || rec.JobID == "" {
		if _, err := o.readOp(ctx, read, params); err != nil {
			if ctx.Err() != nil {
				return stepResult{}, ctx.Err()
			}
			if IsKaggleNotFound(err) {
				return o.cleanupDone(ctx, r, res, cleanAbsent, kaggleTurn{})
			}
			return o.cleanupDone(ctx, r, res, cleanFailed, kaggleTurn{})
		}
	}
	t, err := o.kaggleStep(ctx, r, step, op, "", "", params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if t.blocked != nil {
		return o.cleanupDone(ctx, r, res, cleanFailed, kaggleTurn{})
	}
	if out, wait, err := o.pending(ctx, r, t); wait {
		return out, err
	}
	switch t.job.Status {
	case KaggleJobCompleted:
		return o.cleanupDone(ctx, r, res, cleanDeleted, t)
	case KaggleJobUnknown:
		_, err := o.readOp(ctx, read, params)
		if IsKaggleNotFound(err) {
			return o.cleanupDone(ctx, r, res, cleanDeleted, t)
		}
		if n := rem.Attempts[step] + 1; err == nil && n < maxStepAttempts {
			_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": n, "status": t.job.Status}),
				func(x *Run) { t.save(x); setAttempt(x, step, n) })
			return stepResult{progressed: true}, err
		}
	}
	return o.cleanupDone(ctx, r, res, cleanFailed, t)
}

// admittedStepMayExist reports whether the object a Kaggle job creates may exist: the job was admitted and did not
// end in a state that proves Kaggle refused it. A running, completed or unknown job counts as "may exist".
func admittedStepMayExist(rec *KaggleStep) bool {
	return rec != nil && rec.JobID != "" && rec.Status != KaggleJobFailed && rec.Status != KaggleJobCancelled
}

// notebookMayExist derives the notebook's existence from evidence: a version number, a status read that found the
// session, or a save_kernel job that Kaggle did not reject. A job that failed before Kaggle accepted it proves
// nothing exists.
func notebookMayExist(rem RunRemote) bool {
	if rem.KernelVersion > 0 || (rem.RemoteStatus != "" && rem.RemoteStatus != "NOT_FOUND") {
		return true
	}
	if rem.Untracked {
		return rem.RemoteStatus != "NOT_FOUND"
	}
	return admittedStepMayExist(rem.Step(StepSaveKernel))
}

// datasetMayExist: create_dataset was admitted and not rejected (completed or unknown).
func datasetMayExist(rem RunRemote) bool { return admittedStepMayExist(rem.Step(StepCreateDataset)) }

func (o *Orchestrator) cleanupDone(ctx context.Context, r Run, res, outcome string, t kaggleTurn) (stepResult, error) {
	ev := NewRunEvent("kaggle", map[string]any{"cleanup": res, "result": outcome})
	if outcome == cleanFailed {
		ev = NewRunEvent("warn", map[string]any{"code": "cleanup_failed", "resource": res})
	}
	_, err := o.commit(ctx, r, ev, func(x *Run) { t.save(x); markCleanup(x, res, outcome) })
	return stepResult{progressed: true}, err
}

func (o *Orchestrator) finishRun(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	left := []Leftover{}
	if c := rem.Cleanup[cleanupDataset]; c == cleanKept || c == cleanFailed {
		ref := KaggleRef(rem.Owner, rem.DatasetSlug)
		left = append(left, Leftover{Kind: "dataset", Ref: ref, URL: KaggleDatasetURL(ref)})
	}
	if c := rem.Cleanup[cleanupNotebook]; c == cleanKept || c == cleanFailed {
		ref := KaggleRef(rem.Owner, rem.NotebookSlug)
		left = append(left, Leftover{Kind: "notebook", Ref: ref, URL: KaggleNotebookURL(ref)})
	}
	if err := o.registerLogTail(ctx, r); err != nil {
		return o.localError(ctx, r, err)
	}
	end := rem.EndState
	if end != RunSucceeded && end != RunCancelled {
		end = RunFailed
	}
	now := ts(o.now())
	return o.transition(ctx, r, end, map[string]any{"error_code": r.ErrorCode, "leftovers": len(left)}, func(x *Run) {
		x.Leftovers, x.Remote.Finished = left, now
	})
}

func (o *Orchestrator) registerLogTail(ctx context.Context, r Run) error {
	rel := runRel(r.ID, outSubdir, logTailFileName)
	b, err := readChibiFile(o.dataDir, rel, 2*logTailBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	return o.st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: r.ProjectID, Kind: "log_tail", RelPath: rel,
		Bytes: int64(len(b)), SHA256: hex.EncodeToString(sum[:])})
}

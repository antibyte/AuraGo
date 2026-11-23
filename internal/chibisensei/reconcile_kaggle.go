package chibisensei

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/kaggle"
)

// kaggleTurn is the outcome of one call on a Kaggle mutation step.
type kaggleTurn struct {
	job     kaggle.Job
	ok      bool        // job is the receipt of the current attempt
	blocked error       // the driver refused the call; the run waits
	rec     *KaggleStep // receipt to store with the caller's commit; nil = unchanged
	extra   func(*Run)  // stored together with rec
}

func (t kaggleTurn) save(x *Run) {
	if t.rec != nil {
		x.Remote.PutStep(*t.rec)
		if t.extra != nil {
			t.extra(x)
		}
	}
}

func jobActive(status string) bool { return status == KaggleJobQueued || status == KaggleJobRunning }

func setAttempt(x *Run, step string, n int) {
	if x.Remote.Attempts == nil {
		x.Remote.Attempts = map[string]int{}
	}
	x.Remote.Attempts[step] = n
}

// kaggleStep advances the current attempt of a mutation step by one driver call:
//   - With a stored job ID it reads the job (Job); a status change comes back in rec.
//   - Otherwise it executes the deterministic request (a known ID returns the original receipt).
//   - A refused Execute marks the receipt refused and waits.
//   - A refused attempt whose replay is failed (the host gate admitted it as failed) never ran: the step moves to
//     attempt+1 and returns ok=false.
func (o *Orchestrator) kaggleStep(ctx context.Context, r Run, step, op, project, file string, params any) (kaggleTurn, error) {
	attempt := r.Remote.Attempts[step]
	id := KaggleRequestID(r.ID, step, attempt)
	rec := r.Remote.Step(step)
	now := ts(o.now())
	if rec != nil && rec.RequestID == id && rec.JobID != "" {
		job, err := o.drv.Job(ctx, rec.JobID)
		if err != nil {
			if ctx.Err() != nil {
				return kaggleTurn{}, ctx.Err()
			}
			return kaggleTurn{blocked: err}, nil
		}
		t := kaggleTurn{job: job, ok: true}
		if job.Status != rec.Status || job.Error != rec.Error {
			next := *rec
			next.Status, next.Error, next.Updated = job.Status, job.Error, now
			t.rec = &next
		}
		return t, nil
	}
	req, err := NewKaggleRequest(id, op, project, file, params)
	if err != nil {
		return kaggleTurn{}, err
	}
	res, err := o.drv.Execute(ctx, req)
	if err != nil {
		if ctx.Err() != nil {
			return kaggleTurn{}, ctx.Err()
		}
		if rec == nil || rec.RequestID != id || !rec.Refused {
			mark := KaggleStep{Step: step, Attempt: attempt, RequestID: id, File: file, Refused: true, Updated: now}
			if _, uerr := o.st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error {
				if x.State != r.State {
					return errRunMoved
				}
				x.Remote.PutStep(mark)
				return nil
			}); uerr != nil {
				return kaggleTurn{}, uerr
			}
		}
		return kaggleTurn{blocked: err}, nil
	}
	if res.Job == nil {
		return kaggleTurn{}, fmt.Errorf("kaggle %s: no job receipt", op)
	}
	job := *res.Job
	if rec != nil && rec.RequestID == id && rec.Refused && job.Status == KaggleJobFailed {
		_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": attempt + 1, "reason": "refused"}),
			func(x *Run) { setAttempt(x, step, attempt+1) })
		return kaggleTurn{}, err
	}
	return kaggleTurn{job: job, ok: true, rec: &KaggleStep{Step: step, Attempt: attempt, RequestID: id, JobID: job.ID,
		Status: job.Status, File: file, Error: job.Error, Updated: now}}, nil
}

// pending returns (result, true) when the step must stop here: refused, moved to a new attempt, or the job is still
// queued or running locally (a new receipt is stored first).
func (o *Orchestrator) pending(ctx context.Context, r Run, t kaggleTurn) (stepResult, bool, error) {
	switch {
	case t.blocked != nil:
		res, err := o.blocked(ctx, r, t.blocked)
		return res, true, err
	case !t.ok:
		return stepResult{progressed: true}, true, nil
	case jobActive(t.job.Status) && t.rec != nil:
		_, err := o.commit(ctx, r, kaggleEvent(*t.rec), t.save)
		return stepResult{progressed: true}, true, err
	case jobActive(t.job.Status):
		return stepResult{wait: o.iv.localJob}, true, nil
	}
	return stepResult{}, false, nil
}

// nextAttempt stores the receipt and moves the step to attempt+1; the third failed attempt fails the run with code.
func (o *Orchestrator) nextAttempt(ctx context.Context, r Run, t kaggleTurn, step, code, msg string) (stepResult, error) {
	n := r.Remote.Attempts[step] + 1
	if n >= maxStepAttempts {
		return o.fail(ctx, r, code, msg, t.save)
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": n, "status": t.job.Status}),
		func(x *Run) { t.save(x); setAttempt(x, step, n) })
	return stepResult{progressed: true}, err
}

// readOp runs a read operation (no receipt, no request ID).
func (o *Orchestrator) readOp(ctx context.Context, op string, params any) (json.RawMessage, error) {
	req, err := NewKaggleRequest("", op, "", "", params)
	if err != nil {
		return nil, err
	}
	res, err := o.drv.Execute(ctx, req)
	return res.Data, err
}

func kaggleEvent(rec KaggleStep) RunEvent {
	return NewRunEvent("kaggle", map[string]any{"step": rec.Step, "job_id": rec.JobID, "status": rec.Status, "attempt": rec.Attempt})
}

type jobResultFields struct {
	UploadRef     string `json:"upload_ref"`
	VersionNumber int    `json:"versionNumber"`
	KernelID      int64  `json:"kernelId"`
	RemoteStatus  string `json:"remote_status"`
}

func jobFields(j kaggle.Job) jobResultFields {
	var f jobResultFields
	_ = json.Unmarshal(j.Result, &f)
	return f
}

// remoteTerminalState mirrors kaggle's remoteState (remote_jobs.go): completed, failed, cancelled or "" while running.
func remoteTerminalState(v string) string {
	v = strings.ToUpper(v)
	switch {
	case v == "COMPLETE" || v == "COMPLETED" || strings.HasSuffix(v, "_SUCCEEDED") || strings.HasSuffix(v, "_COMPLETED"):
		return KaggleJobCompleted
	case v == "ERROR" || v == "FAILED" || strings.HasSuffix(v, "_FAILED") || strings.HasSuffix(v, "_ERRORED"):
		return KaggleJobFailed
	case v == "CANCELLED" || v == "CANCELED" || v == "CANCEL_ACKNOWLEDGED" || strings.HasSuffix(v, "_CANCELLED"):
		return KaggleJobCancelled
	}
	return ""
}

// statusText reads {"status": "..."} or a bare JSON string.
func statusText(data []byte) string {
	var s string
	if json.Unmarshal(data, &s) != nil {
		var m struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(data, &m)
		s = m.Status
	}
	return strings.TrimSpace(s)
}

func sessionStatus(data []byte) string { return strings.ToUpper(statusText(data)) }

// datasetReady classifies get_dataset_status. PROVISIONAL until the proof run records the real vocabulary.
func datasetReady(data []byte) (ready, failed bool) {
	switch strings.ToLower(statusText(data)) {
	case "ready", "complete", "completed":
		return true, false
	case "error", "failed", "deleted":
		return false, true
	}
	return false, false
}

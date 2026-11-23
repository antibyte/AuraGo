package chibisensei

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

const (
	maxLogEventsPerRead = 200
	// lateReadGrace is how long past the deadline a failing status read keeps the run waiting before it times out:
	// one refused or failed read says nothing about whether the session finished.
	lateReadGrace = 30 * time.Minute
)

type remoteView struct {
	status  string // upper-case Kaggle session status
	final   string // completed | failed | cancelled; "" while running
	polled  bool   // a read happened
	untrack bool   // the tracked job lost its outcome: switch to status reads
}

// readRemote reads the tracked save_kernel job, or (untracked) get_kernel_session_status at most every 10 s
// unless force.
func (o *Orchestrator) readRemote(ctx context.Context, r Run, force bool) (remoteView, error) {
	rem := r.Remote
	if !rem.Untracked {
		rec := rem.Step(StepSaveKernel)
		if rec == nil || rec.JobID == "" {
			return remoteView{untrack: true}, nil
		}
		job, err := o.drv.Job(ctx, rec.JobID)
		if err != nil {
			return remoteView{}, err
		}
		v := remoteView{status: jobFields(job).RemoteStatus, polled: true}
		switch job.Status {
		case KaggleJobCompleted, KaggleJobFailed, KaggleJobCancelled:
			v.final = job.Status
		case KaggleJobUnknown, KaggleJobResolvedUnknown:
			v.untrack = true
		}
		return v, nil
	}
	if last, err := time.Parse(time.RFC3339, rem.LastPolled); !force && err == nil && o.now().Sub(last) < o.iv.untrackedPoll {
		return remoteView{status: rem.RemoteStatus}, nil
	}
	data, err := o.readOp(ctx, "get_kernel_session_status", map[string]any{"userName": rem.Owner,
		"kernelSlug": rem.NotebookSlug, "versionLabel": strconv.Itoa(max(rem.KernelVersion, 1))})
	if IsKaggleNotFound(err) {
		return remoteView{status: "NOT_FOUND", final: KaggleJobFailed, polled: true}, nil
	}
	if err != nil {
		return remoteView{}, err
	}
	st := sessionStatus(data)
	return remoteView{status: st, final: remoteTerminalState(st), polled: true}, nil
}

// running: watch the remote, enforce the own deadline (remote start + session + 20 min; one final read decides) and
// read the live log every 60 s.
func (o *Orchestrator) stepRunning(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	now := o.now().UTC()
	deadline, _ := time.Parse(time.RFC3339, r.Deadline)
	late := !deadline.IsZero() && now.After(deadline)
	v, err := o.readRemote(ctx, r, late)
	switch {
	case err != nil && ctx.Err() != nil:
		return stepResult{}, ctx.Err()
	case err != nil && late && now.After(deadline.Add(lateReadGrace)):
		return o.timedOut(ctx, r)
	case err != nil:
		return o.blocked(ctx, r, err)
	case v.untrack:
		_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": StepSaveKernel, "untracked": true}),
			func(x *Run) { x.Remote.Untracked, x.Remote.KernelVersion = true, max(x.Remote.KernelVersion, 1) })
		return stepResult{progressed: true}, err
	case v.final != "":
		setStatus := func(x *Run) {
			if v.status != "" {
				x.Remote.RemoteStatus = v.status
			}
		}
		if r.CancelRequested {
			return o.endRun(ctx, r, RunCancelled, codeCancelled, "", setStatus)
		}
		return o.transition(ctx, r, RunDownloading, map[string]any{"remote": v.final, "remote_status": v.status}, setStatus)
	case late:
		return o.timedOut(ctx, r)
	}
	var muts []func(*Run)
	ev := RunEvent{}
	if v.polled && rem.Untracked {
		muts = append(muts, func(x *Run) { x.Remote.LastPolled = ts(now) })
	}
	if v.status != "" && v.status != rem.RemoteStatus {
		muts = append(muts, func(x *Run) { x.Remote.RemoteStatus = v.status })
		ev = NewRunEvent("kaggle", map[string]any{"remote_status": v.status})
	}
	var logged *logOutcome
	if last, perr := time.Parse(time.RFC3339, rem.LastLogRead); perr != nil || now.Sub(last) >= o.iv.logRead {
		out, err := o.readLog(ctx, r)
		if err != nil {
			return stepResult{}, err
		}
		if err := o.st.UpsertMetrics(ctx, r.ID, out.metrics); err != nil {
			return o.localError(ctx, r, err)
		}
		logged = &out
		muts = append(muts, out.apply)
	}
	if len(muts) == 0 && rem.BlockedSince == "" && rem.BlockedCode == "" {
		return stepResult{wait: o.iv.running}, nil
	}
	next, err := o.commit(ctx, r, ev, func(x *Run) {
		for _, m := range muts {
			m(x)
		}
	})
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if logged != nil {
		o.afterLog(ctx, next, *logged)
	}
	return stepResult{wait: o.iv.running}, nil
}

func (o *Orchestrator) timedOut(ctx context.Context, r Run) (stepResult, error) {
	if r.CancelRequested {
		return o.endRun(ctx, r, RunCancelled, codeCancelled, "", func(x *Run) { x.Remote.WarningCode = "remote_timeout" })
	}
	url := KaggleNotebookURL(KaggleRef(r.Remote.Owner, r.Remote.NotebookSlug))
	return o.fail(ctx, r, "remote_timeout", "The Kaggle session did not end in time. Stop it on Kaggle: "+url, nil)
}

type logOutcome struct {
	readAt  string
	events  []LogEvent // new events (seq > last stored)
	metrics []Metric
	stage   *RunStage
	parsed  bool // the body had at least one @@chibi line
	maxSeq  int64
}

// readLog reads the whole log (A1, A2), stores the scrubbed 64 KiB tail and parses the new @@chibi lines. Only a
// context error is returned; a refused or empty read counts as an empty read.
func (o *Orchestrator) readLog(ctx context.Context, r Run) (logOutcome, error) {
	rem := r.Remote
	out := logOutcome{readAt: ts(o.now()), maxSeq: rem.LastLogSeq}
	body, err := o.drv.ReadKernelLog(ctx, rem.Owner, rem.NotebookSlug, max(rem.KernelVersion, 1), logReadMaxBytes)
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if err != nil || len(body) == 0 {
		if err != nil {
			o.log.Info("Chibi Sensei kernel log not readable", "run", r.ID, "error", err)
		}
		return out, nil
	}
	if err := writeLogTail(o.dataDir, r.ID, body); err != nil {
		o.log.Warn("Chibi Sensei log tail not stored", "run", r.ID, "error", err)
	}
	evs := ParseKernelLog(body)
	out.parsed = len(evs) > 0
	var stage *RunStage
	if rem.Stage != nil {
		s := *rem.Stage
		stage = &s
	}
	for _, e := range evs {
		if e.Seq <= rem.LastLogSeq {
			continue
		}
		out.events = append(out.events, e)
		out.maxSeq = max(out.maxSeq, e.Seq)
		if s, ok := e.Stage(); ok {
			if stage == nil || stage.Name != s.Stage {
				stage = &RunStage{Name: s.Stage}
			}
			stage.State = s.State
		}
		if m, ok := e.Metric(); ok {
			out.metrics = append(out.metrics, m)
			if m.Kind == "train" {
				if stage == nil || stage.Name != "train" {
					stage = &RunStage{Name: "train", State: "start"}
				}
				stage.Step, stage.MaxSteps, stage.Epoch, stage.EtaS = m.Step, m.MaxSteps, m.Epoch, int(m.EtaS)
			}
		}
	}
	out.stage = stage
	return out, nil
}

// apply: live is yes after the first parsed line, no after three reads without one (unless already yes).
func (l logOutcome) apply(x *Run) {
	x.Remote.LastLogRead = l.readAt
	if l.parsed {
		x.Remote.Live, x.Remote.EmptyLogReads = LiveYes, 0
	} else {
		x.Remote.EmptyLogReads++
		if x.Remote.EmptyLogReads >= 3 && x.Remote.Live != LiveYes {
			x.Remote.Live = LiveNo
		}
	}
	if l.maxSeq > x.Remote.LastLogSeq {
		x.Remote.LastLogSeq = l.maxSeq
	}
	if l.stage != nil {
		s := *l.stage
		x.Remote.Stage = &s
	}
}

// afterLog writes the run events of the new lines (at most 200 per read) and publishes metric and stage events.
func (o *Orchestrator) afterLog(ctx context.Context, r Run, l logOutcome) {
	if o.hub != nil && len(l.metrics) > 0 {
		o.hub.Publish(r.ProjectID, Event{Name: "metric", Data: map[string]any{"run_id": r.ID, "points": l.metrics}})
	}
	n := 0
	for _, e := range l.events {
		ev, ok := logRunEvent(e)
		if !ok {
			continue
		}
		if n++; n > maxLogEventsPerRead {
			return
		}
		if err := o.st.AddRunEvent(ctx, r.ID, ev); err != nil {
			o.log.Warn("Chibi Sensei log event not stored", "run", r.ID, "error", err)
			return
		}
		if s, ok := e.Stage(); ok && o.hub != nil {
			o.hub.Publish(r.ProjectID, Event{Name: "stage", Data: map[string]any{"run_id": r.ID, "stage": s.Stage, "state": s.State}})
		}
	}
}

func logRunEvent(e LogEvent) (RunEvent, bool) {
	if s, ok := e.Stage(); ok {
		return NewRunEvent("stage", map[string]any{"stage": s.Stage, "state": s.State, "seq": e.Seq}), true
	}
	if x, ok := e.Error(); ok {
		return NewRunEvent("error", map[string]any{"stage": x.Stage, "code": x.Code, "message": scrubText(x.Message, 500), "seq": e.Seq}), true
	}
	if c, ok := e.ExitCode(); ok {
		return NewRunEvent("log", map[string]any{"exit_code": c}), true
	}
	if e.T == "warn" {
		var w struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(e.Raw, &w)
		if !codePattern.MatchString(w.Code) {
			w.Code = ""
		}
		return NewRunEvent("warn", map[string]any{"source": "trainer", "code": w.Code, "message": scrubText(w.Message, 500), "seq": e.Seq}), true
	}
	return RunEvent{}, false
}

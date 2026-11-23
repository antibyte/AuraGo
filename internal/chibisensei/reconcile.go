package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"aurago/internal/chibisensei/trainer"
	"aurago/internal/security"
)

// ErrDesktopReadOnly marks a driver refusal caused by the read-only desktop (the driver adapter wraps it).
var ErrDesktopReadOnly = errors.New("chibisensei: the desktop is read-only")

// errRunMoved aborts an update whose run changed state since the step loaded it.
var errRunMoved = errors.New("chibisensei: the run changed state")

var kaggleOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

const (
	codeCancelled      = "cancelled_by_user"
	msgDataChanged     = "The dataset changed after the start. Start the run again."
	msgUploadFailed    = "Kaggle did not accept the training data."
	msgDatasetFailed   = "Kaggle could not prepare the private dataset."
	msgDownloadFailed  = "A result file could not be downloaded from Kaggle."
	msgChecksum        = "A downloaded file does not match its checksum."
	msgManifestInvalid = "The result list of the run is invalid."
	msgStatusMissing   = "The run left no readable status file. The log tail is kept."

	maxStepAttempts   = 3 // attempts 0, 1, 2 of one Kaggle step
	maxLocalErrors    = 5
	datasetWaitMax    = 15 * time.Minute
	deadlineSlack     = 20 * time.Minute
	blockedLimit      = 6 * time.Hour
	maxImmediateSteps = 200
	logReadMaxBytes   = 16 << 20 // A2: the whole JSON array, cut to 64 KiB only for display
)

// OrchestratorConfig is read at every step.
type OrchestratorConfig struct {
	MachineShape         string
	DeleteRemoteAfterRun bool
	AllowDelete          bool
	RemoteTimeoutMinutes int
	AuraGoVersion        string
	MaxTransferBytes     int64 // kaggle.max_transfer_mb in bytes; 0 = no limit
	StorageMaxBytes      int64 // chibi_sensei.storage.max_gb in bytes; 0 = no limit
}

// SubmitInput is a confirmed start (the server checked plan token, preflight and consent).
type SubmitInput struct {
	RunID      string
	Project    Project
	Profile    *Profile
	ProfileSHA string
	Plan       Plan
	Owner      string
}

type intervals struct {
	localJob, running, dataset, logRead, untrackedPoll, blocked, idle, errorWait time.Duration
}

var defaultIntervals = intervals{localJob: 2 * time.Second, running: 10 * time.Second, dataset: 15 * time.Second,
	logRead: 60 * time.Second, untrackedPoll: 10 * time.Second, blocked: 30 * time.Second, idle: 60 * time.Second,
	errorWait: 15 * time.Second}

// stepResult: progressed = something was persisted and another step may continue at once; wait = the pause the
// state asks for otherwise.
type stepResult struct {
	progressed bool
	wait       time.Duration
}

// Orchestrator drives the training runs. One goroutine (Start) owns the active run (D13); Step is the unit of work.
type Orchestrator struct {
	st      *Store
	hub     *Hub
	cat     *Catalog
	drv     KaggleDriver
	dataDir string
	cfg     func() OrchestratorConfig
	now     func() time.Time
	iv      intervals
	log     *slog.Logger

	stepMu sync.Mutex
	wake   chan struct{}
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	closed bool
}

func NewOrchestrator(st *Store, hub *Hub, cat *Catalog, drv KaggleDriver, dataDir string, cfg func() OrchestratorConfig) *Orchestrator {
	return &Orchestrator{st: st, hub: hub, cat: cat, drv: drv, dataDir: dataDir, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() }, iv: defaultIntervals, log: slog.Default(),
		wake: make(chan struct{}, 1)}
}

// SetLogger sets the logger; call it before Start.
func (o *Orchestrator) SetLogger(l *slog.Logger) {
	if l != nil {
		o.log = l
	}
}

// Start runs the reconcile loop until ctx ends or Close; it resumes every non-terminal run. Ending the context
// never changes a run.
func (o *Orchestrator) Start(ctx context.Context) {
	o.mu.Lock()
	if o.cancel != nil || o.closed {
		o.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	o.cancel, o.done = cancel, make(chan struct{})
	o.mu.Unlock()
	go o.loop(ctx)
}

func (o *Orchestrator) loop(ctx context.Context) {
	defer close(o.done)
	immediate := 0
	for ctx.Err() == nil {
		wait := o.iv.idle
		runs, err := o.st.ActiveRuns(ctx)
		if err != nil {
			if ctx.Err() == nil {
				o.log.Warn("Chibi Sensei runs could not be loaded", "error", err)
			}
			wait = o.iv.errorWait
		} else if len(runs) > 0 {
			res, err := o.step(ctx, runs[0].ID)
			switch {
			case err != nil:
				if ctx.Err() != nil {
					return
				}
				o.log.Warn("Chibi Sensei run step failed", "run", runs[0].ID, "error", err)
				wait = max(res.wait, o.iv.errorWait)
			case res.progressed && immediate < maxImmediateSteps:
				immediate++
				continue
			case res.progressed:
				wait = o.iv.localJob
			default:
				wait = res.wait
			}
		}
		immediate = 0
		if wait <= 0 {
			wait = o.iv.localJob
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-o.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (o *Orchestrator) poke() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// Close stops the loop and waits for it (at most 15 s).
func (o *Orchestrator) Close() {
	o.mu.Lock()
	o.closed = true
	cancel, done := o.cancel, o.done
	o.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		o.log.Warn("Chibi Sensei orchestrator still busy at close")
	}
}

// Submit creates the run in state queued with everything later states need and wakes the loop. Snapshot, job and
// notebook are built in queued. It returns ErrRunActive while another run is not terminal.
func (o *Orchestrator) Submit(ctx context.Context, in SubmitInput) (Run, error) {
	p, pl := in.Profile, in.Plan
	switch {
	case !ValidID(in.RunID) || !ValidID(in.Project.ID):
		return Run{}, errors.New("submit run: invalid run or project id")
	case p == nil || pl.ProfileID != p.ID || !pl.Trainable:
		return Run{}, errors.New("submit run: the plan does not fit the profile")
	case !sha256Hex.MatchString(in.ProfileSHA):
		return Run{}, errors.New("submit run: invalid profile sha")
	case !kaggleOwnerPattern.MatchString(in.Owner):
		return Run{}, errors.New("submit run: invalid Kaggle owner")
	}
	cfg := o.cfg()
	plan := pl
	r := Run{ID: in.RunID, ProjectID: in.Project.ID, Backend: BackendKaggle, State: RunQueued, ProfileID: p.ID,
		ProfileSHA: in.ProfileSHA, Preset: pl.Preset, Hyper: pl.Hyper,
		Remote: RunRemote{ProfileStatus: p.Status, Method: pl.Method, Split: pl.Split, EstimateMinutes: pl.EstimateMinutes,
			SessionTimeoutMinutes: pl.SessionTimeoutMinutes, MaxTrainSeconds: pl.MaxTrainSeconds, DataRev: pl.DataRev,
			PlanHash:     PlanHash(pl, in.ProfileSHA, trainer.ScriptSHA256(), in.Owner, cfg.MachineShape, cfg.DeleteRemoteAfterRun),
			MachineShape: cfg.MachineShape, DeleteRemoteAfterRun: cfg.DeleteRemoteAfterRun, Owner: in.Owner,
			DatasetSlug: DatasetSlug(in.RunID), NotebookSlug: NotebookSlug(in.Project.ID, in.RunID),
			Marker: MarkerFileName(in.RunID), Live: LiveUnknown, Plan: &plan, Attempts: map[string]int{}}}
	if err := o.st.CreateRun(ctx, r); err != nil {
		return Run{}, err
	}
	got, err := o.st.GetRun(ctx, r.ID)
	if err != nil {
		return Run{}, err
	}
	o.publishRun(got)
	o.poke()
	return got, nil
}

// Cancel marks the run cancel_requested in one transaction and wakes the loop; it never calls Kaggle. The returned
// run shows cancel_requested and the cancel object at once. A terminal run is returned unchanged.
func (o *Orchestrator) Cancel(ctx context.Context, runID string) (Run, error) {
	r, err := o.st.GetRun(ctx, runID)
	if err != nil || RunTerminal(r.State) || r.CancelRequested {
		return r, err
	}
	next, err := o.st.UpdateRun(ctx, runID, NewRunEvent("state", map[string]any{"state": r.State, "cancel_requested": true}),
		func(x *Run) error {
			if RunTerminal(x.State) {
				return errRunMoved
			}
			x.CancelRequested = true
			return nil
		})
	if errors.Is(err, errRunMoved) {
		return o.st.GetRun(ctx, runID)
	}
	if err != nil {
		return Run{}, err
	}
	o.publishRun(next)
	o.poke()
	return next, nil
}

// Step performs one unit of work for the run: at most one Kaggle mutation and one state transition. It returns
// whether something was persisted that allows another step at once.
func (o *Orchestrator) Step(ctx context.Context, runID string) (bool, error) {
	res, err := o.step(ctx, runID)
	return res.progressed, err
}

func (o *Orchestrator) step(ctx context.Context, runID string) (stepResult, error) {
	o.stepMu.Lock()
	defer o.stepMu.Unlock()
	r, err := o.st.GetRun(ctx, runID)
	if err != nil {
		return stepResult{}, fmt.Errorf("load run %s: %w", runID, err)
	}
	if RunTerminal(r.State) {
		return stepResult{}, nil
	}
	var res stepResult
	handled := false
	if r.CancelRequested && r.State != RunCleaning {
		res, handled, err = o.handleCancel(ctx, r)
	}
	if !handled && err == nil {
		switch r.State {
		case RunQueued:
			res, err = o.stepQueued(ctx, r)
		case RunPackaging:
			res, err = o.stepPackaging(ctx, r)
		case RunUploading:
			res, err = o.stepUploading(ctx, r)
		case RunCreatingDataset:
			res, err = o.stepCreatingDataset(ctx, r)
		case RunWaitingDataset:
			res, err = o.stepWaitingDataset(ctx, r)
		case RunStarting:
			res, err = o.stepStarting(ctx, r)
		case RunRunning:
			res, err = o.stepRunning(ctx, r)
		case RunDownloading:
			res, err = o.stepDownloading(ctx, r)
		case RunVerifying:
			res, err = o.stepVerifying(ctx, r)
		case RunCleaning:
			res, err = o.stepCleaning(ctx, r)
		default:
			return stepResult{}, fmt.Errorf("run %s: unknown state %q", r.ID, r.State)
		}
	}
	if errors.Is(err, errRunMoved) {
		return stepResult{progressed: true}, nil
	}
	return res, err
}

// handleCancel: before submission (and after the remote ended) the run goes to cleaning as cancelled. After
// submission the remote cancel is tried once; the state handler then keeps tracking until the remote is terminal.
func (o *Orchestrator) handleCancel(ctx context.Context, r Run) (stepResult, bool, error) {
	switch {
	case !submitted(r) || r.State == RunDownloading || r.State == RunVerifying:
		res, err := o.endRun(ctx, r, RunCancelled, codeCancelled, "", nil)
		return res, true, err
	case r.Remote.CancelTried:
		return stepResult{}, false, nil
	}
	stopped, note := false, "No tracked Kaggle job; stop the notebook on Kaggle."
	if rec := r.Remote.Step(StepSaveKernel); rec != nil && rec.JobID != "" && !r.Remote.Untracked {
		err := o.drv.Cancel(ctx, rec.JobID)
		if err != nil && ctx.Err() != nil {
			return stepResult{}, true, ctx.Err()
		}
		stopped, note = err == nil, ""
		if err != nil {
			note = scrubText(err.Error(), 300)
		}
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": "cancel", "remote_stopped": stopped}),
		func(x *Run) {
			x.Remote.CancelTried, x.Remote.CancelRemoteStopped, x.Remote.CancelError = true, stopped, note
		})
	return stepResult{progressed: true}, true, err
}

// submitted reports whether save_kernel was admitted (or the run tracks the notebook by status reads).
func submitted(r Run) bool {
	if r.Remote.Untracked {
		return true
	}
	rec := r.Remote.Step(StepSaveKernel)
	return rec != nil && rec.JobID != ""
}

// commit applies mut in one UpdateRun while the run is still in r's state, appends ev (Kind "" = none), clears the
// blocked marker and the local error count, and publishes the run summary after the commit.
func (o *Orchestrator) commit(ctx context.Context, r Run, ev RunEvent, mut func(*Run)) (Run, error) {
	next, err := o.st.UpdateRun(ctx, r.ID, ev, func(x *Run) error {
		if x.State != r.State {
			return errRunMoved
		}
		mut(x)
		x.Remote.BlockedSince, x.Remote.BlockedCode, x.StepAttempt = "", "", 0
		return nil
	})
	if err != nil {
		return Run{}, err
	}
	o.publishRun(next)
	return next, nil
}

func (o *Orchestrator) transition(ctx context.Context, r Run, to string, data map[string]any, mut func(*Run)) (stepResult, error) {
	if data == nil {
		data = map[string]any{}
	}
	// "to" is the field the UI reads; "state" and "from" stay for the log.
	data["to"], data["state"], data["from"] = to, to, r.State
	_, err := o.commit(ctx, r, NewRunEvent("state", data), func(x *Run) {
		if mut != nil {
			mut(x)
		}
		x.State = to
		// LastState is where the run was working. A run that ends in failure or cancel passes through
		// cleaning too; it keeps the state it stopped in, so the UI can mark that phase.
		stopping := to == RunCleaning && x.Remote.EndState != "" && x.Remote.EndState != RunSucceeded
		if !RunTerminal(to) && !stopping {
			x.Remote.LastState = to
		}
	})
	return stepResult{progressed: true}, err
}

// endRun moves the run to cleaning; cleaning ends it in end (succeeded | failed | cancelled).
func (o *Orchestrator) endRun(ctx context.Context, r Run, end, code, msg string, extra func(*Run)) (stepResult, error) {
	data := map[string]any{"end_state": end}
	if code != "" {
		data["error_code"] = code
	}
	return o.transition(ctx, r, RunCleaning, data, func(x *Run) {
		if extra != nil {
			extra(x)
		}
		x.Remote.EndState = end
		if code != "" {
			x.ErrorCode, x.Error = code, msg
		}
	})
}

func (o *Orchestrator) fail(ctx context.Context, r Run, code, msg string, extra func(*Run)) (stepResult, error) {
	return o.endRun(ctx, r, RunFailed, code, msg, extra)
}

// blocked handles a refused driver call: the run waits. A new wait or a new code records one warn event. Before
// submission and while downloading, a wait longer than blockedLimit fails the run with the code.
func (o *Orchestrator) blocked(ctx context.Context, r Run, cause error) (stepResult, error) {
	if err := ctx.Err(); err != nil {
		return stepResult{}, err
	}
	code := "kaggle_unavailable"
	if errors.Is(cause, ErrDesktopReadOnly) {
		code = "desktop_readonly"
	}
	o.log.Warn("Chibi Sensei run waits for Kaggle", "run", r.ID, "state", r.State, "code", code, "error", cause)
	now := o.now()
	if since, err := time.Parse(time.RFC3339, r.Remote.BlockedSince); err == nil && now.Sub(since) > blockedLimit &&
		r.State != RunRunning && r.State != RunCleaning {
		return o.fail(ctx, r, code, "Kaggle stayed unavailable for six hours.", nil)
	}
	if r.Remote.BlockedCode == code {
		return stepResult{wait: o.iv.blocked}, nil
	}
	next, err := o.st.UpdateRun(ctx, r.ID, NewRunEvent("warn", map[string]any{"code": code, "state": r.State}),
		func(x *Run) error {
			if x.State != r.State {
				return errRunMoved
			}
			if x.Remote.BlockedSince == "" {
				x.Remote.BlockedSince = ts(now)
			}
			x.Remote.BlockedCode = code
			return nil
		})
	if err != nil {
		return stepResult{}, err
	}
	o.publishRun(next)
	return stepResult{wait: o.iv.blocked}, nil
}

// localError counts a local failure (store, files). The fifth in a row fails the run with internal_error; cleaning
// only waits. The cause is returned so the loop logs it.
func (o *Orchestrator) localError(ctx context.Context, r Run, cause error) (stepResult, error) {
	if ctx.Err() != nil || errors.Is(cause, errRunMoved) || r.State == RunCleaning {
		return stepResult{wait: o.iv.errorWait}, cause
	}
	if r.StepAttempt+1 >= maxLocalErrors {
		if _, err := o.fail(ctx, r, "internal_error", "AuraGo could not continue the run.", nil); err != nil {
			return stepResult{}, errors.Join(cause, err)
		}
		return stepResult{progressed: true}, cause
	}
	if _, err := o.st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error {
		if x.State != r.State {
			return errRunMoved
		}
		x.StepAttempt++
		return nil
	}); err != nil {
		return stepResult{}, errors.Join(cause, err)
	}
	return stepResult{wait: o.iv.errorWait}, cause
}

func (o *Orchestrator) publishRun(r Run) {
	if o.hub != nil {
		o.hub.Publish(r.ProjectID, Event{Name: "run", Data: r.Summary()})
	}
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func scrubText(s string, n int) string { return cutRunes(security.Scrub(s), n) }

package cydcreator

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Runner executes one agent phase of a job. The server implements it
// (internal/server/cyd_creator_runner.go) with an isolated agent loop whose
// tools reach the job through WithJobContext; tests use fakes. ctx carries
// the job binding, its timeout and its cancellation.
type Runner interface {
	RunCYDJob(ctx context.Context, run JobRun) error
}

// JobRun is the input of one agent phase.
type JobRun struct {
	Job               Job
	Project           ProjectSummary
	Phase             string       // planning | layout | coding | repairing
	Prompt            string       // the user request (edited on re-plan)
	Attachments       []Attachment // log lines sent with the request; untrusted
	Plan              *JobPlan     // accepted plan (layout and later phases of new jobs)
	Diagnostics       []Diagnostic // repairing: build findings without generated ones
	BuildErrorCode    string       // repairing: error code of the failed build
	RepairAttempt     int
	MaxRepairAttempts int
	Model             string // provider ID from cyd_creator.model, "" = main provider
	UserChanges       *UserChanges
	Hooks             []LogicHook // logic hooks of the layout (the coding task list)
	Resumed           bool        // the working copy continues a saved draft
}

// UserChanges lists what the user changed since the last successful job.
type UserChanges struct {
	LayoutChanged bool             `json:"layout_changed"`
	Files         []UserFileChange `json:"files"`
}

// UserFileChange is one file the user added, modified or deleted.
type UserFileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// LogicHook is a `logic` event binding the agent implements in src/app.
type LogicHook struct {
	Hook   string `json:"hook"` // C function name on_<widget>_<event>
	Widget string `json:"widget"`
	Event  string `json:"event"`
	Screen string `json:"screen"`
	Hint   string `json:"hint"`
}

var (
	errJobCancelled = errors.New("the job was cancelled")
	errJobTimeout   = errors.New("the job exceeded its time limit")
)

// jobRuntime is the job state of a Service (field Service.jobs).
//
// Lock order: jobs.mu comes first, before the project lock, buildsMu, eventsMu
// and subsMu (jobs.mu → project lock → buildsMu → eventsMu → subsMu).
// StartJob, ConfirmJob, CancelJob and DiscardDraft hold jobs.mu while they
// take the project lock or emit; nothing that holds one of the later locks
// may take jobs.mu. jobRun.mu is a leaf: never take another lock or emit
// while holding it.
type jobRuntime struct {
	mu           sync.Mutex
	runner       Runner
	active       *jobRun
	closing      bool
	planningUsed map[string]time.Duration
	newTimer     jobTimer // nil = realJobTimer; tests install fake timers
}

// jobTimer starts the job timeout's timer: the channel delivers once after d,
// stop releases the timer.
type jobTimer func(d time.Duration) (c <-chan time.Time, stop func() bool)

func realJobTimer(d time.Duration) (<-chan time.Time, func() bool) {
	t := time.NewTimer(d)
	return t.C, t.Stop
}

// formatBudget renders a time budget for job messages: whole seconds, or
// milliseconds below one second.
func formatBudget(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

// jobRun is the in-memory state of the job that holds the agent slot.
type jobRun struct {
	jobID, projectID string
	job              Job // owned by the orchestrator goroutine
	startPhase       string
	stage            string
	attachments      []Attachment
	userChanges      *UserChanges
	resumed          bool
	budget           time.Duration
	segmentStart     time.Time
	ctx              context.Context
	cancel           context.CancelCauseFunc

	mu           sync.Mutex
	phase        string
	plan         *JobPlan
	planAccepted bool
	finished     bool
	summary      string
	reply        string
	diagnostics  []Diagnostic
	buildID      string
	lastBuild    *Build
	lastBuildFP  string
	builds       int
	lastChange   time.Time
}

func (r *jobRun) setPhase(phase string) {
	r.mu.Lock()
	r.phase = phase
	if phase == PhaseCoding || phase == PhaseRepairing {
		r.finished = false
	}
	r.mu.Unlock()
}

func (r *jobRun) currentPhase() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.phase
}

func (r *jobRun) touch() {
	r.mu.Lock()
	r.lastChange = time.Now()
	r.mu.Unlock()
}

// SetRunner installs the agent runner; nil disables new jobs.
func (s *Service) SetRunner(r Runner) {
	s.jobs.mu.Lock()
	s.jobs.runner = r
	s.jobs.mu.Unlock()
}

// activeRun returns the job that holds the agent slot, or nil.
func (s *Service) activeRun() *jobRun {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	return s.jobs.active
}

// PlanAccepted reports whether the running job's plan was accepted in this
// planning round (the runner's RunComplete hook for planning).
func (s *Service) PlanAccepted(jobID string) bool {
	run := s.activeRun()
	if run == nil || run.jobID != jobID {
		return false
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	return run.planAccepted
}

// JobFinished reports whether the agent called cyd_creator_finish in the
// current coding or repair round (the runner's RunComplete hook).
func (s *Service) JobFinished(jobID string) bool {
	run := s.activeRun()
	if run == nil || run.jobID != jobID {
		return false
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	return run.finished
}

// NoteAgentReply keeps the agent's final prose of a coding or repair round; it
// becomes the chat summary when the agent did not call cyd_creator_finish.
func (s *Service) NoteAgentReply(jobID, text string) {
	run := s.activeRun()
	if run == nil || run.jobID != jobID {
		return
	}
	run.mu.Lock()
	if p := run.phase; p == PhaseCoding || p == PhaseRepairing {
		run.reply = text
	}
	run.mu.Unlock()
}

// jobTimeoutGrace is the one extra window a productive job may get when its
// budget ends (pattern internal/gamemaker/job_timeout.go): never sliding.
func jobTimeoutGrace(budget time.Duration) time.Duration {
	return min(budget/2, 15*time.Minute)
}

// newJobContext returns a context that ends with errJobTimeout after budget,
// or after one grace window when productive() holds at that moment. The
// timer goroutine is counted in wg (when not nil) and ends with the context.
// newTimer starts its timers (nil = realJobTimer).
func newJobContext(parent context.Context, wg *sync.WaitGroup, newTimer jobTimer, budget time.Duration, productive func() bool, extended func(time.Duration)) (context.Context, context.CancelCauseFunc) {
	if newTimer == nil {
		newTimer = realJobTimer
	}
	ctx, cancel := context.WithCancelCause(parent)
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		expired := func(d time.Duration) bool {
			c, stop := newTimer(d)
			defer stop()
			select {
			case <-ctx.Done():
				return false
			case <-c:
				return true
			}
		}
		if !expired(budget) {
			return
		}
		if productive() {
			grace := jobTimeoutGrace(budget)
			extended(grace)
			if !expired(grace) {
				return
			}
		}
		cancel(errJobTimeout)
	}()
	return ctx, cancel
}

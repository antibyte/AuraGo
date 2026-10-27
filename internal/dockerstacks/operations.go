package dockerstacks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"aurago/internal/dockerhost"
)

var errServiceClosed = errors.New("the stacks service is closed")

// Prepare starts the prepare operation of a host (stack ""): the runner image
// is pulled or copied and the IO container is created. Stack operations do the
// same implicitly as their first step; Prepare lets the UI do it up front.
func (s *Service) Prepare(ctx context.Context, hostID string) (Operation, error) {
	return s.begin(ctx, hostID, "", KindPrepare, OpOptions{})
}

// Start starts a stack operation. It returns at once with the operation in state
// queued (or running); follow it with Subscribe or Operation. A second operation
// on the same stack gets a *BusyError that names the running one. Options.Volumes
// only has a meaning for down and delete.
func (s *Service) Start(ctx context.Context, hostID, name, kind string, opts OpOptions) (Operation, error) {
	if !slices.Contains(StackOperationKinds, kind) {
		return Operation{}, newError(CodeInvalidOperation, "%q is not a stack operation", kind)
	}
	if err := ValidateName(name); err != nil {
		return Operation{}, err
	}
	if kind != KindDown && kind != KindDelete {
		opts.Volumes = false
	}
	return s.begin(ctx, hostID, name, kind, opts)
}

// begin records an operation, takes the lock of its stack and starts it.
func (s *Service) begin(ctx context.Context, hostID, name, kind string, opts OpOptions) (Operation, error) {
	if _, err := s.hosts.Host(ctx, hostID); err != nil {
		return Operation{}, err
	}
	id, err := newOperationID()
	if err != nil {
		return Operation{}, err
	}
	op := Operation{ID: id, HostID: hostID, Stack: name, Kind: kind, Options: opts, State: StateQueued, CreatedAt: s.opts.Now().UTC()}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Operation{}, errServiceClosed
	}
	key := lockKey{hostID, name}
	if other, busy := s.locks[key]; busy {
		busyKind := ""
		if run := s.runs[other]; run != nil {
			run.mu.Lock()
			busyKind = run.op.Kind
			run.mu.Unlock()
		}
		s.mu.Unlock()
		return Operation{}, &BusyError{OperationID: other, Kind: busyKind}
	}
	if err := s.journal.insert(ctx, op); err != nil {
		s.mu.Unlock()
		return Operation{}, err
	}
	run := newOpRun(s, op)
	s.runs[id] = run
	s.locks[key] = id
	s.wg.Add(1)
	s.mu.Unlock()

	go s.execute(run)
	return run.snapshot(), nil
}

// runContext bounds one operation: it ends with the service (Close) and after
// MaxDuration counted from the operation's start (an operation whose time ran
// out while AuraGo was down gets a context that is over already).
func (s *Service) runContext(op Operation) (context.Context, context.CancelFunc) {
	start := s.opts.Now()
	if op.StartedAt != nil {
		start = *op.StartedAt
	}
	remaining := max(s.opts.MaxDuration-s.opts.Now().Sub(start), 0)
	return context.WithTimeout(s.baseCtx, remaining)
}

// execute runs one operation to its end.
func (s *Service) execute(run *opRun) {
	defer s.wg.Done()
	out := s.drive(run)
	if out.abandoned {
		run.abandon()
		return
	}
	run.finish(out)
	if out.cleanup != nil {
		out.cleanup()
	}
}

func (s *Service) drive(run *opRun) outcome {
	op := run.snapshot()
	ctx, cancel := s.runContext(op)
	defer cancel()
	run.setState(StateRunning)
	h, e, err := s.target(ctx, op.HostID)
	if err != nil {
		return s.failure(ctx, err)
	}
	if op.Kind == KindPrepare {
		return s.drivePrepare(ctx, run, e, h)
	}
	return s.driveStack(ctx, run, op, e, h)
}

// timeoutOutcome is the outcome of an operation that ran out of time.
func (s *Service) timeoutOutcome() outcome {
	return outcome{state: StateFailed, code: opCodeTimeout,
		message: fmt.Sprintf("The operation took longer than %s and was stopped.", s.opts.MaxDuration)}
}

// failure turns an error into the outcome of a failed operation. A service that
// is closing abandons the operation instead; a passed deadline is a timeout.
func (s *Service) failure(ctx context.Context, err error) outcome {
	if s.baseCtx.Err() != nil {
		return outcome{abandoned: true}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return s.timeoutOutcome()
	}
	var coded *Error
	if errors.As(err, &coded) {
		return outcome{state: StateFailed, code: coded.Code, message: coded.Message}
	}
	if dockerhost.StatusOf(err) != 0 {
		return outcome{state: StateFailed, code: opCodeEngineError, message: engineMessage(err)}
	}
	return outcome{state: StateFailed, code: opCodeHostUnreachable,
		message: fmt.Sprintf("The Docker host could not be reached (%s): %s", dockerhost.ClassifyError(err), engineMessage(err))}
}

// drivePrepare is the prepare operation: make the host ready, then run the
// lister once, which proves that the stacks directory can be bound and read.
func (s *Service) drivePrepare(ctx context.Context, run *opRun, e engine, h dockerhost.Host) outcome {
	image, err := s.ensureHost(ctx, run, e, h, true)
	if err != nil {
		return s.failure(ctx, err)
	}
	run.line("Checking the stacks directory " + h.StacksDir)
	scan, err := s.scanFolders(ctx, e, h, image)
	if err != nil {
		return s.failure(ctx, err)
	}
	stacks := 0
	for _, f := range scan.folders {
		if f.composeFile != "" {
			stacks++
		}
	}
	run.line(fmt.Sprintf("Ready: %d stack folder(s) found in %s", stacks, h.StacksDir))
	return outcome{state: StateSucceeded}
}

// driveStack runs a stack operation: prepare the host if needed, find the
// compose file, run the kind's script in a runner container and follow it.
func (s *Service) driveStack(ctx context.Context, run *opRun, op Operation, e engine, h dockerhost.Host) outcome {
	image, err := s.ensureHost(ctx, run, e, h, false)
	if err != nil {
		return s.failure(ctx, err)
	}
	composeFile := ""
	if op.Kind == KindDelete {
		exists, err := e.pathExists(ctx, stackDir(h, op.Stack))
		if err != nil {
			return s.failure(ctx, err)
		}
		if !exists {
			return outcome{state: StateFailed, code: CodeStackNotFound, message: fmt.Sprintf("There is no stack %q on this host.", op.Stack)}
		}
	} else {
		if composeFile, err = s.locateCompose(ctx, e, h, op.Stack); err != nil {
			return s.failure(ctx, err)
		}
		if composeFile == "" {
			return outcome{state: StateFailed, code: CodeStackNotFound, message: fmt.Sprintf("There is no compose file for stack %q on this host.", op.Stack)}
		}
	}
	spec, err := runnerSpec(image, h, op, composeFile)
	if err != nil {
		return outcome{state: StateFailed, code: CodeInvalidOperation, message: err.Error()}
	}

	runnerID, err := e.createContainer(ctx, runnerName(op.ID), spec)
	if err != nil {
		return s.failure(ctx, err)
	}
	run.mutate(func(o *Operation) { o.RunnerID = runnerID })
	cleanup := s.runnerCleanup(e, runnerID)
	if err := e.startContainer(ctx, runnerID); err != nil {
		out := s.failure(ctx, err)
		out.cleanup = cleanup
		return out
	}
	st, err := s.followRunner(ctx, run, e, runnerID)
	if err != nil {
		out := s.failure(ctx, err)
		if out.code == opCodeTimeout {
			killCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			_ = e.killContainer(killCtx, runnerID)
			cancel()
		}
		out.cleanup = cleanup
		return out
	}
	return s.runnerOutcome(ctx, run, e, op, st, runnerID)
}

// runnerOutcome judges a finished runner by its exit code. After a successful up or
// update the health wait follows; the runner is removed once its result is
// recorded.
func (s *Service) runnerOutcome(ctx context.Context, run *opRun, e engine, op Operation, st containerState, runnerID string) outcome {
	exit := st.ExitCode
	if exit != 0 {
		message := run.errorMessage()
		switch {
		case message != "":
		case st.OOMKilled:
			message = "The runner ran out of memory and was killed."
		case st.Error != "":
			message = st.Error
		default:
			message = fmt.Sprintf("docker compose exited with code %d.", exit)
		}
		return outcome{state: StateFailed, code: opCodeRunnerFailed, message: message, exit: &exit,
			errorLine: errorLineOf(message), cleanup: s.runnerCleanup(e, runnerID)}
	}
	if op.Kind != KindUp && op.Kind != KindUpdate {
		return outcome{state: StateSucceeded, exit: &exit, cleanup: s.runnerCleanup(e, runnerID)}
	}
	run.mutate(func(o *Operation) { o.ExitCode = &exit })
	s.runnerCleanup(e, runnerID)()
	warning, ports, err := s.healthWait(ctx, run, e, op.Stack)
	if err != nil {
		out := s.failure(ctx, err)
		if out.code == opCodeTimeout || out.abandoned {
			return out
		}
		// The stack came up; only the check on it failed.
		return outcome{state: StateSucceeded, exit: &exit, warning: "The health of the services could not be checked: " + engineMessage(err)}
	}
	return outcome{state: StateSucceeded, exit: &exit, warning: warning, ports: ports}
}

// runnerCleanup removes a runner container, unless the service is closing (the
// next start sweeps leftovers).
func (s *Service) runnerCleanup(e engine, id string) func() {
	return func() {
		if s.isClosed() {
			return
		}
		s.removeQuietly(e, id)
	}
}

// followDelay is the pause before the log stream of a runner is picked up again.
const (
	followDelay      = 200 * time.Millisecond
	followErrorDelay = 2 * time.Second
	followMaxErrors  = 30
)

// replay hands the lines of a log that is read again from its start to the
// operation, skipping the ones it already has.
type replay struct {
	run       *opRun
	delivered int
	seen      int
}

func (r *replay) emit(line string) {
	r.seen++
	if r.seen > r.delivered {
		r.delivered++
		r.run.composeLine(line)
	}
}

// followRunner reads the runner's output into the operation until the container
// has exited and returns its final state. When the log stream ends while the
// container still runs (a dropped connection), the stream is opened again and
// the lines already seen are skipped. An Engine that stays unreachable for about
// a minute fails the follow.
func (s *Service) followRunner(ctx context.Context, run *opRun, e engine, id string) (containerState, error) {
	sink := &replay{run: run}
	failures := 0
	for {
		sink.seen = 0
		stdout, stderr := newLineWriter(sink.emit), newLineWriter(sink.emit)
		followErr := e.followLogs(ctx, id, 0, stdout, stderr)
		stdout.Flush()
		stderr.Flush()
		if err := ctx.Err(); err != nil {
			return containerState{}, err
		}
		st, err := e.inspectContainer(ctx, id)
		delay := followDelay
		switch {
		case err == nil && !st.Running && st.Status != "created" && st.Status != "restarting":
			return st, nil
		case err != nil:
			if isNotFound(err) {
				return containerState{}, err
			}
			if failures++; failures > followMaxErrors {
				return containerState{}, err
			}
			delay = followErrorDelay
		default:
			failures = 0
		}
		if followErr != nil {
			s.log.Debug("runner log stream ended", "container", id, "error", followErr)
		}
		select {
		case <-ctx.Done():
			return containerState{}, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// errorMessage is the line of the output that explains a failure so far.
func (r *opRun) errorMessage() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scan.message()
}

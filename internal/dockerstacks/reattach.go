package dockerstacks

import (
	"context"
	"errors"

	"aurago/internal/dockerhost"
)

const interruptedMessage = "AuraGo restarted while this operation was running and its runner container is gone. Check the state of the stack."

// resume takes up an operation that the previous process left unfinished and
// brings it to an end: the runner container is found by its name, its log is read
// again from the start (the log is the source of truth, so the output is rebuilt)
// and the exit code decides, exactly as for an operation that was never
// interrupted. A host that does not answer parks the operation until the next
// List, Get, Start or Operation call for that host.
func (s *Service) resume(run *opRun) {
	defer s.wg.Done()
	out, park := s.resumeOutcome(run)
	switch {
	case park:
		s.park(run)
	case out.abandoned:
		run.abandon()
	default:
		run.finish(out)
		if out.cleanup != nil {
			out.cleanup()
		}
	}
}

// park keeps an adopted operation registered, with its lock, until its host answers.
func (s *Service) park(run *opRun) {
	s.mu.Lock()
	closed := s.closed
	if !closed {
		s.parked[run.op.ID] = run
	}
	s.mu.Unlock()
	if closed {
		run.abandon()
	}
}

// kickReattach resumes the parked operations of a host. The callers have just
// reached the host, so it is worth another try.
func (s *Service) kickReattach(hostID string) {
	s.mu.Lock()
	if s.closed || len(s.parked) == 0 {
		s.mu.Unlock()
		return
	}
	var due []*opRun
	for id, run := range s.parked {
		if run.op.HostID == hostID {
			due = append(due, run)
			delete(s.parked, id)
		}
	}
	s.wg.Add(len(due))
	s.mu.Unlock()
	for _, run := range due {
		go s.resume(run)
	}
}

// lostRunner is the outcome of an operation whose runner container is gone. An
// operation that was waiting for health had finished its work; any other was cut off.
func lostRunner(op Operation) outcome {
	if op.State == StateHealthWait {
		return outcome{state: StateSucceeded}
	}
	return outcome{state: StateInterrupted, code: opCodeInterrupted, message: interruptedMessage}
}

func (r *opRun) resetOutput() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tail = nil
	r.lastLine = map[string]string{}
	r.scan = errorScan{}
}

// resumeOutcome decides what became of an adopted operation. park is true when
// the host cannot be asked right now.
func (s *Service) resumeOutcome(run *opRun) (out outcome, park bool) {
	op := run.snapshot()
	if op.Kind == KindPrepare {
		return lostRunner(op), false // prepare has no runner to find
	}
	ctx, cancel := s.runContext(op)
	defer cancel()

	_, e, err := s.target(ctx, op.HostID)
	if err != nil {
		switch {
		case s.baseCtx.Err() != nil:
			return outcome{abandoned: true}, false
		case errors.Is(err, dockerhost.ErrHostNotFound):
			return outcome{state: StateInterrupted, code: opCodeInterrupted, message: "The Docker host was removed while this operation was running."}, false
		}
		return outcome{}, true
	}
	name := runnerName(op.ID)
	st, err := e.inspectContainer(ctx, name)
	switch {
	case isNotFound(err):
		return lostRunner(op), false
	case err != nil && s.baseCtx.Err() != nil:
		return outcome{abandoned: true}, false
	case err != nil:
		return outcome{}, true
	case st.Status == "created": // never started: nothing ran
		s.removeQuietly(e, st.ID)
		return lostRunner(op), false
	}

	run.resetOutput()
	if op.State == StateQueued {
		run.setState(StateRunning)
	}
	final, err := s.followRunner(ctx, run, e, st.ID)
	if err != nil {
		out := s.failure(ctx, err)
		switch {
		case out.abandoned:
			return out, false
		case out.code == opCodeTimeout:
			killCtx, cancelKill := context.WithTimeout(context.Background(), cleanupTimeout)
			_ = e.killContainer(killCtx, st.ID)
			cancelKill()
		case isNotFound(err):
			return lostRunner(op), false
		case out.code == opCodeHostUnreachable:
			return outcome{}, true // the runner may well still be going
		}
		out.cleanup = s.runnerCleanup(e, st.ID)
		return out, false
	}
	return s.runnerOutcome(ctx, run, e, op, final, st.ID), false
}

// sweepOnce removes, once per host and process, the helper containers an earlier
// process left behind: exited runners whose operation is not active and old
// listers. Running containers and the runner of an active operation are never touched.
func (s *Service) sweepOnce(hostID string, e engine) {
	s.mu.Lock()
	if s.closed || s.swept[hostID] {
		s.mu.Unlock()
		return
	}
	s.swept[hostID] = true
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		if err := s.sweep(hostID, e); err != nil {
			s.log.Debug("docker manager helper sweep failed; it will be tried again", "host", hostID, "error", err)
			s.mu.Lock()
			delete(s.swept, hostID)
			s.mu.Unlock()
		}
	}()
}

// listerGrace is how old an exited lister must be before the sweep removes it;
// a younger one may still be read by the List that started it.
const listerGrace = 60 // seconds

func (s *Service) sweep(hostID string, e engine) error {
	ctx, cancel := context.WithTimeout(s.baseCtx, cleanupTimeout)
	defer cancel()
	containers, err := dockerhost.ListContainers(ctx, e.c)
	if err != nil {
		return err
	}
	now := s.opts.Now().Unix()
	for _, c := range containers {
		if c.State == "running" {
			continue
		}
		switch c.Labels[LabelRole] {
		case RoleRunner:
			s.mu.Lock()
			_, active := s.runs[c.Labels[LabelOperation]]
			s.mu.Unlock()
			if active {
				continue
			}
		case RoleLister:
			if now-c.Created < listerGrace {
				continue
			}
		default:
			continue
		}
		if err := e.removeContainer(ctx, c.ID); err != nil {
			s.log.Warn("docker manager helper container could not be removed", "host", hostID, "container", c.ID, "error", err)
		}
	}
	return nil
}

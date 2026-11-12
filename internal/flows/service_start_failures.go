package flows

import (
	"context"
	"errors"
	"time"
)

// StartFailedCode is the error code of a live run that never started because the trigger could
// not start it (see Service.recordStartFailure). The client translates it as
// easydrag.ui.error_flow_start_failed.
const StartFailedCode = "FLOW_START_FAILED"

// startFailureTimeout bounds the store writes of one recorded start failure.
const startFailureTimeout = 30 * time.Second

// Streak reasons of the failures that have a reason of their own; any other start error is
// "start_error: " and its text (cut), so a repeated identical error is one streak too.
const (
	startFailureNotPublished = "not_published"
	startFailureNoTrigger    = "no_trigger"
	startFailureOther        = "start_error: "
)

// startFailureReason classifies the error of a gated live start. It returns "" for a refusal or a
// condition that is not a fault of the flow (the failure is not recorded):
//   - the flow was deleted meanwhile (ErrNotFound), the runner is shutting down (ErrRunnerClosed),
//     the queue is full (ErrQueueFull, also ErrRunnerQueueFull) or the caller's context ended;
//   - the flow is switched off or Mission Control cannot say (ErrFlowDisabled,
//     ErrFlowMissionMissing, ErrMissionControlUnavailable), the refusals of the block admission;
//   - the durable runner refuses or cannot serve (ErrDurableCapacity, ErrDurableUnavailable,
//     ErrDurableDisabled, ErrRunRecoveryUnavailable): a condition of the service, not of the flow.
//
// Otherwise it returns the reason that names the streak.
func startFailureReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotPublished):
		return startFailureNotPublished
	case errors.Is(err, ErrNoTrigger):
		return startFailureNoTrigger
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrRunnerClosed), errors.Is(err, ErrQueueFull),
		errors.Is(err, ErrFlowDisabled), errors.Is(err, ErrFlowMissionMissing), errors.Is(err, ErrMissionControlUnavailable),
		errors.Is(err, ErrDurableCapacity), errors.Is(err, ErrDurableUnavailable), errors.Is(err, ErrDurableDisabled), errors.Is(err, ErrRunRecoveryUnavailable),
		errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ""
	}
	return startFailureOther + truncateRunes(err.Error(), 200)
}

// IsStartFailure reports whether err, returned by TriggerFromMission, is a start failure that
// the Service records as a failed run. The server uses it to mark the error for Mission Control.
// A refusal of the RunGate is not visible here (see Service.afterTriggerStart); the server's
// hook asks that gate before it starts anything.
func IsStartFailure(err error) bool { return startFailureReason(err) != "" }

// startFailureMessage says in plain words why a start failed. The Store scrubs it when it is
// written, like every run error; it is cut to maxErrorMessageRunes first.
func startFailureMessage(reason string, err error) string {
	switch reason {
	case startFailureNotPublished:
		return "the flow is not published"
	case startFailureNoTrigger:
		return "the trigger that fired is not an enabled trigger of the published flow"
	}
	return truncateRunes("the flow could not start: "+err.Error(), maxErrorMessageRunes)
}

// afterTriggerStart is TriggerFromMission's bookkeeping after startLive: a start (started or
// queued) ends the flow's failure streak; a start failure that is not a refusal opens a new
// streak, and only then a goroutine records it. The streak is decided here, synchronously,
// because TriggerFromMission can run inside another flow's DeleteFlow (through
// MissionBridge.FlowRunFinished) with that flow's lock held: nothing slow may run on this
// goroutine, and the streak bounds the goroutines to one per flow and reason. A closed RunGate
// (runs paused) is a refusal, not a fault of the flow, so it records nothing.
func (s *Service) afterTriggerStart(rec *FlowRecord, nodeID, triggerType string, res StartResult, err error) {
	if err == nil {
		if res.Status == StartStarted || res.Status == StartQueued {
			s.clearStartFailure(rec.ID)
		}
		return
	}
	reason := startFailureReason(err)
	if reason == "" || s.automaticRunsAllowed() != nil {
		return
	}
	if !s.beginStartFailure(rec.ID, reason) {
		return
	}
	go s.recordStartFailure(rec, nodeID, triggerType, reason, err)
}

// beginStartFailure reports whether a failure with reason opens a new streak of the flow, and
// then counts the goroutine that records it. It reports false for a repeat of the current
// streak and after Shutdown began.
func (s *Service) beginStartFailure(flowID, reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.startStreaks[flowID] == reason {
		return false
	}
	s.startStreaks[flowID] = reason
	s.startFailures.Add(1)
	return true
}

// clearStartFailure ends the flow's failure streak: a start succeeded, or the flow is gone.
func (s *Service) clearStartFailure(flowID string) {
	s.mu.Lock()
	delete(s.startStreaks, flowID)
	s.mu.Unlock()
}

// recordStartFailure writes the failed run of a start that did not happen and reports it to
// the bridge's optional StartFailureReporter. The run is live, has the flow's live revision (0
// when there is none), the trigger node and type, status error, StartFailedCode, a plain
// message, StartedAt = FinishedAt = now and no steps and no trigger data (a start that failed
// never ran on it). It runs on its own goroutine (afterTriggerStart) and ends the WaitGroup
// count. When the flow is gone (ErrNotFound) it records nothing and clears the streak; on any
// other store error it logs at Warn and clears the streak, so the next failure tries again.
func (s *Service) recordStartFailure(rec *FlowRecord, nodeID, triggerType, reason string, startErr error) {
	defer s.startFailures.Done()
	ctx, cancel := context.WithTimeout(context.Background(), startFailureTimeout)
	defer cancel()
	now := s.now()
	msg := startFailureMessage(reason, startErr)
	run := RunRecord{ID: NewRunID(), FlowID: rec.ID, Revision: rec.LiveRevision, Mode: ModeLive,
		TriggerNode: truncateRunes(nodeID, 80), TriggerType: cleanTriggerType(triggerType),
		Status: RunError, StartedAt: now}
	if run.TriggerNode == "" {
		doc := rec.Live
		if doc == nil {
			doc = rec.Draft
		}
		if doc != nil {
			run.TriggerNode = s.pickTrigger(doc, "")
		}
	}
	res := RunResult{Status: RunError, ErrorCode: StartFailedCode, ErrorMessage: msg, StartedAt: now, FinishedAt: now}
	err := s.store.CreateRun(ctx, run, nil)
	if err == nil {
		err = s.store.FinishRun(ctx, run.ID, res)
	}
	if err != nil {
		s.clearStartFailure(rec.ID)
		s.logLookup("a trigger start failure could not be recorded as a run", run.ID, err, "flow", rec.ID)
		return
	}
	run.ErrorCode, run.ErrorMessage, run.FinishedAt = StartFailedCode, msg, &now
	reporter, ok := s.bridge.(StartFailureReporter)
	if !ok {
		return
	}
	notify := ""
	if doc := rec.Live; doc != nil {
		notify = doc.Settings.NotifyOnError
	} else if rec.Draft != nil {
		notify = rec.Draft.Settings.NotifyOnError
	}
	defer func() {
		if v := recover(); v != nil {
			s.logger.Error("the bridge panicked on a start failure", "flow", rec.ID, "run", run.ID, "panic", v)
		}
	}()
	reporter.FlowStartFailed(RunFinishedInfo{MissionID: rec.MissionID, Started: false, FlowName: rec.Name,
		NotifyOnError: notify, Record: run, Result: res, Outputs: map[string]any{}})
}

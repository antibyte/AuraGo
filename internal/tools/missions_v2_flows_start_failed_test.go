package tools

import (
	"errors"
	"testing"
)

// A trigger whose start failed for a reason of the flow (the service recorded a failed run) is
// noted on the mission like a refusal, once per change, and the next run result replaces it.
func TestFlowTriggerStartFailureNotesTheMission(t *testing.T) {
	mm, hooks, webhooks := newFlowTestManager(t)
	id := publishTestFlow(t, mm, pausedWebhookSpec())

	hooks.setErr(&FlowStartFailedError{Err: errors.New("the flow has not been published yet")})
	webhooks.Fire("hook-1", []byte(`{}`))
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	hooks.waitStart(t)
	eventually(t, "the start-failed note", func() bool {
		m, _ := mm.Get(id)
		return m.LastResult == MissionResultError && m.LastOutput == flowStartFailedOutput && m.RunCount == 0
	})

	hooks.setErr(nil)
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	mm.FlowRunFinished(id, mm.FlowRunStarted(id, "webhook", ""), MissionResultSuccess, "alles gut", nil)
	if m, _ := mm.Get(id); m.LastResult != MissionResultSuccess || m.LastOutput != "alles gut" {
		t.Fatalf("flow mission after the next run: %+v", m)
	}
}

func TestFlowStartFailedErrorKeepsTheCauseAndItsText(t *testing.T) {
	cause := errors.New("the flow has not been published yet")
	err := error(&FlowStartFailedError{Err: cause})
	if !errors.Is(err, ErrFlowStartFailed) || !errors.Is(err, cause) || err.Error() != cause.Error() {
		t.Fatalf("FlowStartFailedError = %v", err)
	}
	if flowRefusal(err) != ErrFlowStartFailed || flowRefusalOutput(ErrFlowStartFailed) != flowStartFailedOutput {
		t.Fatalf("flowRefusal = %v", flowRefusal(err))
	}
}

// A skipped start (the flow's skip policy dropped it) gets a quiet note: the output only, not
// the result, and only when the mission does not show a skip already.
func TestFlowSkippedStartGetsAQuietNoteOnce(t *testing.T) {
	mm, hooks, webhooks := newFlowTestManager(t)
	reporter := &fb4ReporterHooks{fakeFlowHooks: hooks, outcome: FlowStartOutcome{Status: "skipped"}}
	mm.SetFlowHooks(reporter)
	id := publishTestFlow(t, mm, pausedWebhookSpec())
	mm.FlowRunFinished(id, mm.FlowRunStarted(id, "webhook", ""), MissionResultSuccess, "alles gut", nil)

	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	eventually(t, "the skip note", func() bool {
		m, _ := mm.Get(id)
		return m.LastOutput == flowStartSkippedOutput
	})
	if m, _ := mm.Get(id); m.LastResult != MissionResultSuccess {
		t.Fatalf("a skip must not change the result: %+v", m)
	}
	mm.mu.Lock()
	again := mm.noteFlowSkipLocked(id)
	mm.mu.Unlock()
	if again {
		t.Fatal("a second skip changed the mission")
	}
}

package tools

import (
	"errors"
	"fmt"
	"testing"
)

type fb4ReporterHooks struct {
	*fakeFlowHooks
	outcome FlowStartOutcome
}

func (h *fb4ReporterHooks) StartFlowRunReported(missionID, nodeID, triggerType, data string) (FlowStartOutcome, error) {
	if err := h.fakeFlowHooks.StartFlowRun(missionID, nodeID, triggerType, data); err != nil {
		return FlowStartOutcome{}, err
	}
	return h.outcome, nil
}

func TestFB4RunNowReportsAFlowSkip(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	reporter := &fb4ReporterHooks{fakeFlowHooks: hooks, outcome: FlowStartOutcome{Status: "skipped"}}
	mm.SetFlowHooks(reporter)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	var skipped *MissionTriggerSkippedError
	if err := mm.RunNow(id); !errors.As(err, &skipped) || skipped.Reason != FlowStartSkippedReason {
		t.Fatalf("RunNow of a busy skip flow = %v", err)
	}
	reporter.outcome = FlowStartOutcome{RunID: "run_fb4", Status: "started"}
	if err := mm.RunNow(id); err != nil {
		t.Fatalf("RunNow of a started run = %v", err)
	}
}

func TestFB4QueueFullIsANotedRefusal(t *testing.T) {
	mm, hooks, webhooks := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-fb4"}})
	hooks.setErr(fmt.Errorf("%w: the flow's run queue is full", ErrFlowQueueFull))
	webhooks.Fire("hook-fb4", []byte(`{}`))
	hooks.waitStart(t)
	eventually(t, "the queue-full note", func() bool {
		m, _ := mm.Get(id)
		return m.LastResult == MissionResultError && m.LastOutput == flowQueueFullOutput
	})
}

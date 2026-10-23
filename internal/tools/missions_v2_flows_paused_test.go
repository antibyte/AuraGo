package tools

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
)

// A flow trigger or planner admission that the hooks refuse on purpose (ErrFlowRunsPaused while
// missions are read-only, ErrFlowsUnavailable when flows were switched off after the boot) is no
// failed start: the mission shows why, a trigger that keeps firing logs one warning, and a
// planner target is canceled instead of left "unknown".

func pausedWebhookSpec() FlowTriggerSpec {
	return FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}}
}

func TestFlowTriggerRefusedByTheHooksNotesTheMissionAndWarnsOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		output string
		reason string
	}{
		{"paused", ErrFlowRunsPaused, flowRunsPausedOutput, "flow runs are paused"},
		{"paused and wrapped", fmt.Errorf("gate: %w", ErrFlowRunsPaused), flowRunsPausedOutput, "flow runs are paused"},
		{"unavailable", ErrFlowsUnavailable, flowsUnavailableOutput, "flows are not available"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := c07CaptureLogs(t, slog.LevelDebug)
			mm, hooks, webhooks := newFlowTestManager(t)
			hooks.setErr(tc.err)
			id := publishTestFlow(t, mm, pausedWebhookSpec())
			// Every save fails while a directory sits where save writes its temporary file, and
			// each failed save logs once: the log counts the save attempts.
			if err := os.Mkdir(mm.file+".tmp", 0o700); err != nil {
				t.Fatal(err)
			}

			webhooks.Fire("hook-1", []byte(`{}`))
			webhooks.Fire("hook-1", []byte(`{}`))
			hooks.waitStart(t)
			hooks.waitStart(t)

			m, _ := mm.Get(id)
			if m.LastResult != MissionResultError || m.LastOutput != tc.output || m.RunCount != 0 || m.Status != MissionStatusIdle {
				t.Fatalf("flow mission after two refused triggers: %+v", m)
			}
			out := logs.String()
			if got := c16LogLines(out, "level=WARN", tc.reason); got != 1 {
				t.Fatalf("warnings for a repeated refusal = %d, want 1:\n%s", got, out)
			}
			if got := c16LogLines(out, "level=DEBUG", tc.reason); got != 1 {
				t.Fatalf("debug lines for the repeat = %d, want 1:\n%s", got, out)
			}
			if got := c16LogLines(out, "could not start a run"); got != 0 {
				t.Fatalf("a deliberate refusal logged as a failed start:\n%s", out)
			}
			if got := c16LogLines(out, "Failed to persist the flow mission state"); got != 1 {
				t.Fatalf("save attempts for two refused triggers = %d, want 1:\n%s", got, out)
			}
		})
	}
}

func TestFlowPausedNoteGivesWayToTheNextRunResult(t *testing.T) {
	logs := c07CaptureLogs(t, slog.LevelDebug)
	mm, hooks, webhooks := newFlowTestManager(t)
	id := publishTestFlow(t, mm, pausedWebhookSpec())

	hooks.setErr(ErrFlowRunsPaused)
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	if m, _ := mm.Get(id); m.LastResult != MissionResultError || m.LastOutput != flowRunsPausedOutput {
		t.Fatalf("flow mission while runs are paused: %+v", m)
	}

	// Runs resume: the run that starts reports its result, which replaces the note.
	hooks.setErr(nil)
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	mm.FlowRunFinished(id, mm.FlowRunStarted(id, "webhook", ""), MissionResultSuccess, "alles gut", nil)
	if m, _ := mm.Get(id); m.LastResult != MissionResultSuccess || m.LastOutput != "alles gut" || m.RunCount != 1 {
		t.Fatalf("flow mission after the run: %+v", m)
	}

	// Paused again: the note comes back and warns again, because it changed the mission again.
	hooks.setErr(ErrFlowRunsPaused)
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	if m, _ := mm.Get(id); m.LastResult != MissionResultError || m.LastOutput != flowRunsPausedOutput {
		t.Fatalf("flow mission when paused again: %+v", m)
	}
	if out := logs.String(); c16LogLines(out, "level=WARN", "flow runs are paused") != 2 {
		t.Fatalf("want one warning per pause:\n%s", out)
	}
}

func TestFlowTriggerOtherStartErrorsStillWarnEveryTime(t *testing.T) {
	logs := c07CaptureLogs(t, slog.LevelDebug)
	mm, hooks, webhooks := newFlowTestManager(t)
	hooks.setErr(errors.New("store down"))
	id := publishTestFlow(t, mm, pausedWebhookSpec())

	webhooks.Fire("hook-1", []byte(`{}`))
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	hooks.waitStart(t)

	if out := logs.String(); c16LogLines(out, "level=WARN", "could not start a run", "store down") != 2 {
		t.Fatalf("want a warning for every failed start:\n%s", out)
	}
	if m, _ := mm.Get(id); m.LastResult != "" || m.LastOutput != "" {
		t.Fatalf("a failed start must not leave a note: %+v", m)
	}
}

func TestPlannerFlowAdmissionRefusedOnPurposeIsCanceled(t *testing.T) {
	const unknownNote = "Planner flow dispatch outcome unknown; verify before retrying"
	for _, tc := range []struct {
		name    string
		err     error
		state   string
		output  string
		unknown int // "outcome unknown" warnings
	}{
		{"paused", ErrFlowRunsPaused, "canceled", flowRunsPausedOutput, 0},
		{"unavailable and wrapped", fmt.Errorf("start: %w", ErrFlowsUnavailable), "canceled", flowsUnavailableOutput, 0},
		{"other error", errors.New("store down"), "unknown", unknownNote, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := c07CaptureLogs(t, slog.LevelDebug)
			mm, hooks, _ := newFlowTestManager(t)
			hooks.setErr(tc.err)
			id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerPlannerAppointmentDue, TriggerConfig: &TriggerConfig{}})
			if err := mm.NotifyPlannerAppointmentDueEvent("event-1", "apt", "Zahnarzt", "2026-10-19T00:00:00Z"); err != nil {
				t.Fatal(err)
			}

			mm.processPlannerFlowAdmissions()
			hooks.waitStart(t)
			mm.mu.RLock()
			state := mm.plannerEvents["event-1"].Flows[0].State
			mm.mu.RUnlock()
			if state != tc.state {
				t.Fatalf("planner target state = %q, want %q", state, tc.state)
			}
			if m, _ := mm.Get(id); m.LastOutput != tc.output {
				t.Fatalf("flow mission note = %q, want %q", m.LastOutput, tc.output)
			}
			if got := c16LogLines(logs.String(), "outcome unknown"); got != tc.unknown {
				t.Fatalf("outcome-unknown warnings = %d, want %d:\n%s", got, tc.unknown, logs.String())
			}
			// A refused target is settled: the next tick does not dispatch it again.
			mm.processPlannerFlowAdmissions()
			hooks.expectNoStart(t)
		})
	}
}

package tools

import (
	"errors"
	"fmt"
	"testing"
)

// R1: a queued flow run that the flow service's gate stopped before it executed is noted on its
// mission like a refused trigger; other errors leave the mission alone.
func TestB3NoteFlowRunRefused(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, pausedWebhookSpec())
	if mm.NoteFlowRunRefused(id, errors.New("store down")) {
		t.Fatal("an unrelated error noted the mission")
	}
	if m, _ := mm.Get(id); m.LastResult != "" || m.LastOutput != "" {
		t.Fatalf("mission after an unrelated error: %+v", m)
	}
	if !mm.NoteFlowRunRefused(id, fmt.Errorf("gate: %w", ErrFlowRunsPaused)) {
		t.Fatal("the paused refusal did not change the mission")
	}
	if m, _ := mm.Get(id); m.LastResult != MissionResultError || m.LastOutput != flowRunsPausedOutput || m.RunCount != 0 {
		t.Fatalf("mission after a paused queued run: %+v", m)
	}
	if mm.NoteFlowRunRefused(id, ErrFlowRunsPaused) {
		t.Fatal("the same note again reported a change")
	}
}

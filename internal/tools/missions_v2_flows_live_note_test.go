package tools

import "testing"

// R2b: a published revision that no longer passes the flow service's current checks is noted on
// its mission once; an unknown mission is ignored.
func TestB3NoteFlowLiveRevisionOutdated(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, pausedWebhookSpec())
	if !mm.NoteFlowLiveRevisionOutdated(id) {
		t.Fatal("the note did not change the mission")
	}
	if m, _ := mm.Get(id); m.LastResult != MissionResultError || m.LastOutput != flowLiveRevisionOutdatedOutput || m.RunCount != 0 {
		t.Fatalf("mission after the note: %+v", m)
	}
	if mm.NoteFlowLiveRevisionOutdated(id) {
		t.Fatal("the same note again reported a change")
	}
	if mm.NoteFlowLiveRevisionOutdated("mission_none") {
		t.Fatal("an unknown mission was noted")
	}
}

// R2b: a new publication (or a clean re-check) takes the outdated-revision note away again; any
// other result or output stays.
func TestB3ClearFlowLiveRevisionOutdated(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, pausedWebhookSpec())
	if mm.ClearFlowLiveRevisionOutdated(id) {
		t.Fatal("a clean mission reported a change")
	}
	mm.NoteFlowLiveRevisionOutdated(id)
	if !mm.ClearFlowLiveRevisionOutdated(id) {
		t.Fatal("the note was not cleared")
	}
	if m, _ := mm.Get(id); m.LastResult != "" || m.LastOutput != "" || m.RunCount != 0 {
		t.Fatalf("mission after the clear: %+v", m)
	}
	mm.FlowRunFinished(id, mm.FlowRunStarted(id, "webhook", ""), MissionResultSuccess, "alles gut", nil)
	if mm.ClearFlowLiveRevisionOutdated(id) {
		t.Fatal("a real run result was cleared")
	}
	if m, _ := mm.Get(id); m.LastResult != MissionResultSuccess || m.LastOutput != "alles gut" {
		t.Fatalf("mission after the no-op clear: %+v", m)
	}
	if mm.ClearFlowLiveRevisionOutdated("mission_none") {
		t.Fatal("an unknown mission was cleared")
	}
}

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

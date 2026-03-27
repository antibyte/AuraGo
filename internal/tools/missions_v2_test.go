package tools

import "testing"

func TestMissionQueueTryStartNextIsAtomic(t *testing.T) {
	q := NewMissionQueue()
	q.Enqueue("m1", "high", "", "")
	q.Enqueue("m2", "low", "", "")

	item, ok := q.TryStartNext()
	if !ok {
		t.Fatal("expected first mission to start")
	}
	if item.MissionID != "m1" {
		t.Fatalf("started mission = %q, want m1", item.MissionID)
	}

	if _, ok := q.TryStartNext(); ok {
		t.Fatal("did not expect a second mission to start while one is running")
	}

	if got := q.GetRunning(); got != "m1" {
		t.Fatalf("running mission = %q, want m1", got)
	}

	q.Done()
	item, ok = q.TryStartNext()
	if !ok {
		t.Fatal("expected second mission to start after Done")
	}
	if item.MissionID != "m2" {
		t.Fatalf("started mission = %q, want m2", item.MissionID)
	}
}

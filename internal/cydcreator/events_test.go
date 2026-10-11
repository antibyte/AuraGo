package cydcreator

import (
	"context"
	"testing"
	"time"
)

func TestEmitPersistsAndFansOut(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe("ccp_x")
	defer cancel()
	other, cancelOther := env.svc.Subscribe("ccp_other")
	defer cancelOther()

	env.svc.Emit("ccp_x", EventProjectChanged, "", "", map[string]any{"version": 2})
	env.svc.EmitEphemeral("ccp_x", EventBuildLog, "", "ccb_1", map[string]any{"lines": []string{"a"}})
	env.svc.Emit("ccp_x", EventBuildStatus, "ccj_1", "ccb_1", nil)

	got := collectEvents(t, events, 3, 2*time.Second)
	if got[0].Seq == 0 || got[1].Seq != 0 || got[2].Seq <= got[0].Seq {
		t.Fatalf("seqs = %d %d %d; persisted events need increasing seqs, ephemeral 0", got[0].Seq, got[1].Seq, got[2].Seq)
	}
	if got[2].JobID != "ccj_1" || got[2].BuildID != "ccb_1" || string(got[2].Data) != `{}` || got[2].CreatedAt.IsZero() {
		t.Fatalf("event = %+v", got[2])
	}
	stored, err := env.svc.EventsAfter(ctx, "ccp_x", 0, 0)
	if err != nil || len(stored) != 2 || stored[0].Seq != got[0].Seq || stored[1].Type != EventBuildStatus {
		t.Fatalf("EventsAfter = %+v, %v; want the two persisted events", stored, err)
	}
	after, _ := env.svc.EventsAfter(ctx, "ccp_x", stored[0].Seq, 0)
	if len(after) != 1 || after[0].Seq != stored[1].Seq {
		t.Fatalf("EventsAfter(after first) = %+v", after)
	}
	select {
	case ev := <-other:
		t.Fatalf("another project's subscriber got %+v", ev)
	default:
	}
}

func TestEventsAfterPagesOf500(t *testing.T) {
	env := newTestService(t)
	for i := 0; i < 1200; i++ {
		env.svc.Emit("ccp_pages", EventProjectChanged, "", "", map[string]any{"i": i})
	}
	var after int64
	var pages []int
	for {
		page, err := env.svc.EventsAfter(context.Background(), "ccp_pages", after, 500)
		if err != nil {
			t.Fatal(err)
		}
		pages = append(pages, len(page))
		if len(page) < 500 {
			break
		}
		after = page[len(page)-1].Seq
	}
	if len(pages) != 3 || pages[0] != 500 || pages[1] != 500 || pages[2] != 200 {
		t.Fatalf("pages = %v, want [500 500 200]", pages)
	}
	if page, _ := env.svc.EventsAfter(context.Background(), "ccp_pages", 0, 9999); len(page) != 500 {
		t.Fatalf("limit above 500 returned %d events", len(page))
	}
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	env := newTestService(t)
	events, cancel := env.svc.Subscribe("ccp_slow")
	defer cancel()
	for i := 0; i < subscriberBuffer+1; i++ {
		env.svc.EmitEphemeral("ccp_slow", EventBuildLog, "", "", nil)
	}
	n := 0
	for range events {
		n++
	}
	if n != subscriberBuffer {
		t.Fatalf("received %d buffered events before the close, want %d", n, subscriberBuffer)
	}
	cancel() // idempotent after the drop
}

func TestCloseEndsSubscriptions(t *testing.T) {
	env := newTestService(t)
	events, cancel := env.svc.Subscribe("ccp_close")
	defer cancel()
	if err := env.svc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, ok := <-events; ok {
		t.Fatal("a subscription survived Close")
	}
	late, cancelLate := env.svc.Subscribe("ccp_close")
	defer cancelLate()
	if _, ok := <-late; ok {
		t.Fatal("Subscribe after Close returned an open channel")
	}
	env.svc.EmitEphemeral("ccp_close", EventBuildLog, "", "", nil) // must not panic
}

func TestEventsArePrunedPerProject(t *testing.T) {
	savedMax, savedEvery := maxEventsPerProject, eventPruneEvery
	maxEventsPerProject, eventPruneEvery = 10, 5
	t.Cleanup(func() { maxEventsPerProject, eventPruneEvery = savedMax, savedEvery })
	env := newTestService(t)
	for i := 0; i < 30; i++ {
		env.svc.Emit("ccp_prune", EventProjectChanged, "", "", map[string]any{"i": i})
	}
	env.svc.Emit("ccp_keep", EventProjectChanged, "", "", nil)
	stored, _ := env.svc.EventsAfter(context.Background(), "ccp_prune", 0, 0)
	if len(stored) < 10 || len(stored) > 14 {
		t.Fatalf("kept %d events, want about 10", len(stored))
	}
	if last := stored[len(stored)-1]; string(last.Data) != `{"i":29}` {
		t.Fatalf("newest event lost: %s", last.Data)
	}
	if kept, _ := env.svc.EventsAfter(context.Background(), "ccp_keep", 0, 0); len(kept) != 1 {
		t.Fatalf("another project's events were pruned: %d", len(kept))
	}
}

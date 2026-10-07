package flows

import (
	"context"
	"errors"
	"testing"
)

// FF1: Run now refuses a published flow that is switched off (paused), as Mission
// Control, the agent and the missions page refuse a disabled mission. Test runs of the
// draft stay allowed, and switching the flow on lets Run now start it.
func TestFF1RunNowRefusesAPausedFlow(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	pub, _ := publishedSimpleFlow(t, s)
	if err := s.SetEnabled(ctx, pub.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunNow(ctx, pub.ID); !errors.Is(err, ErrFlowDisabled) {
		t.Fatalf("RunNow on a paused flow = %v", err)
	}
	if len(bridge.startedRuns()) != 0 {
		t.Fatal("a refused Run now started a run")
	}
	if ErrFlowDisabled.Error() != "the flow is paused; switch it on first" {
		t.Fatalf("message = %q", ErrFlowDisabled.Error())
	}
	res, err := s.StartTestRun(ctx, pub.ID, TestRunRequest{})
	if err != nil {
		t.Fatalf("a test run of a paused flow = %v", err)
	}
	waitRun(t, s, res.RunID)

	if err := s.SetEnabled(ctx, pub.ID, true); err != nil {
		t.Fatal(err)
	}
	res, err = s.RunNow(ctx, pub.ID)
	if err != nil {
		t.Fatalf("RunNow on an enabled flow = %v", err)
	}
	if rec := waitRun(t, s, res.RunID); rec.Status != RunSuccess {
		t.Fatalf("run = %+v", rec)
	}
	// A draft that was never published still answers ErrNotPublished, not paused.
	draft, err := s.CreateFlow(ctx, CreateRequest{Name: "Entwurf"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunNow(ctx, draft.ID); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("RunNow on a draft = %v", err)
	}
}

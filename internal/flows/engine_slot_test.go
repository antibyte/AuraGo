package flows

import (
	"context"
	"testing"
)

type slotGateTestSlot struct {
	releases int
	acquires int
}

func (s *slotGateTestSlot) Release() bool { s.releases++; return true }
func (s *slotGateTestSlot) Acquire(ctx context.Context) error {
	s.acquires++
	return ctx.Err()
}

func TestSlotGateKeepsSlotWhileDispatching(t *testing.T) {
	for _, afterResult := range []bool{false, true} {
		slot := &slotGateTestSlot{}
		gate := newSlotGate(slot, context.Background())
		if afterResult {
			gate.launched()
			gate.idle()
			gate.received()
		}
		gate.launched()
		gate.Park() // The wait runs before its ready sibling has been launched.
		if slot.releases != 0 {
			t.Fatalf("afterResult=%v: released while dispatching", afterResult)
		}
		gate.launched()
		gate.idle()
		if slot.releases != 0 {
			t.Fatalf("afterResult=%v: released with a busy sibling", afterResult)
		}
		gate.received()
		gate.idle()
		if slot.releases != 1 {
			t.Fatalf("afterResult=%v: releases=%d, want 1 after sibling completed", afterResult, slot.releases)
		}
		if err := gate.Unpark(); err != nil || slot.acquires != 1 {
			t.Fatalf("afterResult=%v: unpark: %v, acquires=%d", afterResult, err, slot.acquires)
		}
		gate.received()
	}
}

package agent

import (
	"context"
	"testing"
)

func TestWithAgentLoopSlotReleasesAfterPanic(t *testing.T) {
	original := agentLoopLimiter
	agentLoopLimiter = make(chan struct{}, 1)
	t.Cleanup(func() { agentLoopLimiter = original })

	func() {
		defer func() {
			if rec := recover(); rec == nil {
				t.Fatalf("expected panic")
			}
		}()
		withAgentLoopSlot(context.Background(), func() {
			panic("boom")
		})
	}()

	select {
	case agentLoopLimiter <- struct{}{}:
	default:
		t.Fatalf("agent loop slot leaked after panic")
	}
}

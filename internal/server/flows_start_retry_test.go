package server

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/flows"
)

// A Start that fails at boot (a locked flows.db) used to leave the Date/Time timers, the
// retention loop and the interrupted-run marking off until the next restart. startFlows now
// retries in the background with a growing delay, logs every failure (Warn, with the attempt
// and the next delay) and the recovery (Info), and reconciles Mission Control once Start
// worked. Shutdown ends the retry at once and joins it.
func TestAudit23FlowsStartIsRetriedInTheBackground(t *testing.T) {
	realStart := flowsStart
	t.Cleanup(func() { flowsStart = realStart })
	t.Run("recovers", func(t *testing.T) {
		logs := &c15LogBuffer{}
		s := c16Server(t, true, c15Logger(logs))
		s.initFlows()
		if s.Flows == nil {
			t.Fatal("flows were not initialised")
		}
		if err := s.MissionManagerV2.Start(); err != nil {
			t.Fatalf("missions: %v", err)
		}
		var calls atomic.Int32
		flowsStart = func(ctx context.Context, svc *flows.Service) error {
			if calls.Add(1) == 1 {
				return errors.New("database is locked")
			}
			return realStart(ctx, svc)
		}
		delays := flowsStartRetryDelay
		flowsStartRetryDelay = func(int) time.Duration { return time.Millisecond }
		t.Cleanup(func() { flowsStartRetryDelay = delays })
		t.Cleanup(func() { s.shutdownFlows(context.Background()) })

		s.startFlows(context.Background())
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(logs.String(), "flow missions reconciled") {
			if time.Now().After(deadline) {
				t.Fatalf("flows never started or reconciled; %d start calls:\n%s", calls.Load(), logs.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
		out := logs.String()
		if got := calls.Load(); got != 2 {
			t.Errorf("Start was called %d times, want 2 (one failure, one retry)", got)
		}
		if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "could not start") || !strings.Contains(out, "attempt=1") ||
			!strings.Contains(out, "next_attempt_in=1ms") || !strings.Contains(out, "level=INFO") || !strings.Contains(out, "flows started after") {
			t.Errorf("the failure and the recovery are not logged as asked:\n%s", out)
		}
	})
	t.Run("shutdown ends the retry", func(t *testing.T) {
		s := c16Server(t, true, nil)
		s.initFlows()
		if s.Flows == nil {
			t.Fatal("flows were not initialised")
		}
		if err := s.MissionManagerV2.Start(); err != nil {
			t.Fatalf("missions: %v", err)
		}
		var calls atomic.Int32
		flowsStart = func(context.Context, *flows.Service) error {
			calls.Add(1)
			return errors.New("database is locked")
		}
		delays := flowsStartRetryDelay
		flowsStartRetryDelay = func(int) time.Duration { return time.Hour }
		t.Cleanup(func() { flowsStartRetryDelay = delays })

		s.startFlows(context.Background())
		if _, ok := flowsStartRetries.Load(s.Flows); !ok {
			t.Fatal("no background retry is running")
		}
		begin := time.Now()
		s.shutdownFlows(context.Background())
		if waited := time.Since(begin); waited > 5*time.Second {
			t.Errorf("shutdown waited %s for the retry", waited)
		}
		if _, ok := flowsStartRetries.Load(s.Flows); ok {
			t.Error("the retry is still registered after the shutdown")
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("Start was called %d times, want 1: the retry ended while it slept", got)
		}
	})
}

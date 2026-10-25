package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/flows"
)

// R4: the flow list and the flow record say whether the Date/Time timers run.
func TestB3FlowsAPIReportsTimersRunning(t *testing.T) {
	s, token := newFlowsTestServer(t)
	if list := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, "")); list["timers_running"] != true {
		t.Errorf("list timers_running = %v, want true once the flows started", list["timers_running"])
	}
	rec := createTestFlow(t, s, greetFlowJSON)
	if one := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+rec.ID, token, "")); one["timers_running"] != true {
		t.Errorf("flow timers_running = %v, want true", one["timers_running"])
	}
	_ = s.Flows.Shutdown(context.Background())
	if list := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, "")); list["timers_running"] != false {
		t.Errorf("list timers_running after Shutdown = %v, want false", list["timers_running"])
	}
}

// R4: a start retry stuck inside Service.Start (it does not observe its context) holds
// shutdownFlows up for flowsStartRetryJoinTimeout at most, logged at Warn, so Service.Shutdown
// keeps its budget.
func TestB3ShutdownBoundsTheWaitForAStuckStartRetry(t *testing.T) {
	realStart := flowsStart
	delays, join := flowsStartRetryDelay, flowsStartRetryJoinTimeout
	t.Cleanup(func() { flowsStart, flowsStartRetryDelay, flowsStartRetryJoinTimeout = realStart, delays, join })
	flowsStartRetryDelay = func(int) time.Duration { return time.Millisecond }
	flowsStartRetryJoinTimeout = 200 * time.Millisecond
	logs := &c15LogBuffer{}
	s := c16Server(t, true, c15Logger(logs))
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flows were not initialised")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatalf("missions: %v", err)
	}
	stuck, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	var calls atomic.Int32
	flowsStart = func(context.Context, *flows.Service) error {
		if calls.Add(1) == 1 {
			return errors.New("database is locked")
		}
		close(stuck)
		<-release // ignores its context, like a Start stuck in SQLite
		return errors.New("database is locked")
	}
	s.startFlows(context.Background())
	select {
	case <-stuck:
	case <-time.After(5 * time.Second):
		t.Fatal("the background retry never called Start again")
	}
	begin := time.Now()
	s.shutdownFlows(context.Background())
	if waited := time.Since(begin); waited > 3*time.Second {
		t.Errorf("shutdown waited %s for a stuck start retry, want about %s", waited, flowsStartRetryJoinTimeout)
	}
	if !strings.Contains(logs.String(), "did not end in time") {
		t.Errorf("the abandoned retry is not logged:\n%s", logs.String())
	}
}

package flows

import (
	"context"
	"testing"
)

// R4: TimersRunning is true from a successful Start until Shutdown, so the API can show that
// Date/Time triggers do not fire while Start keeps failing.
func TestB3TimersRunningFollowsStartAndShutdown(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	if s.TimersRunning() {
		t.Fatal("timers reported running before Start")
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !s.TimersRunning() {
		t.Fatal("timers not reported running after Start")
	}
	_ = s.Shutdown(context.Background())
	if s.TimersRunning() {
		t.Fatal("timers reported running after Shutdown")
	}
}

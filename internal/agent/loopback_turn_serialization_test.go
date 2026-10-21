package agent

import (
	"context"
	"testing"
	"testing/synctest"
)

func TestLoopbackIngressWaitsBeforeHistoryAndStopCancelsQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run, client, cleanup := newPromptPipelineTestRunConfig(t, "default", "sms")
		defer cleanup()
		id := run.SessionID
		unlock, err := run.ShortTermMem.LockSessionTurn(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		defer unlock()
		done := make(chan struct{})
		go func() { defer close(done); LoopbackContext(context.Background(), run, "queued loopback", NoopBroker{}) }()
		// Wait for the queued goroutine instead of guessing its registration time.
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("loopback bypassed shared ingress gate")
		default:
		}
		messages, err := run.ShortTermMem.GetRecentMessages(id, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(messages) != 0 {
			t.Fatal("loopback wrote history before acquiring shared turn gate")
		}
		run.ShortTermMem.CancelSessionRuns(id)
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("Stop failed to cancel queued loopback")
		}
		if client.calls != 0 {
			t.Fatal("canceled loopback reached provider")
		}
	})
}

package agent

import (
	"context"
	"testing"
	"time"
)

func TestLoopbackIngressWaitsBeforeHistoryAndStopCancelsQueue(t *testing.T) {
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
	select {
	case <-done:
		t.Fatal("loopback bypassed shared ingress gate")
	case <-time.After(20 * time.Millisecond):
	}
	messages, err := run.ShortTermMem.GetRecentMessages(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatal("loopback wrote history before acquiring shared turn gate")
	}
	run.ShortTermMem.CancelSessionRuns(id)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop failed to cancel queued loopback")
	}
	if client.calls != 0 {
		t.Fatal("canceled loopback reached provider")
	}
}

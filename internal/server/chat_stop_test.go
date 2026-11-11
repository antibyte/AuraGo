package server

import (
	"context"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestChatStopBypassesPendingQuestionAndActiveTurn(t *testing.T) {
	s := newOperatorCommandTestServer(t)
	active, release, err := s.ShortTermMem.BeginSessionTurn(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	tools.RegisterQuestion("default", &tools.PendingQuestion{Question: "Continue?", AllowFreeText: true})
	defer tools.CancelQuestion("default")
	done := make(chan struct{})
	go func() {
		defer close(done)
		postChatCommand(t, s, "/stop", nil)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stop waited for the active turn")
	}
	if active.Err() != context.Canceled {
		t.Fatal("active run was not cancelled")
	}
	if tools.HasPendingQuestion("default") {
		t.Fatal("stop was consumed as a question answer")
	}
}

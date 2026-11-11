package whatsapp

import (
	"context"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestPipelineStopBypassesQuestionAndTurnGate(t *testing.T) {
	f := newPipelineFixture(t, dedicatedPipelineConfig())
	active, release, err := f.stm.BeginSessionTurn(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	tools.RegisterQuestion("default", &tools.PendingQuestion{Question: "Continue?", AllowFreeText: true})
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.p.handle(context.Background(), ownerText("stop-fixture", "/stop"))
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stop waited behind the running turn")
	}
	if active.Err() != context.Canceled {
		t.Fatal("active run was not cancelled")
	}
	if tools.HasPendingQuestion("default") {
		t.Fatal("stop left a question pending")
	}
	if len(f.callList()) != 0 {
		t.Fatal("stop started a model request")
	}
	if sent := sentTexts(f.fake); len(sent) != 1 {
		t.Fatalf("stop acknowledgements = %d, want 1", len(sent))
	}
}

package sipphone

import (
	"context"
	"testing"
	"time"

	"github.com/emiago/diago/diagotest"
	"github.com/emiago/sipgo/sip"
)

func TestManualInboundCallEndsAtRingTimeout(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Inbound.Route = "manual"
	cfg.Permissions.AnswerInbound = true
	cfg.Inbound.TrustedPeerCIDRs = []string{"192.0.2.10"}
	cfg.Inbound.AllowedCallers = []string{"alice"}
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.mu.Lock()
	manager.cfg.Inbound.RingTimeoutSeconds = 1
	manager.rootCtx = context.Background()
	manager.mu.Unlock()

	req, err := diagotest.NewRequest(sip.INVITE, sip.Uri{Scheme: "sip", User: "aurago", Host: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	req.SetSource("192.0.2.10:5060")
	req.From().Address.User = "alice"
	dialog, recorder, err := diagotest.NewDialogServerSession(req)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	terminate := time.AfterFunc(1200*time.Millisecond, recorder.Terminate)
	defer terminate.Stop()
	go func() {
		manager.handleIncoming(dialog)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("manual inbound call did not reach its ring timeout")
	}
	responses := recorder.Result()
	if len(responses) == 0 || responses[len(responses)-1].StatusCode != sip.StatusTemporarilyUnavailable {
		t.Fatalf("ring-timeout responses = %#v", responses)
	}
	calls, err := manager.ListCalls(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].EndReason != "ring_timeout" {
		t.Fatalf("ring-timeout history = %#v", calls)
	}
}

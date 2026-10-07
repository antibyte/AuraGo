package sipphone

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/voice"
	"github.com/emiago/diago"
)

type gatedAdmissionStore struct {
	callStore
	started, release chan struct{}
}

func (s *gatedAdmissionStore) AdmitAgentOutbound(ctx context.Context, record CallRecord, start, end time.Time, limit int) (int, bool, error) {
	close(s.started)
	select {
	case <-s.release:
	case <-ctx.Done():
		return 0, false, ctx.Err()
	}
	return s.callStore.(interface {
		AdmitAgentOutbound(context.Context, CallRecord, time.Time, time.Time, int) (int, bool, error)
	}).AdmitAgentOutbound(ctx, record, start, end, limit)
}

func TestOutboundAdmissionDoesNotHoldManagerLockAndRejectsChangedConfig(t *testing.T) {
	cfg := validTestSIPConfig()
	var store *gatedAdmissionStore
	manager, err := NewManager(cfg, t.TempDir(), readyTestBackendFactory, nil, nil, func(m *Manager) {
		store = &gatedAdmissionStore{callStore: m.store, started: make(chan struct{}), release: make(chan struct{})}
		m.store = store
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	var once sync.Once
	release := func() { once.Do(func() { close(store.release) }) }
	defer release()
	manager.endpoint = &diago.Diago{}
	manager.rootCtx = context.Background()
	result := make(chan error, 1)
	go func() { _, err := manager.Dial(context.Background(), "sip:alice@example.com"); result <- err }()
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("admission did not start")
	}
	statusRead := make(chan struct{})
	go func() { manager.Status(); close(statusRead) }()
	select {
	case <-statusRead:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("database admission holds manager lock")
	}
	if _, err := manager.Dial(context.Background(), "sip:alice@example.com"); !errors.Is(err, ErrBusy) {
		t.Fatalf("preparation reservation lost: %v", err)
	}
	next := cfg
	next.Voice.AgentProviderID = "changed"
	manager.UpdateAgentConfig(next)
	release()
	select {
	case err := <-result:
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("stale admission proceeded: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("admission did not finish")
	}
	if manager.Status().ActiveCall != nil {
		t.Fatal("stale admission created call")
	}
	calls, err := manager.ListCalls(context.Background(), 10)
	if err != nil || len(calls) != 1 || calls[0].EndReason != "admission_cancelled" {
		t.Fatalf("cancelled attempt not finalized: %v", err)
	}
}

type drainingCallSession struct {
	started, release chan struct{}
	finished         atomic.Bool
}

func (*drainingCallSession) Interrupt()                      {}
func (*drainingCallSession) Events() <-chan voice.VoiceEvent { return nil }
func (s *drainingCallSession) Close() error {
	close(s.started)
	<-s.release
	s.finished.Store(true)
	return nil
}

func TestFinishCallJoinsVoiceBeforeSessionCleanup(t *testing.T) {
	backend := &drainingCallSession{started: make(chan struct{}), release: make(chan struct{})}
	cleaned := make(chan struct{}, 1)
	manager, err := NewManager(validTestSIPConfig(), t.TempDir(), nil, nil, nil, WithCallFinishedHook(func(CallRecord, bool) {
		if !backend.finished.Load() {
			t.Error("cleanup preceded final transcript producer")
		}
		cleaned <- struct{}{}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	var once sync.Once
	release := func() { once.Do(func() { close(backend.release) }) }
	defer release()
	manager.rootCtx = context.Background()
	manager.mu.Lock()
	call := manager.newActiveCallLocked("outbound", "redacted")
	call.backend = backend
	manager.mu.Unlock()
	go manager.finishCall(call, "local_hangup")
	select {
	case <-backend.started:
	case <-time.After(time.Second):
		t.Fatal("backend not closed")
	}
	select {
	case <-cleaned:
		t.Fatal("cleanup ran while producer was active")
	default:
	}
	if manager.Status().ActiveCall == nil {
		t.Fatal("call released before producer joined")
	}
	release()
	select {
	case <-call.done:
	case <-time.After(time.Second):
		t.Fatal("call did not finish")
	}
	select {
	case <-cleaned:
	default:
		t.Fatal("missing final cleanup")
	}
}

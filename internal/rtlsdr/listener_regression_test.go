package rtlsdr

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type auditStreamBody struct {
	closed chan struct{}
	once   sync.Once
}

func (b *auditStreamBody) Read([]byte) (int, error) { <-b.closed; return 0, io.EOF }
func (b *auditStreamBody) Close() error             { b.once.Do(func() { close(b.closed) }); return nil }

type leaseTestReceiver struct {
	testReceiver
	streamMu sync.Mutex
	bodies   []*auditStreamBody
	contexts []context.Context
}

// Deliberately ignore cancellation: the service must close an already open body.
func (f *leaseTestReceiver) Stream(ctx context.Context) (io.ReadCloser, error) {
	f.streamMu.Lock()
	defer f.streamMu.Unlock()
	body := &auditStreamBody{closed: make(chan struct{})}
	f.bodies = append(f.bodies, body)
	f.contexts = append(f.contexts, ctx)
	return body, nil
}

func TestRTLSDRStreamLifetimeFollowsExactLease(t *testing.T) {
	for _, action := range []string{"stop", "expire", "request", "readonly", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			receiver := &leaseTestReceiver{}
			var now atomic.Int64
			now.Store(time.Now().UnixNano())
			var readonly atomic.Bool
			service, err := New(Options{Directory: t.TempDir(), Backend: receiver, Now: func() time.Time { return time.Unix(0, now.Load()) }, Policy: func() Policy { return Policy{Enabled: true, ReadOnly: readonly.Load()} }})
			if err != nil {
				t.Fatal(err)
			}
			if action != "shutdown" {
				defer service.Close()
			}
			for _, client := range []string{"old", "other"} {
				if err := service.Tune(context.Background(), client, DefaultTuning()); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream, err := service.Stream(ctx, "old")
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			readDone := make(chan struct{})
			go func() { defer close(readDone); _, _ = stream.Read(make([]byte, 1)) }()
			switch action {
			case "stop":
				if err := service.Stop(context.Background(), "old"); err != nil {
					t.Fatal(err)
				}
				if err := service.Tune(context.Background(), "old", DefaultTuning()); err != nil {
					t.Fatal(err)
				}
			case "expire":
				now.Add(int64(LeaseTTL / 2))
				if err := service.Heartbeat("other"); err != nil {
					t.Fatal(err)
				}
				now.Add(int64(LeaseTTL/2 + time.Millisecond))
				if err := service.Heartbeat("old"); !errors.Is(err, ErrNotFound) {
					t.Fatal("expired lease revived")
				}
				service.tick()
			case "request":
				cancel()
			case "readonly":
				readonly.Store(true)
				service.tick()
			case "shutdown":
				if err := service.Close(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-readDone:
			case <-time.After(time.Second):
				t.Fatal("expired/cancelled stream stayed open")
			}
			select {
			case <-receiver.contexts[0].Done():
			case <-time.After(time.Second):
				t.Fatal("backend request not cancelled")
			}
			if action == "stop" || action == "expire" {
				receiver.mu.Lock()
				stops := receiver.stops
				receiver.mu.Unlock()
				if stops != 0 {
					t.Fatal("another listener's receiver was stopped")
				}
			}
			if action == "stop" {
				fresh, err := service.Stream(context.Background(), "old")
				if err != nil {
					t.Fatal(err)
				}
				defer fresh.Close()
				select {
				case <-receiver.contexts[1].Done():
					t.Fatal("new lease inherited stale cancellation")
				default:
				}
			}
		})
	}
}

func TestRTLSDRUnknownStopCannotAffectReceiver(t *testing.T) {
	receiver := &testReceiver{}
	service := newTestService(t, receiver, enabled, nil)
	if err := service.Stop(context.Background(), "unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown stop: %v", err)
	}
	receiver.mu.Lock()
	defer receiver.mu.Unlock()
	if receiver.stops != 0 {
		t.Fatal("unknown client stopped receiver")
	}
}

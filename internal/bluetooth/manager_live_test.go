package bluetooth

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type fakeAgentHost struct {
	mu       sync.Mutex
	acquired int
	defaults int
	released int
	pairHook func(context.Context) error
}

func (h *fakeAgentHost) Acquire(_ context.Context, asDefault bool) (func(), error) {
	h.mu.Lock()
	h.acquired++
	if asDefault {
		h.defaults++
	}
	h.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			h.released++
			h.mu.Unlock()
		})
	}, nil
}

func (h *fakeAgentHost) Pair(ctx context.Context, _ string) error {
	if h.pairHook != nil {
		return h.pairHook(ctx)
	}
	return nil
}

func (h *fakeAgentHost) Close() error { return nil }

func (h *fakeAgentHost) counts() (int, int, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.acquired, h.defaults, h.released
}

func newLiveTestManager(t *testing.T, tree objectTree) (*Manager, *fakeBus, *fakeAgentHost) {
	t.Helper()
	bus := newFakeBus(tree)
	host := &fakeAgentHost{}
	manager := newTestManager(&fakeAdapter{})
	manager.broker = newInteractionBroker()
	manager.live = startTestLive(t, bus)
	manager.broker.onChange = manager.live.notifyInteraction
	manager.newAgents = func(*interactionBroker, deviceResolver, *slog.Logger) (agentHost, error) { return host, nil }
	return manager, bus, host
}

func TestManagerStatusFollowsTheLiveSession(t *testing.T) {
	manager, bus, _ := newLiveTestManager(t, testObjectTree(false))
	status := manager.Status()
	if !status.Present || status.Usable || status.Adapter.Powered {
		t.Fatalf("powered-off adapter status = %+v", status)
	}
	if !manager.Present() {
		t.Fatal("powered-off adapter must still be present")
	}
	if _, _, err := manager.requireUsable(); ErrorCode(err) != ErrorPoweredOff {
		t.Fatalf("requireUsable = %v, want powered off", err)
	}
	bus.events <- objectEvent{Kind: objectChanged, Path: testAdapterPath, Interface: bluezAdapterInterface, Changed: map[string]interface{}{"Powered": true, "PowerState": "on"}}
	waitFor(t, "usable", func() bool { return manager.Status().Usable })
	devices, err := manager.List(context.Background())
	if err != nil || len(devices) != 1 {
		t.Fatalf("List = %+v, %v", devices, err)
	}
}

func TestManagerPresentRespectsConfiguration(t *testing.T) {
	manager, _, _ := newLiveTestManager(t, testObjectTree(true))
	manager.Configure(Options{Enabled: false})
	waitFor(t, "absent when disabled", func() bool { return !manager.Present() })
	manager.Configure(Options{Enabled: true})
	waitFor(t, "present when enabled", manager.Present)
}

func TestManagerSnapshotFallsBackWithoutLiveSession(t *testing.T) {
	manager := newTestManager(&fakeAdapter{devices: []Device{{Address: testDeviceAddress, Paired: true}}})
	snapshot := manager.Snapshot(context.Background())
	if len(snapshot.Devices) != 1 || snapshot.Devices[0].Address != testDeviceAddress {
		t.Fatalf("fallback snapshot = %+v", snapshot)
	}
}

func TestManagerSubscribeWithoutLiveSessionNeverBlocks(t *testing.T) {
	manager := newTestManager(&fakeAdapter{})
	changes, cancel := manager.Subscribe()
	defer cancel()
	select {
	case <-changes:
		t.Fatal("unexpected change")
	case <-time.After(10 * time.Millisecond):
	}
}

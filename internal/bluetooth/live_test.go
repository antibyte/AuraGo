package bluetooth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testAdapterPath   = "/org/bluez/hci0"
	testDevicePath    = "/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF"
	testDeviceAddress = "AA:BB:CC:DD:EE:FF"
)

type fakeBus struct {
	mu        sync.Mutex
	initial   objectTree
	events    chan objectEvent
	calls     []string
	blockCall map[string]chan struct{}
	callErr   map[string]error
	watchErr  error
	closed    int
}

func newFakeBus(initial objectTree) *fakeBus {
	return &fakeBus{initial: initial, events: make(chan objectEvent, 64), blockCall: map[string]chan struct{}{}, callErr: map[string]error{}}
}

func (b *fakeBus) Watch(ctx context.Context, emit func(objectEvent)) error {
	b.mu.Lock()
	initial, watchErr := b.initial, b.watchErr
	b.mu.Unlock()
	if watchErr != nil {
		return watchErr
	}
	emit(objectEvent{Kind: objectsReset, Objects: initial})
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-b.events:
			emit(event)
		}
	}
}

func (b *fakeBus) record(entry string) {
	b.mu.Lock()
	b.calls = append(b.calls, entry)
	b.mu.Unlock()
}

func (b *fakeBus) has(entry string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, call := range b.calls {
		if call == entry {
			return true
		}
	}
	return false
}

func (b *fakeBus) Call(ctx context.Context, path, method string) error {
	b.record(path + " " + method)
	b.mu.Lock()
	gate, err := b.blockCall[method], b.callErr[method]
	b.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (b *fakeBus) RemoveDevice(_ context.Context, adapterPath, devicePath string) error {
	b.record(adapterPath + " RemoveDevice " + devicePath)
	return nil
}

func (b *fakeBus) SetProperty(_ context.Context, path, _, name string, value interface{}) error {
	b.record(fmt.Sprintf("%s %s=%v", path, name, value))
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.callErr["Set "+name]
}

func (b *fakeBus) Close() error {
	b.mu.Lock()
	b.closed++
	b.mu.Unlock()
	return nil
}

func testObjectTree(powered bool) objectTree {
	state := "off"
	if powered {
		state = "on"
	}
	return objectTree{
		testAdapterPath: {bluezAdapterInterface: {"Address": "00:11:22:33:44:55", "Alias": "aurago", "Powered": powered, "PowerState": state, "Pairable": false, "Discoverable": false}},
		testDevicePath: {
			bluezDeviceInterface:  {"Address": testDeviceAddress, "Alias": "Headphones", "Name": "Headphones", "Adapter": testAdapterPath, "Paired": true, "Trusted": true, "Icon": "audio-headphones", "UUIDs": []string{"0000110b-0000-1000-8000-00805f9b34fb"}},
			bluezBatteryInterface: {"Percentage": uint8(80)},
		},
	}
}

func newTestLive(dial busDialer) *liveSession {
	live := newLiveSession(dial, slog.Default())
	live.debounce = time.Millisecond
	live.backoffInitial = 5 * time.Millisecond
	live.backoffMax = 20 * time.Millisecond
	return live
}

func startTestLive(t *testing.T, bus *fakeBus) *liveSession {
	t.Helper()
	live := newTestLive(func() (bluezBus, error) { return bus, nil })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); live.stop(time.Second) })
	live.start(ctx, true)
	waitFor(t, "live session observes BlueZ", live.active)
	return live
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting until %s", what)
}

func TestLiveSessionBuildsSnapshotFromReset(t *testing.T) {
	live := startTestLive(t, newFakeBus(testObjectTree(true)))
	waitFor(t, "adapter present", func() bool { return live.snapshot(time.Now()).Present })
	snapshot := live.snapshot(time.Now())
	if snapshot.Adapter.Name != "aurago" || !snapshot.Adapter.Powered || snapshot.Revision == 0 {
		t.Fatalf("snapshot adapter = %+v rev %d", snapshot.Adapter, snapshot.Revision)
	}
	if len(snapshot.Devices) != 1 || snapshot.Devices[0].Type != "headphones" || snapshot.Devices[0].Battery == nil || !snapshot.Devices[0].Audio {
		t.Fatalf("snapshot devices = %+v", snapshot.Devices)
	}
}

func TestLiveSessionReportsAdapterHotplugAsCapabilityChange(t *testing.T) {
	bus := newFakeBus(objectTree{})
	live := startTestLive(t, bus)
	if live.snapshot(time.Now()).Present {
		t.Fatal("no adapter must not be present")
	}
	changes, cancel := live.subscribe()
	defer cancel()
	bus.events <- objectEvent{Kind: objectAdded, Path: testAdapterPath, Interfaces: map[string]map[string]interface{}{bluezAdapterInterface: {"Powered": true}}}
	expectChange(t, changes, func(change Change) bool { return change.CapabilityChanged && change.Present })
	bus.events <- objectEvent{Kind: objectRemoved, Path: testAdapterPath, RemovedInterfaces: []string{bluezAdapterInterface}}
	expectChange(t, changes, func(change Change) bool { return change.CapabilityChanged && !change.Present })
}

func expectChange(t *testing.T, changes <-chan Change, match func(Change) bool) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case change := <-changes:
			if match(change) {
				return
			}
		case <-deadline:
			t.Fatal("expected change did not arrive")
		}
	}
}

func TestLiveSessionTreatsHardBlockAsAbsent(t *testing.T) {
	tree := testObjectTree(false)
	tree[testAdapterPath][bluezAdapterInterface]["PowerState"] = powerStateOffBlocked
	bus := newFakeBus(tree)
	live := startTestLive(t, bus)
	snapshot := live.snapshot(time.Now())
	if snapshot.Present || !strings.Contains(snapshot.Reason, "hardware switch") {
		t.Fatalf("hard-blocked adapter = present:%v reason:%q", snapshot.Present, snapshot.Reason)
	}
	bus.events <- objectEvent{Kind: objectChanged, Path: testAdapterPath, Interface: bluezAdapterInterface, Changed: map[string]interface{}{"PowerState": "off"}}
	waitFor(t, "unblocked adapter present", func() bool { return live.snapshot(time.Now()).Present })
}

func TestLiveSessionCoalescesBursts(t *testing.T) {
	bus := newFakeBus(testObjectTree(true))
	live := newTestLive(func() (bluezBus, error) { return bus, nil })
	live.debounce = 40 * time.Millisecond
	changes, cancel := live.subscribe()
	defer cancel()
	ctx, stop := context.WithCancel(context.Background())
	defer func() { stop(); live.stop(time.Second) }()
	live.start(ctx, true)
	for i := 0; i < 20; i++ {
		bus.events <- objectEvent{Kind: objectChanged, Path: testDevicePath, Interface: bluezDeviceInterface, Changed: map[string]interface{}{"RSSI": int16(-40 - i)}}
	}
	time.Sleep(150 * time.Millisecond)
	received := 0
	var last Change
drain:
	for {
		select {
		case last = <-changes:
			received++
		default:
			break drain
		}
	}
	if received == 0 || received > 2 {
		t.Fatalf("received %d changes for one burst, want 1 or 2", received)
	}
	if last.Revision != live.snapshot(time.Now()).Revision {
		t.Fatalf("last change revision %d != snapshot revision %d", last.Revision, live.snapshot(time.Now()).Revision)
	}
}

func TestLiveSessionReconnectsAfterWatchFailure(t *testing.T) {
	failing := newFakeBus(testObjectTree(true))
	failing.watchErr = errors.New("bus dropped")
	working := newFakeBus(testObjectTree(true))
	var mu sync.Mutex
	dials := 0
	live := newTestLive(func() (bluezBus, error) {
		mu.Lock()
		defer mu.Unlock()
		dials++
		if dials == 1 {
			return failing, nil
		}
		return working, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); live.stop(time.Second) }()
	live.start(ctx, true)
	waitFor(t, "reconnected and present", func() bool { return live.snapshot(time.Now()).Present })
	mu.Lock()
	defer mu.Unlock()
	if dials < 2 || failing.closed == 0 {
		t.Fatalf("dials=%d failing closed=%d", dials, failing.closed)
	}
}

func TestLiveSessionDisableAndEnable(t *testing.T) {
	live := startTestLive(t, newFakeBus(testObjectTree(true)))
	waitFor(t, "present", func() bool { return live.snapshot(time.Now()).Present })
	live.setEnabled(false)
	waitFor(t, "disabled", func() bool {
		snapshot := live.snapshot(time.Now())
		return !snapshot.Present && strings.Contains(snapshot.Reason, "disabled")
	})
	live.setEnabled(true)
	waitFor(t, "present again", func() bool { return live.snapshot(time.Now()).Present })
}

func TestLiveSessionLostEventFailsRunningOperations(t *testing.T) {
	bus := newFakeBus(testObjectTree(true))
	live := startTestLive(t, bus)
	if err := live.beginOperation(testDeviceAddress, "connecting"); err != nil {
		t.Fatal(err)
	}
	bus.events <- objectEvent{Kind: objectsLost, Reason: "BlueZ stopped."}
	waitFor(t, "lost", func() bool { return !live.snapshot(time.Now()).Present })
	live.mu.Lock()
	op := live.operations[testDeviceAddress]
	live.mu.Unlock()
	if op.Kind != "" || op.Error != ErrorUnavailable {
		t.Fatalf("operation after loss = %+v", op)
	}
}

func TestLiveSessionOperationsAreExclusivePerDevice(t *testing.T) {
	live := startTestLive(t, newFakeBus(testObjectTree(true)))
	if err := live.beginOperation(testDeviceAddress, "connecting"); err != nil {
		t.Fatal(err)
	}
	if err := live.beginOperation(testDeviceAddress, "removing"); ErrorCode(err) != ErrorOperationBusy {
		t.Fatalf("second operation error = %v, want busy", err)
	}
	live.endOperation(testDeviceAddress, codedError(ErrorDeviceUnreachable, "no answer", nil))
	device := live.snapshot(time.Now()).Devices[0]
	if device.Operation != "" || device.Error != ErrorDeviceUnreachable {
		t.Fatalf("device after failure = %+v", device)
	}
	if err := live.beginOperation(testDeviceAddress, "connecting"); err != nil {
		t.Fatalf("operation after failure must be allowed: %v", err)
	}
	live.endOperation(testDeviceAddress, nil)
	if device := live.snapshot(time.Now()).Devices[0]; device.Operation != "" || device.Error != "" {
		t.Fatalf("device after success = %+v", device)
	}
}

func TestDeliverLatestMergesFlags(t *testing.T) {
	ch := make(chan Change, 1)
	deliverLatest(ch, Change{Revision: 1, CapabilityChanged: true, InteractionID: "a"})
	deliverLatest(ch, Change{Revision: 2})
	got := <-ch
	if got.Revision != 2 || !got.CapabilityChanged || got.InteractionID != "a" {
		t.Fatalf("merged change = %+v", got)
	}
}

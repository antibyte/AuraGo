package bluetooth

import (
	"context"
	"testing"
	"time"
)

func TestDeviceOperationRunsAsyncAndReportsProgress(t *testing.T) {
	manager, bus, _ := newLiveTestManager(t, testObjectTree(true))
	gate := make(chan struct{})
	bus.mu.Lock()
	bus.blockCall[bluezDeviceInterface+".Connect"] = gate
	bus.mu.Unlock()
	ctx := context.Background()
	if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "connect", Address: testDeviceAddress}, false); err != nil {
		t.Fatalf("async connect: %v", err)
	}
	waitFor(t, "connecting shown", func() bool { return manager.Snapshot(ctx).Devices[0].Operation == "connecting" })
	if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "disconnect", Address: testDeviceAddress}, false); ErrorCode(err) != ErrorOperationBusy {
		t.Fatalf("second operation = %v, want busy", err)
	}
	close(gate)
	waitFor(t, "operation finished", func() bool {
		device := manager.Snapshot(ctx).Devices[0]
		return device.Operation == "" && device.Error == ""
	})
	if !bus.has(testDevicePath + " " + bluezDeviceInterface + ".Connect") {
		t.Fatal("Connect was not called on the device")
	}
}

func TestDeviceOperationErrorsBecomeCodes(t *testing.T) {
	manager, bus, _ := newLiveTestManager(t, testObjectTree(true))
	bus.mu.Lock()
	bus.callErr[bluezDeviceInterface+".Connect"] = &busError{Name: "org.bluez.Error.Failed", Message: "Page Timeout"}
	bus.mu.Unlock()
	err := manager.RunDeviceOperation(context.Background(), ActorOperator, DeviceRequest{Operation: "connect", Address: testDeviceAddress}, true)
	if ErrorCode(err) != ErrorDeviceUnreachable {
		t.Fatalf("connect error = %v, want unreachable", err)
	}
	if device := manager.Snapshot(context.Background()).Devices[0]; device.Error != ErrorDeviceUnreachable {
		t.Fatalf("device error = %q", device.Error)
	}
}

func TestRemoveTrustAndPowerUseBlueZ(t *testing.T) {
	manager, bus, _ := newLiveTestManager(t, testObjectTree(true))
	ctx := context.Background()
	for _, operation := range []string{"untrust", "remove"} {
		if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: operation, Address: testDeviceAddress}, true); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
	}
	if !bus.has(testDevicePath + " Trusted=false") {
		t.Fatal("untrust did not set Trusted=false")
	}
	if !bus.has(testAdapterPath + " RemoveDevice " + testDevicePath) {
		t.Fatal("remove did not call RemoveDevice")
	}
	if err := manager.SetPowered(ctx, ActorOperator, false); err != nil {
		t.Fatalf("SetPowered: %v", err)
	}
	if !bus.has(testAdapterPath + " Powered=false") {
		t.Fatal("power off did not set Powered=false")
	}
}

func TestReadOnlyAppliesOnlyToAgentDeviceOperations(t *testing.T) {
	manager, _, _ := newLiveTestManager(t, testObjectTree(true))
	manager.Configure(Options{Enabled: true, ReadOnly: true, ScanTimeout: time.Second})
	ctx := context.Background()
	if err := manager.RunDeviceOperation(ctx, ActorAgent, DeviceRequest{Operation: "connect", Address: testDeviceAddress}, true); ErrorCode(err) != ErrorReadOnly {
		t.Fatalf("agent connect = %v, want read-only", err)
	}
	if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "connect", Address: testDeviceAddress}, true); err != nil {
		t.Fatalf("operator connect = %v", err)
	}
	if err := manager.RunDeviceOperation(ctx, ActorAgent, DeviceRequest{Operation: "pair", Address: testDeviceAddress, Interactive: true}, true); ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("agent interactive pair = %v, want invalid argument", err)
	}
}

func TestPoweredOffAdapterRefusesDeviceOperations(t *testing.T) {
	manager, _, _ := newLiveTestManager(t, testObjectTree(false))
	ctx := context.Background()
	if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "connect", Address: testDeviceAddress}, true); ErrorCode(err) != ErrorPoweredOff {
		t.Fatalf("connect while off = %v, want powered off", err)
	}
	if err := manager.SetPowered(ctx, ActorOperator, true); err != nil {
		t.Fatalf("power on while off = %v", err)
	}
}

func TestDiscoveryStartsStopsAndExpires(t *testing.T) {
	manager, bus, _ := newLiveTestManager(t, testObjectTree(true))
	ctx := context.Background()
	state, err := manager.StartDiscovery(ctx, ActorOperator, 80*time.Millisecond)
	if err != nil || !state.Active {
		t.Fatalf("StartDiscovery = %+v, %v", state, err)
	}
	if !bus.has(testAdapterPath + " " + bluezAdapterInterface + ".StartDiscovery") {
		t.Fatal("StartDiscovery not called")
	}
	if !manager.Snapshot(ctx).Discovery.Active {
		t.Fatal("snapshot does not show discovery")
	}
	waitFor(t, "discovery stops on its own", func() bool {
		return bus.has(testAdapterPath+" "+bluezAdapterInterface+".StopDiscovery") && !manager.Snapshot(ctx).Discovery.Active
	})
}

func TestDiscoveryKeepsTheLongerRunningScan(t *testing.T) {
	manager, _, _ := newLiveTestManager(t, testObjectTree(true))
	ctx := context.Background()
	long, err := manager.StartDiscovery(ctx, ActorOperator, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	short, err := manager.StartDiscovery(ctx, ActorAgent, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !short.EndsAt.Equal(long.EndsAt) {
		t.Fatalf("short scan shortened the running one: %v vs %v", short.EndsAt, long.EndsAt)
	}
	manager.stopDiscoveryQuietly(ctx)
}

func TestDiscoverableWindowClaimsDefaultAgentAndRestores(t *testing.T) {
	manager, bus, host := newLiveTestManager(t, testObjectTree(true))
	ctx := context.Background()
	if _, err := manager.SetDiscoverable(ctx, ActorAgent, 0); ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("agent discoverable = %v, want invalid argument", err)
	}
	state, err := manager.SetDiscoverable(ctx, ActorOperator, 0)
	if err != nil || !state.Active || state.RemainingSeconds != 180 {
		t.Fatalf("SetDiscoverable = %+v, %v", state, err)
	}
	for _, want := range []string{"Pairable=true", "DiscoverableTimeout=180", "Discoverable=true"} {
		if !bus.has(testAdapterPath + " " + want) {
			t.Fatalf("missing %s", want)
		}
	}
	if acquired, defaults, _ := host.counts(); acquired != 1 || defaults != 1 {
		t.Fatalf("agent acquired=%d defaults=%d", acquired, defaults)
	}
	if !manager.Snapshot(ctx).Discoverable.Active {
		t.Fatal("snapshot does not show the window")
	}
	if err := manager.StopDiscoverable(ctx, ActorOperator); err != nil {
		t.Fatal(err)
	}
	if !bus.has(testAdapterPath+" Discoverable=false") || !bus.has(testAdapterPath+" Pairable=false") {
		t.Fatal("window end did not restore the adapter")
	}
	if _, _, released := host.counts(); released != 1 {
		t.Fatalf("agent released %d times", released)
	}
	if manager.Snapshot(ctx).Discoverable.Active {
		t.Fatal("window still shown after stop")
	}
}

func TestClampDurations(t *testing.T) {
	if got := clampDiscoverable(0); got != 180*time.Second {
		t.Fatalf("default discoverable = %v", got)
	}
	if got := clampDiscoverable(30 * time.Second); got != 60*time.Second {
		t.Fatalf("short discoverable = %v", got)
	}
	if got := clampDiscoverable(time.Hour); got != 600*time.Second {
		t.Fatalf("long discoverable = %v", got)
	}
	if got := clampDiscovery(0, 10*time.Second); got != 10*time.Second {
		t.Fatalf("default discovery = %v", got)
	}
	if got := clampDiscovery(5*time.Minute, 10*time.Second); got != 60*time.Second {
		t.Fatalf("long discovery = %v", got)
	}
}

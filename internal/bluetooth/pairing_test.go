package bluetooth

import (
	"context"
	"testing"
)

func confirmingPairHook(manager *Manager) func(context.Context) error {
	return func(ctx context.Context) error {
		answer, err := manager.broker.ask(ctx, Interaction{Kind: InteractionConfirmPasskey, DeviceAddress: testDeviceAddress, Passkey: "482913"})
		if err != nil || !answer.accept {
			return &busError{Name: "org.bluez.Error.AuthenticationRejected"}
		}
		return nil
	}
}

func TestInteractivePairingAsksTheOperator(t *testing.T) {
	manager, bus, host := newLiveTestManager(t, testObjectTree(true))
	host.pairHook = confirmingPairHook(manager)
	ctx := context.Background()
	if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "pair", Address: testDeviceAddress, Interactive: true}, false); err != nil {
		t.Fatal(err)
	}
	var id string
	waitFor(t, "interaction in snapshot", func() bool { id = manager.Snapshot(ctx).InteractionID; return id != "" })
	view, err := manager.Interaction(id)
	if err != nil || view.Passkey != "482913" || view.Kind != InteractionConfirmPasskey {
		t.Fatalf("interaction = %+v, %v", view, err)
	}
	if err := manager.AnswerInteraction(id, true, ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "paired and trusted", func() bool { return bus.has(testDevicePath + " Trusted=true") })
	waitFor(t, "agent released", func() bool { _, _, released := host.counts(); return released == 1 })
	if acquired, defaults, _ := host.counts(); acquired != 1 || defaults != 0 {
		t.Fatalf("outgoing pairing must not claim the default agent: acquired=%d defaults=%d", acquired, defaults)
	}
	waitFor(t, "interaction cleared", func() bool { return manager.Snapshot(ctx).InteractionID == "" })
}

func TestRejectedInteractivePairingReportsCode(t *testing.T) {
	manager, _, host := newLiveTestManager(t, testObjectTree(true))
	host.pairHook = confirmingPairHook(manager)
	ctx := context.Background()
	if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "pair", Address: testDeviceAddress, Interactive: true}, false); err != nil {
		t.Fatal(err)
	}
	var id string
	waitFor(t, "interaction", func() bool { id = manager.Snapshot(ctx).InteractionID; return id != "" })
	if err := manager.AnswerInteraction(id, false, ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "rejection code on device", func() bool { return manager.Snapshot(ctx).Devices[0].Error == ErrorPairingRejected })
}

func TestNonInteractivePairingNeverUsesTheAgentHost(t *testing.T) {
	manager, _, host := newLiveTestManager(t, testObjectTree(true))
	adapter := manager.adapter.(*fakeAdapter)
	if err := manager.RunDeviceOperation(context.Background(), ActorOperator, DeviceRequest{Operation: "pair", Address: testDeviceAddress}, true); err != nil {
		t.Fatal(err)
	}
	if adapter.paired != testDeviceAddress {
		t.Fatalf("legacy pairing not used: %q", adapter.paired)
	}
	if acquired, _, _ := host.counts(); acquired != 0 {
		t.Fatalf("agent host acquired %d times for non-interactive pairing", acquired)
	}
}

//go:build linux

package bluetooth

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	foreignDevicePath    = dbus.ObjectPath("/org/bluez/hci0/dev_11_22_33_44_55_66")
	foreignDeviceAddress = "11:22:33:44:55:66"
)

type staticResolver map[string]Device

func (r staticResolver) deviceByPath(path string) (Device, bool) {
	device, ok := r[path]
	return device, ok
}

// isDeviceMismatch reports whether err is the rejection both agents send for a
// device other than the one being paired.
func isDeviceMismatch(err *dbus.Error) bool {
	return err != nil && err.Name == "org.bluez.Error.Rejected" &&
		len(err.Body) == 1 && err.Body[0] == "PAIRING_DEVICE_MISMATCH"
}

// countOpenedInteractions counts every question the broker opens, including
// one that opened and closed again before the caller looks.
func countOpenedInteractions(broker *interactionBroker) *atomic.Int32 {
	opened := new(atomic.Int32)
	broker.onChange = func(_ string, isNew bool) {
		if isNew {
			opened.Add(1)
		}
	}
	return opened
}

// agentRequest is one Agent1 request that reaches the operator. Display
// requests are only shown; dropsForeign marks the one whose foreign request is
// dropped with a nil reply instead of rejected.
type agentRequest struct {
	name         string
	display      bool
	dropsForeign bool
	call         func(*interactiveAgent, dbus.ObjectPath) *dbus.Error
}

var interactiveAgentRequests = []agentRequest{
	{name: "RequestPinCode", call: func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { _, err := a.RequestPinCode(p); return err }},
	{name: "RequestPasskey", call: func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { _, err := a.RequestPasskey(p); return err }},
	{name: "RequestConfirmation", call: func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { return a.RequestConfirmation(p, 42) }},
	{name: "RequestAuthorization", call: func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { return a.RequestAuthorization(p) }},
	{name: "AuthorizeService", call: func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error {
		return a.AuthorizeService(p, "0000110b-0000-1000-8000-00805f9b34fb")
	}},
	{name: "DisplayPinCode", display: true, call: func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { return a.DisplayPinCode(p, "0000") }},
	{name: "DisplayPasskey", display: true, dropsForeign: true, call: func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error {
		return a.DisplayPasskey(p, 123456, 0)
	}},
}

// expectOperatorSees calls request for device and expects it to reach the
// operator: display requests are shown, answerable ones are accepted.
func expectOperatorSees(t *testing.T, broker *interactionBroker, agent *interactiveAgent, request agentRequest, device dbus.ObjectPath) {
	t.Helper()
	if request.display {
		if err := request.call(agent, device); err != nil {
			t.Fatalf("display request returned %v", err)
		}
		if broker.current() == "" {
			t.Fatal("display request was not shown to the operator")
		}
		broker.cancel()
		return
	}
	result := make(chan *dbus.Error, 1)
	go func() { result <- request.call(agent, device) }()
	id := waitForInteraction(t, broker)
	if err := broker.answer(id, true, "123456"); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if err := <-result; err != nil {
		t.Fatalf("accepted request returned %v", err)
	}
}

func TestInteractiveAgentRejectsForeignDevicesWhileBound(t *testing.T) {
	for _, request := range interactiveAgentRequests {
		t.Run(request.name, func(t *testing.T) {
			broker := newInteractionBroker()
			// A regression that asks the operator fails fast instead of waiting 20 s.
			broker.timeout = 50 * time.Millisecond
			opened := countOpenedInteractions(broker)
			agent := &interactiveAgent{broker: broker, devices: staticResolver{}}
			release, err := agent.bind(dbus.ObjectPath(testDevicePath))
			if err != nil {
				t.Fatalf("bind: %v", err)
			}
			defer release()
			reply := request.call(agent, foreignDevicePath)
			if request.dropsForeign && reply != nil {
				t.Fatalf("foreign device: reply = %v, want the request dropped (nil)", reply)
			}
			if !request.dropsForeign && !isDeviceMismatch(reply) {
				t.Fatalf("foreign device: reply = %#v, want org.bluez.Error.Rejected PAIRING_DEVICE_MISMATCH", reply)
			}
			if n := opened.Load(); n != 0 {
				t.Fatalf("the operator saw %d question(s) for a foreign device", n)
			}
		})
	}
}

func TestInteractiveAgentAsksTheOperatorForTheBoundDevice(t *testing.T) {
	for _, request := range interactiveAgentRequests {
		t.Run(request.name, func(t *testing.T) {
			broker := newInteractionBroker()
			agent := &interactiveAgent{broker: broker, devices: staticResolver{}}
			release, err := agent.bind(dbus.ObjectPath(testDevicePath))
			if err != nil {
				t.Fatalf("bind: %v", err)
			}
			defer release()
			expectOperatorSees(t, broker, agent, request, dbus.ObjectPath(testDevicePath))
		})
	}
}

func TestInteractiveAgentAsksTheOperatorForAnyDeviceWhenUnbound(t *testing.T) {
	for _, request := range interactiveAgentRequests {
		t.Run(request.name, func(t *testing.T) {
			broker := newInteractionBroker()
			agent := &interactiveAgent{broker: broker, devices: staticResolver{}}
			// A released binding must leave the discoverable-window behaviour intact.
			release, err := agent.bind(dbus.ObjectPath(testDevicePath))
			if err != nil {
				t.Fatalf("bind: %v", err)
			}
			release()
			expectOperatorSees(t, broker, agent, request, foreignDevicePath)
		})
	}
}

func TestInteractiveAgentBindsOneDeviceAtATime(t *testing.T) {
	agent := &interactiveAgent{broker: newInteractionBroker(), devices: staticResolver{}}
	answers := func(device dbus.ObjectPath) bool { return !agent.foreign(device) }
	release, err := agent.bind(dbus.ObjectPath(testDevicePath))
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if !answers(dbus.ObjectPath(testDevicePath)) || answers(foreignDevicePath) {
		t.Fatal("a bound agent must answer only the bound device")
	}
	_, err = agent.bind(foreignDevicePath)
	var busy *busError
	if !errors.As(err, &busy) || busy.Name != "org.bluez.Error.InProgress" {
		t.Fatalf("second device bind err = %v, want org.bluez.Error.InProgress", err)
	}
	if code := ErrorCode(mapBlueZError("pair", err)); code != ErrorOperationBusy {
		t.Fatalf("second device bind maps to %q, want %q", code, ErrorOperationBusy)
	}
	again, err := agent.bind(dbus.ObjectPath(testDevicePath))
	if err != nil {
		t.Fatalf("re-binding the bound device: %v", err)
	}
	again()
	if answers(foreignDevicePath) {
		t.Fatal("the binding ended while another holder still pairs the device")
	}
	release()
	release()
	again()
	if !answers(dbus.ObjectPath(testDevicePath)) || !answers(foreignDevicePath) {
		t.Fatal("a released agent must answer any device (discoverable window)")
	}
	other, err := agent.bind(foreignDevicePath)
	if err != nil {
		t.Fatalf("bind after release: %v", err)
	}
	if answers(dbus.ObjectPath(testDevicePath)) {
		t.Fatal("an agent bound to another device answered the previous one")
	}
	other()
	if !answers(dbus.ObjectPath(testDevicePath)) {
		t.Fatal("a released agent must answer any device")
	}
}

// bindingAgentHost pairs the way linuxAgentHost.Pair does: it binds the real
// interactive agent to the device and lets BlueZ ask that device's
// confirmation through it.
type bindingAgentHost struct {
	fakeAgentHost
	agent *interactiveAgent
}

func (h *bindingAgentHost) Pair(_ context.Context, devicePath string) error {
	release, err := h.agent.bind(dbus.ObjectPath(devicePath))
	if err != nil {
		return err
	}
	defer release()
	if reply := h.agent.RequestConfirmation(dbus.ObjectPath(devicePath), 42); reply != nil {
		return &busError{Name: reply.Name}
	}
	return nil
}

func TestBusyInteractivePairingLeavesTheRunningOneUntouched(t *testing.T) {
	tree := testObjectTree(true)
	tree[string(foreignDevicePath)] = map[string]map[string]interface{}{
		bluezDeviceInterface: {"Address": foreignDeviceAddress, "Alias": "Phone", "Adapter": testAdapterPath, "Paired": false},
	}
	manager, bus, _ := newLiveTestManager(t, tree)
	host := &bindingAgentHost{agent: &interactiveAgent{broker: manager.broker, devices: manager.live}}
	manager.newAgents = func(*interactionBroker, deviceResolver, *slog.Logger) (agentHost, error) { return host, nil }
	ctx := context.Background()
	if err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "pair", Address: testDeviceAddress, Interactive: true}, false); err != nil {
		t.Fatal(err)
	}
	first := waitForInteraction(t, manager.broker)
	if view, err := manager.Interaction(first); err != nil || view.Kind != InteractionConfirmPasskey || view.DeviceAddress != testDeviceAddress {
		t.Fatalf("running pairing question = %+v, %v", view, err)
	}

	err := manager.RunDeviceOperation(ctx, ActorOperator, DeviceRequest{Operation: "pair", Address: foreignDeviceAddress, Interactive: true}, true)
	if code := ErrorCode(err); code != ErrorOperationBusy {
		t.Fatalf("second pairing = %v (code %q), want %s", err, code, ErrorOperationBusy)
	}
	if got := manager.broker.current(); got != first {
		t.Fatalf("the busy pairing closed the running pairing's question: first=%s current=%q", first, got)
	}

	if err := manager.AnswerInteraction(first, true, ""); err != nil {
		t.Fatalf("accept the running pairing: %v", err)
	}
	waitFor(t, "running pairing trusted", func() bool { return bus.has(testDevicePath + " Trusted=true") })
	waitFor(t, "running pairing ends without error", func() bool {
		for _, device := range manager.Snapshot(ctx).Devices {
			if device.Address == testDeviceAddress {
				return device.Operation == "" && device.Error == ""
			}
		}
		return false
	})
}

func TestInteractiveAgentForwardsConfirmationToTheBroker(t *testing.T) {
	broker := newInteractionBroker()
	agent := &interactiveAgent{broker: broker, devices: staticResolver{testDevicePath: {Address: testDeviceAddress, Alias: "Pixel 9"}}}
	result := make(chan *dbus.Error, 1)
	go func() { result <- agent.RequestConfirmation(dbus.ObjectPath(testDevicePath), 42) }()
	id := waitForInteraction(t, broker)
	view, _ := broker.get(id)
	if view.Passkey != "000042" || view.DeviceName != "Pixel 9" {
		t.Fatalf("view = %+v", view)
	}
	_ = broker.answer(id, true, "")
	if err := <-result; err != nil {
		t.Fatalf("accepted confirmation returned %v", err)
	}
	go func() { result <- agent.RequestConfirmation(dbus.ObjectPath(testDevicePath), 7) }()
	id = waitForInteraction(t, broker)
	_ = broker.answer(id, false, "")
	if err := <-result; err == nil || err.Name != "org.bluez.Error.Rejected" {
		t.Fatalf("rejected confirmation returned %v", err)
	}
}

func TestInteractiveAgentReturnsEnteredPasskey(t *testing.T) {
	broker := newInteractionBroker()
	agent := &interactiveAgent{broker: broker, devices: staticResolver{}}
	type reply struct {
		value uint32
		err   *dbus.Error
	}
	result := make(chan reply, 1)
	go func() {
		value, err := agent.RequestPasskey(dbus.ObjectPath(testDevicePath))
		result <- reply{value, err}
	}()
	id := waitForInteraction(t, broker)
	_ = broker.answer(id, true, "012345")
	if got := <-result; got.err != nil || got.value != 12345 {
		t.Fatalf("RequestPasskey = %+v", got)
	}
}

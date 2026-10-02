//go:build linux

package bluetooth

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

type staticResolver map[string]Device

func (r staticResolver) deviceByPath(path string) (Device, bool) {
	device, ok := r[path]
	return device, ok
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

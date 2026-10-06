//go:build linux

package bluetooth

import (
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

const foreignDevicePath = dbus.ObjectPath("/org/bluez/hci0/dev_11_22_33_44_55_66")

type staticResolver map[string]Device

func (r staticResolver) deviceByPath(path string) (Device, bool) {
	device, ok := r[path]
	return device, ok
}

// assertNoInteraction fails if the broker opens a question within 100 ms.
func assertNoInteraction(t *testing.T, broker *interactionBroker) {
	t.Helper()
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if id := broker.current(); id != "" {
			t.Fatalf("the operator was asked (interaction %s)", id)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// interactiveAgentRequests calls every answerable Agent1 request for device.
var interactiveAgentRequests = []struct {
	name string
	call func(*interactiveAgent, dbus.ObjectPath) *dbus.Error
}{
	{"RequestPinCode", func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { _, err := a.RequestPinCode(p); return err }},
	{"RequestPasskey", func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { _, err := a.RequestPasskey(p); return err }},
	{"RequestConfirmation", func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { return a.RequestConfirmation(p, 42) }},
	{"RequestAuthorization", func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error { return a.RequestAuthorization(p) }},
	{"AuthorizeService", func(a *interactiveAgent, p dbus.ObjectPath) *dbus.Error {
		return a.AuthorizeService(p, "0000110b-0000-1000-8000-00805f9b34fb")
	}},
}

func TestInteractiveAgentRejectsForeignDevicesWhileBound(t *testing.T) {
	for _, request := range interactiveAgentRequests {
		t.Run(request.name, func(t *testing.T) {
			broker := newInteractionBroker()
			agent := &interactiveAgent{broker: broker, devices: staticResolver{}}
			release, err := agent.bind(dbus.ObjectPath(testDevicePath))
			if err != nil {
				t.Fatalf("bind: %v", err)
			}
			defer release()
			result := make(chan *dbus.Error, 1)
			go func() { result <- request.call(agent, foreignDevicePath) }()
			select {
			case err := <-result:
				if err == nil || err.Name != "org.bluez.Error.Rejected" {
					t.Fatalf("foreign device: err = %v, want org.bluez.Error.Rejected", err)
				}
			case <-time.After(time.Second):
				broker.cancel()
				t.Fatal("foreign device request was not rejected immediately")
			}
			assertNoInteraction(t, broker)
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
			result := make(chan *dbus.Error, 1)
			go func() { result <- request.call(agent, dbus.ObjectPath(testDevicePath)) }()
			id := waitForInteraction(t, broker)
			if err := broker.answer(id, true, "123456"); err != nil {
				t.Fatalf("answer: %v", err)
			}
			if err := <-result; err != nil {
				t.Fatalf("bound device: accepted request returned %v", err)
			}
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
			result := make(chan *dbus.Error, 1)
			go func() { result <- request.call(agent, foreignDevicePath) }()
			id := waitForInteraction(t, broker)
			if err := broker.answer(id, true, "123456"); err != nil {
				t.Fatalf("answer: %v", err)
			}
			if err := <-result; err != nil {
				t.Fatalf("unbound agent: accepted request returned %v", err)
			}
		})
	}
}

func TestInteractiveAgentBindsOneDeviceAtATime(t *testing.T) {
	agent := &interactiveAgent{broker: newInteractionBroker(), devices: staticResolver{}}
	bound := func() dbus.ObjectPath {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.bound
	}
	release, err := agent.bind(dbus.ObjectPath(testDevicePath))
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if got := bound(); got != dbus.ObjectPath(testDevicePath) {
		t.Fatalf("bound = %q after bind", got)
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
	if got := bound(); got != dbus.ObjectPath(testDevicePath) {
		t.Fatalf("bound = %q after one of two holders released, want %s", got, testDevicePath)
	}
	release()
	release()
	again()
	if got := bound(); got != "" {
		t.Fatalf("bound = %q after release, want empty", got)
	}
	other, err := agent.bind(foreignDevicePath)
	if err != nil {
		t.Fatalf("bind after release: %v", err)
	}
	other()
	if got := bound(); got != "" {
		t.Fatalf("bound = %q after the second release, want empty", got)
	}
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

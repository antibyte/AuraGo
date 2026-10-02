//go:build linux

package bluetooth

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const interactiveAgentPath = dbus.ObjectPath("/com/aurago/bluetooth/agent")

type linuxAgentHost struct {
	logger     *slog.Logger
	conn       *dbus.Conn
	mu         sync.Mutex
	registered int
	defaults   int
}

// newPlatformAgentHost exports the interactive agent on its own connection, so
// a blocked agent call never stalls the live session's signal processing.
func newPlatformAgentHost(broker *interactionBroker, devices deviceResolver, logger *slog.Logger) (agentHost, error) {
	if logger == nil {
		logger = slog.Default()
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect pairing agent to the system D-Bus: %w", err)
	}
	agent := &interactiveAgent{broker: broker, devices: devices}
	if err := conn.Export(agent, interactiveAgentPath, "org.bluez.Agent1"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("export BlueZ pairing agent: %w", err)
	}
	return &linuxAgentHost{logger: logger, conn: conn}, nil
}

func (h *linuxAgentHost) call(ctx context.Context, method string, args ...interface{}) error {
	return asBusError(h.conn.Object(bluezService, "/org/bluez").CallWithContext(ctx, agentManager+"."+method, 0, args...).Err)
}

func (h *linuxAgentHost) Acquire(ctx context.Context, asDefault bool) (func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.registered == 0 {
		if err := h.call(ctx, "RegisterAgent", interactiveAgentPath, "KeyboardDisplay"); err != nil {
			return nil, err
		}
	}
	h.registered++
	if asDefault {
		if h.defaults == 0 {
			if err := h.call(ctx, "RequestDefaultAgent", interactiveAgentPath); err != nil {
				h.releaseLocked(false)
				return nil, err
			}
		}
		h.defaults++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.releaseLocked(asDefault)
		})
	}, nil
}

func (h *linuxAgentHost) releaseLocked(asDefault bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if asDefault {
		h.defaults--
	}
	h.registered--
	switch {
	case h.registered == 0:
		if err := h.call(ctx, "UnregisterAgent", interactiveAgentPath); err != nil {
			h.logger.Warn("[Bluetooth] Failed to unregister pairing agent", "error", err)
		}
	case asDefault && h.defaults == 0:
		// BlueZ cannot drop only the default role; re-register as a plain agent.
		_ = h.call(ctx, "UnregisterAgent", interactiveAgentPath)
		if err := h.call(ctx, "RegisterAgent", interactiveAgentPath, "KeyboardDisplay"); err != nil {
			h.logger.Warn("[Bluetooth] Failed to re-register pairing agent", "error", err)
		}
	}
}

func (h *linuxAgentHost) Pair(ctx context.Context, devicePath string) error {
	return asBusError(h.conn.Object(bluezService, dbus.ObjectPath(devicePath)).CallWithContext(ctx, bluezDeviceInterface+".Pair", 0).Err)
}

func (h *linuxAgentHost) Close() error {
	h.mu.Lock()
	if h.registered > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = h.call(ctx, "UnregisterAgent", interactiveAgentPath)
		cancel()
		h.registered, h.defaults = 0, 0
	}
	h.mu.Unlock()
	return h.conn.Close()
}

// interactiveAgent implements org.bluez.Agent1 by asking the operator through
// the broker. It never logs passkeys or PINs.
type interactiveAgent struct {
	broker  *interactionBroker
	devices deviceResolver
}

func agentRejected() *dbus.Error {
	return &dbus.Error{Name: "org.bluez.Error.Rejected", Body: []interface{}{"Rejected"}}
}

func (a *interactiveAgent) view(kind InteractionKind, device dbus.ObjectPath) Interaction {
	view := Interaction{Kind: kind}
	if resolved, ok := a.devices.deviceByPath(string(device)); ok {
		view.DeviceAddress = resolved.Address
		view.DeviceName = displayDeviceName(resolved)
	}
	return view
}

func (a *interactiveAgent) ask(view Interaction) (interactionAnswer, bool) {
	answer, err := a.broker.ask(context.Background(), view)
	return answer, err == nil && answer.accept
}

func (a *interactiveAgent) Release() *dbus.Error {
	a.broker.cancel()
	return nil
}

func (a *interactiveAgent) Cancel() *dbus.Error {
	a.broker.cancel()
	return nil
}

func (a *interactiveAgent) RequestPinCode(device dbus.ObjectPath) (string, *dbus.Error) {
	answer, ok := a.ask(a.view(InteractionEnterPIN, device))
	if !ok {
		return "", agentRejected()
	}
	return answer.value, nil
}

func (a *interactiveAgent) DisplayPinCode(device dbus.ObjectPath, pin string) *dbus.Error {
	view := a.view(InteractionDisplayPIN, device)
	view.PIN = pin
	a.broker.show(view)
	return nil
}

func (a *interactiveAgent) RequestPasskey(device dbus.ObjectPath) (uint32, *dbus.Error) {
	answer, ok := a.ask(a.view(InteractionEnterPasskey, device))
	if !ok {
		return 0, agentRejected()
	}
	value, err := strconv.ParseUint(answer.value, 10, 32)
	if err != nil || value > 999999 {
		return 0, agentRejected()
	}
	return uint32(value), nil
}

func (a *interactiveAgent) DisplayPasskey(device dbus.ObjectPath, passkey uint32, entered uint16) *dbus.Error {
	view := a.view(InteractionDisplayPasskey, device)
	view.Passkey = fmt.Sprintf("%06d", passkey)
	view.Entered = int(entered)
	a.broker.show(view)
	return nil
}

func (a *interactiveAgent) RequestConfirmation(device dbus.ObjectPath, passkey uint32) *dbus.Error {
	view := a.view(InteractionConfirmPasskey, device)
	view.Passkey = fmt.Sprintf("%06d", passkey)
	if _, ok := a.ask(view); !ok {
		return agentRejected()
	}
	return nil
}

func (a *interactiveAgent) RequestAuthorization(device dbus.ObjectPath) *dbus.Error {
	if _, ok := a.ask(a.view(InteractionAuthorizePairing, device)); !ok {
		return agentRejected()
	}
	return nil
}

func (a *interactiveAgent) AuthorizeService(device dbus.ObjectPath, uuid string) *dbus.Error {
	view := a.view(InteractionAuthorizeService, device)
	view.Service = serviceName(uuid)
	if _, ok := a.ask(view); !ok {
		return agentRejected()
	}
	return nil
}

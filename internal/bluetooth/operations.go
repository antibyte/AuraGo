package bluetooth

import (
	"context"
	"errors"
	"strings"
	"time"
)

type discoverableWindow struct {
	release      func()
	prevPairable bool
	timer        *time.Timer
}

func (m *Manager) endDiscoverable(ctx context.Context) {
	m.mu.Lock()
	window := m.window
	m.window = nil
	m.mu.Unlock()
	if window == nil {
		return
	}
	if window.timer != nil {
		window.timer.Stop()
	}
	if m.live != nil {
		m.live.setDiscoverable(time.Time{})
		if bus, path, err := m.liveAdapter(); err == nil {
			_ = bus.SetProperty(ctx, path, bluezAdapterInterface, "Discoverable", false)
			_ = bus.SetProperty(ctx, path, bluezAdapterInterface, "Pairable", window.prevPairable)
		}
	}
	window.release()
}

func (m *Manager) stopDiscoveryQuietly(ctx context.Context) {
	m.mu.Lock()
	if m.discoveryTimer != nil {
		m.discoveryTimer.Stop()
		m.discoveryTimer = nil
	}
	m.mu.Unlock()
	if m.live == nil || m.live.discoveryEnd().IsZero() {
		return
	}
	m.live.setDiscovery(time.Time{})
	if bus, path, err := m.liveAdapter(); err == nil {
		if callErr := bus.Call(ctx, path, bluezAdapterInterface+".StopDiscovery"); callErr != nil {
			m.logger.Debug("[Bluetooth] StopDiscovery failed", "error", callErr)
		}
	}
}

func (m *Manager) liveAdapter() (bluezBus, string, error) {
	if !m.liveActive() {
		return nil, "", codedError(ErrorUnavailable, "The live Bluetooth session is not running.", nil)
	}
	bus, err := m.live.currentBus()
	if err != nil {
		return nil, "", err
	}
	path := m.live.adapterPath()
	if path == "" {
		return nil, "", codedError(ErrorUnavailable, "No Bluetooth adapter was found.", nil)
	}
	return bus, path, nil
}

// DeviceRequest is one device operation from the admin API or the desktop app.
type DeviceRequest struct {
	Operation   string
	Address     string
	PIN         string
	Interactive bool
}

var deviceOperationTimeouts = map[string]time.Duration{
	"pair":           60 * time.Second,
	"cancel_pairing": 5 * time.Second,
	"connect":        20 * time.Second,
	"disconnect":     10 * time.Second,
	"remove":         10 * time.Second,
	"trust":          5 * time.Second,
	"untrust":        5 * time.Second,
}

var deviceOperationKinds = map[string]string{
	"pair":       "pairing",
	"connect":    "connecting",
	"disconnect": "disconnecting",
	"remove":     "removing",
	"trust":      "trusting",
	"untrust":    "trusting",
}

// RunDeviceOperation validates and runs one device operation. With wait=false
// it returns after starting the operation; progress and the final error code
// appear on the device in the live snapshot.
func (m *Manager) RunDeviceOperation(ctx context.Context, actor Actor, request DeviceRequest, wait bool) error {
	operation := strings.ToLower(strings.TrimSpace(request.Operation))
	timeout, ok := deviceOperationTimeouts[operation]
	if !ok {
		return codedError(ErrorInvalidArgument, "Operation must be pair, cancel_pairing, connect, disconnect, remove, trust, or untrust.", nil)
	}
	address, err := NormalizeAddress(request.Address)
	if err != nil {
		return err
	}
	if actor == ActorAgent && request.Interactive {
		return codedError(ErrorInvalidArgument, "Interactive pairing is available only to operators.", nil)
	}
	request.Operation, request.Address = operation, address
	request.PIN = strings.TrimSpace(request.PIN)
	if operation == "pair" && request.PIN != "" && (len(request.PIN) > 16 || !allDigits(request.PIN)) {
		return codedError(ErrorInvalidArgument, "Bluetooth PIN must contain 1 to 16 digits.", nil)
	}
	if _, _, err := m.requireWritableFor(actor); err != nil {
		return err
	}
	if operation == "cancel_pairing" {
		return m.cancelPairing(ctx, address)
	}
	if !m.liveActive() {
		return m.runLegacyOperation(ctx, request)
	}
	if err := m.live.beginOperation(address, deviceOperationKinds[operation]); err != nil {
		return err
	}
	run := func(runCtx context.Context) error {
		err := m.executeDeviceOperation(runCtx, request)
		m.live.endOperation(address, err)
		return err
	}
	if wait {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return run(runCtx)
	}
	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := run(runCtx); err != nil {
			m.logger.Info("[Bluetooth] Device operation failed", "operation", operation, "address", address, "code", ErrorCode(err), "cause", errors.Unwrap(err))
		}
	}()
	return nil
}

// runLegacyOperation serves pair/connect/disconnect without a live session.
func (m *Manager) runLegacyOperation(ctx context.Context, request DeviceRequest) error {
	switch request.Operation {
	case "pair":
		return mapBlueZError("pair", m.adapter.Pair(ctx, request.Address, request.PIN))
	case "connect":
		return mapBlueZError("connect", m.adapter.Connect(ctx, request.Address))
	case "disconnect":
		return mapBlueZError("disconnect", m.adapter.Disconnect(ctx, request.Address))
	}
	return codedError(ErrorUnavailable, "This Bluetooth operation needs the live BlueZ session.", nil)
}

func (m *Manager) executeDeviceOperation(ctx context.Context, request DeviceRequest) error {
	if request.Operation == "pair" {
		if request.Interactive {
			return m.pairInteractive(ctx, request.Address)
		}
		// Non-interactive pairing keeps its own short-lived NoInputNoOutput agent.
		return mapBlueZError("pair", m.adapter.Pair(ctx, request.Address, request.PIN))
	}
	bus, adapterPath, err := m.liveAdapter()
	if err != nil {
		return err
	}
	path, err := m.live.devicePath(request.Address)
	if err != nil {
		return err
	}
	switch request.Operation {
	case "connect":
		return mapBlueZError("connect", bus.Call(ctx, path, bluezDeviceInterface+".Connect"))
	case "disconnect":
		return mapBlueZError("disconnect", bus.Call(ctx, path, bluezDeviceInterface+".Disconnect"))
	case "remove":
		return mapBlueZError("remove", bus.RemoveDevice(ctx, adapterPath, path))
	case "trust", "untrust":
		return mapBlueZError("trust", bus.SetProperty(ctx, path, bluezDeviceInterface, "Trusted", request.Operation == "trust"))
	}
	return codedError(ErrorInvalidArgument, "Unsupported Bluetooth operation.", nil)
}

func (m *Manager) cancelPairing(ctx context.Context, address string) error {
	m.broker.cancel()
	if !m.liveActive() {
		return nil
	}
	bus, _, err := m.liveAdapter()
	if err != nil {
		return err
	}
	path, err := m.live.devicePath(address)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := mapBlueZError("cancel", bus.Call(callCtx, path, bluezDeviceInterface+".CancelPairing")); err != nil && ErrorCode(err) != ErrorDeviceNotFound {
		return err
	}
	return nil
}

func (m *Manager) interactiveAgents() (agentHost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.agents != nil {
		return m.agents, nil
	}
	if m.newAgents == nil || m.live == nil {
		return nil, codedError(ErrorUnavailable, "Interactive Bluetooth pairing is not available.", nil)
	}
	host, err := m.newAgents(m.broker, m.live, m.logger)
	if err != nil {
		return nil, codedError(ErrorUnavailable, "The Bluetooth pairing agent could not be started.", err)
	}
	m.agents = host
	return host, nil
}

func (m *Manager) pairInteractive(ctx context.Context, address string) error {
	host, err := m.interactiveAgents()
	if err != nil {
		return err
	}
	release, err := host.Acquire(ctx, false)
	if err != nil {
		return mapBlueZError("pair", err)
	}
	defer release()
	defer m.broker.cancel()
	path, err := m.live.devicePath(address)
	if err != nil {
		return err
	}
	if err := mapBlueZError("pair", host.Pair(ctx, path)); err != nil {
		return err
	}
	if bus, busErr := m.live.currentBus(); busErr == nil {
		if trustErr := bus.SetProperty(ctx, path, bluezDeviceInterface, "Trusted", true); trustErr != nil {
			m.logger.Warn("[Bluetooth] Paired device could not be marked trusted", "address", address, "error", trustErr)
		}
	}
	return nil
}

// SetPowered switches the adapter. It needs a present adapter, not a powered one.
func (m *Manager) SetPowered(ctx context.Context, actor Actor, powered bool) error {
	if _, _, err := m.requirePresentFor(actor); err != nil {
		return err
	}
	bus, path, err := m.liveAdapter()
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !powered {
		m.endDiscoverable(callCtx)
		m.stopDiscoveryQuietly(callCtx)
	}
	return mapBlueZError("power", bus.SetProperty(callCtx, path, bluezAdapterInterface, "Powered", powered))
}

func clampDiscovery(duration, fallback time.Duration) time.Duration {
	if duration <= 0 {
		duration = fallback
	}
	if duration > 60*time.Second {
		duration = 60 * time.Second
	}
	return duration
}

func clampDiscoverable(duration time.Duration) time.Duration {
	switch {
	case duration <= 0:
		return 180 * time.Second
	case duration < 60*time.Second:
		return 60 * time.Second
	case duration > 600*time.Second:
		return 600 * time.Second
	}
	return duration
}

// StartDiscovery starts or extends one server-owned scan that stops on its own.
// A shorter request never shortens a scan that is already running.
func (m *Manager) StartDiscovery(ctx context.Context, actor Actor, duration time.Duration) (TimedState, error) {
	options, _, err := m.requireUsable()
	if err != nil {
		return TimedState{}, err
	}
	bus, path, err := m.liveAdapter()
	if err != nil {
		return TimedState{}, err
	}
	duration = clampDiscovery(duration, options.ScanTimeout)
	ends := time.Now().Add(duration)
	if current := m.live.discoveryEnd(); current.After(ends) {
		return TimedState{Active: true, EndsAt: current, RemainingSeconds: int(time.Until(current).Seconds() + 0.999)}, nil
	}
	if err := mapBlueZError("discovery", bus.Call(ctx, path, bluezAdapterInterface+".StartDiscovery")); err != nil && ErrorCode(err) != ErrorOperationBusy {
		return TimedState{}, err
	}
	m.live.setDiscovery(ends)
	m.mu.Lock()
	if m.discoveryTimer != nil {
		m.discoveryTimer.Stop()
	}
	m.discoveryTimer = time.AfterFunc(duration, func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		m.stopDiscoveryQuietly(stopCtx)
	})
	m.mu.Unlock()
	return TimedState{Active: true, EndsAt: ends, RemainingSeconds: int(duration.Seconds() + 0.999)}, nil
}

// StopDiscovery ends the server-owned scan.
func (m *Manager) StopDiscovery(ctx context.Context, _ Actor) error {
	m.stopDiscoveryQuietly(ctx)
	return nil
}

// SetDiscoverable opens a bounded window in which phones can pair with the
// server. Only operators may open it; its questions go to the desktop app.
func (m *Manager) SetDiscoverable(ctx context.Context, actor Actor, duration time.Duration) (TimedState, error) {
	if actor != ActorOperator {
		return TimedState{}, codedError(ErrorInvalidArgument, "Only operators can make the server discoverable.", nil)
	}
	if _, _, err := m.requireUsable(); err != nil {
		return TimedState{}, err
	}
	bus, path, err := m.liveAdapter()
	if err != nil {
		return TimedState{}, err
	}
	host, err := m.interactiveAgents()
	if err != nil {
		return TimedState{}, err
	}
	duration = clampDiscoverable(duration)
	m.endDiscoverable(ctx)
	release, err := host.Acquire(ctx, true)
	if err != nil {
		return TimedState{}, mapBlueZError("discoverable", err)
	}
	previous := m.live.adapter().Pairable
	seconds := uint32(duration / time.Second)
	for _, step := range []struct {
		name  string
		value interface{}
	}{{"Pairable", true}, {"DiscoverableTimeout", seconds}, {"Discoverable", true}} {
		if err := bus.SetProperty(ctx, path, bluezAdapterInterface, step.name, step.value); err != nil {
			_ = bus.SetProperty(ctx, path, bluezAdapterInterface, "Pairable", previous)
			release()
			return TimedState{}, mapBlueZError("discoverable", err)
		}
	}
	ends := time.Now().Add(duration)
	window := &discoverableWindow{release: release, prevPairable: previous}
	// BlueZ turns Discoverable off itself; this timer restores Pairable and
	// releases the agent shortly afterwards.
	window.timer = time.AfterFunc(duration+time.Second, func() {
		endCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		m.endDiscoverable(endCtx)
	})
	m.mu.Lock()
	m.window = window
	m.mu.Unlock()
	m.live.setDiscoverable(ends)
	return TimedState{Active: true, EndsAt: ends, RemainingSeconds: int(seconds)}, nil
}

// StopDiscoverable ends the window early.
func (m *Manager) StopDiscoverable(ctx context.Context, _ Actor) error {
	m.endDiscoverable(ctx)
	return nil
}

// Interaction returns the open pairing question for the admin API.
func (m *Manager) Interaction(id string) (Interaction, error) {
	return m.broker.get(id)
}

// AnswerInteraction answers the open pairing question.
func (m *Manager) AnswerInteraction(id string, accept bool, value string) error {
	return m.broker.answer(id, accept, value)
}

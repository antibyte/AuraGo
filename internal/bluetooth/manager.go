package bluetooth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type platformAdapter interface {
	Probe(context.Context) (AdapterStatus, error)
	List(context.Context) ([]Device, error)
	Discover(context.Context, time.Duration) ([]Device, error)
	Pair(context.Context, string, string) error
	Connect(context.Context, string) error
	Disconnect(context.Context, string) error
}

// Manager coordinates capability probing, the live BlueZ session, operator and
// agent operations, pairing questions, and one AuraGo playback.
type Manager struct {
	mu             sync.RWMutex
	adapter        platformAdapter
	runner         commandRunner
	logger         *slog.Logger
	options        Options
	status         Status
	playback       *playbackSession
	live           *liveSession
	broker         *interactionBroker
	agents         agentHost
	newAgents      agentHostFactory
	window         *discoverableWindow
	discoveryTimer *time.Timer
	audioStale     bool
	headsetRunner  headsetRunner
	headsets       map[string]bool
}

var (
	defaultManagerMu sync.RWMutex
	defaultManager   *Manager
)

// NewManager constructs the platform-specific Bluetooth service.
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	manager := &Manager{
		adapter:       newPlatformAdapter(logger),
		runner:        execCommandRunner{},
		logger:        logger,
		status:        PlaybackUnavailableStatus(),
		broker:        newInteractionBroker(),
		newAgents:     newPlatformAgentHost,
		audioStale:    true,
		headsetRunner: execHeadsetRunner{},
	}
	manager.live = newLiveSession(platformBusDialer(logger), logger)
	manager.broker.onChange = manager.live.notifyInteraction
	return manager
}

func liveAllowed(options Options) bool {
	return options.Enabled && !options.IsDocker
}

// StartLive begins the live BlueZ session on supported platforms. Disabled or
// Docker configurations keep the session idle until Configure enables it.
func (m *Manager) StartLive(ctx context.Context) {
	if m == nil || m.live == nil || !platformSupported() {
		return
	}
	m.live.start(ctx, liveAllowed(m.currentOptions()))
	changes, cancel := m.live.subscribe()
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-changes:
				m.refreshAudioIfNeeded(ctx)
				// A question whose BlueZ caller vanished can never be answered.
				if !m.live.snapshot(time.Now()).Present {
					m.broker.cancel()
				}
			}
		}
	}()
}

func (m *Manager) liveActive() bool {
	return m != nil && m.live != nil && m.live.active()
}

// refreshAudioIfNeeded probes the audio backend once the adapter becomes usable
// after the last probe found it unusable.
func (m *Manager) refreshAudioIfNeeded(ctx context.Context) {
	status := m.Status()
	m.mu.Lock()
	stale := m.audioStale
	m.audioStale = !status.Usable
	options := m.options
	m.mu.Unlock()
	if !status.Usable || !stale {
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	audio := probeAudioBackend(probeCtx, m.runner, options.AudioBackend)
	cancel()
	m.mu.Lock()
	m.status.Audio = audio
	m.mu.Unlock()
}

// PlaybackUnavailableStatus returns the conservative initial runtime state.
func PlaybackUnavailableStatus() Status {
	return Status{
		Reason:       "Bluetooth has not been probed yet.",
		Audio:        AudioStatus{Reason: "Bluetooth audio has not been probed yet."},
		LastProbedAt: time.Now().UTC(),
	}
}

// SetDefaultManager shares one service between agent dispatch and the admin API.
func SetDefaultManager(manager *Manager) {
	defaultManagerMu.Lock()
	defaultManager = manager
	defaultManagerMu.Unlock()
}

// DefaultManager returns the process-wide Bluetooth manager.
func DefaultManager() *Manager {
	defaultManagerMu.RLock()
	defer defaultManagerMu.RUnlock()
	return defaultManager
}

// Detect performs a one-shot startup probe without starting discovery.
func Detect(ctx context.Context, options Options, logger *slog.Logger) Status {
	manager := NewManager(logger)
	manager.Configure(options)
	return manager.Reprobe(ctx)
}

// SeedStatus initializes a manager from the startup probe.
func (m *Manager) SeedStatus(status Status) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.status = status
	m.audioStale = !status.Usable
	m.mu.Unlock()
}

// Configure hot-reloads permissions, stops playback when access is revoked and
// switches the live session. It is called for every agent tool call, so it
// must stay cheap and idempotent.
func (m *Manager) Configure(options Options) {
	if m == nil {
		return
	}
	options = normalizeOptions(options)
	m.mu.Lock()
	previous := m.options
	m.options = options
	live := m.live
	m.mu.Unlock()
	if previous.AllowPlayback && (!options.Enabled || !options.AllowPlayback) {
		_ = m.Stop()
	}
	if live != nil {
		live.setEnabled(liveAllowed(options))
	}
	if previous.Enabled && !options.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		m.endDiscoverable(ctx)
		cancel()
	}
}

// Status returns an immutable runtime snapshot, live when the BlueZ session runs.
func (m *Manager) Status() Status {
	if m == nil {
		return PlaybackUnavailableStatus()
	}
	m.mu.RLock()
	status, live := m.status, m.live
	m.mu.RUnlock()
	if live != nil && live.active() {
		return live.status(status)
	}
	return status
}

// Present reports whether a usable adapter exists, powered or not. It gates the
// desktop app.
func (m *Manager) Present() bool {
	if m == nil {
		return false
	}
	return liveAllowed(m.currentOptions()) && m.Status().Present
}

// Snapshot returns the live view for the admin API and the desktop app.
func (m *Manager) Snapshot(ctx context.Context) Snapshot {
	if m.liveActive() {
		return m.live.snapshot(time.Now())
	}
	status := m.Status()
	snapshot := Snapshot{Present: status.Present, Reason: status.Reason, Adapter: status.Adapter, Devices: []Device{}, InteractionID: m.broker.current()}
	if status.Usable {
		listCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		if devices, err := m.List(listCtx); err == nil {
			snapshot.Devices = devices
		}
		cancel()
	}
	return snapshot
}

// Subscribe delivers debounced live changes. Without a live session the
// returned channel never fires.
func (m *Manager) Subscribe() (<-chan Change, func()) {
	if m == nil || m.live == nil {
		return make(chan Change), func() {}
	}
	return m.live.subscribe()
}

func (m *Manager) resyncLive() {
	if m.live != nil {
		m.live.resync()
	}
}

// Reprobe refreshes adapter and audio capability state without discovering devices.
func (m *Manager) Reprobe(ctx context.Context) Status {
	if m == nil {
		return PlaybackUnavailableStatus()
	}
	defer m.resyncLive()
	m.mu.RLock()
	options := m.options
	m.mu.RUnlock()

	status := Status{
		Supported:    platformSupported(),
		LastProbedAt: time.Now().UTC(),
	}
	if !options.Enabled {
		status.Reason = "Bluetooth is disabled in the AuraGo configuration."
		status.Audio.Reason = status.Reason
		m.storeStatus(status)
		return status
	}
	if options.IsDocker {
		status.Reason = "Bluetooth is unavailable in the default Docker deployment because host D-Bus and audio sockets are not passed through."
		status.Audio.Reason = status.Reason
		m.storeStatus(status)
		return status
	}
	if !status.Supported {
		status.Reason = "Bluetooth is currently supported only on Linux with BlueZ."
		status.Audio.Reason = status.Reason
		m.storeStatus(status)
		return status
	}

	adapter, err := m.adapter.Probe(ctx)
	status.Adapter = adapter
	status.Present = adapter.Path != "" && adapter.PowerState != powerStateOffBlocked
	if err != nil {
		status.Reason = err.Error()
		status.Audio.Reason = "Bluetooth audio requires a usable BlueZ adapter."
		m.storeStatus(status)
		return status
	}
	status.Usable = adapter.Powered
	if !status.Usable {
		status.Reason = "No powered Bluetooth adapter is available."
		status.Audio.Reason = "Bluetooth audio requires a powered adapter."
		m.storeStatus(status)
		return status
	}

	status.Audio = probeAudioBackend(ctx, m.runner, options.AudioBackend)
	m.storeStatus(status)
	m.logger.Info("[Bluetooth] Runtime probe completed",
		"usable", status.Usable,
		"adapter", status.Adapter.Name,
		"audio_usable", status.Audio.Usable,
		"audio_backend", status.Audio.Backend)
	return status
}

func (m *Manager) storeStatus(status Status) {
	m.mu.Lock()
	m.status = status
	m.mu.Unlock()
}

func (m *Manager) requireUsable() (Options, Status, error) {
	status := m.Status()
	options := m.currentOptions()
	if options.Enabled && status.Usable {
		return options, status, nil
	}
	if options.Enabled && status.Present && !status.Adapter.Powered {
		return Options{}, status, codedError(ErrorPoweredOff, "The Bluetooth adapter is turned off.", nil)
	}
	reason := status.Reason
	if reason == "" {
		reason = "Bluetooth is not usable."
	}
	return Options{}, status, codedError(ErrorUnavailable, reason, nil)
}

// requirePresentFor allows adapter-level operations on a present adapter, even
// when it is powered off.
func (m *Manager) requirePresentFor(actor Actor) (Options, Status, error) {
	status := m.Status()
	options := m.currentOptions()
	if !options.Enabled || !status.Present {
		reason := firstNonEmpty(status.Reason, "No Bluetooth adapter is available.")
		return Options{}, status, codedError(ErrorUnavailable, reason, nil)
	}
	if actor == ActorAgent && options.ReadOnly {
		return Options{}, status, codedError(ErrorReadOnly, "Bluetooth is in read-only mode; adapter changes are disabled.", nil)
	}
	return options, status, nil
}

// List returns BlueZ's current devices without starting discovery.
func (m *Manager) List(ctx context.Context) ([]Device, error) {
	if _, _, err := m.requireUsable(); err != nil {
		return nil, err
	}
	if m.liveActive() {
		return m.live.snapshot(time.Now()).Devices, nil
	}
	devices, err := m.adapter.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Bluetooth devices: %w", err)
	}
	return devices, nil
}

// Discover starts a bounded BlueZ scan and always asks BlueZ to stop discovery.
// It reuses the server-owned scan when the live session runs.
func (m *Manager) Discover(ctx context.Context, timeout time.Duration) ([]Device, error) {
	options, _, err := m.requireUsable()
	if err != nil {
		return nil, err
	}
	timeout = clampDiscovery(timeout, options.ScanTimeout)
	if !m.liveActive() {
		devices, err := m.adapter.Discover(ctx, timeout)
		if err != nil {
			return nil, fmt.Errorf("discover Bluetooth devices: %w", err)
		}
		return devices, nil
	}
	state, err := m.StartDiscovery(ctx, ActorAgent, timeout)
	if err != nil {
		return nil, err
	}
	timer := time.NewTimer(time.Until(state.EndsAt))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	return m.live.snapshot(time.Now()).Devices, nil
}

// Pair pairs only the explicitly addressed device without user interaction.
func (m *Manager) Pair(ctx context.Context, actor Actor, address, pin string) error {
	return m.RunDeviceOperation(ctx, actor, DeviceRequest{Operation: "pair", Address: address, PIN: pin}, true)
}

// Connect connects a previously paired device.
func (m *Manager) Connect(ctx context.Context, actor Actor, address string) error {
	return m.RunDeviceOperation(ctx, actor, DeviceRequest{Operation: "connect", Address: address}, true)
}

// Disconnect disconnects a device.
func (m *Manager) Disconnect(ctx context.Context, actor Actor, address string) error {
	return m.RunDeviceOperation(ctx, actor, DeviceRequest{Operation: "disconnect", Address: address}, true)
}

// ResolveTarget applies explicit target, configured default, then sole connected audio device.
func (m *Manager) ResolveTarget(ctx context.Context, requested string) (Device, error) {
	options, _, err := m.requireUsable()
	if err != nil {
		return Device{}, err
	}
	devices, err := m.List(ctx)
	if err != nil {
		return Device{}, err
	}
	target := strings.TrimSpace(requested)
	if target == "" {
		target = options.DefaultDevice
	}
	if target != "" {
		matches := matchDevices(devices, target)
		if len(matches) == 0 {
			return Device{}, codedError(ErrorDeviceNotFound, fmt.Sprintf("Bluetooth device %q was not found.", target), nil)
		}
		if len(matches) > 1 {
			return Device{}, codedError(ErrorDeviceAmbiguous, fmt.Sprintf("Bluetooth device name %q is ambiguous; use its address.", target), nil)
		}
		if !isAudioDevice(matches[0]) {
			return Device{}, codedError(ErrorAudioTargetUnavailable, fmt.Sprintf("Bluetooth device %q does not expose a supported audio profile.", target), nil)
		}
		return matches[0], nil
	}

	var connected []Device
	for _, device := range devices {
		if device.Connected && isAudioDevice(device) {
			connected = append(connected, device)
		}
	}
	if len(connected) == 1 {
		return connected[0], nil
	}
	if len(connected) == 0 {
		return Device{}, codedError(ErrorDeviceNotFound, "No connected Bluetooth audio device is available; specify a device or configure bluetooth.default_device.", nil)
	}
	return Device{}, codedError(ErrorDeviceAmbiguous, "More than one Bluetooth audio device is connected; specify a device or configure bluetooth.default_device.", nil)
}

func matchDevices(devices []Device, target string) []Device {
	if address, err := NormalizeAddress(target); err == nil {
		for _, device := range devices {
			if strings.EqualFold(device.Address, address) {
				return []Device{device}
			}
		}
		return nil
	}
	target = strings.ToLower(strings.TrimSpace(target))
	var matches []Device
	for _, device := range devices {
		if strings.ToLower(strings.TrimSpace(device.Name)) == target ||
			strings.ToLower(strings.TrimSpace(device.Alias)) == target {
			matches = append(matches, device)
		}
	}
	return matches
}

func allDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

// Close stops AuraGo-owned playback, discovery, visibility and the live session.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	err := m.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	m.endDiscoverable(ctx)
	m.stopDiscoveryQuietly(ctx)
	m.broker.cancel()
	if m.live != nil {
		m.live.stop(2 * time.Second)
	}
	m.mu.Lock()
	agents := m.agents
	m.agents = nil
	m.mu.Unlock()
	if agents != nil {
		_ = agents.Close()
	}
	return err
}

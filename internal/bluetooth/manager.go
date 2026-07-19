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

// Manager coordinates capability probing, BlueZ operations, and one AuraGo playback.
type Manager struct {
	mu       sync.RWMutex
	adapter  platformAdapter
	runner   commandRunner
	logger   *slog.Logger
	options  Options
	status   Status
	playback *playbackSession
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
	return &Manager{
		adapter: newPlatformAdapter(logger),
		runner:  execCommandRunner{},
		logger:  logger,
		status:  PlaybackUnavailableStatus(),
	}
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
	m.mu.Unlock()
}

// Configure hot-reloads permissions and stops playback when access is revoked.
func (m *Manager) Configure(options Options) {
	if m == nil {
		return
	}
	options = normalizeOptions(options)
	m.mu.Lock()
	previous := m.options
	m.options = options
	m.mu.Unlock()
	if previous.AllowPlayback && (!options.Enabled || !options.AllowPlayback) {
		_ = m.Stop()
	}
}

// Status returns an immutable runtime snapshot.
func (m *Manager) Status() Status {
	if m == nil {
		return PlaybackUnavailableStatus()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// Reprobe refreshes adapter and audio capability state without discovering devices.
func (m *Manager) Reprobe(ctx context.Context) Status {
	if m == nil {
		return PlaybackUnavailableStatus()
	}
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
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.options.Enabled || !m.status.Usable {
		reason := m.status.Reason
		if reason == "" {
			reason = "Bluetooth is not usable."
		}
		return Options{}, m.status, codedError(ErrorUnavailable, reason, nil)
	}
	return m.options, m.status, nil
}

func (m *Manager) requireWritable() (Options, Status, error) {
	options, status, err := m.requireUsable()
	if err != nil {
		return Options{}, status, err
	}
	if options.ReadOnly {
		return Options{}, status, codedError(ErrorReadOnly, "Bluetooth is in read-only mode; pairing and connection changes are disabled.", nil)
	}
	return options, status, nil
}

func (m *Manager) requirePlayback() (Options, Status, error) {
	options, status, err := m.requireUsable()
	if err != nil {
		return Options{}, status, err
	}
	if !options.AllowPlayback {
		return Options{}, status, codedError(ErrorPlaybackDisabled, "Bluetooth playback is disabled in the AuraGo configuration.", nil)
	}
	if !status.Audio.Usable {
		return Options{}, status, codedError(ErrorAudioTargetUnavailable, status.Audio.Reason, nil)
	}
	return options, status, nil
}

// List returns BlueZ's current devices without starting discovery.
func (m *Manager) List(ctx context.Context) ([]Device, error) {
	if _, _, err := m.requireUsable(); err != nil {
		return nil, err
	}
	devices, err := m.adapter.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Bluetooth devices: %w", err)
	}
	return devices, nil
}

// Discover starts a bounded BlueZ scan and always asks BlueZ to stop discovery.
func (m *Manager) Discover(ctx context.Context, timeout time.Duration) ([]Device, error) {
	options, _, err := m.requireUsable()
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = options.ScanTimeout
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	devices, err := m.adapter.Discover(ctx, timeout)
	if err != nil {
		return nil, fmt.Errorf("discover Bluetooth devices: %w", err)
	}
	return devices, nil
}

// Pair pairs only the explicitly addressed device.
func (m *Manager) Pair(ctx context.Context, address, pin string) error {
	if _, _, err := m.requireWritable(); err != nil {
		return err
	}
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return err
	}
	pin = strings.TrimSpace(pin)
	if pin != "" && (len(pin) > 16 || !allDigits(pin)) {
		return codedError(ErrorInvalidArgument, "Bluetooth PIN must contain 1 to 16 digits.", nil)
	}
	if err := m.adapter.Pair(ctx, normalized, pin); err != nil {
		return err
	}
	return nil
}

// Connect connects a previously paired device.
func (m *Manager) Connect(ctx context.Context, address string) error {
	if _, _, err := m.requireWritable(); err != nil {
		return err
	}
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return err
	}
	if err := m.adapter.Connect(ctx, normalized); err != nil {
		return fmt.Errorf("connect Bluetooth device: %w", err)
	}
	return nil
}

// Disconnect disconnects a device.
func (m *Manager) Disconnect(ctx context.Context, address string) error {
	if _, _, err := m.requireWritable(); err != nil {
		return err
	}
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return err
	}
	if err := m.adapter.Disconnect(ctx, normalized); err != nil {
		return fmt.Errorf("disconnect Bluetooth device: %w", err)
	}
	return nil
}

// ResolveTarget applies explicit target, configured default, then sole connected audio device.
func (m *Manager) ResolveTarget(ctx context.Context, requested string) (Device, error) {
	options, _, err := m.requireUsable()
	if err != nil {
		return Device{}, err
	}
	devices, err := m.adapter.List(ctx)
	if err != nil {
		return Device{}, fmt.Errorf("list Bluetooth devices: %w", err)
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

// Close stops AuraGo-owned playback.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	return m.Stop()
}

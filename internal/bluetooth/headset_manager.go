package bluetooth

import (
	"context"
	"strings"
)

// HeadsetDevice is a paired Bluetooth device with a microphone that Live
// Speech can use as its audio device.
type HeadsetDevice struct {
	Address   string `json:"id"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Busy      bool   `json:"busy"`
}

// HeadsetDevices lists paired devices with a microphone. An empty list comes
// with the reason why server headsets are unavailable.
func (m *Manager) HeadsetDevices(ctx context.Context) ([]HeadsetDevice, string) {
	if reason := m.headsetUnavailableReason(); reason != "" {
		return []HeadsetDevice{}, reason
	}
	devices := []HeadsetDevice{}
	for _, device := range m.Snapshot(ctx).Devices {
		if !device.Paired || !hasMicrophone(device) {
			continue
		}
		devices = append(devices, HeadsetDevice{
			Address:   device.Address,
			Name:      firstNonEmpty(device.Alias, device.Name, device.Address),
			Connected: device.Connected,
			Busy:      m.headsetBusy(device.Address),
		})
	}
	return devices, ""
}

// OpenHeadset reserves a paired headset with a microphone for one Live Speech
// bridge and starts following its connection. Close the link to free it.
func (m *Manager) OpenHeadset(ctx context.Context, address string) (*HeadsetLink, error) {
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return nil, err
	}
	if reason := m.headsetUnavailableReason(); reason != "" {
		return nil, codedError(ErrorHeadsetAudioUnavailable, reason, nil)
	}
	if _, ok := m.headsetDevice(ctx, normalized); !ok {
		return nil, codedError(ErrorHeadsetUnknown, "The headset is not paired with this server or has no microphone.", nil)
	}
	key := compactAddress(normalized)
	m.mu.Lock()
	if m.headsets == nil {
		m.headsets = map[string]bool{}
	}
	if m.headsets[key] {
		m.mu.Unlock()
		return nil, codedError(ErrorHeadsetBusy, "The headset is already used by another browser.", nil)
	}
	m.headsets[key] = true
	runner := m.headsetRunner
	m.mu.Unlock()

	changes, unsubscribe := m.Subscribe()
	release := func() {
		unsubscribe()
		m.mu.Lock()
		delete(m.headsets, key)
		m.mu.Unlock()
	}
	return startHeadsetLink(ctx, headsetLinkConfig{
		address: normalized,
		runner:  runner,
		lookup:  func(ctx context.Context) (Device, bool) { return m.headsetDevice(ctx, normalized) },
		changes: changes,
		release: release,
	}), nil
}

func (m *Manager) headsetUnavailableReason() string {
	if m == nil {
		return "Bluetooth is not available."
	}
	if !liveAllowed(m.currentOptions()) {
		return "Bluetooth is disabled in the AuraGo configuration."
	}
	status := m.Status()
	if !status.Present {
		return firstNonEmpty(status.Reason, "No Bluetooth adapter is available.")
	}
	if !status.Audio.Usable {
		return firstNonEmpty(status.Audio.Reason, "Bluetooth audio is not usable.")
	}
	if status.Audio.Backend != "pipewire" {
		return "Server headsets need PipeWire; PulseAudio is not supported."
	}
	m.mu.RLock()
	runner := m.headsetRunner
	m.mu.RUnlock()
	if runner == nil {
		return "Bluetooth headset audio is not available."
	}
	return ""
}

func (m *Manager) headsetBusy(address string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.headsets[compactAddress(address)]
}

func (m *Manager) headsetDevice(ctx context.Context, address string) (Device, bool) {
	for _, device := range m.Snapshot(ctx).Devices {
		if strings.EqualFold(device.Address, address) && device.Paired && hasMicrophone(device) {
			return device, true
		}
	}
	return Device{}, false
}

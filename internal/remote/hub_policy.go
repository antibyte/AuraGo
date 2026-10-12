//go:build !remote_minimal

package remote

import (
	"encoding/json"
	"time"
)

type RuntimePolicy struct {
	ReadOnly      bool
	AutoApprove   bool
	AuditLog      bool
	MaxFileSizeMB int
	AllowedPaths  []string
}

func (h *RemoteHub) RuntimePolicy() RuntimePolicy {
	p := RuntimePolicy{ReadOnly: h.DefaultReadOnly, AutoApprove: h.AutoApprove, AuditLog: h.AuditLogEnabled, MaxFileSizeMB: h.MaxFileSizeMB}
	if h.Policy != nil {
		p = h.Policy()
	} else if h.DefaultAllowedPaths != nil {
		p.AllowedPaths = h.DefaultAllowedPaths()
	}
	if p.MaxFileSizeMB <= 0 {
		p.MaxFileSizeMB = DefaultMaxFileSizeMB
	}
	if p.MaxFileSizeMB > 500 {
		p.MaxFileSizeMB = 500
	}
	p.AllowedPaths = append([]string{}, p.AllowedPaths...)
	return p
}

type DeviceRuntime struct {
	EffectiveReadOnly     bool              `json:"effective_read_only"`
	EffectiveAllowedPaths []string          `json:"effective_allowed_paths"`
	Capabilities          []string          `json:"capabilities"`
	ConfigState           string            `json:"config_state"`
	Telemetry             *HeartbeatPayload `json:"telemetry,omitempty"`
}

func (h *RemoteHub) DeviceRuntime(device DeviceRecord) DeviceRuntime {
	result := DeviceRuntime{EffectiveReadOnly: device.ReadOnly || h.RuntimePolicy().ReadOnly, EffectiveAllowedPaths: h.EffectiveAllowedPaths(device.ID), Capabilities: []string{}, ConfigState: "pending"}
	h.mu.RLock()
	if state := h.configStates[device.ID]; state != "" {
		result.ConfigState = state
	}
	h.mu.RUnlock()
	if IsAgodeskDevice(device) {
		result.ConfigState = "server_enforced"
		if transport := h.connectedCommandTransport(device.ID); transport != nil {
			if provider, ok := transport.(interface{ Capabilities(string) []string }); ok {
				result.Capabilities = provider.Capabilities(device.ID)
			}
		}
	}
	if conn := h.GetConnection(device.ID); conn != nil {
		conn.mu.Lock()
		result.Capabilities = append([]string{}, conn.Capabilities...)
		result.ConfigState = conn.configState
		if result.ConfigState == "" {
			result.ConfigState = "confirmed"
		}
		hb := conn.Telemetry
		result.Telemetry = &hb
		conn.mu.Unlock()
	}
	return result
}

// RefreshPolicy pushes a complete snapshot; enforcement reads live policy immediately.
func (h *RemoteHub) RefreshPolicy() {
	for _, id := range h.ConnectedDevices() {
		h.refreshDevicePolicy(id)
	}
}
func (h *RemoteHub) refreshDevicePolicy(id string) {
	defer func() {
		if p := recover(); p != nil {
			h.logger.Error("Remote settings delivery panicked", "device_id", id, "panic", p)
		}
	}()
	device := DeviceRecord{ID: id}
	if h.db != nil {
		var err error
		device, err = GetDevice(h.db, id)
		if err != nil {
			return
		}
	} else if conn := h.GetConnection(id); conn != nil {
		conn.mu.Lock()
		device.ReadOnly = conn.ReadOnly
		device.AllowedPaths = append([]string{}, conn.AllowedPaths...)
		conn.mu.Unlock()
	}
	maxMB := h.RuntimePolicy().MaxFileSizeMB
	if err := h.SendConfigUpdate(id, ConfigUpdatePayload{ReadOnly: &device.ReadOnly, AllowedPaths: append([]string{}, device.AllowedPaths...), MaxFileSizeMB: &maxMB}); err != nil {
		h.logger.Error("Remote settings delivery failed", "device_id", id, "error", err)
	}
}

// ApplyDevicePolicy keeps metadata-only saves and masked preferences from interrupting work.
func (h *RemoteHub) ApplyDevicePolicy(previous, device DeviceRecord) error {
	policy := h.RuntimePolicy()
	signature := func(d DeviceRecord) string {
		paths := cleanAllowedPaths(d.AllowedPaths)
		if IsAgodeskDevice(d) {
			paths = nil // AgoDesk manages paths locally.
		} else if len(paths) == 0 {
			paths = policy.AllowedPaths
		}
		return policySignature(d.ReadOnly || policy.ReadOnly, paths, policy.MaxFileSizeMB)
	}
	if signature(previous) == signature(device) {
		if conn := h.GetConnection(device.ID); conn != nil {
			conn.mu.Lock()
			conn.ReadOnly = device.ReadOnly
			conn.AllowedPaths = append([]string{}, device.AllowedPaths...)
			conn.mu.Unlock()
		}
		return nil
	}
	h.InvalidateDevicePolicy(device.ID)
	if h.GetConnection(device.ID) == nil {
		return nil // Active work and alternate transports were disconnected by invalidation.
	}
	return h.SendConfigUpdate(device.ID, ConfigUpdatePayload{
		ReadOnly: &device.ReadOnly, AllowedPaths: append([]string{}, device.AllowedPaths...), MaxFileSizeMB: &policy.MaxFileSizeMB,
	})
}

// InvalidateDevicePolicy revokes admission before a potentially blocked config write.
func (h *RemoteHub) InvalidateDevicePolicy(deviceID string) {
	if conn := h.GetConnection(deviceID); conn != nil {
		conn.mu.Lock()
		conn.configState = "pending"
		conn.mu.Unlock()
		h.pendingMu.Lock()
		active := false
		for _, owner := range h.pendingOwners {
			active = active || owner == conn
		}
		h.pendingMu.Unlock()
		if active {
			h.unregisterConnection(deviceID, conn)
		}
	}
	if transport := h.connectedCommandTransport(deviceID); transport != nil {
		transport.Disconnect(deviceID)
	}
}

// InvalidatePolicy blocks new work before delivery and cancels existing commands.
func (h *RemoteHub) InvalidatePolicy(disconnectTransports bool) {
	h.mu.RLock()
	conns := make([]*RemoteConnection, 0, len(h.connections))
	for _, conn := range h.connections {
		conns = append(conns, conn)
	}
	transports := make([]CommandTransport, 0, len(h.transports))
	for _, transport := range h.transports {
		transports = append(transports, transport)
	}
	h.mu.RUnlock()
	for _, conn := range conns {
		conn.mu.Lock()
		conn.configState = "pending"
		conn.mu.Unlock()
	}
	h.pendingMu.Lock()
	active := make(map[*RemoteConnection]bool)
	for _, conn := range h.pendingOwners {
		active[conn] = true
	}
	h.pendingMu.Unlock()
	for _, conn := range conns {
		if active[conn] {
			h.unregisterConnection(conn.DeviceID, conn)
		}
	}
	if disconnectTransports {
		for _, transport := range transports {
			transport.Disconnect("")
		}
	}
}

func (h *RemoteHub) awaitConfigAck(conn *RemoteConnection, ref string) {
	conn.mu.Lock()
	if conn.configTimer != nil {
		conn.configTimer.Stop()
	}
	conn.configRef, conn.configState = ref, "pending"
	conn.configTimer = time.AfterFunc(10*time.Second, func() {
		conn.mu.Lock()
		expired := conn.configRef == ref && conn.configState == "pending"
		if expired {
			conn.configState = "error"
		}
		conn.mu.Unlock()
		if expired {
			conn.Close()
			h.unregisterConnection(conn.DeviceID, conn)
		}
	})
	conn.mu.Unlock()
}

func policySignature(ro bool, paths []string, maxMB int) string {
	value, _ := json.Marshal(struct {
		ReadOnly bool
		Paths    []string
		MaxMB    int
	}{ro, cleanAllowedPaths(paths), maxMB})
	return string(value)
}
func (h *RemoteHub) devicePolicySignature(device DeviceRecord) string {
	p := h.RuntimePolicy()
	paths := cleanAllowedPaths(device.AllowedPaths)
	if len(paths) == 0 {
		paths = cleanAllowedPaths(p.AllowedPaths)
	}
	return policySignature(device.ReadOnly || p.ReadOnly, paths, p.MaxFileSizeMB)
}

package meshcore

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

type DeviceSettingsRequest struct {
	Identity string       `json:"identity"`
	Revision string       `json:"revision"`
	Section  string       `json:"section"`
	Values   DeviceValues `json:"values"`
}

func (m *Manager) readDeviceLocked(ctx context.Context, c *companion) (DeviceSnapshot, error) {
	st, err := m.refresh(ctx, c)
	if err != nil {
		return DeviceSnapshot{}, err
	}
	d, err := c.readDevice(ctx, st)
	if err != nil {
		return d, err
	}
	b, _ := json.Marshal(d.Values)
	h := sha256.Sum256(append([]byte(st.IdentityKey+"\x00"+c.session+"\x00"), b...))
	d.Revision = hex.EncodeToString(h[:])
	m.mu.Lock()
	d.AllowSettings = m.cfg.AllowDeviceSettings
	d.AllowRemote = m.cfg.AllowRemoteDiagnostics
	m.mu.Unlock()
	return d, nil
}

func (m *Manager) Device(ctx context.Context) (DeviceSnapshot, error) {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	c, enabled := m.conn, m.cfg.Enabled
	m.mu.Unlock()
	if c == nil || !enabled {
		return DeviceSnapshot{Status: m.Status()}, fmt.Errorf("not_connected")
	}
	select {
	case m.writeSlot <- struct{}{}:
		defer func() { <-m.writeSlot }()
	case <-ctx.Done():
		return DeviceSnapshot{}, ctx.Err()
	}
	d, err := m.readDeviceLocked(ctx, c)
	if err != nil {
		return d, err
	}
	m.mu.Lock()
	local := m.localDiagnostic
	fresh := m.localSession == c.session && local != nil && time.Now().Unix()-local.At < 30
	busy := m.activeDiagnostic != ""
	m.mu.Unlock()
	if !fresh && !busy {
		local = c.readLocal(ctx, d.Status.Device.ProtocolVersion)
		m.mu.Lock()
		m.localDiagnostic = local
		m.localSession = c.session
		m.mu.Unlock()
	}
	if fresh || !busy {
		d.Local = local
	}
	return d, nil
}

func otherCommand(v DeviceValues, version byte) []byte {
	b := []byte{38, v.ManualAddContacts}
	if version >= 5 {
		b = append(b, v.TelemetryBase|v.TelemetryLocation<<2|v.TelemetryEnvironment<<4, v.AdvertLocationPolicy)
	}
	if version >= 7 {
		b = append(b, v.MultiACKs)
	}
	return b
}

func deviceCommands(d DeviceSnapshot, section string, proposed DeviceValues) (DeviceValues, [][]byte, error) {
	v := d.Values
	var commands [][]byte
	invalid := func() (DeviceValues, [][]byte, error) { return v, nil, fmt.Errorf("invalid_settings") }
	unsupported := func() (DeviceValues, [][]byte, error) { return v, nil, fmt.Errorf("unsupported") }
	switch section {
	case "identity":
		proposed.Name = strings.TrimSpace(proposed.Name)
		if !validName(proposed.Name) || math.IsNaN(proposed.Latitude) || math.IsNaN(proposed.Longitude) || math.IsInf(proposed.Latitude, 0) || math.IsInf(proposed.Longitude, 0) || math.Abs(proposed.Latitude) > 90 || math.Abs(proposed.Longitude) > 180 || proposed.AdvertLocationPolicy > 1 {
			return invalid()
		}
		v.Name = proposed.Name
		v.Latitude = math.Round(proposed.Latitude*1e6) / 1e6
		v.Longitude = math.Round(proposed.Longitude*1e6) / 1e6
		v.AdvertLocationPolicy = proposed.AdvertLocationPolicy
		if v.Name != d.Values.Name {
			commands = append(commands, append([]byte{8}, v.Name...))
		}
		if v.Latitude != d.Values.Latitude || v.Longitude != d.Values.Longitude {
			b := make([]byte, 9)
			b[0] = 14
			binary.LittleEndian.PutUint32(b[1:], uint32(int32(math.Round(v.Latitude*1e6))))
			binary.LittleEndian.PutUint32(b[5:], uint32(int32(math.Round(v.Longitude*1e6))))
			commands = append(commands, b)
		}
		if v.AdvertLocationPolicy != d.Values.AdvertLocationPolicy {
			if d.Features["other"] != "available" {
				return unsupported()
			}
			commands = append(commands, otherCommand(v, d.Status.Device.ProtocolVersion))
		}
	case "contacts":
		if proposed.ManualAddContacts > 1 || proposed.TelemetryBase > 2 || proposed.TelemetryLocation > 2 || proposed.TelemetryEnvironment > 2 {
			return invalid()
		}
		v.ManualAddContacts = proposed.ManualAddContacts
		v.TelemetryBase = proposed.TelemetryBase
		v.TelemetryLocation = proposed.TelemetryLocation
		v.TelemetryEnvironment = proposed.TelemetryEnvironment
		if v.ManualAddContacts != d.Values.ManualAddContacts || v.TelemetryBase != d.Values.TelemetryBase || v.TelemetryLocation != d.Values.TelemetryLocation || v.TelemetryEnvironment != d.Values.TelemetryEnvironment {
			if d.Features["other"] != "available" {
				return unsupported()
			}
			commands = append(commands, otherCommand(v, d.Status.Device.ProtocolVersion))
		}
		if !reflect.DeepEqual(proposed.AutoAddMask, v.AutoAddMask) || !reflect.DeepEqual(proposed.AutoAddMaxHops, v.AutoAddMaxHops) {
			if d.Features["auto_add"] != "available" || proposed.AutoAddMask == nil {
				return unsupported()
			}
			if *proposed.AutoAddMask > 31 {
				return invalid()
			}
			b := []byte{58, *proposed.AutoAddMask}
			if proposed.AutoAddMaxHops != nil {
				if d.Features["auto_add_max_hops"] != "available" {
					return unsupported()
				}
				if *proposed.AutoAddMaxHops > 64 {
					return invalid()
				}
				b = append(b, *proposed.AutoAddMaxHops)
			} else if v.AutoAddMaxHops != nil {
				return invalid()
			}
			v.AutoAddMask = proposed.AutoAddMask
			v.AutoAddMaxHops = proposed.AutoAddMaxHops
			commands = append(commands, b)
		}
	case "radio":
		if proposed.FrequencyKHz < 150000 || proposed.FrequencyKHz > 2500000 || proposed.BandwidthHz < 7000 || proposed.BandwidthHz > 500000 || proposed.SpreadingFactor < 5 || proposed.SpreadingFactor > 12 || proposed.CodingRate < 5 || proposed.CodingRate > 8 || proposed.TxPower < -9 || proposed.TxPower > d.Status.Radio.MaxTxPower {
			return invalid()
		}
		v.FrequencyKHz = proposed.FrequencyKHz
		v.BandwidthHz = proposed.BandwidthHz
		v.SpreadingFactor = proposed.SpreadingFactor
		v.CodingRate = proposed.CodingRate
		v.TxPower = proposed.TxPower
		v.MultiACKs = proposed.MultiACKs
		if !reflect.DeepEqual(proposed.Repeat, v.Repeat) && (d.Features["repeat"] != "available" || proposed.Repeat == nil) {
			return unsupported()
		}
		v.Repeat = proposed.Repeat
		if v.FrequencyKHz != d.Values.FrequencyKHz || v.BandwidthHz != d.Values.BandwidthHz || v.SpreadingFactor != d.Values.SpreadingFactor || v.CodingRate != d.Values.CodingRate || !reflect.DeepEqual(v.Repeat, d.Values.Repeat) {
			if v.Repeat != nil && *v.Repeat {
				valid := false
				for _, r := range d.RepeatRanges {
					valid = valid || (v.FrequencyKHz >= r[0] && v.FrequencyKHz <= r[1])
				}
				if !valid {
					return invalid()
				}
			}
			b := make([]byte, 11)
			b[0] = 11
			binary.LittleEndian.PutUint32(b[1:], v.FrequencyKHz)
			binary.LittleEndian.PutUint32(b[5:], v.BandwidthHz)
			b[9] = v.SpreadingFactor
			b[10] = v.CodingRate
			if v.Repeat != nil {
				flag := byte(0)
				if *v.Repeat {
					flag = 1
				}
				b = append(b, flag)
			}
			commands = append(commands, b)
		}
		if v.TxPower != d.Values.TxPower {
			commands = append(commands, []byte{12, byte(v.TxPower)})
		}
		if v.MultiACKs != d.Values.MultiACKs {
			if d.Features["multi_ack"] != "available" {
				return unsupported()
			}
			commands = append(commands, otherCommand(v, d.Status.Device.ProtocolVersion))
		}
		if !reflect.DeepEqual(proposed.PathHashMode, v.PathHashMode) {
			if d.Features["path_hash"] != "available" || proposed.PathHashMode == nil {
				return unsupported()
			}
			if *proposed.PathHashMode > 2 {
				return invalid()
			}
			v.PathHashMode = proposed.PathHashMode
			commands = append(commands, []byte{61, 0, *v.PathHashMode})
		}
	default:
		return invalid()
	}
	return v, commands, nil
}

// Device writes use the same lifecycle and wire mutation locks as channel edits,
// with an independent recovery marker so reconciliation never resets channels.
func (m *Manager) SaveDeviceSettings(ctx context.Context, req DeviceSettingsRequest) (DeviceSnapshot, error) {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	c, cfg, busy := m.conn, m.cfg, m.activeDiagnostic != ""
	m.mu.Unlock()
	if !cfg.AllowDeviceSettings {
		return DeviceSnapshot{}, fmt.Errorf("permission_denied")
	}
	if c == nil || !cfg.Enabled {
		return DeviceSnapshot{}, fmt.Errorf("not_connected")
	}
	if busy {
		return DeviceSnapshot{}, fmt.Errorf("busy")
	}
	select {
	case m.writeSlot <- struct{}{}:
		defer func() { <-m.writeSlot }()
	case <-ctx.Done():
		return DeviceSnapshot{}, ctx.Err()
	}
	d, err := m.readDeviceLocked(ctx, c)
	if err != nil {
		return d, err
	}
	if req.Identity != d.Status.IdentityKey || cfg.IdentityKey != d.Status.IdentityKey {
		return d, fmt.Errorf("binding_required")
	}
	if req.Revision == "" || req.Revision != d.Revision {
		return d, fmt.Errorf("settings_conflict")
	}
	if req.Section == "reconcile" {
		if d.Status.State != "settings_uncertain" {
			return d, fmt.Errorf("invalid_request")
		}
		if _, err = m.store.db.Exec("DELETE FROM meshcore_meta WHERE key='device_settings_pending'"); err != nil {
			return d, err
		}
		d.Status, err = m.refresh(ctx, c)
		m.changed(Change{})
		return d, err
	}
	if d.Status.State != "connected" {
		return d, fmt.Errorf("binding_required")
	}
	var expected DeviceValues
	var commands [][]byte
	var clockTarget uint32
	if req.Section == "clock" {
		if d.Features["clock"] != "available" {
			return d, fmt.Errorf("unsupported")
		}
		clockTarget = uint32(time.Now().Unix())
		b := make([]byte, 5)
		b[0] = 6
		binary.LittleEndian.PutUint32(b[1:], clockTarget)
		commands = [][]byte{b}
		expected = d.Values
	} else {
		expected, commands, err = deviceCommands(d, req.Section, req.Values)
		if err != nil {
			return d, err
		}
	}
	if len(commands) == 0 {
		return d, nil
	}
	marker, _ := json.Marshal(struct {
		Identity, Section string
		Expected          DeviceValues
	}{req.Identity, req.Section, expected})
	if _, err = m.store.db.Exec("INSERT INTO meshcore_meta(key,value) VALUES('device_settings_pending',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(marker)); err != nil {
		return d, err
	}
	m.mu.Lock()
	m.editing = true
	m.status.State = "updating"
	if m.runCancel != nil {
		m.runCancel()
	}
	m.mu.Unlock()
	m.changed(Change{})
	defer func() { m.mu.Lock(); m.editing = false; m.mu.Unlock(); m.changed(Change{}) }()
	var writeErr error
	for _, cmd := range commands {
		if _, writeErr = c.request(ctx, cmd, 0); writeErr != nil {
			break
		}
	}
	// Readback happens even after a rejected middle command, showing partial writes.
	next, readErr := m.readDeviceLocked(ctx, c)
	verified := readErr == nil && next.Status.IdentityKey == req.Identity && reflect.DeepEqual(next.Values, expected)
	if req.Section == "clock" {
		verified = verified && next.Clock != nil && *next.Clock >= clockTarget && uint64(*next.Clock) <= uint64(clockTarget)+20
	}
	if writeErr == nil && verified {
		if _, err = m.store.db.Exec("DELETE FROM meshcore_meta WHERE key='device_settings_pending'"); err == nil {
			m.mu.Lock()
			m.editing = false
			m.mu.Unlock()
			next.Status, err = m.refresh(ctx, c)
			return next, err
		}
	}
	if readErr != nil {
		next = d
	}
	next.Status.State = "settings_uncertain"
	m.mu.Lock()
	m.status.State = "settings_uncertain"
	m.mu.Unlock()
	return next, fmt.Errorf("outcome_unknown")
}

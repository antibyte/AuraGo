package meshcore

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

type commandError struct {
	Command, Code byte
	Disabled      bool
}

func (e *commandError) Error() string {
	return fmt.Sprintf("companion rejected command %d (code %d)", e.Command, e.Code)
}
func commandFailure(err error) string {
	var rejected *commandError
	if errors.As(err, &rejected) && (rejected.Code == 1 || rejected.Disabled) {
		return "unsupported"
	}
	return "unavailable"
}

type DeviceValues struct {
	Name                 string  `json:"name"`
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	AdvertLocationPolicy byte    `json:"advert_location_policy"`
	ManualAddContacts    byte    `json:"manual_add_contacts"`
	TelemetryBase        byte    `json:"telemetry_base"`
	TelemetryLocation    byte    `json:"telemetry_location"`
	TelemetryEnvironment byte    `json:"telemetry_environment"`
	AutoAddMask          *byte   `json:"auto_add_mask"`
	AutoAddMaxHops       *byte   `json:"auto_add_max_hops"`
	FrequencyKHz         uint32  `json:"frequency_khz"`
	BandwidthHz          uint32  `json:"bandwidth_hz"`
	SpreadingFactor      byte    `json:"spreading_factor"`
	CodingRate           byte    `json:"coding_rate"`
	TxPower              int8    `json:"tx_power_dbm"`
	MultiACKs            byte    `json:"multi_acks"`
	Repeat               *bool   `json:"repeat"`
	PathHashMode         *byte   `json:"path_hash_mode"`
}

type DeviceSnapshot struct {
	Status        Status            `json:"status"`
	Values        DeviceValues      `json:"values"`
	Revision      string            `json:"revision"`
	Features      map[string]string `json:"features"`
	RepeatRanges  [][2]uint32       `json:"repeat_ranges_khz"`
	Clock         *uint32           `json:"clock"`
	ClockAt       int64             `json:"clock_at"`
	Local         *LocalDiagnostic  `json:"local,omitempty"`
	AllowSettings bool              `json:"allow_device_settings"`
	AllowRemote   bool              `json:"allow_remote_diagnostics"`
}

// Every group is timestamped independently. A missing group is not a zero.
type Measurement struct {
	At     int64              `json:"at"`
	Values map[string]float64 `json:"values"`
	State  string             `json:"state"`
}
type LocalDiagnostic struct {
	Groups map[string]Measurement `json:"groups"`
	At     int64                  `json:"at"`
}

func deviceValues(st Status) DeviceValues {
	v := DeviceValues{Name: st.Name}
	if r := st.Radio; r != nil {
		v.AdvertLocationPolicy = r.AdvertLocationPolicy
		v.ManualAddContacts = r.ManualAddContacts
		v.TelemetryBase = r.TelemetryMode & 3
		v.TelemetryLocation = (r.TelemetryMode >> 2) & 3
		v.TelemetryEnvironment = (r.TelemetryMode >> 4) & 3
		v.FrequencyKHz = r.FrequencyKHz
		v.BandwidthHz = r.BandwidthHz
		v.SpreadingFactor = r.SpreadingFactor
		v.CodingRate = r.CodingRate
		v.TxPower = r.TxPower
		v.MultiACKs = r.MultiACKs
		if r.Position != nil {
			v.Latitude = r.Position.Latitude
			v.Longitude = r.Position.Longitude
		}
	}
	if st.Device != nil {
		v.Repeat = st.Device.RepeatEnabled
		v.PathHashMode = st.Device.PathHashMode
	}
	return v
}

func (c *companion) readDevice(ctx context.Context, st Status) (DeviceSnapshot, error) {
	d := DeviceSnapshot{Features: map[string]string{}}
	d.Status = st
	d.Values = deviceValues(st)
	for _, f := range []string{"identity", "radio", "clock"} {
		d.Features[f] = "available"
	}
	for _, f := range []string{"other", "auto_add", "auto_add_max_hops", "repeat", "path_hash"} {
		d.Features[f] = "unsupported"
	}
	if st.Device.ProtocolVersion >= 5 {
		d.Features["other"] = "available"
	}
	d.Features["multi_ack"] = "unsupported"
	if st.Device.ProtocolVersion >= 7 {
		d.Features["multi_ack"] = "available"
	}
	if st.Device.PathHashMode != nil {
		d.Features["path_hash"] = "available"
	}
	if st.Device.ProtocolVersion >= 9 {
		b, e := c.request(ctx, []byte{59}, 25)
		if e == nil && len(b[0]) >= 2 && len(b[0]) <= 3 {
			d.Values.AutoAddMask = &b[0][1]
			d.Features["auto_add"] = "available"
			if len(b[0]) == 3 && b[0][2] <= 64 {
				d.Values.AutoAddMaxHops = &b[0][2]
				d.Features["auto_add_max_hops"] = "available"
			}
		} else {
			d.Features["auto_add"] = commandFailure(e)
		}
	}
	if st.Device.RepeatEnabled != nil {
		b, e := c.request(ctx, []byte{60}, 26)
		if e == nil && len(b[0]) > 1 && (len(b[0])-1)%8 == 0 {
			d.Features["repeat"] = "available"
			for i := 1; i < len(b[0]); i += 8 {
				lo, hi := binary.LittleEndian.Uint32(b[0][i:]), binary.LittleEndian.Uint32(b[0][i+4:])
				if lo > hi || lo < 150000 || hi > 2500000 {
					d.Features["repeat"] = "unavailable"
					d.RepeatRanges = nil
					break
				}
				d.RepeatRanges = append(d.RepeatRanges, [2]uint32{lo, hi})
			}
		} else {
			d.Features["repeat"] = commandFailure(e)
		}
	}
	b, e := c.request(ctx, []byte{5}, 9)
	if e == nil && len(b[0]) == 5 {
		n := binary.LittleEndian.Uint32(b[0][1:])
		d.Clock = &n
		d.ClockAt = time.Now().Unix()
	} else {
		d.Features["clock"] = commandFailure(e)
	}
	select {
	case <-c.done:
		return d, fmt.Errorf("not_connected")
	default:
	}
	return d, nil
}

func decodeLocal(b []byte, group string) (map[string]float64, error) {
	v := map[string]float64{}
	u16 := func(i int) float64 { return float64(binary.LittleEndian.Uint16(b[i:])) }
	u32 := func(i int) float64 { return float64(binary.LittleEndian.Uint32(b[i:])) }
	switch group {
	case "storage":
		if (len(b) != 3 && len(b) != 11) || b[0] != 12 {
			return nil, fmt.Errorf("invalid_frame")
		}
		v["battery_mv"] = u16(1)
		if len(b) == 11 {
			v["storage_used_kb"] = u32(3)
			v["storage_total_kb"] = u32(7)
		}
	case "core":
		if len(b) != 11 || b[0] != 24 || b[1] != 0 {
			return nil, fmt.Errorf("invalid_frame")
		}
		v["battery_mv"] = u16(2)
		v["uptime_seconds"] = u32(4)
		v["error_flags"] = u16(8)
		v["queue_length"] = float64(b[10])
	case "radio":
		if len(b) != 14 || b[0] != 24 || b[1] != 1 {
			return nil, fmt.Errorf("invalid_frame")
		}
		v["noise_floor_dbm"] = float64(int16(binary.LittleEndian.Uint16(b[2:])))
		v["last_rssi_dbm"] = float64(int8(b[4]))
		v["last_snr_db"] = float64(int8(b[5])) / 4
		v["tx_airtime_seconds"] = u32(6)
		v["rx_airtime_seconds"] = u32(10)
	case "packets":
		if len(b) != 30 || b[0] != 24 || b[1] != 2 {
			return nil, fmt.Errorf("invalid_frame")
		}
		for i, k := range []string{"received", "sent", "sent_flood", "sent_direct", "received_flood", "received_direct", "receive_errors"} {
			v[k] = u32(2 + i*4)
		}
	default:
		return nil, fmt.Errorf("invalid_request")
	}
	return v, nil
}

func (c *companion) readLocal(ctx context.Context, version byte) *LocalDiagnostic {
	d := &LocalDiagnostic{At: time.Now().Unix(), Groups: map[string]Measurement{}}
	for i, group := range []string{"storage", "core", "radio", "packets"} {
		g := Measurement{State: "unsupported"}
		if i == 0 || version >= 8 {
			cmd, code := []byte{20}, byte(12)
			if i > 0 {
				cmd = []byte{56, byte(i - 1)}
				code = 24
			}
			b, e := c.request(ctx, cmd, code)
			if e == nil {
				g.Values, e = decodeLocal(b[0], group)
			}
			g.At = time.Now().Unix()
			g.State = "available"
			if e != nil {
				g.State = commandFailure(e)
			}
		}
		d.Groups[group] = g
	}
	return d
}

package meshcore

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// PathInfo describes either the incoming flood hop count or a cached outgoing
// contact path. Direct-routed text does not carry its incoming hop count.
type PathInfo struct {
	Encoded   byte   `json:"encoded"`
	Route     string `json:"route"`
	Hops      *int   `json:"hops"`
	HashBytes *int   `json:"hash_bytes"`
	Hashes    string `json:"hashes,omitempty"`
}

type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type ReceptionInfo struct {
	FrameType             byte     `json:"frame_type"`
	FrameBytes            int      `json:"companion_frame_bytes"`
	SNR                   *float64 `json:"snr_db"`
	Path                  PathInfo `json:"path"`
	Reserved              string   `json:"reserved_hex,omitempty"`
	ForwardedSenderPrefix string   `json:"forwarded_sender_prefix,omitempty"`
}

// Explicit fields prevent PINs, channel secrets and raw device frames from
// leaking into status responses or persisted message context.
type DeviceInfo struct {
	ProtocolVersion byte   `json:"protocol_version"`
	ContactCapacity int    `json:"contact_capacity"`
	BuildDate       string `json:"build_date,omitempty"`
	Manufacturer    string `json:"manufacturer,omitempty"`
	RepeatEnabled   *bool  `json:"repeat_enabled,omitempty"`
	PathHashMode    *byte  `json:"path_hash_mode,omitempty"`
}

type RadioInfo struct {
	Type                 byte      `json:"type"`
	TxPower              int8      `json:"tx_power_dbm"`
	MaxTxPower           int8      `json:"max_tx_power_dbm"`
	Position             *Position `json:"configured_position,omitempty"`
	MultiACKs            byte      `json:"multi_acks"`
	AdvertLocationPolicy byte      `json:"advert_location_policy"`
	TelemetryMode        byte      `json:"telemetry_mode"`
	ManualAddContacts    byte      `json:"manual_add_contacts"`
	FrequencyKHz         uint32    `json:"frequency_khz"`
	BandwidthHz          uint32    `json:"bandwidth_hz"`
	SpreadingFactor      byte      `json:"spreading_factor"`
	CodingRate           byte      `json:"coding_rate_denominator"`
}

type ReceiverInfo struct {
	IdentityKey string      `json:"identity_key"`
	Name        string      `json:"name"`
	Firmware    string      `json:"firmware"`
	SnapshotAt  int64       `json:"snapshot_at"`
	Device      *DeviceInfo `json:"device,omitempty"`
	Radio       *RadioInfo  `json:"radio,omitempty"`
}

func decodePath(encoded byte, outgoing []byte) (PathInfo, error) {
	p := PathInfo{Encoded: encoded, Route: "direct"}
	if encoded == 0xff {
		if outgoing != nil {
			p.Route = "unknown"
		}
		return p, nil
	}
	hops, size := int(encoded&63), int(encoded>>6)+1
	if size == 4 || hops*size > 64 || (outgoing != nil && hops*size > len(outgoing)) {
		return p, fmt.Errorf("invalid encoded path length")
	}
	p.Route = "flood"
	p.Hops, p.HashBytes = &hops, &size
	if outgoing != nil {
		p.Route = "direct"
		p.Hashes = hex.EncodeToString(outgoing[:hops*size])
	}
	return p, nil
}

func decodePosition(b []byte) *Position {
	lat := float64(int32(binary.LittleEndian.Uint32(b[:4]))) / 1e6
	lon := float64(int32(binary.LittleEndian.Uint32(b[4:8]))) / 1e6
	if (lat == 0 && lon == 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return nil
	}
	return &Position{Latitude: lat, Longitude: lon}
}

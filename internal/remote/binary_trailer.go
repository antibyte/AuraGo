package remote

import (
	"encoding/json"
	"fmt"
)

// TrailerMagic is the magic string appended to personalized binaries.
const TrailerMagic = "AURAGO_REMOTE_CONFIG_V1\x00"

// BinaryConfig is injected into the binary trailer for personalized downloads.
type BinaryConfig struct {
	SupervisorURL string `json:"supervisor_url"`
	CACert        string `json:"ca_cert,omitempty"` // PEM-encoded
	EnrollToken   string `json:"enroll_token"`
	DeviceName    string `json:"device_name,omitempty"`
}

// BuildPersonalizedBinary reads a generic binary and appends a config trailer.
func BuildPersonalizedBinary(genericBinary []byte, cfg BinaryConfig) ([]byte, error) {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal binary config: %w", err)
	}

	payloadLen := uint32(len(payload))
	magic := []byte(TrailerMagic)

	// [binary][JSON payload][uint32 length LE][magic]
	result := make([]byte, len(genericBinary)+len(payload)+4+len(magic))
	copy(result, genericBinary)
	offset := len(genericBinary)
	copy(result[offset:], payload)
	offset += len(payload)
	result[offset] = byte(payloadLen)
	result[offset+1] = byte(payloadLen >> 8)
	result[offset+2] = byte(payloadLen >> 16)
	result[offset+3] = byte(payloadLen >> 24)
	offset += 4
	copy(result[offset:], magic)

	return result, nil
}

// ParseBinaryTrailer reads the config trailer from a binary.
func ParseBinaryTrailer(data []byte) (*BinaryConfig, error) {
	magic := []byte(TrailerMagic)
	magicLen := len(magic)

	if len(data) < magicLen+4 {
		return nil, fmt.Errorf("binary too small for trailer")
	}

	tail := data[len(data)-magicLen:]
	for i := range magic {
		if tail[i] != magic[i] {
			return nil, fmt.Errorf("no trailer found (magic mismatch)")
		}
	}

	lenOffset := len(data) - magicLen - 4
	payloadLen := uint32(data[lenOffset]) |
		uint32(data[lenOffset+1])<<8 |
		uint32(data[lenOffset+2])<<16 |
		uint32(data[lenOffset+3])<<24

	if payloadLen > 1<<20 {
		return nil, fmt.Errorf("trailer payload too large: %d bytes", payloadLen)
	}

	payloadStart := lenOffset - int(payloadLen)
	if payloadStart < 0 {
		return nil, fmt.Errorf("invalid trailer payload length")
	}

	var cfg BinaryConfig
	if err := json.Unmarshal(data[payloadStart:lenOffset], &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse trailer config: %w", err)
	}
	return &cfg, nil
}

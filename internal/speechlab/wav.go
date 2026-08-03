package speechlab

import (
	"encoding/binary"
	"fmt"
	"time"
)

type wavMetadata struct {
	format     uint16
	channels   uint16
	sampleRate uint32
	bits       uint16
	dataBytes  uint64
}

// ValidateWAV validates the RIFF/WAVE structure without constraining its PCM
// representation. TTS backends may return a different valid sample rate.
func ValidateWAV(data []byte) error {
	_, err := parseWAV(data)
	return err
}

func parseWAV(data []byte) (wavMetadata, error) {
	var metadata wavMetadata
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return metadata, fmt.Errorf("expected a RIFF/WAVE container")
	}
	declared := int(binary.LittleEndian.Uint32(data[4:8])) + 8
	if declared > len(data) || declared < 44 {
		return metadata, fmt.Errorf("invalid RIFF length")
	}
	foundFormat := false
	foundData := false
	for offset := 12; offset+8 <= declared; {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := offset + 8
		end := start + chunkSize
		if end < start || end > declared {
			return metadata, fmt.Errorf("invalid WAV chunk length")
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return metadata, fmt.Errorf("invalid WAV format chunk")
			}
			metadata.format = binary.LittleEndian.Uint16(data[start : start+2])
			metadata.channels = binary.LittleEndian.Uint16(data[start+2 : start+4])
			metadata.sampleRate = binary.LittleEndian.Uint32(data[start+4 : start+8])
			metadata.bits = binary.LittleEndian.Uint16(data[start+14 : start+16])
			foundFormat = true
		case "data":
			foundData = foundData || chunkSize > 0
			metadata.dataBytes += uint64(chunkSize)
		}
		offset = end + chunkSize%2
	}
	if !foundFormat || !foundData {
		return metadata, fmt.Errorf("WAV is missing format or audio data")
	}
	if metadata.format == 0 || metadata.channels == 0 || metadata.sampleRate == 0 || metadata.bits == 0 {
		return metadata, fmt.Errorf("invalid WAV audio format")
	}
	return metadata, nil
}

// ValidatePCM16WAV validates the canonical browser/SIP ASR wire format.
func ValidatePCM16WAV(data []byte) error {
	metadata, err := parseWAV(data)
	if err != nil {
		return err
	}
	if metadata.format != 1 || metadata.bits != 16 || metadata.channels != 1 || metadata.sampleRate != 16000 {
		return fmt.Errorf("ASR requires mono PCM16 WAV at 16000 Hz")
	}
	return nil
}

// PCM16WAVDuration returns the actual duration represented by PCM data chunks.
func PCM16WAVDuration(data []byte) (time.Duration, error) {
	metadata, err := parseWAV(data)
	if err != nil {
		return 0, err
	}
	if metadata.format != 1 || metadata.bits != 16 || metadata.channels != 1 || metadata.sampleRate != 16000 {
		return 0, fmt.Errorf("ASR requires mono PCM16 WAV at 16000 Hz")
	}
	bytesPerSecond := uint64(metadata.sampleRate) * uint64(metadata.channels) * uint64(metadata.bits) / 8
	if bytesPerSecond == 0 {
		return 0, fmt.Errorf("invalid WAV byte rate")
	}
	return time.Duration(metadata.dataBytes) * time.Second / time.Duration(bytesPerSecond), nil
}
